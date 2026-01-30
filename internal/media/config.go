package media

// ConfigOverride represents a per-printer media configuration from config file
type ConfigOverride struct {
	PrinterName  string   // Match by printer name
	ProfileName  string   // Use a named profile (e.g., "zebra-4x6")
	MediaSizes   []string // Or specify sizes directly
	DefaultMedia string   // Default size
}

// ApplyConfigOverrides loads config overrides into the registry
func (r *Registry) ApplyConfigOverrides(overrides []ConfigOverride) {
	for _, o := range overrides {
		if o.ProfileName != "" {
			// Reference an existing profile
			if p := r.GetProfileByName(o.ProfileName); p != nil {
				r.SetCustom(o.PrinterName, *p)
			}
		} else if len(o.MediaSizes) > 0 {
			// Custom media list - convert strings to MediaSize
			sizes := make([]MediaSize, len(o.MediaSizes))
			for i, name := range o.MediaSizes {
				sizes[i] = MediaSize{Name: name, Description: ""}
			}
			p := Profile{
				Name:         "custom-" + o.PrinterName,
				Sizes:        sizes,
				DefaultMedia: o.DefaultMedia,
			}
			if p.DefaultMedia == "" && len(p.Sizes) > 0 {
				p.DefaultMedia = p.Sizes[0].Name
			}
			r.SetCustom(o.PrinterName, p)
		}
	}
}
