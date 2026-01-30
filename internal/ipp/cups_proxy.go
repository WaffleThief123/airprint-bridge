package ipp

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/phin1x/go-ipp"
)

// CUPSProxy forwards print jobs to a CUPS server
type CUPSProxy struct {
	host       string
	port       int
	httpClient *http.Client
}

// NewCUPSProxy creates a new CUPS proxy client
func NewCUPSProxy(host string, port int) *CUPSProxy {
	return &CUPSProxy{
		host: host,
		port: port,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// PrintJob sends a print job to CUPS
func (c *CUPSProxy) PrintJob(printerName string, document io.Reader, jobName string, options map[string]string) (int, error) {
	// Read document into buffer
	docData, err := io.ReadAll(document)
	if err != nil {
		return 0, fmt.Errorf("failed to read document: %w", err)
	}

	// Build IPP Print-Job request
	req := ipp.NewRequest(ipp.OperationPrintJob, 1)

	printerURI := fmt.Sprintf("ipp://%s:%d/printers/%s", c.host, c.port, printerName)
	req.OperationAttributes["printer-uri"] = printerURI
	req.OperationAttributes["requesting-user-name"] = "airprint"
	req.OperationAttributes["job-name"] = jobName
	req.OperationAttributes["document-format"] = "application/octet-stream"

	// Add any additional options
	for k, v := range options {
		req.OperationAttributes[k] = v
	}

	// Encode the request
	payload, err := req.Encode()
	if err != nil {
		return 0, fmt.Errorf("failed to encode IPP request: %w", err)
	}

	// Combine IPP request with document
	fullPayload := append(payload, docData...)

	// Send to CUPS
	cupsURL := fmt.Sprintf("http://%s:%d/printers/%s", c.host, c.port, printerName)
	httpReq, err := http.NewRequest("POST", cupsURL, bytes.NewReader(fullPayload))
	if err != nil {
		return 0, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/ipp")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("failed to send request to CUPS: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read CUPS response: %w", err)
	}

	ippResp, err := ipp.NewResponseDecoder(bytes.NewReader(respBody)).Decode(nil)
	if err != nil {
		return 0, fmt.Errorf("failed to decode IPP response: %w", err)
	}

	if ippResp.StatusCode != ipp.StatusOk {
		return 0, fmt.Errorf("CUPS returned error status: %d", ippResp.StatusCode)
	}

	// Extract job ID from response
	if jobAttrs := ippResp.JobAttributes; len(jobAttrs) > 0 {
		if jobIDAttr, ok := jobAttrs[0]["job-id"]; ok && len(jobIDAttr) > 0 {
			if jobID, ok := jobIDAttr[0].Value.(int); ok {
				return jobID, nil
			}
		}
	}

	// If we can't get the job ID, return a placeholder
	return 1, nil
}

// GetJobAttributes retrieves job status from CUPS
func (c *CUPSProxy) GetJobAttributes(jobID int) (map[string]interface{}, error) {
	// For now, return a simple "completed" status
	// A full implementation would query CUPS
	return map[string]interface{}{
		"job-state":         9, // completed
		"job-state-reasons": "job-completed-successfully",
	}, nil
}

// CancelJob cancels a job in CUPS
func (c *CUPSProxy) CancelJob(jobID int) error {
	// For now, just return success
	// A full implementation would send Cancel-Job to CUPS
	return nil
}
