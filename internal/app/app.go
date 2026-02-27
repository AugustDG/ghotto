package app

import (
	"fmt"

	"github.com/AugustDG/ghotto/internal/commit"
	"github.com/AugustDG/ghotto/internal/setup"
)

// Run is the top-level command router.
func Run(args []string) error {
	if len(args) == 0 {
		printHelp()
		return nil
	}

	switch args[0] {
	case "commit":
		return commit.Run()
	case "setup":
		return setup.Run()
	case "help", "--help", "-h":
		printHelp()
		return nil
	default:
		return fmt.Errorf("unknown command: %s (run 'gho help' for usage)", args[0])
	}
}

func printHelp() {
	fmt.Print(`gho — AI-powered git commit messages via opencode

Usage:
  gho commit    generate a commit message from staged changes, edit, and commit
  gho setup     configure model and provider (interactive)
  gho help      show this help

Workflow:
  1. Stage your changes:  git add -p
  2. Generate & commit:   gho commit
  3. Edit the message in your editor, save to commit (or clear to abort)

Configuration:
  ~/.config/ghotto/config.toml

  Run 'gho setup' to configure interactively, or edit the file directly:
    model = "anthropic/claude-sonnet-4-20250514"

Requires:
  - opencode (https://opencode.ai) installed and configured
  - git
`)
}
