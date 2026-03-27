// internal/tui/statusbar/statusbar.go
package statusbar

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/tui"
)

var defaultActions = []string{"[↑↓] navigate", "[enter] select", "[r]etry", "[q]uit"}

// Model represents the status bar at the bottom of the TUI.
type Model struct {
	Width        int
	provider     string
	actions      []string
	notification string
	isError      bool
}

// New creates a new status bar with the given width.
func New(width int) Model {
	return Model{
		Width:   width,
		actions: defaultActions,
	}
}

// SetProvider sets the provider/remote info shown on the right.
func (m *Model) SetProvider(p string) {
	m.provider = p
}

// SetActions sets the context-sensitive action hints shown on the left.
func (m *Model) SetActions(actions []string) {
	m.actions = actions
}

// SetNotification sets a temporary notification message.
// If isError is true, the message is styled as an error.
func (m *Model) SetNotification(msg string, isError bool) {
	m.notification = msg
	m.isError = isError
}

// ClearNotification clears the current notification.
func (m *Model) ClearNotification() {
	m.notification = ""
	m.isError = false
}

// View renders the status bar.
func (m Model) View() string {
	leftStyle := tui.StatusBarStyle
	if m.notification != "" && m.isError {
		leftStyle = lipgloss.NewStyle().Foreground(tui.ColorRed)
	} else if m.notification != "" {
		leftStyle = lipgloss.NewStyle().Foreground(tui.ColorYellow)
	}

	var left string
	if m.notification != "" {
		left = leftStyle.Render(m.notification)
	} else {
		left = leftStyle.Render(strings.Join(m.actions, "  "))
	}

	right := tui.StatusBarStyle.Render(m.provider)

	// Pad to fill width
	bar := lipgloss.NewStyle().Width(m.Width).Render(left + "  " + right)
	return bar
}
