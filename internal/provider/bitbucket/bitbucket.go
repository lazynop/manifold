package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/steven/manifold/internal/provider"
)

const defaultBaseURL = "https://api.bitbucket.org"

// Bitbucket implements provider.Provider for Bitbucket Pipelines.
type Bitbucket struct {
	token    string
	username string
	owner    string
	repo     string
	baseURL  string
	client   http.Client
}

// New creates a new Bitbucket provider. If baseURL is empty it defaults to
// the public Bitbucket API URL. If username is set, HTTP Basic Auth is used;
// otherwise Bearer token auth is used.
func New(token, username, owner, repo, baseURL string) *Bitbucket {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Bitbucket{
		token:    token,
		username: username,
		owner:    owner,
		repo:     repo,
		baseURL:  baseURL,
	}
}

// Name returns the provider identifier.
func (b *Bitbucket) Name() string {
	return "bitbucket"
}

// webHost returns the browser-facing host URL, derived from the API baseURL.
func (b *Bitbucket) webHost() string {
	host := strings.Replace(b.baseURL, "api.bitbucket.org", "bitbucket.org", 1)
	host = strings.TrimSuffix(host, "/")
	return host
}

// setAuth applies the appropriate authentication to the request.
func (b *Bitbucket) setAuth(req *http.Request) {
	if b.username != "" {
		req.SetBasicAuth(b.username, b.token)
	} else {
		req.Header.Set("Authorization", "Bearer "+b.token)
	}
}

// doJSON performs an authenticated HTTP request and decodes the JSON response.
func (b *Bitbucket) doJSON(ctx context.Context, method, url string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return err
	}
	b.setAuth(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bitbucket API error %d: %s", resp.StatusCode, string(body))
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// trimUUID removes the surrounding braces from a Bitbucket UUID like {abc-123}.
func trimUUID(s string) string {
	return strings.Trim(s, "{}")
}

// mapStatus converts Bitbucket state/result strings to provider.PipelineStatus.
func mapStatus(state, result string) provider.PipelineStatus {
	switch state {
	case "PENDING":
		return provider.StatusPending
	case "IN_PROGRESS":
		return provider.StatusRunning
	case "COMPLETED":
		switch result {
		case "SUCCESSFUL":
			return provider.StatusSuccess
		case "FAILED":
			return provider.StatusFailed
		case "STOPPED":
			return provider.StatusCanceled
		default:
			return provider.StatusFailed
		}
	case "HALTED":
		return provider.StatusCanceled
	default:
		return provider.StatusPending
	}
}

// --- API response types ---

type commitInfo struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
	Author  struct {
		Raw string `json:"raw"`
	} `json:"author"`
}

type pipelineTarget struct {
	RefName string     `json:"ref_name"`
	Commit  commitInfo `json:"commit"`
}

type stateResult struct {
	Name string `json:"name"`
}

type pipelineState struct {
	Name   string      `json:"name"`
	Result stateResult `json:"result"`
}

type pipelineLinks struct {
	HTML struct {
		Href string `json:"href"`
	} `json:"html"`
}

type pipelineResponse struct {
	UUID        string         `json:"uuid"`
	Target      pipelineTarget `json:"target"`
	State       pipelineState  `json:"state"`
	CreatedOn   string         `json:"created_on"`
	CompletedOn string         `json:"completed_on"`
	Links       pipelineLinks  `json:"links"`
}

type pipelinesPage struct {
	Values []pipelineResponse `json:"values"`
}

type stepState struct {
	Name   string      `json:"name"`
	Result stateResult `json:"result"`
}

type stepResponse struct {
	UUID      string    `json:"uuid"`
	Name      string    `json:"name"`
	State     stepState `json:"state"`
	StartedOn string    `json:"started_on"`
	CompletedOn string  `json:"completed_on"`
}

type stepsPage struct {
	Values []stepResponse `json:"values"`
}

// --- Provider methods ---

