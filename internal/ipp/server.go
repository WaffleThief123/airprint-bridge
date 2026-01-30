package ipp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
)

// IPP operation codes
const (
	OpPrintJob            = 0x0002
	OpValidateJob         = 0x0004
	OpGetJobAttributes    = 0x0009
	OpGetJobs             = 0x000a
	OpGetPrinterAttributes = 0x000b
	OpCancelJob           = 0x0008
)

// IPP status codes
const (
	StatusOK                    = 0x0000
	StatusOKIgnoredOrSubstituted = 0x0001
	StatusClientErrorBadRequest = 0x0400
	StatusClientErrorNotFound   = 0x0406
	StatusServerErrorInternalError = 0x0500
)

// IPP attribute tags
const (
	TagEnd              = 0x03
	TagOperationAttrs   = 0x01
	TagJobAttrs         = 0x02
	TagPrinterAttrs     = 0x04
	TagUnsupportedAttrs = 0x05
	TagInteger          = 0x21
	TagBoolean          = 0x22
	TagEnum             = 0x23
	TagTextWithoutLang  = 0x41
	TagNameWithoutLang  = 0x42
	TagKeyword          = 0x44
	TagURI              = 0x45
	TagURIScheme        = 0x46
	TagCharset          = 0x47
	TagNaturalLang      = 0x48
	TagMimeMediaType    = 0x49
)

// Server is an IPP proxy server
type Server struct {
	listenAddr  string
	cupsClient  CUPSClient
	printerName string
	printerURI  string
	log         zerolog.Logger
}

// CUPSClient interface for forwarding jobs
type CUPSClient interface {
	PrintJob(printerName string, document io.Reader, jobName string, options map[string]string) (int, error)
	GetJobAttributes(jobID int) (map[string]interface{}, error)
	CancelJob(jobID int) error
}

// PrinterConfig holds printer information for advertising
type PrinterConfig struct {
	Name        string
	MakeModel   string
	Location    string
	Color       bool
	Duplex      bool
	Resolutions []int
}

// NewServer creates a new IPP server
func NewServer(listenAddr string, cupsClient CUPSClient, printer PrinterConfig, log zerolog.Logger) *Server {
	return &Server{
		listenAddr:  listenAddr,
		cupsClient:  cupsClient,
		printerName: printer.Name,
		printerURI:  fmt.Sprintf("ipp://cups.local:%s/printers/%s", strings.Split(listenAddr, ":")[1], printer.Name),
		log:         log.With().Str("component", "ipp-server").Logger(),
	}
}

// ListenAndServe starts the IPP server
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/printers/", s.handlePrinter)

	s.log.Info().Str("addr", s.listenAddr).Msg("starting IPP server")
	return http.ListenAndServe(s.listenAddr, mux)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("AirPrint Bridge IPP Server"))
		return
	}
	s.handleIPP(w, r, "")
}

func (s *Server) handlePrinter(w http.ResponseWriter, r *http.Request) {
	// Extract printer name from path /printers/<name>
	path := strings.TrimPrefix(r.URL.Path, "/printers/")
	printerName := strings.Split(path, "/")[0]
	s.handleIPP(w, r, printerName)
}

func (s *Server) handleIPP(w http.ResponseWriter, r *http.Request, printerName string) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the IPP request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to read request body")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if len(body) < 8 {
		s.log.Error().Msg("request too short")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Parse IPP header
	version := binary.BigEndian.Uint16(body[0:2])
	operation := binary.BigEndian.Uint16(body[2:4])
	requestID := binary.BigEndian.Uint32(body[4:8])

	s.log.Debug().
		Uint16("version", version).
		Uint16("operation", operation).
		Uint32("request_id", requestID).
		Str("printer", printerName).
		Msg("received IPP request")

	var response []byte
	switch operation {
	case OpGetPrinterAttributes:
		response = s.handleGetPrinterAttributes(requestID, printerName)
	case OpPrintJob:
		response = s.handlePrintJob(requestID, printerName, body)
	case OpValidateJob:
		response = s.handleValidateJob(requestID)
	case OpGetJobs:
		response = s.handleGetJobs(requestID)
	case OpGetJobAttributes:
		response = s.handleGetJobAttributes(requestID, body)
	case OpCancelJob:
		response = s.handleCancelJob(requestID, body)
	default:
		s.log.Warn().Uint16("operation", operation).Msg("unsupported operation")
		response = s.buildErrorResponse(requestID, StatusClientErrorBadRequest)
	}

	w.Header().Set("Content-Type", "application/ipp")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(response)
}

