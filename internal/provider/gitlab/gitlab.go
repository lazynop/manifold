package gitlab

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/steven/manifold/internal/provider"
)

const defaultBaseURL = "https://gitlab.com"

// GitLab implements provider.Provider for GitLab CI.
type GitLab struct {
	token   string
	owner   string
	repo    string
	baseURL string
	client  http.Client
}

// New creates a new GitLab provider. If baseURL is empty, it defaults to
// the public GitLab URL.
func New(token, owner, repo, baseURL string) *GitLab {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &GitLab{
		token:   token,
		owner:   owner,
		repo:    repo,
		baseURL: baseURL,
	}
}

// Name returns the provider identifier.
func (g *GitLab) Name() string {
	return "gitlab"
}

// projectPath returns the URL-encoded owner/repo project path.
func (g *GitLab) projectPath() string {
	return url.PathEscape(g.owner + "/" + g.repo)
}

// doJSON performs an authenticated HTTP request and decodes the JSON response.
func (g *GitLab) doJSON(ctx context.Context, method, reqURL string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, method, reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", g.token)

	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gitlab API error %d: %s", resp.StatusCode, string(body))
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// doRaw performs an authenticated HTTP request and returns the raw body.
func (g *GitLab) doRaw(ctx context.Context, method, reqURL string, extraHeaders map[string]string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, reqURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("PRIVATE-TOKEN", g.token)
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, resp.StatusCode, fmt.Errorf("gitlab API error %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

// mapStatus converts a GitLab status string to provider.PipelineStatus.
func mapStatus(status string) provider.PipelineStatus {
	switch status {
	case "created", "preparing", "pending", "manual":
		return provider.StatusPending
	case "waiting_for_resource", "scheduled":
		return provider.StatusQueued
	case "running":
		return provider.StatusRunning
	case "success":
		return provider.StatusSuccess
	case "failed":
		return provider.StatusFailed
	case "canceled":
		return provider.StatusCanceled
	case "skipped":
		return provider.StatusSkipped
	default:
		return provider.StatusPending
	}
}

// --- API response types ---

type userInfo struct {
	Name string `json:"name"`
}

type pipelineResponse struct {
	ID        int      `json:"id"`
	Ref       string   `json:"ref"`
	SHA       string   `json:"sha"`
	Status    string   `json:"status"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	HTMLURL   string   `json:"web_url"`
	User      userInfo `json:"user"`
}

type jobAPIResponse struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	WebURL     string `json:"web_url"`
}

// --- Provider methods ---

// ListPipelines returns the most recent pipelines, up to limit.
func (g *GitLab) ListPipelines(ctx context.Context, limit int) ([]provider.Pipeline, error) {
	reqURL := fmt.Sprintf("%s/api/v4/projects/%s/pipelines?per_page=%d",
		g.baseURL, g.projectPath(), limit)

	var result []pipelineResponse
	if err := g.doJSON(ctx, http.MethodGet, reqURL, &result); err != nil {
		return nil, err
	}

	pipelines := make([]provider.Pipeline, 0, len(result))
	for _, p := range result {
		commit := p.SHA
		if len(commit) > 7 {
			commit = commit[:7]
		}

		startedAt, _ := time.Parse(time.RFC3339, p.CreatedAt)
		updatedAt, _ := time.Parse(time.RFC3339, p.UpdatedAt)

		var duration time.Duration
		if !startedAt.IsZero() && !updatedAt.IsZero() {
			duration = updatedAt.Sub(startedAt)
		}

		pipelines = append(pipelines, provider.Pipeline{
			ID:        strconv.Itoa(p.ID),
			Ref:       p.Ref,
			Commit:    commit,
			Author:    p.User.Name,
			Status:    mapStatus(p.Status),
			StartedAt: startedAt,
			Duration:  duration,
			WebURL:    p.HTMLURL,
		})
	}
	return pipelines, nil
}

// GetJobs returns jobs for a pipeline.
func (g *GitLab) GetJobs(ctx context.Context, pipelineID string) ([]provider.Job, error) {
	reqURL := fmt.Sprintf("%s/api/v4/projects/%s/pipelines/%s/jobs",
		g.baseURL, g.projectPath(), pipelineID)

	var result []jobAPIResponse
	if err := g.doJSON(ctx, http.MethodGet, reqURL, &result); err != nil {
		return nil, err
	}

	jobs := make([]provider.Job, 0, len(result))
	for _, j := range result {
		startedAt, _ := time.Parse(time.RFC3339, j.StartedAt)
		finishedAt, _ := time.Parse(time.RFC3339, j.FinishedAt)

		var duration time.Duration
		if !startedAt.IsZero() && !finishedAt.IsZero() {
			duration = finishedAt.Sub(startedAt)
		}

		jobs = append(jobs, provider.Job{
			ID:        strconv.Itoa(j.ID),
			Name:      j.Name,
			Status:    mapStatus(j.Status),
			StartedAt: startedAt,
			Duration:  duration,
			WebURL:    j.WebURL,
		})
	}
	return jobs, nil
}

// GetSteps returns steps parsed from a job's trace using section markers.
func (g *GitLab) GetSteps(ctx context.Context, jobID string) ([]provider.Step, error) {
	reqURL := fmt.Sprintf("%s/api/v4/projects/%s/jobs/%s/trace",
		g.baseURL, g.projectPath(), jobID)

	body, _, err := g.doRaw(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	return parseStepsFromTrace(string(body)), nil
}

// parseStepsFromTrace extracts steps from GitLab trace section markers.
func parseStepsFromTrace(trace string) []provider.Step {
	var steps []provider.Step
	// section_start:timestamp:name\r\033[0K
	// section_end:timestamp:name\r\033[0K
	scanner := bufio.NewScanner(strings.NewReader(trace))
	lineNum := 0
	type openStep struct {
		name      string
		startLine int
		startTime int64
	}
	open := map[string]openStep{}

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++
		// Strip ANSI escape sequences for matching
		clean := stripANSI(line)
		if idx := strings.Index(clean, "section_start:"); idx >= 0 {
			rest := clean[idx+len("section_start:"):]
			parts := strings.SplitN(rest, ":", 2)
			if len(parts) == 2 {
				ts, _ := strconv.ParseInt(parts[0], 10, 64)
				name := strings.TrimRight(parts[1], "\r\n\x1b[0K")
				// Remove trailing control chars
				if i := strings.IndexAny(name, "\r\x1b"); i >= 0 {
					name = name[:i]
				}
				open[name] = openStep{name: name, startLine: lineNum, startTime: ts}
			}
		} else if idx := strings.Index(clean, "section_end:"); idx >= 0 {
			rest := clean[idx+len("section_end:"):]
			parts := strings.SplitN(rest, ":", 2)
			if len(parts) == 2 {
				ts, _ := strconv.ParseInt(parts[0], 10, 64)
				name := strings.TrimRight(parts[1], "\r\n\x1b[0K")
				if i := strings.IndexAny(name, "\r\x1b"); i >= 0 {
					name = name[:i]
				}
				if o, ok := open[name]; ok {
					duration := time.Duration(ts-o.startTime) * time.Second
					steps = append(steps, provider.Step{
						Name:     name,
						Status:   provider.StatusSuccess,
						Duration: duration,
						LogStart: o.startLine,
						LogEnd:   lineNum,
					})
					delete(open, name)
				}
			}
		}
	}
	return steps
}

// stripANSI removes ANSI escape sequences from a string.
func stripANSI(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++ // skip 'm'
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// GetLog returns log output starting from offset bytes.
func (g *GitLab) GetLog(ctx context.Context, jobID string, offset int) (string, int, error) {
	reqURL := fmt.Sprintf("%s/api/v4/projects/%s/jobs/%s/trace",
		g.baseURL, g.projectPath(), jobID)

	headers := map[string]string{
		"Range": fmt.Sprintf("bytes=%d-", offset),
	}

	body, statusCode, err := g.doRaw(ctx, http.MethodGet, reqURL, headers)
	if err != nil {
		// 416 Range Not Satisfiable means no new content
		if statusCode == http.StatusRequestedRangeNotSatisfiable {
			return "", offset, nil
		}
		return "", offset, err
	}

	return string(body), offset + len(body), nil
}

// RetryPipeline re-runs an entire pipeline.
func (g *GitLab) RetryPipeline(ctx context.Context, pipelineID string) error {
	reqURL := fmt.Sprintf("%s/api/v4/projects/%s/pipelines/%s/retry",
		g.baseURL, g.projectPath(), pipelineID)
	return g.doJSON(ctx, http.MethodPost, reqURL, nil)
}

// RetryJob re-runs a single job.
func (g *GitLab) RetryJob(ctx context.Context, jobID string) error {
	reqURL := fmt.Sprintf("%s/api/v4/projects/%s/jobs/%s/retry",
		g.baseURL, g.projectPath(), jobID)
	return g.doJSON(ctx, http.MethodPost, reqURL, nil)
}

// CancelPipeline stops a running pipeline.
func (g *GitLab) CancelPipeline(ctx context.Context, pipelineID string) error {
	reqURL := fmt.Sprintf("%s/api/v4/projects/%s/pipelines/%s/cancel",
		g.baseURL, g.projectPath(), pipelineID)
	return g.doJSON(ctx, http.MethodPost, reqURL, nil)
}

// CancelJob stops a single running job.
func (g *GitLab) CancelJob(ctx context.Context, jobID string) error {
	reqURL := fmt.Sprintf("%s/api/v4/projects/%s/jobs/%s/cancel",
		g.baseURL, g.projectPath(), jobID)
	return g.doJSON(ctx, http.MethodPost, reqURL, nil)
}
