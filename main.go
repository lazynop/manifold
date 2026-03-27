// main.go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/alecthomas/kong"
	"github.com/steven/manifold/internal/config"
	"github.com/steven/manifold/internal/poller"
	"github.com/steven/manifold/internal/provider"
	"github.com/steven/manifold/internal/provider/bitbucket"
	"github.com/steven/manifold/internal/provider/github"
	"github.com/steven/manifold/internal/provider/gitlab"
	"github.com/steven/manifold/internal/tui"
	"github.com/steven/manifold/internal/tui/selector"
)

var version = "dev"

type CLI struct {
	Dir     string          `short:"C" help:"Path to git repository." default:"." type:"path"`
	Remote  string          `short:"r" help:"Git remote to use." default:""`
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
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// run is the main application flow.
func run(cli CLI) error {
	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// 2. Resolve which remote to use
	remote, err := resolveRemote(cli.Dir, cli.Remote, cfg)
	if err != nil {
		return err
	}

	// 3. Get the remote URL
	remoteURL, err := gitRemoteURL(cli.Dir, remote)
	if err != nil {
		return fmt.Errorf("getting URL for remote %q: %w", remote, err)
	}

	// 4. Detect provider from URL
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
	activeInterval := time.Duration(cfg.Defaults.PollingActive) * time.Second
	idleInterval := time.Duration(cfg.Defaults.PollingIdle) * time.Second
	poll := poller.New(activeInterval, idleInterval)

	// 8. Create TUI app
	app := tui.NewApp(detect, cfg.Defaults.ConfirmActions, cfg.Defaults.PipelineLimit)
	app.SetProvider(prov)
	app.SetPoller(poll)

	// 9. Run the program
	p := tea.NewProgram(app)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}
	return nil
}

// resolveRemote determines which git remote to use in priority order:
// 1. CLI flag, 2. config default, 3. single remote (auto), 4. interactive selector.
func resolveRemote(dir, flagRemote string, cfg config.Config) (string, error) {
	// 1. Explicit flag wins
	if flagRemote != "" {
		return flagRemote, nil
	}

	// 2. Config default
	if cfg.Defaults.Remote != "" {
		return cfg.Defaults.Remote, nil
	}

	// 3. List remotes — if exactly one, use it automatically
	remotes, err := listGitRemotes(dir)
	if err != nil {
		return "", fmt.Errorf("listing git remotes: %w", err)
	}
	if len(remotes) == 0 {
		return "", fmt.Errorf("no git remotes found in %s", dir)
	}
	if len(remotes) == 1 {
		return remotes[0].Name, nil
	}

	// 4. Multiple remotes — show interactive selector
	return runSelectorTUI(remotes)
}

// runSelectorTUI runs the remote selector and returns the chosen remote name.
func runSelectorTUI(remotes []selector.Remote) (string, error) {
	m := selector.New(remotes)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("selector: %w", err)
	}
	sel, ok := finalModel.(selector.Model)
	if !ok {
		return "", fmt.Errorf("selector: unexpected model type")
	}
	chosen, ok := sel.Selected()
	if !ok {
		return "", fmt.Errorf("no remote selected")
	}
	return chosen.Name, nil
}

// createProvider instantiates the correct provider given detection and auth.
func createProvider(detect provider.DetectResult, auth provider.Auth) (provider.Provider, error) {
	switch detect.ProviderType {
	case "github":
		var apiBase string
		if detect.Host == "github.com" {
			apiBase = "https://api.github.com"
		} else {
			apiBase = "https://" + detect.Host + "/api/v3"
		}
		return github.New(auth.Token, detect.Owner, detect.Repo, apiBase), nil

	case "gitlab":
		apiBase := "https://" + detect.Host
		return gitlab.New(auth.Token, detect.Owner, detect.Repo, apiBase), nil

	case "bitbucket":
		return bitbucket.New(auth.Token, auth.Username, detect.Owner, detect.Repo, ""), nil

	default:
		return nil, fmt.Errorf("unsupported provider type: %s", detect.ProviderType)
	}
}

// gitRemoteURL returns the URL for a given git remote.
func gitRemoteURL(dir, remote string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", remote)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// listGitRemotes returns all remotes in the repository.
// It deduplicates by name (git remote -v shows both fetch and push URLs).
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
