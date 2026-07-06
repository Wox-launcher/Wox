package dictation

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"
	"wox/util/keyboard"
	"wox/util/mouse"
	"wox/util/overlay"
	"wox/util/screen"
	"wox/util/speech"
)

const (
	dictationPluginID = "a3f7b8c2-d1e4-4f6a-9b0c-7e2d1a5f8b3e"

	// Setting keys
	settingKeyHotkey      = "hotkey"
	settingKeyInputDevice = "inputDevice"
	settingKeyModel       = "model"
	settingKeyTriggerMode = "triggerMode"

	// Trigger mode values
	triggerModeToggle = "toggle"
	triggerModeHold   = "hold"

	// Overlay
	dictationOverlayName = "dictation-indicator"

	// Overlay position: distance from the bottom of the mouse screen.
	overlayBottomOffset = 80.0
)

var (
	dictationIcon = common.PluginDictationIcon

	// showOverlay and closeOverlay are replaceable for testing.
	showOverlay  = overlay.Show
	closeOverlay = overlay.Close

	// hotkeyRegistrar is set by the UI layer at startup to avoid a circular
	// import (ui imports plugin/system/dictation, so dictation cannot import ui).
	// The UI Manager registers itself as the hotkey registrar via SetHotkeyRegistrar.
	hotkeyRegistrar HotkeyRegistrar
)

// HotkeyRegistrar abstracts the UI Manager's RegisterDictationHotkey method
// so the dictation plugin can register/unregister its global hotkey without
// importing the ui package directly.
type HotkeyRegistrar interface {
	RegisterDictationHotkey(ctx context.Context, combineKey string, triggerMode string) error
}

// SetHotkeyRegistrar is called by the UI Manager during startup to inject
// the hotkey registration implementation. This breaks the import cycle between
// ui and plugin/system/dictation.
func SetHotkeyRegistrar(r HotkeyRegistrar) {
	hotkeyRegistrar = r
}

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &DictationPlugin{})
}

type DictationPlugin struct {
	api          plugin.API
	modelManager *speech.ModelManager

	// Session state
	sessionMu   sync.Mutex
	session     *speech.Session
	isRecording bool

	// Overlay update throttling
	lastOverlayUpdate time.Time

	// registeredHotkey tracks the currently bound hotkey so we can
	// unregister the old one before binding a new one.
	registeredHotkeyMu sync.Mutex
}

func (p *DictationPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            dictationPluginID,
		Name:          "i18n:plugin_dictation_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_dictation_plugin_description",
		Icon:          dictationIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"dictation",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Macos",
			"Windows",
			"Linux",
		},
		SettingDefinitions: []definition.PluginSettingDefinitionItem{
			// Trigger mode - static select. Placed first so the user chooses how
			// the hotkey fires before recording the key itself.
			{
				Type: definition.PluginSettingDefinitionTypeSelect,
				Value: &definition.PluginSettingValueSelect{
					Key:          settingKeyTriggerMode,
					Label:        "i18n:plugin_dictation_trigger_mode",
					Tooltip:      "i18n:plugin_dictation_trigger_mode_tooltip",
					DefaultValue: triggerModeToggle,
					Options: []definition.PluginSettingValueSelectOption{
						{Label: "i18n:plugin_dictation_trigger_toggle", Value: triggerModeToggle},
						{Label: "i18n:plugin_dictation_trigger_hold", Value: triggerModeHold},
					},
				},
			},
			// Hotkey recorder - dynamic setting. The actual definition is built
			// at render time by the OnGetDynamicSetting callback below, using a
			// different tooltip depending on the selected trigger mode.
			{
				Type: definition.PluginSettingDefinitionTypeDynamic,
				Value: &definition.PluginSettingValueDynamic{
					Key: settingKeyHotkey,
				},
			},
			// Model manager - a dedicated component showing recommended models
			// with download status. The options are populated dynamically by the
			// OnGetDynamicSetting callback below; here we provide an empty initial
			// definition so the setting type is known to the UI.
			{
				Type: definition.PluginSettingDefinitionTypeDynamic,
				Value: &definition.PluginSettingValueDynamic{
					Key: settingKeyModel,
				},
			},
			// Input device - dynamic select populated at render time.
			{
				Type: definition.PluginSettingDefinitionTypeDynamic,
				Value: &definition.PluginSettingValueDynamic{
					Key: settingKeyInputDevice,
				},
			},
		},
	}
}

