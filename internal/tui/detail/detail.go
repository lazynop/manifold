// internal/tui/detail/detail.go
package detail

import (
	"fmt"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/provider"
	"github.com/steven/manifold/internal/tui/shared"
)

// Matches ISO 8601 timestamps at the start of log lines (GitHub Actions format)
var logTimestampRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T(\d{2}:\d{2}:\d{2})\.\d+Z)\s*`)

var timestampStyle = lipgloss.NewStyle().Foreground(shared.ColorDarkGray)

const defaultMaxLogLines = 10000

type Model struct {
	job             provider.Job
	logLines        []string
	logOffset       int
	remoteLogOffset int
	autoScroll      bool
	maxLogLines     int
	Width           int
	Height          int
	Focused         bool
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

func (m *Model) SetJob(j provider.Job) {
	m.job = j
	m.ClearLog()
}

func (m Model) HasJob() bool {
	return m.job.ID != ""
}

// Job returns the current job.
func (m Model) Job() provider.Job {
	return m.job
}

func (m *Model) AppendLog(text string) {
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	m.logLines = append(m.logLines, lines...)

	if len(m.logLines) > m.maxLogLines {
		overflow := len(m.logLines) - m.maxLogLines
		m.logLines = m.logLines[overflow:]
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

func (m *Model) ClearLog() {
	m.logLines = nil
	m.logOffset = 0
	m.remoteLogOffset = 0
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

// RemoteLogOffset returns the byte offset used for fetching the next chunk of log from the provider.
func (m Model) RemoteLogOffset() int {
	return m.remoteLogOffset
}

// SetRemoteLogOffset sets the byte offset for the next remote log fetch.
func (m *Model) SetRemoteLogOffset(offset int) {
	m.remoteLogOffset = offset
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
	// Height - 2 (border) - 1 (title) - len(steps) - 1 (separator)
	available := m.Height - 4 - len(m.job.Steps)
	if available < 1 {
		return 1
	}
	return available
}

// formatLogLine colorizes timestamps at the start of log lines.
func formatLogLine(line string, maxWidth int) string {
	if len(line) > maxWidth {
		line = line[:maxWidth]
	}
	if m := logTimestampRe.FindStringIndex(line); m != nil {
		ts := line[:m[1]]
		rest := line[m[1]:]
		return timestampStyle.Render(ts) + shared.NormalItem.Render(rest)
	}
	return shared.NormalItem.Render(line)
}

// contentWidth returns the usable width inside the border.
func (m Model) contentWidth() int {
	// Border takes 2 chars (left + right)
	w := m.Width - 2
	if w < 1 {
		return 1
	}
	return w
}

// View renders the detail panel with steps and log output.
func (m Model) View() string {
	var b strings.Builder

	if !m.HasJob() {
		borderStyle := shared.PanelBorder
		if m.Focused {
			borderStyle = shared.PanelBorderActive
		}
		placeholder := shared.NormalItem.Render("  Select a job to view details")
		return borderStyle.Width(m.Width).Height(m.Height).Render(
			shared.PanelTitle.Render("Detail") + "\n" + placeholder,
		)
	}

	cw := m.contentWidth()

	// Steps section
	title := fmt.Sprintf("Job: %s", m.job.Name)
	if len(title) > cw {
		title = title[:cw]
	}
	b.WriteString(shared.PanelTitle.Render(title))
	b.WriteString("\n")

	for _, step := range m.job.Steps {
		icon := shared.StatusIcon(string(step.Status))
		color := shared.StatusColor(string(step.Status))
		iconStyled := lipgloss.NewStyle().Foreground(color).Render(icon)
		name := step.Name
		if len(name) > cw-4 {
			name = name[:cw-4]
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", iconStyled, name))
	}

	// Separator
	b.WriteString(strings.Repeat("─", max(0, cw)))
	b.WriteString("\n")

	// Log section
	logHeight := m.logViewHeight()
	end := m.logOffset + logHeight
	if end > len(m.logLines) {
		end = len(m.logLines)
	}

	visible := m.logLines[m.logOffset:end]
	for i, line := range visible {
		line = formatLogLine(line, cw)
		b.WriteString(line)
		if i < len(visible)-1 {
			b.WriteString("\n")
		}
	}

	borderStyle := shared.PanelBorder
	if m.Focused {
		borderStyle = shared.PanelBorderActive
	}

	return borderStyle.Width(m.Width).Height(m.Height).Render(b.String())
}