func (s *Server) handleGetPrinterAttributes(requestID uint32, printerName string) []byte {
	s.log.Debug().Str("printer", printerName).Msg("handling Get-Printer-Attributes")

	buf := &bytes.Buffer{}

	// IPP header
	_ = binary.Write(buf, binary.BigEndian, uint16(0x0200)) // version 2.0
	_ = binary.Write(buf, binary.BigEndian, uint16(StatusOK))
	_ = binary.Write(buf, binary.BigEndian, requestID)

	// Operation attributes
	buf.WriteByte(TagOperationAttrs)
	s.writeAttribute(buf, TagCharset, "attributes-charset", "utf-8")
	s.writeAttribute(buf, TagNaturalLang, "attributes-natural-language", "en-us")

	// Printer attributes
	buf.WriteByte(TagPrinterAttrs)

	// Required AirPrint attributes
	s.writeAttribute(buf, TagURI, "printer-uri-supported", s.printerURI)
	s.writeAttribute(buf, TagKeyword, "uri-security-supported", "none")
	s.writeAttribute(buf, TagKeyword, "uri-authentication-supported", "none")
	s.writeAttribute(buf, TagNameWithoutLang, "printer-name", s.printerName)
	s.writeAttribute(buf, TagEnum, "printer-state", int32(3)) // idle
	s.writeAttribute(buf, TagKeyword, "printer-state-reasons", "none")
	s.writeAttribute(buf, TagKeyword, "ipp-versions-supported", "2.0")
	s.writeAttribute(buf, TagKeyword, "operations-supported", "") // We'll add these specially
	s.writeOperationsSupported(buf)

	s.writeAttribute(buf, TagMimeMediaType, "document-format-supported", "image/urf")
	s.writeAttributeMulti(buf, TagMimeMediaType, "document-format-supported", []string{
		"application/pdf",
		"image/jpeg",
		"image/png",
	})
	s.writeAttribute(buf, TagMimeMediaType, "document-format-default", "image/urf")

	s.writeAttribute(buf, TagBoolean, "printer-is-accepting-jobs", true)
	s.writeAttribute(buf, TagInteger, "queued-job-count", int32(0))
	s.writeAttribute(buf, TagKeyword, "pdl-override-supported", "attempted")
	s.writeAttribute(buf, TagNameWithoutLang, "printer-make-and-model", "Zebra ZPL Label Printer")
	s.writeAttribute(buf, TagTextWithoutLang, "printer-location", "Local")
	s.writeAttribute(buf, TagBoolean, "color-supported", false)

	// Media sizes - common labels
	s.writeAttribute(buf, TagKeyword, "media-default", "oe_4x6-label_4x6in")
	s.writeAttributeMulti(buf, TagKeyword, "media-supported", []string{
		"oe_4x6-label_4x6in",
		"oe_4x3-label_4x3in",
		"oe_4x4-label_4x4in",
		"oe_3x2-label_3x2in",
		"oe_2x1-label_2x1in",
	})

	// Sides (no duplex for labels)
	s.writeAttribute(buf, TagKeyword, "sides-supported", "one-sided")
	s.writeAttribute(buf, TagKeyword, "sides-default", "one-sided")

	// URF capabilities
	s.writeAttribute(buf, TagKeyword, "urf-supported", "W8")
	s.writeAttributeMulti(buf, TagKeyword, "urf-supported", []string{
		"CP255",
		"RS203",
		"DM1",
		"V1.4",
	})

	// End
	buf.WriteByte(TagEnd)

	return buf.Bytes()
}

