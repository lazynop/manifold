package poller

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

type TickMsg struct{}
type LogTickMsg struct{}

type Poller struct {
	activeInterval time.Duration
	idleInterval   time.Duration
	hasRunning     bool
	forceRefresh   bool
}

func New(active, idle time.Duration) *Poller {
	return &Poller{activeInterval: active, idleInterval: idle}
}

func (p *Poller) SetHasRunning(running bool) { p.hasRunning = running }

func (p *Poller) CurrentInterval() time.Duration {
	if p.hasRunning { return p.activeInterval }
	return p.idleInterval
}

func (p *Poller) ShouldPollLog() bool { return p.hasRunning }

func (p *Poller) RequestRefresh() { p.forceRefresh = true }

func (p *Poller) ShouldForceRefresh() bool {
	if p.forceRefresh { p.forceRefresh = false; return true }
	return false
}

func (p *Poller) TickCmd() tea.Cmd {
	interval := p.CurrentInterval()
	return tea.Tick(interval, func(t time.Time) tea.Msg { return TickMsg{} })
}

func (p *Poller) LogTickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return LogTickMsg{} })
}
