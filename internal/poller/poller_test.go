package poller

import (
	"testing"
	"time"
)

func TestNewPoller(t *testing.T) {
	p := New(5*time.Second, 60*time.Second)
	if p.activeInterval != 5*time.Second { t.Errorf("active interval: got %v, want 5s", p.activeInterval) }
	if p.idleInterval != 60*time.Second { t.Errorf("idle interval: got %v, want 60s", p.idleInterval) }
}

func TestIntervalSelection(t *testing.T) {
	p := New(5*time.Second, 60*time.Second)
	p.SetHasRunning(true)
	if p.CurrentInterval() != 5*time.Second { t.Errorf("with running: got %v, want 5s", p.CurrentInterval()) }
	p.SetHasRunning(false)
	if p.CurrentInterval() != 60*time.Second { t.Errorf("without running: got %v, want 60s", p.CurrentInterval()) }
}

func TestShouldPollLog(t *testing.T) {
	p := New(5*time.Second, 60*time.Second)
	p.SetHasRunning(false)
	if p.ShouldPollLog() { t.Error("should not poll log when nothing is running") }
	p.SetHasRunning(true)
	if !p.ShouldPollLog() { t.Error("should poll log when something is running") }
}

func TestForceRefresh(t *testing.T) {
	p := New(5*time.Second, 60*time.Second)
	if p.ShouldForceRefresh() { t.Error("should not force refresh initially") }
	p.RequestRefresh()
	if !p.ShouldForceRefresh() { t.Error("should force refresh after request") }
	if p.ShouldForceRefresh() { t.Error("flag should be cleared after read") }
}