func (p *DictationPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	p.api = initParams.API

	// Initialize the model manager in the Wox data directory.
	modelsDir := filepath.Join(util.GetLocation().GetWoxDataDirectory(), "dictation", "models")
	mgr, err := speech.NewModelManager(modelsDir)
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to create model manager: %s", err.Error()))
	} else {
		p.modelManager = mgr
	}

	// Register dynamic setting callbacks for hotkey, input device and model.
	p.api.OnGetDynamicSetting(ctx, func(ctx context.Context, key string) definition.PluginSettingDefinitionItem {
		switch key {
		case settingKeyHotkey:
			return p.buildHotkeySetting(ctx)
		case settingKeyInputDevice:
			return p.buildInputDeviceSetting(ctx)
		case settingKeyModel:
			return p.buildModelSetting(ctx)
		}
		return definition.PluginSettingDefinitionItem{}
	})

	// React to setting changes: re-register the hotkey when it or the trigger mode changes.
	p.api.OnSettingChanged(ctx, func(ctx context.Context, key string, value string) {
		if key == settingKeyHotkey {
			p.reregisterHotkey(ctx, value)
		}
		if key == settingKeyTriggerMode {
			// Trigger mode changed - re-register with the new mode.
			hotkey := p.api.GetSetting(ctx, settingKeyHotkey)
			if hotkey != "" {
				p.reregisterHotkey(ctx, hotkey)
			}
		}
	})

	// Register the initial hotkey if one was previously saved.
	initialHotkey := p.api.GetSetting(ctx, settingKeyHotkey)
	if initialHotkey != "" {
		p.reregisterHotkey(ctx, initialHotkey)
	}
}

// reregisterHotkey binds the dictation global hotkey via the injected
// HotkeyRegistrar (the UI Manager). Called on init and whenever the hotkey
// setting or trigger mode changes.
func (p *DictationPlugin) reregisterHotkey(ctx context.Context, combineKey string) {
	p.registeredHotkeyMu.Lock()
	defer p.registeredHotkeyMu.Unlock()
	if hotkeyRegistrar == nil {
		p.api.Log(ctx, plugin.LogLevelDebug, "hotkey registrar not set, skipping hotkey registration")
		return
	}
	triggerMode := p.api.GetSetting(ctx, settingKeyTriggerMode)
	if triggerMode == "" {
		triggerMode = triggerModeToggle
	}
	if err := hotkeyRegistrar.RegisterDictationHotkey(ctx, combineKey, triggerMode); err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to register dictation hotkey: %s", err.Error()))
	}
}

// buildHotkeySetting returns the hotkey setting as a dictationHotkey
// definition, with a tooltip that reflects the current trigger mode.
func (p *DictationPlugin) buildHotkeySetting(ctx context.Context) definition.PluginSettingDefinitionItem {
	triggerMode := p.api.GetSetting(ctx, settingKeyTriggerMode)
	if triggerMode == "" {
		triggerMode = triggerModeToggle
	}

	tooltip := "i18n:plugin_dictation_hotkey_tooltip"
	if triggerMode == triggerModeHold {
		tooltip = "i18n:plugin_dictation_hotkey_hold_tooltip"
	}

	return definition.PluginSettingDefinitionItem{
		Type: definition.PluginSettingDefinitionTypeDictationHotkey,
		Value: &definition.PluginSettingValueDictationHotkey{
			Key:          settingKeyHotkey,
			Label:        "i18n:plugin_dictation_hotkey",
			Tooltip:      tooltip,
			DefaultValue: "",
			TriggerMode:  triggerMode,
		},
	}
}

