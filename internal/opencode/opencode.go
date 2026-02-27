package opencode

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// AgentName is the name of the opencode agent used for commit message generation.
const AgentName = "gho-commit"

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

// AgentExists returns true if the gho-commit agent file exists in the
// global opencode agents directory.
func AgentExists() bool {
	path, err := agentPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// WriteCommitAgent creates or updates the gho-commit agent markdown file
// in ~/.config/opencode/agents/. The agent is a locked-down, one-shot
// commit message generator with all tool permissions denied.
func WriteCommitAgent(model string) error {
	path, err := agentPath()
	if err != nil {
		return fmt.Errorf("determining agent path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating agents directory: %w", err)
	}

	content := buildAgentFile(model)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing agent file: %w", err)
	}

	return nil
}

// agentPath returns the path to the gho-commit agent file.
func agentPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); p != "" {
		return filepath.Join(p, "opencode", "agents", AgentName+".md"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "opencode", "agents", AgentName+".md"), nil
}

// buildAgentFile generates the markdown content for the gho-commit agent.
func buildAgentFile(model string) string {
	return fmt.Sprintf(`---
description: "Generates conventional commit messages from staged git diffs. Output only."
mode: all
model: %s
temperature: 0.3
steps: 1
permission:
  read: deny
  write: deny
  edit: deny
  bash: deny
  glob: deny
  grep: deny
  list: deny
  webfetch: deny
  task: deny
  todowrite: deny
  todoread: deny
---

You are a commit message generator. Your entire response must be the raw commit message and nothing else. Do not include any explanation, commentary, markdown fencing, code blocks, quotation marks, or prefixes like "Here is...". Just the commit message text, ready to be passed directly to git commit.

Format rules:
- Conventional Commits: type(scope): description
- Valid types: feat, fix, chore, refactor, docs, style, test, perf, ci, build, revert
- Scope is optional but encouraged
- Subject line: concise, imperative mood, lowercase, no trailing period
- If the change is complex, add a blank line then a short body (1-3 lines)
- If changes span multiple concerns, use the most significant type

Remember: output ONLY the commit message. No explanation, no markdown, no wrapping. Just the raw message text.
`, model)
}
