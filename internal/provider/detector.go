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
