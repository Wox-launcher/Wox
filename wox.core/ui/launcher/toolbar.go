package launcher

import (
	"context"
	"log"
	"runtime"
	"strings"
	"time"

	woxui "wox/ui/runtime"
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
	a.mu.Lock()
	if !message.persistent() && a.toolbarMsg != nil && a.toolbarMsg.persistent() {
		a.mu.Unlock()
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
	a.mu.Unlock()
	if panelVisible {
		_ = a.applyWindowBounds()
	}
	if panelClosed {
		a.restoreQueryTextInput()
	}
	_ = a.window.Invalidate()
	if !message.persistent() && message.DisplaySeconds > 0 {
		go func() {
			timer := time.NewTimer(time.Duration(message.DisplaySeconds) * time.Second)
			defer timer.Stop()
			<-timer.C
			a.mu.Lock()
			if a.toolbarRevision == revision && a.toolbarMsg != nil && a.toolbarMsg.Text == message.Text {
				a.toolbarMsg = nil
				a.toolbarRevision++
			}
			a.mu.Unlock()
			_ = a.window.Invalidate()
		}()
	}
}

func (a *App) clearToolbarMessageByID(toolbarMessageID string) {
	a.mu.Lock()
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
	a.mu.Unlock()
	if changed {
		_ = a.applyWindowBounds()
	}
	if panelClosed {
		a.restoreQueryTextInput()
	}
	_ = a.window.Invalidate()
}

func (a *App) onToolbarKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	message := a.toolbarMsg
	panelVisible := a.actionPanel
	a.mu.RUnlock()
	if message == nil || panelVisible {
		return false
	}
	if event.Key == woxui.KeyEnter && event.Modifiers == 0 {
		return false
	}
	for _, action := range message.Actions {
		if toolbarHotkeyMatches(action.Hotkey, event) {
			a.activateToolbarAction(action)
			return true
		}
	}
	return false
}

func toolbarHotkeyMatches(hotkey string, event woxui.KeyEvent) bool {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(hotkey)), "+")
	if len(parts) == 0 {
		return false
	}
	key := strings.TrimSpace(parts[len(parts)-1])
	if key == "return" {
		key = string(woxui.KeyEnter)
	}
	if key != string(event.Key) {
		return false
	}
	var expected woxui.KeyModifiers
	for _, modifier := range parts[:len(parts)-1] {
		switch strings.TrimSpace(modifier) {
		case "ctrl", "control":
			expected |= woxui.KeyModifierControl
		case "cmd", "command", "meta":
			expected |= woxui.KeyModifierMeta
		case "alt", "option":
			expected |= woxui.KeyModifierAlt
		case "shift":
			expected |= woxui.KeyModifierShift
		}
	}
	return event.Modifiers == expected
}

func (a *App) activateToolbarAction(action toolbarMessageAction) {
	a.mu.RLock()
	message := a.toolbarMsg
	a.mu.RUnlock()
	if message == nil {
		return
	}
	a.activateToolbarActionForMessage(message.ID, action)
}

// activateToolbarActionForMessage prevents a refreshed toolbar from executing an action from an older panel snapshot.
func (a *App) activateToolbarActionForMessage(messageID string, action toolbarMessageAction) {
	a.mu.RLock()
	message := a.toolbarMsg
	a.mu.RUnlock()
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
		go func() {
			if err := a.hideWindow(true); err != nil {
				log.Printf("hide launcher after toolbar action: %v", err)
			}
		}()
	}
}