// buildInputDeviceSetting enumerates capture devices and returns a select
// definition with "system default" plus each available device.
func (p *DictationPlugin) buildInputDeviceSetting(ctx context.Context) definition.PluginSettingDefinitionItem {
	options := []definition.PluginSettingValueSelectOption{
		{Label: "i18n:plugin_dictation_system_default", Value: "system"},
	}

	devices, err := speech.ListCaptureDevices(ctx)
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to list capture devices: %s", err.Error()))
	} else {
		for _, d := range devices {
			options = append(options, definition.PluginSettingValueSelectOption{
				Label: d.Name,
				Value: d.ID,
			})
		}
	}

	return definition.PluginSettingDefinitionItem{
		Type: definition.PluginSettingDefinitionTypeSelect,
		Value: &definition.PluginSettingValueSelect{
			Key:          settingKeyInputDevice,
			Label:        "i18n:plugin_dictation_input_device",
			Tooltip:      "i18n:plugin_dictation_input_device_tooltip",
			DefaultValue: "system",
			Options:      options,
		},
	}
}

// buildModelSetting returns the model manager setting as a dictationModel
// definition, populated with recommended models and their current download
// status. The Flutter side renders this as a dropdown where not-downloaded
// models show a download button and downloading models show a progress bar.
func (p *DictationPlugin) buildModelSetting(ctx context.Context) definition.PluginSettingDefinitionItem {
	options := p.buildModelOptions(ctx)

	// Determine the default value: the first downloaded model, or empty.
	defaultValue := ""
	for _, opt := range options {
		if opt.Status == definition.DictationModelStatusDownloaded {
			defaultValue = opt.ID
			break
		}
	}

	return definition.PluginSettingDefinitionItem{
		Type: definition.PluginSettingDefinitionTypeDictationModel,
		Value: &definition.PluginSettingValueDictationModel{
			Key:          settingKeyModel,
			Label:        "i18n:plugin_dictation_model",
			Tooltip:      "i18n:plugin_dictation_model_tooltip",
			DefaultValue: defaultValue,
			Options:      options,
		},
	}
}

// buildModelOptions builds the list of model options for the dictationModel
// setting, combining recommended models with their current download status.
func (p *DictationPlugin) buildModelOptions(ctx context.Context) []definition.DictationModelOption {
	// Start with recommended models.
	options := make([]definition.DictationModelOption, 0, len(speech.RecommendedModels))

	// Get the set of locally downloaded model IDs.
	localModels := make(map[string]bool)
	if p.modelManager != nil {
		if models, err := p.modelManager.ListLocalModels(); err == nil {
			for _, m := range models {
				localModels[m.ID] = true
			}
		}
	}

	for _, rec := range speech.RecommendedModels {
		status := definition.DictationModelStatusNotDownloaded
		progress := 0
		errMsg := ""

		if localModels[rec.ID] {
			status = definition.DictationModelStatusDownloaded
			progress = 100
		} else if p.modelManager != nil {
			if ds := p.modelManager.GetDownloadStatus(rec.ID); ds != nil {
				switch ds.State {
				case speech.DownloadStateDownloading:
					status = definition.DictationModelStatusDownloading
					progress = ds.Progress
				case speech.DownloadStateExtracting:
					status = definition.DictationModelStatusExtracting
					progress = 100
				case speech.DownloadStateFailed:
					status = definition.DictationModelStatusFailed
					errMsg = ds.Error
				}
			}
		}

		options = append(options, definition.DictationModelOption{
			ID:               rec.ID,
			DisplayName:      rec.DisplayName,
			Status:           status,
			DownloadProgress: progress,
			SizeMB:           rec.SizeMB,
			Error:            errMsg,
		})
	}

	return options
}

// StartModelDownload triggers a model download asynchronously. It is called
// by the HTTP handler when the user clicks download in the settings UI.
func (p *DictationPlugin) StartModelDownload(ctx context.Context, modelID string) error {
	if p.modelManager == nil {
		return fmt.Errorf("model manager not initialized")
	}

	// Find the model in the recommended list.
	var info *speech.ModelInfo
	for i := range speech.RecommendedModels {
		if speech.RecommendedModels[i].ID == modelID {
			info = &speech.RecommendedModels[i]
			break
		}
	}
	if info == nil {
		return fmt.Errorf("model not found in recommended list: %s", modelID)
	}

	if p.modelManager.IsDownloading(modelID) {
		return fmt.Errorf("model %s is already downloading", modelID)
	}

	// Check if already downloaded.
	targetDir := filepath.Join(p.modelManager.ModelsDir(), modelID)
	if _, ok := p.modelManager.InspectModelDir(targetDir); ok {
		return nil
	}

	util.Go(ctx, "download dictation model", func() {
		err := p.modelManager.DownloadModel(ctx, *info, nil)
		if err != nil {
			p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to download model %s: %s", modelID, err.Error()))
		}
	})

	return nil
}

