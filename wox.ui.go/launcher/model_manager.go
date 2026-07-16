package launcher

import (
	"context"
	"fmt"
	"strings"
	"time"

	woxui "github.com/Wox-launcher/wox.ui.go"
)

const modelManagerRowHeight = float32(82)

type modelEngineStatus struct {
	Known    bool
	State    string `json:"State"`
	Progress int    `json:"Progress"`
	Error    string `json:"Error"`
	Ready    bool   `json:"Ready"`
}

type modelManagerState struct {
	kind        string
	target      *formFieldsState
	fieldIndex  int
	options     []formOption
	selected    string
	selectedRow int
	engine      modelEngineStatus
	loading     bool
	busy        string
	error       string
	scroll      float32
	viewport    float32
}

type modelManagerSnapshot struct {
	kind        string
	options     []formOption
	selected    string
	selectedRow int
	engine      modelEngineStatus
	loading     bool
	busy        string
	error       string
	scroll      float32
}

func snapshotModelManagerLocked(state *modelManagerState) *modelManagerSnapshot {
	if state == nil {
		return nil
	}
	return &modelManagerSnapshot{
		kind: state.kind, options: append([]formOption(nil), state.options...), selected: state.selected, selectedRow: state.selectedRow,
		engine: state.engine, loading: state.loading, busy: state.busy, error: state.error, scroll: state.scroll,
	}
}

func modelOptionID(option formOption) string {
	if option.ID != "" {
		return option.ID
	}
	return option.Value
}

func modelOptionLabel(option formOption) string {
	if option.DisplayName != "" {
		return option.DisplayName
	}
	if option.Label != "" {
		return option.Label
	}
	return modelOptionID(option)
}

func modelStatusLabel(option formOption) string {
	switch option.Status {
	case "downloading":
		return fmt.Sprintf("Downloading · %d%%", option.DownloadProgress)
	case "extracting":
		return fmt.Sprintf("Extracting · %d%%", option.DownloadProgress)
	case "finalizing":
		return "Finalizing"
	case "downloaded":
		return "Downloaded"
	case "failed":
		if option.Error != "" {
			return "Failed · " + option.Error
		}
		return "Failed"
	default:
		if option.SizeMB > 0 {
			return fmt.Sprintf("Not downloaded · %d MB", option.SizeMB)
		}
		return "Not downloaded"
	}
}

func modelOptionUsable(kind string, option formOption) bool {
	if option.Status != "downloaded" {
		return false
	}
	return kind != "ocrModel" || option.Available
}

func modelManagerNeedsPoll(state *modelManagerState) bool {
	if state == nil {
		return false
	}
	if state.engine.State == "downloading" || state.engine.State == "extracting" || state.engine.State == "finalizing" {
		return true
	}
	for _, option := range state.options {
		if option.Status == "downloading" || option.Status == "extracting" || option.Status == "finalizing" {
			return true
		}
	}
	return false
}

// openPluginModelManager binds the overlay to the current plugin form without exposing model routes to widgets.
func (a *App) openPluginModelManager(index int) {
	a.stopHotkeyRecording()
	a.mu.Lock()
	state := a.pluginForm
	if state == nil || state.saving || index < 0 || index >= len(state.definitions) {
		a.mu.Unlock()
		return
	}
	definition := state.definitions[index]
	if definition.Type != "dictationModel" && definition.Type != "ocrModel" {
		a.mu.Unlock()
		return
	}
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	selected := state.values[definition.Value.Key]
	selectedRow := 0
	for optionIndex, option := range definition.Value.Options {
		if modelOptionID(option) == selected {
			selectedRow = optionIndex
			break
		}
	}
	manager := &modelManagerState{
		kind: definition.Type, target: &state.formFieldsState, fieldIndex: index, options: append([]formOption(nil), definition.Value.Options...),
		selected: selected, selectedRow: selectedRow, viewport: 400,
	}
	a.modelManager = manager
	a.mu.Unlock()
	_ = a.window.SetTextInputState(woxui.TextInputState{})
	_ = a.window.Invalidate()
	go a.refreshModelManager(manager)
}

