package config

// validateConfig checks if a config is valid.
// It returns true if the config is valid, false otherwise.
func validateConfig(cfg *Config) bool {
	// A config is valid if it's not nil
	if cfg == nil {
		return false
	}

	// SessionDir is optional, but if present it should be non-empty
	// ContextFiles is optional
	// KeepWalking is optional (defaults to false)

	return true
}