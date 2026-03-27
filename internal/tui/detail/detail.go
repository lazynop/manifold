// internal/tui/detail/detail.go
package detail

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/provider"
	"github.com/steven/manifold/internal/tui"
)

const defaultMaxLogLines = 10000

// Model represents the detail panel showing job steps and log output.
type Model struct {
	job        provider.Job
	hasJob     bool
	logLines   []string
	logOffset  int
	autoScroll bool
	maxLogLines int
	Width      int
	Height     int
	Focused    bool
}

// New creates a new detail panel.
func New(width, height int) Model {
	return Model{
		Width:       width,
		Height:      height,
		autoScroll:  true,
		maxLogLines: defaultMaxLogLines,
	}
}

// SetJob sets the current job and clears the log.
func (m *Model) SetJob(j provider.Job) {
	m.job = j
	m.hasJob = true
	m.ClearLog()
}

// HasJob returns true if a job has been set.
func (m Model) HasJob() bool {
	return m.hasJob
}

// Job returns the current job.
func (m Model) Job() provider.Job {
	return m.job
}

// AppendLog appends log text, splitting on newlines. Implements a ring buffer.
func (m *Model) AppendLog(text string) {
	lines := strings.Split(text, "\n")
	// If text ends with \n, last element is empty — drop it
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	m.logLines = append(m.logLines, lines...)

	// Ring buffer: trim to maxLogLines
	if len(m.logLines) > m.maxLogLines {
		overflow := len(m.logLines) - m.maxLogLines
		m.logLines = m.logLines[overflow:]
		// Adjust offset
		if m.logOffset > overflow {
			m.logOffset -= overflow
		} else {
			m.logOffset = 0
		}
	}

	if m.autoScroll {
		m.logOffset = max(0, len(m.logLines)-m.logViewHeight())
	}
}

// ClearLog clears the log buffer.
func (m *Model) ClearLog() {
	m.logLines = nil
	m.logOffset = 0
}

// LogLineCount returns the number of log lines.
func (m Model) LogLineCount() int {
	return len(m.logLines)
}

// LogOffset returns the current log scroll offset.
func (m Model) LogOffset() int {
	return m.logOffset
}

// SetLogOffset sets the log scroll offset.
func (m *Model) SetLogOffset(offset int) {
	m.logOffset = offset
}

// ScrollUp scrolls the log up and disables auto-scroll.
func (m *Model) ScrollUp() {
	m.autoScroll = false
	if m.logOffset > 0 {
		m.logOffset--
	}
}

// ScrollDown scrolls the log down, re-enabling auto-scroll when at bottom.
func (m *Model) ScrollDown() {
	maxOffset := max(0, len(m.logLines)-m.logViewHeight())
	if m.logOffset < maxOffset {
		m.logOffset++
	}
	if m.logOffset >= maxOffset {
		m.autoScroll = true
	}
}

// logViewHeight returns the number of lines available for log display.
func (m Model) logViewHeight() int {
	stepsHeight := len(m.job.Steps) + 1 // steps + separator
	available := m.Height - 2 - stepsHeight
	if available < 1 {
		return 1
	}
	return available
}

// View renders the detail panel with steps and log output.
func (m Model) View() string {
	var b strings.Builder

	if !m.hasJob {
		borderStyle := tui.PanelBorder
		if m.Focused {
			borderStyle = tui.PanelBorderActive
		}
		placeholder := tui.NormalItem.Render("  Select a job to view details")
		return borderStyle.Width(m.Width).Height(m.Height).Render(
			tui.PanelTitle.Render("Detail") + "\n" + placeholder,
		)
	}

	// Steps section
	b.WriteString(tui.PanelTitle.Render(fmt.Sprintf("Job: %s", m.job.Name)))
	b.WriteString("\n")

	for _, step := range m.job.Steps {
		icon := tui.StatusIcon(string(step.Status))
		color := tui.StatusColor(string(step.Status))
		iconStyled := lipgloss.NewStyle().Foreground(color).Render(icon)
		line := fmt.Sprintf("  %s %s", iconStyled, step.Name)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Separator
	b.WriteString(strings.Repeat("─", max(0, m.Width-4)))
	b.WriteString("\n")

	// Log section
	logHeight := m.logViewHeight()
	end := m.logOffset + logHeight
	if end > len(m.logLines) {
		end = len(m.logLines)
	}

	visible := m.logLines[m.logOffset:end]
	for _, line := range visible {
		b.WriteString(tui.NormalItem.Render(line))
		b.WriteString("\n")
	}

	borderStyle := tui.PanelBorder
	if m.Focused {
		borderStyle = tui.PanelBorderActive
	}

	return borderStyle.Width(m.Width).Height(m.Height).Render(b.String())
}
