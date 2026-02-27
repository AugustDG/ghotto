package git

import (
	"os"
	"os/exec"
	"strings"
)

// IsRepo returns true if the current directory is inside a git repository.
func IsRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// StagedDiff returns the full diff of staged changes.
func StagedDiff() (string, error) {
	return run("git", "diff", "--cached")
}

// StagedStat returns the diffstat summary of staged changes.
func StagedStat() (string, error) {
	return run("git", "diff", "--cached", "--stat")
}

// HasStagedChanges returns true if there are staged changes.
func HasStagedChanges() bool {
	out, err := run("git", "diff", "--cached", "--quiet")
	// git diff --quiet exits 1 when there ARE changes
	if err != nil {
		return true
	}
	_ = out
	return false
}

// GetEditor returns the user's preferred editor for commit messages.
// It checks git var GIT_EDITOR, then $GIT_EDITOR, $VISUAL, $EDITOR,
// and falls back to "vi".
func GetEditor() string {
	// git var GIT_EDITOR respects core.editor, GIT_EDITOR, VISUAL, EDITOR
	out, err := run("git", "var", "GIT_EDITOR")
	if err == nil {
		editor := strings.TrimSpace(out)
		if editor != "" {
			return editor
		}
	}

	for _, env := range []string{"GIT_EDITOR", "VISUAL", "EDITOR"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}

	return "vi"
}

// CommitWithFile runs git commit using the given file as the message source.
func CommitWithFile(path string) error {
	cmd := exec.Command("git", "commit", "-F", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RecentLog returns the last n commit messages for style reference.
func RecentLog(n int) (string, error) {
	return run("git", "log", "--oneline", "-n", strings.TrimSpace(strings.Repeat("0", 0)+itoa(n)))
}

func run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return string(out), err
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