func (a *App) modelManagerCurrentLocked(state *modelManagerState) bool {
	return state != nil && a.modelManager == state && a.settingTab == "plugins" && a.pluginForm != nil && state.target == &a.pluginForm.formFieldsState
}

// refreshModelManager merges runtime-only progress into translated definition metadata.
func (a *App) refreshModelManager(state *modelManagerState) {
	a.mu.Lock()
	if !a.modelManagerCurrentLocked(state) || state.loading {
		a.mu.Unlock()
		return
	}
	state.loading = true
	state.error = ""
	kind := state.kind
	a.mu.Unlock()
	_ = a.window.Invalidate()

	statusRoute := "/dictation/model/status"
	engineRoute := "/dictation/native-lib/status"
	if kind == "ocrModel" {
		statusRoute = "/ocr/model/status"
		engineRoute = "/ocr/engine/status"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	var statuses []formOption
	statusErr := a.client.Post(ctx, statusRoute, nil, &statuses)
	var engine modelEngineStatus
	engineErr := a.client.Post(ctx, engineRoute, nil, &engine)
	cancel()
	engine.Known = engineErr == nil

	a.mu.Lock()
	if !a.modelManagerCurrentLocked(state) {
		a.mu.Unlock()
		return
	}
	state.loading = false
	if statusErr == nil {
		mergeModelStatuses(state.options, statuses)
		if state.selected == "" {
			for _, option := range state.options {
				if modelOptionUsable(state.kind, option) {
					state.selected = modelOptionID(option)
					key := state.target.definitions[state.fieldIndex].Value.Key
					state.target.values[key] = state.selected
					break
				}
			}
		}
	}
	if engineErr == nil {
		state.engine = engine
	}
	errors := make([]string, 0, 2)
	if statusErr != nil {
		errors = append(errors, "models: "+statusErr.Error())
	}
	if engineErr != nil {
		errors = append(errors, "engine: "+engineErr.Error())
	}
	state.error = strings.Join(errors, " · ")
	state.target.definitions[state.fieldIndex].Value.Options = append([]formOption(nil), state.options...)
	poll := modelManagerNeedsPoll(state)
	a.mu.Unlock()
	_ = a.window.Invalidate()
	if poll {
		time.AfterFunc(time.Second, func() { a.refreshModelManager(state) })
	}
}

func mergeModelStatuses(options []formOption, statuses []formOption) {
	for _, status := range statuses {
		id := modelOptionID(status)
		for index := range options {
			if modelOptionID(options[index]) != id {
				continue
			}
			options[index].Status = status.Status
			options[index].DownloadProgress = status.DownloadProgress
			options[index].SizeMB = status.SizeMB
			options[index].Error = status.Error
			break
		}
	}
}

func (a *App) closeModelManager() {
	a.mu.Lock()
	state := a.modelManager
	if state == nil {
		a.mu.Unlock()
		return
	}
	if a.pluginForm != nil && state.target == &a.pluginForm.formFieldsState {
		a.pluginForm.active = true
		setFormFieldsFocusLocked(&a.pluginForm.formFieldsState, state.fieldIndex)
	}
	a.modelManager = nil
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) selectModelManagerRow(index int) {
	a.mu.Lock()
	state := a.modelManager
	if state == nil || index < 0 || index >= len(state.options) {
		a.mu.Unlock()
		return
	}
	state.selectedRow = index
	a.ensureModelManagerRowVisibleLocked(state)
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) setModelManagerViewport(height float32) {
	a.mu.Lock()
	if state := a.modelManager; state != nil {
		state.viewport = max(float32(1), height)
		a.ensureModelManagerRowVisibleLocked(state)
	}
	a.mu.Unlock()
}

func (a *App) scrollModelManager(delta float32) {
	a.mu.Lock()
	if state := a.modelManager; state != nil {
		maximum := max(float32(0), float32(len(state.options))*modelManagerRowHeight-state.viewport)
		state.scroll = min(max(float32(0), state.scroll+delta), maximum)
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) ensureModelManagerRowVisibleLocked(state *modelManagerState) {
	if state.selectedRow < 0 || state.selectedRow >= len(state.options) {
		return
	}
	top := float32(state.selectedRow) * modelManagerRowHeight
	bottom := top + modelManagerRowHeight
	if top < state.scroll {
		state.scroll = top
	} else if bottom > state.scroll+state.viewport {
		state.scroll = bottom - state.viewport
	}
	maximum := max(float32(0), float32(len(state.options))*modelManagerRowHeight-state.viewport)
	state.scroll = min(max(float32(0), state.scroll), maximum)
}

func (a *App) chooseManagedModel(index int) {
	a.mu.Lock()
	state := a.modelManager
	if state == nil || state.busy != "" || index < 0 || index >= len(state.options) || !modelOptionUsable(state.kind, state.options[index]) {
		a.mu.Unlock()
		return
	}
	option := state.options[index]
	key := state.target.definitions[state.fieldIndex].Value.Key
	state.target.values[key] = modelOptionID(option)
	state.selected = modelOptionID(option)
	a.mu.Unlock()
	a.closeModelManager()
}

// runModelManagerAction starts core-owned downloads or deletion and leaves progress polling in the shared overlay.
func (a *App) runModelManagerAction(action string, index int) {
	a.mu.Lock()
	state := a.modelManager
	if state == nil || state.busy != "" {
		a.mu.Unlock()
		return
	}
	modelID := ""
	if action != "engine" {
		if index < 0 || index >= len(state.options) {
			a.mu.Unlock()
			return
		}
		modelID = modelOptionID(state.options[index])
	}
	state.busy = action + ":" + modelID
	state.error = ""
	kind := state.kind
	a.mu.Unlock()
	_ = a.window.Invalidate()

	go func() {
		route := "/dictation/model/download"
		payload := any(map[string]string{"modelId": modelID})
		if kind == "ocrModel" {
			route = "/ocr/model/download"
		}
		if action == "delete" {
			route = "/dictation/model/delete"
		} else if action == "engine" {
			payload = nil
			route = "/dictation/native-lib/download"
			if kind == "ocrModel" {
				route = "/ocr/engine/download"
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := a.client.Post(ctx, route, payload, nil)
		cancel()
		a.mu.Lock()
		if !a.modelManagerCurrentLocked(state) {
			a.mu.Unlock()
			return
		}
		state.busy = ""
		if err != nil {
			state.error = err.Error()
		} else if action == "delete" && state.selected == modelID {
			state.selected = ""
			key := state.target.definitions[state.fieldIndex].Value.Key
			state.target.values[key] = ""
		}
		a.mu.Unlock()
		_ = a.window.Invalidate()
		if err == nil {
			go a.refreshModelManager(state)
		}
	}()
}

func (a *App) onModelManagerKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	state := a.modelManager
	selected := -1
	count := 0
	if state != nil {
		selected = state.selectedRow
		count = len(state.options)
	}
	a.mu.RUnlock()
	if state == nil {
		return false
	}
	switch event.Key {
	case woxui.KeyEscape:
		a.closeModelManager()
	case woxui.KeyArrowUp, woxui.KeyArrowDown:
		if count > 0 {
			delta := 1
			if event.Key == woxui.KeyArrowUp {
				delta = -1
			}
			a.selectModelManagerRow((selected + delta + count) % count)
		}
	case woxui.KeyEnter, woxui.KeySpace:
		a.mu.RLock()
		if a.modelManager == state && selected >= 0 && selected < len(state.options) {
			option := state.options[selected]
			usable := modelOptionUsable(state.kind, option)
			status := option.Status
			a.mu.RUnlock()
			if usable {
				a.chooseManagedModel(selected)
			} else if status == "not_downloaded" || status == "failed" || status == "" {
				a.runModelManagerAction("download", selected)
			}
		} else {
			a.mu.RUnlock()
		}
	case woxui.KeyDelete:
		a.mu.RLock()
		canDelete := a.modelManager == state && state.kind == "dictationModel" && selected >= 0 && selected < len(state.options) && state.options[selected].Status == "downloaded"
		a.mu.RUnlock()
		if canDelete {
			a.runModelManagerAction("delete", selected)
		}
	default:
		if event.Modifiers.HasPrimary() && event.Key == woxui.Key("r") {
			go a.refreshModelManager(state)
			return true
		}
		return true
	}
	return true
}
