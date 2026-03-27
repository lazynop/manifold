// internal/tui/app_test.go
package tui

import (
	"testing"

	"github.com/steven/manifold/internal/provider"
)

func TestNewApp(t *testing.T) {
	app := NewApp(provider.DetectResult{
		ProviderType: "github",
		Host:         "github.com",
		Owner:        "user",
		Repo:         "repo",
	}, true, 25)

	if app.focusedPanel != PanelPipelines {
		t.Errorf("initial focus: got %d, want %d", app.focusedPanel, PanelPipelines)
	}
	if app.confirmActions != true {
		t.Error("confirmActions should be true")
	}
}

func TestFocusCycle(t *testing.T) {
	app := NewApp(provider.DetectResult{}, true, 25)

	app.FocusNext()
	if app.focusedPanel != PanelJobs {
		t.Errorf("after FocusNext: got %d, want %d", app.focusedPanel, PanelJobs)
	}

	app.FocusNext()
	if app.focusedPanel != PanelDetail {
		t.Errorf("after FocusNext: got %d, want %d", app.focusedPanel, PanelDetail)
	}

	app.FocusNext() // should wrap
	if app.focusedPanel != PanelPipelines {
		t.Errorf("after wrap: got %d, want %d", app.focusedPanel, PanelPipelines)
	}

	app.FocusPrev() // should wrap backwards
	if app.focusedPanel != PanelDetail {
		t.Errorf("after FocusPrev wrap: got %d, want %d", app.focusedPanel, PanelDetail)
	}
}

func TestProviderLabel(t *testing.T) {
	app := NewApp(provider.DetectResult{
		ProviderType: "github",
		Host:         "github.com",
		Owner:        "user",
		Repo:         "myrepo",
	}, true, 25)

	label := app.ProviderLabel()
	if label != "github.com/user/myrepo" {
		t.Errorf("label: got %q", label)
	}
}