func (s *Server) handlePrintJob(requestID uint32, printerName string, body []byte) []byte {
	s.log.Info().Str("printer", printerName).Msg("handling Print-Job")

	// Find where attributes end and document begins
	docStart := s.findDocumentStart(body)
	if docStart < 0 {
		s.log.Error().Msg("could not find document in print job")
		return s.buildErrorResponse(requestID, StatusClientErrorBadRequest)
	}

	document := bytes.NewReader(body[docStart:])

	// Forward to CUPS
	jobID, err := s.cupsClient.PrintJob(s.printerName, document, "AirPrint Job", nil)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to forward job to CUPS")
		return s.buildErrorResponse(requestID, StatusServerErrorInternalError)
	}

	s.log.Info().Int("job_id", jobID).Msg("job forwarded to CUPS")

	// Build success response
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, uint16(0x0200))
	_ = binary.Write(buf, binary.BigEndian, uint16(StatusOK))
	_ = binary.Write(buf, binary.BigEndian, requestID)

	buf.WriteByte(TagOperationAttrs)
	s.writeAttribute(buf, TagCharset, "attributes-charset", "utf-8")
	s.writeAttribute(buf, TagNaturalLang, "attributes-natural-language", "en-us")

	buf.WriteByte(TagJobAttrs)
	s.writeAttribute(buf, TagInteger, "job-id", int32(jobID))
	s.writeAttribute(buf, TagURI, "job-uri", fmt.Sprintf("%s/jobs/%d", s.printerURI, jobID))
	s.writeAttribute(buf, TagEnum, "job-state", int32(3)) // pending

	buf.WriteByte(TagEnd)

	return buf.Bytes()
}

func (s *Server) handleValidateJob(requestID uint32) []byte {
	s.log.Debug().Msg("handling Validate-Job")

	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, uint16(0x0200))
	_ = binary.Write(buf, binary.BigEndian, uint16(StatusOK))
	_ = binary.Write(buf, binary.BigEndian, requestID)

	buf.WriteByte(TagOperationAttrs)
	s.writeAttribute(buf, TagCharset, "attributes-charset", "utf-8")
	s.writeAttribute(buf, TagNaturalLang, "attributes-natural-language", "en-us")

	buf.WriteByte(TagEnd)

	return buf.Bytes()
}

func (s *Server) handleGetJobs(requestID uint32) []byte {
	s.log.Debug().Msg("handling Get-Jobs")

	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, uint16(0x0200))
	_ = binary.Write(buf, binary.BigEndian, uint16(StatusOK))
	_ = binary.Write(buf, binary.BigEndian, requestID)

	buf.WriteByte(TagOperationAttrs)
	s.writeAttribute(buf, TagCharset, "attributes-charset", "utf-8")
	s.writeAttribute(buf, TagNaturalLang, "attributes-natural-language", "en-us")

	// No jobs to report for now
	buf.WriteByte(TagEnd)

	return buf.Bytes()
}

func (s *Server) handleGetJobAttributes(requestID uint32, _ []byte) []byte {
	s.log.Debug().Msg("handling Get-Job-Attributes")

	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, uint16(0x0200))
	_ = binary.Write(buf, binary.BigEndian, uint16(StatusOK))
	_ = binary.Write(buf, binary.BigEndian, requestID)

	buf.WriteByte(TagOperationAttrs)
	s.writeAttribute(buf, TagCharset, "attributes-charset", "utf-8")
	s.writeAttribute(buf, TagNaturalLang, "attributes-natural-language", "en-us")

	buf.WriteByte(TagJobAttrs)
	s.writeAttribute(buf, TagEnum, "job-state", int32(9)) // completed
	s.writeAttribute(buf, TagKeyword, "job-state-reasons", "job-completed-successfully")

	buf.WriteByte(TagEnd)

	return buf.Bytes()
}

