// internal/tui/confirm/confirm.go
package confirm

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/tui/shared"
)

// Model represents a confirmation dialog.
type Model struct {
	Message   string
	Action    string
	Confirmed bool
	Answered  bool
	Width     int
}

// New creates a new confirmation dialog.
func New(message, action string) Model {
	return Model{
		Message: message,
		Action:  action,
		Width:   50,
	}
}

// Confirm marks the dialog as confirmed and answered.
func (m *Model) Confirm() {
	m.Confirmed = true
	m.Answered = true
}

// Deny marks the dialog as denied (not confirmed) and answered.
func (m *Model) Deny() {
	m.Confirmed = false
	m.Answered = true
}

// View renders the confirmation dialog.
func (m Model) View() string {
	prompt := fmt.Sprintf("%s  [y] yes  [n] no", m.Message)
	inner := lipgloss.NewStyle().
		Foreground(shared.ColorWhite).
		Render(prompt)

	return shared.PanelBorderActive.
		Width(m.Width).
		Render(shared.PanelTitle.Render(fmt.Sprintf("Confirm: %s", m.Action)) + "\n\n" + inner)
}
