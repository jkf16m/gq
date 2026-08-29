package config

// defaultConfig returns the hardcoded default configuration.
// This is the lowest priority in the compound chain.
func defaultConfig() *Config {
	return &Config{
		SessionDir:   ".gq/sessions",
		ContextFiles: []string{"AGENTS.md"},
		KeepWalking:  false,
	}
}