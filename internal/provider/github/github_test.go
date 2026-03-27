package github_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/steven/manifold/internal/provider"
	"github.com/steven/manifold/internal/provider/github"
)

func strPtr(s string) *string {
	return &s
}

func TestListPipelines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/actions/runs" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer testtoken" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		resp := map[string]interface{}{
			"workflow_runs": []map[string]interface{}{
				{
					"id":           1,
					"head_branch":  "main",
					"head_sha":     "abcdef1234567890",
					"display_title": "Fix bug",
					"actor":        map[string]interface{}{"login": "alice"},
					"status":       "completed",
					"conclusion":   "success",
					"created_at":   "2024-01-01T00:00:00Z",
					"updated_at":   "2024-01-01T00:05:00Z",
					"html_url":     "https://github.com/owner/repo/actions/runs/1",
				},
				{
					"id":           2,
					"head_branch":  "feature",
					"head_sha":     "deadbeef12345678",
					"display_title": "Add feature",
					"actor":        map[string]interface{}{"login": "bob"},
					"status":       "in_progress",
					"conclusion":   nil,
					"created_at":   "2024-01-02T00:00:00Z",
					"updated_at":   "2024-01-02T00:01:00Z",
					"html_url":     "https://github.com/owner/repo/actions/runs/2",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	g := github.New("testtoken", "owner", "repo", srv.URL)
	pipelines, err := g.ListPipelines(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListPipelines error: %v", err)
	}
	if len(pipelines) != 2 {
		t.Fatalf("expected 2 pipelines, got %d", len(pipelines))
	}

	p := pipelines[0]
	if p.ID != "1" {
		t.Errorf("expected ID '1', got %q", p.ID)
	}
	if p.Ref != "main" {
		t.Errorf("expected Ref 'main', got %q", p.Ref)
	}
	if p.Commit != "abcdef1" {
		t.Errorf("expected Commit 'abcdef1', got %q", p.Commit)
	}
	if p.Status != provider.StatusSuccess {
		t.Errorf("expected StatusSuccess, got %q", p.Status)
	}
	if p.Author != "alice" {
		t.Errorf("expected Author 'alice', got %q", p.Author)
	}

	p2 := pipelines[1]
	if p2.Status != provider.StatusRunning {
		t.Errorf("expected StatusRunning, got %q", p2.Status)
	}
}

func TestGetJobs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/actions/runs/42/jobs" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := map[string]interface{}{
			"jobs": []map[string]interface{}{
				{
					"id":          101,
					"name":        "build",
					"status":      "completed",
					"conclusion":  "success",
					"started_at":  "2024-01-01T00:00:00Z",
					"completed_at": "2024-01-01T00:03:00Z",
					"html_url":    "https://github.com/owner/repo/actions/runs/42/jobs/101",
				},
				{
					"id":          102,
					"name":        "test",
					"status":      "completed",
					"conclusion":  "failure",
					"started_at":  "2024-01-01T00:03:00Z",
					"completed_at": "2024-01-01T00:06:00Z",
					"html_url":    "https://github.com/owner/repo/actions/runs/42/jobs/102",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	g := github.New("testtoken", "owner", "repo", srv.URL)
	jobs, err := g.GetJobs(context.Background(), "42")
	if err != nil {
		t.Fatalf("GetJobs error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].ID != "101" {
		t.Errorf("expected ID '101', got %q", jobs[0].ID)
	}
	if jobs[0].Name != "build" {
		t.Errorf("expected Name 'build', got %q", jobs[0].Name)
	}
	if jobs[0].Status != provider.StatusSuccess {
		t.Errorf("expected StatusSuccess, got %q", jobs[0].Status)
	}
	if jobs[1].Status != provider.StatusFailed {
		t.Errorf("expected StatusFailed, got %q", jobs[1].Status)
	}
}

func TestStatusMapping(t *testing.T) {
	tests := []struct {
		status     string
		conclusion *string
		want       provider.PipelineStatus
	}{
		{"queued", nil, provider.StatusQueued},
		{"in_progress", nil, provider.StatusRunning},
		{"completed", strPtr("success"), provider.StatusSuccess},
		{"completed", strPtr("failure"), provider.StatusFailed},
		{"completed", strPtr("cancelled"), provider.StatusCanceled},
		{"completed", strPtr("skipped"), provider.StatusSkipped},
		{"waiting", nil, provider.StatusPending},
		{"pending", nil, provider.StatusPending},
	}

	for _, tt := range tests {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			run := map[string]interface{}{
				"id":           1,
				"head_branch":  "main",
				"head_sha":     "abc1234",
				"display_title": "test",
				"actor":        map[string]interface{}{"login": "user"},
				"status":       tt.status,
				"created_at":   "2024-01-01T00:00:00Z",
				"updated_at":   "2024-01-01T00:00:00Z",
				"html_url":     "https://github.com/owner/repo/actions/runs/1",
			}
			if tt.conclusion != nil {
				run["conclusion"] = *tt.conclusion
			} else {
				run["conclusion"] = nil
			}
			resp := map[string]interface{}{
				"workflow_runs": []map[string]interface{}{run},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))

		g := github.New("tok", "owner", "repo", srv.URL)
		pipelines, err := g.ListPipelines(context.Background(), 1)
		srv.Close()
		if err != nil {
			t.Errorf("status=%s conclusion=%v: error %v", tt.status, tt.conclusion, err)
			continue
		}
		if len(pipelines) != 1 {
			t.Errorf("status=%s: expected 1 pipeline", tt.status)
			continue
		}
		if pipelines[0].Status != tt.want {
			t.Errorf("status=%s conclusion=%v: got %q, want %q",
				tt.status, tt.conclusion, pipelines[0].Status, tt.want)
		}
	}
}

func TestName(t *testing.T) {
	g := github.New("tok", "owner", "repo", "")
	if g.Name() != "github" {
		t.Errorf("expected 'github', got %q", g.Name())
	}
}

func TestCancelJobNotSupported(t *testing.T) {
	g := github.New("tok", "owner", "repo", "")
	err := g.CancelJob(context.Background(), "123")
	if err != provider.ErrNotSupported {
		t.Errorf("expected ErrNotSupported, got %v", err)
	}
}
