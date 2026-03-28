package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/steven/manifold/internal/poller"
	"github.com/steven/manifold/internal/provider"
	"github.com/steven/manifold/internal/tui/detail"
	"github.com/steven/manifold/internal/tui/jobs"
	"github.com/steven/manifold/internal/tui/pipelines"
	"github.com/steven/manifold/internal/tui/shared"
	"github.com/steven/manifold/internal/tui/statusbar"
)

const (
	PanelPipelines = 0
	PanelJobs      = 1
	PanelDetail    = 2
	panelCount     = 3

	minTerminalWidth = 80
	fetchTimeout     = 10 * time.Second
	logTimeout       = 5 * time.Second
)

type (
	PipelinesMsg    struct{ Pipelines []provider.Pipeline }
	JobsMsg         struct{ Jobs []provider.Job }
	StepsMsg        struct{ Steps []provider.Step }
	LogMsg          struct {
		Content   string
		NewOffset int
	}
	ActionResultMsg struct {
		Err    error
		Action string
	}
	ErrMsg struct{ Err error }
)

// actionRequest represents a retry or cancel action on a pipeline or job.
type actionRequest struct {
	Verb   string // "retry" or "cancel"
	Target string // "pipeline" or "job"
	ID     string
}

type confirmState struct {
	Message string
	Action  actionRequest
	Width   int
}

func (c *confirmState) view() string {
	prompt := fmt.Sprintf("%s  [y] yes  [n] no", c.Message)
	inner := lipgloss.NewStyle().
		Foreground(shared.ColorWhite).
		Render(prompt)
	return shared.PanelBorderActive.
		Width(c.Width).
		Render(shared.PanelTitle.Render("Confirm") + "\n\n" + inner)
}

// App is the root Bubble Tea model.
type App struct {
	prov         provider.Provider
	poll         *poller.Poller
	detectResult provider.DetectResult

	pipelinesPanel pipelines.Model
	jobsPanel      jobs.Model
	detailPanel    detail.Model
	statusBar      statusbar.Model
	confirmDialog  *confirmState

	focusedPanel      int
	confirmActions    bool
	pipelineLimit     int
	selectedPipeline  string // ID of currently selected pipeline, to avoid spurious re-fetches
	width             int
	height            int
	ready             bool
	tooNarrow         bool
	showHelp          bool
}

func NewApp(detect provider.DetectResult, confirmActions bool, pipelineLimit int) *App {
	return &App{
		detectResult:   detect,
		confirmActions: confirmActions,
		pipelineLimit:  pipelineLimit,
		focusedPanel:   PanelPipelines,
		pipelinesPanel: pipelines.New(0, 0),
		jobsPanel:      jobs.New(0, 0),
		detailPanel:    detail.New(0, 0),
		statusBar:      statusbar.New(0),
	}
}

func (a *App) SetProvider(p provider.Provider) { a.prov = p }
func (a *App) SetPoller(p *poller.Poller)      { a.poll = p }

func (a *App) FocusNext() {
	a.focusedPanel = (a.focusedPanel + 1) % panelCount
	a.updatePanelFocus()
}

func (a *App) FocusPrev() {
	a.focusedPanel = (a.focusedPanel - 1 + panelCount) % panelCount
	a.updatePanelFocus()
}

func (a *App) ProviderLabel() string {
	d := a.detectResult
	return fmt.Sprintf("%s/%s/%s", d.Host, d.Owner, d.Repo)
}

func (a *App) updatePanelFocus() {
	a.pipelinesPanel.Focused = a.focusedPanel == PanelPipelines
	a.jobsPanel.Focused = a.focusedPanel == PanelJobs
	a.detailPanel.Focused = a.focusedPanel == PanelDetail
}

