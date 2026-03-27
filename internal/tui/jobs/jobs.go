// internal/tui/jobs/jobs.go
package jobs

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/provider"
	"github.com/steven/manifold/internal/tui/shared"
)

// Model represents the jobs list panel.
type Model struct {
	jobs    []provider.Job
	cursor  int
	Width   int
	Height  int
	Focused bool
}

// New creates a new jobs list panel.
func New(width, height int) Model {
	return Model{Width: width, Height: height}
}

// SetJobs updates the job list and clamps the cursor.
func (m *Model) SetJobs(jobs []provider.Job) {
	m.jobs = jobs
	if m.cursor >= len(m.jobs) {
		m.cursor = max(0, len(m.jobs)-1)
	}
}

// Clear empties the job list and resets the cursor.
func (m *Model) Clear() {
	m.jobs = nil
	m.cursor = 0
}

// Selected returns the currently selected job.
func (m Model) Selected() (provider.Job, bool) {
	if len(m.jobs) == 0 {
		return provider.Job{}, false
	}
	return m.jobs[m.cursor], true
}

// MoveDown moves the cursor down by one.
func (m *Model) MoveDown() {
	if m.cursor < len(m.jobs)-1 {
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
	if len(m.jobs) > 0 {
		m.cursor = len(m.jobs) - 1
	}
}

// View renders the jobs list panel.
func (m Model) View() string {
	var b strings.Builder

	contentHeight := m.Height - 2 // account for border

	for i, j := range m.jobs {
		if i >= contentHeight {
			break
		}

		icon := shared.StatusIcon(string(j.Status))
		color := shared.StatusColor(string(j.Status))
		iconStyled := lipgloss.NewStyle().Foreground(color).Render(icon)

		dur := formatDuration(j.Duration)
		name := j.Name

		line := fmt.Sprintf(" %s %-20s %s", iconStyled, name, dur)
		if i == m.cursor {
			line = shared.SelectedItem.Render(fmt.Sprintf(" %s %-20s %s", icon, name, dur))
		}

		b.WriteString(line)
		if i < len(m.jobs)-1 {
			b.WriteString("\n")
		}
	}

	borderStyle := shared.PanelBorder
	if m.Focused {
		borderStyle = shared.PanelBorderActive
	}

	return borderStyle.Width(m.Width).Height(m.Height).Render(shared.PanelTitle.Render("Jobs") + "\n" + b.String())
}

// formatDuration formats a duration as a short human-readable string.
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
