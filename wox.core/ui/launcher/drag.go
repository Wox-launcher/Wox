package launcher

import (
	"log"
	"strings"

	"wox/ui/launcher/view"
	woxui "wox/ui/runtime"
)

// handleFileDrop turns a native file drop into the same selection query used by Flutter.
func (a *App) handleFileDrop(paths []string) {
	cleaned := make([]string, 0, len(paths))
	for _, path := range paths {
		if path = strings.TrimSpace(path); path != "" {
			cleaned = append(cleaned, path)
		}
	}
	if len(cleaned) == 0 {
		return
	}

	if a.window != nil {
		_, _ = a.window.Show()
	}
	if a.host != nil {
		a.host.RequestFocus(view.LauncherQueryInputKey)
	}
	a.canRecallHistory = false
	a.setQuery(plainQuery{
		QueryID:          newID(),
		QueryType:        "selection",
		QuerySelection:   selection{Type: "file", FilePaths: cleaned},
		QueryRefinements: map[string]string{},
		ContextData:      map[string]string{},
	})
	if err := a.sendCurrentQuery(); err != nil {
		log.Printf("send query after file drop: %v", err)
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
