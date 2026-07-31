package launcher

import (
	"context"
	"fmt"
	"strings"
	"time"

	"wox/ui/contract"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

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
	anchor      woxui.Rect
	anchored    bool
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
	anchor      woxui.Rect
	anchored    bool
}

type modelManagerOptionAction struct {
	label     string
	enabled   bool
	operation string
}

// buildModelManagerOverlay converts controller state into the pure modal view.
func (a *App) buildModelManagerOverlay(snapshot *modelManagerSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	title := "Dictation models"
	downloadLabel := a.translate("i18n:plugin_dictation_model_download")
	retryLabel := a.translate("i18n:plugin_dictation_model_retry")
	deleteLabel := a.translate("i18n:plugin_dictation_model_delete")
	extractingLabel := a.translate("i18n:plugin_dictation_model_extracting")
	finalizingLabel := a.translate("i18n:plugin_dictation_model_finalizing")
	recommendedLabel := a.translate("i18n:plugin_dictation_model_recommended")
	if snapshot.kind == "ocrModel" {
		title = "OCR models"
		downloadLabel = a.translate("i18n:plugin_ocr_model_download")
		retryLabel = a.translate("i18n:plugin_ocr_model_retry")
		deleteLabel = "Delete"
	}
	engineLabel := "Checking inference engine…"
	engineButtonLabel := "Download engine"
	engineEnabled := false
	if snapshot.engine.Known {
		if snapshot.engine.Ready {
			engineLabel = "Inference engine ready"
		} else {
			switch snapshot.engine.State {
			case "downloading", "extracting", "finalizing":
				engineLabel = fmt.Sprintf("Engine %s · %d%%", snapshot.engine.State, snapshot.engine.Progress)
			case "failed":
				engineLabel = "Engine failed"
				engineButtonLabel = "Retry engine"
				engineEnabled = snapshot.busy == "" && !snapshot.loading
			default:
				engineLabel = "Inference engine is not installed"
				engineEnabled = snapshot.busy == "" && !snapshot.loading
			}
		}
	}
	if snapshot.engine.Error != "" {
		engineLabel += " · " + snapshot.engine.Error
	}
	options := make([]launcherview.ModelManagerOption, 0, len(snapshot.options))
	for index, option := range snapshot.options {
		index := index
		option := option
		selected := modelOptionID(option) == snapshot.selected
		usable := modelOptionUsable(snapshot.kind, option)
		actionState := resolveModelManagerOptionAction(snapshot.kind, option, selected, snapshot.busy != "" || snapshot.loading, downloadLabel, retryLabel, extractingLabel, finalizingLabel)
		action := func() { a.runModelManagerAction(actionState.operation, index) }
		if actionState.operation == "select" {
			action = func() { a.chooseManagedModel(index) }
		}
		detail := strings.TrimSpace(option.Description)
		if option.Languages != "" {
			if detail != "" {
				detail += " · "
			}
			detail += option.Languages
		}
		if detail == "" {
			detail = modelStatusLabel(option)
		}
		name := modelOptionLabel(option)
		converted := launcherview.ModelManagerOption{
			Name: name, Detail: detail, Status: modelStatusLabel(option), State: option.Status, Progress: option.DownloadProgress, Languages: option.Languages, Description: option.Description, SizeMB: option.SizeMB, Recommended: option.Recommended, SelectedRow: index == snapshot.selectedRow,
			PrimaryAction: usable, ActionLabel: actionState.label, ActionEnabled: actionState.enabled, OnAction: action,
			OnSelect: func() { a.selectModelManagerRow(index) },
		}
		if usable {
			converted.OnChoose = func() { a.chooseManagedModel(index) }
		}
		if snapshot.kind == "dictationModel" && option.Status == "downloaded" {
			converted.OnDelete = func() { a.runModelManagerAction("delete", index) }
		}
		options = append(options, converted)
	}
	iconTint := palette.resultSubtitle
	errorTint := palette.componentTheme().ErrorText
	return launcherview.ModelManagerView(launcherview.ModelManagerProps{
		Width: width, Height: height, Theme: palette.componentTheme(), Title: title,
		Anchor: snapshot.anchor, Anchored: snapshot.anchored,
		Loading: snapshot.loading, Busy: snapshot.busy != "", Error: snapshot.error,
		EngineLabel: engineLabel, EngineButtonLabel: engineButtonLabel, EngineEnabled: engineEnabled, EngineKnown: snapshot.engine.Known, EngineReady: snapshot.engine.Ready,
		RecommendedLabel: recommendedLabel, DeleteLabel: deleteLabel,
		DownloadIcon: a.imageForTint(settingControlIconSource("download"), &iconTint, 16), DeleteIcon: a.imageForTint(settingControlIconSource("delete"), &iconTint, 16), ErrorIcon: a.imageForTint(settingControlIconSource("error"), &errorTint, 16), Options: options,
		OnEngine: func() { a.runModelManagerAction("engine", -1) },
		OnRefresh: func() {
			state := a.aiSettings.ModelManager()
			if state != nil {
				util.Go(a.lifecycleCtx, "refresh model manager", func() {
					a.refreshModelManager(state)
				})
			}
		},
		OnClose: a.closeModelManager,
	})
}

