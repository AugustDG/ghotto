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
	"github.com/AugustDG/ghotto/internal/opencode"
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

	// Ensure the opencode agent exists; auto-create from config if missing.
	if !opencode.AgentExists() {
		fmt.Println("opencode agent not found, creating with default model...")
		fmt.Println("(run 'gho setup' to configure interactively)")
		if err := opencode.WriteCommitAgent(cfg.Model); err != nil {
			return fmt.Errorf("creating opencode agent: %w", err)
		}
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

	msg, err := generateMessage(diff, recentLog)
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

// generateMessage calls opencode run with the gho-commit agent to produce
// a commit message. The system prompt / role / format rules live in the
// agent file; this prompt is just the data (diff + recent log).
func generateMessage(diff, recentLog string) (string, error) {
	prompt := buildPrompt(diff, recentLog)

	args := []string{"run", "--agent", opencode.AgentName, "--format", "json", prompt}

	cmd := exec.Command("opencode", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("opencode run failed: %w", err)
	}

	return parseOpenCodeOutput(out)
}

// buildPrompt constructs the user message for the agent. The system prompt
// (role, format rules, output constraints) lives in the gho-commit agent
// file, so this is just the data: recent commits for style + the diff.
func buildPrompt(diff, recentLog string) string {
	var b strings.Builder

	b.WriteString("Generate a commit message for the following staged changes.\n")

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
// opencode --format json emits newline-delimited JSON with events like:
//
//	{"type":"text","part":{"type":"text","text":"the message"}}
//	{"type":"error","error":{"name":"APIError","data":{"message":"..."}}}
//	{"type":"step_start",...}
//	{"type":"step_finish",...}
type opencodeEvent struct {
	Type  string           `json:"type"`
	Part  opencodeTextPart `json:"part"`
	Error *opencodeError   `json:"error"`
}

type opencodeTextPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type opencodeError struct {
	Name string `json:"name"`
	Data struct {
		Message string `json:"message"`
	} `json:"data"`
}

// parseOpenCodeOutput extracts the assistant's text from opencode JSON output.
func parseOpenCodeOutput(data []byte) (string, error) {
	var texts []string
	var lastError string

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
			continue
		}

		// Check for error events
		if event.Type == "error" && event.Error != nil {
			msg := event.Error.Data.Message
			if msg == "" {
				msg = event.Error.Name
			}
			lastError = msg
			continue
		}

		// Collect text from assistant text parts
		if event.Type == "text" && event.Part.Type == "text" && event.Part.Text != "" {
			texts = append(texts, event.Part.Text)
		}
	}

	if len(texts) == 0 {
		if lastError != "" {
			return "", fmt.Errorf("opencode error: %s", lastError)
		}
		// Fallback: try to use the raw output if it looks like plain text
		raw := strings.TrimSpace(string(data))
		if raw != "" && !strings.HasPrefix(raw, "{") {
			return raw, nil
		}
		return "", errors.New("no text found in opencode output")
	}

	// Take the last text event (the final answer)
	return texts[len(texts)-1], nil
}

// cleanMessage strips opencode system noise, markdown fencing, and extra
// whitespace from the generated commit message.
func cleanMessage(msg string) string {
	msg = strings.TrimSpace(msg)

	// Strip <system-reminder>...</system-reminder> blocks (and any XML-like tags).
	msg = stripXMLTags(msg)

	// Strip markdown code fences if present
	if strings.HasPrefix(msg, "```") {
		lines := strings.Split(msg, "\n")
		if len(lines) >= 3 {
			lines = lines[1 : len(lines)-1]
			if len(lines) > 0 && strings.HasPrefix(lines[len(lines)-1], "```") {
				lines = lines[:len(lines)-1]
			}
			msg = strings.Join(lines, "\n")
		}
	}

	// Strip leading "---" separators that opencode sometimes injects
	msg = strings.TrimLeft(msg, "-")

	return strings.TrimSpace(msg)
}

// stripXMLTags removes any <tag>...</tag> blocks from the text, including
// multiline ones like <system-reminder>...\n...</system-reminder>.
func stripXMLTags(s string) string {
	result := s
	for {
		start := strings.Index(result, "<")
		if start == -1 {
			break
		}
		// Find the tag name
		tagEnd := strings.IndexAny(result[start+1:], "> \t\n")
		if tagEnd == -1 {
			break
		}
		tagName := result[start+1 : start+1+tagEnd]
		if tagName == "" || tagName[0] == '/' {
			// Stray closing tag or empty — just remove it up to >
			closeAngle := strings.Index(result[start:], ">")
			if closeAngle == -1 {
				break
			}
			result = result[:start] + result[start+closeAngle+1:]
			continue
		}

		// Look for the closing tag </tagName>
		closingTag := "</" + tagName + ">"
		closeIdx := strings.Index(result[start:], closingTag)
		if closeIdx == -1 {
			// Self-closing or unclosed — just remove the opening tag
			closeAngle := strings.Index(result[start:], ">")
			if closeAngle == -1 {
				break
			}
			result = result[:start] + result[start+closeAngle+1:]
		} else {
			// Remove the entire <tag>...</tag> block
			result = result[:start] + result[start+closeIdx+len(closingTag):]
		}
	}
	return result
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
