package launcher

import (
	"context"
	"log"
	"strings"

	woxui "wox/ui/runtime"
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
		a.mu.Lock()
		if !a.actionPanel || a.actionFilter == nil {
			a.mu.Unlock()
			return false
		}
		handled, changed := a.actionFilter.HandleKey(event)
		if changed {
			a.actionScroll = 0
			a.normalizeActionSelectionLocked()
		}
		a.mu.Unlock()
		if handled {
			if changed {
				_ = a.applyWindowBounds()
			}
			_ = a.window.Invalidate()
		}
		return handled
	}
	return true
}

func (a *App) toggleActionPanel() {
	a.mu.Lock()
	if a.actionPanel {
		a.actionPanel = false
		a.actionSelected = 0
		a.actionFilter = nil
		a.actionScroll = 0
		a.mu.Unlock()
		_ = a.applyWindowBounds()
		a.restoreQueryTextInput()
		_ = a.window.Invalidate()
		return
	}
	if a.selected < 0 || a.selected >= len(a.results) || a.results[a.selected].IsGroup || len(a.results[a.selected].Actions) == 0 {
		a.mu.Unlock()
		return
	}
	a.actionPanel = true
	a.actionFilter = woxui.NewTextEditor("")
	a.actionScroll = 0
	a.normalizeActionSelectionLocked()
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
	if a.actionFilter == nil {
		a.mu.Unlock()
		return
	}
	indices := filteredActionIndices(actions, a.actionFilter.State().Text, a.translations)
	if len(indices) == 0 {
		a.mu.Unlock()
		return
	}
	position := 0
	for index, actionIndex := range indices {
		if actionIndex == a.actionSelected {
			position = index
			break
		}
	}
	position = (position + delta + len(indices)) % len(indices)
	a.actionSelected = indices[position]
	a.ensureActionPositionVisibleLocked(position, len(indices))
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) onActionTextInput(event woxui.TextInputEvent) bool {
	a.mu.Lock()
	if !a.actionPanel || a.actionFilter == nil {
		a.mu.Unlock()
		return false
	}
	changed := a.actionFilter.HandleTextInput(event)
	if changed {
		a.actionScroll = 0
		a.normalizeActionSelectionLocked()
	}
	a.mu.Unlock()
	if changed {
		_ = a.applyWindowBounds()
	}
	_ = a.window.Invalidate()
	return true
}

// ensureActionPositionVisibleLocked follows keyboard navigation inside the eight-row action viewport.
func (a *App) ensureActionPositionVisibleLocked(position, count int) {
	viewport := float32(min(count, maxVisibleActions) * actionRowHeight)
	content := float32(count * actionRowHeight)
	top := float32(position * actionRowHeight)
	bottom := top + actionRowHeight
	if top < a.actionScroll {
		a.actionScroll = top
	} else if bottom > a.actionScroll+viewport {
		a.actionScroll = bottom - viewport
	}
	a.actionScroll = min(max(float32(0), a.actionScroll), max(float32(0), content-viewport))
}

// configureActionScroll clamps stale offsets when filtering changes the action count.
func (a *App) configureActionScroll(count int) float32 {
	a.mu.Lock()
	maximum := float32(max(0, count-maxVisibleActions) * actionRowHeight)
	a.actionScroll = min(max(float32(0), a.actionScroll), maximum)
	offset := a.actionScroll
	a.mu.Unlock()
	return offset
}

func (a *App) scrollActions(delta float32, count int) {
	a.mu.Lock()
	maximum := float32(max(0, count-maxVisibleActions) * actionRowHeight)
	a.actionScroll = min(max(float32(0), a.actionScroll+delta), maximum)
	a.mu.Unlock()
}

func (a *App) setActionFilterCaret(offset int) {
	a.mu.Lock()
	if a.actionPanel && a.actionFilter != nil {
		a.actionFilter.SetCaret(offset)
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

// normalizeActionSelectionLocked keeps the selected source index valid after filtering or result updates.
func (a *App) normalizeActionSelectionLocked() {
	if !a.actionPanel || a.actionFilter == nil || a.selected < 0 || a.selected >= len(a.results) {
		return
	}
	indices := filteredActionIndices(a.results[a.selected].Actions, a.actionFilter.State().Text, a.translations)
	if len(indices) == 0 {
		a.actionSelected = -1
		return
	}
	for _, index := range indices {
		if index == a.actionSelected {
			return
		}
	}
	a.actionSelected = indices[0]
}

// filteredActionIndices preserves source indices so activation still addresses core's original action array.
func filteredActionIndices(actions []resultAction, query string, translations map[string]string) []int {
	query = strings.ToLower(strings.TrimSpace(query))
	indices := make([]int, 0, len(actions))
	for index, action := range actions {
		label := action.Name
		if strings.HasPrefix(label, "i18n:") {
			key := strings.TrimPrefix(label, "i18n:")
			if translated := translations[key]; translated != "" {
				label = translated
			}
		}
		if query == "" || strings.Contains(strings.ToLower(label), query) || strings.Contains(strings.ToLower(action.Hotkey), query) {
			indices = append(indices, index)
		}
	}
	return indices
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
	if err := a.services.ExecuteAction(context.Background(), a.sessionID, result.QueryID, result.ID, action.ID); err != nil {
		log.Printf("execute result action: %v", err)
		return
	}
	a.mu.Lock()
	a.actionPanel = false
	a.actionSelected = 0
	a.actionFilter = nil
	a.actionScroll = 0
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
