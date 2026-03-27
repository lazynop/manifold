package provider

import (
	"context"
	"errors"
)

// ErrNotSupported is returned when a provider does not support an operation.
var ErrNotSupported = errors.New("operation not supported by this provider")

// Provider defines the interface that all CI/CD providers must implement.
type Provider interface {
	// Name returns the provider identifier (github, gitlab, bitbucket).
	Name() string

	// ListPipelines returns the most recent pipelines, up to limit.
	ListPipelines(ctx context.Context, limit int) ([]Pipeline, error)

	// GetJobs returns jobs for a pipeline. Steps slices are empty — use GetSteps.
	GetJobs(ctx context.Context, pipelineID string) ([]Job, error)

	// GetSteps returns steps for a specific job.
	GetSteps(ctx context.Context, jobID string) ([]Step, error)

	// GetLog returns log output for a job starting from offset.
	// Returns the new content and the next offset for incremental polling.
	GetLog(ctx context.Context, jobID string, offset int) (content string, newOffset int, err error)

	// RetryPipeline re-runs an entire pipeline.
	RetryPipeline(ctx context.Context, pipelineID string) error

	// RetryJob re-runs a single job (returns ErrNotSupported if not available).
	RetryJob(ctx context.Context, jobID string) error

	// CancelPipeline stops a running pipeline.
	CancelPipeline(ctx context.Context, pipelineID string) error

	// CancelJob stops a single running job (returns ErrNotSupported if not available).
	CancelJob(ctx context.Context, jobID string) error
}
