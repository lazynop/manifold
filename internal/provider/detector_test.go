package provider

import (
	"testing"

	"github.com/steven/manifold/internal/config"
)

func TestDetectFromURL(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		extraHosts   map[string]string
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
