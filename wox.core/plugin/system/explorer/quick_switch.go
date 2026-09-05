package explorer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"wox/util"
	"wox/util/window"
)

const quickSwitchRetryDelay = 100 * time.Millisecond

type quickSwitchDeps struct {
	getExplorerPath func(pid int, windowID string) string
	getDialogPath   func(windowID string, pid int) string
	isTargetCurrent func(pid int, windowID string) bool
	navigate        func(ctx context.Context, windowID string, pid int, path string) bool
	normalizePath   func(path string) string
	updateCache     func(pid int, windowID string, path string)
	sleep           func(d time.Duration)
	logInfo         func(ctx context.Context, msg string)
	logWarn         func(ctx context.Context, msg string)
	platform        string
}

type quickSwitchCoordinator struct {
	generation atomic.Uint64
	deps       quickSwitchDeps
}

// newQuickSwitchCoordinator wires injectable path, identity, and navigation callbacks for tests and production.
func newQuickSwitchCoordinator(deps quickSwitchDeps) *quickSwitchCoordinator {
	if deps.sleep == nil {
		deps.sleep = time.Sleep
	}
	if deps.normalizePath == nil {
		deps.normalizePath = normalizeQuickSwitchPath
	}
	if deps.platform == "" {
		deps.platform = quickSwitchPlatformName()
	}
	if deps.logInfo == nil {
		deps.logInfo = func(ctx context.Context, msg string) {
			util.GetLogger().Info(ctx, msg)
		}
	}
	if deps.logWarn == nil {
		deps.logWarn = func(ctx context.Context, msg string) {
			util.GetLogger().Warn(ctx, msg)
		}
	}
	return &quickSwitchCoordinator{deps: deps}
}

func (c *quickSwitchCoordinator) Invalidate() {
	if c == nil {
		return
	}
	c.generation.Add(1)
}

// Request starts an async Quick Switch and invalidates any in-flight request.
func (c *quickSwitchCoordinator) Request(ctx context.Context, source ExplorerWindowRef, targetPid int, targetWindowID string) {
	if c == nil || source.Pid <= 0 || targetPid <= 0 {
		return
	}

	generation := c.generation.Add(1)
	util.Go(ctx, "explorer quick switch", func() {
		c.execute(ctx, generation, source, targetPid, targetWindowID)
	})
}

func (c *quickSwitchCoordinator) execute(ctx context.Context, generation uint64, source ExplorerWindowRef, targetPid int, targetWindowID string) {
	startedAt := time.Now()
	logSkip := func(stage string) {
		c.deps.logInfo(ctx, fmt.Sprintf("Explorer Quick Switch skipped: stage=%s platform=%s sourcePid=%d sourceWindowId=%q pid=%d windowId=%q elapsedMs=%d", stage, c.deps.platform, source.Pid, source.WindowID, targetPid, targetWindowID, time.Since(startedAt).Milliseconds()))
	}
	logFail := func(stage string) {
		c.deps.logWarn(ctx, fmt.Sprintf("Explorer Quick Switch failed: stage=%s platform=%s sourcePid=%d sourceWindowId=%q pid=%d windowId=%q elapsedMs=%d", stage, c.deps.platform, source.Pid, source.WindowID, targetPid, targetWindowID, time.Since(startedAt).Milliseconds()))
	}

	if !c.isCurrent(generation) {
		logSkip("generation")
		return
	}

	// Phase timings: navigation itself is fast, so anything slow here is what the
	// user actually feels between the dialog appearing and the folder changing.
	phaseStart := time.Now()
	sourcePath := strings.TrimSpace(c.deps.getExplorerPath(source.Pid, source.WindowID))
	sourcePathMs := time.Since(phaseStart).Milliseconds()
	if sourcePath == "" {
		logSkip("source-path")
		return
	}

	phaseStart = time.Now()
	targetCurrent := c.isCurrent(generation) && c.deps.isTargetCurrent(targetPid, targetWindowID)
	targetIdentityMs := time.Since(phaseStart).Milliseconds()
	if !targetCurrent {
		logSkip("target-identity")
		return
	}

	phaseStart = time.Now()
	dialogPath := strings.TrimSpace(c.deps.getDialogPath(targetWindowID, targetPid))
	dialogPathMs := time.Since(phaseStart).Milliseconds()
	c.deps.logInfo(ctx, fmt.Sprintf("Explorer Quick Switch phases: sourcePathMs=%d targetIdentityMs=%d dialogPathMs=%d pid=%d windowId=%q", sourcePathMs, targetIdentityMs, dialogPathMs, targetPid, targetWindowID))
	if dialogPath != "" && c.deps.normalizePath(dialogPath) == c.deps.normalizePath(sourcePath) {
		if c.deps.updateCache != nil {
			c.deps.updateCache(targetPid, targetWindowID, dialogPath)
		}
		logSkip("same-directory")
		return
	}

	if c.tryNavigate(ctx, generation, targetPid, targetWindowID, sourcePath) {
		c.finishSuccess(ctx, startedAt, targetPid, targetWindowID, sourcePath)
		return
	}

	if !c.isCurrent(generation) {
		logSkip("generation")
		return
	}

	c.deps.sleep(quickSwitchRetryDelay)

	if !c.isCurrent(generation) {
		logSkip("generation")
		return
	}
	if !c.deps.isTargetCurrent(targetPid, targetWindowID) {
		logSkip("target-identity")
		return
	}
	if strings.TrimSpace(c.deps.getExplorerPath(source.Pid, source.WindowID)) == "" {
		logSkip("source-invalid")
		return
	}

	if c.tryNavigate(ctx, generation, targetPid, targetWindowID, sourcePath) {
		c.finishSuccess(ctx, startedAt, targetPid, targetWindowID, sourcePath)
		return
	}
	logFail("navigate")
}

