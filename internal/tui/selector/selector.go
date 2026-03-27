package selector

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/tui"
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

// viewContent returns the rendered content as a string (used internally and by tea.Model View).
func (m Model) viewContent() string {
	var b strings.Builder
	title := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorAccent).Render("Select a remote:")
	b.WriteString("\n  " + title + "\n\n")
	for i, r := range m.remotes {
		cursor := "  "
		if i == m.cursor { cursor = "> " }
		line := fmt.Sprintf("  %s%-12s %s", cursor, r.Name, r.URL)
		if i == m.cursor { line = tui.SelectedItem.Render(line) }
		b.WriteString(line + "\n")
	}
	b.WriteString("\n  Press Enter to select, q to quit\n")
	return b.String()
}