func (a *App) Init() tea.Cmd {
	a.statusBar.SetProvider(a.ProviderLabel())
	a.updatePanelFocus()

	cmds := []tea.Cmd{a.fetchPipelines()}
	if a.poll != nil {
		cmds = append(cmds, a.poll.TickCmd(), a.poll.LogTickCmd())
	}
	return tea.Batch(cmds...)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return a, a.handleWindowSize(msg)

	case tea.KeyPressMsg:
		if a.showHelp {
			a.showHelp = false
			return a, nil
		}
		if a.confirmDialog != nil {
			return a, a.handleConfirmKey(msg)
		}
		return a, a.handleKey(msg)

	case poller.TickMsg:
		var cmds []tea.Cmd
		if a.poll != nil {
			cmds = append(cmds, a.fetchPipelines(), a.poll.TickCmd())
		}
		return a, tea.Batch(cmds...)

	case poller.LogTickMsg:
		var cmds []tea.Cmd
		if a.poll != nil {
			if a.poll.ShouldPollLog() && a.detailPanel.HasJob() {
				cmds = append(cmds, a.fetchLog())
			}
			cmds = append(cmds, a.poll.LogTickCmd())
		}
		return a, tea.Batch(cmds...)

	case PipelinesMsg:
		return a, a.handlePipelinesMsg(msg)

	case JobsMsg:
		a.jobsPanel.SetJobs(msg.Jobs)
		return a, nil

	case StepsMsg:
		if a.detailPanel.HasJob() {
			j := a.detailPanel.Job()
			j.Steps = msg.Steps
			a.detailPanel.SetJob(j)
		}
		return a, nil

	case LogMsg:
		a.detailPanel.AppendLog(msg.Content)
		a.detailPanel.SetRemoteLogOffset(msg.NewOffset)
		return a, nil

	case ActionResultMsg:
		if msg.Err != nil {
			a.statusBar.SetNotification(fmt.Sprintf("%s failed: %v", msg.Action, msg.Err), true)
		} else {
			a.statusBar.SetNotification(fmt.Sprintf("%s succeeded", msg.Action), false)
		}
		return a, a.fetchPipelines()

	case ErrMsg:
		a.statusBar.SetNotification(fmt.Sprintf("Error: %v", msg.Err), true)
		return a, nil
	}

	return a, nil
}

func (a *App) View() tea.View {
	if !a.ready {
		v := tea.NewView("Loading…")
		v.AltScreen = true
		return v
	}

	if a.tooNarrow {
		msg := fmt.Sprintf("Terminal too narrow (%d cols). Minimum: %d columns.", a.width, minTerminalWidth)
		v := tea.NewView(lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, msg))
		v.AltScreen = true
		return v
	}

	var content string

	if a.showHelp {
		content = a.helpView()
	} else if a.confirmDialog != nil {
		// Skip panel rendering when dialog is shown
		dialogView := a.confirmDialog.view()
		content = lipgloss.Place(
			a.width, a.height-1,
			lipgloss.Center, lipgloss.Center,
			dialogView,
		) + "\n" + a.statusBar.View()
	} else {
		panelsRow := lipgloss.JoinHorizontal(
			lipgloss.Top,
			a.pipelinesPanel.View(),
			a.jobsPanel.View(),
			a.detailPanel.View(),
		)
		content = lipgloss.JoinVertical(lipgloss.Left, panelsRow, a.statusBar.View())
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (a *App) helpView() string {
	help := strings.Join([]string{
		shared.PanelTitle.Render("Manifold — Keybindings"),
		"",
		"  Navigation",
		"  Tab / l          Next panel",
		"  Shift+Tab / h    Previous panel",
		"  j / ↓            Move down",
		"  k / ↑            Move up",
		"  g                Jump to top",
		"  G                Jump to bottom",
		"  Enter            Drill down",
		"  Esc              Go back",
		"",
		"  Actions",
		"  r                Retry pipeline/job",
		"  c                Cancel pipeline/job",
		"  o                Open in browser",
		"  y                Copy URL to clipboard",
		"  R                Force refresh",
		"  ?                Toggle this help",
		"  q                Quit",
		"",
		"  Press any key to dismiss",
	}, "\n")

	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center,
		shared.PanelBorderActive.Width(50).Render(help))
}

// ---------------------------------------------------------------------------
// Window sizing
// ---------------------------------------------------------------------------

