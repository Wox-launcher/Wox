package launcher

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	"wox/common"
	"wox/setting"
	woxui "wox/ui/runtime"
	"wox/util"
)

func primaryHotkey(key string) string {
	if runtime.GOOS == "darwin" {
		return "command+" + key
	}
	return "control+" + key
}

// actionPanelHotkey returns the configured Action Hotkey, falling back to the new default.
func (a *App) actionPanelHotkey() string {
	if a != nil && a.generalSettings != nil {
		if hotkey := strings.TrimSpace(a.generalSettings.Data().ActionPanelHotkey); hotkey != "" {
			return hotkey
		}
	}
	return setting.DefaultActionPanelHotkey()
}

// refinementHotkey returns the configured filter shortcut or its original platform default.
func (a *App) refinementHotkey() string {
	if a != nil && a.generalSettings != nil {
		if hotkey := strings.TrimSpace(a.generalSettings.Data().RefinementHotkey); hotkey != "" {
			return hotkey
		}
	}
	return primaryHotkey("f")
}

func normalizeToolbarHotkey(hotkey string) string {
	return strings.ToLower(strings.ReplaceAll(hotkey, " ", ""))
}

type toolbarMessage struct {
	IsCurrent     func() bool
	ID            string                 `json:"Id"`
	Title         string                 `json:"Title"`
	Icon          woxImage               `json:"Icon"`
	Progress      *int                   `json:"Progress"`
	Indeterminate bool                   `json:"Indeterminate"`
	Actions       []toolbarMessageAction `json:"Actions"`
	System        bool
}

type toolbarMessageAction struct {
	SettingsPath           string
	SettingsParam          string
	ID                     string            `json:"Id"`
	Name                   string            `json:"Name"`
	Icon                   woxImage          `json:"Icon"`
	Hotkey                 string            `json:"Hotkey"`
	IsDefault              bool              `json:"IsDefault"`
	PreventHideAfterAction bool              `json:"PreventHideAfterAction"`
	ContextData            map[string]string `json:"ContextData"`
}

// systemToolbarEntry belongs to one launcher UI thread; later equal-priority messages win.
type systemToolbarEntry struct {
	message  toolbarMessage
	priority common.SystemToolbarPriority
	revision uint64
}

func (a *toolbarController) effectiveSystemToolbarMessage() *toolbarMessage {
	var selected *systemToolbarEntry
	for _, entry := range a.systemMessages {
		if selected == nil || entry.priority > selected.priority || (entry.priority == selected.priority && entry.revision > selected.revision) {
			selected = entry
		}
	}
	if selected == nil {
		return nil
	}
	return &selected.message
}

func (a *toolbarController) effectiveToolbarMessage() *toolbarMessage {
	if message := a.effectiveSystemToolbarMessage(); message != nil {
		return message
	}
	if a.toolbarMsg != nil && (a.toolbarMsg.IsCurrent == nil || a.toolbarMsg.IsCurrent()) {
		return a.toolbarMsg
	}
	return nil
}

// applySystemToolbarMessage saves each system message independently and starts its own expiry.
func (a *toolbarController) applySystemToolbarMessage(message toolbarMessage, priority common.SystemToolbarPriority, duration time.Duration) {
	if message.ID == "" {
		return
	}
	before := a.effectiveToolbarMessage()
	if a.systemMessages == nil {
		a.systemMessages = make(map[string]*systemToolbarEntry)
	}
	a.systemMessageRevision++
	revision := a.systemMessageRevision
	a.systemMessages[message.ID] = &systemToolbarEntry{message: message, priority: priority, revision: revision}
	a.systemToolbarMessageChanged(before)
	if duration <= 0 {
		return
	}
	util.Go(a.lifecycleCtx, "expire system toolbar message", func() {
		timer := time.NewTimer(duration)
		defer timer.Stop()
		select {
		case <-a.lifecycleCtx.Done():
			return
		case <-timer.C:
		}
		if err := a.runOnUI("expire system toolbar message", func() {
			if a.lifecycleCtx.Err() == nil {
				a.expireSystemToolbarMessage(message.ID, revision)
			}
		}); err != nil {
			util.GetLogger().Warn(a.lifecycleCtx, "expire system toolbar message: "+err.Error())
		}
	})
}

// expireSystemToolbarMessage ignores timers for a replaced message with the same ID.
func (a *toolbarController) expireSystemToolbarMessage(id string, revision uint64) {
	entry := a.systemMessages[id]
	if entry == nil || entry.revision != revision {
		return
	}
	a.clearSystemToolbarMessage(id)
}

func (a *toolbarController) clearSystemToolbarMessage(id string) {
	if a.systemMessages[id] == nil {
		return
	}
	before := a.effectiveToolbarMessage()
	delete(a.systemMessages, id)
	a.systemToolbarMessageChanged(before)
}

