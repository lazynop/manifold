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
	return Auth{}, fmt.Errorf("%s", authErrorMessage(providerType, host))
}

func tryCliAuth(providerType string) (Auth, bool) {
	// Allow tests to bypass CLI auth by setting MANIFOLD_SKIP_CLI_AUTH=1
	if os.Getenv("MANIFOLD_SKIP_CLI_AUTH") == "1" {
		return Auth{}, false
	}
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
