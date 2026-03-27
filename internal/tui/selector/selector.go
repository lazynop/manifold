package selector

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/tui/shared"
)

type Remote struct {
	Name string
	URL  string
}

type Model struct {
	remotes  []Remote
	cursor   int
	selected bool
}

func New(remotes []Remote) Model { return Model{remotes: remotes} }

func (m *Model) MoveDown() { if m.cursor < len(m.remotes)-1 { m.cursor++ } }
func (m *Model) MoveUp() { if m.cursor > 0 { m.cursor-- } }
func (m *Model) Select() { m.selected = true }

func (m Model) Selected() (Remote, bool) {
	if !m.selected { return Remote{}, false }
	return m.remotes[m.cursor], true
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model. It handles key navigation and selection.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "j", "down":
			m.MoveDown()
		case "k", "up":
			m.MoveUp()
		case "enter":
			m.Select()
			return m, func() tea.Msg { return tea.Quit() }
		case "q", "ctrl+c":
			return m, func() tea.Msg { return tea.Quit() }
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() tea.View {
	return tea.NewView(m.viewContent())
}

// viewContent returns the rendered content as a string.
func (m Model) viewContent() string {
	var b strings.Builder
	title := lipgloss.NewStyle().Bold(true).Foreground(shared.ColorAccent).Render("Select a remote:")
	b.WriteString("\n  " + title + "\n\n")
	for i, r := range m.remotes {
		cursor := "  "
		if i == m.cursor { cursor = "> " }
		line := fmt.Sprintf("  %s%-12s %s", cursor, r.Name, r.URL)
		if i == m.cursor { line = shared.SelectedItem.Render(line) }
		b.WriteString(line + "\n")
	}
	b.WriteString("\n  Press Enter to select, q to quit\n")
	return b.String()
}