// DeleteModel removes a downloaded model from disk. If the model is the
// currently selected one, the caller is responsible for clearing the setting.
func (p *DictationPlugin) DeleteModel(ctx context.Context, modelID string) error {
	if p.modelManager == nil {
		return fmt.Errorf("model manager not initialized")
	}
	if p.modelManager.IsDownloading(modelID) {
		return fmt.Errorf("cannot delete model %s while it is downloading", modelID)
	}
	return p.modelManager.DeleteModel(modelID)
}

// ModelStatusInfo is the JSON-serializable model status sent to the Flutter side.
// The JSON tags use the same PascalCase keys as DictationModelOption.fromJson
// expects (ID, DisplayName, Status, DownloadProgress, SizeMB, Error) so the
// Flutter entity can parse the response without a separate mapping.
type ModelStatusInfo struct {
	ID               string `json:"ID"`
	DisplayName      string `json:"DisplayName"`
	Status           string `json:"Status"`
	DownloadProgress int    `json:"DownloadProgress"`
	SizeMB           int    `json:"SizeMB"`
	Error            string `json:"Error"`
}

// GetModelStatuses returns the current status of all known models, combining
// recommended models with local models. Called by the HTTP status endpoint.
func (p *DictationPlugin) GetModelStatuses(ctx context.Context) []ModelStatusInfo {
	options := p.buildModelOptions(ctx)
	result := make([]ModelStatusInfo, 0, len(options))
	for _, opt := range options {
		result = append(result, ModelStatusInfo{
			ID:               opt.ID,
			DisplayName:      opt.DisplayName,
			Status:           string(opt.Status),
			DownloadProgress: opt.DownloadProgress,
			SizeMB:           opt.SizeMB,
			Error:            opt.Error,
		})
	}
	return result
}

func (p *DictationPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	return plugin.QueryResponse{}
}

// ToggleDictation is called by the hotkey handler in toggle mode. It starts
// recording if idle and stops if recording. In hold mode, StartDictation and
// StopDictation are called instead.
func (p *DictationPlugin) ToggleDictation(ctx context.Context) {
	p.sessionMu.Lock()
	if p.isRecording {
		p.sessionMu.Unlock()
		p.stopAndOutput(ctx)
		return
	}
	p.sessionMu.Unlock()
	p.startRecording(ctx)
}

// StartDictation is called by the hotkey press handler in hold mode.
func (p *DictationPlugin) StartDictation(ctx context.Context) {
	p.sessionMu.Lock()
	if p.isRecording {
		p.sessionMu.Unlock()
		return
	}
	p.sessionMu.Unlock()
	p.startRecording(ctx)
}

// StopDictation is called by the hotkey release handler in hold mode.
func (p *DictationPlugin) StopDictation(ctx context.Context) {
	p.sessionMu.Lock()
	if !p.isRecording {
		p.sessionMu.Unlock()
		return
	}
	p.sessionMu.Unlock()
	p.stopAndOutput(ctx)
}

