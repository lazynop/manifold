package provider

import (
	"testing"
	"time"
)

func TestPipelineStatusString(t *testing.T) {
	tests := []struct {
		status PipelineStatus
		want   string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "running"},
		{StatusSuccess, "success"},
		{StatusFailed, "failed"},
		{StatusCanceled, "canceled"},
		{StatusQueued, "queued"},
		{StatusSkipped, "skipped"},
	}
	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("got %q, want %q", tt.status, tt.want)
		}
	}
}

func TestPipelineIsTerminal(t *testing.T) {
	terminal := []PipelineStatus{StatusSuccess, StatusFailed, StatusCanceled, StatusSkipped}
	nonTerminal := []PipelineStatus{StatusPending, StatusRunning, StatusQueued}

	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("%q should be terminal", s)
		}
	}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("%q should not be terminal", s)
		}
	}
}

func TestPipelineHasRunningJobs(t *testing.T) {
	p := Pipeline{
		Jobs: []Job{
			{Name: "build", Status: StatusSuccess},
			{Name: "test", Status: StatusRunning},
		},
	}
	if !p.HasRunningJobs() {
		t.Error("pipeline should have running jobs")
	}

	p2 := Pipeline{
		Jobs: []Job{
			{Name: "build", Status: StatusSuccess},
			{Name: "test", Status: StatusFailed},
		},
	}
	if p2.HasRunningJobs() {
		t.Error("pipeline should not have running jobs")
	}
}

func TestStepLogRange(t *testing.T) {
	s := Step{Name: "build", LogStart: 10, LogEnd: 50}
	if s.LogStart != 10 || s.LogEnd != 50 {
		t.Errorf("unexpected log range: %d-%d", s.LogStart, s.LogEnd)
	}
}

func TestPipelineDuration(t *testing.T) {
	p := Pipeline{
		Duration: 2*time.Minute + 34*time.Second,
	}
	if p.Duration != 154*time.Second {
		t.Errorf("unexpected duration: %v", p.Duration)
	}
}
