package filesearch

import (
	"context"
	"testing"
	"time"
	"wox/util"
)

// TestRunHeartbeatPublishesWithoutFileProgress exercises a stalled diff phase,
// then verifies that terminal snapshots keep their final elapsed value.
func TestRunHeartbeatPublishesWithoutFileProgress(t *testing.T) {
	s := NewScanner(nil)
	s.setTransientRunState(StatusSnapshot{ActiveRunStatus: RunStatusExecuting, ActiveRunFileCount: 551167, ActiveRunElapsedMs: 31000})
	updates := make(chan StatusSnapshot, 2)
	s.SetStateChangeHandler(func(context.Context) {
		state, _ := s.GetTransientRunState()
		select {
		case updates <- state:
		default:
		}
	})
	stop := s.startRunHeartbeat(context.Background(), util.GetSystemTimestamp()-31000)
	select {
	case state := <-updates:
		if state.ActiveRunElapsedMs <= 31000 || state.ActiveRunFileCount != 551167 {
			t.Errorf("invalid heartbeat: %+v", state)
		}
	case <-time.After(3 * time.Second):
		t.Error("elapsed clock stopped during a phase without file progress")
	}
	stop()
	s.setTransientRunState(StatusSnapshot{ActiveRunStatus: RunStatusCompleted, ActiveRunElapsedMs: 32000})
	if s.refreshRunElapsed(60000) {
		t.Fatal("heartbeat changed a completed run")
	}
	state, _ := s.GetTransientRunState()
	if state.ActiveRunElapsedMs != 32000 {
		t.Fatal("completed duration drifted")
	}
	s.clearTransientRunState()
	if s.refreshRunElapsed(61000) {
		t.Fatal("heartbeat restored cleared status")
	}
}
