package dictation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/resource"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
	"wox/util/audio"
	"wox/util/keyboard"
	"wox/util/mouse"
	"wox/util/overlay"
	"wox/util/overlay/dictationoverlay"
	"wox/util/overlay/textoverlay"
	"wox/util/screen"
	"wox/util/speech"
)

const (
	// Setting keys
	settingKeyHotkey          = "hotkey"
	settingKeyInputDevice     = "inputDevice"
	settingKeyInputDeviceName = "inputDeviceName"
	settingKeyModel           = "model"
	settingKeyTriggerMode     = "triggerMode"
	settingKeyPlaySound       = "playSound"
	settingKeyDuckVolume      = "duckVolume"
	settingKeyAIRefine        = "aiRefineEnabled"
	settingKeyAIModel         = "aiModel"

	inputDeviceSystem = "system"

	// AI refinement timeout. Picked to cover a normal model response for a
	// short dictation transcript while keeping the wait perceptible.
	aiRefineTimeout = 5 * time.Second

	// recognizerPoolIdleTTL controls how long an unused speech model stays in
	// memory before being evicted. 10 minutes covers typical back-to-back
	// dictation bursts while reclaiming ~70-150MB during longer pauses.
	recognizerPoolIdleTTL = 10 * time.Minute

	// Embedded audio clips played when the dictation overlay shows/hides.
	soundStart = "dictation_start.wav"
	soundStop  = "dictation_stop.wav"

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

	errInputDeviceMissing = errors.New("input device missing")

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

	// recognizerPool keeps the speech model in memory across sessions so
	// subsequent dictations start without the model-loading delay.
	recognizerPool *speech.RecognizerPool

	// audioCapturePool keeps the malgo context + capture device alive across
	// sessions, eliminating the InitDevice delay.
	audioCapturePool *speech.AudioCapturePool

	// vadPool keeps the silero VAD model in memory across sessions.
	vadPool *speech.VadPool

	// vadModelPath is the extracted path to silero_vad.onnx.
	vadModelPath string

	// Session state
	sessionMu   sync.Mutex
	session     *speech.Session
	isRecording bool
	// isStarting tracks that startRecording is in progress (model loading,
	// audio init). When the user releases the hotkey during this window,
	// StopDictation sets pendingStop so startRecording can stop immediately
	// after it finishes.
	isStarting  bool
	pendingStop bool

	// Overlay update throttling
	lastOverlayUpdate time.Time

	// Voice activity state drives the recording waveform overlay without
	// forcing audio callbacks to refresh native UI on every sample buffer.
	voiceOverlayMu       sync.Mutex
	voiceOverlayActive   bool
	voiceOverlayStateSet bool
	voiceOverlayVisible  bool

	// registeredHotkey tracks the currently bound hotkey so we can
	// unregister the old one before binding a new one.
	registeredHotkeyMu sync.Mutex

	// history persists past dictation transcripts so the Query surface can
	// list them by time. Stored as a plugin setting so cloud sync covers it.
	history *historyStore

	// dictionary keeps user-approved correction rules for future dictations.
	dictionary *dictionaryStore
}

func (p *DictationPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "a3f7b8c2-d1e4-4f6a-9b0c-7e2d1a5f8b3e",
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
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
			},
			{
				Name: plugin.MetadataFeatureQueryEnv,
				Params: map[string]any{
					"requireActiveWindowName": true,
					"requireActiveWindowPid":  true,
					"requireActiveWindowIcon": true,
				},
			},
			// Required so the plugin can call AIChatStream for AI refinement.
			{
				Name: plugin.MetadataFeatureAI,
			},
		},
		SettingDefinitions: []definition.PluginSettingDefinitionItem{
			{
				Type: definition.PluginSettingDefinitionTypeSelect,
				Value: &definition.PluginSettingValueSelect{
					Key:          settingKeyTriggerMode,
					Label:        "i18n:plugin_dictation_trigger_mode",
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
			// Play a short beep when dictation starts and stops.
			{
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          settingKeyPlaySound,
					Label:        "i18n:plugin_dictation_play_sound",
					Tooltip:      "i18n:plugin_dictation_play_sound_tooltip",
					DefaultValue: "true",
				},
			},
			// Lower other audio during dictation.
			{
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          settingKeyDuckVolume,
					Label:        "i18n:plugin_dictation_duck_volume",
					Tooltip:      "i18n:plugin_dictation_duck_volume_tooltip",
					DefaultValue: "false",
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key:          settingKeyDictionary,
					DefaultValue: "[]",
					Title:        "i18n:plugin_dictation_dictionary",
					Tooltip:      "i18n:plugin_dictation_dictionary_tooltip",
					MaxHeight:    260,
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:          "context",
							Label:        "i18n:plugin_dictation_dictionary_context",
							Type:         definition.PluginSettingValueTableColumnTypeText,
							Width:        260,
							TextMaxLines: 3,
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:          "wrongPhrase",
							Label:        "i18n:plugin_dictation_dictionary_wrong_phrase",
							Type:         definition.PluginSettingValueTableColumnTypeText,
							Width:        180,
							TextMaxLines: 2,
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:          "correctPhrase",
							Label:        "i18n:plugin_dictation_dictionary_correct_phrase",
							Type:         definition.PluginSettingValueTableColumnTypeText,
							Width:        180,
							TextMaxLines: 2,
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
					},
				},
			},
			// AI refinement group header.
			{
				Type: definition.PluginSettingDefinitionTypeHead,
				Value: &definition.PluginSettingValueHead{
					Content: "i18n:plugin_dictation_ai_group",
				},
			},
			// Master switch for AI refinement after dictation stops.
			{
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          settingKeyAIRefine,
					Label:        "i18n:plugin_dictation_ai_enable",
					Tooltip:      "i18n:plugin_dictation_ai_enable_tooltip",
					DefaultValue: "false",
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeSelectAIModel,
				Value: &definition.PluginSettingValueSelectAIModel{
					Key:   settingKeyAIModel,
					Label: "i18n:plugin_dictation_ai_model",
				},
			},
		},
	}
}

