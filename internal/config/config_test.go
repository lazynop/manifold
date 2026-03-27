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
