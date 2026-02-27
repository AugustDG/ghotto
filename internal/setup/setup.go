package setup

import (
	"fmt"
	"strings"

	"github.com/AugustDG/ghotto/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// provider holds a display name and its default model suggestion.
type provider struct {
	Name         string
	ID           string
	DefaultModel string
}

var providers = []provider{
	{Name: "Anthropic", ID: "anthropic", DefaultModel: "claude-sonnet-4-20250514"},
	{Name: "OpenAI", ID: "openai", DefaultModel: "gpt-4o"},
	{Name: "Google", ID: "google", DefaultModel: "gemini-2.5-pro"},
	{Name: "Groq", ID: "groq", DefaultModel: "llama-3.3-70b-versatile"},
	{Name: "DeepSeek", ID: "deepseek", DefaultModel: "deepseek-chat"},
	{Name: "OpenRouter", ID: "openrouter", DefaultModel: "anthropic/claude-sonnet-4-20250514"},
	{Name: "Custom", ID: "", DefaultModel: ""},
}

type step int

const (
	stepProvider step = iota
	stepCustomProvider
	stepModel
	stepDone
)

type model struct {
	step     step
	cursor   int
	provider provider

	// Text input state (used for custom provider and model ID)
	textInput string
	modelID   string

	// Result
	finalModel string
	aborted    bool

	width int
}

// Run launches the interactive setup TUI and returns the selected model string
// in "provider/model" format.
func Run() error {
	cfg, _, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Println()

	p := tea.NewProgram(initialModel(cfg))
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("setup tui failed: %w", err)
	}

	m := result.(model)
	if m.aborted {
		fmt.Println("setup cancelled.")
		return nil
	}

	cfg.Model = m.finalModel
	if err := config.SaveDefault(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("\nsaved model: %s\n", cfg.Model)
	return nil
}

func initialModel(cfg config.Config) model {
	return model{
		step:  stepProvider,
		width: 60,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case tea.KeyMsg:
		switch m.step {
		case stepProvider:
			return m.updateProvider(msg)
		case stepCustomProvider:
			return m.updateTextInput(msg, stepModel)
		case stepModel:
			return m.updateTextInput(msg, stepDone)
		}
	}

	return m, nil
}

func (m model) updateProvider(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(providers)-1 {
			m.cursor++
		}
	case "enter":
		m.provider = providers[m.cursor]
		if m.provider.ID == "" {
			// Custom provider — need text input
			m.step = stepCustomProvider
			m.textInput = ""
		} else {
			m.step = stepModel
			m.textInput = m.provider.DefaultModel
		}
	case "q", "esc", "ctrl+c":
		m.aborted = true
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateTextInput(msg tea.KeyMsg, nextStep step) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		value := strings.TrimSpace(m.textInput)
		if value == "" {
			return m, nil
		}

		if m.step == stepCustomProvider {
			m.provider = provider{Name: value, ID: value, DefaultModel: ""}
			m.step = stepModel
			m.textInput = ""
		} else if m.step == stepModel {
			m.modelID = value
			m.finalModel = m.provider.ID + "/" + m.modelID
			m.step = stepDone
			return m, tea.Quit
		}
	case "backspace":
		if len(m.textInput) > 0 {
			m.textInput = m.textInput[:len(m.textInput)-1]
		}
	case "esc", "ctrl+c":
		m.aborted = true
		return m, tea.Quit
	default:
		// Only accept printable characters
		if len(msg.String()) == 1 {
			m.textInput += msg.String()
		}
	}
	return m, nil
}

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")).
			MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true)

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15"))

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true)
)

func (m model) View() string {
	var b strings.Builder

	switch m.step {
	case stepProvider:
		b.WriteString(titleStyle.Render("Select a provider"))
		b.WriteString("\n")

		for i, p := range providers {
			cursor := "  "
			style := normalStyle
			if i == m.cursor {
				cursor = "> "
				style = selectedStyle
			}
			line := cursor + style.Render(p.Name)
			if p.DefaultModel != "" && i == m.cursor {
				line += dimStyle.Render("  (" + p.DefaultModel + ")")
			}
			b.WriteString(line + "\n")
		}

		b.WriteString("\n")
		b.WriteString(dimStyle.Render("↑/↓ navigate • enter select • esc quit"))

	case stepCustomProvider:
		b.WriteString(titleStyle.Render("Enter custom provider ID"))
		b.WriteString("\n")
		b.WriteString(promptStyle.Render("provider: "))
		b.WriteString(inputStyle.Render(m.textInput))
		b.WriteString(cursorStyle.Render("█"))
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("type provider id • enter confirm • esc cancel"))

	case stepModel:
		b.WriteString(titleStyle.Render("Enter model ID"))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("provider: "+m.provider.ID) + "\n")
		b.WriteString(promptStyle.Render("model: "))
		b.WriteString(inputStyle.Render(m.textInput))
		b.WriteString(cursorStyle.Render("█"))
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("type model id • enter confirm • esc cancel"))

	case stepDone:
		// Will be cleared by tea.Quit
	}

	return b.String()
}
