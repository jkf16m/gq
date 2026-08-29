package config

// compoundConfigs merges multiple configs into one.
// The order is: nearest config overrides, default config is lowest priority.
// Priority order (highest to lowest):
// 1. Nearest .gq/config.json
// 2. Farthest .gq/config.json
// 3. ~/.gq/config.json
// 4. Default hardcoded config
func compoundConfigs(configs []*Config) *Config {
	// Start with the default config
	result := defaultConfig()

	// Apply configs in reverse order (farthest first, nearest last)
	// This way, the nearest config overrides everything else
	for i := len(configs) - 1; i >= 0; i-- {
		cfg := configs[i]
		if cfg == nil {
			continue
		}

		// Only override if the field is non-zero
		if cfg.SessionDir != "" {
			result.SessionDir = cfg.SessionDir
		}

		if len(cfg.ContextFiles) > 0 {
			// Append context files, nearest first
			result.ContextFiles = append(cfg.ContextFiles, result.ContextFiles...)
		}

		// KeepWalking is always false (stop walking)
		// The walking logic handles this externally
	}

	return result
}