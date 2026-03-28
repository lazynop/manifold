package gitlab_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/steven/manifold/internal/provider"
	"github.com/steven/manifold/internal/provider/gitlab"
)

func TestListPipelines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v4/projects/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("PRIVATE-TOKEN") != "testtoken" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("PRIVATE-TOKEN"))
		}
		resp := []map[string]interface{}{
			{
				"id":         101,
				"ref":        "main",
				"sha":        "abcdef1234567890",
				"status":     "success",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:05:00Z",
				"web_url":    "https://gitlab.com/owner/repo/-/pipelines/101",
				"user":       map[string]interface{}{"name": "Alice"},
			},
			{
				"id":         102,
				"ref":        "feature",
				"sha":        "deadbeef12345678",
				"status":     "running",
				"created_at": "2024-01-02T00:00:00Z",
				"updated_at": "2024-01-02T00:01:00Z",
				"web_url":    "https://gitlab.com/owner/repo/-/pipelines/102",
				"user":       map[string]interface{}{"name": "Bob"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	g := gitlab.New("testtoken", "owner", "repo", srv.URL)
	pipelines, err := g.ListPipelines(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListPipelines error: %v", err)
	}
	if len(pipelines) != 2 {
		t.Fatalf("expected 2 pipelines, got %d", len(pipelines))
	}

	p := pipelines[0]
	if p.ID != "101" {
		t.Errorf("expected ID '101', got %q", p.ID)
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
	if p.Author != "Alice" {
		t.Errorf("expected Author 'Alice', got %q", p.Author)
	}

	p2 := pipelines[1]
	if p2.Status != provider.StatusRunning {
		t.Errorf("expected StatusRunning, got %q", p2.Status)
	}
}

func TestGetJobs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := []map[string]interface{}{
			{
				"id":          201,
				"name":        "build",
				"status":      "success",
				"started_at":  "2024-01-01T00:00:00Z",
				"finished_at": "2024-01-01T00:03:00Z",
				"web_url":     "https://gitlab.com/owner/repo/-/jobs/201",
			},
			{
				"id":          202,
				"name":        "test",
				"status":      "failed",
				"started_at":  "2024-01-01T00:03:00Z",
				"finished_at": "2024-01-01T00:06:00Z",
				"web_url":     "https://gitlab.com/owner/repo/-/jobs/202",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	g := gitlab.New("testtoken", "owner", "repo", srv.URL)
	jobs, err := g.GetJobs(context.Background(), "42")
	if err != nil {
		t.Fatalf("GetJobs error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].ID != "201" {
		t.Errorf("expected ID '201', got %q", jobs[0].ID)
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
	statuses := []struct {
		input string
		want  provider.PipelineStatus
	}{
		{"created", provider.StatusPending},
		{"preparing", provider.StatusPending},
		{"pending", provider.StatusPending},
		{"manual", provider.StatusPending},
		{"waiting_for_resource", provider.StatusQueued},
		{"scheduled", provider.StatusQueued},
		{"running", provider.StatusRunning},
		{"success", provider.StatusSuccess},
		{"failed", provider.StatusFailed},
		{"canceled", provider.StatusCanceled},
		{"skipped", provider.StatusSkipped},
	}

	for _, tt := range statuses {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := []map[string]interface{}{
				{
					"id":         1,
					"ref":        "main",
					"sha":        "abc1234",
					"status":     tt.input,
					"created_at": "2024-01-01T00:00:00Z",
					"updated_at": "2024-01-01T00:00:00Z",
					"web_url":    "https://gitlab.com/owner/repo/-/pipelines/1",
					"user":       map[string]interface{}{"name": "user"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))

		g := gitlab.New("tok", "owner", "repo", srv.URL)
		pipelines, err := g.ListPipelines(context.Background(), 1)
		srv.Close()
		if err != nil {
			t.Errorf("status=%s: error %v", tt.input, err)
			continue
		}
		if len(pipelines) != 1 {
			t.Errorf("status=%s: expected 1 pipeline", tt.input)
			continue
		}
		if pipelines[0].Status != tt.want {
			t.Errorf("status=%s: got %q, want %q", tt.input, pipelines[0].Status, tt.want)
		}
	}
}

func TestGetStepsFromTrace(t *testing.T) {
	trace := "section_start:1700000000:build_step\r\x1b[0K\n" +
		"Building...\n" +
		"section_end:1700000030:build_step\r\x1b[0K\n" +
		"section_start:1700000030:test_step\r\x1b[0K\n" +
		"Testing...\n"
	// test_step has no section_end — it's still running

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(trace))
	}))
	defer srv.Close()

	g := gitlab.New("tok", "owner", "repo", srv.URL)
	steps, err := g.GetSteps(context.Background(), "42")
	if err != nil {
		t.Fatalf("GetSteps error: %v", err)
	}

	// Should have 2 steps: build (completed) and test (running/unclosed)
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}

	var buildStep, testStep *provider.Step
	for i := range steps {
		switch steps[i].Name {
		case "build_step":
			buildStep = &steps[i]
		case "test_step":
			testStep = &steps[i]
		}
	}

	if buildStep == nil {
		t.Fatal("missing build_step")
	}
	if buildStep.Status != provider.StatusSuccess {
		t.Errorf("build_step: expected StatusSuccess, got %q", buildStep.Status)
	}
	if buildStep.Duration != 30_000_000_000 { // 30 seconds
		t.Errorf("build_step: expected 30s duration, got %v", buildStep.Duration)
	}

	if testStep == nil {
		t.Fatal("missing test_step")
	}
	if testStep.Status != provider.StatusRunning {
		t.Errorf("test_step: expected StatusRunning, got %q", testStep.Status)
	}
}

func TestName(t *testing.T) {
	g := gitlab.New("tok", "owner", "repo", "")
	if g.Name() != "gitlab" {
		t.Errorf("expected 'gitlab', got %q", g.Name())
	}
}
