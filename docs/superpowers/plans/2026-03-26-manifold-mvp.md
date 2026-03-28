# Manifold MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a TUI that monitors and controls CI/CD pipelines for GitHub Actions, GitLab CI, and Bitbucket Pipelines from the terminal.

**Architecture:** Provider interface pattern with normalized models. Each CI platform implements a common interface. The TUI (Bubble Tea v2) talks only to the interface, never to provider internals. An adaptive poller manages refresh intervals based on pipeline state.

**Tech Stack:** Go 1.23+, Bubble Tea v2 (`charm.land/bubbletea/v2`), Lipgloss v2 (`charm.land/lipgloss/v2`), Kong (`github.com/alecthomas/kong`), go-toml v2 (`github.com/pelletier/go-toml/v2`)

**Spec:** `docs/superpowers/specs/2026-03-26-manifold-design.md`

**IMPORTANT — Bubble Tea v2 API:**
- `View()` returns `tea.View`, not `string`. Use `tea.NewView(content)`.
- Set `v.AltScreen = true` in View(), not in `NewProgram()`.
- Use `tea.KeyPressMsg` (not `tea.KeyMsg`) for key presses.
- `msg.String()` for key matching. Space is `"space"`, not `" "`.
- Import: `charm.land/bubbletea/v2` (vanity domain).

---

## File Structure

```
manifold/
├── main.go                              # entrypoint, kong parse, wiring
├── go.mod
├── go.sum
├── internal/
│   ├── config/
│   │   ├── config.go                    # Config struct, Load(), defaults
│   │   └── config_test.go
│   ├── provider/
│   │   ├── models.go                    # PipelineStatus, Pipeline, Job, Step
│   │   ├── models_test.go
│   │   ├── provider.go                  # Provider interface, ErrNotSupported
│   │   ├── detector.go                  # Detect() — git remote → provider type
│   │   ├── detector_test.go
│   │   ├── auth.go                      # ResolveAuth() — CLI/env/TOML chain
│   │   ├── auth_test.go
│   │   ├── github/
│   │   │   ├── github.go               # GitHub provider implementation
│   │   │   └── github_test.go
│   │   ├── gitlab/
│   │   │   ├── gitlab.go               # GitLab provider implementation
│   │   │   └── gitlab_test.go
│   │   └── bitbucket/
│   │       ├── bitbucket.go            # Bitbucket provider implementation
│   │       └── bitbucket_test.go
│   ├── poller/
│   │   ├── poller.go                    # Adaptive poller with Bubble Tea integration
│   │   └── poller_test.go
│   └── tui/
│       ├── app.go                       # Root model, panel focus, layout
│       ├── app_test.go
│       ├── keys.go                      # Keybindings
│       ├── styles.go                    # Lipgloss styles, status icons/colors
│       ├── pipelines/
│       │   ├── pipelines.go             # Pipeline list panel
│       │   └── pipelines_test.go
│       ├── jobs/
│       │   ├── jobs.go                  # Jobs list panel
│       │   └── jobs_test.go
│       ├── detail/
│       │   ├── detail.go                # Detail panel (steps + log viewer)
│       │   └── detail_test.go
│       ├── statusbar/
│       │   ├── statusbar.go             # Bottom status bar
│       │   └── statusbar_test.go
│       ├── confirm/
│       │   ├── confirm.go               # Confirmation dialog overlay
│       │   └── confirm_test.go
│       └── selector/
│           ├── selector.go              # Remote selector (pre-TUI)
│           └── selector_test.go
```

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: directory structure

- [ ] **Step 1: Initialize Go module and directories**

```bash
cd /home/steven/work/project-manifold/manifold
go mod init github.com/steven/manifold
mkdir -p internal/{config,provider/{github,gitlab,bitbucket},poller,tui/{pipelines,jobs,detail,statusbar,confirm,selector}}
```

- [ ] **Step 2: Install dependencies**

```bash
cd /home/steven/work/project-manifold/manifold
go get charm.land/bubbletea/v2@latest
go get charm.land/lipgloss/v2@latest
go get github.com/alecthomas/kong@latest
go get github.com/pelletier/go-toml/v2@latest
```

- [ ] **Step 3: Create minimal main.go**

```go
// main.go
package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

var version = "dev"

type CLI struct {
	Dir     string `short:"C" help:"Path to git repository." default:"." type:"path"`
	Remote  string `short:"r" help:"Git remote to use." default:""`
	Version kong.VersionFlag `short:"v" help:"Print version."`
}

func main() {
	var cli CLI
	kong.Parse(&cli,
		kong.Name("manifold"),
		kong.Description("Monitor CI/CD pipelines from the terminal."),
		kong.Vars{"version": version},
	)

	fmt.Fprintf(os.Stderr, "manifold %s — dir=%s remote=%s\n", version, cli.Dir, cli.Remote)
}
```

- [ ] **Step 4: Verify it builds and runs**

Run: `cd /home/steven/work/project-manifold/manifold && go build -o manifold . && ./manifold --help`
Expected: help output showing `-C`, `-r`, `-v` flags

Run: `./manifold --version`
Expected: `dev`

- [ ] **Step 5: Commit**

```bash
git init
git add go.mod go.sum main.go internal/
git commit -m "feat: scaffold project with kong CLI and directory structure"
```

---

### Task 2: Normalized Models

**Files:**
- Create: `internal/provider/models.go`
- Create: `internal/provider/models_test.go`
- Create: `internal/provider/provider.go`

- [ ] **Step 1: Write test for models**

```go
// internal/provider/models_test.go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/provider/ -v`
Expected: FAIL — types not defined

- [ ] **Step 3: Write models implementation**

```go
// internal/provider/models.go
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
```

- [ ] **Step 4: Write provider interface**

```go
// internal/provider/provider.go
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
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/provider/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/provider/models.go internal/provider/models_test.go internal/provider/provider.go
git commit -m "feat: add normalized models and provider interface"
```

---

### Task 3: Configuration

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write config tests**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg := Default()
	if cfg.Defaults.PollingActive != 5 {
		t.Errorf("polling_active: got %d, want 5", cfg.Defaults.PollingActive)
	}
	if cfg.Defaults.PollingIdle != 60 {
		t.Errorf("polling_idle: got %d, want 60", cfg.Defaults.PollingIdle)
	}
	if cfg.Defaults.ConfirmActions != true {
		t.Error("confirm_actions should default to true")
	}
	if cfg.Defaults.PipelineLimit != 25 {
		t.Errorf("pipeline_limit: got %d, want 25", cfg.Defaults.PipelineLimit)
	}
}

func TestLoadFromFile(t *testing.T) {
	content := `
[defaults]
remote = "upstream"
polling_active = 10
polling_idle = 120
confirm_actions = false
pipeline_limit = 50

[[providers]]
type = "github"
token = "ghp_test123"

[[providers]]
type = "gitlab"
host = "gitlab.mycompany.com"
token = "glpat_test456"

[[providers]]
type = "bitbucket"
token = "bb_test789"
username = "myuser"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if cfg.Defaults.Remote != "upstream" {
		t.Errorf("remote: got %q, want %q", cfg.Defaults.Remote, "upstream")
	}
	if cfg.Defaults.PollingActive != 10 {
		t.Errorf("polling_active: got %d, want 10", cfg.Defaults.PollingActive)
	}
	if cfg.Defaults.ConfirmActions != false {
		t.Error("confirm_actions should be false")
	}
	if cfg.Defaults.PipelineLimit != 50 {
		t.Errorf("pipeline_limit: got %d, want 50", cfg.Defaults.PipelineLimit)
	}
	if len(cfg.Providers) != 3 {
		t.Fatalf("providers: got %d, want 3", len(cfg.Providers))
	}

	gh := cfg.Providers[0]
	if gh.Type != "github" || gh.Token != "ghp_test123" {
		t.Errorf("github provider: %+v", gh)
	}

	gl := cfg.Providers[1]
	if gl.Type != "gitlab" || gl.Host != "gitlab.mycompany.com" || gl.Token != "glpat_test456" {
		t.Errorf("gitlab provider: %+v", gl)
	}

	bb := cfg.Providers[2]
	if bb.Type != "bitbucket" || bb.Token != "bb_test789" || bb.Username != "myuser" {
		t.Errorf("bitbucket provider: %+v", bb)
	}
}

func TestLoadFromFileMissing(t *testing.T) {
	cfg, err := LoadFromPath("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("missing file should not error, got: %v", err)
	}
	// Should return defaults
	if cfg.Defaults.PollingActive != 5 {
		t.Errorf("should return defaults, got polling_active=%d", cfg.Defaults.PollingActive)
	}
}

func TestDuplicateProviderError(t *testing.T) {
	content := `
[[providers]]
type = "github"
token = "ghp_one"

[[providers]]
type = "github"
token = "ghp_two"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromPath(path)
	if err == nil {
		t.Fatal("expected error for duplicate providers")
	}
}

