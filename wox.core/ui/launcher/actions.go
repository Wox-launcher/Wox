package launcher

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

const (
	actionRowHeight   = launcherview.ActionRowHeight
	maxVisibleActions = launcherview.MaxVisibleActions
)

type actionPanelSource uint8

const (
	actionPanelSourceResult actionPanelSource = iota
	actionPanelSourceToolbar
)

// actionPanelEntry keeps the unified picker presentation tied to its original execution target.
type actionPanelEntry struct {
	Key                  string
	ID                   string
	Name                 string
	Icon                 woxImage
	Hotkey               string
	IsDefault            bool
	Source               actionPanelSource
	ResultIndex          int
	ActionIndex          int
	ToolbarMessageID     string
	ToolbarMessageAction toolbarMessageAction
}

func actionPanelBaseHeightForPalette(palette uiPalette) float32 {
	return launcherview.ActionPanelBaseHeight(palette.actionPadding)
}

// unifiedActionPanelEntries mirrors Flutter's toolbar-before-plugin action ordering and hotkey conflict handling.
func unifiedActionPanelEntries(results []queryResult, selected int, message *toolbarMessage) []actionPanelEntry {
	toolbarCount := 0
	if message != nil {
		toolbarCount = len(message.Actions)
	}
	resultCount := 0
	if selected >= 0 && selected < len(results) && !results[selected].IsGroup {
		resultCount = len(results[selected].Actions)
	}
	entries := make([]actionPanelEntry, 0, toolbarCount+resultCount)
	reservedHotkeys := make(map[string]struct{}, toolbarCount)
	if message != nil {
		for index, action := range message.Actions {
			entries = append(entries, actionPanelEntry{
				Key: fmt.Sprintf("toolbar:%s:%s:%d", message.ID, action.ID, index), ID: fmt.Sprintf("toolbar-%s-%d", action.ID, index),
				Name: action.Name, Icon: action.Icon, Hotkey: action.Hotkey, IsDefault: action.IsDefault, Source: actionPanelSourceToolbar,
				ToolbarMessageID: message.ID, ToolbarMessageAction: action,
			})
			if hotkey := normalizeToolbarHotkey(action.Hotkey); hotkey != "" {
				reservedHotkeys[hotkey] = struct{}{}
			}
		}
	}
	if selected < 0 || selected >= len(results) || results[selected].IsGroup {
		return entries
	}
	result := results[selected]
	for index, action := range result.Actions {
		hotkey := action.Hotkey
		if _, conflicted := reservedHotkeys[normalizeToolbarHotkey(hotkey)]; conflicted && strings.TrimSpace(hotkey) != "" {
			hotkey = ""
		}
		entries = append(entries, actionPanelEntry{
			Key: fmt.Sprintf("result:%s:%s:%d", result.ID, action.ID, index), ID: fmt.Sprintf("result-%s-%d", action.ID, index),
			Name: action.Name, Icon: action.Icon, Hotkey: hotkey, IsDefault: action.IsDefault, Source: actionPanelSourceResult,
			ResultIndex: selected, ActionIndex: index,
		})
	}
	return entries
}

// buildActionPanel resolves action labels and icons before delegating to the pure panel view.
func (a *App) buildActionPanel(snapshot viewSnapshot, windowWidth, windowHeight, queryHeight, toolbarHeight float32) (woxwidget.Widget, float32, float32) {
	if len(snapshot.actionEntries) == 0 {
		return nil, 0, 0
	}
	items := make([]launcherview.ActionItem, 0, len(snapshot.actionIndices))
	for _, index := range snapshot.actionIndices {
		if index < 0 || index >= len(snapshot.actionEntries) {
			continue
		}
		action := snapshot.actionEntries[index]
		items = append(items, launcherview.ActionItem{
			Index: index, ID: action.ID, Label: a.translate(action.Name), Icon: a.imageFor(action.Icon), HotkeyLabels: formatHotkeyLabels(action.Hotkey),
		})
	}
	scroll := a.configureActionScroll(len(items))
	return launcherview.ActionsView(launcherview.ActionsProps{
		Window: a.window, WindowWidth: windowWidth, WindowHeight: windowHeight, QueryHeight: queryHeight, ToolbarHeight: toolbarHeight,
		Theme: snapshot.palette.componentTheme(), ActionHeader: snapshot.palette.actionHeader,
		ActionQueryBackground: snapshot.palette.actionQueryBackground, ActionQueryText: snapshot.palette.actionQueryText,
		ResultTail: snapshot.palette.resultTail, SelectedTail: snapshot.palette.selectedTail,
		ResultItemRadius: snapshot.palette.resultItemRadius, ActionQueryRadius: snapshot.palette.actionQueryRadius,
		ActionPadding: snapshot.palette.actionPadding, HeaderLabel: a.translate("i18n:ui_actions"), NoMatchesLabel: a.translate("i18n:ui_no_matches"),
		Items: items, Selected: snapshot.actionSelected, Editing: snapshot.actionEditing, Scroll: scroll,
		OnSelect: a.selectAction, OnActivate: a.activateSelectedAction,
		OnScroll: func(delta float32) { a.scrollActions(delta, len(items)) }, OnCaret: a.setActionFilterCaret,
	})
}

