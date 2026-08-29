package llm

import (
	"fmt"
	"os/exec"
	"strings"
)

// GetAPIKey retrieves the OpenRouter API key from pass.
func GetAPIKey() (string, error) {
	out, err := exec.Command("bash", "-c", "pass show pi/openrouter").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get API key from pass: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
