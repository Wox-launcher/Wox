package explorer

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func testQuickSwitchDeps() quickSwitchDeps {
	return quickSwitchDeps{
		normalizePath: normalizeQuickSwitchPath,
		sleep:         func(d time.Duration) {},
		logInfo:       func(context.Context, string) {},
		logWarn:       func(context.Context, string) {},
		platform:      "test",
	}
}

func TestQuickSwitchSkipsSameDirectory(t *testing.T) {
	var navigated atomic.Int32
	var cachedPath string
	deps := testQuickSwitchDeps()
	deps.getExplorerPath = func(pid int, windowID string) string { return `C:\src` }
	deps.getDialogPath = func(windowID string, pid int) string { return `C:\src` }
	deps.isTargetCurrent = func(pid int, windowID string) bool { return true }
	deps.navigate = func(ctx context.Context, windowID string, pid int, path string) bool {
		navigated.Add(1)
		return true
	}
	deps.updateCache = func(pid int, windowID string, path string) { cachedPath = path }
	coordinator := newQuickSwitchCoordinator(deps)

	coordinator.generation.Store(1)
	coordinator.execute(context.Background(), 1, ExplorerWindowRef{Pid: 11, WindowID: "100"}, 22, "200")
	if navigated.Load() != 0 {
		t.Fatal("same directory should not navigate")
	}
	if cachedPath != `C:\src` {
		t.Fatalf("cache = %q", cachedPath)
	}
}

func TestQuickSwitchCancelsWhenGenerationChanges(t *testing.T) {
	var navigated atomic.Int32
	deps := testQuickSwitchDeps()
	deps.getExplorerPath = func(pid int, windowID string) string { return `C:\src` }
	deps.getDialogPath = func(windowID string, pid int) string { return `C:\other` }
	deps.isTargetCurrent = func(pid int, windowID string) bool { return true }
	deps.navigate = func(ctx context.Context, windowID string, pid int, path string) bool {
		navigated.Add(1)
		return true
	}
	coordinator := newQuickSwitchCoordinator(deps)

	coordinator.generation.Store(1)
	coordinator.Invalidate()
	coordinator.execute(context.Background(), 1, ExplorerWindowRef{Pid: 11, WindowID: "100"}, 22, "200")
	if navigated.Load() != 0 {
		t.Fatal("stale generation should cancel navigation")
	}
}

func TestQuickSwitchCancelsRetryWhenTargetLeavesForeground(t *testing.T) {
	var navigated atomic.Int32
	targetCurrent := true
	deps := testQuickSwitchDeps()
	deps.getExplorerPath = func(pid int, windowID string) string { return `C:\src` }
	deps.getDialogPath = func(windowID string, pid int) string { return `C:\other` }
	deps.isTargetCurrent = func(pid int, windowID string) bool { return targetCurrent }
	deps.navigate = func(ctx context.Context, windowID string, pid int, path string) bool {
		navigated.Add(1)
		return false
	}
	deps.sleep = func(d time.Duration) {
		targetCurrent = false
	}
	coordinator := newQuickSwitchCoordinator(deps)

	coordinator.generation.Store(1)
	coordinator.execute(context.Background(), 1, ExplorerWindowRef{Pid: 11, WindowID: "100"}, 22, "200")
	if navigated.Load() != 1 {
		t.Fatalf("expected one failed attempt before cancel, got %d", navigated.Load())
	}
}

func TestQuickSwitchCancelsRetryWhenGenerationChanges(t *testing.T) {
	var navigated atomic.Int32
	deps := testQuickSwitchDeps()
	deps.getExplorerPath = func(pid int, windowID string) string { return `C:\src` }
	deps.getDialogPath = func(windowID string, pid int) string { return `C:\other` }
	deps.isTargetCurrent = func(pid int, windowID string) bool { return true }
	deps.navigate = func(ctx context.Context, windowID string, pid int, path string) bool {
		navigated.Add(1)
		return false
	}
	coordinator := newQuickSwitchCoordinator(deps)
	coordinator.deps.sleep = func(d time.Duration) {
		coordinator.Invalidate()
	}

	coordinator.generation.Store(1)
	coordinator.execute(context.Background(), 1, ExplorerWindowRef{Pid: 11, WindowID: "100"}, 22, "200")
	if navigated.Load() != 1 {
		t.Fatalf("expected the retry to be cancelled after generation change, got %d", navigated.Load())
	}
}

func TestQuickSwitchRetriesOnceThenUpdatesCache(t *testing.T) {
	var navigated atomic.Int32
	var cachedPath string
	deps := testQuickSwitchDeps()
	deps.getExplorerPath = func(pid int, windowID string) string { return `C:\src` }
	deps.getDialogPath = func(windowID string, pid int) string { return `C:\other` }
	deps.isTargetCurrent = func(pid int, windowID string) bool { return true }
	deps.navigate = func(ctx context.Context, windowID string, pid int, path string) bool {
		count := navigated.Add(1)
		return count == 2
	}
	deps.updateCache = func(pid int, windowID string, path string) { cachedPath = path }
	coordinator := newQuickSwitchCoordinator(deps)

	coordinator.generation.Store(1)
	coordinator.execute(context.Background(), 1, ExplorerWindowRef{Pid: 11, WindowID: "100"}, 22, "200")
	if navigated.Load() != 2 {
		t.Fatalf("expected one retry, got %d navigations", navigated.Load())
	}
	if cachedPath != `C:\src` {
		t.Fatalf("cache = %q", cachedPath)
	}
}

func TestQuickSwitchSkipsEmptyAndVirtualSource(t *testing.T) {
	var navigated atomic.Int32
	deps := testQuickSwitchDeps()
	deps.getExplorerPath = func(pid int, windowID string) string { return "" }
	deps.getDialogPath = func(windowID string, pid int) string { return `C:\other` }
	deps.isTargetCurrent = func(pid int, windowID string) bool { return true }
	deps.navigate = func(ctx context.Context, windowID string, pid int, path string) bool {
		navigated.Add(1)
		return true
	}
	coordinator := newQuickSwitchCoordinator(deps)

	coordinator.generation.Store(1)
	coordinator.execute(context.Background(), 1, ExplorerWindowRef{Pid: 11, WindowID: "100"}, 22, "200")
	if navigated.Load() != 0 {
		t.Fatal("empty or virtual source path should skip navigation")
	}
}

func TestTypeToSearchOffKeepsQuickSwitchWithoutRawKeys(t *testing.T) {
	quickSwitch, rawKeys, dialogHint := explorerIntegrationCapabilities(false)
	if !quickSwitch {
		t.Fatal("Quick Switch must stay enabled when type-to-search is off")
	}
	if rawKeys || dialogHint {
		t.Fatal("type-to-search off should not register raw keys or show the dialog hint")
	}
}

func TestTypeToSearchOnKeepsHintAndShortcuts(t *testing.T) {
	quickSwitch, rawKeys, dialogHint := explorerIntegrationCapabilities(true)
	if !quickSwitch || !rawKeys || !dialogHint {
		t.Fatal("type-to-search on should keep Quick Switch, raw keys, and the dialog hint")
	}
}