func (s *Server) handleCancelJob(requestID uint32, _ []byte) []byte {
	s.log.Debug().Msg("handling Cancel-Job")

	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, uint16(0x0200))
	_ = binary.Write(buf, binary.BigEndian, uint16(StatusOK))
	_ = binary.Write(buf, binary.BigEndian, requestID)

	buf.WriteByte(TagOperationAttrs)
	s.writeAttribute(buf, TagCharset, "attributes-charset", "utf-8")
	s.writeAttribute(buf, TagNaturalLang, "attributes-natural-language", "en-us")

	buf.WriteByte(TagEnd)

	return buf.Bytes()
}

func (s *Server) buildErrorResponse(requestID uint32, status uint16) []byte {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, uint16(0x0200))
	_ = binary.Write(buf, binary.BigEndian, status)
	_ = binary.Write(buf, binary.BigEndian, requestID)

	buf.WriteByte(TagOperationAttrs)
	s.writeAttribute(buf, TagCharset, "attributes-charset", "utf-8")
	s.writeAttribute(buf, TagNaturalLang, "attributes-natural-language", "en-us")

	buf.WriteByte(TagEnd)

	return buf.Bytes()
}

func (s *Server) writeAttribute(buf *bytes.Buffer, tag byte, name string, value interface{}) {
	_ = buf.WriteByte(tag)
	_ = binary.Write(buf, binary.BigEndian, uint16(len(name)))
	_, _ = buf.WriteString(name)

	switch v := value.(type) {
	case string:
		_ = binary.Write(buf, binary.BigEndian, uint16(len(v)))
		_, _ = buf.WriteString(v)
	case int32:
		_ = binary.Write(buf, binary.BigEndian, uint16(4))
		_ = binary.Write(buf, binary.BigEndian, v)
	case bool:
		_ = binary.Write(buf, binary.BigEndian, uint16(1))
		if v {
			_ = buf.WriteByte(1)
		} else {
			_ = buf.WriteByte(0)
		}
	}
}

func (s *Server) writeAttributeMulti(buf *bytes.Buffer, tag byte, _ string, values []string) {
	for _, v := range values {
		_ = buf.WriteByte(tag)
		_ = binary.Write(buf, binary.BigEndian, uint16(0)) // empty name = additional value
		_ = binary.Write(buf, binary.BigEndian, uint16(len(v)))
		_, _ = buf.WriteString(v)
	}
}

func (s *Server) writeOperationsSupported(buf *bytes.Buffer) {
	ops := []int32{
		OpPrintJob,
		OpValidateJob,
		OpGetJobAttributes,
		OpGetJobs,
		OpGetPrinterAttributes,
		OpCancelJob,
	}

	// First value with name
	_ = buf.WriteByte(TagEnum)
	name := "operations-supported"
	_ = binary.Write(buf, binary.BigEndian, uint16(len(name)))
	_, _ = buf.WriteString(name)
	_ = binary.Write(buf, binary.BigEndian, uint16(4))
	_ = binary.Write(buf, binary.BigEndian, ops[0])

	// Additional values without name
	for _, op := range ops[1:] {
		_ = buf.WriteByte(TagEnum)
		_ = binary.Write(buf, binary.BigEndian, uint16(0))
		_ = binary.Write(buf, binary.BigEndian, uint16(4))
		_ = binary.Write(buf, binary.BigEndian, op)
	}
}

func (s *Server) findDocumentStart(body []byte) int {
	// IPP attributes end with TagEnd (0x03)
	// Document data follows immediately after
	for i := 8; i < len(body); i++ {
		if body[i] == TagEnd {
			return i + 1
		}
	}
	return -1
}
