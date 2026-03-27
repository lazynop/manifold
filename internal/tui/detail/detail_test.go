package detail

import (
	"testing"

	"github.com/steven/manifold/internal/provider"
)

func TestNewModel(t *testing.T) {
	m := New(80, 20)
	if m.Width != 80 || m.Height != 20 {
		t.Errorf("unexpected dimensions: %dx%d", m.Width, m.Height)
	}
}

func TestSetJob(t *testing.T) {
	m := New(80, 20)
	j := provider.Job{
		ID:   "123",
		Name: "build",
		Steps: []provider.Step{
			{Name: "checkout", Status: provider.StatusSuccess},
			{Name: "compile", Status: provider.StatusRunning},
		},
	}
	m.SetJob(j)
	if m.job.ID != "123" {
		t.Errorf("job ID: got %q, want %q", m.job.ID, "123")
	}
}

func TestAppendLog(t *testing.T) {
	m := New(80, 20)
	m.AppendLog("line 1\nline 2\n")
	m.AppendLog("line 3\n")
	if m.LogLineCount() != 3 {
		t.Errorf("log lines: got %d, want 3", m.LogLineCount())
	}
}

func TestLogRingBuffer(t *testing.T) {
	m := New(80, 20)
	m.maxLogLines = 5
	for i := 0; i < 10; i++ {
		m.AppendLog("line\n")
	}
	if m.LogLineCount() != 5 {
		t.Errorf("log lines: got %d, want 5 (ring buffer)", m.LogLineCount())
	}
}

func TestClearLog(t *testing.T) {
	m := New(80, 20)
	m.AppendLog("some log\n")
	m.ClearLog()
	if m.LogLineCount() != 0 {
		t.Errorf("log should be empty after clear, got %d", m.LogLineCount())
	}
}

func TestSetJobClearsLog(t *testing.T) {
	m := New(80, 20)
	m.AppendLog("old log\n")
	m.SetJob(provider.Job{ID: "new"})
	if m.LogLineCount() != 0 {
		t.Error("SetJob should clear previous log")
	}
}

func TestAutoScroll(t *testing.T) {
	m := New(80, 20)
	if !m.autoScroll {
		t.Error("autoScroll should default to true")
	}
}

func TestLogOffset(t *testing.T) {
	m := New(80, 20)
	if m.LogOffset() != 0 {
		t.Errorf("initial offset should be 0, got %d", m.LogOffset())
	}
	m.SetLogOffset(42)
	if m.LogOffset() != 42 {
		t.Errorf("offset: got %d, want 42", m.LogOffset())
	}
}
