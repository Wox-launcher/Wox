package launcher

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	woxui "github.com/Wox-launcher/wox.ui.go"
	"github.com/Wox-launcher/wox.ui.go/coreclient"
)

type triggerConflictPreviewPlugin struct {
	PluginID        string   `json:"PluginId"`
	PluginName      string   `json:"PluginName"`
	TriggerKeywords []string `json:"TriggerKeywords"`
}

type triggerConflictPreviewData struct {
	Keyword string                         `json:"Keyword"`
	Title   string                         `json:"Title"`
	Message string                         `json:"Message"`
	Plugins []triggerConflictPreviewPlugin `json:"Plugins"`
}

type triggerConflictPreviewState struct {
	formFieldsState
	key       string
	keyword   string
	title     string
	message   string
	pluginIDs []string
	initial   map[string]string
	saving    bool
	error     string
	revision  uint64
}

type triggerConflictPreviewSnapshot struct {
	formFieldsSnapshot
	key     string
	keyword string
	title   string
	message string
	initial map[string]string
	saving  bool
	error   string
}

// ensureTriggerConflictPreview reuses the shared form engine for the current conflict payload.
func (a *App) ensureTriggerConflictPreview(result queryResult, preview queryPreview) (*triggerConflictPreviewSnapshot, error) {
	var data triggerConflictPreviewData
	if err := json.Unmarshal([]byte(preview.PreviewData), &data); err != nil {
		return nil, fmt.Errorf("decode trigger keyword conflict: %w", err)
	}
	if len(data.Plugins) == 0 {
		return nil, fmt.Errorf("trigger keyword conflict has no plugins")
	}
	hash := sha256.Sum256([]byte(preview.PreviewData))
	key := fmt.Sprintf("%s|%s|%x", result.QueryID, result.ID, hash)

	a.mu.Lock()
	if a.triggerConflict == nil || a.triggerConflict.key != key {
		definitions := make([]formDefinition, 0, len(data.Plugins))
		values := make(map[string]string, len(data.Plugins))
		initial := make(map[string]string, len(data.Plugins))
		pluginIDs := make([]string, 0, len(data.Plugins))
		for _, plugin := range data.Plugins {
			if plugin.PluginID == "" {
				continue
			}
			label := plugin.PluginName
			if label == "" {
				label = plugin.PluginID
			}
			value := strings.Join(plugin.TriggerKeywords, ", ")
			definitions = append(definitions, formDefinition{Type: "textbox", Value: formDefinitionValue{Key: plugin.PluginID, Label: label, Tooltip: "Comma-separated trigger keywords"}})
			pluginIDs = append(pluginIDs, plugin.PluginID)
			values[plugin.PluginID] = value
			initial[plugin.PluginID] = value
		}
		if len(definitions) == 0 {
			a.mu.Unlock()
			return nil, fmt.Errorf("trigger keyword conflict has no valid plugin ids")
		}
		fields := newFormFieldsState(definitions, values, false)
		a.triggerConflict = &triggerConflictPreviewState{
			formFieldsState: fields,
			key:             key,
			keyword:         data.Keyword,
			title:           data.Title,
			message:         data.Message,
			pluginIDs:       pluginIDs,
			initial:         initial,
		}
	}
	snapshot := snapshotTriggerConflictPreviewLocked(a.triggerConflict)
	a.mu.Unlock()
	return snapshot, nil
}

func snapshotTriggerConflictPreviewLocked(state *triggerConflictPreviewState) *triggerConflictPreviewSnapshot {
	if state == nil {
		return nil
	}
	initial := make(map[string]string, len(state.initial))
	for key, value := range state.initial {
		initial[key] = value
	}
	return &triggerConflictPreviewSnapshot{
		formFieldsSnapshot: snapshotFormFieldsLocked(&state.formFieldsState),
		key:                state.key,
		keyword:            state.keyword,
		title:              state.title,
		message:            state.message,
		initial:            initial,
		saving:             state.saving,
		error:              state.error,
	}
}

func parseTriggerKeywords(value string) []string {
	parts := strings.Split(value, ",")
	keywords := make([]string, 0, len(parts))
	for _, part := range parts {
		if keyword := strings.TrimSpace(part); keyword != "" {
			keywords = append(keywords, keyword)
		}
	}
	return keywords
}

// onTriggerConflictPreviewKey keeps editing portable and leaves query focus on Escape.
func (a *App) onTriggerConflictPreviewKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	state := a.triggerConflict
	active := state != nil && state.active
	a.mu.RUnlock()
	if !active {
		return false
	}
	if event.Key == woxui.KeyEscape {
		a.deactivateTriggerConflictPreview()
		return true
	}
	if event.Key == woxui.KeyEnter && event.Modifiers.HasPrimary() {
		a.submitTriggerConflictPreview()
		return true
	}
	switch event.Key {
	case woxui.KeyTab, woxui.KeyArrowDown:
		delta := 1
		if event.Key == woxui.KeyTab && event.Modifiers&woxui.KeyModifierShift != 0 {
			delta = -1
		}
		a.moveTriggerConflictFocus(delta)
	case woxui.KeyArrowUp:
		a.moveTriggerConflictFocus(-1)
	default:
		a.editTriggerConflictKey(event)
	}
	return true
}

