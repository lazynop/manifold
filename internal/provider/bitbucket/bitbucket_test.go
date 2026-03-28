package bitbucket_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/steven/manifold/internal/provider"
	"github.com/steven/manifold/internal/provider/bitbucket"
)

func TestListPipelines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/2.0/repositories/owner/repo/pipelines/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// Basic auth check
		user, pass, ok := r.BasicAuth()
		if !ok || user != "testuser" || pass != "testtoken" {
			t.Errorf("unexpected auth: user=%q pass=%q ok=%v", user, pass, ok)
		}

		resp := map[string]interface{}{
			"values": []map[string]interface{}{
				{
					"uuid":        "{aaaa-1111}",
					"target":      map[string]interface{}{"ref_name": "main", "commit": map[string]interface{}{"hash": "abcdef1234567890", "message": "Fix bug", "author": map[string]interface{}{"raw": "Alice <alice@example.com>"}}},
					"state":       map[string]interface{}{"name": "COMPLETED", "result": map[string]interface{}{"name": "SUCCESSFUL"}},
					"created_on":  "2024-01-01T00:00:00Z",
					"completed_on": "2024-01-01T00:05:00Z",
					"links":       map[string]interface{}{"html": map[string]interface{}{"href": "https://bitbucket.org/owner/repo/pipelines/aaaa-1111"}},
				},
				{
					"uuid":        "{bbbb-2222}",
					"target":      map[string]interface{}{"ref_name": "feature", "commit": map[string]interface{}{"hash": "deadbeef12345678", "message": "Add feature", "author": map[string]interface{}{"raw": "Bob <bob@example.com>"}}},
					"state":       map[string]interface{}{"name": "IN_PROGRESS"},
					"created_on":  "2024-01-02T00:00:00Z",
					"completed_on": nil,
					"links":       map[string]interface{}{"html": map[string]interface{}{"href": "https://bitbucket.org/owner/repo/pipelines/bbbb-2222"}},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	b := bitbucket.New("testtoken", "testuser", "owner", "repo", srv.URL)
	pipelines, err := b.ListPipelines(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListPipelines error: %v", err)
	}
	if len(pipelines) != 2 {
		t.Fatalf("expected 2 pipelines, got %d", len(pipelines))
	}

	p := pipelines[0]
	// UUID braces should be trimmed
	if p.ID != "aaaa-1111" {
		t.Errorf("expected ID 'aaaa-1111', got %q", p.ID)
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

	p2 := pipelines[1]
	if p2.Status != provider.StatusRunning {
		t.Errorf("expected StatusRunning, got %q", p2.Status)
	}
}

func TestStatusMapping(t *testing.T) {
	tests := []struct {
		state  string
		result string
		want   provider.PipelineStatus
	}{
		{"PENDING", "", provider.StatusPending},
		{"IN_PROGRESS", "", provider.StatusRunning},
		{"COMPLETED", "SUCCESSFUL", provider.StatusSuccess},
		{"COMPLETED", "FAILED", provider.StatusFailed},
		{"COMPLETED", "STOPPED", provider.StatusCanceled},
		{"HALTED", "", provider.StatusCanceled},
	}

	for _, tt := range tests {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			state := map[string]interface{}{"name": tt.state}
			if tt.result != "" {
				state["result"] = map[string]interface{}{"name": tt.result}
			}
			resp := map[string]interface{}{
				"values": []map[string]interface{}{
					{
						"uuid":       "{test-uuid}",
						"target":     map[string]interface{}{"ref_name": "main", "commit": map[string]interface{}{"hash": "abc1234", "message": "test", "author": map[string]interface{}{"raw": "user"}}},
						"state":      state,
						"created_on": "2024-01-01T00:00:00Z",
						"links":      map[string]interface{}{"html": map[string]interface{}{"href": "https://bitbucket.org"}},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))

		b := bitbucket.New("tok", "user", "owner", "repo", srv.URL)
		pipelines, err := b.ListPipelines(context.Background(), 1)
		srv.Close()
		if err != nil {
			t.Errorf("state=%s result=%s: error %v", tt.state, tt.result, err)
			continue
		}
		if len(pipelines) != 1 {
			t.Errorf("state=%s result=%s: expected 1 pipeline", tt.state, tt.result)
			continue
		}
		if pipelines[0].Status != tt.want {
			t.Errorf("state=%s result=%s: got %q, want %q",
				tt.state, tt.result, pipelines[0].Status, tt.want)
		}
	}
}

func TestName(t *testing.T) {
	b := bitbucket.New("tok", "user", "owner", "repo", "")
	if b.Name() != "bitbucket" {
		t.Errorf("expected 'bitbucket', got %q", b.Name())
	}
}

func TestRetryNotSupported(t *testing.T) {
	b := bitbucket.New("tok", "user", "owner", "repo", "")
	if err := b.RetryPipeline(context.Background(), "123"); err != provider.ErrNotSupported {
		t.Errorf("RetryPipeline: expected ErrNotSupported, got %v", err)
	}
	if err := b.RetryJob(context.Background(), "123"); err != provider.ErrNotSupported {
		t.Errorf("RetryJob: expected ErrNotSupported, got %v", err)
	}
	if err := b.CancelJob(context.Background(), "123"); err != provider.ErrNotSupported {
		t.Errorf("CancelJob: expected ErrNotSupported, got %v", err)
	}
}

func TestBearerAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer beareronlytoken" {
			t.Errorf("unexpected auth header: %q", auth)
		}
		resp := map[string]interface{}{"values": []interface{}{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// No username — should use Bearer token
	b := bitbucket.New("beareronlytoken", "", "owner", "repo", srv.URL)
	_, err := b.ListPipelines(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListPipelines error: %v", err)
	}
}

func TestGetJobsCompositeID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"values": []map[string]interface{}{
				{
					"uuid":         "{step-uuid-1}",
					"name":         "Build",
					"state":        map[string]interface{}{"name": "COMPLETED", "result": map[string]interface{}{"name": "SUCCESSFUL"}},
					"started_on":   "2024-01-01T00:00:00Z",
					"completed_on": "2024-01-01T00:02:00Z",
				},
				{
					"uuid":         "{step-uuid-2}",
					"name":         "Test",
					"state":        map[string]interface{}{"name": "IN_PROGRESS", "result": map[string]interface{}{"name": ""}},
					"started_on":   "2024-01-01T00:02:00Z",
					"completed_on": "",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	b := bitbucket.New("tok", "", "owner", "repo", srv.URL)
	jobs, err := b.GetJobs(context.Background(), "pipeline-123")
	if err != nil {
		t.Fatalf("GetJobs error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Job ID must be composite "pipelineID/stepID" for GetLog to work
	if jobs[0].ID != "pipeline-123/step-uuid-1" {
		t.Errorf("expected composite ID 'pipeline-123/step-uuid-1', got %q", jobs[0].ID)
	}
	if jobs[1].ID != "pipeline-123/step-uuid-2" {
		t.Errorf("expected composite ID 'pipeline-123/step-uuid-2', got %q", jobs[1].ID)
	}

	// WebURL should be populated
	if jobs[0].WebURL == "" {
		t.Error("expected non-empty WebURL for job")
	}
	if !strings.Contains(jobs[0].WebURL, "pipeline-123") {
		t.Errorf("WebURL should contain pipelineID, got %q", jobs[0].WebURL)
	}

	// Status mapping
	if jobs[0].Status != provider.StatusSuccess {
		t.Errorf("expected StatusSuccess, got %q", jobs[0].Status)
	}
	if jobs[1].Status != provider.StatusRunning {
		t.Errorf("expected StatusRunning, got %q", jobs[1].Status)
	}
}

func TestGetStepsEmpty(t *testing.T) {
	b := bitbucket.New("tok", "user", "owner", "repo", "")
	steps, err := b.GetSteps(context.Background(), "123")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if len(steps) != 0 {
		t.Errorf("expected empty steps, got %v", steps)
	}
}
