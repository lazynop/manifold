# Manifold — CI Pipeline TUI Monitor

## Overview

Manifold is a terminal user interface (TUI) for monitoring and controlling CI/CD pipelines across GitHub Actions, GitLab CI, and Bitbucket Pipelines. Built in Go with Bubble Tea v2 (`charm.land/bubbletea/v2`) and Lipgloss v2 (`charm.land/lipgloss/v2`), it provides a unified, interactive experience similar to lazygit/lazydocker — but for CI.

### Key Dependencies

| Library | Module Path | Purpose |
|---------|------------|---------|
| Bubble Tea v2 | `charm.land/bubbletea/v2` | TUI framework |
| Lipgloss v2 | `charm.land/lipgloss/v2` | Styling and layout |
| Kong | `github.com/alecthomas/kong` | CLI flags (struct-based, declarative) |
| go-toml v2 | `github.com/pelletier/go-toml/v2` | TOML config parsing |

### Bubble Tea v2 Notes

Bubble Tea v2 introduces a declarative paradigm. Key differences from v1:
- `View()` returns `tea.View` (not `string`). Use `tea.NewView(content)`.
- Alt screen, mouse mode, window title are set as `tea.View` fields, not program options or commands.
- `tea.KeyMsg` is now an interface; use `tea.KeyPressMsg` for key presses.
- `msg.Type` → `msg.Code`, `msg.Runes` → `msg.Text`, `msg.Alt` (bool) → `msg.Mod` (bitfield; use `msg.Mod.Contains(tea.ModAlt)`).
- Import path: `charm.land/bubbletea/v2` (vanity domain, not GitHub).

### Positioning

No existing tool combines multi-provider support, interactive drill-down, and direct actions in a single open-source TUI:

| Tool | Multi-provider | Interactive TUI | Drill-down | Actions | OSS |
|------|---------------|-----------------|------------|---------|-----|
| gh-dash | No (GitHub) | Yes | Partial | Yes | Yes |
| glim | No (GitLab) | Yes | Yes | Limited | Yes |
| WTFutil | Yes | No (static) | No | No | Yes |
| GitKraken CLI | Yes | Yes | Yes | Yes | No |
| **Manifold** | **Yes** | **Yes** | **Yes** | **Yes** | **Yes** |

## Architecture

### High-Level Structure

```
manifold
├── cmd/                    # CLI entrypoint (kong)
├── internal/
│   ├── config/             # TOML parsing, defaults
│   ├── provider/           # Provider interface + implementations
│   │   ├── provider.go     # Provider interface
│   │   ├── models.go       # Normalized models
│   │   ├── detector.go     # Detect provider from git remote
│   │   ├── github/
│   │   ├── gitlab/
│   │   └── bitbucket/
│   ├── poller/             # Adaptive polling, refresh management
│   └── tui/                # Bubble Tea v2
│       ├── app.go          # Root model
│       ├── pipelines/      # Left panel
│       ├── jobs/           # Center panel
│       ├── detail/         # Right panel (steps + log)
│       └── statusbar/      # Bottom bar (shortcuts, provider info)
└── go.mod
```

### Flow

1. `manifold` starts, reads git remote from current directory (or `-C path`)
2. Detector identifies the provider from the remote URL
3. If multiple remotes and no default configured, a Bubble Tea selector is shown
4. Config TOML is loaded for auth (CLI tool fallback → token)
5. Provider is initialized and the TUI starts

## Normalized Models

Every provider maps its API responses to these common models:

```go
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

type Pipeline struct {
    ID        string
    Ref       string          // branch or tag
    Commit    string          // short SHA
    Message   string          // commit message
    Author    string
    Status    PipelineStatus
    StartedAt time.Time
    Duration  time.Duration
    WebURL    string
    Jobs      []Job
}

type Job struct {
    ID        string
    Name      string
    Status    PipelineStatus
    StartedAt time.Time
    Duration  time.Duration
    Steps     []Step
}

type Step struct {
    Name     string
    Status   PipelineStatus
    Duration time.Duration
    LogStart int             // line offset where this step begins in job log
    LogEnd   int             // line offset where this step ends in job log
}
```

### Provider Mapping

- **GitHub Actions**: Workflow Run → Pipeline, Job → Job, Step → Step
- **GitLab CI**: Pipeline → Pipeline, Job → Job, log sections → Step
- **Bitbucket Pipelines**: Pipeline → Pipeline, Step → Job (Bitbucket "steps" are equivalent to jobs)

## Provider Interface

