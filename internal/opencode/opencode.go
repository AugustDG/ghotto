package opencode

import (
	"errors"
	"os/exec"
	"strings"
)

// ListModels runs `opencode models` and returns the available model IDs
// (e.g. "anthropic/claude-sonnet-4-20250514", "openai/gpt-5.2-codex").
func ListModels() ([]string, error) {
	cmd := exec.Command("opencode", "models")
	out, err := cmd.Output()
	if err != nil {
		// Check if opencode is even installed
		if errors.Is(err, exec.ErrNotFound) {
			return nil, errors.New("opencode not found in PATH; install it from https://opencode.ai")
		}
		return nil, errors.New("failed to list models: " + err.Error())
	}

	var models []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Each line should be "provider/model" — skip anything that
		// doesn't look like a model ID (e.g. error messages).
		if strings.Contains(line, "/") {
			models = append(models, line)
		}
	}

	if len(models) == 0 {
		return nil, errors.New("no models returned by opencode; check your opencode configuration")
	}

	return models, nil
}
