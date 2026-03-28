package jobs

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/provider"
	"github.com/steven/manifold/internal/tui/shared"
)

type Model struct {
	jobs    []provider.Job
	cursor  int
	offset  int // viewport scroll offset
	Width   int
	Height  int
	Focused bool
}

func New(width, height int) Model {
	return Model{Width: width, Height: height}
}

func (m *Model) SetJobs(jobs []provider.Job) {
	m.jobs = jobs
	if m.cursor >= len(m.jobs) {
		m.cursor = max(0, len(m.jobs)-1)
	}
	m.clampOffset()
}

func (m *Model) Clear() {
	m.jobs = nil
	m.cursor = 0
	m.offset = 0
}

func (m Model) Selected() (provider.Job, bool) {
	if len(m.jobs) == 0 {
		return provider.Job{}, false
	}
	return m.jobs[m.cursor], true
}

func (m *Model) MoveDown() {
	if m.cursor < len(m.jobs)-1 {
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
	if len(m.jobs) > 0 {
		m.cursor = len(m.jobs) - 1
		m.clampOffset()
	}
}

// viewHeight returns the number of visible item lines inside the border.
func (m Model) viewHeight() int {
	h := m.Height - 3 // border (2) + title (1)
	if h < 1 {
		return 1
	}
	return h
}

func (m *Model) clampOffset() {
	vh := m.viewHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vh {
		m.offset = m.cursor - vh + 1
	}
	maxOffset := len(m.jobs) - vh
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
	cw := m.Width - 2

	end := m.offset + vh
	if end > len(m.jobs) {
		end = len(m.jobs)
	}

	for i := m.offset; i < end; i++ {
		j := m.jobs[i]

		icon := shared.StatusIcon(string(j.Status))
		color := shared.StatusColor(string(j.Status))
		iconStyled := lipgloss.NewStyle().Foreground(color).Render(icon)

		dur := formatDuration(j.Duration)
		name := truncate(j.Name, cw-len(dur)-5)

		if i == m.cursor {
			b.WriteString(shared.SelectedItem.Render(fmt.Sprintf(" %s %-20s %s", icon, name, dur)))
		} else {
			b.WriteString(fmt.Sprintf(" %s %-20s %s", iconStyled, name, dur))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	borderStyle := shared.PanelBorder
	if m.Focused {
		borderStyle = shared.PanelBorderActive
	}

	return borderStyle.Width(m.Width).Height(m.Height).Render(shared.PanelTitle.Render("Jobs") + "\n" + b.String())
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return ""
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%02ds", m, s)
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
