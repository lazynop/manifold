package jobs

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
}

func TestSetJobs(t *testing.T) {
	m := New(80, 20)
	jobs := []provider.Job{
		{ID: "1", Name: "build", Status: provider.StatusSuccess, Duration: 30 * time.Second},
		{ID: "2", Name: "test", Status: provider.StatusRunning, Duration: 45 * time.Second},
		{ID: "3", Name: "deploy", Status: provider.StatusPending},
	}
	m.SetJobs(jobs)
	if len(m.jobs) != 3 {
		t.Errorf("got %d jobs, want 3", len(m.jobs))
	}
}

func TestCursorMovement(t *testing.T) {
	m := New(80, 20)
	m.SetJobs([]provider.Job{
		{ID: "1", Name: "build"}, {ID: "2", Name: "test"}, {ID: "3", Name: "deploy"},
	})
	m.MoveDown()
	if m.cursor != 1 {
		t.Errorf("cursor: got %d, want 1", m.cursor)
	}
	m.MoveDown()
	m.MoveDown() // should clamp
	if m.cursor != 2 {
		t.Errorf("cursor: got %d, want 2", m.cursor)
	}
	m.MoveUp()
	if m.cursor != 1 {
		t.Errorf("cursor: got %d, want 1", m.cursor)
	}
	m.GoToTop()
	if m.cursor != 0 {
		t.Errorf("cursor: got %d, want 0", m.cursor)
	}
	m.GoToBottom()
	if m.cursor != 2 {
		t.Errorf("cursor: got %d, want 2", m.cursor)
	}
}

func TestSelected(t *testing.T) {
	m := New(80, 20)
	m.SetJobs([]provider.Job{{ID: "1", Name: "build"}, {ID: "2", Name: "test"}})
	j, ok := m.Selected()
	if !ok || j.ID != "1" {
		t.Errorf("selected: got %q, want %q", j.ID, "1")
	}
}

func TestSelectedEmpty(t *testing.T) {
	m := New(80, 20)
	_, ok := m.Selected()
	if ok {
		t.Error("should not have selection on empty list")
	}
}

func TestClear(t *testing.T) {
	m := New(80, 20)
	m.SetJobs([]provider.Job{{ID: "1"}})
	m.Clear()
	if len(m.jobs) != 0 {
		t.Error("clear should empty the list")
	}
	if m.cursor != 0 {
		t.Error("clear should reset cursor")
	}
}
