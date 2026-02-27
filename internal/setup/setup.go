package setup

import (
	"fmt"
	"strings"

	"github.com/AugustDG/ghotto/internal/config"
	"github.com/AugustDG/ghotto/internal/opencode"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const maxVisible = 12

type model struct {
	allModels []string // full list from opencode
	filtered  []string // subset matching the filter
	filter    string   // current filter text
	cursor    int      // index into filtered
	offset    int      // scroll offset for the visible window

	selected string // final selection
	aborted  bool

	width int
}

// Run fetches the available models from opencode, launches the interactive
// picker, and saves the selection to config.
func Run() error {
	cfg, _, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Println("fetching available models from opencode...")

	models, err := opencode.ListModels()
	if err != nil {
		return err
	}

	fmt.Printf("found %d models\n\n", len(models))

	p := tea.NewProgram(initialModel(models, cfg))
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("setup tui failed: %w", err)
	}

	m := result.(model)
	if m.aborted {
		fmt.Println("setup cancelled.")
		return nil
	}

	cfg.Model = m.selected
	if err := config.SaveDefault(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("\nsaved model: %s\n", cfg.Model)

	// Create or update the dedicated opencode agent for commit generation.
	verb := "created"
	if opencode.AgentExists() {
		verb = "updated"
	}
	if err := opencode.WriteCommitAgent(cfg.Model); err != nil {
		return fmt.Errorf("writing opencode agent: %w", err)
	}
	fmt.Printf("%s opencode agent: %s\n", verb, opencode.AgentName)

	return nil
}

func initialModel(models []string, cfg config.Config) model {
	m := model{
		allModels: models,
		filtered:  models,
		width:     60,
	}

	// If the user already has a model configured, try to pre-select it
	// by setting the filter to match and positioning the cursor.
	if cfg.Model != "" {
		for i, mod := range models {
			if mod == cfg.Model {
				m.cursor = i
				if i >= maxVisible {
					m.offset = i - maxVisible/2
				}
				break
			}
		}
	}

	return m
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
		switch msg.String() {
		case "ctrl+c":
			m.aborted = true
			return m, tea.Quit

		case "esc":
			if m.filter != "" {
				// Clear filter first
				m.filter = ""
				m.applyFilter()
			} else {
				m.aborted = true
				return m, tea.Quit
			}

		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
				m.scrollToCursor()
			}

		case "down", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				m.scrollToCursor()
			}

		case "home", "ctrl+a":
			m.cursor = 0
			m.offset = 0

		case "end", "ctrl+e":
			m.cursor = max(0, len(m.filtered)-1)
			m.scrollToCursor()

		case "pgup":
			m.cursor = max(0, m.cursor-maxVisible)
			m.scrollToCursor()

		case "pgdown":
			m.cursor = min(len(m.filtered)-1, m.cursor+maxVisible)
			m.scrollToCursor()

		case "enter":
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				m.selected = m.filtered[m.cursor]
				return m, tea.Quit
			}

		case "backspace":
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.applyFilter()
			}

		default:
			// Printable characters go into the filter
			ch := msg.String()
			if len(ch) == 1 && ch[0] >= 32 && ch[0] <= 126 {
				m.filter += ch
				m.applyFilter()
			}
		}
	}

	return m, nil
}

func (m *model) applyFilter() {
	if m.filter == "" {
		m.filtered = m.allModels
	} else {
		q := strings.ToLower(m.filter)
		m.filtered = m.filtered[:0]
		for _, mod := range m.allModels {
			if strings.Contains(strings.ToLower(mod), q) {
				m.filtered = append(m.filtered, mod)
			}
		}
	}

	// Reset cursor to top of filtered list
	m.cursor = 0
	m.offset = 0
}

func (m *model) scrollToCursor() {
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+maxVisible {
		m.offset = m.cursor - maxVisible + 1
	}
}

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")).
			MarginBottom(1)

	filterLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("12")).
				Bold(true)

	filterTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15"))

	cursorBlockStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("10")).
				Bold(true)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("10")).
				Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	noMatchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Italic(true)
)

func (m model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Select a model"))
	b.WriteString("\n")

	// Filter input
	b.WriteString(filterLabelStyle.Render("  filter: "))
	b.WriteString(filterTextStyle.Render(m.filter))
	b.WriteString(cursorBlockStyle.Render("█"))
	b.WriteString("\n\n")

	// Model list
	if len(m.filtered) == 0 {
		b.WriteString("  ")
		b.WriteString(noMatchStyle.Render("no models match \"" + m.filter + "\""))
		b.WriteString("\n")
	} else {
		end := m.offset + maxVisible
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		// Scroll indicator (top)
		if m.offset > 0 {
			b.WriteString(dimStyle.Render("  ↑ " + fmt.Sprintf("%d more", m.offset)))
			b.WriteString("\n")
		}

		for i := m.offset; i < end; i++ {
			if i == m.cursor {
				b.WriteString("  > ")
				b.WriteString(selectedItemStyle.Render(m.filtered[i]))
			} else {
				b.WriteString("    ")
				b.WriteString(normalItemStyle.Render(m.filtered[i]))
			}
			b.WriteString("\n")
		}

		// Scroll indicator (bottom)
		remaining := len(m.filtered) - end
		if remaining > 0 {
			b.WriteString(dimStyle.Render("  ↓ " + fmt.Sprintf("%d more", remaining)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑/↓ navigate • type to filter • enter select • esc quit"))

	return b.String()
}