// ListPipelines returns the most recent pipelines, up to limit.
func (b *Bitbucket) ListPipelines(ctx context.Context, limit int) ([]provider.Pipeline, error) {
	url := fmt.Sprintf("%s/2.0/repositories/%s/%s/pipelines/?pagelen=%d&sort=-created_on",
		b.baseURL, b.owner, b.repo, limit)

	var page pipelinesPage
	if err := b.doJSON(ctx, http.MethodGet, url, &page); err != nil {
		return nil, err
	}

	pipelines := make([]provider.Pipeline, 0, len(page.Values))
	for _, p := range page.Values {
		commit := p.Target.Commit.Hash
		if len(commit) > 7 {
			commit = commit[:7]
		}

		startedAt, _ := time.Parse(time.RFC3339, p.CreatedOn)
		completedAt, _ := time.Parse(time.RFC3339, p.CompletedOn)

		var duration time.Duration
		if !startedAt.IsZero() && !completedAt.IsZero() {
			duration = completedAt.Sub(startedAt)
		}

		// Extract author name from "Name <email>" format
		author := p.Target.Commit.Author.Raw
		if i := strings.Index(author, " <"); i >= 0 {
			author = author[:i]
		}

		pipelines = append(pipelines, provider.Pipeline{
			ID:        trimUUID(p.UUID),
			Ref:       p.Target.RefName,
			Commit:    commit,
			Message:   p.Target.Commit.Message,
			Author:    author,
			Status:    mapStatus(p.State.Name, p.State.Result.Name),
			StartedAt: startedAt,
			Duration:  duration,
			WebURL:    p.Links.HTML.Href,
		})
	}
	return pipelines, nil
}

// GetJobs returns jobs (Bitbucket "steps") for a pipeline.
// Job IDs use composite format "pipelineID/stepID" because GetLog needs both.
func (b *Bitbucket) GetJobs(ctx context.Context, pipelineID string) ([]provider.Job, error) {
	url := fmt.Sprintf("%s/2.0/repositories/%s/%s/pipelines/%s/steps/",
		b.baseURL, b.owner, b.repo, pipelineID)

	var page stepsPage
	if err := b.doJSON(ctx, http.MethodGet, url, &page); err != nil {
		return nil, err
	}

	jobs := make([]provider.Job, 0, len(page.Values))
	for _, s := range page.Values {
		startedAt, _ := time.Parse(time.RFC3339, s.StartedOn)
		completedAt, _ := time.Parse(time.RFC3339, s.CompletedOn)

		var duration time.Duration
		if !startedAt.IsZero() && !completedAt.IsZero() {
			duration = completedAt.Sub(startedAt)
		}

		stepID := trimUUID(s.UUID)
		jobs = append(jobs, provider.Job{
			ID:        pipelineID + "/" + stepID,
			Name:      s.Name,
			Status:    mapStatus(s.State.Name, s.State.Result.Name),
			StartedAt: startedAt,
			Duration:  duration,
			WebURL:    fmt.Sprintf("%s/%s/%s/pipelines/results/%s/steps/%s", b.webHost(), b.owner, b.repo, pipelineID, stepID),
		})
	}
	return jobs, nil
}

// GetSteps returns empty because Bitbucket steps have no sub-steps.
func (b *Bitbucket) GetSteps(_ context.Context, _ string) ([]provider.Step, error) {
	return []provider.Step{}, nil
}

// GetLog returns the log for a step. jobID must be in "pipelineID/stepID" format.
func (b *Bitbucket) GetLog(ctx context.Context, jobID string, offset int) (string, int, error) {
	parts := strings.SplitN(jobID, "/", 2)
	if len(parts) != 2 {
		return "", offset, fmt.Errorf("bitbucket: jobID must be in 'pipelineID/stepID' format, got %q", jobID)
	}
	pipelineID, stepID := parts[0], parts[1]

	url := fmt.Sprintf("%s/2.0/repositories/%s/%s/pipelines/%s/steps/%s/log",
		b.baseURL, b.owner, b.repo, pipelineID, stepID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", offset, err
	}
	b.setAuth(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return "", offset, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", offset, fmt.Errorf("bitbucket API error %d: %s", resp.StatusCode, string(body))
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

// RetryPipeline returns ErrNotSupported because Bitbucket does not support
// re-running pipelines via API.
func (b *Bitbucket) RetryPipeline(ctx context.Context, pipelineID string) error {
	return provider.ErrNotSupported
}

// RetryJob returns ErrNotSupported.
func (b *Bitbucket) RetryJob(ctx context.Context, jobID string) error {
	return provider.ErrNotSupported
}

// CancelPipeline stops a running pipeline.
func (b *Bitbucket) CancelPipeline(ctx context.Context, pipelineID string) error {
	url := fmt.Sprintf("%s/2.0/repositories/%s/%s/pipelines/%s/stopPipeline",
		b.baseURL, b.owner, b.repo, pipelineID)
	return b.doJSON(ctx, http.MethodPost, url, nil)
}

// CancelJob returns ErrNotSupported.
func (b *Bitbucket) CancelJob(ctx context.Context, jobID string) error {
	return provider.ErrNotSupported
}