func (p *DictationPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	p.api = initParams.API
	p.history = newHistoryStore(p.api)
	p.history.load(ctx)
	p.dictionary = newDictionaryStore(p.api)
	p.dictionary.load(ctx)

	// Initialize the model manager in the Wox data directory.
	modelsDir := filepath.Join(util.GetLocation().GetDictationDirectory(), "models")
	mgr, err := speech.NewModelManager(modelsDir)
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to create model manager: %s", err.Error()))
	} else {
		p.modelManager = mgr
	}

	// Start the recognizer pool so the speech model stays in memory across
	// sessions. The idle reaper evicts the model after recognizerPoolIdleTTL
	// of inactivity to reclaim memory.
	p.recognizerPool = speech.NewRecognizerPool(recognizerPoolIdleTTL)
	p.recognizerPool.StartReaper(ctx)

	// Start the audio capture pool so the malgo context + device stay alive
	// across sessions, eliminating the InitDevice delay.
	p.audioCapturePool = speech.NewAudioCapturePool(recognizerPoolIdleTTL)
	p.audioCapturePool.StartReaper(ctx)

	// Extract the embedded silero VAD model to a temp file and start the VAD pool.
	p.vadModelPath = extractVadModel(ctx)
	p.vadPool = speech.NewVadPool(recognizerPoolIdleTTL)
	p.vadPool.StartReaper(ctx)

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

	// React to setting changes: re-register the hotkey and remember input device names.
	p.api.OnSettingChanged(ctx, func(ctx context.Context, key string, value string) {
		switch key {
		case settingKeyHotkey:
			p.reregisterHotkey(ctx, value)
		case settingKeyTriggerMode:
			// Trigger mode changed - re-register with the new mode.
			hotkey := p.api.GetSetting(ctx, settingKeyHotkey)
			if hotkey != "" {
				p.reregisterHotkey(ctx, hotkey)
			}
		case settingKeyInputDevice:
			p.rememberInputDeviceName(ctx, value)
		case settingKeyDictionary:
			if p.dictionary != nil {
				p.dictionary.load(ctx)
			}
		case settingKeyModel:
			// Model changed - evict the old model from the recognizer pool so
			// its memory is freed immediately instead of waiting for the idle
			// timeout.
			p.evictOldModels(ctx, value)
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
	devices, err := speech.ListCaptureDevices(ctx)
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to list capture devices: %s", err.Error()))
	}

	return definition.PluginSettingDefinitionItem{
		Type: definition.PluginSettingDefinitionTypeSelect,
		Value: &definition.PluginSettingValueSelect{
			Key:          settingKeyInputDevice,
			Label:        "i18n:plugin_dictation_input_device",
			Tooltip:      "i18n:plugin_dictation_input_device_tooltip",
			DefaultValue: inputDeviceSystem,
			Options:      buildInputDeviceOptions(ctx, p.api.GetSetting(ctx, settingKeyInputDevice), p.api.GetSetting(ctx, settingKeyInputDeviceName), devices),
		},
	}
}

// buildInputDeviceOptions keeps a missing selected device visible instead of
// letting the UI display the first option as a fallback.
func buildInputDeviceOptions(ctx context.Context, rawSelectedDeviceID string, savedDeviceName string, devices []speech.AudioDevice) []definition.PluginSettingValueSelectOption {
	selectedDeviceID := normalizeInputDeviceID(rawSelectedDeviceID)
	options := []definition.PluginSettingValueSelectOption{
		{Label: "i18n:plugin_dictation_system_default", Value: inputDeviceSystem},
	}

	selectedFound := selectedDeviceID == inputDeviceSystem
	for _, d := range devices {
		options = append(options, definition.PluginSettingValueSelectOption{
			Label: d.Name,
			Value: d.ID,
		})
		if d.ID == selectedDeviceID {
			selectedFound = true
		}
	}

	if !selectedFound {
		unavailable := buildUnavailableInputDeviceOption(ctx, selectedDeviceID, savedDeviceName)
		options = append(options[:1], append([]definition.PluginSettingValueSelectOption{unavailable}, options[1:]...)...)
	}

	return options
}

// buildUnavailableInputDeviceOption represents a saved device that is no
// longer present in the current capture device list.
func buildUnavailableInputDeviceOption(ctx context.Context, deviceID string, savedDeviceName string) definition.PluginSettingValueSelectOption {
	deviceName := inputDeviceDisplayName(deviceID, savedDeviceName)
	return definition.PluginSettingValueSelectOption{
		Label: translateDictationTemplate(ctx, "plugin_dictation_input_device_unavailable", map[string]string{
			"device": deviceName,
		}),
		Value: deviceID,
	}
}

func normalizeInputDeviceID(deviceID string) string {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return inputDeviceSystem
	}
	return deviceID
}

func inputDeviceDisplayName(deviceID string, savedDeviceName string) string {
	if name := strings.TrimSpace(savedDeviceName); name != "" {
		return name
	}
	if normalized := strings.TrimSpace(deviceID); normalized != "" {
		return normalized
	}
	return inputDeviceSystem
}

