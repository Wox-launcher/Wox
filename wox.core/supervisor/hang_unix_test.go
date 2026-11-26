//go:build linux || darwin

package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exercise the actual heartbeat and signal path without blocking the test process.
func TestUnixUIHeartbeatCapturesBlockedDispatch(t *testing.T) {
	setupSupervisorTest(t)
	args := os.Args
	os.Args = append(append([]string{}, args...), ArgDebugMode)
	t.Cleanup(func() { os.Args = args })
	m := GetManager()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stop := m.watchUI(ctx, os.Getpid())
	defer stop()
	queued := make(chan func(), 1)
	m.StartUIHeartbeat(ctx, func(fn func()) error { queued <- fn; return nil })
	select {
	case dispatch := <-queued:
		dispatch()
	case <-time.After(4 * time.Second):
		t.Fatal("UI heartbeat was never posted")
	}
	deadline := time.After(15 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("blocked UI dispatcher did not produce a hang snapshot")
		case <-ticker.C:
			paths, err := filepath.Glob(filepath.Join(m.CrashDumpsDirectory(), "*-hang-*.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if len(paths) == 0 {
				continue
			}
			data, err := os.ReadFile(paths[0])
			if err != nil || !strings.Contains(string(data), "TestUnixUIHeartbeatCapturesBlockedDispatch") {
				t.Fatalf("invalid goroutine snapshot: %v", err)
			}
			return
		}
	}
}