// startRecording initializes the recognizer and audio capture, then shows the overlay.
func (p *DictationPlugin) startRecording(ctx context.Context) {
	// Read settings
	deviceID := p.api.GetSetting(ctx, settingKeyInputDevice)
	if deviceID == "" {
		deviceID = "system"
	}
	modelID := p.api.GetSetting(ctx, settingKeyModel)

	if modelID == "" {
		p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_no_model_selected"))
		return
	}

	if p.modelManager == nil {
		p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_model_error"))
		return
	}

	// Find the model on disk.
	models, err := p.modelManager.ListLocalModels()
	if err != nil {
		p.api.Notify(ctx, fmt.Sprintf("%s: %s", i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_model_error"), err.Error()))
		return
	}

	var selectedModel *speech.LocalModel
	for i := range models {
		if models[i].ID == modelID {
			selectedModel = &models[i]
			break
		}
	}
	if selectedModel == nil {
		p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_model_not_found"))
		return
	}

	// Create the session.
	config := speech.RecognizerConfig{
		ModelPath:  selectedModel.Path,
		ModelType:  selectedModel.ModelType,
		NumThreads: 1,
	}

	session := speech.NewSession(ctx, config, deviceID,
		func(text string) {
			p.updateOverlay(ctx, text)
		},
		func(text string) {
			p.updateOverlay(ctx, text)
		},
	)

	if err := session.Start(); err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("dictation start failed: %s", err.Error()))
		p.api.Notify(ctx, fmt.Sprintf("%s: %s", i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_start_failed"), err.Error()))
		return
	}

	p.sessionMu.Lock()
	p.session = session
	p.isRecording = true
	p.sessionMu.Unlock()

	p.showDictationOverlay(ctx)
}

// stopAndOutput stops the recording, closes the overlay, and types the
// recognized text into the focused window.
func (p *DictationPlugin) stopAndOutput(ctx context.Context) {
	p.sessionMu.Lock()
	session := p.session
	p.session = nil
	p.isRecording = false
	p.sessionMu.Unlock()

	if session == nil {
		return
	}

	text, err := session.Stop()
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to stop dictation session: %s", err.Error()))
	}

	p.closeDictationOverlay()

	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	// Wait briefly for the overlay to close and focus to return to the
	// previously focused window.
	time.Sleep(100 * time.Millisecond)
	if err := keyboard.SimulateType(text); err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to type dictation text: %s", err.Error()))
		p.api.Notify(ctx, fmt.Sprintf("%s: %s", i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_type_failed"), err.Error()))
	}
}

// showDictationOverlay displays the recording overlay at the bottom-center
// of the screen the mouse is currently on.
func (p *DictationPlugin) showDictationOverlay(ctx context.Context) {
	mouseScreen := screen.GetMouseScreen()

	opts := overlay.OverlayOptions{
		Name:             dictationOverlayName,
		Message:          i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_listening"),
		Loading:          true,
		Topmost:          true,
		AbsolutePosition: true,
		Anchor:           overlay.AnchorBottomCenter,
		OffsetX:          float64(mouseScreen.X) + float64(mouseScreen.Width)/2,
		OffsetY:          float64(mouseScreen.Y+mouseScreen.Height) - overlayBottomOffset,
		AutoCloseSeconds: 0,
		Closable:         false,
		CloseOnEscape:    false,
		Movable:          false,
		FontSize:         14,
		MinWidth:         200,
		MaxWidth:         600,
	}

	if mouseScreen.Width == 0 {
		pos, ok := mouse.CurrentPosition()
		if ok {
			opts.OffsetX = pos.X
			opts.OffsetY = pos.Y - overlayBottomOffset
		}
	}

	showOverlay(opts)
}

// updateOverlay refreshes the overlay text with the latest partial result.
// Throttled to ~80ms intervals to avoid excessive redraw.
func (p *DictationPlugin) updateOverlay(ctx context.Context, text string) {
	now := time.Now()
	if now.Sub(p.lastOverlayUpdate) < 80*time.Millisecond {
		return
	}
	p.lastOverlayUpdate = now

	mouseScreen := screen.GetMouseScreen()

	opts := overlay.OverlayOptions{
		Name:             dictationOverlayName,
		Message:          text,
		Loading:          true,
		Topmost:          true,
		PreservePosition: true,
		FontSize:         14,
		MinWidth:         200,
		MaxWidth:         600,
		AbsolutePosition: true,
		Anchor:           overlay.AnchorBottomCenter,
		OffsetX:          float64(mouseScreen.X) + float64(mouseScreen.Width)/2,
		OffsetY:          float64(mouseScreen.Y+mouseScreen.Height) - overlayBottomOffset,
	}

	showOverlay(opts)
}

// closeDictationOverlay removes the recording overlay.
func (p *DictationPlugin) closeDictationOverlay() {
	closeOverlay(dictationOverlayName)
}