// resolveModelManagerOptionAction keeps persisted selection separate from actual on-disk availability.
func resolveModelManagerOptionAction(kind string, option formOption, selected, busy bool, downloadLabel, retryLabel, extractingLabel, finalizingLabel string) modelManagerOptionAction {
	if modelOptionUsable(kind, option) {
		return modelManagerOptionAction{label: "Select", enabled: !busy && !selected, operation: "select"}
	}
	if kind == "ocrModel" && option.Status == "downloaded" && !option.Available {
		return modelManagerOptionAction{label: "Unavailable"}
	}
	if option.Status == "downloading" {
		return modelManagerOptionAction{label: fmt.Sprintf("%d%%", option.DownloadProgress)}
	}
	if option.Status == "extracting" {
		return modelManagerOptionAction{label: extractingLabel}
	}
	if option.Status == "finalizing" {
		return modelManagerOptionAction{label: finalizingLabel}
	}
	if option.Status == "failed" {
		return modelManagerOptionAction{label: retryLabel, enabled: !busy, operation: "download"}
	}
	return modelManagerOptionAction{label: downloadLabel, enabled: !busy, operation: "download"}
}

func snapshotModelManagerLocked(state *modelManagerState) *modelManagerSnapshot {
	if state == nil {
		return nil
	}
	return &modelManagerSnapshot{
		kind: state.kind, options: append([]formOption(nil), state.options...), selected: state.selected, selectedRow: state.selectedRow,
		engine: state.engine, loading: state.loading, busy: state.busy, error: state.error, anchor: state.anchor, anchored: state.anchored,
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
func (a *App) openPluginModelManager(index int, anchor woxui.Rect) {
	a.stopHotkeyRecording()
	state := a.pluginSettings.Form()
	if state == nil || index < 0 || index >= len(state.definitions) {
		return
	}
	definition := state.definitions[index]
	if definition.Type != "dictationModel" && definition.Type != "ocrModel" {
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
		selected: selected, selectedRow: selectedRow, anchor: anchor, anchored: definition.Type == "dictationModel",
	}
	a.aiSettings.SetModelManager(manager)
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
	util.Go(a.lifecycleCtx, "open model manager", func() {
		a.refreshModelManager(manager)
	})
}

func (a *App) modelManagerCurrentLocked(state *modelManagerState) bool {
	pluginForm := a.pluginSettings.Form()
	return state != nil && a.aiSettings.ModelManager() == state && a.settingTab == "plugins" && pluginForm != nil && state.target == &pluginForm.formFieldsState
}

// refreshModelManager merges runtime-only progress into translated definition metadata.
func (a *App) refreshModelManager(state *modelManagerState) {
	shouldLoad := false
	kind := ""
	if err := a.runOnUI("start refreshing model manager", func() {
		if !a.modelManagerCurrentLocked(state) || state.loading {
			return
		}
		state.loading = true
		state.error = ""
		kind = state.kind
		shouldLoad = true
		a.invalidateSettingsWindow()
	}); err != nil || !shouldLoad {
		return
	}

	modelKind := contract.ManagedModelDictation
	if kind == "ocrModel" {
		modelKind = contract.ManagedModelOCR
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	loadedStatuses, statusErr := a.services.ManagedModelStatuses(ctx, a.sessionID, modelKind)
	statuses := make([]formOption, len(loadedStatuses))
	for index, status := range loadedStatuses {
		statuses[index] = formOption{
			ID: status.ID, DisplayName: status.DisplayName, Description: status.Description, Languages: status.Languages,
			Recommended: status.Recommended, Status: status.Status, DownloadProgress: status.DownloadProgress, SizeMB: status.SizeMB, Error: status.Error,
		}
	}
	loadedEngine, engineErr := a.services.ManagedModelEngineStatus(ctx, a.sessionID, modelKind)
	engine := modelEngineStatus{
		State: loadedEngine.State, Progress: loadedEngine.Progress, Error: loadedEngine.Error, Ready: loadedEngine.Ready,
	}
	cancel()
	engine.Known = engineErr == nil

	_ = a.runOnUI("apply model manager refresh", func() {
		if !a.modelManagerCurrentLocked(state) {
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
		a.invalidateSettingsWindow()
		if poll {
			time.AfterFunc(time.Second, func() {
				util.Go(a.lifecycleCtx, "poll model manager", func() {
					a.refreshModelManager(state)
				})
			})
		}
	})
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
	state := a.aiSettings.ModelManager()
	if state == nil {
		return
	}
	pluginForm := a.pluginSettings.Form()
	if pluginForm != nil && state.target == &pluginForm.formFieldsState {
		pluginForm.active = true
		setFormFieldsFocusLocked(&pluginForm.formFieldsState, state.fieldIndex)
	}
	a.aiSettings.SetModelManager(nil)
	a.invalidateSettingsWindow()
}

func (a *App) selectModelManagerRow(index int) {
	state := a.aiSettings.ModelManager()
	if state == nil || index < 0 || index >= len(state.options) {
		return
	}
	state.selectedRow = index
	a.invalidateSettingsWindow()
}

func (a *App) chooseManagedModel(index int) {
	state := a.aiSettings.ModelManager()
	if state == nil || state.busy != "" || index < 0 || index >= len(state.options) || !modelOptionUsable(state.kind, state.options[index]) {
		return
	}
	option := state.options[index]
	key := state.target.definitions[state.fieldIndex].Value.Key
	state.target.values[key] = modelOptionID(option)
	state.selected = modelOptionID(option)
	pluginForm := a.pluginSettings.Form()
	pluginTarget := pluginForm != nil && state.target == &pluginForm.formFieldsState
	a.closeModelManager()
	if pluginTarget {
		a.submitPluginSettings()
	}
}

// runModelManagerAction starts core-owned downloads or deletion and leaves progress polling in the shared overlay.
func (a *App) runModelManagerAction(action string, index int) {
	state := a.aiSettings.ModelManager()
	if state == nil || state.busy != "" {
		return
	}
	modelID := ""
	if action != "engine" {
		if index < 0 || index >= len(state.options) {
			return
		}
		modelID = modelOptionID(state.options[index])
	}
	state.busy = action + ":" + modelID
	state.error = ""
	kind := state.kind
	a.invalidateSettingsWindow()

	util.Go(a.lifecycleCtx, "run model manager action", func() {
		modelKind := contract.ManagedModelDictation
		if kind == "ocrModel" {
			modelKind = contract.ManagedModelOCR
		}
		operation := contract.ManagedModelOperationDownload
		if action == "delete" {
			operation = contract.ManagedModelOperationDelete
		} else if action == "engine" {
			operation = contract.ManagedModelOperationDownloadEngine
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := a.services.OperateManagedModel(ctx, a.sessionID, modelKind, operation, modelID)
		cancel()
		_ = a.runOnUI("apply model manager action", func() {
			if !a.modelManagerCurrentLocked(state) {
				return
			}
			state.busy = ""
			if err != nil {
				state.error = err.Error()
			} else if action == "download" {
				// Match Flutter's optimistic transition so the first refresh cannot leave an accepted download idle and unpolled.
				state.options[index].Status = "downloading"
				state.options[index].DownloadProgress = 0
				state.options[index].Error = ""
				state.target.definitions[state.fieldIndex].Value.Options = append([]formOption(nil), state.options...)
			} else if action == "delete" && state.selected == modelID {
				state.selected = ""
				key := state.target.definitions[state.fieldIndex].Value.Key
				state.target.values[key] = ""
			}
			a.invalidateSettingsWindow()
			if err == nil {
				delay := time.Duration(0)
				if action == "download" || action == "engine" {
					delay = 500 * time.Millisecond
				}
				time.AfterFunc(delay, func() {
					util.Go(a.lifecycleCtx, "refresh model manager after action", func() {
						a.refreshModelManager(state)
					})
				})
			}
		})
	})
}

func (a *App) onModelManagerKey(event woxui.KeyEvent) bool {
	state := a.aiSettings.ModelManager()
	selected := -1
	count := 0
	if state != nil {
		selected = state.selectedRow
		count = len(state.options)
	}
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
		if a.aiSettings.ModelManager() == state && selected >= 0 && selected < len(state.options) {
			option := state.options[selected]
			usable := modelOptionUsable(state.kind, option)
			status := option.Status
			if usable {
				a.chooseManagedModel(selected)
			} else if status == "not_downloaded" || status == "failed" || status == "" {
				a.runModelManagerAction("download", selected)
			}
		}
	case woxui.KeyDelete:
		canDelete := a.aiSettings.ModelManager() == state && state.kind == "dictationModel" && selected >= 0 && selected < len(state.options) && state.options[selected].Status == "downloaded"
		if canDelete {
			a.runModelManagerAction("delete", selected)
		}
	default:
		if event.Modifiers.HasPrimary() && event.Key == woxui.Key("r") {
			util.Go(a.lifecycleCtx, "refresh model manager from keyboard", func() {
				a.refreshModelManager(state)
			})
			return true
		}
		return true
	}
	return true
}