// resolveInputDeviceForStart verifies concrete devices before creating a
// speech session. System default intentionally skips enumeration.
func resolveInputDeviceForStart(ctx context.Context, rawDeviceID string, savedDeviceName string) (string, string, error) {
	deviceID := normalizeInputDeviceID(rawDeviceID)
	if deviceID == inputDeviceSystem {
		return inputDeviceSystem, "", nil
	}

	devices, err := speech.ListCaptureDevices(ctx)
	if err != nil {
		return deviceID, inputDeviceDisplayName(deviceID, savedDeviceName), fmt.Errorf("failed to list capture devices: %w", err)
	}

	return resolveInputDeviceFromDevices(deviceID, savedDeviceName, devices)
}

// resolveInputDeviceFromDevices applies the concrete-device validation once
// the caller has already decided enumeration is required.
func resolveInputDeviceFromDevices(deviceID string, savedDeviceName string, devices []speech.AudioDevice) (string, string, error) {
	for _, device := range devices {
		if device.ID == deviceID {
			return deviceID, device.Name, nil
		}
	}

	return deviceID, inputDeviceDisplayName(deviceID, savedDeviceName), errInputDeviceMissing
}

// translateDictationTemplate replaces simple named placeholders in localized
// dictation messages.
func translateDictationTemplate(ctx context.Context, key string, replacements map[string]string) string {
	message := i18n.GetI18nManager().TranslateWox(ctx, key)
	for name, value := range replacements {
		message = strings.ReplaceAll(message, "{"+name+"}", value)
	}
	return message
}

// rememberInputDeviceName stores the current human-readable name for the
// selected concrete device so the setting can still explain it after removal.
func (p *DictationPlugin) rememberInputDeviceName(ctx context.Context, rawDeviceID string) {
	deviceID := normalizeInputDeviceID(rawDeviceID)
	if deviceID == inputDeviceSystem {
		p.api.SaveSetting(ctx, settingKeyInputDeviceName, "", false)
		return
	}

	devices, err := speech.ListCaptureDevices(ctx)
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to remember dictation input device name: %s", err.Error()))
		return
	}

	for _, device := range devices {
		if device.ID == deviceID {
			p.api.SaveSetting(ctx, settingKeyInputDeviceName, device.Name, false)
			return
		}
	}
	p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("selected dictation input device not found while saving name: %s", deviceID))
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
				case speech.DownloadStateFinalizing:
					status = definition.DictationModelStatusFinalizing
					progress = 100
				case speech.DownloadStateFailed:
					status = definition.DictationModelStatusFailed
					errMsg = ds.Error
				}
			}
		}

		// Translate i18n keys in Description and Languages.
		desc := rec.Description
		langs := rec.Languages
		if strings.HasPrefix(desc, "i18n:") {
			desc = i18n.GetI18nManager().TranslateWox(ctx, desc)
		}
		if strings.HasPrefix(langs, "i18n:") {
			langs = i18n.GetI18nManager().TranslateWox(ctx, langs)
		}

		options = append(options, definition.DictationModelOption{
			ID:               rec.ID,
			DisplayName:      rec.DisplayName,
			Description:      desc,
			Languages:        langs,
			Recommended:      rec.Recommended,
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
// expects so the Flutter entity can parse status refreshes without dropping
// static model metadata such as description, languages, and recommendation.
type ModelStatusInfo struct {
	ID               string `json:"ID"`
	DisplayName      string `json:"DisplayName"`
	Description      string `json:"Description"`
	Languages        string `json:"Languages"`
	Recommended      bool   `json:"Recommended"`
	Status           string `json:"Status"`
	DownloadProgress int    `json:"DownloadProgress"`
	SizeMB           int    `json:"SizeMB"`
	Error            string `json:"Error"`
}

type CorrectHistoryRequest struct {
	RecordID        string `json:"recordId"`
	PreviousContent string `json:"previousContent"`
	SelectedText    string `json:"selectedText"`
	ReplacementText string `json:"replacementText"`
	UpdatedContent  string `json:"updatedContent"`
}

type CorrectHistoryResponse struct {
	RecordID        string            `json:"recordId"`
	OriginalContent string            `json:"originalContent"`
	Content         string            `json:"content"`
	Timestamp       int64             `json:"timestamp"`
	Title           string            `json:"title"`
	Preview         plugin.WoxPreview `json:"preview"`
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
			Description:      opt.Description,
			Languages:        opt.Languages,
			Recommended:      opt.Recommended,
			Status:           string(opt.Status),
			DownloadProgress: opt.DownloadProgress,
			SizeMB:           opt.SizeMB,
			Error:            opt.Error,
		})
	}
	return result
}

// CorrectHistory applies a user-approved inline correction and records the
// same change as a future dictation dictionary rule.
func (p *DictationPlugin) CorrectHistory(ctx context.Context, req CorrectHistoryRequest) (CorrectHistoryResponse, error) {
	if p.history == nil {
		return CorrectHistoryResponse{}, fmt.Errorf("dictation history is not initialized")
	}
	record, err := p.history.correct(ctx, historyCorrectRequest{
		RecordID:        req.RecordID,
		PreviousContent: req.PreviousContent,
		SelectedText:    req.SelectedText,
		ReplacementText: req.ReplacementText,
		UpdatedContent:  req.UpdatedContent,
	})
	if err != nil {
		return CorrectHistoryResponse{}, err
	}
	if p.dictionary != nil {
		if err := p.dictionary.addOrUpdateCorrection(ctx, extractCorrectionContext(req.PreviousContent, req.SelectedText), req.SelectedText, req.ReplacementText, util.GetSystemTimestamp()); err != nil {
			return CorrectHistoryResponse{}, err
		}
	}
	if p.api != nil {
		p.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
	}

	return CorrectHistoryResponse{
		RecordID:        record.ID,
		OriginalContent: record.OriginalContent,
		Content:         record.Content,
		Timestamp:       record.Timestamp,
		Title:           truncateHistoryTitle(record.Content),
		Preview: plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeDictationHistory,
			PreviewData: record.previewData(ctx, p.api),
		},
	}, nil
}