```go
type Provider interface {
    // Name returns the provider identifier (github, gitlab, bitbucket)
    Name() string

    // ListPipelines returns the most recent pipelines for the detected repo.
    // Limited to the last 25 pipelines by default (configurable).
    ListPipelines(ctx context.Context, limit int) ([]Pipeline, error)

    // GetJobs returns jobs for a specific pipeline.
    // Jobs are returned with empty Steps slices — steps are loaded lazily via GetSteps.
    GetJobs(ctx context.Context, pipelineID string) ([]Job, error)

    // GetSteps returns steps for a specific job.
    // For GitHub: fetched from job detail endpoint.
    // For GitLab: parsed from log section headers.
    // For Bitbucket: derived from script sections.
    GetSteps(ctx context.Context, jobID string) ([]Step, error)

    // GetLog returns log output for a job, starting from offset.
    // Returns content and new offset for incremental polling.
    GetLog(ctx context.Context, jobID string, offset int) (content string, newOffset int, err error)

    // RetryPipeline re-runs an entire pipeline
    RetryPipeline(ctx context.Context, pipelineID string) error

    // RetryJob re-runs a single job (where supported by the provider)
    RetryJob(ctx context.Context, jobID string) error

    // CancelPipeline stops a running pipeline
    CancelPipeline(ctx context.Context, pipelineID string) error

    // CancelJob stops a single running job (where supported by the provider)
    CancelJob(ctx context.Context, jobID string) error
}
```

Adding a new provider means implementing this interface and registering it in the detector. No changes to the TUI or poller required. If a provider does not support job-level retry/cancel, the method should return a descriptive `ErrNotSupported` error, and the TUI will disable the action for that context.

### Auth Resolution (per provider)

Auth is resolved in order, first match wins:

1. CLI tool available? (`gh auth status`, `glab auth status`) → use it. Note: Bitbucket has no official CLI, so this step is skipped for Bitbucket.
2. Environment variable? (`GITHUB_TOKEN`, `GITLAB_TOKEN`, `BITBUCKET_TOKEN`) → use it
3. TOML config token for matching host → use it
4. None found → exit with clear instructions on how to configure

## Detector

```go
func Detect(remotePath string) (providerType string, owner string, repo string, err error)
```

- Reads `git remote get-url <remote>` from the target directory
- Matches host against known defaults and custom hosts configured in TOML:
  - `github.com` (or custom `host` in TOML) → `github`
  - `gitlab.com` (or custom `host` in TOML) → `gitlab`
  - `bitbucket.org` (or custom `host` in TOML) → `bitbucket`
- Extracts owner/repo from the URL path
- Unrecognized host → error: `"unsupported provider for remote: git@custom.host:foo/bar.git"`

### Multiple Remotes

- Default: uses `origin`
- Flag `-r` / `--remote` to specify a different remote
- Configurable default in TOML (`[defaults] remote = "origin"`)
- If multiple remotes and no default: Bubble Tea selector listing remotes with their URLs

## Configuration

File: `~/.config/manifold/config.toml`

```toml
[defaults]
remote = "origin"                    # omit for selector when multiple remotes
polling_active = 5                   # seconds, when pipelines are running
polling_idle = 60                    # seconds, when everything is idle
confirm_actions = true               # confirm before retry/cancel/open
pipeline_limit = 25                  # max pipelines to fetch

# Provider auth — fallback when CLI tool is not available
[[providers]]
type = "github"
# host = "github.com"               # default, can be omitted
token = "ghp_xxx"

[[providers]]
type = "github"
host = "github.mycompany.com"        # GitHub Enterprise Server
token = "ghp_xxx"

[[providers]]
type = "gitlab"
host = "gitlab.mycompany.com"        # self-hosted
token = "glpat-xxx"

[[providers]]
type = "bitbucket"
token = "xxx"                        # repository access token (recommended, no username needed)
# For app passwords, also provide username:
# username = "user"
# token = "app-password"
```

Bitbucket supports two auth modes:
- **Repository access tokens** (recommended): only `token` required, no username needed.
- **App passwords**: both `username` and `token` fields required (HTTP Basic Auth).

Duplicate provider entries for the same `type` + `host` combination result in an error at startup.

Environment variables are also supported as an alternative to TOML config:
- `GITHUB_TOKEN` for GitHub (and `GITHUB_HOST` for GHE)
- `GITLAB_TOKEN` for GitLab (and `GITLAB_HOST` for self-hosted)
- `BITBUCKET_TOKEN` for Bitbucket (and `BITBUCKET_USERNAME` for app passwords)

## Adaptive Poller

The poller adjusts its frequency based on pipeline state:

- **Active interval** (default 5s): when any pipeline has `StatusRunning`
- **Idle interval** (default 60s): when all pipelines are terminal (success/failed/canceled)
- **Log polling** (2-3s): active only for the currently selected job in the detail panel
- **Manual refresh**: `R` triggers an immediate poll regardless of timer
- Each tick calls `ListPipelines`, updates the TUI via Bubble Tea `Cmd`/`Msg`
- **Pipeline limit**: `ListPipelines` fetches the 25 most recent pipelines by default (configurable via `pipeline_limit` in TOML)

### Log Buffer Strategy

