package launcher

import (
	"fmt"
	"log"

	"wox/plugin"
	"wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	"wox/util"
)

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
}

// startResultDrag exports the selected result's file payload and follows Flutter's hide policy.
func (a *App) startResultDrag(index int) {
	if index < 0 || index >= len(a.results) || a.results[index].IsGroup || !a.results[index].DragData.isFiles() || a.window == nil {
		return
	}
	status, err := a.window.StartFileDrag(append([]string(nil), a.results[index].DragData.Files...))
	if err != nil || status == woxui.FileDragStatusCancelInSource {
		return
	}
	if status == woxui.FileDragStatusPending {
		return
	}
	if err := a.hideWindow(true); err != nil {
		log.Printf("hide launcher after result file drag: %v", err)
	}
}

// handleResultDragEnded applies the final status for asynchronous macOS drag sessions.
func (a *App) handleResultDragEnded(status woxui.FileDragStatus) {
	if status == woxui.FileDragStatusCancelInSource || status == woxui.FileDragStatusPending {
		return
	}
	if err := a.hideWindow(true); err != nil {
		log.Printf("hide launcher after macOS result file drag: %v", err)
	}
}
