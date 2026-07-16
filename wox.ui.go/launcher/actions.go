package launcher

import (
	"log"

	woxui "github.com/Wox-launcher/wox.ui.go"
)

func (a *App) onActionKey(event woxui.KeyEvent) bool {
	if event.Key == woxui.Key("j") && event.Modifiers.HasPrimary() {
		a.toggleActionPanel()
		return true
	}
	a.mu.RLock()
	open := a.actionPanel
	a.mu.RUnlock()
	if !open {
		return false
	}
	switch event.Key {
	case woxui.KeyEscape:
		a.toggleActionPanel()
	case woxui.KeyArrowUp:
		a.moveActionSelection(-1)
	case woxui.KeyArrowDown:
		a.moveActionSelection(1)
	case woxui.KeyEnter:
		a.activateSelectedAction()
	default:
		return false
	}
	return true
}

func (a *App) toggleActionPanel() {
	a.mu.Lock()
	if a.actionPanel {
		a.actionPanel = false
		a.actionSelected = 0
		a.mu.Unlock()
		_ = a.applyWindowBounds()
		_ = a.window.Invalidate()
		return
	}
	if a.selected < 0 || a.selected >= len(a.results) || a.results[a.selected].IsGroup || len(a.results[a.selected].Actions) == 0 {
		a.mu.Unlock()
		return
	}
	a.actionPanel = true
	a.actionSelected = 0
	a.mu.Unlock()
	_ = a.applyWindowBounds()
	_ = a.window.Invalidate()
}

func (a *App) moveActionSelection(delta int) {
	a.mu.Lock()
	if !a.actionPanel || a.selected < 0 || a.selected >= len(a.results) {
		a.mu.Unlock()
		return
	}
	actions := a.results[a.selected].Actions
	if len(actions) == 0 {
		a.mu.Unlock()
		return
	}
	a.actionSelected = (a.actionSelected + delta + len(actions)) % len(actions)
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) selectAction(index int) {
	a.mu.Lock()
	if a.actionPanel && a.selected >= 0 && a.selected < len(a.results) && index >= 0 && index < len(a.results[a.selected].Actions) {
		a.actionSelected = index
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) activateSelectedAction() {
	a.mu.RLock()
	resultIndex := a.selected
	actionIndex := a.actionSelected
	a.mu.RUnlock()
	a.activateAction(resultIndex, actionIndex)
}

// activateResultActionByID resolves preview-owned controls against the latest result snapshot.
func (a *App) activateResultActionByID(queryID, resultID, actionID string) {
	a.mu.RLock()
	resultIndex := -1
	actionIndex := -1
	for index, result := range a.results {
		if result.QueryID != queryID || result.ID != resultID {
			continue
		}
		resultIndex = index
		for candidate, action := range result.Actions {
			if action.ID == actionID {
				actionIndex = candidate
				break
			}
		}
		break
	}
	a.mu.RUnlock()
	if resultIndex >= 0 && actionIndex >= 0 {
		a.activateAction(resultIndex, actionIndex)
	}
}

func (a *App) activateAction(resultIndex, actionIndex int) {
	a.mu.RLock()
	if resultIndex < 0 || resultIndex >= len(a.results) || actionIndex < 0 || actionIndex >= len(a.results[resultIndex].Actions) || a.results[resultIndex].IsGroup {
		a.mu.RUnlock()
		return
	}
	result := a.results[resultIndex]
	action := result.Actions[actionIndex]
	a.mu.RUnlock()
	if action.ID == enterChatModeActionID && result.Preview.PreviewType == "chat" {
		a.enterChatMode()
		return
	}
	if action.Type == "form" {
		a.openFormAction(result, action)
		return
	}
	if action.Type == "local" {
		log.Printf("Go UI local action %q is not implemented yet", action.ID)
		return
	}
	if _, err := a.client.SendRequest("Action", map[string]any{"resultId": result.ID, "actionId": action.ID, "queryId": result.QueryID}); err != nil {
		log.Printf("execute result action: %v", err)
		return
	}
	a.mu.Lock()
	a.actionPanel = false
	a.actionSelected = 0
	a.mu.Unlock()
	_ = a.applyWindowBounds()
	_ = a.window.Invalidate()
	if !action.PreventHideAfterAction {
		go func() {
			if err := a.hideWindow(true); err != nil {
				log.Printf("hide launcher after action: %v", err)
			}
		}()
	}
}
