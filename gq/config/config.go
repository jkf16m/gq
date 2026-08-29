package config

// Config holds application configuration.
type Config struct {
	SessionDir   string   `json:"sessionDir"`
	ContextFiles []string `json:"contextFiles"`
	KeepWalking  bool     `json:"keepWalking"`
}