func (a *App) handleWindowSize(msg tea.WindowSizeMsg) tea.Cmd {
	a.width = msg.Width
	a.height = msg.Height
	a.ready = true

	if a.width < minTerminalWidth {
		a.tooNarrow = true
		return nil
	}
	a.tooNarrow = false

	panelHeight := a.height - 1
	quarter := a.width / 4

	a.pipelinesPanel.Width = quarter
	a.pipelinesPanel.Height = panelHeight

	a.jobsPanel.Width = quarter
	a.jobsPanel.Height = panelHeight

	a.detailPanel.Width = a.width - (quarter * 2)
	a.detailPanel.Height = panelHeight

	a.statusBar.Width = a.width

	return nil
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

func (a *App) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case KeyQ, KeyCtrlC:
		return func() tea.Msg { return tea.Quit() }

	case KeyTab, KeyL:
		a.FocusNext()
		return nil

	case KeyShiftTab, KeyH:
		a.FocusPrev()
		return nil

	case KeyJ, KeyDown:
		a.moveFocusedDown()
		return nil

	case KeyK, KeyUp:
		a.moveFocusedUp()
		return nil

	case KeyG:
		a.moveFocusedTop()
		return nil

	case KeyShiftG:
		a.moveFocusedBottom()
		return nil

	case KeyEnter:
		return a.handleEnter()

	case KeyEsc:
		return a.handleEsc()

	case KeyR:
		return a.handleAction("retry")

	case KeyC:
		return a.handleAction("cancel")

	case KeyO:
		return a.openBrowserCmd()

	case KeyY:
		return a.yankURL()

	case KeyShiftR:
		if a.poll != nil {
			a.poll.RequestRefresh()
		}
		return a.fetchPipelines()

	case KeyQuestion:
		a.showHelp = true
		return nil
	}

	return nil
}

func (a *App) handleConfirmKey(msg tea.KeyPressMsg) tea.Cmd {
	if a.confirmDialog == nil {
		return nil
	}
	switch msg.String() {
	case "y":
		action := a.confirmDialog.Action
		a.confirmDialog = nil
		return a.executeAction(action)
	case "n", KeyEsc:
		a.confirmDialog = nil
		a.statusBar.SetNotification("Cancelled", false)
		return nil
	}
	return nil
}

// ---------------------------------------------------------------------------
// Movement helpers
// ---------------------------------------------------------------------------

func (a *App) moveFocusedDown() {
	switch a.focusedPanel {
	case PanelPipelines:
		a.pipelinesPanel.MoveDown()
		a.jobsPanel.Clear()
		a.detailPanel.ClearLog()
	case PanelJobs:
		a.jobsPanel.MoveDown()
	case PanelDetail:
		a.detailPanel.ScrollDown()
	}
}

func (a *App) moveFocusedUp() {
	switch a.focusedPanel {
	case PanelPipelines:
		a.pipelinesPanel.MoveUp()
		a.jobsPanel.Clear()
		a.detailPanel.ClearLog()
	case PanelJobs:
		a.jobsPanel.MoveUp()
	case PanelDetail:
		a.detailPanel.ScrollUp()
	}
}

func (a *App) moveFocusedTop() {
	switch a.focusedPanel {
	case PanelPipelines:
		a.pipelinesPanel.GoToTop()
	case PanelJobs:
		a.jobsPanel.GoToTop()
	}
}

func (a *App) moveFocusedBottom() {
	switch a.focusedPanel {
	case PanelPipelines:
		a.pipelinesPanel.GoToBottom()
	case PanelJobs:
		a.jobsPanel.GoToBottom()
	}
}

// ---------------------------------------------------------------------------
// Action handlers
// ---------------------------------------------------------------------------

func (a *App) handleEnter() tea.Cmd {
	switch a.focusedPanel {
	case PanelPipelines:
		if _, ok := a.pipelinesPanel.Selected(); ok {
			a.FocusNext()
			return a.fetchJobsForSelected()
		}
	case PanelJobs:
		if j, ok := a.jobsPanel.Selected(); ok {
			a.detailPanel.SetJob(j)
			a.FocusNext()
			return tea.Batch(a.fetchStepsForSelected(), a.fetchLog())
		}
	}
	return nil
}

func (a *App) handleEsc() tea.Cmd {
	switch a.focusedPanel {
	case PanelJobs:
		a.FocusPrev()
	case PanelDetail:
		a.FocusPrev()
	}
	return nil
}

func (a *App) handleAction(verb string) tea.Cmd {
	switch a.focusedPanel {
	case PanelPipelines:
		if p, ok := a.pipelinesPanel.Selected(); ok {
			req := actionRequest{Verb: verb, Target: "pipeline", ID: p.ID}
			return a.confirmOrExecute(req, fmt.Sprintf("%s pipeline %s?", verb, p.Ref))
		}
	case PanelJobs:
		if j, ok := a.jobsPanel.Selected(); ok {
			req := actionRequest{Verb: verb, Target: "job", ID: j.ID}
			return a.confirmOrExecute(req, fmt.Sprintf("%s job %s?", verb, j.Name))
		}
	}
	return nil
}