func (a *App) onTriggerConflictPreviewTextInput(event woxui.TextInputEvent) bool {
	a.mu.Lock()
	state := a.triggerConflict
	if state == nil || !state.active {
		a.mu.Unlock()
		return false
	}
	if state.editor != nil && state.focused >= 0 && state.focused < len(state.definitions) {
		if state.editor.HandleTextInput(event) {
			syncFormFieldsEditorLocked(&state.formFieldsState)
			state.error = ""
		}
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
	return true
}

func (a *App) editTriggerConflictKey(event woxui.KeyEvent) {
	a.mu.Lock()
	if state := a.triggerConflict; state != nil && state.active && state.editor != nil && state.focused >= 0 && state.focused < len(state.definitions) {
		_, changed := handleFormEditorKey(state.editor, state.definitions[state.focused], event)
		if changed {
			syncFormFieldsEditorLocked(&state.formFieldsState)
			state.error = ""
		}
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) moveTriggerConflictFocus(delta int) {
	a.mu.Lock()
	state := a.triggerConflict
	if state == nil || len(state.definitions) == 0 {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	index := state.focused
	for step := 0; step < len(state.definitions); step++ {
		index = (index + delta + len(state.definitions)) % len(state.definitions)
		if formDefinitionFocusable(state.definitions[index]) {
			setFormFieldsFocusLocked(&state.formFieldsState, index)
			break
		}
	}
	textInput := state.editor != nil
	a.mu.Unlock()
	a.updateFormTextInput(textInput)
	_ = a.window.Invalidate()
}

func (a *App) focusTriggerConflictField(index int) {
	a.mu.Lock()
	state := a.triggerConflict
	if state == nil || state.saving || index < 0 || index >= len(state.definitions) {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	state.error = ""
	textInput := state.editor != nil
	a.mu.Unlock()
	a.updateFormTextInput(textInput)
	_ = a.window.Invalidate()
}

func (a *App) setTriggerConflictCaret(index, offset int) {
	a.mu.Lock()
	state := a.triggerConflict
	if state == nil || !state.active || state.focused != index || state.editor == nil {
		a.mu.Unlock()
		return
	}
	state.editor.SetCaret(offset)
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) deactivateTriggerConflictPreview() {
	a.mu.Lock()
	wasActive := a.triggerConflict != nil && a.triggerConflict.active
	if wasActive {
		syncFormFieldsEditorLocked(&a.triggerConflict.formFieldsState)
		a.triggerConflict.active = false
	}
	a.mu.Unlock()
	if !wasActive {
		return
	}
	a.restoreQueryTextInput()
	_ = a.window.Invalidate()
}

func (a *App) setTriggerConflictViewport(key string, height float32) {
	a.mu.Lock()
	if state := a.triggerConflict; state != nil && state.key == key {
		state.viewportHeight = max(float32(1), height)
		state.scroll = min(state.scroll, max(float32(0), formDefinitionsContentHeight(state.definitions)-state.viewportHeight))
	}
	a.mu.Unlock()
}

func (a *App) scrollTriggerConflictPreview(key string, delta float32) {
	a.mu.Lock()
	state := a.triggerConflict
	if state == nil || state.key != key {
		a.mu.Unlock()
		return
	}
	maxOffset := max(float32(0), formDefinitionsContentHeight(state.definitions)-state.viewportHeight)
	state.scroll = min(max(float32(0), state.scroll+delta), maxOffset)
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

// submitTriggerConflictPreview persists only changed plugins through the existing settings endpoint.
func (a *App) submitTriggerConflictPreview() {
	a.mu.Lock()
	state := a.triggerConflict
	if state == nil || state.saving {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	changes := make(map[string]string)
	for _, pluginID := range state.pluginIDs {
		value := state.values[pluginID]
		keywords := parseTriggerKeywords(value)
		if len(keywords) == 0 {
			state.error = "Trigger keywords cannot be empty."
			a.mu.Unlock()
			_ = a.window.Invalidate()
			return
		}
		normalized := strings.Join(keywords, ",")
		if normalized != strings.Join(parseTriggerKeywords(state.initial[pluginID]), ",") {
			changes[pluginID] = normalized
		}
	}
	if len(changes) == 0 {
		state.active = false
		a.mu.Unlock()
		a.restoreQueryTextInput()
		_ = a.window.Invalidate()
		return
	}
	state.saving = true
	state.active = false
	state.error = ""
	state.revision++
	revision := state.revision
	key := state.key
	pluginIDs := append([]string(nil), state.pluginIDs...)
	a.mu.Unlock()
	a.restoreQueryTextInput()
	_ = a.window.Invalidate()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var saveErr error
		for _, pluginID := range pluginIDs {
			value, changed := changes[pluginID]
			if !changed {
				continue
			}
			if err := a.client.Post(ctx, "/setting/plugin/update", map[string]string{"PluginId": pluginID, "Key": "TriggerKeywords", "Value": value}, nil); err != nil {
				saveErr = fmt.Errorf("save %s: %w", pluginID, err)
				break
			}
		}
		a.mu.Lock()
		current := a.triggerConflict != nil && a.triggerConflict.key == key && a.triggerConflict.revision == revision
		if current {
			a.triggerConflict.saving = false
			if saveErr != nil {
				a.triggerConflict.error = saveErr.Error()
			}
		}
		a.mu.Unlock()
		if saveErr != nil {
			log.Printf("save trigger keyword conflict: %v", saveErr)
			_ = a.window.Invalidate()
			return
		}

		a.mu.RLock()
		query := a.query
		a.mu.RUnlock()
		query.QueryID = coreclient.NewID()
		a.setQuery(query)
		if err := a.sendCurrentQuery(); err != nil {
			log.Printf("refresh query after trigger keyword conflict: %v", err)
		}
		if err := a.applyWindowBounds(); err != nil {
			log.Printf("resize launcher after trigger keyword conflict: %v", err)
		}
	}()
}