func TestFindProviderByHost(t *testing.T) {
	cfg := Config{
		Providers: []ProviderConfig{
			{Type: "github", Token: "ghp_default"},
			{Type: "gitlab", Host: "gitlab.mycompany.com", Token: "glpat_custom"},
		},
	}

	p, ok := cfg.FindProvider("github", "github.com")
	if !ok || p.Token != "ghp_default" {
		t.Errorf("should find github default: %+v", p)
	}

	p, ok = cfg.FindProvider("gitlab", "gitlab.mycompany.com")
	if !ok || p.Token != "glpat_custom" {
		t.Errorf("should find gitlab custom: %+v", p)
	}

	_, ok = cfg.FindProvider("gitlab", "gitlab.com")
	if ok {
		t.Error("should not find gitlab.com when only custom host is configured")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/config/ -v`
Expected: FAIL — package not found

- [ ] **Step 3: Write config implementation**

```go
// internal/config/config.go
package config

import (
	"errors"
	"fmt"
	"os"

	toml "github.com/pelletier/go-toml/v2"
)

type Defaults struct {
	Remote        string `toml:"remote"`
	PollingActive int    `toml:"polling_active"`
	PollingIdle   int    `toml:"polling_idle"`
	ConfirmActions bool  `toml:"confirm_actions"`
	PipelineLimit int    `toml:"pipeline_limit"`
}

type ProviderConfig struct {
	Type     string `toml:"type"`
	Host     string `toml:"host"`
	Token    string `toml:"token"`
	Username string `toml:"username"`
}

type Config struct {
	Defaults  Defaults         `toml:"defaults"`
	Providers []ProviderConfig `toml:"providers"`
}

// Default returns a Config with sensible defaults.
func Default() Config {
	return Config{
		Defaults: Defaults{
			PollingActive:  5,
			PollingIdle:    60,
			ConfirmActions: true,
			PipelineLimit:  25,
		},
	}
}

// LoadFromPath reads a TOML config file. If the file does not exist, returns defaults.
func LoadFromPath(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading config: %w", err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config: %w", err)
	}

	// Apply defaults for zero values
	if cfg.Defaults.PollingActive == 0 {
		cfg.Defaults.PollingActive = 5
	}
	if cfg.Defaults.PollingIdle == 0 {
		cfg.Defaults.PollingIdle = 60
	}
	if cfg.Defaults.PipelineLimit == 0 {
		cfg.Defaults.PipelineLimit = 25
	}

	if err := cfg.validate(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// Load reads the config from the default path (~/.config/manifold/config.toml).
func Load() (Config, error) {
	home, err := os.UserConfigDir()
	if err != nil {
		return Default(), nil
	}
	return LoadFromPath(home + "/manifold/config.toml")
}

func (c Config) validate() error {
	seen := make(map[string]bool)
	for _, p := range c.Providers {
		host := p.Host
		if host == "" {
			switch p.Type {
			case "github":
				host = "github.com"
			case "gitlab":
				host = "gitlab.com"
			case "bitbucket":
				host = "bitbucket.org"
			}
		}
		key := p.Type + ":" + host
		if seen[key] {
			return fmt.Errorf("duplicate provider config for %s at %s", p.Type, host)
		}
		seen[key] = true
	}
	return nil
}

// FindProvider looks up a provider config by type and host.
func (c Config) FindProvider(providerType, host string) (ProviderConfig, bool) {
	for _, p := range c.Providers {
		pHost := p.Host
		if pHost == "" {
			switch p.Type {
			case "github":
				pHost = "github.com"
			case "gitlab":
				pHost = "gitlab.com"
			case "bitbucket":
				pHost = "bitbucket.org"
			}
		}
		if p.Type == providerType && pHost == host {
			return p, true
		}
	}
	return ProviderConfig{}, false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/config/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add TOML config loading with defaults and validation"
```

---

### Task 4: Detector

**Files:**
- Create: `internal/provider/detector.go`
- Create: `internal/provider/detector_test.go`

- [ ] **Step 1: Write detector tests**

```go
// internal/provider/detector_test.go
package provider

import (
	"testing"

	"github.com/steven/manifold/internal/config"
)

func TestDetectFromURL(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		extraHosts   map[string]string // host → provider type
		wantType     string
		wantOwner    string
		wantRepo     string
		wantHost     string
		wantErr      bool
	}{
		{
			name:     "github https",
			url:      "https://github.com/user/repo.git",
			wantType: "github", wantOwner: "user", wantRepo: "repo", wantHost: "github.com",
		},
		{
			name:     "github ssh",
			url:      "git@github.com:user/repo.git",
			wantType: "github", wantOwner: "user", wantRepo: "repo", wantHost: "github.com",
		},
		{
			name:     "gitlab https",
			url:      "https://gitlab.com/org/project.git",
			wantType: "gitlab", wantOwner: "org", wantRepo: "project", wantHost: "gitlab.com",
		},
		{
			name:     "gitlab ssh",
			url:      "git@gitlab.com:org/project.git",
			wantType: "gitlab", wantOwner: "org", wantRepo: "project", wantHost: "gitlab.com",
		},
		{
			name:     "bitbucket https",
			url:      "https://bitbucket.org/team/repo.git",
			wantType: "bitbucket", wantOwner: "team", wantRepo: "repo", wantHost: "bitbucket.org",
		},
		{
			name:     "bitbucket ssh",
			url:      "git@bitbucket.org:team/repo.git",
			wantType: "bitbucket", wantOwner: "team", wantRepo: "repo", wantHost: "bitbucket.org",
		},
		{
			name:       "github enterprise custom host",
			url:        "git@github.mycompany.com:org/repo.git",
			extraHosts: map[string]string{"github.mycompany.com": "github"},
			wantType:   "github", wantOwner: "org", wantRepo: "repo", wantHost: "github.mycompany.com",
		},
		{
			name:       "gitlab self-hosted",
			url:        "https://gitlab.internal.io/team/app.git",
			extraHosts: map[string]string{"gitlab.internal.io": "gitlab"},
			wantType:   "gitlab", wantOwner: "team", wantRepo: "app", wantHost: "gitlab.internal.io",
		},
		{
			name:    "unknown host no config",
			url:     "git@unknown.host:foo/bar.git",
			wantErr: true,
		},
		{
			name:     "https without .git suffix",
			url:      "https://github.com/user/repo",
			wantType: "github", wantOwner: "user", wantRepo: "repo", wantHost: "github.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extraHosts := tt.extraHosts
			if extraHosts == nil {
				extraHosts = map[string]string{}
			}

			result, err := DetectFromURL(tt.url, extraHosts)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.ProviderType != tt.wantType {
				t.Errorf("type: got %q, want %q", result.ProviderType, tt.wantType)
			}
			if result.Owner != tt.wantOwner {
				t.Errorf("owner: got %q, want %q", result.Owner, tt.wantOwner)
			}
			if result.Repo != tt.wantRepo {
				t.Errorf("repo: got %q, want %q", result.Repo, tt.wantRepo)
			}
			if result.Host != tt.wantHost {
				t.Errorf("host: got %q, want %q", result.Host, tt.wantHost)
			}
		})
	}
}

func TestExtraHostsFromConfig(t *testing.T) {
	cfg := config.Config{
		Providers: []config.ProviderConfig{
			{Type: "github", Host: "github.mycompany.com", Token: "xxx"},
			{Type: "gitlab", Host: "gitlab.internal.io", Token: "yyy"},
			{Type: "github"}, // default host, should not appear in extra
		},
	}

	extra := ExtraHostsFromConfig(cfg)
	if extra["github.mycompany.com"] != "github" {
		t.Errorf("missing github enterprise host")
	}
	if extra["gitlab.internal.io"] != "gitlab" {
		t.Errorf("missing gitlab self-hosted host")
	}
	if _, ok := extra["github.com"]; ok {
		t.Error("default hosts should not be in extra")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/provider/ -v -run TestDetect`
Expected: FAIL — functions not defined

- [ ] **Step 3: Write detector implementation**

```go
// internal/provider/detector.go
package provider

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/steven/manifold/internal/config"
)

var defaultHosts = map[string]string{
	"github.com":    "github",
	"gitlab.com":    "gitlab",
	"bitbucket.org": "bitbucket",
}

// DetectResult holds the result of detecting a provider from a git remote URL.
type DetectResult struct {
	ProviderType string
	Host         string
	Owner        string
	Repo         string
}

// DetectFromURL parses a git remote URL and identifies the CI provider.
// extraHosts maps custom hostnames to provider types (for self-hosted instances).
func DetectFromURL(remoteURL string, extraHosts map[string]string) (DetectResult, error) {
	host, owner, repo, err := parseGitURL(remoteURL)
	if err != nil {
		return DetectResult{}, err
	}

	// Check default hosts first
	if providerType, ok := defaultHosts[host]; ok {
		return DetectResult{
			ProviderType: providerType,
			Host:         host,
			Owner:        owner,
			Repo:         repo,
		}, nil
	}

	// Check custom hosts from config
	if providerType, ok := extraHosts[host]; ok {
		return DetectResult{
			ProviderType: providerType,
			Host:         host,
			Owner:        owner,
			Repo:         repo,
		}, nil
	}

	return DetectResult{}, fmt.Errorf("unsupported provider for remote: %s", remoteURL)
}

// ExtraHostsFromConfig extracts custom host → provider type mappings from config.
func ExtraHostsFromConfig(cfg config.Config) map[string]string {
	extra := make(map[string]string)
	for _, p := range cfg.Providers {
		if p.Host == "" {
			continue
		}
		// Skip if it's a default host
		if _, isDefault := defaultHosts[p.Host]; isDefault {
			continue
		}
		extra[p.Host] = p.Type
	}
	return extra
}

func parseGitURL(raw string) (host, owner, repo string, err error) {
	// SSH format: git@host:owner/repo.git
	if strings.HasPrefix(raw, "git@") {
		raw = strings.TrimPrefix(raw, "git@")
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) != 2 {
			return "", "", "", fmt.Errorf("invalid SSH URL: %s", raw)
		}
		host = parts[0]
		path := parts[1]
		owner, repo = splitOwnerRepo(path)
		if owner == "" || repo == "" {
			return "", "", "", fmt.Errorf("cannot extract owner/repo from: %s", path)
		}
		return host, owner, repo, nil
	}

	// HTTPS format
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid URL: %w", err)
	}
	host = u.Hostname()
	owner, repo = splitOwnerRepo(strings.TrimPrefix(u.Path, "/"))
	if owner == "" || repo == "" {
		return "", "", "", fmt.Errorf("cannot extract owner/repo from: %s", u.Path)
	}
	return host, owner, repo, nil
}

func splitOwnerRepo(path string) (owner, repo string) {
	path = strings.TrimSuffix(path, ".git")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/provider/ -v -run TestDetect`
Expected: PASS

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/provider/ -v -run TestExtraHosts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/provider/detector.go internal/provider/detector_test.go
git commit -m "feat: add provider detector from git remote URL"
```

---

### Task 5: Auth Resolution

**Files:**
- Create: `internal/provider/auth.go`
- Create: `internal/provider/auth_test.go`

- [ ] **Step 1: Write auth tests**

```go
// internal/provider/auth_test.go
package provider

import (
	"os"
	"testing"

	"github.com/steven/manifold/internal/config"
)

func TestResolveAuthFromEnv(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env_token_123")

	cfg := config.Config{}
	auth, err := ResolveAuth("github", "github.com", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.Token != "env_token_123" {
		t.Errorf("token: got %q, want %q", auth.Token, "env_token_123")
	}
	if auth.Source != "env" {
		t.Errorf("source: got %q, want %q", auth.Source, "env")
	}
}

func TestResolveAuthFromConfig(t *testing.T) {
	// Ensure env vars are not set
	os.Unsetenv("GITHUB_TOKEN")

	cfg := config.Config{
		Providers: []config.ProviderConfig{
			{Type: "github", Token: "config_token_456"},
		},
	}

	auth, err := ResolveAuth("github", "github.com", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.Token != "config_token_456" {
		t.Errorf("token: got %q, want %q", auth.Token, "config_token_456")
	}
	if auth.Source != "config" {
		t.Errorf("source: got %q, want %q", auth.Source, "config")
	}
}

func TestResolveAuthBitbucketWithUsername(t *testing.T) {
	t.Setenv("BITBUCKET_TOKEN", "bb_token")
	t.Setenv("BITBUCKET_USERNAME", "bb_user")

	cfg := config.Config{}
	auth, err := ResolveAuth("bitbucket", "bitbucket.org", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.Token != "bb_token" || auth.Username != "bb_user" {
		t.Errorf("got token=%q username=%q", auth.Token, auth.Username)
	}
}

func TestResolveAuthNotFound(t *testing.T) {
	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("GITLAB_TOKEN")
	os.Unsetenv("BITBUCKET_TOKEN")

	cfg := config.Config{}
	_, err := ResolveAuth("github", "github.com", cfg)
	if err == nil {
		t.Fatal("expected error when no auth is found")
	}
}

func TestResolveAuthConfigWithUsername(t *testing.T) {
	os.Unsetenv("BITBUCKET_TOKEN")
	os.Unsetenv("BITBUCKET_USERNAME")

	cfg := config.Config{
		Providers: []config.ProviderConfig{
			{Type: "bitbucket", Token: "bb_config_token", Username: "bb_config_user"},
		},
	}

	auth, err := ResolveAuth("bitbucket", "bitbucket.org", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.Username != "bb_config_user" {
		t.Errorf("username: got %q, want %q", auth.Username, "bb_config_user")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/provider/ -v -run TestResolveAuth`
Expected: FAIL — ResolveAuth not defined

- [ ] **Step 3: Write auth implementation**

```go
// internal/provider/auth.go
package provider

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/steven/manifold/internal/config"
)

// Auth holds the resolved authentication credentials for a provider.
type Auth struct {
	Token    string
	Username string // only used for Bitbucket app passwords
	Source   string // "cli", "env", or "config"
}

// ResolveAuth resolves authentication for a provider in order:
// 1. CLI tool (gh/glab) — skipped for Bitbucket
// 2. Environment variables
// 3. TOML config
// 4. Error
func ResolveAuth(providerType, host string, cfg config.Config) (Auth, error) {
	// 1. Try CLI tool
	if auth, ok := tryCliAuth(providerType); ok {
		return auth, nil
	}

	// 2. Try environment variables
	if auth, ok := tryEnvAuth(providerType); ok {
		return auth, nil
	}

	// 3. Try config file
	if p, ok := cfg.FindProvider(providerType, host); ok && p.Token != "" {
		return Auth{
			Token:    p.Token,
			Username: p.Username,
			Source:   "config",
		}, nil
	}

	// 4. No auth found
	return Auth{}, fmt.Errorf(authErrorMessage(providerType, host))
}

func tryCliAuth(providerType string) (Auth, bool) {
	switch providerType {
	case "github":
		if _, err := exec.LookPath("gh"); err == nil {
			cmd := exec.Command("gh", "auth", "token")
			out, err := cmd.Output()
			if err == nil {
				token := strings.TrimSpace(string(out))
				if token != "" {
					return Auth{Token: token, Source: "cli"}, true
				}
			}
		}
	case "gitlab":
		if _, err := exec.LookPath("glab"); err == nil {
			cmd := exec.Command("glab", "auth", "status", "-t")
			out, err := cmd.Output()
			if err == nil {
				// glab outputs token info; extract the token
				for _, line := range strings.Split(string(out), "\n") {
					if strings.Contains(line, "Token:") {
						parts := strings.Fields(line)
						if len(parts) >= 2 {
							return Auth{Token: parts[len(parts)-1], Source: "cli"}, true
						}
					}
				}
			}
		}
	}
	// Bitbucket has no CLI tool — skip
	return Auth{}, false
}

func tryEnvAuth(providerType string) (Auth, bool) {
	switch providerType {
	case "github":
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			return Auth{Token: token, Source: "env"}, true
		}
	case "gitlab":
		if token := os.Getenv("GITLAB_TOKEN"); token != "" {
			return Auth{Token: token, Source: "env"}, true
		}
	case "bitbucket":
		if token := os.Getenv("BITBUCKET_TOKEN"); token != "" {
			return Auth{
				Token:    token,
				Username: os.Getenv("BITBUCKET_USERNAME"),
				Source:   "env",
			}, true
		}
	}
	return Auth{}, false
}

func authErrorMessage(providerType, host string) string {
	switch providerType {
	case "github":
		return fmt.Sprintf("no auth found for %s. Run \"gh auth login\" or set GITHUB_TOKEN or add a token to ~/.config/manifold/config.toml", host)
	case "gitlab":
		return fmt.Sprintf("no auth found for %s. Run \"glab auth login\" or set GITLAB_TOKEN or add a token to ~/.config/manifold/config.toml", host)
	case "bitbucket":
		return fmt.Sprintf("no auth found for %s. Set BITBUCKET_TOKEN or add a token to ~/.config/manifold/config.toml", host)
	default:
		return fmt.Sprintf("no auth found for %s (%s)", host, providerType)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/provider/ -v -run TestResolveAuth`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/provider/auth.go internal/provider/auth_test.go
git commit -m "feat: add auth resolution chain (CLI, env, config)"
```

---

### Task 6: TUI Styles and Keys

**Files:**
- Create: `internal/tui/styles.go`
- Create: `internal/tui/keys.go`

- [ ] **Step 1: Write styles**

```go
// internal/tui/styles.go
package tui

import "charm.land/lipgloss/v2"

// Status icons — unified across all providers.
const (
	IconQueued   = "◷"
	IconPending  = "○"
	IconRunning  = "●"
	IconSuccess  = "✓"
	IconFailed   = "✗"
	IconCanceled = "⊘"
	IconSkipped  = "–"
)

// Colors
var (
	ColorBlue     = lipgloss.Color("#5B9BD5")
	ColorGray     = lipgloss.Color("#808080")
	ColorYellow   = lipgloss.Color("#E5C07B")
	ColorGreen    = lipgloss.Color("#98C379")
	ColorRed      = lipgloss.Color("#E06C75")
	ColorDarkGray = lipgloss.Color("#5C6370")
	ColorWhite    = lipgloss.Color("#ABB2BF")
	ColorAccent   = lipgloss.Color("#61AFEF")
)

// Panel styles
var (
	PanelBorder = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorGray)

	PanelBorderActive = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(ColorAccent)

	PanelTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorGray)

	SelectedItem = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent)

	NormalItem = lipgloss.NewStyle().
			Foreground(ColorWhite)
)