// systemToolbarMessageChanged leaves an open panel alone when a lower-priority message changes.
func (a *toolbarController) systemToolbarMessageChanged(before *toolbarMessage) {
	if before != a.effectiveToolbarMessage() {
		if a.actions.actionPanel && a.actions.actionPanelPurpose == actionPanelPurposeToolbar {
			a.actions.hideActionPanel()
		}
		a.messageTruncated = false
	}
	a.toolbarRevision++
	_ = a.applyWindowBounds()
	_ = a.window().Invalidate()
}

func (a *toolbarController) applyToolbarMessage(message toolbarMessage) {
	if a.effectiveSystemToolbarMessage() != nil {
		a.toolbarMsg = &message
		a.toolbarRevision++
		_ = a.window().Invalidate()
		return
	}
	wasVisible := a.effectiveToolbarMessage() != nil
	if a.actions.actionPanel && a.actions.actionPanelPurpose == actionPanelPurposeToolbar && (a.toolbarMsg == nil || a.toolbarMsg.ID != message.ID) {
		a.actions.hideActionPanel()
	}
	if a.toolbarMsg == nil || a.toolbarMsg.ID != message.ID {
		a.messageTruncated = false
	}
	a.toolbarRevision++
	a.toolbarMsg = &message
	if a.actions.actionPanel && a.actions.actionPanelPurpose == actionPanelPurposeToolbar {
		a.actions.normalizeActionSelectionLocked()
	}
	panelVisible := a.actions.actionPanel
	panelClosed := false
	// About Menu defers presenting the latest toolbar message until it closes.
	if a.actions.shouldSyncActionPanelWithResults() {
		if len(a.actions.currentActionPanelEntries()) == 0 {
			panelClosed = a.actions.resetActionPanelLocked()
		} else {
			a.actions.normalizeActionSelectionLocked()
		}
	}
	// A newly visible status needs its own footer space even with no results.
	if panelVisible || wasVisible != (a.effectiveToolbarMessage() != nil) {
		_ = a.applyWindowBounds()
	}
	if panelClosed {
		a.restoreQueryTextInput()
	}
	_ = a.window().Invalidate()
}

func (a *toolbarController) clearToolbarMessageByID(toolbarMessageID string) {
	changed := false
	panelClosed := false
	if a.toolbarMsg != nil && a.toolbarMsg.ID == toolbarMessageID && a.effectiveSystemToolbarMessage() != nil {
		a.toolbarMsg = nil
		a.toolbarRevision++
		_ = a.window().Invalidate()
		return
	}
	if a.toolbarMsg != nil && a.toolbarMsg.ID == toolbarMessageID {
		if a.actions.actionPanel && a.actions.actionPanelPurpose == actionPanelPurposeToolbar {
			a.actions.hideActionPanel()
		}
		a.toolbarMsg = nil
		a.messageTruncated = false
		a.toolbarRevision++
		changed = true
		if a.actions.shouldSyncActionPanelWithResults() {
			if len(a.actions.currentActionPanelEntries()) == 0 {
				panelClosed = a.actions.resetActionPanelLocked()
			} else {
				a.actions.normalizeActionSelectionLocked()
			}
		}
	}

	if changed {
		_ = a.applyWindowBounds()
	}
	if panelClosed {
		a.restoreQueryTextInput()
	}
	_ = a.window().Invalidate()
}

// copyToolbarMessage copies the full, untruncated message shown by the launcher.
func (a *App) copyToolbarMessage() {
	message := a.toolbar.effectiveToolbarMessage()
	if message == nil || a.toolbar.writeClipboard == nil {
		return
	}
	if err := a.toolbar.writeClipboard(message.Title); err != nil {
		util.GetLogger().Error(a.lifecycleCtx, fmt.Sprintf("copy toolbar message: %v", err))
		a.actions.aboutMenuError = a.language.translate("i18n:ui_toolbar_copy_failed")
		if a.window != nil {
			_ = a.window.Invalidate()
		}
		return
	}
	a.actions.hideActionPanel()
}

// showFullToolbarMessage opens a persistent, scrollable toast with the actual message text.
func (a *App) showFullToolbarMessage() {
	message := a.toolbar.effectiveToolbarMessage()
	if message == nil || message.Title == "" || a.toolbar.showFullMessage == nil {
		return
	}
	text := message.Title
	a.actions.hideActionPanel()
	util.Go(a.lifecycleCtx, "show full toolbar message", func() {
		if !a.toolbar.showFullMessage(text) {
			util.GetLogger().Error(a.lifecycleCtx, "show full toolbar message failed")
		}
	})
}

func (a *toolbarController) onToolbarKey(event woxui.KeyEvent) bool {
	if a.effectiveSystemToolbarMessage() != nil {
		return false
	}
	message := a.toolbarMsg
	panelVisible := a.actions.actionPanel
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

func (a *toolbarController) activateToolbarAction(action toolbarMessageAction) {
	message := a.toolbarMsg
	if message == nil {
		return
	}
	a.activateToolbarActionForMessage(message.ID, action)
}

// activateToolbarActionForMessage prevents a refreshed toolbar from executing an action from an older panel snapshot.
func (a *toolbarController) activateToolbarActionForMessage(messageID string, action toolbarMessageAction) {
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
	a.actions.hideActionPanel()
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
