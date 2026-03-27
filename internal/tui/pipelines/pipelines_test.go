// internal/tui/pipelines/pipelines_test.go
package pipelines

import (
	"testing"
	"time"

	"github.com/steven/manifold/internal/provider"
)

func TestNewModel(t *testing.T) {
	m := New(80, 20)
	if m.cursor != 0 {
		t.Errorf("cursor should start at 0, got %d", m.cursor)
	}
	if m.Width != 80 || m.Height != 20 {
		t.Errorf("unexpected dimensions: %dx%d", m.Width, m.Height)
	}
}

func TestSetPipelines(t *testing.T) {
	m := New(80, 20)
	pipelines := []provider.Pipeline{
		{ID: "1", Ref: "main", Status: provider.StatusSuccess, Commit: "abc1234"},
		{ID: "2", Ref: "feature", Status: provider.StatusRunning, Commit: "def5678"},
	}
	m.SetPipelines(pipelines)
	if len(m.pipelines) != 2 {
		t.Errorf("got %d pipelines, want 2", len(m.pipelines))
	}
}

func TestCursorMovement(t *testing.T) {
	m := New(80, 20)
	m.SetPipelines([]provider.Pipeline{
		{ID: "1", Ref: "main", Status: provider.StatusSuccess},
		{ID: "2", Ref: "dev", Status: provider.StatusRunning},
		{ID: "3", Ref: "fix", Status: provider.StatusFailed},
	})

	m.MoveDown()
	if m.cursor != 1 {
		t.Errorf("cursor after MoveDown: got %d, want 1", m.cursor)
	}

	m.MoveDown()
	if m.cursor != 2 {
		t.Errorf("cursor after second MoveDown: got %d, want 2", m.cursor)
	}

	m.MoveDown() // should not go past end
	if m.cursor != 2 {
		t.Errorf("cursor should stay at 2, got %d", m.cursor)
	}

	m.MoveUp()
	if m.cursor != 1 {
		t.Errorf("cursor after MoveUp: got %d, want 1", m.cursor)
	}

	m.GoToTop()
	if m.cursor != 0 {
		t.Errorf("cursor after GoToTop: got %d, want 0", m.cursor)
	}

	m.GoToBottom()
	if m.cursor != 2 {
		t.Errorf("cursor after GoToBottom: got %d, want 2", m.cursor)
	}
}

func TestSelected(t *testing.T) {
	m := New(80, 20)
	m.SetPipelines([]provider.Pipeline{
		{ID: "1", Ref: "main", Status: provider.StatusSuccess, Duration: 2 * time.Minute},
		{ID: "2", Ref: "dev", Status: provider.StatusRunning},
	})

	p, ok := m.Selected()
	if !ok {
		t.Fatal("should have selection")
	}
	if p.ID != "1" {
		t.Errorf("selected ID: got %q, want %q", p.ID, "1")
	}

	m.MoveDown()
	p, ok = m.Selected()
	if !ok || p.ID != "2" {
		t.Errorf("selected after move: got %q, want %q", p.ID, "2")
	}
}

func TestSelectedEmpty(t *testing.T) {
	m := New(80, 20)
	_, ok := m.Selected()
	if ok {
		t.Error("should not have selection on empty list")
	}
}

func TestCursorClampOnUpdate(t *testing.T) {
	m := New(80, 20)
	m.SetPipelines([]provider.Pipeline{
		{ID: "1"}, {ID: "2"}, {ID: "3"},
	})
	m.GoToBottom() // cursor = 2

	// Reduce list size — cursor should clamp
	m.SetPipelines([]provider.Pipeline{
		{ID: "1"},
	})
	if m.cursor != 0 {
		t.Errorf("cursor should clamp to 0, got %d", m.cursor)
	}
}