func (c *quickSwitchCoordinator) tryNavigate(ctx context.Context, generation uint64, targetPid int, targetWindowID string, path string) bool {
	if !c.isCurrent(generation) || !c.deps.isTargetCurrent(targetPid, targetWindowID) {
		return false
	}
	return c.deps.navigate(ctx, targetWindowID, targetPid, path)
}

func (c *quickSwitchCoordinator) finishSuccess(ctx context.Context, startedAt time.Time, targetPid int, targetWindowID string, path string) {
	if c.deps.updateCache != nil {
		c.deps.updateCache(targetPid, targetWindowID, path)
	}
	c.deps.logInfo(ctx, fmt.Sprintf("Explorer Quick Switch succeeded: platform=%s pid=%d windowId=%q path=%q elapsedMs=%d", c.deps.platform, targetPid, targetWindowID, path, time.Since(startedAt).Milliseconds()))
}

func (c *quickSwitchCoordinator) isCurrent(generation uint64) bool {
	return c.generation.Load() == generation
}

func normalizeQuickSwitchPath(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if util.IsWindows() {
		return strings.ToLower(path)
	}
	return path
}

func quickSwitchPlatformName() string {
	if util.IsWindows() {
		return "windows"
	}
	if util.IsMacOS() {
		return "darwin"
	}
	return "linux"
}

// explorerIntegrationCapabilities reports which integration features follow the type-to-search setting.
// Quick Switch always runs; raw keys and the dialog hint stay tied to type-to-search.
func explorerIntegrationCapabilities(typeToSearch bool) (quickSwitch bool, rawKeys bool, dialogHint bool) {
	return true, typeToSearch, typeToSearch
}

func (c *ExplorerPlugin) newQuickSwitchDeps() quickSwitchDeps {
	return quickSwitchDeps{
		getExplorerPath: window.GetFileExplorerPathByWindow,
		getDialogPath: func(windowID string, pid int) string {
			if window.IsBrowseForFolderDialog(windowID, pid) {
				return ""
			}
			if ok, _ := window.IsFileExplorer(pid); ok {
				return ""
			}
			if strings.TrimSpace(windowID) != "" {
				if dialogPath := strings.TrimSpace(window.GetFileDialogPathByWindowId(windowID, pid)); dialogPath != "" {
					return dialogPath
				}
			}
			return strings.TrimSpace(window.GetFileDialogPathByPid(pid))
		},
		isTargetCurrent: c.isQuickSwitchTargetCurrent,
		navigate:        c.performFileDialogNavigation,
		normalizePath:   c.normalizePathKey,
		updateCache: func(pid int, windowID string, path string) {
			c.setCachedOpenSaveDialogPath(pid, window.GetWindowNameByPid(pid), windowID, path)
		},
		platform: quickSwitchPlatformName(),
	}
}

// isQuickSwitchTargetCurrent verifies the captured dialog is still the foreground native file dialog.
func (c *ExplorerPlugin) isQuickSwitchTargetCurrent(pid int, windowID string) bool {
	if pid <= 0 {
		return false
	}
	if ok, err := window.IsOpenSaveDialogByPid(pid); err != nil || !ok {
		return false
	}
	if window.GetActiveWindowPid() != pid {
		return false
	}
	if util.IsWindows() {
		if strings.TrimSpace(windowID) == "" {
			return false
		}
		currentID := strings.TrimSpace(window.GetActiveWindowId())
		if currentID == "" {
			currentID = strings.TrimSpace(GetOpenSaveDialogWindowIdByPid(pid))
		}
		return currentID == windowID
	}
	if util.IsMacOS() {
		if strings.TrimSpace(windowID) == "" {
			return false
		}
		return strings.TrimSpace(window.GetActiveWindowId()) == windowID
	}
	return true
}

// performFileDialogNavigation serializes Shell-hook and automation navigation so Quick Switch and user jumps cannot overlap.
func (c *ExplorerPlugin) performFileDialogNavigation(ctx context.Context, windowID string, pid int, folderPath string) bool {
	c.dialogNavigateMu.Lock()
	defer c.dialogNavigateMu.Unlock()
	browse := window.IsBrowseForFolderDialog(windowID, pid)
	explorerHost, _ := window.IsFileExplorer(pid)
	// Never inject the Shell-view hook into explorer.exe. Move Items lives there and
	// SHBrowseForFolder has no IShellBrowser; the hook crashes explorer.
	if !browse && !explorerHost {
		if navigateFileDialogWithHook(ctx, windowID, pid, folderPath) {
			return true
		}
	}
	return window.NavigateFileDialog(windowID, pid, folderPath)
}

// requestQuickSwitch starts background navigation only for a direct Explorer/Finder → dialog transition.
func (c *ExplorerPlugin) requestQuickSwitch(ctx context.Context, event OpenSaveDialogActivatedEvent) {
	if event.PreviousExplorer == nil {
		return
	}
	c.quickSwitch.Request(ctx, *event.PreviousExplorer, event.Pid, event.WindowID)
}
