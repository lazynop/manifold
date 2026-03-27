package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/steven/manifold/internal/provider"
)

const defaultBaseURL = "https://api.github.com"

// GitHub implements provider.Provider for GitHub Actions.
type GitHub struct {
	token   string
	owner   string
	repo    string
	baseURL string
	client  http.Client
}

// New creates a new GitHub provider. If baseURL is empty, it defaults to
// the public GitHub API URL.
func New(token, owner, repo, baseURL string) *GitHub {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &GitHub{
		token:   token,
		owner:   owner,
		repo:    repo,
		baseURL: baseURL,
	}
}

// Name returns the provider identifier.
func (g *GitHub) Name() string {
	return "github"
}

// do performs an authenticated HTTP request and decodes the JSON response.
func (g *GitHub) do(ctx context.Context, method, url string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github API error %d: %s", resp.StatusCode, string(body))
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// mapStatus converts GitHub status/conclusion strings to provider.PipelineStatus.
func mapStatus(status string, conclusion *string) provider.PipelineStatus {
	switch status {
	case "queued":
		return provider.StatusQueued
	case "in_progress":
		return provider.StatusRunning
	case "completed":
		if conclusion == nil {
			return provider.StatusSuccess
		}
		switch *conclusion {
		case "success":
			return provider.StatusSuccess
		case "failure":
			return provider.StatusFailed
		case "cancelled":
			return provider.StatusCanceled
		case "skipped":
			return provider.StatusSkipped
		default:
			return provider.StatusFailed
		}
	case "waiting", "pending":
		return provider.StatusPending
	default:
		return provider.StatusPending
	}
}

// --- API response types ---

type actor struct {
	Login string `json:"login"`
}

type workflowRun struct {
	ID           int64   `json:"id"`
	HeadBranch   string  `json:"head_branch"`
	HeadSHA      string  `json:"head_sha"`
	DisplayTitle string  `json:"display_title"`
	Actor        actor   `json:"actor"`
	Status       string  `json:"status"`
	Conclusion   *string `json:"conclusion"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	HTMLURL      string  `json:"html_url"`
}

type workflowRunsResponse struct {
	WorkflowRuns []workflowRun `json:"workflow_runs"`
}

type jobResponse struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	Conclusion  *string `json:"conclusion"`
	StartedAt   string  `json:"started_at"`
	CompletedAt string  `json:"completed_at"`
	HTMLURL     string  `json:"html_url"`
	Steps       []stepResponse `json:"steps"`
}

type jobsResponse struct {
	Jobs []jobResponse `json:"jobs"`
}

type stepResponse struct {
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	Conclusion  *string `json:"conclusion"`
	Number      int     `json:"number"`
	StartedAt   string  `json:"started_at"`
	CompletedAt string  `json:"completed_at"`
}

// --- Provider methods ---

// ListPipelines returns the most recent workflow runs, up to limit.
func (g *GitHub) ListPipelines(ctx context.Context, limit int) ([]provider.Pipeline, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs?per_page=%d",
		g.baseURL, g.owner, g.repo, limit)

	var result workflowRunsResponse
	if err := g.do(ctx, http.MethodGet, url, &result); err != nil {
		return nil, err
	}

	pipelines := make([]provider.Pipeline, 0, len(result.WorkflowRuns))
	for _, run := range result.WorkflowRuns {
		commit := run.HeadSHA
		if len(commit) > 7 {
			commit = commit[:7]
		}

		startedAt, _ := time.Parse(time.RFC3339, run.CreatedAt)
		updatedAt, _ := time.Parse(time.RFC3339, run.UpdatedAt)

		var duration time.Duration
		if !startedAt.IsZero() && !updatedAt.IsZero() {
			duration = updatedAt.Sub(startedAt)
		}

		pipelines = append(pipelines, provider.Pipeline{
			ID:        strconv.FormatInt(run.ID, 10),
			Ref:       run.HeadBranch,
			Commit:    commit,
			Message:   run.DisplayTitle,
			Author:    run.Actor.Login,
			Status:    mapStatus(run.Status, run.Conclusion),
			StartedAt: startedAt,
			Duration:  duration,
			WebURL:    run.HTMLURL,
		})
	}
	return pipelines, nil
}

// GetJobs returns jobs for a pipeline run.
func (g *GitHub) GetJobs(ctx context.Context, pipelineID string) ([]provider.Job, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%s/jobs",
		g.baseURL, g.owner, g.repo, pipelineID)

	var result jobsResponse
	if err := g.do(ctx, http.MethodGet, url, &result); err != nil {
		return nil, err
	}

	jobs := make([]provider.Job, 0, len(result.Jobs))
	for _, j := range result.Jobs {
		startedAt, _ := time.Parse(time.RFC3339, j.StartedAt)
		completedAt, _ := time.Parse(time.RFC3339, j.CompletedAt)

		var duration time.Duration
		if !startedAt.IsZero() && !completedAt.IsZero() {
			duration = completedAt.Sub(startedAt)
		}

		jobs = append(jobs, provider.Job{
			ID:        strconv.FormatInt(j.ID, 10),
			Name:      j.Name,
			Status:    mapStatus(j.Status, j.Conclusion),
			StartedAt: startedAt,
			Duration:  duration,
			WebURL:    j.HTMLURL,
		})
	}
	return jobs, nil
}

// GetSteps returns steps for a specific job.
func (g *GitHub) GetSteps(ctx context.Context, jobID string) ([]provider.Step, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/jobs/%s",
		g.baseURL, g.owner, g.repo, jobID)

	var j jobResponse
	if err := g.do(ctx, http.MethodGet, url, &j); err != nil {
		return nil, err
	}

	steps := make([]provider.Step, 0, len(j.Steps))
	for _, s := range j.Steps {
		startedAt, _ := time.Parse(time.RFC3339, s.StartedAt)
		completedAt, _ := time.Parse(time.RFC3339, s.CompletedAt)

		var duration time.Duration
		if !startedAt.IsZero() && !completedAt.IsZero() {
			duration = completedAt.Sub(startedAt)
		}

		steps = append(steps, provider.Step{
			Name:     s.Name,
			Status:   mapStatus(s.Status, s.Conclusion),
			Duration: duration,
		})
	}
	return steps, nil
}

// GetLog returns log output for a job starting from offset.
func (g *GitHub) GetLog(ctx context.Context, jobID string, offset int) (string, int, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/jobs/%s/logs",
		g.baseURL, g.owner, g.repo, jobID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", offset, err
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", offset, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", offset, fmt.Errorf("github API error %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", offset, err
	}

	if offset >= len(body) {
		return "", offset, nil
	}
	content := string(body[offset:])
	return content, len(body), nil
}

// RetryPipeline re-runs an entire pipeline.
func (g *GitHub) RetryPipeline(ctx context.Context, pipelineID string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%s/rerun",
		g.baseURL, g.owner, g.repo, pipelineID)
	return g.do(ctx, http.MethodPost, url, nil)
}

// RetryJob re-runs a single job.
func (g *GitHub) RetryJob(ctx context.Context, jobID string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/jobs/%s/rerun",
		g.baseURL, g.owner, g.repo, jobID)
	return g.do(ctx, http.MethodPost, url, nil)
}

// CancelPipeline stops a running pipeline.
func (g *GitHub) CancelPipeline(ctx context.Context, pipelineID string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%s/cancel",
		g.baseURL, g.owner, g.repo, pipelineID)
	return g.do(ctx, http.MethodPost, url, nil)
}

// CancelJob returns ErrNotSupported because GitHub Actions does not support
// cancelling individual jobs.
func (g *GitHub) CancelJob(ctx context.Context, jobID string) error {
	return provider.ErrNotSupported
}
