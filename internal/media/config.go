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
			// Custom media list
			p := Profile{
				Name:         "custom-" + o.PrinterName,
				MediaSizes:   o.MediaSizes,
				DefaultMedia: o.DefaultMedia,
			}
			if p.DefaultMedia == "" && len(p.MediaSizes) > 0 {
				p.DefaultMedia = p.MediaSizes[0]
			}
			r.SetCustom(o.PrinterName, p)
		}
	}
}
