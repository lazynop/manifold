package provider

import "time"

// PipelineStatus represents the normalized status of a pipeline, job, or step.
type PipelineStatus string

const (
	StatusPending  PipelineStatus = "pending"
	StatusRunning  PipelineStatus = "running"
	StatusSuccess  PipelineStatus = "success"
	StatusFailed   PipelineStatus = "failed"
	StatusCanceled PipelineStatus = "canceled"
	StatusQueued   PipelineStatus = "queued"
	StatusSkipped  PipelineStatus = "skipped"
)

// IsTerminal returns true if the status represents a final state.
func (s PipelineStatus) IsTerminal() bool {
	switch s {
	case StatusSuccess, StatusFailed, StatusCanceled, StatusSkipped:
		return true
	}
	return false
}

// Pipeline represents a CI/CD pipeline run, normalized across providers.
type Pipeline struct {
	ID        string
	Ref       string         // branch or tag
	Commit    string         // short SHA
	Message   string         // commit message
	Author    string
	Status    PipelineStatus
	StartedAt time.Time
	Duration  time.Duration
	WebURL    string
	Jobs      []Job
}

// HasRunningJobs returns true if any job in the pipeline is currently running.
func (p Pipeline) HasRunningJobs() bool {
	for _, j := range p.Jobs {
		if j.Status == StatusRunning {
			return true
		}
	}
	return false
}

// Job represents a single job within a pipeline.
type Job struct {
	ID        string
	Name      string
	Status    PipelineStatus
	StartedAt time.Time
	Duration  time.Duration
	WebURL    string
	Steps     []Step
}

// Step represents a single step within a job.
type Step struct {
	Name     string
	Status   PipelineStatus
	Duration time.Duration
	LogStart int // line offset where this step begins in job log
	LogEnd   int // line offset where this step ends in job log
}
