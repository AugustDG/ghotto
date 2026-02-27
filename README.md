# ghotto

AI-powered git commit messages via [opencode](https://opencode.ai).

`ghotto` (binary: `gho`) generates conventional commit messages from your staged diff using an LLM, opens your editor for review, and commits. It creates a dedicated, locked-down opencode agent with no tool access — pure text generation.

## Install

From source (requires Go):

```bash
scripts/install.zsh
```

For bash:

```bash
scripts/install.bash
```

Or download the latest pre-built binary (no Go required):

```bash
curl -fsSL https://raw.githubusercontent.com/AugustDG/ghotto/main/scripts/install-release.sh | bash
```

With a custom install directory:

```bash
scripts/install-release.sh --prefix ~/.local/bin
```

Or using `go install`:

```bash
go install ./...
```

## Quick start

```bash
# configure your model (interactive picker)
gho setup

# stage changes and generate a commit
git add -p
gho commit
```

`gho setup` fetches the available models from opencode, lets you pick one with a filterable list, saves it to config, and creates a dedicated opencode agent (`gho-commit`) for commit message generation.

`gho commit` reads the staged diff, sends it to the agent, and opens your editor (via `git var GIT_EDITOR`) with the generated message. Edit it, save to commit, or clear the file to abort.

## Commands

```text
gho commit    generate a commit message from staged changes, edit, and commit
gho setup     configure model and provider (interactive)
gho help      show help
```

## Config

Config path: `~/.config/ghotto/config.toml`

`ghotto` auto-creates this file on first run.

Example:

```toml
model = "anthropic/claude-sonnet-4-20250514"
```

The `model` field uses the `provider/model` format expected by opencode.

Run `gho setup` to configure interactively, or edit the file directly.

## How it works

1. **`gho setup`** fetches models from `opencode models`, presents an interactive picker, and writes:
   - `~/.config/ghotto/config.toml` — your model choice
   - `~/.config/opencode/agents/gho-commit.md` — a locked-down opencode agent (no tools, one step, low temperature)

2. **`gho commit`** runs when you have staged changes:
   - Reads `git diff --cached` and `git log --oneline -n 10`
   - Sends the diff to `opencode run --agent gho-commit`
   - Writes the generated message to a temp file with a commented stat block
   - Opens your editor (`$GIT_EDITOR` / `$VISUAL` / `$EDITOR` / `vi`)
   - Strips comment lines, commits if non-empty, aborts if empty

3. If `gho commit` is run before `gho setup`, it auto-creates the agent with the default model and suggests running setup.

## The agent

The `gho-commit` agent lives at `~/.config/opencode/agents/gho-commit.md` and is intentionally minimal:

- **All tool permissions denied** — no file reads, no shell commands, no web access
- **One step** — single-shot generation, no agentic looping
- **Low temperature** (0.3) — consistent, predictable output
- **Conventional Commits format** — `type(scope): description`

The agent's system prompt instructs the model to output *only* the raw commit message text with no explanation, fencing, or commentary.

## Commit message format

Generated messages follow [Conventional Commits](https://www.conventionalcommits.org/):

```text
type(scope): description
```

Valid types: `feat`, `fix`, `chore`, `refactor`, `docs`, `style`, `test`, `perf`, `ci`, `build`, `revert`

The agent also receives your last 10 commits for style reference.

## Requirements

- [opencode](https://opencode.ai) — installed and configured with at least one provider
- [git](https://git-scm.com/)
