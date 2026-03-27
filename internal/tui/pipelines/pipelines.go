// internal/tui/pipelines/pipelines.go
package pipelines

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/provider"
	"github.com/steven/manifold/internal/tui"
)

// Model represents the pipeline list panel.
type Model struct {
	pipelines []provider.Pipeline
	cursor    int
	Width     int
	Height    int
	Focused   bool
}

// New creates a new pipeline list panel.
func New(width, height int) Model {
	return Model{
		Width:  width,
		Height: height,
	}
}

// SetPipelines updates the pipeline list and clamps the cursor.
func (m *Model) SetPipelines(pipelines []provider.Pipeline) {
	m.pipelines = pipelines
	if m.cursor >= len(m.pipelines) {
		m.cursor = max(0, len(m.pipelines)-1)
	}
}

// Selected returns the currently selected pipeline.
func (m Model) Selected() (provider.Pipeline, bool) {
	if len(m.pipelines) == 0 {
		return provider.Pipeline{}, false
	}
	return m.pipelines[m.cursor], true
}

// MoveDown moves the cursor down by one.
func (m *Model) MoveDown() {
	if m.cursor < len(m.pipelines)-1 {
		m.cursor++
	}
}

// MoveUp moves the cursor up by one.
func (m *Model) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

// GoToTop moves the cursor to the first item.
func (m *Model) GoToTop() {
	m.cursor = 0
}

// GoToBottom moves the cursor to the last item.
func (m *Model) GoToBottom() {
	if len(m.pipelines) > 0 {
		m.cursor = len(m.pipelines) - 1
	}
}

// View renders the pipeline list panel.
func (m Model) View() string {
	var b strings.Builder

	contentHeight := m.Height - 2 // account for border

	for i, p := range m.pipelines {
		if i >= contentHeight {
			break
		}

		icon := tui.StatusIcon(string(p.Status))
		color := tui.StatusColor(string(p.Status))
		iconStyled := lipgloss.NewStyle().Foreground(color).Render(icon)

		ref := truncate(p.Ref, m.Width-10)
		line := fmt.Sprintf(" %s %s", iconStyled, ref)

		if i == m.cursor {
			line = tui.SelectedItem.Render(fmt.Sprintf(" %s %s", icon, ref))
		}

		b.WriteString(line)
		if i < len(m.pipelines)-1 {
			b.WriteString("\n")
		}
	}

	borderStyle := tui.PanelBorder
	if m.Focused {
		borderStyle = tui.PanelBorderActive
	}

	return borderStyle.
		Width(m.Width).
		Height(m.Height).
		Render(tui.PanelTitle.Render("Pipelines") + "\n" + b.String())
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