func (p *DictationPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	// Only the bare trigger keyword surfaces history; sub-commands are not
	// handled here yet.
	if query.Command != "" {
		return plugin.QueryResponse{}
	}

	if p.history.isEmpty() && strings.TrimSpace(query.Search) == "" {
		return plugin.NewQueryResponse([]plugin.QueryResult{historyEmptyResult()})
	}

	results := p.history.buildHistoryResults(ctx, query)
	return plugin.NewQueryResponse(results)
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
// If startRecording is still in progress (model loading), it sets a
// pendingStop flag so startRecording can stop immediately after it
// finishes — preventing the overlay from being stuck open.
func (p *DictationPlugin) StopDictation(ctx context.Context) {
	p.sessionMu.Lock()
	p.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("dictation: StopDictation enter, isStarting=%t isRecording=%t", p.isStarting, p.isRecording))
	if p.isStarting {
		// startRecording hasn't finished yet; mark for deferred stop.
		p.pendingStop = true
		p.sessionMu.Unlock()
		p.api.Log(ctx, plugin.LogLevelDebug, "dictation: StopDictation during startup, pendingStop set")
		return
	}
	if !p.isRecording {
		p.sessionMu.Unlock()
		p.api.Log(ctx, plugin.LogLevelDebug, "dictation: StopDictation not recording, ignoring")
		return
	}
	p.sessionMu.Unlock()
	p.api.Log(ctx, plugin.LogLevelDebug, "dictation: StopDictation calling stopAndOutput")
	p.stopAndOutput(ctx)
}

// startRecording initializes the recognizer and audio capture, then shows the overlay.
func (p *DictationPlugin) startRecording(ctx context.Context) {
	t0 := time.Now()
	p.api.Log(ctx, plugin.LogLevelDebug, "dictation timing: plugin.startRecording enter")

	// Mark that we are in the startup phase so StopDictation can defer
	// its action via pendingStop if the user releases the key before the
	// model finishes loading.
	p.sessionMu.Lock()
	p.isStarting = true
	p.pendingStop = false
	p.sessionMu.Unlock()

	// Read settings
	deviceID := p.api.GetSetting(ctx, settingKeyInputDevice)
	if deviceID == "" {
		deviceID = inputDeviceSystem
	}
	modelID := p.api.GetSetting(ctx, settingKeyModel)

	if modelID == "" {
		p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_no_model_selected"))
		p.sessionMu.Lock()
		p.isStarting = false
		p.sessionMu.Unlock()
		return
	}

	if p.modelManager == nil {
		p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_model_error"))
		p.sessionMu.Lock()
		p.isStarting = false
		p.sessionMu.Unlock()
		return
	}

	// Find the model on disk.
	models, err := p.modelManager.ListLocalModels()
	if err != nil {
		p.api.Notify(ctx, fmt.Sprintf("%s: %s", i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_model_error"), err.Error()))
		p.sessionMu.Lock()
		p.isStarting = false
		p.sessionMu.Unlock()
		return
	}
	p.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("dictation timing: plugin.ListLocalModels cost=%dms", time.Since(t0).Milliseconds()))

	var selectedModel *speech.LocalModel
	for i := range models {
		if models[i].ID == modelID {
			selectedModel = &models[i]
			break
		}
	}
	if selectedModel == nil {
		p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_model_not_found"))
		p.sessionMu.Lock()
		p.isStarting = false
		p.sessionMu.Unlock()
		return
	}

	resolvedDeviceID, resolvedDeviceName, deviceErr := resolveInputDeviceForStart(ctx, deviceID, p.api.GetSetting(ctx, settingKeyInputDeviceName))
	if deviceErr != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("dictation input device validation failed: %s", deviceErr.Error()))
		if errors.Is(deviceErr, errInputDeviceMissing) {
			p.showErrorOverlay(ctx, translateDictationTemplate(ctx, "plugin_dictation_input_device_missing", map[string]string{
				"device": resolvedDeviceName,
			}))
		} else {
			p.showErrorOverlay(ctx, fmt.Sprintf("%s: %s", i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_input_device_list_failed"), deviceErr.Error()))
		}
		p.sessionMu.Lock()
		p.isStarting = false
		p.pendingStop = false
		p.sessionMu.Unlock()
		return
	}
	deviceID = resolvedDeviceID
	if resolvedDeviceName != "" {
		p.api.SaveSetting(ctx, settingKeyInputDeviceName, resolvedDeviceName, false)
	}

	// Show the overlay immediately so the user gets instant feedback while the
	// native dictation engine and recognition model are being prepared.
	loadingKey := "plugin_dictation_loading_model"
	triggerMode := p.api.GetSetting(ctx, settingKeyTriggerMode)
	if triggerMode == triggerModeHold {
		loadingKey = "plugin_dictation_loading_model_hold"
	}

	// Lower other audio as soon as the overlay appears. This pauses/ducks
	// other apps' audio but does not affect Wox's own beep sounds.
	p.startVolumeDucking(ctx)

	p.showLoadingOverlay(ctx, loadingKey)

	// Create the session with VAD + offline recognizer pools.
	config := speech.RecognizerConfig{
		ModelPath:  selectedModel.Path,
		ModelType:  selectedModel.ModelType,
		NumThreads: 1,
	}
	vadConfig := speech.DefaultVadConfig(p.vadModelPath)

	p.resetVoiceOverlayState()
	session := speech.NewSessionWithPools(ctx, config, vadConfig, deviceID, p.recognizerPool, p.audioCapturePool, p.vadPool,
		func(text string) {
			// onPartial: in streaming mode this is called with interim text.
			// We don't update the overlay during recording — the overlay
			// shows a voice activity animation only.
		},
		func(text string) {
			// onFinal: full transcript so far. Not shown during recording.
		},
	)
	session.SetSpeechActivityCallback(func(activity speech.SpeechActivity) {
		p.updateVoiceOverlay(ctx, activity.Speaking)
	})

	if err := session.Start(); err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("dictation start failed: %s", err.Error()))
		p.api.Notify(ctx, fmt.Sprintf("%s: %s", i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_start_failed"), err.Error()))
		p.closeDictationOverlay()
		p.sessionMu.Lock()
		p.isStarting = false
		p.pendingStop = false
		p.sessionMu.Unlock()
		return
	}
	p.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("dictation timing: plugin.sessionStart cost=%dms", time.Since(t0).Milliseconds()))

	// Model loaded and audio capture started. Switch the overlay to
	// "Listening..." and play the start sound to signal the user can speak.
	p.setVoiceOverlayVisible(true)
	p.showDictationOverlay(ctx, p.currentVoiceOverlayActive())

	p.playSoundIfEnabled(ctx, soundStart)

	p.sessionMu.Lock()
	p.session = session
	p.isRecording = true
	p.isStarting = false
	shouldStop := p.pendingStop
	p.pendingStop = false
	p.sessionMu.Unlock()

	p.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("dictation timing: plugin.total cost=%dms", time.Since(t0).Milliseconds()))

	// If the user released the hotkey while we were still loading the model,
	// stop immediately so the overlay doesn't get stuck open.
	if shouldStop {
		p.api.Log(ctx, plugin.LogLevelDebug, "dictation: pendingStop triggered, stopping immediately")
		p.stopAndOutput(ctx)
	}
}

