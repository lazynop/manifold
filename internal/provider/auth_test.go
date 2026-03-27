package provider

import (
	"os"
	"testing"

	"github.com/steven/manifold/internal/config"
)

func TestResolveAuthFromEnv(t *testing.T) {
	t.Setenv("MANIFOLD_SKIP_CLI_AUTH", "1")
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
	t.Setenv("MANIFOLD_SKIP_CLI_AUTH", "1")
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
	t.Setenv("MANIFOLD_SKIP_CLI_AUTH", "1")
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
