package launcher

import (
	"context"
	"log"
	"runtime"
	"strings"
	"time"

	woxui "wox/ui/runtime"
	"wox/util"
)

func primaryHotkey(key string) string {
	if runtime.GOOS == "darwin" {
		return "command+" + key
	}
	return "control+" + key
}

func normalizeToolbarHotkey(hotkey string) string {
	return strings.ToLower(strings.ReplaceAll(hotkey, " ", ""))
}

type toolbarMessage struct {
	ID             string                 `json:"Id"`
	Title          string                 `json:"Title"`
	Text           string                 `json:"Text"`
	Icon           woxImage               `json:"Icon"`
	Progress       *int                   `json:"Progress"`
	Indeterminate  bool                   `json:"Indeterminate"`
	Actions        []toolbarMessageAction `json:"Actions"`
	DisplaySeconds int                    `json:"DisplaySeconds"`
}

type toolbarMessageAction struct {
	ID                     string            `json:"Id"`
	Name                   string            `json:"Name"`
	Icon                   woxImage          `json:"Icon"`
	Hotkey                 string            `json:"Hotkey"`
	IsDefault              bool              `json:"IsDefault"`
	PreventHideAfterAction bool              `json:"PreventHideAfterAction"`
	ContextData            map[string]string `json:"ContextData"`
}

func (m toolbarMessage) persistent() bool {
	return m.ID != "" || m.Title != "" || m.Progress != nil || m.Indeterminate || len(m.Actions) > 0
}

func (m toolbarMessage) displayText() string {
	if m.persistent() {
		return m.Title
	}
	return m.Text
}

func (a *App) applyToolbarMessage(message toolbarMessage) {
	if !message.persistent() && a.toolbarMsg != nil && a.toolbarMsg.persistent() {
		return
	}
	a.toolbarRevision++
	revision := a.toolbarRevision
	a.toolbarMsg = &message
	panelVisible := a.actionPanel
	panelClosed := false
	if panelVisible {
		if len(unifiedActionPanelEntries(a.results, a.selected, a.toolbarMsg)) == 0 {
			panelClosed = a.resetActionPanelLocked()
		} else {
			a.normalizeActionSelectionLocked()
		}
	}
	if panelVisible {
		_ = a.applyWindowBounds()
	}
	if panelClosed {
		a.restoreQueryTextInput()
	}
	_ = a.window.Invalidate()
	if !message.persistent() && message.DisplaySeconds > 0 {
		util.Go(a.lifecycleCtx, "expire transient toolbar message", func() {
			timer := time.NewTimer(time.Duration(message.DisplaySeconds) * time.Second)
			defer timer.Stop()
			<-timer.C
			if err := a.runOnUI("expire transient toolbar message", func() {
				if a.toolbarRevision == revision && a.toolbarMsg != nil && a.toolbarMsg.Text == message.Text {
					a.toolbarMsg = nil
					a.toolbarRevision++
				}
				_ = a.window.Invalidate()
			}); err != nil {
				log.Printf("dispatch toolbar message expiry: %v", err)
			}
		})
	}
}

func (a *App) clearToolbarMessageByID(toolbarMessageID string) {
	changed := false
	panelClosed := false
	if a.toolbarMsg != nil && a.toolbarMsg.ID == toolbarMessageID {
		a.toolbarMsg = nil
		a.toolbarRevision++
		changed = true
		if a.actionPanel {
			if len(unifiedActionPanelEntries(a.results, a.selected, a.toolbarMsg)) == 0 {
				panelClosed = a.resetActionPanelLocked()
			} else {
				a.normalizeActionSelectionLocked()
			}
		}
	}
	if changed {
		_ = a.applyWindowBounds()
	}
	if panelClosed {
		a.restoreQueryTextInput()
	}
	_ = a.window.Invalidate()
}

func (a *App) onToolbarKey(event woxui.KeyEvent) bool {
	message := a.toolbarMsg
	panelVisible := a.actionPanel
	if message == nil || panelVisible {
		return false
	}
	if event.Key == woxui.KeyEnter && event.Modifiers == 0 {
		return false
	}
	for _, action := range message.Actions {
		if hotkeyMatches(action.Hotkey, event) {
			a.activateToolbarAction(action)
			return true
		}
	}
	return false
}

func (a *App) activateToolbarAction(action toolbarMessageAction) {
	message := a.toolbarMsg
	if message == nil {
		return
	}
	a.activateToolbarActionForMessage(message.ID, action)
}

// activateToolbarActionForMessage prevents a refreshed toolbar from executing an action from an older panel snapshot.
func (a *App) activateToolbarActionForMessage(messageID string, action toolbarMessageAction) {
	message := a.toolbarMsg
	if message == nil || message.ID != messageID || messageID == "" || action.ID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := a.services.ExecuteToolbarMessageAction(ctx, a.sessionID, messageID, action.ID)
	cancel()
	if err != nil {
		log.Printf("execute toolbar message action: %v", err)
		return
	}
	a.hideActionPanel()
	if !action.PreventHideAfterAction {
		util.Go(a.lifecycleCtx, "hide launcher after toolbar action", func() {
			if err := a.runOnUI("hide launcher after toolbar action", func() {
				if err := a.hideWindow(true); err != nil {
					log.Printf("hide launcher after toolbar action: %v", err)
				}
			}); err != nil {
				log.Printf("dispatch launcher hide after toolbar action: %v", err)
			}
		})
	}
}