// StatusIcon returns the icon for a given pipeline status.
func StatusIcon(status string) string {
	switch status {
	case "queued":
		return IconQueued
	case "pending":
		return IconPending
	case "running":
		return IconRunning
	case "success":
		return IconSuccess
	case "failed":
		return IconFailed
	case "canceled":
		return IconCanceled
	case "skipped":
		return IconSkipped
	default:
		return "?"
	}
}

// StatusColor returns the color for a given pipeline status.
func StatusColor(status string) lipgloss.Color {
	switch status {
	case "queued":
		return ColorBlue
	case "pending":
		return ColorGray
	case "running":
		return ColorYellow
	case "success":
		return ColorGreen
	case "failed":
		return ColorRed
	case "canceled":
		return ColorGray
	case "skipped":
		return ColorDarkGray
	default:
		return ColorWhite
	}
}
```

- [ ] **Step 2: Write keybindings**

```go
// internal/tui/keys.go
package tui

// Key constants for navigation and actions.
const (
	KeyTab      = "tab"
	KeyShiftTab = "shift+tab"
	KeyUp       = "up"
	KeyDown     = "down"
	KeyLeft     = "left"
	KeyRight    = "right"
	KeyJ        = "j"
	KeyK        = "k"
	KeyH        = "h"
	KeyL        = "l"
	KeyG        = "g"
	KeyShiftG   = "G"
	KeyEnter    = "enter"
	KeyEsc      = "escape"
	KeyR        = "r"
	KeyC        = "c"
	KeyO        = "o"
	KeyY        = "y"
	KeyShiftR   = "R"
	KeyQuestion = "?"
	KeyQ        = "q"
	KeyCtrlC    = "ctrl+c"
)
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /home/steven/work/project-manifold/manifold && go build ./internal/tui/`
Expected: success (no output)

- [ ] **Step 4: Commit**

```bash
git add internal/tui/styles.go internal/tui/keys.go
git commit -m "feat: add TUI styles, status icons, and keybindings"
```

---

### Task 7: Pipeline List Panel

**Files:**
- Create: `internal/tui/pipelines/pipelines.go`
- Create: `internal/tui/pipelines/pipelines_test.go`

- [ ] **Step 1: Write pipeline panel tests**

```go
// internal/tui/pipelines/pipelines_test.go
package pipelines

import (
	"testing"
	"time"

	"github.com/steven/manifold/internal/provider"
)

func TestNewModel(t *testing.T) {
	m := New(80, 20)
	if m.cursor != 0 {
		t.Errorf("cursor should start at 0, got %d", m.cursor)
	}
	if m.Width != 80 || m.Height != 20 {
		t.Errorf("unexpected dimensions: %dx%d", m.Width, m.Height)
	}
}

func TestSetPipelines(t *testing.T) {
	m := New(80, 20)
	pipelines := []provider.Pipeline{
		{ID: "1", Ref: "main", Status: provider.StatusSuccess, Commit: "abc1234"},
		{ID: "2", Ref: "feature", Status: provider.StatusRunning, Commit: "def5678"},
	}
	m.SetPipelines(pipelines)
	if len(m.pipelines) != 2 {
		t.Errorf("got %d pipelines, want 2", len(m.pipelines))
	}
}

func TestCursorMovement(t *testing.T) {
	m := New(80, 20)
	m.SetPipelines([]provider.Pipeline{
		{ID: "1", Ref: "main", Status: provider.StatusSuccess},
		{ID: "2", Ref: "dev", Status: provider.StatusRunning},
		{ID: "3", Ref: "fix", Status: provider.StatusFailed},
	})

	m.MoveDown()
	if m.cursor != 1 {
		t.Errorf("cursor after MoveDown: got %d, want 1", m.cursor)
	}

	m.MoveDown()
	if m.cursor != 2 {
		t.Errorf("cursor after second MoveDown: got %d, want 2", m.cursor)
	}

	m.MoveDown() // should not go past end
	if m.cursor != 2 {
		t.Errorf("cursor should stay at 2, got %d", m.cursor)
	}

	m.MoveUp()
	if m.cursor != 1 {
		t.Errorf("cursor after MoveUp: got %d, want 1", m.cursor)
	}

	m.GoToTop()
	if m.cursor != 0 {
		t.Errorf("cursor after GoToTop: got %d, want 0", m.cursor)
	}

	m.GoToBottom()
	if m.cursor != 2 {
		t.Errorf("cursor after GoToBottom: got %d, want 2", m.cursor)
	}
}

func TestSelected(t *testing.T) {
	m := New(80, 20)
	m.SetPipelines([]provider.Pipeline{
		{ID: "1", Ref: "main", Status: provider.StatusSuccess, Duration: 2 * time.Minute},
		{ID: "2", Ref: "dev", Status: provider.StatusRunning},
	})

	p, ok := m.Selected()
	if !ok {
		t.Fatal("should have selection")
	}
	if p.ID != "1" {
		t.Errorf("selected ID: got %q, want %q", p.ID, "1")
	}

	m.MoveDown()
	p, ok = m.Selected()
	if !ok || p.ID != "2" {
		t.Errorf("selected after move: got %q, want %q", p.ID, "2")
	}
}

func TestSelectedEmpty(t *testing.T) {
	m := New(80, 20)
	_, ok := m.Selected()
	if ok {
		t.Error("should not have selection on empty list")
	}
}