func (a *App) onActionKey(event woxui.KeyEvent) bool {
	if toolbarHotkeyMatches(primaryHotkey("j"), event) {
		a.toggleActionPanel()
		return true
	}
	a.mu.RLock()
	open := a.actionPanel
	a.mu.RUnlock()
	if !open {
		return false
	}
	if event.Key == woxui.KeyTab {
		return true
	}
	if event.Modifiers == 0 {
		switch event.Key {
		case woxui.KeyEscape:
			a.hideActionPanel()
			return true
		case woxui.KeyArrowUp:
			a.moveActionSelection(-1)
			return true
		case woxui.KeyArrowDown:
			a.moveActionSelection(1)
			return true
		case woxui.KeyEnter:
			a.activateSelectedAction()
			return true
		}
	}
	if event.Modifiers == woxui.KeyModifierControl {
		switch event.Key {
		case woxui.Key("n"):
			a.moveActionSelection(1)
			return true
		case woxui.Key("p"):
			a.moveActionSelection(-1)
			return true
		}
	}

	a.mu.Lock()
	if !a.actionPanel || a.actionFilter == nil {
		a.mu.Unlock()
		return false
	}
	entries := unifiedActionPanelEntries(a.results, a.selected, a.toolbarMsg)
	indices := filteredActionIndices(entries, a.actionFilter.State().Text, a.translations, a.settings.UsePinYin)
	for _, index := range indices {
		if toolbarHotkeyMatches(entries[index].Hotkey, event) {
			a.actionSelected = index
			a.actionSelectionKey = entries[index].Key
			a.mu.Unlock()
			a.activateSelectedAction()
			return true
		}
	}
	handled, changed := a.actionFilter.HandleKey(event)
	if changed {
		a.actionScroll = 0
		a.selectFirstFilteredActionLocked()
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

func (a *App) toggleActionPanel() {
	a.mu.RLock()
	open := a.actionPanel
	a.mu.RUnlock()
	if open {
		a.hideActionPanel()
		return
	}

	a.mu.Lock()
	if len(unifiedActionPanelEntries(a.results, a.selected, a.toolbarMsg)) == 0 {
		a.mu.Unlock()
		return
	}
	// Flutter dismisses a form action before transferring keyboard ownership to the action filter.
	a.form = nil
	a.actionPanel = true
	a.actionSelected = -1
	a.actionSelectionKey = ""
	a.actionFilter = woxui.NewTextEditor("")
	a.actionScroll = 0
	a.normalizeActionSelectionLocked()
	a.mu.Unlock()
	_ = a.applyWindowBounds()
	_ = a.window.Invalidate()
}

// hideActionPanel clears filter state and returns keyboard ownership to the query editor.
func (a *App) hideActionPanel() bool {
	a.mu.Lock()
	changed := a.resetActionPanelLocked()
	a.mu.Unlock()
	if !changed {
		return false
	}
	_ = a.applyWindowBounds()
	a.restoreQueryTextInput()
	_ = a.window.Invalidate()
	return true
}

func (a *App) resetActionPanelLocked() bool {
	if !a.actionPanel {
		return false
	}
	a.actionPanel = false
	a.actionSelected = 0
	a.actionSelectionKey = ""
	a.actionFilter = nil
	a.actionScroll = 0
	return true
}

func (a *App) moveActionSelection(delta int) {
	a.mu.Lock()
	if !a.actionPanel || a.actionFilter == nil {
		a.mu.Unlock()
		return
	}
	entries := unifiedActionPanelEntries(a.results, a.selected, a.toolbarMsg)
	indices := filteredActionIndices(entries, a.actionFilter.State().Text, a.translations, a.settings.UsePinYin)
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
	a.actionSelectionKey = entries[a.actionSelected].Key
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
		a.selectFirstFilteredActionLocked()
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

// normalizeActionSelectionLocked preserves the same unified action across live result and toolbar refreshes.
func (a *App) normalizeActionSelectionLocked() {
	if !a.actionPanel || a.actionFilter == nil {
		return
	}
	entries := unifiedActionPanelEntries(a.results, a.selected, a.toolbarMsg)
	indices := filteredActionIndices(entries, a.actionFilter.State().Text, a.translations, a.settings.UsePinYin)
	if len(indices) == 0 {
		a.actionSelected = -1
		a.actionSelectionKey = ""
		return
	}
	if a.actionSelectionKey != "" {
		for _, index := range indices {
			if entries[index].Key == a.actionSelectionKey {
				a.actionSelected = index
				return
			}
		}
	}
	for _, index := range indices {
		if entries[index].IsDefault {
			a.actionSelected = index
			a.actionSelectionKey = entries[index].Key
			return
		}
	}
	a.actionSelected = indices[0]
	a.actionSelectionKey = entries[a.actionSelected].Key
}

func (a *App) selectFirstFilteredActionLocked() {
	entries := unifiedActionPanelEntries(a.results, a.selected, a.toolbarMsg)
	indices := filteredActionIndices(entries, a.actionFilter.State().Text, a.translations, a.settings.UsePinYin)
	if len(indices) == 0 {
		a.actionSelected = -1
		a.actionSelectionKey = ""
		return
	}
	a.actionSelected = indices[0]
	a.actionSelectionKey = entries[a.actionSelected].Key
}

// filteredActionIndices matches Flutter's fuzzy title filter while retaining unified source positions.
func filteredActionIndices(actions []actionPanelEntry, query string, translations map[string]string, usePinYin bool) []int {
	query = strings.TrimSpace(query)
	indices := make([]int, 0, len(actions))
	for index, action := range actions {
		label := translatedActionLabel(action.Name, translations)
		if query == "" || util.IsStringMatch(label, query, usePinYin) {
			indices = append(indices, index)
		}
	}
	return indices
}

func translatedActionLabel(value string, translations map[string]string) string {
	if !strings.HasPrefix(value, "i18n:") {
		return value
	}
	key := strings.TrimPrefix(value, "i18n:")
	if translated := translations[key]; translated != "" {
		return translated
	}
	return strings.ReplaceAll(key, "_", " ")
}

func (a *App) selectAction(index int) {
	a.mu.Lock()
	entries := unifiedActionPanelEntries(a.results, a.selected, a.toolbarMsg)
	if a.actionPanel && index >= 0 && index < len(entries) {
		a.actionSelected = index
		a.actionSelectionKey = entries[index].Key
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) activateSelectedAction() {
	a.mu.RLock()
	entries := unifiedActionPanelEntries(a.results, a.selected, a.toolbarMsg)
	selected := a.actionSelected
	if selected < 0 || selected >= len(entries) {
		a.mu.RUnlock()
		return
	}
	entry := entries[selected]
	a.mu.RUnlock()
	a.activateActionPanelEntry(entry)
}

func (a *App) activateActionPanelEntry(entry actionPanelEntry) {
	if entry.Source == actionPanelSourceToolbar {
		a.activateToolbarActionForMessage(entry.ToolbarMessageID, entry.ToolbarMessageAction)
		return
	}
	a.activateAction(entry.ResultIndex, entry.ActionIndex)
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
		a.hideActionPanel()
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
	a.hideActionPanel()
	if !action.PreventHideAfterAction {
		go func() {
			if err := a.hideWindow(true); err != nil {
				log.Printf("hide launcher after action: %v", err)
			}
		}()
	}
}

// onQueryFocusChanged mirrors Flutter's query-focus notification after panel focus returns.
func (a *App) onQueryFocusChanged(focused bool) {
	if !focused {
		return
	}
	a.mu.RLock()
	formVisible := a.form != nil
	a.mu.RUnlock()
	if formVisible {
		return
	}
	a.hideActionPanel()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.services.QueryBoxFocused(ctx, a.sessionID); err != nil {
			log.Printf("notify query box focus: %v", err)
		}
	}()
}
