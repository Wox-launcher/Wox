//go:build linux

package window

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestKDEActiveWindowCaptureBypassesClipboardCache(t *testing.T) {
	resetLinuxForegroundState()
	t.Cleanup(resetLinuxForegroundState)
	startForegroundBus(t)
	setForegroundSession(t, "KDE", true)
	isolateForegroundDesktop(t)
	kwin := newFakeKWin(t, `{"windowId":"d48a7b2b-4e8d-4ea3-b540-c3b1dd6592ef","resourceClass":"google-chrome","pid":42}`)
	kwin.mu.Lock()
	kwin.reportDelay = 20 * time.Millisecond
	kwin.mu.Unlock()
	storeLinuxForeground(linuxForegroundApp{WindowID: "6cdee7dc-65ab-42f6-843a-dd2beed1d2ab", Pid: 42, Name: "Chrome"}, nil)

	pid, id, _ := GetActiveWindowIdentity(context.Background())
	if pid != 42 || id != "d48a7b2b-4e8d-4ea3-b540-c3b1dd6592ef" {
		t.Fatalf("captured pid=%d id=%q", pid, id)
	}
}

func TestFocusKDEWindowChecksExactWindowAfterActivation(t *testing.T) {
	startForegroundBus(t)
	kwin := newFakeKWin(t, "")
	const target = "d48a7b2b-4e8d-4ea3-b540-c3b1dd6592ef"
	kwin.mu.Lock()
	kwin.reportFor = func(script string) string {
		if strings.Contains(script, "workspace.activeWindow = candidate") {
			if !strings.Contains(script, target) || !strings.Contains(script, "targetPid = 42") {
				t.Error("activation did not use the captured window ID and PID")
			}
			return `{"windowId":"6cdee7dc-65ab-42f6-843a-dd2beed1d2ab","pid":42}`
		}
		return `{"windowId":"` + target + `","pid":42}`
	}
	kwin.mu.Unlock()

	if err := FocusKDEWindow(context.Background(), target, 42); err != nil {
		t.Fatal(err)
	}
	if loads := kwin.loads.Load(); loads != 2 {
		t.Fatalf("expected one activation and a fresh confirmation, loaded %d scripts", loads)
	}
}

func TestFocusKDEWindowRejectsOtherWindowWithSamePID(t *testing.T) {
	startForegroundBus(t)
	newFakeKWin(t, `{"windowId":"6cdee7dc-65ab-42f6-843a-dd2beed1d2ab","pid":42}`)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := FocusKDEWindow(ctx, "d48a7b2b-4e8d-4ea3-b540-c3b1dd6592ef", 42)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected a focus timeout, got %v", err)
	}
}