// stopAndOutput stops the recording, closes the overlay, and types the
// recognized text into the focused window. When AI refinement is enabled,
// the overlay stays visible showing a loading state while the transcript is
// rewritten by the selected AI model; on failure or timeout it falls back to
// the raw transcript and notifies the user.
func (p *DictationPlugin) stopAndOutput(ctx context.Context) {
	// Restore system audio volume as early as possible.
	p.stopVolumeDucking(ctx)

	p.sessionMu.Lock()
	session := p.session
	p.session = nil
	p.isRecording = false
	p.sessionMu.Unlock()

	if session == nil {
		return
	}
	p.setVoiceOverlayVisible(false)
	p.showProcessingOverlay(ctx)

	text, err := session.Stop()
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to stop dictation session: %s", err.Error()))
	}

	text = strings.TrimSpace(text)
	if text == "" {
		p.closeDictationOverlay()
		p.playSoundIfEnabled(ctx, soundStop)
		return
	}

	// AI refinement is opt-in. When enabled we keep the overlay open as a
	// loading indicator; on any failure we fall back to the raw transcript.
	aiRefineSucceeded := false
	if p.isAIRefineEnabled(ctx) {
		model, ok := p.getAIModel(ctx)
		if !ok {
			p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_ai_no_model"))
		} else {
			// Gather recent context BEFORE persisting the current utterance so
			// the context only contains prior dictations. The finalized
			// transcripts help the model preserve continuity across consecutive
			// sentences (pronouns, tense, topic).
			recentCtx := p.history.recentContext(util.GetSystemTimestamp())
			p.showRefiningOverlay(ctx)
			var dictionaryEntries []dictionaryEntry
			if p.dictionary != nil {
				dictionaryEntries = p.dictionary.activeEntries()
			}
			refined, refineErr := p.refineWithAI(ctx, model, text, recentCtx, dictionaryEntries)
			if refineErr != nil {
				p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("AI refine failed: %s", refineErr.Error()))
				if ctxErr := refineErr; ctxErr != nil && strings.Contains(ctxErr.Error(), "timeout") {
					p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_ai_timeout"))
				} else {
					p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_ai_failed"))
				}
			} else if strings.TrimSpace(refined) != "" {
				text = strings.TrimSpace(refined)
				aiRefineSucceeded = true
			}
		}
	}

	if !aiRefineSucceeded && p.dictionary != nil {
		text = p.dictionary.applyExact(text)
	}

	// Persist the final transcript (refined if AI was applied, raw otherwise)
	// after refinement resolves so history matches what the user actually gets.
	// Best-effort: save failures are logged inside the store and do not block
	// the typing output.
	p.history.add(ctx, text, util.GetSystemTimestamp())

	p.closeDictationOverlay()
	p.playSoundIfEnabled(ctx, soundStop)

	// Wait briefly for the overlay to close and focus to return to the
	// previously focused window.
	time.Sleep(100 * time.Millisecond)
	if err := keyboard.SimulateType(text); err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to type dictation text: %s", err.Error()))
		p.api.Notify(ctx, fmt.Sprintf("%s: %s", i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_type_failed"), err.Error()))
	}
}

// isAIRefineEnabled reports whether the user turned on AI refinement.
func (p *DictationPlugin) isAIRefineEnabled(ctx context.Context) bool {
	return parseBoolSetting(p.api.GetSetting(ctx, settingKeyAIRefine))
}

// getAIModel parses the stored AI model setting (a JSON-encoded common.Model,
// the same format the selectAIModel component persists) and returns it. The
// second return value is false when no model is selected or the stored value
// is malformed.
func (p *DictationPlugin) getAIModel(ctx context.Context) (common.Model, bool) {
	raw := strings.TrimSpace(p.api.GetSetting(ctx, settingKeyAIModel))
	if raw == "" {
		return common.Model{}, false
	}
	var model common.Model
	if err := json.Unmarshal([]byte(raw), &model); err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to parse AI model setting: %s", err.Error()))
		return common.Model{}, false
	}
	if model.Name == "" || model.Provider == "" {
		return common.Model{}, false
	}
	return model, true
}