func (a *App) confirmOrExecute(action actionRequest, message string) tea.Cmd {
	if !a.confirmActions {
		return a.executeAction(action)
	}
	a.confirmDialog = &confirmState{
		Message: message,
		Action:  action,
		Width:   50,
	}
	return nil
}

func (a *App) executeAction(action actionRequest) tea.Cmd {
	if a.prov == nil {
		return nil
	}
	prov := a.prov
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		var err error
		switch action.Verb + "-" + action.Target {
		case "retry-pipeline":
			err = prov.RetryPipeline(ctx, action.ID)
		case "cancel-pipeline":
			err = prov.CancelPipeline(ctx, action.ID)
		case "retry-job":
			err = prov.RetryJob(ctx, action.ID)
		case "cancel-job":
			err = prov.CancelJob(ctx, action.ID)
		}
		return ActionResultMsg{Err: err, Action: action.Verb + " " + action.Target}
	}
}

// ---------------------------------------------------------------------------
// Browser and clipboard
// ---------------------------------------------------------------------------

func (a *App) openBrowserCmd() tea.Cmd {
	url := a.selectedURL()
	if url == "" {
		return nil
	}
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		_ = cmd.Start()
		return nil
	}
}

func (a *App) yankURL() tea.Cmd {
	url := a.selectedURL()
	if url == "" {
		return nil
	}
	a.statusBar.SetNotification("URL copied: "+url, false)
	return tea.SetClipboard(url)
}

func (a *App) selectedURL() string {
	switch a.focusedPanel {
	case PanelPipelines:
		if p, ok := a.pipelinesPanel.Selected(); ok {
			return p.WebURL
		}
	case PanelJobs, PanelDetail:
		if j, ok := a.jobsPanel.Selected(); ok {
			return j.WebURL
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Fetch commands
// ---------------------------------------------------------------------------

func (a *App) fetchPipelines() tea.Cmd {
	if a.prov == nil {
		return nil
	}
	prov := a.prov
	limit := a.pipelineLimit
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		ps, err := prov.ListPipelines(ctx, limit)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return PipelinesMsg{Pipelines: ps}
	}
}

func (a *App) fetchJobsForSelected() tea.Cmd {
	if a.prov == nil {
		return nil
	}
	p, ok := a.pipelinesPanel.Selected()
	if !ok {
		return nil
	}
	prov := a.prov
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		js, err := prov.GetJobs(ctx, p.ID)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return JobsMsg{Jobs: js}
	}
}

func (a *App) fetchStepsForSelected() tea.Cmd {
	if a.prov == nil {
		return nil
	}
	j, ok := a.jobsPanel.Selected()
	if !ok {
		return nil
	}
	prov := a.prov
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		steps, err := prov.GetSteps(ctx, j.ID)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return StepsMsg{Steps: steps}
	}
}

func (a *App) fetchLog() tea.Cmd {
	if a.prov == nil || !a.detailPanel.HasJob() {
		return nil
	}
	j := a.detailPanel.Job()
	offset := a.detailPanel.RemoteLogOffset()
	prov := a.prov
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), logTimeout)
		defer cancel()
		content, newOffset, err := prov.GetLog(ctx, j.ID, offset)
		if err != nil {
			return ErrMsg{Err: err}
		}
		if content == "" {
			return nil
		}
		return LogMsg{Content: content, NewOffset: newOffset}
	}
}

// ---------------------------------------------------------------------------
// Pipelines message handler
// ---------------------------------------------------------------------------

func (a *App) handlePipelinesMsg(msg PipelinesMsg) tea.Cmd {
	a.pipelinesPanel.SetPipelines(msg.Pipelines)

	if a.poll != nil {
		hasRunning := false
		for _, p := range msg.Pipelines {
			if !p.Status.IsTerminal() {
				hasRunning = true
				break
			}
		}
		a.poll.SetHasRunning(hasRunning)
	}

	// Only re-fetch jobs if the selected pipeline changed
	if p, ok := a.pipelinesPanel.Selected(); ok && a.focusedPanel != PanelPipelines {
		if p.ID != a.selectedPipeline {
			a.selectedPipeline = p.ID
			return a.fetchJobsForSelected()
		}
	}
	return nil
}
