package pipelines

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/provider"
	"github.com/steven/manifold/internal/tui/shared"
)

type Model struct {
	pipelines []provider.Pipeline
	cursor    int
	offset    int // viewport scroll offset
	Width     int
	Height    int
	Focused   bool
}

func New(width, height int) Model {
	return Model{
		Width:  width,
		Height: height,
	}
}

func (m *Model) SetPipelines(pipelines []provider.Pipeline) {
	m.pipelines = pipelines
	if m.cursor >= len(m.pipelines) {
		m.cursor = max(0, len(m.pipelines)-1)
	}
	m.clampOffset()
}

func (m Model) Selected() (provider.Pipeline, bool) {
	if len(m.pipelines) == 0 {
		return provider.Pipeline{}, false
	}
	return m.pipelines[m.cursor], true
}

func (m *Model) MoveDown() {
	if m.cursor < len(m.pipelines)-1 {
		m.cursor++
		m.clampOffset()
	}
}

func (m *Model) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
		m.clampOffset()
	}
}

func (m *Model) GoToTop() {
	m.cursor = 0
	m.offset = 0
}

func (m *Model) GoToBottom() {
	if len(m.pipelines) > 0 {
		m.cursor = len(m.pipelines) - 1
		m.clampOffset()
	}
}

// viewHeight returns the number of visible item lines inside the border.
func (m Model) viewHeight() int {
	// Height - 2 (border) - 1 (title)
	h := m.Height - 3
	if h < 1 {
		return 1
	}
	return h
}

func (m *Model) clampOffset() {
	vh := m.viewHeight()
	// Ensure cursor is visible
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vh {
		m.offset = m.cursor - vh + 1
	}
	// Clamp offset bounds
	maxOffset := len(m.pipelines) - vh
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
}

func (m Model) View() string {
	var b strings.Builder

	vh := m.viewHeight()
	cw := m.Width - 2 // content width inside border

	end := m.offset + vh
	if end > len(m.pipelines) {
		end = len(m.pipelines)
	}

	for i := m.offset; i < end; i++ {
		p := m.pipelines[i]

		icon := shared.StatusIcon(string(p.Status))
		color := shared.StatusColor(string(p.Status))
		iconStyled := lipgloss.NewStyle().Foreground(color).Render(icon)

		ref := truncate(p.Ref, cw-4)

		if i == m.cursor {
			b.WriteString(shared.SelectedItem.Render(fmt.Sprintf(" %s %s", icon, ref)))
		} else {
			b.WriteString(fmt.Sprintf(" %s %s", iconStyled, ref))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	borderStyle := shared.PanelBorder
	if m.Focused {
		borderStyle = shared.PanelBorderActive
	}

	return borderStyle.
		Width(m.Width).
		Height(m.Height).
		Render(shared.PanelTitle.Render("Pipelines") + "\n" + b.String())
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