// buildDictationTextOverlayWindow returns the shared window placement for dictation text HUD states.
func buildDictationTextOverlayWindow() overlay.WindowOptions {
	mouseScreen := screen.GetMouseScreen()
	opts := overlay.WindowOptions{
		ID:               dictationOverlayName,
		Topmost:          true,
		AbsolutePosition: true,
		Anchor:           overlay.AnchorBottomCenter,
		OffsetX:          float64(mouseScreen.X) + float64(mouseScreen.Width)/2,
		OffsetY:          float64(mouseScreen.Y+mouseScreen.Height) - overlayBottomOffset,
		CloseOnEscape:    true,
		Movable:          false,
	}

	if mouseScreen.Width == 0 {
		pos, ok := mouse.CurrentPosition()
		if ok {
			opts.OffsetX = pos.X
			opts.OffsetY = pos.Y - overlayBottomOffset
		}
	}

	return opts
}

// showRefiningOverlay switches the existing dictation overlay into a loading
// state with an "AI refining" message while the transcript is being rewritten.
func (p *DictationPlugin) showRefiningOverlay(ctx context.Context) {
	window := buildDictationTextOverlayWindow()
	window.PreservePosition = true
	window.MinWidth = 200
	window.MaxWidth = 600
	window.OnClose = func() {
		// During AI refinement the session is already stopped; just close
		// the overlay without typing the result.
		p.api.Log(util.NewTraceContext(), plugin.LogLevelInfo, "dictation overlay closed during AI refinement")
	}
	opts := textoverlay.Options{
		Window:   window,
		Closable: true,
		Message:  i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_ai_refining"),
		Loading:  true,
		FontSize: 14,
	}

	textoverlay.Show(opts)
}

// showProcessingOverlay replaces the waveform immediately after release so
// the UI acknowledges the key-up event while local recognition finishes.
func (p *DictationPlugin) showProcessingOverlay(ctx context.Context) {
	window := buildDictationTextOverlayWindow()
	window.PreservePosition = true
	window.MinWidth = 200
	window.MaxWidth = 600
	window.OnClose = func() {
		p.api.Log(util.NewTraceContext(), plugin.LogLevelInfo, "dictation overlay closed while processing transcript")
	}

	textoverlay.Show(textoverlay.Options{
		Window:   window,
		Closable: true,
		Message:  i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_processing"),
		Loading:  true,
		FontSize: 14,
	})
	dictationoverlay.Release(dictationOverlayName)
}

// refineWithAI sends the raw transcript to the selected AI model and returns
// the refined text. It blocks until the stream finishes, fails, or the
// timeout elapses; on timeout it returns an error mentioning "timeout" so the
// caller can surface a dedicated message.
//
// recentContext carries the finalized transcripts from the last few minutes
// (oldest-first). It lets the model understand pronouns, tense, and topic
// continuity across consecutive dictations. The current utterance is the only
// text that should be output; context is provided for reference only.
func (p *DictationPlugin) refineWithAI(ctx context.Context, model common.Model, rawText string, recentContext []string, dictionaryEntries []dictionaryEntry) (string, error) {
	refineCtx, cancel := context.WithTimeout(ctx, aiRefineTimeout)
	defer cancel()

	systemPrompt := strings.Join([]string{
		"You are a transcription editor. Rewrite the user's dictated text into fluent, coherent, easy-to-understand sentences while preserving the original meaning and language.",
		"Remove filler words (um, uh, like, you know), fix disfluencies, false starts, repeated words, and sentence fragments.",
		"Choose punctuation based on grammar and meaning, not on speech pauses. Merge fragments that belong to the same sentence, and remove punctuation that splits a natural phrase or clause.",
		"For Chinese, avoid inserting commas or periods inside short verb/result phrases. For example, write \"改为单击就可以更正\" instead of \"改为单击就可以，更正\" when that matches the intended meaning.",
		"Do not add new facts, commands, explanations, quotes, or extra formatting. Output only the refined text.",
	}, " ")

	var userPrompt string
	if len(recentContext) > 0 || len(dictionaryEntries) > 0 {
		var ctxBuf strings.Builder
		if len(dictionaryEntries) > 0 {
			ctxBuf.WriteString("Personal dictionary corrections. Apply a correction only when the new dictation has the same or very similar context as the saved context; do not replace a phrase merely because the wrong phrase appears:\n")
			for i, entry := range dictionaryEntries {
				if i >= 80 {
					break
				}
				ctxBuf.WriteString(fmt.Sprintf("%d. Context: %s\n   Wrong: %s\n   Correct: %s\n", i+1, entry.Context, entry.WrongPhrase, entry.CorrectPhrase))
			}
			ctxBuf.WriteString("\n")
		}
		if len(recentContext) > 0 {
			ctxBuf.WriteString("Previous dictation context (for reference only, do not repeat or rewrite these):\n")
			for i, c := range recentContext {
				ctxBuf.WriteString(fmt.Sprintf("%d. %s\n", i+1, c))
			}
			ctxBuf.WriteString("\n")
		}
		ctxBuf.WriteString("Now refine the following new dictation:\n")
		ctxBuf.WriteString(rawText)
		userPrompt = ctxBuf.String()
	} else {
		userPrompt = rawText
	}

	conversations := []common.Conversation{
		{
			Role: common.ConversationRoleSystem,
			Text: systemPrompt,
		},
		{
			Role: common.ConversationRoleUser,
			Text: userPrompt,
		},
	}

	// AIChatStream runs its loop in a goroutine and reports status via the
	// callback. We wait on a channel for a terminal status so this function
	// stays synchronous from stopAndOutput's perspective.
	done := make(chan struct {
		text string
		err  error
	}, 1)

	var accumulated string
	err := p.api.AIChatStream(refineCtx, model, conversations, common.ChatOptions{
		ThinkingMode: common.ChatThinkingModeNonThinking,
	}, func(streamResult common.ChatStreamData) {
		switch streamResult.Status {
		case common.ChatStreamStatusStreaming, common.ChatStreamStatusStreamed:
			accumulated = streamResult.Data
		case common.ChatStreamStatusFinished:
			done <- struct {
				text string
				err  error
			}{streamResult.Data, nil}
		case common.ChatStreamStatusError:
			done <- struct {
				text string
				err  error
			}{accumulated, fmt.Errorf("ai stream error: %s", streamResult.Data)}
		}
	})
	if err != nil {
		return "", fmt.Errorf("failed to start AI stream: %w", err)
	}

	select {
	case res := <-done:
		return res.text, res.err
	case <-refineCtx.Done():
		return "", fmt.Errorf("AI refinement timeout")
	}
}

