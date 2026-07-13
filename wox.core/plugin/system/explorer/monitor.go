package explorer

import (
	"strings"
	"sync/atomic"
	"wox/util"
	"wox/util/keyboard"
)

const explorerOpenSearchEventKey = "open-search"

// isExplorerOpenSearchShortcut matches the platform-specific dialog keyboard equivalent of clicking the Wox hint.
func isExplorerOpenSearchShortcut(event keyboard.RawKeyEvent) bool {
	requiredModifier := keyboard.ModifierCtrl
	blockedModifiers := keyboard.ModifierShift | keyboard.ModifierAlt | keyboard.ModifierSuper
	if util.IsMacOS() {
		requiredModifier = keyboard.ModifierSuper
		blockedModifiers = keyboard.ModifierShift | keyboard.ModifierAlt | keyboard.ModifierCtrl
	}

	return event.Type == keyboard.EventTypeKeyDown &&
		(event.Key == keyboard.KeyG || strings.EqualFold(event.Character, "g")) &&
		event.Modifiers&requiredModifier != 0 &&
		event.Modifiers&blockedModifiers == 0
}

// ExplorerRawKeyListener observes raw keys while the native file explorer or an
// open/save dialog is the active file-selection surface. Returning true consumes
// the key when the platform raw-key backend supports consumption.
type ExplorerRawKeyListener func(event keyboard.RawKeyEvent) bool

// ExplorerRawKeySubscription removes a raw-key listener registered with the
// explorer monitor.
type ExplorerRawKeySubscription interface {
	Close() error
}

var monitorLogger atomic.Value // func(msg string)

func setExplorerMonitorLogger(logger func(msg string)) {
	if logger == nil {
		monitorLogger.Store((func(string))(nil))
		return
	}
	monitorLogger.Store(logger)
}

func logFromMonitor(msg string) {
	if v := monitorLogger.Load(); v != nil {
		if fn, ok := v.(func(string)); ok && fn != nil {
			fn(msg)
		}
	}
}
