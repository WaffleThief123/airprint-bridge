package media

import (
	"strings"
)

// Profile defines media sizes for a specific printer model
type Profile struct {
	Name         string   // Profile name for config reference
	ModelMatch   []string // Substrings to match in printer make/model
	MediaSizes   []string // IPP media size names
	DefaultMedia string   // Default media size
}

// builtinProfiles contains known printer media configurations
var builtinProfiles = []Profile{
	{
		Name:       "zebra-4x6",
		ModelMatch: []string{"Zebra", "ZPL"},
		MediaSizes: []string{
			"oe_4x6-label_4x6in",
			"oe_4x4-label_4x4in",
			"oe_4x3-label_4x3in",
			"oe_4x2-label_4x2in",
			"oe_2.25x1.25-label_2.25x1.25in",
		},
		DefaultMedia: "oe_4x6-label_4x6in",
	},
	{
		Name:       "dymo-labelwriter",
		ModelMatch: []string{"DYMO", "LabelWriter"},
		MediaSizes: []string{
			"oe_w167h288_30256",   // Shipping label 2.31" x 4"
			"oe_w79h252_30252",    // Address label 1.12" x 3.5"
			"oe_w101h252_30320",   // Address label 1.4" x 3.5"
			"oe_w54h144_30330",    // Return address 0.75" x 2"
			"oe_w162h90_30323",    // Shipping label 2.12" x 1.25"
		},
		DefaultMedia: "oe_w167h288_30256",
	},
	{
		Name:       "brother-ql",
		ModelMatch: []string{"Brother", "QL-"},
		MediaSizes: []string{
			"oe_62x100mm_62x100mm",
			"oe_62x29mm_62x29mm",
			"oe_29x90mm_29x90mm",
			"oe_17x54mm_17x54mm",
			"oe_12mm_12mm",
		},
		DefaultMedia: "oe_62x100mm_62x100mm",
	},
	{
		Name:       "rollo",
		ModelMatch: []string{"Rollo"},
		MediaSizes: []string{
			"oe_4x6-label_4x6in",
			"oe_4x4-label_4x4in",
			"oe_4x2-label_4x2in",
		},
		DefaultMedia: "oe_4x6-label_4x6in",
	},
}

// Registry manages media profiles
type Registry struct {
	profiles []Profile
	custom   map[string]Profile // keyed by printer name
}

// NewRegistry creates a registry with builtin profiles
func NewRegistry() *Registry {
	return &Registry{
		profiles: builtinProfiles,
		custom:   make(map[string]Profile),
	}
}

// AddProfile adds a custom profile
func (r *Registry) AddProfile(p Profile) {
	r.profiles = append(r.profiles, p)
}

// SetCustom sets a custom profile for a specific printer name
func (r *Registry) SetCustom(printerName string, p Profile) {
	r.custom[printerName] = p
}

// GetProfile finds the best matching profile for a printer
// Priority: 1. Custom profile for printer name, 2. Model match, 3. nil (use CUPS)
func (r *Registry) GetProfile(printerName, makeModel string) *Profile {
	// Check custom profiles first
	if p, ok := r.custom[printerName]; ok {
		return &p
	}

	// Check model matching
	makeModelLower := strings.ToLower(makeModel)
	for i := range r.profiles {
		for _, match := range r.profiles[i].ModelMatch {
			if strings.Contains(makeModelLower, strings.ToLower(match)) {
				return &r.profiles[i]
			}
		}
	}

	return nil
}

// ApplyProfile applies a profile to override media settings
// Returns the media list and default to use
func (r *Registry) ApplyProfile(printerName, makeModel string, cupsMedia []string, cupsDefault string) (media []string, defaultMedia string) {
	profile := r.GetProfile(printerName, makeModel)

	if profile != nil {
		return profile.MediaSizes, profile.DefaultMedia
	}

	// No profile match, use CUPS values
	return cupsMedia, cupsDefault
}

// ListProfiles returns all available profile names
func (r *Registry) ListProfiles() []string {
	names := make([]string, len(r.profiles))
	for i, p := range r.profiles {
		names[i] = p.Name
	}
	return names
}

// GetProfileByName finds a profile by name
func (r *Registry) GetProfileByName(name string) *Profile {
	for i := range r.profiles {
		if r.profiles[i].Name == name {
			return &r.profiles[i]
		}
	}
	return nil
}
