package launcher

import (
	"fmt"

	"wox/plugin"
	"wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	"wox/util"
)

type pendingResultFileDrag struct {
	notify      func(plugin.DragOutEvent)
	resultID    string
	files       []string
	preventHide bool
}

// handleFileDrop delivers launcher drops to the visible chat composer or a selection query.
func (a *App) handleFileDrop(paths []string) {
	cleaned := cleanFileDropPaths(paths)
	if len(cleaned) == 0 {
		return
	}
	if a.launcherPresentsChatInput() {
		a.enqueueChatDraftAttachments(cleaned, nil, false)
		return
	}
	a.handleLauncherSelectionFileDrop(cleaned)
}

// handleChatWindowFileDrop appends files to the dedicated chat draft only.
func (a *App) handleChatWindowFileDrop(paths []string) {
	cleaned := cleanFileDropPaths(paths)
	if len(cleaned) == 0 {
		return
	}
	a.enqueueChatDraftAttachments(cleaned, nil, true)
}

func (a *App) handleLauncherSelectionFileDrop(paths []string) {
	current := toCorePlainQuery(a.query)
	next := plugin.NewGlobalFileDropQuery(paths)
	if manager := plugin.GetPluginManager(); manager != nil {
		next = manager.BuildFileDropQuery(a.chatImportLifecycleCtx(), current, paths)
	}
	if a.window != nil {
		_, _ = a.window.Show()
	}
	if a.host != nil {
		a.host.RequestFocus(view.LauncherQueryInputKey)
	}
	a.canRecallHistory = false
	a.setQuery(fromCorePlainQuery(next))
	if err := a.sendCurrentQuery(); err != nil {
		util.GetLogger().Warn(a.chatImportLifecycleCtx(), fmt.Sprintf("send query after file drop: %v", err))
	}
	// A targeted drop is one selection query. Restore input afterwards so later
	// keystrokes keep the typed text instead of staying pinned to those files.
	if next.QueryText != "" || !next.QueryScope.IsEmpty() {
		a.query.QueryType = "input"
		a.query.QuerySelection = selection{FilePaths: []string{}}
	}
}

// startResultDrag exports the selected result's file payload.
func (a *App) startResultDrag(index int) {
	if index < 0 || index >= len(a.results) || a.results[index].IsGroup || !a.results[index].DragData.isFiles() || a.window == nil {
		return
	}
	result := a.results[index]
	files := append([]string(nil), result.DragData.Files...)
	pending := pendingResultFileDrag{
		resultID:    result.ID,
		files:       files,
		preventHide: result.DragData.PreventHideAfterDrag,
	}
	if a.services != nil {
		pending.notify = a.services.PrepareResultDrag(a.chatImportLifecycleCtx(), a.sessionID, result.QueryID, result.ID)
	}
	a.beginResultFileDrag()
	util.GetLogger().Info(a.chatImportLifecycleCtx(), fmt.Sprintf(
		"result file drag start result=%s preventHide=%v files=%d",
		pending.resultID, pending.preventHide, len(files),
	))
	status, err := a.window.StartFileDrag(files)
	if err != nil {
		util.GetLogger().Warn(a.chatImportLifecycleCtx(), fmt.Sprintf("result file drag failed: %v", err))
		a.endResultFileDrag()
		return
	}
	if status == woxui.FileDragStatusPending {
		a.pendingResultFileDrag = pending
		return
	}
	a.finishResultFileDrag(status, pending)
}

// handleResultDragEnded applies the final status for asynchronous macOS drag sessions.
func (a *App) handleResultDragEnded(status woxui.FileDragStatus) {
	pending := a.pendingResultFileDrag
	a.pendingResultFileDrag = pendingResultFileDrag{}
	a.finishResultFileDrag(status, pending)
}

func (a *App) finishResultFileDrag(status woxui.FileDragStatus, pending pendingResultFileDrag) {
	// Notify after the OS drag has already finished. OnDragOut cannot block it.
	if endedStatus, ok := dragOutStatus(status); ok {
		if pending.notify != nil {
			pending.notify(plugin.DragOutEvent{ResultId: pending.resultID, Files: append([]string(nil), pending.files...), Status: endedStatus})
		}
	}
	if status == woxui.FileDragStatusPending {
		return
	}
	shouldHide := hideLauncherAfterResultDrag(status, pending.preventHide)
	util.GetLogger().Info(a.chatImportLifecycleCtx(), fmt.Sprintf(
		"result file drag end result=%s status=%v preventHide=%v hide=%v",
		pending.resultID, status, pending.preventHide, shouldHide,
	))
	if shouldHide {
		a.endResultFileDrag()
		if err := a.hideWindow(true); err != nil {
			util.GetLogger().Warn(a.chatImportLifecycleCtx(), fmt.Sprintf("hide launcher after result file drag: %v", err))
		}
		return
	}
	// Native focus events may bounce or be suppressed. Wait for actual launcher input.
	a.resultFileDragHeld = true
}

// beginResultFileDrag suspends hide-on-blur. Drag-out visibility is decided by
// PreventHideAfterDrag after the OS session ends, not by losing focus.
func (a *App) beginResultFileDrag() {
	a.resultFileDragActive = true
	a.resultFileDragHeld = false
	if a.window != nil {
		_ = a.window.SetHideOnBlur(false)
	}
}

func (a *App) endResultFileDrag() {
	a.resultFileDragActive = false
	a.resultFileDragHeld = false
	if a.window == nil || !a.show.HideOnBlur {
		return
	}
	_ = a.window.SetHideOnBlur(true)
}

// releaseResultFileDragHold restores normal blur behavior when the user interacts
// with Wox again, without relying on focus events swallowed by the native drag loop.
func (a *App) releaseResultFileDragHold() {
	if a.resultFileDragHeld {
		a.endResultFileDrag()
	}
}

// hideLauncherAfterResultDrag reports whether a completed file drag should
// dismiss Wox. Plugins opt out with PreventHideAfterDrag. Dropping the payload
// back onto Wox leaves the launcher visible.
func hideLauncherAfterResultDrag(status woxui.FileDragStatus, preventHide bool) bool {
	if preventHide || status == woxui.FileDragStatusCancelInSource {
		return false
	}
	return true
}

func dragOutStatus(status woxui.FileDragStatus) (plugin.DragOutStatus, bool) {
	switch status {
	case woxui.FileDragStatusSuccess:
		return plugin.DragOutStatusSuccess, true
	case woxui.FileDragStatusCancel:
		return plugin.DragOutStatusCancel, true
	case woxui.FileDragStatusCancelInSource:
		return plugin.DragOutStatusCancelInSource, true
	default:
		return "", false
	}
}