func TestCursorClampOnUpdate(t *testing.T) {
	m := New(80, 20)
	m.SetPipelines([]provider.Pipeline{
		{ID: "1"}, {ID: "2"}, {ID: "3"},
	})
	m.GoToBottom() // cursor = 2

	// Reduce list size — cursor should clamp
	m.SetPipelines([]provider.Pipeline{
		{ID: "1"},
	})
	if m.cursor != 0 {
		t.Errorf("cursor should clamp to 0, got %d", m.cursor)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/tui/pipelines/ -v`
Expected: FAIL — package not found

- [ ] **Step 3: Write pipeline panel implementation**

```go
// internal/tui/pipelines/pipelines.go
package pipelines

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/provider"
	"github.com/steven/manifold/internal/tui"
)

// Model represents the pipeline list panel.
type Model struct {
	pipelines []provider.Pipeline
	cursor    int
	Width     int
	Height    int
	Focused   bool
}

// New creates a new pipeline list panel.
func New(width, height int) Model {
	return Model{
		Width:  width,
		Height: height,
	}
}

// SetPipelines updates the pipeline list and clamps the cursor.
func (m *Model) SetPipelines(pipelines []provider.Pipeline) {
	m.pipelines = pipelines
	if m.cursor >= len(m.pipelines) {
		m.cursor = max(0, len(m.pipelines)-1)
	}
}

// Selected returns the currently selected pipeline.
func (m Model) Selected() (provider.Pipeline, bool) {
	if len(m.pipelines) == 0 {
		return provider.Pipeline{}, false
	}
	return m.pipelines[m.cursor], true
}

// MoveDown moves the cursor down by one.
func (m *Model) MoveDown() {
	if m.cursor < len(m.pipelines)-1 {
		m.cursor++
	}
}

// MoveUp moves the cursor up by one.
func (m *Model) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

// GoToTop moves the cursor to the first item.
func (m *Model) GoToTop() {
	m.cursor = 0
}

// GoToBottom moves the cursor to the last item.
func (m *Model) GoToBottom() {
	if len(m.pipelines) > 0 {
		m.cursor = len(m.pipelines) - 1
	}
}

// View renders the pipeline list panel.
func (m Model) View() string {
	var b strings.Builder

	contentHeight := m.Height - 2 // account for border

	for i, p := range m.pipelines {
		if i >= contentHeight {
			break
		}

		icon := tui.StatusIcon(string(p.Status))
		color := tui.StatusColor(string(p.Status))
		iconStyled := lipgloss.NewStyle().Foreground(color).Render(icon)

		ref := truncate(p.Ref, m.Width-10)
		line := fmt.Sprintf(" %s %s", iconStyled, ref)

		if i == m.cursor {
			line = tui.SelectedItem.Render(fmt.Sprintf(" %s %s", icon, ref))
		}

		b.WriteString(line)
		if i < len(m.pipelines)-1 {
			b.WriteString("\n")
		}
	}

	borderStyle := tui.PanelBorder
	if m.Focused {
		borderStyle = tui.PanelBorderActive
	}

	return borderStyle.
		Width(m.Width).
		Height(m.Height).
		Render(tui.PanelTitle.Render("Pipelines") + "\n" + b.String())
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/tui/pipelines/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/pipelines/
git commit -m "feat: add pipeline list panel component"
```

---

### Task 8: Jobs List Panel

**Files:**
- Create: `internal/tui/jobs/jobs.go`
- Create: `internal/tui/jobs/jobs_test.go`

- [ ] **Step 1: Write jobs panel tests**

```go
// internal/tui/jobs/jobs_test.go
package jobs

import (
	"testing"
	"time"

	"github.com/steven/manifold/internal/provider"
)

func TestNewModel(t *testing.T) {
	m := New(80, 20)
	if m.cursor != 0 {
		t.Errorf("cursor should start at 0, got %d", m.cursor)
	}
}

func TestSetJobs(t *testing.T) {
	m := New(80, 20)
	jobs := []provider.Job{
		{ID: "1", Name: "build", Status: provider.StatusSuccess, Duration: 30 * time.Second},
		{ID: "2", Name: "test", Status: provider.StatusRunning, Duration: 45 * time.Second},
		{ID: "3", Name: "deploy", Status: provider.StatusPending},
	}
	m.SetJobs(jobs)
	if len(m.jobs) != 3 {
		t.Errorf("got %d jobs, want 3", len(m.jobs))
	}
}

func TestCursorMovement(t *testing.T) {
	m := New(80, 20)
	m.SetJobs([]provider.Job{
		{ID: "1", Name: "build"}, {ID: "2", Name: "test"}, {ID: "3", Name: "deploy"},
	})

	m.MoveDown()
	if m.cursor != 1 {
		t.Errorf("cursor: got %d, want 1", m.cursor)
	}
	m.MoveDown()
	m.MoveDown() // should clamp
	if m.cursor != 2 {
		t.Errorf("cursor: got %d, want 2", m.cursor)
	}
	m.MoveUp()
	if m.cursor != 1 {
		t.Errorf("cursor: got %d, want 1", m.cursor)
	}
	m.GoToTop()
	if m.cursor != 0 {
		t.Errorf("cursor: got %d, want 0", m.cursor)
	}
	m.GoToBottom()
	if m.cursor != 2 {
		t.Errorf("cursor: got %d, want 2", m.cursor)
	}
}

func TestSelected(t *testing.T) {
	m := New(80, 20)
	m.SetJobs([]provider.Job{
		{ID: "1", Name: "build"}, {ID: "2", Name: "test"},
	})
	j, ok := m.Selected()
	if !ok || j.ID != "1" {
		t.Errorf("selected: got %q, want %q", j.ID, "1")
	}
}

func TestSelectedEmpty(t *testing.T) {
	m := New(80, 20)
	_, ok := m.Selected()
	if ok {
		t.Error("should not have selection on empty list")
	}
}

func TestClear(t *testing.T) {
	m := New(80, 20)
	m.SetJobs([]provider.Job{{ID: "1"}})
	m.Clear()
	if len(m.jobs) != 0 {
		t.Error("clear should empty the list")
	}
	if m.cursor != 0 {
		t.Error("clear should reset cursor")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/tui/jobs/ -v`
Expected: FAIL

- [ ] **Step 3: Write jobs panel implementation**

```go
// internal/tui/jobs/jobs.go
package jobs

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/provider"
	"github.com/steven/manifold/internal/tui"
)

// Model represents the jobs list panel.
type Model struct {
	jobs    []provider.Job
	cursor  int
	Width   int
	Height  int
	Focused bool
}

// New creates a new jobs list panel.
func New(width, height int) Model {
	return Model{Width: width, Height: height}
}

// SetJobs updates the jobs list and clamps the cursor.
func (m *Model) SetJobs(jobs []provider.Job) {
	m.jobs = jobs
	if m.cursor >= len(m.jobs) {
		m.cursor = max(0, len(m.jobs)-1)
	}
}

// Clear empties the jobs list.
func (m *Model) Clear() {
	m.jobs = nil
	m.cursor = 0
}

// Selected returns the currently selected job.
func (m Model) Selected() (provider.Job, bool) {
	if len(m.jobs) == 0 {
		return provider.Job{}, false
	}
	return m.jobs[m.cursor], true
}

func (m *Model) MoveDown() {
	if m.cursor < len(m.jobs)-1 {
		m.cursor++
	}
}

func (m *Model) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *Model) GoToTop() {
	m.cursor = 0
}

func (m *Model) GoToBottom() {
	if len(m.jobs) > 0 {
		m.cursor = len(m.jobs) - 1
	}
}

// View renders the jobs list panel.
func (m Model) View() string {
	var b strings.Builder

	contentHeight := m.Height - 2

	for i, j := range m.jobs {
		if i >= contentHeight {
			break
		}

		icon := tui.StatusIcon(string(j.Status))
		color := tui.StatusColor(string(j.Status))
		iconStyled := lipgloss.NewStyle().Foreground(color).Render(icon)

		dur := formatDuration(j.Duration)
		name := j.Name

		line := fmt.Sprintf(" %s %-20s %s", iconStyled, name, dur)
		if i == m.cursor {
			line = tui.SelectedItem.Render(fmt.Sprintf(" %s %-20s %s", icon, name, dur))
		}

		b.WriteString(line)
		if i < len(m.jobs)-1 {
			b.WriteString("\n")
		}
	}

	borderStyle := tui.PanelBorder
	if m.Focused {
		borderStyle = tui.PanelBorderActive
	}

	return borderStyle.
		Width(m.Width).
		Height(m.Height).
		Render(tui.PanelTitle.Render("Jobs") + "\n" + b.String())
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return ""
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%02ds", m, s)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/tui/jobs/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/jobs/
git commit -m "feat: add jobs list panel component"
```

---

### Task 9: Detail Panel (Steps + Log Viewer)

**Files:**
- Create: `internal/tui/detail/detail.go`
- Create: `internal/tui/detail/detail_test.go`

- [ ] **Step 1: Write detail panel tests**

```go
// internal/tui/detail/detail_test.go
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
	m.maxLogLines = 5 // small buffer for testing

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
	// autoScroll should be true by default
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/tui/detail/ -v`
Expected: FAIL

- [ ] **Step 3: Write detail panel implementation**

```go
// internal/tui/detail/detail.go
package detail

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/provider"
	"github.com/steven/manifold/internal/tui"
)

const defaultMaxLogLines = 10000

// Model represents the detail panel showing steps and logs.
type Model struct {
	job         provider.Job
	logLines    []string
	logOffset   int // offset for incremental log fetching
	scrollPos   int // current scroll position in the log view
	autoScroll  bool
	maxLogLines int
	Width       int
	Height      int
	Focused     bool
}

// New creates a new detail panel.
func New(width, height int) Model {
	return Model{
		Width:       width,
		Height:      height,
		autoScroll:  true,
		maxLogLines: defaultMaxLogLines,
	}
}

// SetJob sets the current job and clears the log buffer.
func (m *Model) SetJob(j provider.Job) {
	m.job = j
	m.ClearLog()
}

// AppendLog appends log content to the buffer.
func (m *Model) AppendLog(content string) {
	if content == "" {
		return
	}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	m.logLines = append(m.logLines, lines...)

	// Ring buffer: discard oldest lines if over limit
	if len(m.logLines) > m.maxLogLines {
		m.logLines = m.logLines[len(m.logLines)-m.maxLogLines:]
	}

	if m.autoScroll {
		m.scrollToBottom()
	}
}

// ClearLog empties the log buffer and resets offset.
func (m *Model) ClearLog() {
	m.logLines = nil
	m.logOffset = 0
	m.scrollPos = 0
	m.autoScroll = true
}

// LogLineCount returns the number of log lines in the buffer.
func (m Model) LogLineCount() int {
	return len(m.logLines)
}

// LogOffset returns the current fetch offset for incremental polling.
func (m Model) LogOffset() int {
	return m.logOffset
}

// SetLogOffset sets the fetch offset.
func (m *Model) SetLogOffset(offset int) {
	m.logOffset = offset
}

// ScrollUp scrolls the log view up.
func (m *Model) ScrollUp() {
	if m.scrollPos > 0 {
		m.scrollPos--
		m.autoScroll = false
	}
}

// ScrollDown scrolls the log view down.
func (m *Model) ScrollDown() {
	maxScroll := m.maxScrollPos()
	if m.scrollPos < maxScroll {
		m.scrollPos++
	}
	if m.scrollPos >= maxScroll {
		m.autoScroll = true
	}
}

func (m *Model) scrollToBottom() {
	m.scrollPos = m.maxScrollPos()
}

func (m Model) maxScrollPos() int {
	logViewHeight := m.logViewHeight()
	if len(m.logLines) <= logViewHeight {
		return 0
	}
	return len(m.logLines) - logViewHeight
}

func (m Model) logViewHeight() int {
	// Height minus border, title, steps section, separator
	stepsHeight := len(m.job.Steps) + 1 // steps + header
	if stepsHeight == 1 {
		stepsHeight = 0 // no steps section if empty
	}
	return max(1, m.Height-4-stepsHeight)
}

// HasJob returns true if a job is currently set.
func (m Model) HasJob() bool {
	return m.job.ID != ""
}

// Job returns the current job.
func (m Model) Job() provider.Job {
	return m.job
}

// View renders the detail panel.
func (m Model) View() string {
	var b strings.Builder

	if !m.HasJob() {
		b.WriteString("\n  Select a job to view details")
	} else {
		// Steps section
		if len(m.job.Steps) > 0 {
			b.WriteString(tui.PanelTitle.Render("Steps") + "\n")
			for _, s := range m.job.Steps {
				icon := tui.StatusIcon(string(s.Status))
				color := tui.StatusColor(string(s.Status))
				iconStyled := lipgloss.NewStyle().Foreground(color).Render(icon)
				b.WriteString(fmt.Sprintf("  %s %s\n", iconStyled, s.Name))
			}
		}

		// Log section
		b.WriteString(tui.PanelTitle.Render("Log") + "\n")
		logHeight := m.logViewHeight()

		if len(m.logLines) == 0 {
			b.WriteString("  (no log output)")
		} else {
			start := m.scrollPos
			end := start + logHeight
			if end > len(m.logLines) {
				end = len(m.logLines)
			}
			for i := start; i < end; i++ {
				b.WriteString("  " + m.logLines[i] + "\n")
			}
		}
	}

	borderStyle := tui.PanelBorder
	if m.Focused {
		borderStyle = tui.PanelBorderActive
	}

	return borderStyle.
		Width(m.Width).
		Height(m.Height).
		Render(tui.PanelTitle.Render("Detail") + "\n" + b.String())
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/tui/detail/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/detail/
git commit -m "feat: add detail panel with steps and log viewer"
```

---

### Task 10: Status Bar

**Files:**
- Create: `internal/tui/statusbar/statusbar.go`
- Create: `internal/tui/statusbar/statusbar_test.go`

- [ ] **Step 1: Write status bar tests**

```go
// internal/tui/statusbar/statusbar_test.go
package statusbar

import (
	"strings"
	"testing"
)

func TestView(t *testing.T) {
	m := New(80)
	m.SetProvider("github.com/user/repo")

	view := m.View()
	if !strings.Contains(view, "github.com/user/repo") {
		t.Error("should contain provider info")
	}
}

func TestSetNotification(t *testing.T) {
	m := New(80)
	m.SetNotification("Retry failed: 403 Forbidden", true)
	view := m.View()
	if !strings.Contains(view, "Retry failed") {
		t.Error("should contain notification")
	}
}

func TestClearNotification(t *testing.T) {
	m := New(80)
	m.SetNotification("temp msg", false)
	m.ClearNotification()
	if m.notification != "" {
		t.Error("notification should be cleared")
	}
}

func TestContextActions(t *testing.T) {
	m := New(80)
	m.SetActions([]string{"[r]etry", "[c]ancel", "[o]pen"})
	view := m.View()
	if !strings.Contains(view, "[r]etry") {
		t.Error("should contain retry action")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/tui/statusbar/ -v`
Expected: FAIL

- [ ] **Step 3: Write status bar implementation**

```go
// internal/tui/statusbar/statusbar.go
package statusbar

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/tui"
)

// Model represents the status bar at the bottom of the TUI.
type Model struct {
	Width        int
	provider     string
	actions      []string
	notification string
	isError      bool
}

// New creates a new status bar.
func New(width int) Model {
	return Model{
		Width: width,
		actions: []string{
			"[r]etry", "[c]ancel", "[o]pen", "[y]ank URL", "[R]efresh", "[?]help",
		},
	}
}

// SetProvider sets the provider display string.
func (m *Model) SetProvider(s string) {
	m.provider = s
}

// SetActions sets the context-sensitive actions.
func (m *Model) SetActions(actions []string) {
	m.actions = actions
}

// SetNotification sets a notification message.
func (m *Model) SetNotification(msg string, isError bool) {
	m.notification = msg
	m.isError = isError
}

// ClearNotification clears the notification.
func (m *Model) ClearNotification() {
	m.notification = ""
	m.isError = false
}

// View renders the status bar.
func (m Model) View() string {
	left := strings.Join(m.actions, " ")
	right := ""
	if m.provider != "" {
		right = tui.IconRunning + " " + m.provider
	}

	if m.notification != "" {
		color := tui.ColorGreen
		if m.isError {
			color = tui.ColorRed
		}
		left = lipgloss.NewStyle().Foreground(color).Render(m.notification)
	}

	gap := m.Width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}

	bar := " " + left + strings.Repeat(" ", gap) + right + " "

	return tui.StatusBarStyle.Width(m.Width).Render(bar)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/tui/statusbar/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/statusbar/
git commit -m "feat: add status bar component"
```

---

### Task 11: Confirmation Dialog

**Files:**
- Create: `internal/tui/confirm/confirm.go`
- Create: `internal/tui/confirm/confirm_test.go`

- [ ] **Step 1: Write confirmation dialog tests**

```go
// internal/tui/confirm/confirm_test.go
package confirm

import "testing"

func TestNewDialog(t *testing.T) {
	d := New("Retry this pipeline?", "retry")
	if d.Message != "Retry this pipeline?" {
		t.Errorf("message: got %q", d.Message)
	}
	if d.Action != "retry" {
		t.Errorf("action: got %q", d.Action)
	}
	if d.Confirmed {
		t.Error("should not be confirmed initially")
	}
	if d.Answered {
		t.Error("should not be answered initially")
	}
}

func TestConfirm(t *testing.T) {
	d := New("Cancel?", "cancel")
	d.Confirm()
	if !d.Confirmed || !d.Answered {
		t.Error("should be confirmed and answered")
	}
}

func TestDeny(t *testing.T) {
	d := New("Cancel?", "cancel")
	d.Deny()
	if d.Confirmed {
		t.Error("should not be confirmed")
	}
	if !d.Answered {
		t.Error("should be answered")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/tui/confirm/ -v`
Expected: FAIL

- [ ] **Step 3: Write confirmation dialog implementation**

```go
// internal/tui/confirm/confirm.go
package confirm

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/tui"
)

// Model represents a confirmation dialog overlay.
type Model struct {
	Message   string
	Action    string // action identifier (e.g., "retry", "cancel")
	Confirmed bool
	Answered  bool
	Width     int
}

// New creates a new confirmation dialog.
func New(message, action string) Model {
	return Model{
		Message: message,
		Action:  action,
		Width:   40,
	}
}

// Confirm marks the dialog as confirmed.
func (m *Model) Confirm() {
	m.Confirmed = true
	m.Answered = true
}

// Deny marks the dialog as denied.
func (m *Model) Deny() {
	m.Confirmed = false
	m.Answered = true
}

// View renders the confirmation dialog.
func (m Model) View() string {
	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(tui.ColorYellow).
		Padding(1, 2).
		Width(m.Width)

	content := fmt.Sprintf("%s\n\n[y]es  [n]o", m.Message)

	return style.Render(content)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/tui/confirm/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/confirm/
git commit -m "feat: add confirmation dialog component"
```

---

### Task 12: Adaptive Poller

**Files:**
- Create: `internal/poller/poller.go`
- Create: `internal/poller/poller_test.go`

- [ ] **Step 1: Write poller tests**

```go
// internal/poller/poller_test.go
package poller

import (
	"testing"
	"time"
)

func TestNewPoller(t *testing.T) {
	p := New(5*time.Second, 60*time.Second)
	if p.activeInterval != 5*time.Second {
		t.Errorf("active interval: got %v, want 5s", p.activeInterval)
	}
	if p.idleInterval != 60*time.Second {
		t.Errorf("idle interval: got %v, want 60s", p.idleInterval)
	}
}

func TestIntervalSelection(t *testing.T) {
	p := New(5*time.Second, 60*time.Second)

	p.SetHasRunning(true)
	if p.CurrentInterval() != 5*time.Second {
		t.Errorf("with running: got %v, want 5s", p.CurrentInterval())
	}

	p.SetHasRunning(false)
	if p.CurrentInterval() != 60*time.Second {
		t.Errorf("without running: got %v, want 60s", p.CurrentInterval())
	}
}

func TestShouldPollLog(t *testing.T) {
	p := New(5*time.Second, 60*time.Second)

	p.SetHasRunning(false)
	if p.ShouldPollLog() {
		t.Error("should not poll log when nothing is running")
	}

	p.SetHasRunning(true)
	if !p.ShouldPollLog() {
		t.Error("should poll log when something is running")
	}
}

func TestForceRefresh(t *testing.T) {
	p := New(5*time.Second, 60*time.Second)
	if p.ShouldForceRefresh() {
		t.Error("should not force refresh initially")
	}
	p.RequestRefresh()
	if !p.ShouldForceRefresh() {
		t.Error("should force refresh after request")
	}
	// Reading should clear the flag
	if p.ShouldForceRefresh() {
		t.Error("flag should be cleared after read")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/poller/ -v`
Expected: FAIL

- [ ] **Step 3: Write poller implementation**

```go
// internal/poller/poller.go
package poller

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// TickMsg is sent on each poll interval.
type TickMsg struct{}

// LogTickMsg is sent on each log poll interval.
type LogTickMsg struct{}

// Poller manages adaptive polling intervals.
type Poller struct {
	activeInterval time.Duration
	idleInterval   time.Duration
	hasRunning     bool
	forceRefresh   bool
}

// New creates a new Poller with the given intervals.
func New(active, idle time.Duration) *Poller {
	return &Poller{
		activeInterval: active,
		idleInterval:   idle,
	}
}

// SetHasRunning updates whether any pipeline is currently running.
func (p *Poller) SetHasRunning(running bool) {
	p.hasRunning = running
}

// CurrentInterval returns the current polling interval.
func (p *Poller) CurrentInterval() time.Duration {
	if p.hasRunning {
		return p.activeInterval
	}
	return p.idleInterval
}

// ShouldPollLog returns true if log polling should be active.
func (p *Poller) ShouldPollLog() bool {
	return p.hasRunning
}

// RequestRefresh sets the force refresh flag.
func (p *Poller) RequestRefresh() {
	p.forceRefresh = true
}

// ShouldForceRefresh returns and clears the force refresh flag.
func (p *Poller) ShouldForceRefresh() bool {
	if p.forceRefresh {
		p.forceRefresh = false
		return true
	}
	return false
}

// TickCmd returns a tea.Cmd that sends a TickMsg after the current interval.
func (p *Poller) TickCmd() tea.Cmd {
	interval := p.CurrentInterval()
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

// LogTickCmd returns a tea.Cmd that sends a LogTickMsg after 3 seconds.
func (p *Poller) LogTickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return LogTickMsg{}
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/poller/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/poller/
git commit -m "feat: add adaptive poller with active/idle intervals"
```

---

### Task 13: Remote Selector

**Files:**
- Create: `internal/tui/selector/selector.go`
- Create: `internal/tui/selector/selector_test.go`

- [ ] **Step 1: Write selector tests**

```go
// internal/tui/selector/selector_test.go
package selector

import "testing"

func TestNewModel(t *testing.T) {
	remotes := []Remote{
		{Name: "origin", URL: "git@github.com:user/repo.git"},
		{Name: "upstream", URL: "git@gitlab.com:org/repo.git"},
	}
	m := New(remotes)
	if m.cursor != 0 {
		t.Errorf("cursor: got %d, want 0", m.cursor)
	}
	if len(m.remotes) != 2 {
		t.Errorf("remotes: got %d, want 2", len(m.remotes))
	}
}

func TestCursorMovement(t *testing.T) {
	remotes := []Remote{
		{Name: "origin", URL: "url1"},
		{Name: "upstream", URL: "url2"},
		{Name: "fork", URL: "url3"},
	}
	m := New(remotes)
	m.MoveDown()
	if m.cursor != 1 {
		t.Errorf("cursor: got %d, want 1", m.cursor)
	}
	m.MoveDown()
	m.MoveDown() // clamp
	if m.cursor != 2 {
		t.Errorf("cursor: got %d, want 2", m.cursor)
	}
	m.MoveUp()
	if m.cursor != 1 {
		t.Errorf("cursor: got %d, want 1", m.cursor)
	}
}

func TestSelect(t *testing.T) {
	remotes := []Remote{
		{Name: "origin", URL: "url1"},
		{Name: "upstream", URL: "url2"},
	}
	m := New(remotes)
	m.MoveDown()
	m.Select()
	r, ok := m.Selected()
	if !ok || r.Name != "upstream" {
		t.Errorf("selected: got %q, want %q", r.Name, "upstream")
	}
}

func TestNotSelectedUntilEnter(t *testing.T) {
	m := New([]Remote{{Name: "origin", URL: "url"}})
	_, ok := m.Selected()
	if ok {
		t.Error("should not be selected before Enter")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/tui/selector/ -v`
Expected: FAIL

- [ ] **Step 3: Write selector implementation**

```go
// internal/tui/selector/selector.go
package selector

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/tui"
)

// Remote represents a git remote.
type Remote struct {
	Name string
	URL  string
}

// Model represents the remote selector screen.
type Model struct {
	remotes  []Remote
	cursor   int
	selected bool
}

// New creates a new remote selector.
func New(remotes []Remote) Model {
	return Model{remotes: remotes}
}

// MoveDown moves the cursor down.
func (m *Model) MoveDown() {
	if m.cursor < len(m.remotes)-1 {
		m.cursor++
	}
}

// MoveUp moves the cursor up.
func (m *Model) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

// Select confirms the current selection.
func (m *Model) Select() {
	m.selected = true
}

// Selected returns the selected remote, if any.
func (m Model) Selected() (Remote, bool) {
	if !m.selected {
		return Remote{}, false
	}
	return m.remotes[m.cursor], true
}

// View renders the remote selector.
func (m Model) View() string {
	var b strings.Builder

	title := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorAccent).Render("Select a remote:")
	b.WriteString("\n  " + title + "\n\n")

	for i, r := range m.remotes {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		line := fmt.Sprintf("  %s%-12s %s", cursor, r.Name, r.URL)
		if i == m.cursor {
			line = tui.SelectedItem.Render(line)
		}

		b.WriteString(line + "\n")
	}

	b.WriteString("\n  Press Enter to select, q to quit\n")

	return b.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/tui/selector/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/selector/
git commit -m "feat: add remote selector for multi-remote repos"
```

---

### Task 14: Root TUI App Model

**Files:**
- Create: `internal/tui/app.go`
- Create: `internal/tui/app_test.go`

- [ ] **Step 1: Write app model tests**

```go
// internal/tui/app_test.go
package tui

import (
	"testing"

	"github.com/steven/manifold/internal/provider"
)

func TestNewApp(t *testing.T) {
	app := NewApp(provider.DetectResult{
		ProviderType: "github",
		Host:         "github.com",
		Owner:        "user",
		Repo:         "repo",
	}, true, 25)

	if app.focusedPanel != PanelPipelines {
		t.Errorf("initial focus: got %d, want %d", app.focusedPanel, PanelPipelines)
	}
	if app.confirmActions != true {
		t.Error("confirmActions should be true")
	}
}

func TestFocusCycle(t *testing.T) {
	app := NewApp(provider.DetectResult{}, true, 25)

	app.FocusNext()
	if app.focusedPanel != PanelJobs {
		t.Errorf("after FocusNext: got %d, want %d", app.focusedPanel, PanelJobs)
	}

	app.FocusNext()
	if app.focusedPanel != PanelDetail {
		t.Errorf("after FocusNext: got %d, want %d", app.focusedPanel, PanelDetail)
	}

	app.FocusNext() // should wrap
	if app.focusedPanel != PanelPipelines {
		t.Errorf("after wrap: got %d, want %d", app.focusedPanel, PanelPipelines)
	}

	app.FocusPrev() // should wrap backwards
	if app.focusedPanel != PanelDetail {
		t.Errorf("after FocusPrev wrap: got %d, want %d", app.focusedPanel, PanelDetail)
	}
}

func TestProviderLabel(t *testing.T) {
	app := NewApp(provider.DetectResult{
		ProviderType: "github",
		Host:         "github.com",
		Owner:        "user",
		Repo:         "myrepo",
	}, true, 25)

	label := app.ProviderLabel()
	if label != "github.com/user/myrepo" {
		t.Errorf("label: got %q", label)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/tui/ -v -run TestNewApp`
Expected: FAIL

- [ ] **Step 3: Write app model implementation**

```go
// internal/tui/app.go
package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/poller"
	"github.com/steven/manifold/internal/provider"
	"github.com/steven/manifold/internal/tui/confirm"
	"github.com/steven/manifold/internal/tui/detail"
	"github.com/steven/manifold/internal/tui/jobs"
	"github.com/steven/manifold/internal/tui/pipelines"
	"github.com/steven/manifold/internal/tui/statusbar"
)

// Panel identifiers.
const (
	PanelPipelines = iota
	PanelJobs
	PanelDetail
	panelCount = 3
)

// Messages for async operations.
type (
	PipelinesMsg   struct{ Pipelines []provider.Pipeline }
	JobsMsg        struct{ Jobs []provider.Job }
	StepsMsg       struct{ Steps []provider.Step }
	LogMsg         struct{ Content string; NewOffset int }
	ActionResultMsg struct{ Err error; Action string }
	ErrMsg         struct{ Err error }
)

// App is the root Bubble Tea model.
type App struct {
	provider       provider.Provider
	poller         *poller.Poller
	pipelinesPanel pipelines.Model
	jobsPanel      jobs.Model
	detailPanel    detail.Model
	statusBar      statusbar.Model
	confirmDialog  *confirm.Model
	focusedPanel   int
	confirmActions bool
	pipelineLimit  int
	detectResult   provider.DetectResult
	width          int
	height         int
	ready          bool
}

// NewApp creates a new App model.
func NewApp(detect provider.DetectResult, confirmActions bool, pipelineLimit int) App {
	return App{
		focusedPanel:   PanelPipelines,
		confirmActions: confirmActions,
		pipelineLimit:  pipelineLimit,
		detectResult:   detect,
	}
}

// SetProvider sets the CI provider on the app.
func (a *App) SetProvider(p provider.Provider) {
	a.provider = p
}

// SetPoller sets the poller on the app.
func (a *App) SetPoller(p *poller.Poller) {
	a.poller = p
}

// FocusNext moves focus to the next panel.
func (a *App) FocusNext() {
	a.focusedPanel = (a.focusedPanel + 1) % panelCount
	a.updatePanelFocus()
}

// FocusPrev moves focus to the previous panel.
func (a *App) FocusPrev() {
	a.focusedPanel = (a.focusedPanel - 1 + panelCount) % panelCount
	a.updatePanelFocus()
}

// ProviderLabel returns a display label for the current provider.
func (a App) ProviderLabel() string {
	return fmt.Sprintf("%s/%s/%s", a.detectResult.Host, a.detectResult.Owner, a.detectResult.Repo)
}

func (a *App) updatePanelFocus() {
	a.pipelinesPanel.Focused = a.focusedPanel == PanelPipelines
	a.jobsPanel.Focused = a.focusedPanel == PanelJobs
	a.detailPanel.Focused = a.focusedPanel == PanelDetail
}

func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.fetchPipelines(),
		a.poller.TickCmd(),
	)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.resizePanels()
		a.ready = true
		return a, nil

	case tea.KeyPressMsg:
		// If confirmation dialog is active, handle it
		if a.confirmDialog != nil {
			return a.handleConfirmKey(msg)
		}
		return a.handleKey(msg)

	case poller.TickMsg:
		cmds := []tea.Cmd{a.fetchPipelines(), a.poller.TickCmd()}
		if a.poller.ShouldPollLog() {
			cmds = append(cmds, a.fetchLog())
		}
		return a, tea.Batch(cmds...)

	case poller.LogTickMsg:
		if a.poller.ShouldPollLog() {
			return a, tea.Batch(a.fetchLog(), a.poller.LogTickCmd())
		}
		return a, nil

	case PipelinesMsg:
		a.pipelinesPanel.SetPipelines(msg.Pipelines)
		hasRunning := false
		for _, p := range msg.Pipelines {
			if !p.Status.IsTerminal() {
				hasRunning = true
				break
			}
		}
		a.poller.SetHasRunning(hasRunning)
		// Auto-fetch jobs for selected pipeline
		return a, a.fetchJobsForSelected()

	case JobsMsg:
		a.jobsPanel.SetJobs(msg.Jobs)
		return a, a.fetchStepsForSelected()

	case StepsMsg:
		if j, ok := a.jobsPanel.Selected(); ok {
			j.Steps = msg.Steps
			a.detailPanel.SetJob(j)
			if !j.Status.IsTerminal() {
				return a, tea.Batch(a.fetchLog(), a.poller.LogTickCmd())
			}
			return a, a.fetchLog()
		}
		return a, nil

	case LogMsg:
		a.detailPanel.AppendLog(msg.Content)
		a.detailPanel.SetLogOffset(msg.NewOffset)
		return a, nil

	case ActionResultMsg:
		if msg.Err != nil {
			a.statusBar.SetNotification(fmt.Sprintf("✗ %s failed: %v", msg.Action, msg.Err), true)
		} else {
			a.statusBar.SetNotification(fmt.Sprintf("✓ %s successful", msg.Action), false)
		}
		return a, a.fetchPipelines()

	case ErrMsg:
		a.statusBar.SetNotification(fmt.Sprintf("⚠ %v", msg.Err), true)
		return a, nil
	}

	return a, nil
}

func (a App) View() tea.View {
	if !a.ready {
		v := tea.NewView("  Loading...")
		v.AltScreen = true
		return v
	}

	// Layout: three panels side by side + status bar
	left := a.pipelinesPanel.View()
	center := a.jobsPanel.View()
	right := a.detailPanel.View()
	bar := a.statusBar.View()

	panels := lipgloss.JoinHorizontal(lipgloss.Top, left, center, right)
	content := lipgloss.JoinVertical(lipgloss.Left, panels, bar)

	if a.confirmDialog != nil {
		// Overlay the confirm dialog centered
		content = content + "\n" + a.confirmDialog.View()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (a *App) resizePanels() {
	pipelineW := a.width * 25 / 100
	jobsW := a.width * 25 / 100
	detailW := a.width - pipelineW - jobsW
	panelH := a.height - 2 // status bar height

	a.pipelinesPanel.Width = pipelineW
	a.pipelinesPanel.Height = panelH
	a.jobsPanel.Width = jobsW
	a.jobsPanel.Height = panelH
	a.detailPanel.Width = detailW
	a.detailPanel.Height = panelH
	a.statusBar.Width = a.width
	a.statusBar.SetProvider(a.ProviderLabel())
	a.updatePanelFocus()
}

func (a App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case KeyQ, KeyCtrlC:
		return a, tea.Quit

	// Panel navigation
	case KeyTab, KeyL:
		a.FocusNext()
	case KeyShiftTab, KeyH:
		a.FocusPrev()

	// List navigation
	case KeyJ, KeyDown:
		a.handleListDown()
		return a, a.handleSelectionChange()
	case KeyK, KeyUp:
		a.handleListUp()
		return a, a.handleSelectionChange()
	case KeyG:
		a.handleListGoToTop()
		return a, a.handleSelectionChange()
	case KeyShiftG:
		a.handleListGoToBottom()
		return a, a.handleSelectionChange()
	case KeyEnter:
		a.FocusNext()
		return a, a.handleSelectionChange()

	case KeyEsc:
		a.FocusPrev()

	// Actions
	case KeyR:
		return a, a.handleRetry()
	case KeyC:
		return a, a.handleCancel()
	case KeyO:
		return a, a.handleOpenBrowser()
	case KeyY:
		return a, a.handleYankURL()
	case KeyShiftR:
		a.poller.RequestRefresh()
		return a, a.fetchPipelines()
	}

	return a, nil
}

func (a *App) handleListDown() {
	switch a.focusedPanel {
	case PanelPipelines:
		a.pipelinesPanel.MoveDown()
	case PanelJobs:
		a.jobsPanel.MoveDown()
	case PanelDetail:
		a.detailPanel.ScrollDown()
	}
}

func (a *App) handleListUp() {
	switch a.focusedPanel {
	case PanelPipelines:
		a.pipelinesPanel.MoveUp()
	case PanelJobs:
		a.jobsPanel.MoveUp()
	case PanelDetail:
		a.detailPanel.ScrollUp()
	}
}

func (a *App) handleListGoToTop() {
	switch a.focusedPanel {
	case PanelPipelines:
		a.pipelinesPanel.GoToTop()
	case PanelJobs:
		a.jobsPanel.GoToTop()
	}
}

func (a *App) handleListGoToBottom() {
	switch a.focusedPanel {
	case PanelPipelines:
		a.pipelinesPanel.GoToBottom()
	case PanelJobs:
		a.jobsPanel.GoToBottom()
	}
}

func (a App) handleSelectionChange() tea.Cmd {
	switch a.focusedPanel {
	case PanelPipelines:
		return a.fetchJobsForSelected()
	case PanelJobs:
		return a.fetchStepsForSelected()
	}
	return nil
}

func (a App) handleConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		a.confirmDialog.Confirm()
		action := a.confirmDialog.Action
		a.confirmDialog = nil
		return a, a.executeAction(action)
	case "n", KeyEsc:
		a.confirmDialog.Deny()
		a.confirmDialog = nil
	}
	return a, nil
}

func (a *App) handleRetry() tea.Cmd {
	if a.confirmActions {
		dialog := confirm.New("Retry this pipeline?", "retry")
		a.confirmDialog = &dialog
		return nil
	}
	return a.executeAction("retry")
}

func (a *App) handleCancel() tea.Cmd {
	if a.confirmActions {
		dialog := confirm.New("Cancel this pipeline?", "cancel")
		a.confirmDialog = &dialog
		return nil
	}
	return a.executeAction("cancel")
}

func (a App) handleOpenBrowser() tea.Cmd {
	if p, ok := a.pipelinesPanel.Selected(); ok && p.WebURL != "" {
		return tea.ExecProcess(openBrowserCmd(p.WebURL), func(err error) tea.Msg {
			return nil
		})
	}
	return nil
}

func (a App) handleYankURL() tea.Cmd {
	if p, ok := a.pipelinesPanel.Selected(); ok && p.WebURL != "" {
		return tea.SetClipboard(p.WebURL)
	}
	return nil
}

func (a App) executeAction(action string) tea.Cmd {
	p, ok := a.pipelinesPanel.Selected()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		ctx := context.Background()
		var err error
		switch action {
		case "retry":
			if a.focusedPanel == PanelJobs {
				if j, ok := a.jobsPanel.Selected(); ok {
					err = a.provider.RetryJob(ctx, j.ID)
				}
			} else {
				err = a.provider.RetryPipeline(ctx, p.ID)
			}
		case "cancel":
			if a.focusedPanel == PanelJobs {
				if j, ok := a.jobsPanel.Selected(); ok {
					err = a.provider.CancelJob(ctx, j.ID)
				}
			} else {
				err = a.provider.CancelPipeline(ctx, p.ID)
			}
		}
		return ActionResultMsg{Err: err, Action: action}
	}
}

func (a App) fetchPipelines() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		pipelines, err := a.provider.ListPipelines(ctx, a.pipelineLimit)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return PipelinesMsg{Pipelines: pipelines}
	}
}

func (a App) fetchJobsForSelected() tea.Cmd {
	p, ok := a.pipelinesPanel.Selected()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		ctx := context.Background()
		jobs, err := a.provider.GetJobs(ctx, p.ID)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return JobsMsg{Jobs: jobs}
	}
}

func (a App) fetchStepsForSelected() tea.Cmd {
	j, ok := a.jobsPanel.Selected()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		ctx := context.Background()
		steps, err := a.provider.GetSteps(ctx, j.ID)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return StepsMsg{Steps: steps}
	}
}

func (a App) fetchLog() tea.Cmd {
	j, ok := a.jobsPanel.Selected()
	if !ok {
		return nil
	}
	offset := a.detailPanel.LogOffset()
	return func() tea.Msg {
		ctx := context.Background()
		content, newOffset, err := a.provider.GetLog(ctx, j.ID, offset)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return LogMsg{Content: content, NewOffset: newOffset}
	}
}

func openBrowserCmd(url string) *tea.ExecCommand {
	return &tea.ExecCommand{
		Command: "xdg-open",
		Args:    []string{url},
	}
}
```

Note: `openBrowserCmd` uses `xdg-open` for Linux. For cross-platform, we can add `open` (macOS) and `start` (Windows) detection later, but for the MVP this suffices.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/tui/ -v -run TestNewApp`
Expected: PASS

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/tui/ -v -run TestFocusCycle`
Expected: PASS

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/tui/ -v -run TestProviderLabel`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat: add root TUI app model with panel layout and actions"
```

---

### Task 15: GitHub Provider

**Files:**
- Create: `internal/provider/github/github.go`
- Create: `internal/provider/github/github_test.go`

- [ ] **Step 1: Write GitHub provider tests**

```go
// internal/provider/github/github_test.go
package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/steven/manifold/internal/provider"
)

func TestListPipelines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/user/repo/actions/runs" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("per_page") != "5" {
			t.Errorf("per_page: got %q, want %q", r.URL.Query().Get("per_page"), "5")
		}
		resp := workflowRunsResponse{
			WorkflowRuns: []workflowRun{
				{
					ID:         1001,
					Name:       "CI",
					HeadBranch: "main",
					HeadSHA:    "abc1234567890",
					Status:     "completed",
					Conclusion: strPtr("success"),
					HTMLURL:    "https://github.com/user/repo/actions/runs/1001",
					RunStartedAt: "2026-03-26T10:00:00Z",
					Actor: actor{Login: "testuser"},
					HeadCommit: headCommit{Message: "fix: something"},
				},
				{
					ID:         1002,
					Name:       "CI",
					HeadBranch: "feature",
					HeadSHA:    "def567890abcd",
					Status:     "in_progress",
					HTMLURL:    "https://github.com/user/repo/actions/runs/1002",
					RunStartedAt: "2026-03-26T10:05:00Z",
					Actor: actor{Login: "testuser"},
					HeadCommit: headCommit{Message: "feat: new thing"},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := New("token123", "user", "repo", server.URL)
	pipelines, err := p.ListPipelines(context.Background(), 5)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(pipelines) != 2 {
		t.Fatalf("got %d pipelines, want 2", len(pipelines))
	}

	first := pipelines[0]
	if first.ID != "1001" {
		t.Errorf("ID: got %q, want %q", first.ID, "1001")
	}
	if first.Ref != "main" {
		t.Errorf("ref: got %q, want %q", first.Ref, "main")
	}
	if first.Commit != "abc1234" {
		t.Errorf("commit: got %q, want %q", first.Commit, "abc1234")
	}
	if first.Status != provider.StatusSuccess {
		t.Errorf("status: got %q, want %q", first.Status, provider.StatusSuccess)
	}
	if first.Author != "testuser" {
		t.Errorf("author: got %q", first.Author)
	}

	second := pipelines[1]
	if second.Status != provider.StatusRunning {
		t.Errorf("status: got %q, want %q", second.Status, provider.StatusRunning)
	}
}

func TestGetJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := jobsResponse{
			Jobs: []ghJob{
				{
					ID:         2001,
					Name:       "build",
					Status:     "completed",
					Conclusion: strPtr("success"),
					StartedAt:  "2026-03-26T10:00:00Z",
					CompletedAt: strPtr("2026-03-26T10:01:30Z"),
					HTMLURL:    "https://github.com/user/repo/actions/runs/1001/jobs/2001",
				},
				{
					ID:         2002,
					Name:       "test",
					Status:     "completed",
					Conclusion: strPtr("failure"),
					StartedAt:  "2026-03-26T10:01:30Z",
					CompletedAt: strPtr("2026-03-26T10:03:00Z"),
					HTMLURL:    "https://github.com/user/repo/actions/runs/1001/jobs/2002",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := New("token123", "user", "repo", server.URL)
	jobs, err := p.GetJobs(context.Background(), "1001")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("got %d jobs, want 2", len(jobs))
	}
	if jobs[0].Name != "build" || jobs[0].Status != provider.StatusSuccess {
		t.Errorf("job 0: %+v", jobs[0])
	}
	if jobs[1].Status != provider.StatusFailed {
		t.Errorf("job 1 status: got %q, want %q", jobs[1].Status, provider.StatusFailed)
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
		got := mapStatus(tt.status, tt.conclusion)
		if got != tt.want {
			concl := "<nil>"
			if tt.conclusion != nil {
				concl = *tt.conclusion
			}
			t.Errorf("mapStatus(%q, %q): got %q, want %q", tt.status, concl, got, tt.want)
		}
	}
}

func strPtr(s string) *string { return &s }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/provider/github/ -v`
Expected: FAIL

- [ ] **Step 3: Write GitHub provider implementation**

```go
// internal/provider/github/github.go
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

// GitHub implements provider.Provider for GitHub Actions.
type GitHub struct {
	token   string
	owner   string
	repo    string
	baseURL string
	client  *http.Client
}

// New creates a new GitHub provider.
func New(token, owner, repo, baseURL string) *GitHub {
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &GitHub{
		token:   token,
		owner:   owner,
		repo:    repo,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (g *GitHub) Name() string { return "github" }

func (g *GitHub) ListPipelines(ctx context.Context, limit int) ([]provider.Pipeline, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs?per_page=%d", g.baseURL, g.owner, g.repo, limit)
	var resp workflowRunsResponse
	if err := g.get(ctx, url, &resp); err != nil {
		return nil, err
	}

	pipelines := make([]provider.Pipeline, 0, len(resp.WorkflowRuns))
	for _, r := range resp.WorkflowRuns {
		started, _ := time.Parse(time.RFC3339, r.RunStartedAt)
		p := provider.Pipeline{
			ID:        strconv.FormatInt(r.ID, 10),
			Ref:       r.HeadBranch,
			Commit:    r.HeadSHA[:min(7, len(r.HeadSHA))],
			Message:   r.HeadCommit.Message,
			Author:    r.Actor.Login,
			Status:    mapStatus(r.Status, r.Conclusion),
			StartedAt: started,
			WebURL:    r.HTMLURL,
		}
		if !started.IsZero() {
			p.Duration = time.Since(started)
		}
		pipelines = append(pipelines, p)
	}
	return pipelines, nil
}

func (g *GitHub) GetJobs(ctx context.Context, pipelineID string) ([]provider.Job, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%s/jobs", g.baseURL, g.owner, g.repo, pipelineID)
	var resp jobsResponse
	if err := g.get(ctx, url, &resp); err != nil {
		return nil, err
	}

	jobs := make([]provider.Job, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		started, _ := time.Parse(time.RFC3339, j.StartedAt)
		var dur time.Duration
		if j.CompletedAt != nil {
			completed, _ := time.Parse(time.RFC3339, *j.CompletedAt)
			if !completed.IsZero() && !started.IsZero() {
				dur = completed.Sub(started)
			}
		} else if !started.IsZero() {
			dur = time.Since(started)
		}

		jobs = append(jobs, provider.Job{
			ID:        strconv.FormatInt(j.ID, 10),
			Name:      j.Name,
			Status:    mapStatus(j.Status, j.Conclusion),
			StartedAt: started,
			Duration:  dur,
			WebURL:    j.HTMLURL,
		})
	}
	return jobs, nil
}

func (g *GitHub) GetSteps(ctx context.Context, jobID string) ([]provider.Step, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/jobs/%s", g.baseURL, g.owner, g.repo, jobID)
	var resp ghJobDetail
	if err := g.get(ctx, url, &resp); err != nil {
		return nil, err
	}

	steps := make([]provider.Step, 0, len(resp.Steps))
	for _, s := range resp.Steps {
		var dur time.Duration
		if s.StartedAt != "" && s.CompletedAt != nil {
			started, _ := time.Parse(time.RFC3339, s.StartedAt)
			completed, _ := time.Parse(time.RFC3339, *s.CompletedAt)
			if !started.IsZero() && !completed.IsZero() {
				dur = completed.Sub(started)
			}
		}
		steps = append(steps, provider.Step{
			Name:     s.Name,
			Status:   mapStatus(s.Status, s.Conclusion),
			Duration: dur,
		})
	}
	return steps, nil
}

func (g *GitHub) GetLog(ctx context.Context, jobID string, offset int) (string, int, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/jobs/%s/logs", g.baseURL, g.owner, g.repo, jobID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

	if resp.StatusCode != http.StatusOK {
		return "", offset, fmt.Errorf("GitHub API error: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", offset, err
	}

	content := string(body)
	if offset >= len(content) {
		return "", offset, nil
	}

	newContent := content[offset:]
	return newContent, len(content), nil
}

func (g *GitHub) RetryPipeline(ctx context.Context, pipelineID string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%s/rerun", g.baseURL, g.owner, g.repo, pipelineID)
	return g.post(ctx, url)
}

func (g *GitHub) RetryJob(ctx context.Context, jobID string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/jobs/%s/rerun", g.baseURL, g.owner, g.repo, jobID)
	return g.post(ctx, url)
}

func (g *GitHub) CancelPipeline(ctx context.Context, pipelineID string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%s/cancel", g.baseURL, g.owner, g.repo, pipelineID)
	return g.post(ctx, url)
}

func (g *GitHub) CancelJob(_ context.Context, _ string) error {
	return provider.ErrNotSupported
}

func (g *GitHub) get(ctx context.Context, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API error: %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func (g *GitHub) post(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
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
		return fmt.Errorf("GitHub API error: %s", resp.Status)
	}
	return nil
}

// API response types

type workflowRunsResponse struct {
	WorkflowRuns []workflowRun `json:"workflow_runs"`
}

type workflowRun struct {
	ID           int64       `json:"id"`
	Name         string      `json:"name"`
	HeadBranch   string      `json:"head_branch"`
	HeadSHA      string      `json:"head_sha"`
	Status       string      `json:"status"`
	Conclusion   *string     `json:"conclusion"`
	HTMLURL      string      `json:"html_url"`
	RunStartedAt string      `json:"run_started_at"`
	Actor        actor       `json:"actor"`
	HeadCommit   headCommit  `json:"head_commit"`
}

type actor struct {
	Login string `json:"login"`
}

type headCommit struct {
	Message string `json:"message"`
}

type jobsResponse struct {
	Jobs []ghJob `json:"jobs"`
}

type ghJob struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	Conclusion  *string `json:"conclusion"`
	StartedAt   string  `json:"started_at"`
	CompletedAt *string `json:"completed_at"`
	HTMLURL     string  `json:"html_url"`
}

type ghJobDetail struct {
	Steps []ghStep `json:"steps"`
}

type ghStep struct {
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	Conclusion  *string `json:"conclusion"`
	StartedAt   string  `json:"started_at"`
	CompletedAt *string `json:"completed_at"`
}

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
	case "waiting", "pending", "requested":
		return provider.StatusPending
	default:
		return provider.StatusPending
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/provider/github/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/provider/github/
git commit -m "feat: add GitHub Actions provider implementation"
```

---

### Task 16: GitLab Provider

**Files:**
- Create: `internal/provider/gitlab/gitlab.go`
- Create: `internal/provider/gitlab/gitlab_test.go`

- [ ] **Step 1: Write GitLab provider tests**

```go
// internal/provider/gitlab/gitlab_test.go
package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/steven/manifold/internal/provider"
)

func TestListPipelines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/user%2Frepo/pipelines" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := []glPipeline{
			{
				ID:        101,
				Ref:       "main",
				SHA:       "abc1234567890",
				Status:    "success",
				WebURL:    "https://gitlab.com/user/repo/-/pipelines/101",
				CreatedAt: "2026-03-26T10:00:00Z",
				User:      glUser{Username: "testuser"},
			},
			{
				ID:        102,
				Ref:       "feature",
				SHA:       "def567890abcd",
				Status:    "running",
				WebURL:    "https://gitlab.com/user/repo/-/pipelines/102",
				CreatedAt: "2026-03-26T10:05:00Z",
				User:      glUser{Username: "testuser"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := New("token123", "user", "repo", server.URL)
	pipelines, err := p.ListPipelines(context.Background(), 5)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(pipelines) != 2 {
		t.Fatalf("got %d pipelines, want 2", len(pipelines))
	}
	if pipelines[0].Status != provider.StatusSuccess {
		t.Errorf("status: got %q, want %q", pipelines[0].Status, provider.StatusSuccess)
	}
	if pipelines[1].Status != provider.StatusRunning {
		t.Errorf("status: got %q, want %q", pipelines[1].Status, provider.StatusRunning)
	}
	if pipelines[0].Commit != "abc1234" {
		t.Errorf("commit: got %q", pipelines[0].Commit)
	}
}

func TestStatusMapping(t *testing.T) {
	tests := []struct {
		status string
		want   provider.PipelineStatus
	}{
		{"created", provider.StatusPending},
		{"waiting_for_resource", provider.StatusQueued},
		{"preparing", provider.StatusPending},
		{"pending", provider.StatusPending},
		{"running", provider.StatusRunning},
		{"success", provider.StatusSuccess},
		{"failed", provider.StatusFailed},
		{"canceled", provider.StatusCanceled},
		{"skipped", provider.StatusSkipped},
		{"manual", provider.StatusPending},
		{"scheduled", provider.StatusQueued},
	}
	for _, tt := range tests {
		got := mapStatus(tt.status)
		if got != tt.want {
			t.Errorf("mapStatus(%q): got %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestGetJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := []glJob{
			{
				ID:         201,
				Name:       "build",
				Status:     "success",
				StartedAt:  strPtr("2026-03-26T10:00:00Z"),
				FinishedAt: strPtr("2026-03-26T10:01:30Z"),
				WebURL:     "https://gitlab.com/user/repo/-/jobs/201",
			},
			{
				ID:         202,
				Name:       "test",
				Status:     "failed",
				StartedAt:  strPtr("2026-03-26T10:01:30Z"),
				FinishedAt: strPtr("2026-03-26T10:03:00Z"),
				WebURL:     "https://gitlab.com/user/repo/-/jobs/202",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := New("token123", "user", "repo", server.URL)
	jobs, err := p.GetJobs(context.Background(), "101")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("got %d jobs, want 2", len(jobs))
	}
	if jobs[0].Status != provider.StatusSuccess {
		t.Errorf("job 0 status: got %q", jobs[0].Status)
	}
	if jobs[1].Status != provider.StatusFailed {
		t.Errorf("job 1 status: got %q", jobs[1].Status)
	}
}

func strPtr(s string) *string { return &s }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/provider/gitlab/ -v`
Expected: FAIL

- [ ] **Step 3: Write GitLab provider implementation**

```go
// internal/provider/gitlab/gitlab.go
package gitlab

import (
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

// GitLab implements provider.Provider for GitLab CI.
type GitLab struct {
	token   string
	owner   string
	repo    string
	baseURL string
	client  *http.Client
}

// New creates a new GitLab provider.
func New(token, owner, repo, baseURL string) *GitLab {
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}
	return &GitLab{
		token:   token,
		owner:   owner,
		repo:    repo,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (g *GitLab) Name() string { return "gitlab" }

func (g *GitLab) projectPath() string {
	return url.PathEscape(g.owner + "/" + g.repo)
}

func (g *GitLab) ListPipelines(ctx context.Context, limit int) ([]provider.Pipeline, error) {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/pipelines?per_page=%d", g.baseURL, g.projectPath(), limit)
	var resp []glPipeline
	if err := g.get(ctx, apiURL, &resp); err != nil {
		return nil, err
	}

	pipelines := make([]provider.Pipeline, 0, len(resp))
	for _, p := range resp {
		created, _ := time.Parse(time.RFC3339, p.CreatedAt)
		pipelines = append(pipelines, provider.Pipeline{
			ID:        strconv.Itoa(p.ID),
			Ref:       p.Ref,
			Commit:    p.SHA[:min(7, len(p.SHA))],
			Author:    p.User.Username,
			Status:    mapStatus(p.Status),
			StartedAt: created,
			WebURL:    p.WebURL,
			Duration:  time.Since(created),
		})
	}
	return pipelines, nil
}

func (g *GitLab) GetJobs(ctx context.Context, pipelineID string) ([]provider.Job, error) {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/pipelines/%s/jobs", g.baseURL, g.projectPath(), pipelineID)
	var resp []glJob
	if err := g.get(ctx, apiURL, &resp); err != nil {
		return nil, err
	}

	jobs := make([]provider.Job, 0, len(resp))
	for _, j := range resp {
		var started time.Time
		var dur time.Duration
		if j.StartedAt != nil {
			started, _ = time.Parse(time.RFC3339, *j.StartedAt)
		}
		if j.StartedAt != nil && j.FinishedAt != nil {
			s, _ := time.Parse(time.RFC3339, *j.StartedAt)
			f, _ := time.Parse(time.RFC3339, *j.FinishedAt)
			if !s.IsZero() && !f.IsZero() {
				dur = f.Sub(s)
			}
		} else if !started.IsZero() {
			dur = time.Since(started)
		}

		jobs = append(jobs, provider.Job{
			ID:        strconv.Itoa(j.ID),
			Name:      j.Name,
			Status:    mapStatus(j.Status),
			StartedAt: started,
			Duration:  dur,
			WebURL:    j.WebURL,
		})
	}
	return jobs, nil
}

func (g *GitLab) GetSteps(ctx context.Context, jobID string) ([]provider.Step, error) {
	// GitLab doesn't have native steps — parse from log section headers
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/jobs/%s/trace", g.baseURL, g.projectPath(), jobID)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", g.token)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseLogSections(string(body)), nil
}

func (g *GitLab) GetLog(ctx context.Context, jobID string, offset int) (string, int, error) {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/jobs/%s/trace", g.baseURL, g.projectPath(), jobID)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", offset, err
	}
	req.Header.Set("PRIVATE-TOKEN", g.token)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))

	resp, err := g.client.Do(req)
	if err != nil {
		return "", offset, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", offset, err
	}

	return string(body), offset + len(body), nil
}

func (g *GitLab) RetryPipeline(ctx context.Context, pipelineID string) error {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/pipelines/%s/retry", g.baseURL, g.projectPath(), pipelineID)
	return g.post(ctx, apiURL)
}

func (g *GitLab) RetryJob(ctx context.Context, jobID string) error {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/jobs/%s/retry", g.baseURL, g.projectPath(), jobID)
	return g.post(ctx, apiURL)
}

func (g *GitLab) CancelPipeline(ctx context.Context, pipelineID string) error {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/pipelines/%s/cancel", g.baseURL, g.projectPath(), pipelineID)
	return g.post(ctx, apiURL)
}

func (g *GitLab) CancelJob(ctx context.Context, jobID string) error {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/jobs/%s/cancel", g.baseURL, g.projectPath(), jobID)
	return g.post(ctx, apiURL)
}

func (g *GitLab) get(ctx context.Context, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", g.token)

	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitLab API error: %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func (g *GitLab) post(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
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
		return fmt.Errorf("GitLab API error: %s", resp.Status)
	}
	return nil
}

// parseLogSections extracts step-like sections from GitLab job logs.
// GitLab uses ANSI section markers: section_start:timestamp:name\r and section_end:timestamp:name\r
func parseLogSections(log string) []provider.Step {
	var steps []provider.Step
	lines := strings.Split(log, "\n")
	lineNum := 0

	for i, line := range lines {
		if strings.Contains(line, "section_start:") {
			parts := strings.SplitN(line, ":", 3)
			if len(parts) >= 3 {
				namePart := strings.SplitN(parts[2], "\r", 2)
				name := namePart[0]
				steps = append(steps, provider.Step{
					Name:     name,
					Status:   provider.StatusRunning,
					LogStart: i,
				})
			}
		}
		if strings.Contains(line, "section_end:") && len(steps) > 0 {
			steps[len(steps)-1].LogEnd = i
			steps[len(steps)-1].Status = provider.StatusSuccess
		}
		lineNum++
	}

	return steps
}

// API response types

type glPipeline struct {
	ID        int    `json:"id"`
	Ref       string `json:"ref"`
	SHA       string `json:"sha"`
	Status    string `json:"status"`
	WebURL    string `json:"web_url"`
	CreatedAt string `json:"created_at"`
	User      glUser `json:"user"`
}

type glUser struct {
	Username string `json:"username"`
}

type glJob struct {
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	StartedAt  *string `json:"started_at"`
	FinishedAt *string `json:"finished_at"`
	WebURL     string  `json:"web_url"`
}

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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/provider/gitlab/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/provider/gitlab/
git commit -m "feat: add GitLab CI provider implementation"
```

---

### Task 17: Bitbucket Provider

**Files:**
- Create: `internal/provider/bitbucket/bitbucket.go`
- Create: `internal/provider/bitbucket/bitbucket_test.go`

- [ ] **Step 1: Write Bitbucket provider tests**

```go
// internal/provider/bitbucket/bitbucket_test.go
package bitbucket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/steven/manifold/internal/provider"
)

func TestListPipelines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/repositories/team/repo/pipelines/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := pipelinesResponse{
			Values: []bbPipeline{
				{
					UUID:      "{uuid-1}",
					State:     bbState{Name: "COMPLETED", Result: &bbResult{Name: "SUCCESSFUL"}},
					Target:    bbTarget{RefName: "main", Commit: bbCommit{Hash: "abc1234567890", Message: "fix: stuff"}},
					CreatedOn: "2026-03-26T10:00:00.000000+00:00",
					Creator:   bbCreator{DisplayName: "testuser"},
				},
				{
					UUID:      "{uuid-2}",
					State:     bbState{Name: "IN_PROGRESS"},
					Target:    bbTarget{RefName: "feature", Commit: bbCommit{Hash: "def567890abcd", Message: "feat: new"}},
					CreatedOn: "2026-03-26T10:05:00.000000+00:00",
					Creator:   bbCreator{DisplayName: "testuser"},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := New("token123", "", "team", "repo", server.URL)
	pipelines, err := p.ListPipelines(context.Background(), 5)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(pipelines) != 2 {
		t.Fatalf("got %d pipelines, want 2", len(pipelines))
	}
	if pipelines[0].Status != provider.StatusSuccess {
		t.Errorf("status: got %q", pipelines[0].Status)
	}
	if pipelines[1].Status != provider.StatusRunning {
		t.Errorf("status: got %q", pipelines[1].Status)
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
		var result *bbResult
		if tt.result != "" {
			result = &bbResult{Name: tt.result}
		}
		got := mapStatus(bbState{Name: tt.state, Result: result})
		if got != tt.want {
			t.Errorf("mapStatus(%q/%q): got %q, want %q", tt.state, tt.result, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/provider/bitbucket/ -v`
Expected: FAIL

- [ ] **Step 3: Write Bitbucket provider implementation**

```go
// internal/provider/bitbucket/bitbucket.go
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

// Bitbucket implements provider.Provider for Bitbucket Pipelines.
type Bitbucket struct {
	token    string
	username string
	owner    string
	repo     string
	baseURL  string
	client   *http.Client
}

// New creates a new Bitbucket provider.
func New(token, username, owner, repo, baseURL string) *Bitbucket {
	if baseURL == "" {
		baseURL = "https://api.bitbucket.org"
	}
	return &Bitbucket{
		token:    token,
		username: username,
		owner:    owner,
		repo:     repo,
		baseURL:  baseURL,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *Bitbucket) Name() string { return "bitbucket" }

func (b *Bitbucket) ListPipelines(ctx context.Context, limit int) ([]provider.Pipeline, error) {
	apiURL := fmt.Sprintf("%s/2.0/repositories/%s/%s/pipelines/?pagelen=%d&sort=-created_on", b.baseURL, b.owner, b.repo, limit)
	var resp pipelinesResponse
	if err := b.get(ctx, apiURL, &resp); err != nil {
		return nil, err
	}

	pipelines := make([]provider.Pipeline, 0, len(resp.Values))
	for _, p := range resp.Values {
		created, _ := time.Parse("2006-01-02T15:04:05.000000+00:00", p.CreatedOn)
		sha := p.Target.Commit.Hash
		if len(sha) > 7 {
			sha = sha[:7]
		}
		pipelines = append(pipelines, provider.Pipeline{
			ID:        strings.Trim(p.UUID, "{}"),
			Ref:       p.Target.RefName,
			Commit:    sha,
			Message:   p.Target.Commit.Message,
			Author:    p.Creator.DisplayName,
			Status:    mapStatus(p.State),
			StartedAt: created,
			Duration:  time.Since(created),
			WebURL:    fmt.Sprintf("https://bitbucket.org/%s/%s/pipelines/results/%s", b.owner, b.repo, strings.Trim(p.UUID, "{}")),
		})
	}
	return pipelines, nil
}

func (b *Bitbucket) GetJobs(ctx context.Context, pipelineID string) ([]provider.Job, error) {
	apiURL := fmt.Sprintf("%s/2.0/repositories/%s/%s/pipelines/%s/steps/", b.baseURL, b.owner, b.repo, pipelineID)
	var resp stepsResponse
	if err := b.get(ctx, apiURL, &resp); err != nil {
		return nil, err
	}

	jobs := make([]provider.Job, 0, len(resp.Values))
	for _, s := range resp.Values {
		var started time.Time
		var dur time.Duration
		if s.StartedOn != "" {
			started, _ = time.Parse("2006-01-02T15:04:05.000000+00:00", s.StartedOn)
		}
		if s.CompletedOn != "" && !started.IsZero() {
			completed, _ := time.Parse("2006-01-02T15:04:05.000000+00:00", s.CompletedOn)
			dur = completed.Sub(started)
		} else if !started.IsZero() {
			dur = time.Since(started)
		}

		jobs = append(jobs, provider.Job{
			ID:        strings.Trim(s.UUID, "{}"),
			Name:      s.Name,
			Status:    mapStatus(s.State),
			StartedAt: started,
			Duration:  dur,
		})
	}
	return jobs, nil
}

func (b *Bitbucket) GetSteps(_ context.Context, _ string) ([]provider.Step, error) {
	// Bitbucket steps (what we call jobs) don't have sub-steps
	return nil, nil
}

func (b *Bitbucket) GetLog(ctx context.Context, jobID string, offset int) (string, int, error) {
	// jobID is the step UUID in Bitbucket
	// Need to find the pipeline UUID — for now we store it in the job ID as "pipelineID/stepID"
	parts := strings.SplitN(jobID, "/", 2)
	if len(parts) != 2 {
		return "", offset, fmt.Errorf("invalid job ID format, expected pipelineID/stepID: %s", jobID)
	}
	pipelineID, stepID := parts[0], parts[1]

	apiURL := fmt.Sprintf("%s/2.0/repositories/%s/%s/pipelines/%s/steps/%s/log", b.baseURL, b.owner, b.repo, pipelineID, stepID)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", offset, err
	}
	b.setAuth(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return "", offset, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", offset, err
	}

	content := string(body)
	if offset >= len(content) {
		return "", offset, nil
	}
	return content[offset:], len(content), nil
}

func (b *Bitbucket) RetryPipeline(_ context.Context, _ string) error {
	return provider.ErrNotSupported
}

func (b *Bitbucket) RetryJob(_ context.Context, _ string) error {
	return provider.ErrNotSupported
}

func (b *Bitbucket) CancelPipeline(ctx context.Context, pipelineID string) error {
	apiURL := fmt.Sprintf("%s/2.0/repositories/%s/%s/pipelines/%s/stopPipeline", b.baseURL, b.owner, b.repo, pipelineID)
	return b.post(ctx, apiURL)
}

func (b *Bitbucket) CancelJob(_ context.Context, _ string) error {
	return provider.ErrNotSupported
}

func (b *Bitbucket) get(ctx context.Context, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	b.setAuth(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Bitbucket API error: %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func (b *Bitbucket) post(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
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
		return fmt.Errorf("Bitbucket API error: %s", resp.Status)
	}
	return nil
}

func (b *Bitbucket) setAuth(req *http.Request) {
	if b.username != "" {
		req.SetBasicAuth(b.username, b.token)
	} else {
		req.Header.Set("Authorization", "Bearer "+b.token)
	}
}

// API response types

type pipelinesResponse struct {
	Values []bbPipeline `json:"values"`
}

type bbPipeline struct {
	UUID      string    `json:"uuid"`
	State     bbState   `json:"state"`
	Target    bbTarget  `json:"target"`
	CreatedOn string    `json:"created_on"`
	Creator   bbCreator `json:"creator"`
}

type bbState struct {
	Name   string    `json:"name"`
	Result *bbResult `json:"result"`
}

type bbResult struct {
	Name string `json:"name"`
}

type bbTarget struct {
	RefName string   `json:"ref_name"`
	Commit  bbCommit `json:"commit"`
}

type bbCommit struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
}

type bbCreator struct {
	DisplayName string `json:"display_name"`
}

type stepsResponse struct {
	Values []bbStep `json:"values"`
}

type bbStep struct {
	UUID        string  `json:"uuid"`
	Name        string  `json:"name"`
	State       bbState `json:"state"`
	StartedOn   string  `json:"started_on"`
	CompletedOn string  `json:"completed_on"`
}

func mapStatus(state bbState) provider.PipelineStatus {
	switch state.Name {
	case "PENDING":
		return provider.StatusPending
	case "IN_PROGRESS":
		return provider.StatusRunning
	case "COMPLETED":
		if state.Result != nil {
			switch state.Result.Name {
			case "SUCCESSFUL":
				return provider.StatusSuccess
			case "FAILED", "ERROR":
				return provider.StatusFailed
			case "STOPPED":
				return provider.StatusCanceled
			}
		}
		return provider.StatusSuccess
	case "HALTED":
		return provider.StatusCanceled
	default:
		return provider.StatusPending
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./internal/provider/bitbucket/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/provider/bitbucket/
git commit -m "feat: add Bitbucket Pipelines provider implementation"
```

---

### Task 18: Wire Everything in main.go

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Write the complete main.go wiring**

```go
// main.go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	tea "charm.land/bubbletea/v2"

	"github.com/steven/manifold/internal/config"
	"github.com/steven/manifold/internal/poller"
	"github.com/steven/manifold/internal/provider"
	"github.com/steven/manifold/internal/provider/bitbucket"
	gh "github.com/steven/manifold/internal/provider/github"
	gl "github.com/steven/manifold/internal/provider/gitlab"
	"github.com/steven/manifold/internal/tui"
	"github.com/steven/manifold/internal/tui/selector"
)

var version = "dev"

type CLI struct {
	Dir     string `short:"C" help:"Path to git repository." default:"." type:"path"`
	Remote  string `short:"r" help:"Git remote to use." default:""`
	Version kong.VersionFlag `short:"v" help:"Print version."`
}

func main() {
	var cli CLI
	kong.Parse(&cli,
		kong.Name("manifold"),
		kong.Description("Monitor CI/CD pipelines from the terminal."),
		kong.Vars{"version": version},
	)

	if err := run(cli); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(cli CLI) error {
	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// 2. Determine which remote to use
	remoteName, err := resolveRemote(cli.Dir, cli.Remote, cfg)
	if err != nil {
		return err
	}

	// 3. Get remote URL
	remoteURL, err := gitRemoteURL(cli.Dir, remoteName)
	if err != nil {
		return fmt.Errorf("getting remote URL: %w", err)
	}

	// 4. Detect provider
	extraHosts := provider.ExtraHostsFromConfig(cfg)
	detect, err := provider.DetectFromURL(remoteURL, extraHosts)
	if err != nil {
		return err
	}

	// 5. Resolve auth
	auth, err := provider.ResolveAuth(detect.ProviderType, detect.Host, cfg)
	if err != nil {
		return err
	}

	// 6. Create provider
	prov, err := createProvider(detect, auth)
	if err != nil {
		return err
	}

	// 7. Create poller
	poll := poller.New(
		time.Duration(cfg.Defaults.PollingActive)*time.Second,
		time.Duration(cfg.Defaults.PollingIdle)*time.Second,
	)

	// 8. Create and run TUI
	app := tui.NewApp(detect, cfg.Defaults.ConfirmActions, cfg.Defaults.PipelineLimit)
	app.SetProvider(prov)
	app.SetPoller(poll)

	p := tea.NewProgram(app)
	_, err = p.Run()
	return err
}

func resolveRemote(dir, flagRemote string, cfg config.Config) (string, error) {
	// Flag takes priority
	if flagRemote != "" {
		return flagRemote, nil
	}

	// Config default
	if cfg.Defaults.Remote != "" {
		return cfg.Defaults.Remote, nil
	}

	// List remotes
	remotes, err := listGitRemotes(dir)
	if err != nil {
		return "", fmt.Errorf("listing remotes: %w", err)
	}

	if len(remotes) == 0 {
		return "", fmt.Errorf("no git remotes found in %s", dir)
	}

	if len(remotes) == 1 {
		return remotes[0].Name, nil
	}

	// Multiple remotes, no default — show selector
	selectorModel := selector.New(remotes)
	p := tea.NewProgram(selectorModel)
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	sel, ok := finalModel.(selector.Model)
	if !ok {
		return "", fmt.Errorf("unexpected model type from selector")
	}

	selected, ok := sel.Selected()
	if !ok {
		return "", fmt.Errorf("no remote selected")
	}

	return selected.Name, nil
}

func createProvider(detect provider.DetectResult, auth provider.Auth) (provider.Provider, error) {
	switch detect.ProviderType {
	case "github":
		apiBase := "https://api.github.com"
		if detect.Host != "github.com" {
			apiBase = "https://" + detect.Host + "/api/v3"
		}
		return gh.New(auth.Token, detect.Owner, detect.Repo, apiBase), nil
	case "gitlab":
		apiBase := "https://" + detect.Host
		return gl.New(auth.Token, detect.Owner, detect.Repo, apiBase), nil
	case "bitbucket":
		return bitbucket.New(auth.Token, auth.Username, detect.Owner, detect.Repo, ""), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", detect.ProviderType)
	}
}

func gitRemoteURL(dir, remote string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", remote)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func listGitRemotes(dir string) ([]selector.Remote, error) {
	cmd := exec.Command("git", "-C", dir, "remote", "-v")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var remotes []selector.Remote
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		url := fields[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		remotes = append(remotes, selector.Remote{Name: name, URL: url})
	}
	return remotes, nil
}
```

- [ ] **Step 2: Update selector to implement tea.Model**

The selector needs `Init`, `Update`, `View` methods to work as a standalone Bubble Tea program:

Add to `internal/tui/selector/selector.go`:

```go
// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			m.MoveDown()
		case "k", "up":
			m.MoveUp()
		case "enter":
			m.Select()
			return m, tea.Quit
		}
	}
	return m, nil
}

// BubbleTeaView implements tea.Model (named differently to avoid conflict with the existing View).
func (m Model) View() tea.View {
	return tea.NewView(m.viewContent())
}
```

Rename the existing `View()` method to `viewContent() string` so the tea.Model `View()` can wrap it.

- [ ] **Step 3: Verify it compiles**

Run: `cd /home/steven/work/project-manifold/manifold && go build .`
Expected: success (binary created)

- [ ] **Step 4: Commit**

```bash
git add main.go internal/tui/selector/selector.go
git commit -m "feat: wire everything together in main.go"
```

---

### Task 19: Full Build Verification

- [ ] **Step 1: Run all tests**

Run: `cd /home/steven/work/project-manifold/manifold && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 2: Build the binary**

Run: `cd /home/steven/work/project-manifold/manifold && go build -o manifold .`
Expected: binary created

- [ ] **Step 3: Verify CLI flags work**

Run: `./manifold --help`
Expected: help with `-C`, `-r`, `-v` flags

Run: `./manifold --version`
Expected: `dev`

- [ ] **Step 4: Run vet and check for issues**

Run: `cd /home/steven/work/project-manifold/manifold && go vet ./...`
Expected: no issues

- [ ] **Step 5: Commit any fixes needed**

```bash
git add -A
git commit -m "fix: resolve build and test issues from integration"
```

---

### Task 20: Add .gitignore and cleanup

- [ ] **Step 1: Create .gitignore**

```gitignore
# Binary
manifold

# Go
*.exe
*.test
*.out

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db
```

- [ ] **Step 2: Commit**

```bash
git add .gitignore
git commit -m "chore: add .gitignore"
```
