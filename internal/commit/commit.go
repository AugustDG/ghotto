package commit

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/AugustDG/ghotto/internal/config"
	"github.com/AugustDG/ghotto/internal/git"
)

// Run executes the full commit flow:
// 1. Verify git repo and staged changes
// 2. Generate commit message via opencode
// 3. Open editor for user review
// 4. Commit if the user saved a non-empty message
func Run() error {
	if !git.IsRepo() {
		return errors.New("not a git repository")
	}

	if !git.HasStagedChanges() {
		return errors.New("nothing staged to commit; stage changes with git add first")
	}

	cfg, _, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	diff, err := git.StagedDiff()
	if err != nil {
		return fmt.Errorf("reading staged diff: %w", err)
	}

	stat, err := git.StagedStat()
	if err != nil {
		return fmt.Errorf("reading staged stat: %w", err)
	}

	recentLog, _ := git.RecentLog(10)

	fmt.Println("generating commit message...")

	msg, err := generateMessage(cfg.Model, diff, recentLog)
	if err != nil {
		return fmt.Errorf("generating commit message: %w", err)
	}

	msg = cleanMessage(msg)
	if msg == "" {
		return errors.New("opencode returned an empty message")
	}

	// Write the message to a temp file for the editor, with
	// a commented stat block (like git commit does).
	tmpFile, err := os.CreateTemp("", "gho-commit-*.txt")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	var content strings.Builder
	content.WriteString(msg)
	content.WriteString("\n\n")
	content.WriteString("# ─── staged changes ───────────────────────────────\n")
	for _, line := range strings.Split(strings.TrimSpace(stat), "\n") {
		content.WriteString("# ")
		content.WriteString(line)
		content.WriteString("\n")
	}
	content.WriteString("#\n")
	content.WriteString("# Lines starting with '#' will be ignored.\n")
	content.WriteString("# An empty message aborts the commit.\n")

	if err := os.WriteFile(tmpFile.Name(), []byte(content.String()), 0o644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	tmpFile.Close()

	// Open the user's editor
	editor := git.GetEditor()
	editorCmd := exec.Command("sh", "-c", editor+" "+shellQuote(tmpFile.Name()))
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}

	// Read back, strip comment lines
	final, err := readNonCommentLines(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("reading edited message: %w", err)
	}

	final = strings.TrimSpace(final)
	if final == "" {
		fmt.Println("aborting commit due to empty message.")
		return nil
	}

	// Write the clean message back for git commit -F
	if err := os.WriteFile(tmpFile.Name(), []byte(final+"\n"), 0o644); err != nil {
		return fmt.Errorf("writing final message: %w", err)
	}

	return git.CommitWithFile(tmpFile.Name())
}

// generateMessage calls opencode run to produce a commit message.
func generateMessage(model, diff, recentLog string) (string, error) {
	prompt := buildPrompt(diff, recentLog)

	args := []string{"run", "--format", "json"}
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, prompt)

	cmd := exec.Command("opencode", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("opencode run failed: %w", err)
	}

	return parseOpenCodeOutput(out)
}

// buildPrompt constructs the prompt for the AI.
func buildPrompt(diff, recentLog string) string {
	var b strings.Builder
	b.WriteString("Generate a git commit message for the following staged changes.\n\n")
	b.WriteString("Rules:\n")
	b.WriteString("- Use Conventional Commits format: type(scope): description\n")
	b.WriteString("- Types: feat, fix, chore, refactor, docs, style, test, perf, ci, build, revert\n")
	b.WriteString("- Scope is optional but encouraged\n")
	b.WriteString("- Description should be concise, imperative mood, lowercase\n")
	b.WriteString("- If changes span multiple concerns, use the most significant type\n")
	b.WriteString("- Add a blank line then a brief body only if the change is complex\n")
	b.WriteString("- Output ONLY the commit message, no markdown fencing, no explanation\n")

	if recentLog != "" {
		b.WriteString("\nRecent commits for style reference:\n")
		b.WriteString(recentLog)
		b.WriteString("\n")
	}

	b.WriteString("\nStaged diff:\n```\n")
	// Truncate very large diffs to avoid token limits
	if len(diff) > 30000 {
		b.WriteString(diff[:30000])
		b.WriteString("\n... (diff truncated)\n")
	} else {
		b.WriteString(diff)
	}
	b.WriteString("```\n")

	return b.String()
}

// opencodeEvent represents a single event from opencode's JSON output.
type opencodeEvent struct {
	Type string `json:"type"`
	// For assistant events
	Part struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"part"`
	// For text events at top level
	Text string `json:"text"`
}

// parseOpenCodeOutput extracts the assistant's text from opencode JSON output.
// opencode --format json outputs newline-delimited JSON events.
func parseOpenCodeOutput(data []byte) (string, error) {
	var texts []string

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	// Increase buffer for large outputs
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event opencodeEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Skip lines that aren't valid JSON
			continue
		}

		// Collect text from assistant text parts
		if event.Type == "text" && event.Part.Text != "" {
			texts = append(texts, event.Part.Text)
		}
		if event.Part.Type == "text" && event.Part.Text != "" {
			texts = append(texts, event.Part.Text)
		}
		if event.Text != "" {
			texts = append(texts, event.Text)
		}
	}

	if len(texts) == 0 {
		// Fallback: try to use the raw output if it looks like plain text
		raw := strings.TrimSpace(string(data))
		if raw != "" && !strings.HasPrefix(raw, "{") {
			return raw, nil
		}
		return "", errors.New("no text found in opencode output")
	}

	// Take the last substantial text block (the final answer)
	return texts[len(texts)-1], nil
}

// cleanMessage strips markdown fencing and extra whitespace from the message.
func cleanMessage(msg string) string {
	msg = strings.TrimSpace(msg)

	// Strip markdown code fences if present
	if strings.HasPrefix(msg, "```") {
		lines := strings.Split(msg, "\n")
		if len(lines) >= 3 {
			// Remove first and last lines (the fences)
			lines = lines[1 : len(lines)-1]
			if strings.HasPrefix(lines[len(lines)-1], "```") {
				lines = lines[:len(lines)-1]
			}
			msg = strings.Join(lines, "\n")
		}
	}

	return strings.TrimSpace(msg)
}

// readNonCommentLines reads a file and returns only non-comment lines.
func readNonCommentLines(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n"), nil
}

// shellQuote wraps a string in single quotes for safe shell use.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
