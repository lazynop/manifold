package statusbar

import (
	"strings"
	"testing"
)

func TestView(t *testing.T) {
	m := New(80)
	m.SetProvider("github.com/user/repo")
	view := m.View()
	if !strings.Contains(view, "github.com/user/repo") {
		t.Error("should contain provider info")
	}
}

func TestSetNotification(t *testing.T) {
	m := New(80)
	m.SetNotification("Retry failed: 403 Forbidden", true)
	view := m.View()
	if !strings.Contains(view, "Retry failed") {
		t.Error("should contain notification")
	}
}

func TestClearNotification(t *testing.T) {
	m := New(80)
	m.SetNotification("temp msg", false)
	m.ClearNotification()
	if m.notification != "" {
		t.Error("notification should be cleared")
	}
}

func TestContextActions(t *testing.T) {
	m := New(80)
	m.SetActions([]string{"[r]etry", "[c]ancel", "[o]pen"})
	view := m.View()
	if !strings.Contains(view, "[r]etry") {
		t.Error("should contain retry action")
	}
}