// showLoadingOverlay displays the overlay with a "Loading model..." message
// before the recognizer is ready. This gives the user immediate feedback when
// they press the hotkey, even while the model is still being loaded.
func (p *DictationPlugin) showLoadingOverlay(ctx context.Context, messageKey string) {
	window := buildDictationTextOverlayWindow()
	window.MinWidth = 200
	window.MaxWidth = 600
	window.OnClose = func() {
		p.cancelDictation(util.NewTraceContext())
	}

	textoverlay.Show(textoverlay.Options{
		Window:   window,
		Closable: true,
		Message:  i18n.GetI18nManager().TranslateWox(ctx, messageKey),
		Loading:  true,
		FontSize: 14,
	})
}

// showErrorOverlay displays a closeable, auto-closing dictation error without
// starting or cancelling a recording session.
func (p *DictationPlugin) showErrorOverlay(ctx context.Context, message string) {
	window := buildDictationTextOverlayWindow()
	window.MinWidth = 240
	window.MaxWidth = 680

	textoverlay.Show(textoverlay.Options{
		Window:           window,
		Closable:         true,
		AutoCloseSeconds: 6,
		Message:          message,
		FontSize:         14,
	})
}

// showDictationOverlay displays the recording waveform overlay at the
// bottom-center of the screen the mouse is currently on.
func (p *DictationPlugin) showDictationOverlay(ctx context.Context, voiceActive bool) {
	mouseScreen := screen.GetMouseScreen()

	opts := overlay.WindowOptions{
		ID:               dictationOverlayName,
		Topmost:          true,
		AbsolutePosition: true,
		Anchor:           overlay.AnchorBottomCenter,
		OffsetX:          float64(mouseScreen.X) + float64(mouseScreen.Width)/2,
		OffsetY:          float64(mouseScreen.Y+mouseScreen.Height) - overlayBottomOffset,
		CloseOnEscape:    true,
		Movable:          false,
		OnClose: func() {
			p.cancelDictation(util.NewTraceContext())
		},
	}

	if mouseScreen.Width == 0 {
		pos, ok := mouse.CurrentPosition()
		if ok {
			opts.OffsetX = pos.X
			opts.OffsetY = pos.Y - overlayBottomOffset
		}
	}

	dictationoverlay.Show(dictationoverlay.Options{
		Window:   opts,
		Active:   voiceActive,
		Closable: true,
	})
}

// resetVoiceOverlayState clears any activity remembered from a previous
// dictation before a new session starts delivering audio callbacks.
func (p *DictationPlugin) resetVoiceOverlayState() {
	p.voiceOverlayMu.Lock()
	defer p.voiceOverlayMu.Unlock()
	p.voiceOverlayActive = false
	p.voiceOverlayStateSet = false
	p.voiceOverlayVisible = false
}

// currentVoiceOverlayActive returns the latest speech activity state to use
// when the recording overlay is first shown after session startup.
func (p *DictationPlugin) currentVoiceOverlayActive() bool {
	p.voiceOverlayMu.Lock()
	defer p.voiceOverlayMu.Unlock()
	return p.voiceOverlayActive
}

// setVoiceOverlayVisible gates native overlay refreshes until the recording
// session has fully started and can be cancelled safely.
func (p *DictationPlugin) setVoiceOverlayVisible(visible bool) {
	p.voiceOverlayMu.Lock()
	defer p.voiceOverlayMu.Unlock()
	p.voiceOverlayVisible = visible
}

// updateVoiceOverlay refreshes the recording waveform only when speech
// activity changes, so native animation owns the steady-state motion.
func (p *DictationPlugin) updateVoiceOverlay(ctx context.Context, voiceActive bool) {
	p.voiceOverlayMu.Lock()
	if p.voiceOverlayStateSet && p.voiceOverlayActive == voiceActive {
		p.voiceOverlayMu.Unlock()
		return
	}
	p.voiceOverlayActive = voiceActive
	p.voiceOverlayStateSet = true
	visible := p.voiceOverlayVisible
	p.voiceOverlayMu.Unlock()

	if !visible {
		return
	}
	dictationoverlay.UpdateActive(dictationOverlayName, voiceActive)
}

