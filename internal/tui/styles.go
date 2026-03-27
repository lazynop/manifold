// internal/tui/styles.go
package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Status icons — unified across all providers.
const (
	IconQueued   = "◷"
	IconPending  = "○"
	IconRunning  = "●"
	IconSuccess  = "✓"
	IconFailed   = "✗"
	IconCanceled = "⊘"
	IconSkipped  = "–"
)

// Colors
var (
	ColorBlue     = lipgloss.Color("#5B9BD5")
	ColorGray     = lipgloss.Color("#808080")
	ColorYellow   = lipgloss.Color("#E5C07B")
	ColorGreen    = lipgloss.Color("#98C379")
	ColorRed      = lipgloss.Color("#E06C75")
	ColorDarkGray = lipgloss.Color("#5C6370")
	ColorWhite    = lipgloss.Color("#ABB2BF")
	ColorAccent   = lipgloss.Color("#61AFEF")
)

// Panel styles
var (
	PanelBorder = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorGray)

	PanelBorderActive = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(ColorAccent)

	PanelTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorGray)

	SelectedItem = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent)

	NormalItem = lipgloss.NewStyle().
			Foreground(ColorWhite)
)

// StatusIcon returns the icon for a given pipeline status.
func StatusIcon(status string) string {
	switch status {
	case "queued":
		return IconQueued
	case "pending":
		return IconPending
	case "running":
		return IconRunning
	case "success":
		return IconSuccess
	case "failed":
		return IconFailed
	case "canceled":
		return IconCanceled
	case "skipped":
		return IconSkipped
	default:
		return "?"
	}
}

// StatusColor returns the color for a given pipeline status.
func StatusColor(status string) color.Color {
	switch status {
	case "queued":
		return ColorBlue
	case "pending":
		return ColorGray
	case "running":
		return ColorYellow
	case "success":
		return ColorGreen
	case "failed":
		return ColorRed
	case "canceled":
		return ColorGray
	case "skipped":
		return ColorDarkGray
	default:
		return ColorWhite
	}
}
