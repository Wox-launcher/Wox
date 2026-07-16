package launcher

import (
	"encoding/json"
	"log"
	"runtime"
	"strings"
	"time"

	woxui "github.com/Wox-launcher/wox.ui.go"
)

func primaryHotkey(key string) string {
	if runtime.GOOS == "darwin" {
		return "command+" + key
	}
	return "control+" + key
}

// formatHotkeyLabels applies platform labels while keeping each physical key separate.
func formatHotkeyLabels(hotkey string) []string {
	parts := strings.Split(strings.TrimSpace(hotkey), "+")
	labels := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch strings.ToLower(part) {
		case "cmd", "command", "meta":
			if runtime.GOOS == "darwin" {
				part = "Cmd"
			} else if runtime.GOOS == "windows" {
				part = "Win"
			} else {
				part = "Super"
			}
		case "ctrl", "control":
			part = "Ctrl"
		case "alt", "option":
			if runtime.GOOS == "darwin" {
				part = "Option"
			} else {
				part = "Alt"
			}
		case "shift":
			part = "Shift"
		case "enter", "return":
			part = "Enter"
		case "space":
			part = "Space"
		case "escape", "esc":
			part = "Esc"
		case "backquote", "tilde":
			part = "~"
		case "arrowup", "up":
			part = "↑"
		case "arrowdown", "down":
			part = "↓"
		case "arrowleft", "left":
			part = "←"
		case "arrowright", "right":
			part = "→"
		default:
			if len([]rune(part)) == 1 {
				part = strings.ToUpper(part)
			}
		}
		if part != "" {
			labels = append(labels, part)
		}
	}
	return labels
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

func (a *App) showToolbarMessage(raw json.RawMessage) error {
	var message toolbarMessage
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}
	a.mu.Lock()
	if !message.persistent() && a.toolbarMsg != nil && a.toolbarMsg.persistent() {
		a.mu.Unlock()
		return nil
	}
	a.toolbarRevision++
	revision := a.toolbarRevision
	a.toolbarMsg = &message
	a.mu.Unlock()
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
	return nil
}

func (a *App) clearToolbarMessage(raw json.RawMessage) error {
	var params struct {
		ToolbarMessageID string `json:"toolbarMsgId"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return err
	}
	a.mu.Lock()
	if a.toolbarMsg != nil && a.toolbarMsg.ID == params.ToolbarMessageID {
		a.toolbarMsg = nil
		a.toolbarRevision++
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
	return nil
}

func defaultToolbarAction(actions []toolbarMessageAction) (toolbarMessageAction, bool) {
	for _, action := range actions {
		if action.IsDefault {
			return action, true
		}
	}
	if len(actions) > 0 {
		return actions[0], true
	}
	return toolbarMessageAction{}, false
}

func (a *App) onToolbarKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	message := a.toolbarMsg
	resultSelected := a.selected >= 0 && a.selected < len(a.results)
	a.mu.RUnlock()
	if message == nil {
		return false
	}
	if event.Key == woxui.KeyEnter && !resultSelected {
		if action, ok := defaultToolbarAction(message.Actions); ok {
			a.activateToolbarAction(action)
			return true
		}
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
	if message == nil || message.ID == "" || action.ID == "" {
		return
	}
	if _, err := a.client.SendRequest("ToolbarMsgAction", map[string]any{"toolbarMsgId": message.ID, "actionId": action.ID, "contextData": action.ContextData}); err != nil {
		log.Printf("execute toolbar message action: %v", err)
		return
	}
	if !action.PreventHideAfterAction {
		go func() {
			if err := a.hideWindow(true); err != nil {
				log.Printf("hide launcher after toolbar action: %v", err)
			}
		}()
	}
}