- Logs are fetched incrementally using offset-based polling and appended to an in-memory buffer
- Maximum buffer size: 10,000 lines per job. When exceeded, oldest lines are discarded (ring buffer)
- **Auto-scroll**: when the user is at the bottom of the log, new lines automatically scroll into view (tail follow). If the user has scrolled up, position is held until they return to the bottom
- When switching to a different job, the previous job's log buffer is discarded and a fresh fetch begins

## TUI Layout

```
┌─────────────┬──────────────────┬─────────────────────────┐
│  Pipelines  │      Jobs        │       Detail            │
│             │                  │                         │
│ > repo-api  │  > build    ✓   │  Step: deploy-prod      │
│   repo-web  │    test     ✓   │  Status: running ●      │
│   repo-cli  │  > deploy   ●   │  Duration: 2m34s        │
│             │    lint     ✗   │                         │
│             │                  │  -- live log ---------- │
│             │                  │  Deploying to prod...   │
│             │                  │  Image: sha-a1b2c3f     │
│             │                  │  Pulling manifest...    │
│             │                  │                         │
├─────────────┴──────────────────┴─────────────────────────┤
│ [r]etry [c]ancel [o]pen [y]ank URL [R]efresh [?]help  ● │
└──────────────────────────────────────────────────────────┘
```

Three fixed panels. Content updates in-place as the user navigates — no screen changes.

### Panel Sizing

Panel widths are proportional to the terminal width:
- Pipelines: 25%
- Jobs: 25%
- Detail: 50%

Terminal resize is handled via `tea.WindowSizeMsg` — panels reflow proportionally on each resize event. Minimum terminal width: 80 columns. Below that, a warning is shown.

### Navigation

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Move focus between panels |
| `h` / `l` | Move focus between panels (vim) |
| `j` / `k` | Navigate up/down in active panel |
| `Enter` | Expand selection / drill down |
| `Esc` | Return to previous panel |
| `g` / `G` | Jump to top / bottom of list |

### Actions

| Key | Action | Confirmation |
|-----|--------|-------------|
| `r` | Retry pipeline/job | Configurable (default: yes) |
| `c` | Cancel pipeline/job | Configurable (default: yes) |
| `o` | Open in browser | No |
| `y` | Copy URL to clipboard | No |
| `R` | Force refresh | No |
| `?` | Show help overlay | No |
| `q` | Quit | No |

### Status Bar

Left side: available actions (context-sensitive — `retry` only on failed pipelines, etc.)
Right side: provider indicator and repo name (e.g., `● github.com/user/repo`)

### Status Icons and Colors

Unified across all providers:

| Status | Icon | Color |
|--------|------|-------|
| Queued | `◷` | Blue |
| Pending | `○` | Gray |
| Running | `●` | Yellow |
| Success | `✓` | Green |
| Failed | `✗` | Red |
| Canceled | `⊘` | Gray |
| Skipped | `–` | Dark gray |

## Error Handling

The TUI must never crash or freeze. All errors are handled inline:

- **Provider unreachable**: inline message in panel (e.g., `⚠ GitHub API: rate limit exceeded, retry in 42s`), polling continues
- **Auth failed**: clear error at startup with instructions (`No auth found for github.com. Run "gh auth login" or add a token to ~/.config/manifold/config.toml`)
- **Action failed** (retry/cancel): notification in status bar (e.g., `✗ Retry failed: 403 Forbidden`)
- **Unrecognized remote**: exit with clear message
- **API timeout**: 10s timeout for list operations, 5s for logs. On timeout, keep stale data and retry next tick

## Testing Strategy

- **Provider unit tests**: mock HTTP server per provider, test normalization of API responses to common models (e.g., GitHub `completed/success` → `StatusSuccess`, GitLab `created` → `StatusPending`)
- **Poller unit tests**: verify adaptive interval switching, forced refresh, incremental log offset
- **Provider interface mocking**: inject fake provider in TUI tests with static data
- **No TUI E2E tests in MVP**: `teatest` can be added post-MVP

## CLI Interface

```
manifold                      # monitor CI for current directory
manifold -C /path/to/repo     # monitor CI for a different repo
manifold -r upstream           # use a specific git remote
manifold --version             # print version
manifold --help                # print help
```

## Roadmap

### MVP (v0.1)
- Unified pipeline view with normalized status icons/colors
- 3-panel fixed layout with vim keybindings
- Step explorer + log viewer with incremental polling
- Actions: retry, cancel, open in browser, copy URL
- Configurable action confirmation (default: confirm all)
- Adaptive polling (5s running / 60s idle)
- Auto-detect provider from git remote
- Remote selector when multiple remotes without default
- Auth: reuse `gh`/`glab` CLI, fallback to TOML token
- `-C path` and `-r remote` flags

### v0.2
- "Only mine" filter (pipelines from authenticated user)
- Branch filter
- Fuzzy search (`/`)
- Focus mode (running pipelines only)

### v0.3
- Bulk actions (select multiple + action)
- Desktop notifications (pipeline completed/failed)
- Sparkline of last N run durations

### v0.4
- Dynamic grouping (by provider/status/project)
- Approve deployment action
- Configurable themes