// updateOverlay refreshes the overlay text with the latest partial result.
// Throttled to ~80ms intervals to avoid excessive redraw.
func (p *DictationPlugin) updateOverlay(ctx context.Context, text string) {
	now := time.Now()
	if now.Sub(p.lastOverlayUpdate) < 80*time.Millisecond {
		return
	}
	p.lastOverlayUpdate = now

	window := buildDictationTextOverlayWindow()
	window.PreservePosition = true
	window.MinWidth = 200
	window.MaxWidth = 600

	textoverlay.Show(textoverlay.Options{
		Window:   window,
		Closable: true,
		Message:  text,
		Loading:  true,
		FontSize: 14,
	})
}

// closeDictationOverlay removes the recording overlay.
func (p *DictationPlugin) closeDictationOverlay() {
	p.setVoiceOverlayVisible(false)
	dictationoverlay.Close(dictationOverlayName)
}

// evictOldModels removes all cached recognizer models from the pool except
// the one matching the newly selected model ID. Called when the user switches
// models in settings so the old model's memory is freed immediately.
func (p *DictationPlugin) evictOldModels(ctx context.Context, newModelID string) {
	if p.recognizerPool == nil || p.modelManager == nil {
		return
	}
	models, err := p.modelManager.ListLocalModels()
	if err != nil {
		return
	}
	for _, m := range models {
		if m.ID == newModelID {
			p.recognizerPool.EvictExcept(m.Path)
			return
		}
	}
	// New model not found on disk yet — evict everything.
	p.recognizerPool.EvictExcept("")
}

// cancelDictation is called when the user clicks the close button on the
// dictation overlay. It stops the recording session and discards the result
// without typing it into the focused window.
func (p *DictationPlugin) cancelDictation(ctx context.Context) {
	p.api.Log(ctx, plugin.LogLevelInfo, "dictation cancelled by user via overlay close button")
	p.setVoiceOverlayVisible(false)
	p.stopVolumeDucking(ctx)

	p.sessionMu.Lock()
	session := p.session
	p.session = nil
	p.isRecording = false
	p.isStarting = false
	p.pendingStop = false
	p.sessionMu.Unlock()

	if session != nil {
		// Stop the session and discard the text. We still need to release
		// resources back to the pools.
		go func() {
			_, _ = session.Stop()
		}()
	}

	// Play the stop sound since the overlay is closing.
	p.playSoundIfEnabled(ctx, soundStop)
}

// playSoundIfEnabled plays an embedded audio clip when the playSound setting
// is on. Errors are logged but never propagated so they can't disrupt
// recording or typing.
// startVolumeDucking pauses other media playback when the duckVolume
// setting is enabled, so other audio does not interfere with dictation.
// Uses InvokePluginCommand to ask the media player plugin to pause.
func (p *DictationPlugin) startVolumeDucking(ctx context.Context) {
	enabled := parseBoolSetting(p.api.GetSetting(ctx, settingKeyDuckVolume))
	p.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("dictation: startVolumeDucking, enabled=%t", enabled))
	if !enabled {
		return
	}
	_, err := p.api.InvokePluginCommand(ctx, plugin.PluginCommandRequest{
		PluginId: "b8f3d4e5-6c7a-4b9c-8d1e-2f3a4b5c6d7e",
		Command:  "pause",
	})
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to pause media: %s", err.Error()))
	} else {
		p.api.Log(ctx, plugin.LogLevelInfo, "dictation: media paused via plugin command")
	}
}

// stopVolumeDucking resumes media playback if it was previously paused.
func (p *DictationPlugin) stopVolumeDucking(ctx context.Context) {
	if !parseBoolSetting(p.api.GetSetting(ctx, settingKeyDuckVolume)) {
		return
	}
	_, err := p.api.InvokePluginCommand(ctx, plugin.PluginCommandRequest{
		PluginId: "b8f3d4e5-6c7a-4b9c-8d1e-2f3a4b5c6d7e",
		Command:  "play",
	})
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to resume media: %s", err.Error()))
	} else {
		p.api.Log(ctx, plugin.LogLevelInfo, "dictation: media resumed via plugin command")
	}
}

func (p *DictationPlugin) playSoundIfEnabled(ctx context.Context, name string) {
	if !parseBoolSetting(p.api.GetSetting(ctx, settingKeyPlaySound)) {
		return
	}
	soundPath := ensureDictationResourceFile(ctx, name)
	if soundPath == "" {
		return
	}
	if err := audio.Play(ctx, soundPath); err != nil {
		p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to play dictation sound %s: %s", name, err.Error()))
	}
}

// parseBoolSetting maps the string setting value ("true"/"false") to a bool,
// defaulting to false for unrecognized values.
func parseBoolSetting(v string) bool {
	return v == "true"
}

// extractVadModel returns the extracted silero_vad.onnx path, writing it from
// embedded resources when the normal startup extraction has not run yet.
func extractVadModel(ctx context.Context) string {
	return ensureDictationResourceFile(ctx, "silero_vad.onnx")
}

// ensureDictationResourceFile returns the extracted resource path, writing it
// from embedded resources when the normal startup extraction has not run yet.
func ensureDictationResourceFile(ctx context.Context, name string) string {
	resourcePath := resource.GetDictationResourcePath(name)
	if util.IsFileExists(resourcePath) {
		return resourcePath
	}

	data, err := resource.GetDictationFile(name)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to read embedded dictation resource %s: %s", name, err.Error()))
		return ""
	}

	if err := util.GetLocation().EnsureDirectoryExist(filepath.Dir(resourcePath)); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to create dictation resource dir: %s", err.Error()))
		return ""
	}

	if err := os.WriteFile(resourcePath, data, 0644); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to write dictation resource %s: %s", name, err.Error()))
		return ""
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("extracted dictation resource %s to %s", name, resourcePath))
	return resourcePath
}
