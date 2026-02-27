# AGENTS.md — ghotto (gho)

Guidelines for AI coding agents operating in this repository.

## Build & Run

```bash
# Build
go build -o gho .

# Build (release-style, static, stripped)
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o gho .

# Vet (the only lint check currently configured)
go vet ./...

# Run
./gho help
./gho commit
./gho setup
```

There is no Makefile. Build and install are handled by shell scripts in `scripts/`.

## Testing

No test files exist yet. When adding tests:

```bash
# Run all tests
go test ./...

# Run a single test
go test ./internal/commit -run TestCleanMessage

# Run tests with verbose output
go test -v ./...
```

Test files go next to the code they test (e.g. `internal/commit/commit_test.go`).

## Project Structure

```
main.go                    # entry point — delegates to app.Run()
internal/
  app/app.go               # command router (switch on args[0])
  commit/commit.go         # core commit workflow
  config/config.go         # TOML config (~/.config/ghotto/config.toml)
  git/git.go               # git operations via os/exec
  opencode/opencode.go     # opencode agent management + model listing
  setup/setup.go           # Bubble Tea interactive model picker
scripts/                   # install scripts (bash/zsh)
.github/workflows/         # release workflow (tag-triggered)
```

Each package under `internal/` is a single file. Leaf packages (`config`, `git`, `opencode`)
have zero internal dependencies. Higher-level packages (`commit`, `setup`) compose from leaves.
`app` is the top-level router.

## Dependency Graph

```
main -> app -> commit -> config, git, opencode
            -> setup  -> config, opencode
```

No circular dependencies. All packages are under `internal/` (unexported outside the module).

## Code Style

### Imports

Two groups separated by a blank line: stdlib first, then everything else (own-module + third-party mixed, alphabetized).

```go
import (
    "fmt"
    "os"
    "strings"

    "github.com/AugustDG/ghotto/internal/config"
    tea "github.com/charmbracelet/bubbletea"
)
```

No dot-imports. Named imports only when necessary (e.g. `tea` for bubbletea).

### Naming

- **Exported functions**: PascalCase, verb-first: `Run()`, `Load()`, `Save()`, `IsRepo()`
- **Unexported functions**: camelCase: `buildPrompt()`, `parseOpenCodeOutput()`, `cleanMessage()`
- **Boolean returns**: `Is`/`Has`/`Exists` prefix: `IsRepo()`, `HasStagedChanges()`, `AgentExists()`
- **Command packages**: export `Run() error` as the entry point
- **Types**: PascalCase exported (`Config`), camelCase unexported (`model`, `opencodeEvent`)
- **Constants**: PascalCase exported (`AgentName`), camelCase unexported (`maxVisible`)
- **Shell scripts**: `snake_case` for functions, `UPPER_SNAKE_CASE` for variables

### Error Handling

Two patterns, used consistently:

```go
// Leaf errors — static messages
return errors.New("not a git repository")

// Wrapping with context — always use %w
return fmt.Errorf("loading config: %w", err)
```

- No custom error types. No logging library.
- Errors propagate up and are printed once in `main.go`: `fmt.Fprintln(os.Stderr, "gho:", err)`
- User-facing messages use `fmt.Println` (stdout). Errors go to stderr.
- Ignore errors explicitly only when the value is optional: `recentLog, _ := git.RecentLog(10)`

### Comments

Standard Go doc comments on all exported identifiers:

```go
// IsRepo returns true if the current directory is inside a git repository.
func IsRepo() bool {
```

First word is the identifier name. Sentence case. Ends with period. Unexported functions
get doc comments too when the logic is non-obvious.

### String Construction

Use `strings.Builder` for multi-line string building, not `+` concatenation:

```go
var b strings.Builder
b.WriteString("line one\n")
b.WriteString("line two\n")
return b.String()
```

### File Permissions

Use `0o` prefix octal notation: `0o644` for files, `0o755` for directories.

### External Commands

Run via `os/exec`. Two patterns:

```go
// Capture output
cmd := exec.Command("git", "diff", "--cached")
out, err := cmd.Output()

// Interactive pass-through (editor, git commit)
cmd := exec.Command(...)
cmd.Stdin = os.Stdin
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
cmd.Run()
```

### No CLI Framework

Command routing is a hand-rolled `switch` on `args[0]` in `app.go`. Do not introduce
cobra, urfave/cli, or similar. Keep it simple.

### No Global State

No package-level mutable variables (except immutable lipgloss styles). No `init()` functions.
Config is loaded explicitly and passed as values.

### No Interfaces, No Goroutines

Everything is concrete types and synchronous. Do not introduce interfaces or concurrency
unless there is a clear, justified need.

## Commit Messages

Conventional Commits format:

```
type(scope): description
```

- Types: `feat`, `fix`, `chore`, `refactor`, `docs`, `style`, `test`, `perf`, `ci`, `build`
- Scope: optional but encouraged (e.g. `commit`, `setup`, `opencode`)
- Subject: imperative mood, lowercase, no trailing period
- Body: optional, separated by blank line, only for complex changes

## Release Process

1. Tag the commit: `git tag v0.1.0`
2. Push the tag: `git push origin v0.1.0`
3. GitHub Actions builds for `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`
4. Creates a GitHub Release with tarballs and checksums

## Shell Scripts

All scripts in `scripts/` follow:
- `set -euo pipefail` (strict mode)
- `snake_case` function names, `UPPER_SNAKE_CASE` variables
- `local` for function-scoped variables
- All variable expansions quoted: `"$var"`
- Error messages to stderr: `echo "..." >&2`
