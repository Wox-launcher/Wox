package dictation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/plugin/system/mediaplayer"
	"wox/resource"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
	"wox/util/audio"
	"wox/util/clipboard"
	"wox/util/keyboard"
	"wox/util/mouse"
	"wox/util/overlay"
	"wox/util/overlay/dictationoverlay"
	"wox/util/overlay/textoverlay"
	"wox/util/screen"
	"wox/util/selection"
	"wox/util/speech"

	"github.com/google/uuid"
)

const (
	// Setting keys
	settingKeyDefaultHotkey   = "defaultHotkey"
	settingKeyDefaultAIRefine = "defaultAIRefineEnabled"
	settingKeyDefaultAIModel  = "defaultAIModel"
	settingKeyInputDevice     = "inputDevice"
	settingKeyInputDeviceName = "inputDeviceName"
	settingKeyModel           = "model"
	settingKeyModelLoadMode   = "modelLoadMode"
	settingKeyPlaySound       = "playSound"
	settingKeyDuckVolume      = "duckVolume"

	inputDeviceSystem = "system"

	dictationModelLoadModeLazy  = "lazy"
	dictationModelLoadModeEager = "eager"

	// AI refinement timeout. Picked to cover a normal model response for a
	// short dictation transcript while keeping the wait perceptible.
	aiRefineTimeout = 5 * time.Second

	// Custom actions can ask the model to explain or transform selected text,
	// so they get a longer timeout than default dictation cleanup.
	aiActionTimeout = 60 * time.Second

	// recognizerPoolIdleTTL controls how long an unused speech model stays in
	// memory before being evicted. 10 minutes covers typical back-to-back
	// dictation bursts while reclaiming ~70-150MB during longer pauses.
	recognizerPoolIdleTTL = 10 * time.Minute

	// Embedded audio clips played when the dictation overlay shows/hides.
	soundStart = "dictation_start.wav"
	soundStop  = "dictation_stop.wav"

	// Overlay
	dictationOverlayName = "dictation-indicator"

	// Overlay position: distance from the bottom of the mouse screen.
	overlayBottomOffset = 80.0

	dictationOverlayResultName = "dictation-action-result"
)

var (
	dictationIcon = common.PluginDictationIcon

	errInputDeviceMissing = errors.New("input device missing")

	// hotkeyRegistrar is set by the UI layer at startup to avoid a circular
	// import (ui imports plugin/system/dictation, so dictation cannot import ui).
	// The UI Manager registers itself as the hotkey registrar via SetHotkeyRegistrar.
	hotkeyRegistrar HotkeyRegistrar
)

// HotkeyRegistrar abstracts the UI Manager's dictation hotkey registration
// so the dictation plugin can register/unregister global hotkeys without
// importing the ui package directly.
type HotkeyRegistrar interface {
	RegisterDictationHotkeys(ctx context.Context, bindings []HotkeyBinding) error
}

// HotkeyBinding is the runtime hotkey binding for one dictation action.
type HotkeyBinding struct {
	ActionID string
	Hotkey   string
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

	// runtimeMu keeps session startup and plugin unload from closing the speech
	// pools while a model is still being acquired.
	runtimeMu sync.Mutex

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
	// mediaPausedForDictation tracks whether this dictation session paused external media.
	mediaPausedForDictation bool

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

	actionsMu sync.RWMutex
	actions   []dictationAction

	// history persists past dictation transcripts so the Query surface can
	// list them by time. Stored as a plugin setting so cloud sync covers it.
	history *historyStore

	// dictionary keeps user-approved correction rules for future dictations.
	dictionary *dictionaryStore

	activeAction       dictationAction
	activeInputContext dictationActionInputContext
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
				Type: definition.PluginSettingDefinitionTypeDictationHotkey,
				Value: &definition.PluginSettingValueDictationHotkey{
					Key:          settingKeyDefaultHotkey,
					Label:        "i18n:plugin_dictation_hotkey",
					Tooltip:      "i18n:plugin_dictation_hotkey_tooltip",
					DefaultValue: "",
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
			{
				Type: definition.PluginSettingDefinitionTypeSelect,
				Value: &definition.PluginSettingValueSelect{
					Key:          settingKeyModelLoadMode,
					Label:        "i18n:plugin_dictation_model_load_mode",
					Tooltip:      "i18n:plugin_dictation_model_load_mode_tooltip",
					DefaultValue: dictationModelLoadModeLazy,
					Options: []definition.PluginSettingValueSelectOption{
						{Label: "i18n:plugin_dictation_model_load_mode_lazy", Value: dictationModelLoadModeLazy},
						{Label: "i18n:plugin_dictation_model_load_mode_eager", Value: dictationModelLoadModeEager},
					},
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
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          settingKeyDefaultAIRefine,
					Label:        "i18n:plugin_dictation_ai_enable",
					Tooltip:      "i18n:plugin_dictation_ai_enable_tooltip",
					DefaultValue: "false",
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeDynamic,
				Value: &definition.PluginSettingValueDynamic{
					Key: settingKeyDefaultAIModel,
				},
			},
			// Dictionary is a dynamic setting: it is only shown when AI refinement
			// is enabled on the default action, because the phrase list is consumed
			// exclusively by the AI refiner prompt.
			{
				Type: definition.PluginSettingDefinitionTypeDynamic,
				Value: &definition.PluginSettingValueDynamic{
					Key: settingKeyDictionary,
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key:          settingKeyActions,
					DefaultValue: "[]",
					Title:        "i18n:plugin_dictation_actions",
					Tooltip:      "i18n:plugin_dictation_actions_tooltip",
					MaxHeight:    300,
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:          "id",
							Label:        "ID",
							Type:         definition.PluginSettingValueTableColumnTypeText,
							HideInTable:  true,
							HideInUpdate: true,
						},
						{
							Key:          "type",
							Label:        "Type",
							Type:         definition.PluginSettingValueTableColumnTypeText,
							HideInTable:  true,
							HideInUpdate: true,
						},

						{
							Key:     "name",
							Label:   "i18n:plugin_dictation_action_name",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Width:   140,
							Tooltip: "i18n:plugin_dictation_action_name_tooltip",
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:     "hotkey",
							Label:   "i18n:plugin_dictation_action_hotkey",
							Type:    definition.PluginSettingValueTableColumnTypeHotkey,
							Width:   150,
							Tooltip: "i18n:plugin_dictation_action_hotkey_tooltip",
							AllowedHotkeyKinds: []string{
								"normalCombo",
								"doubleModifier",
								"capsLockCombo",
								"pressModifier",
								"holdModifier",
							},
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:     "output",
							Label:   "i18n:plugin_dictation_action_output",
							Type:    definition.PluginSettingValueTableColumnTypeSelect,
							Width:   130,
							Tooltip: "i18n:plugin_dictation_action_output_tooltip",
							SelectOptions: []definition.PluginSettingValueSelectOption{
								{Label: "i18n:plugin_dictation_action_output_input", Value: dictationActionOutputInput},
								{Label: "i18n:plugin_dictation_action_output_overlay", Value: dictationActionOutputOverlay},
								{Label: "i18n:plugin_dictation_action_output_chat", Value: dictationActionOutputChat},
							},
						},
						{
							Key:     "model",
							Label:   "i18n:plugin_dictation_action_ai_model",
							Type:    definition.PluginSettingValueTableColumnTypeSelectAIModel,
							Width:   180,
							Tooltip: "i18n:plugin_dictation_action_ai_model_tooltip",
						},
						{
							Key:          "prompt",
							Label:        "i18n:plugin_dictation_action_prompt",
							Type:         definition.PluginSettingValueTableColumnTypeDictationPrompt,
							TextMaxLines: 8,
							Tooltip:      "i18n:plugin_dictation_action_prompt_tooltip",
						},
						{
							Key:     "disabled",
							Label:   "i18n:plugin_dictation_action_disabled",
							Type:    definition.PluginSettingValueTableColumnTypeCheckbox,
							Width:   80,
							Tooltip: "i18n:plugin_dictation_action_disabled_tooltip",
						},
					},
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
	p.reloadActions(ctx, p.api.GetSetting(ctx, settingKeyActions))

	// Initialize the model manager in the Wox data directory.
	modelsDir := filepath.Join(util.GetLocation().GetDictationDirectory(), "models")
	mgr, err := speech.NewModelManager(modelsDir)
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to create model manager: %s", err.Error()))
	} else {
		p.modelManager = mgr
	}

	loadMode := normalizeModelLoadMode(p.api.GetSetting(ctx, settingKeyModelLoadMode))

	// Start the recognizer pool. Lazy mode evicts idle models; eager mode keeps
	// the selected model resident until unload or model switch.
	p.recognizerPool = speech.NewRecognizerPool(recognizerPoolIdleTTLForMode(loadMode))
	p.recognizerPool.StartReaper(ctx)

	// Start the audio capture pool so the malgo context + device stay alive
	// across sessions, eliminating the InitDevice delay.
	p.audioCapturePool = speech.NewAudioCapturePool(recognizerPoolIdleTTL)
	p.audioCapturePool.StartReaper(ctx)

	// Extract the embedded silero VAD model to a temp file and start the VAD pool.
	p.vadModelPath = extractVadModel(ctx)
	p.vadPool = speech.NewVadPool(recognizerPoolIdleTTL)
	p.vadPool.StartReaper(ctx)

	if loadMode == dictationModelLoadModeEager {
		p.preloadSelectedModelAsync(ctx)
	}

	p.api.OnUnload(ctx, func(ctx context.Context) {
		p.releaseRuntime(ctx)
	})

	// Register dynamic setting callbacks for input device and model.
	p.api.OnGetDynamicSetting(ctx, func(ctx context.Context, key string) definition.PluginSettingDefinitionItem {
		switch key {
		case settingKeyInputDevice:
			return p.buildInputDeviceSetting(ctx)
		case settingKeyModel:
			return p.buildModelSetting(ctx)
		case settingKeyDefaultAIModel:
			return p.buildDefaultAIModelSetting(ctx)
		case settingKeyDictionary:
			return p.buildDictionarySetting(ctx)
		}
		return definition.PluginSettingDefinitionItem{}
	})

	// React to setting changes: re-register the hotkey and remember input device names.
	p.api.OnSettingChanged(ctx, func(ctx context.Context, key string, value string) {
		switch key {
		case settingKeyActions:
			p.reloadActions(ctx, value)
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
			if normalizeModelLoadMode(p.api.GetSetting(ctx, settingKeyModelLoadMode)) == dictationModelLoadModeEager {
				p.preloadSelectedModelAsync(ctx)
			}
		case settingKeyModelLoadMode:
			p.updateModelLoadMode(ctx, value)
		}
	})
}

// reloadActions normalizes persisted action rows, updates the in-memory copy,
// and refreshes runtime hotkey registrations.
func (p *DictationPlugin) reloadActions(ctx context.Context, raw string) {
	actions := normalizeDictationActions(parseDictationActions(raw))
	normalizedRaw := marshalDictationActions(actions)

	p.actionsMu.Lock()
	p.actions = actions
	p.actionsMu.Unlock()

	if strings.TrimSpace(raw) != normalizedRaw {
		p.api.SaveSetting(ctx, settingKeyActions, normalizedRaw, false)
	}
	p.reregisterActionHotkeys(ctx, actions)
}

func (p *DictationPlugin) actionSnapshot() []dictationAction {
	p.actionsMu.RLock()
	defer p.actionsMu.RUnlock()

	actions := make([]dictationAction, len(p.actions))
	copy(actions, p.actions)
	return actions
}

func (p *DictationPlugin) actionByID(actionID string) (dictationAction, bool) {
	for _, action := range p.actionSnapshot() {
		if action.ID == actionID {
			return action, true
		}
	}
	return dictationAction{}, false
}

// reregisterActionHotkeys binds all active dictation action hotkeys via the
// injected UI Manager registrar.
func (p *DictationPlugin) reregisterActionHotkeys(ctx context.Context, actions []dictationAction) {
	p.registeredHotkeyMu.Lock()
	defer p.registeredHotkeyMu.Unlock()
	if hotkeyRegistrar == nil {
		p.api.Log(ctx, plugin.LogLevelDebug, "hotkey registrar not set, skipping hotkey registration")
		return
	}

	bindings := make([]HotkeyBinding, 0, len(actions))
	for _, action := range actions {
		if action.Disabled || strings.TrimSpace(action.Hotkey) == "" {
			continue
		}
		bindings = append(bindings, HotkeyBinding{
			ActionID: action.ID,
			Hotkey:   action.Hotkey,
		})
	}

	if err := hotkeyRegistrar.RegisterDictationHotkeys(ctx, bindings); err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to register dictation action hotkeys: %s", err.Error()))
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

func normalizeModelLoadMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case dictationModelLoadModeEager:
		return dictationModelLoadModeEager
	default:
		return dictationModelLoadModeLazy
	}
}

func recognizerPoolIdleTTLForMode(mode string) time.Duration {
	if normalizeModelLoadMode(mode) == dictationModelLoadModeEager {
		return 0
	}
	return recognizerPoolIdleTTL
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

// findLocalModel returns the selected downloaded model from disk.
func (p *DictationPlugin) findLocalModel(ctx context.Context, modelID string) (*speech.LocalModel, error) {
	if p.modelManager == nil {
		return nil, fmt.Errorf("model manager unavailable")
	}
	models, err := p.modelManager.ListLocalModels()
	if err != nil {
		return nil, err
	}
	for i := range models {
		if models[i].ID == modelID {
			model := models[i]
			return &model, nil
		}
	}
	return nil, fmt.Errorf("selected model not found")
}

// recognizerConfigForModel keeps dictation thread selection consistent across
// live sessions and eager preloading.
func (p *DictationPlugin) recognizerConfigForModel(ctx context.Context, selectedModel speech.LocalModel) speech.RecognizerConfig {
	cpuCount := runtime.NumCPU()
	recognizerThreads := max(cpuCount/2, 1)
	p.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("dictation: recognizer config model=%s modelType=%s cpu=%d numThreads=%d", selectedModel.ID, selectedModel.ModelType, cpuCount, recognizerThreads))
	return speech.RecognizerConfig{
		ModelPath:  selectedModel.Path,
		ModelType:  selectedModel.ModelType,
		NumThreads: recognizerThreads,
	}
}

// updateModelLoadMode applies the selected memory/speed policy immediately.
func (p *DictationPlugin) updateModelLoadMode(ctx context.Context, rawMode string) {
	mode := normalizeModelLoadMode(rawMode)
	if p.recognizerPool != nil {
		p.recognizerPool.SetIdleTTL(recognizerPoolIdleTTLForMode(mode))
	}
	if mode == dictationModelLoadModeEager {
		p.preloadSelectedModelAsync(ctx)
	}
}

// preloadSelectedModelAsync loads the selected recognizer in the background
// only after the user explicitly opts into eager loading.
func (p *DictationPlugin) preloadSelectedModelAsync(ctx context.Context) {
	modelID := strings.TrimSpace(p.api.GetSetting(ctx, settingKeyModel))
	if modelID == "" {
		p.api.Log(ctx, plugin.LogLevelDebug, "dictation: eager preload skipped because no model is selected")
		return
	}
	util.Go(ctx, "dictation eager preload recognizer", func() {
		p.preloadSelectedModel(ctx, modelID)
	})
}

// preloadSelectedModel acquires then releases the recognizer so it stays cached.
func (p *DictationPlugin) preloadSelectedModel(ctx context.Context, modelID string) {
	t0 := time.Now()
	if normalizeModelLoadMode(p.api.GetSetting(ctx, settingKeyModelLoadMode)) != dictationModelLoadModeEager {
		return
	}

	selectedModel, err := p.findLocalModel(ctx, modelID)
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("dictation: eager preload skipped for model=%s: %s", modelID, err.Error()))
		return
	}
	config := p.recognizerConfigForModel(ctx, *selectedModel)

	p.runtimeMu.Lock()
	defer p.runtimeMu.Unlock()
	if p.recognizerPool == nil {
		return
	}

	rec, err := p.recognizerPool.Acquire(ctx, config)
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("dictation: eager preload failed for model=%s: %s", selectedModel.ID, err.Error()))
		return
	}
	p.recognizerPool.Release(ctx, rec)

	if p.api.GetSetting(ctx, settingKeyModel) != modelID || normalizeModelLoadMode(p.api.GetSetting(ctx, settingKeyModelLoadMode)) != dictationModelLoadModeEager {
		p.evictOldModels(ctx, p.api.GetSetting(ctx, settingKeyModel))
		return
	}
	p.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("dictation: eager preload ready model=%s cost=%dms", selectedModel.ID, time.Since(t0).Milliseconds()))
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

// buildDefaultAIModelSetting hides the AI model picker until default dictation AI refinement is enabled.
func (p *DictationPlugin) buildDefaultAIModelSetting(ctx context.Context) definition.PluginSettingDefinitionItem {
	defaultAction := defaultDictationActionFromSetting(p.api.GetSetting(ctx, settingKeyActions))
	if !defaultAction.AIRefineEnabled {
		return definition.PluginSettingDefinitionItem{}
	}

	return definition.PluginSettingDefinitionItem{
		Type: definition.PluginSettingDefinitionTypeSelectAIModel,
		Value: &definition.PluginSettingValueSelectAIModel{
			Key:   settingKeyDefaultAIModel,
			Label: "i18n:plugin_dictation_ai_model",
		},
	}
}

// buildDictionarySetting hides the phrase dictionary until AI refinement is
// enabled, because the phrase list is consumed exclusively by the AI refiner.
func (p *DictationPlugin) buildDictionarySetting(ctx context.Context) definition.PluginSettingDefinitionItem {
	defaultAction := defaultDictationActionFromSetting(p.api.GetSetting(ctx, settingKeyActions))
	if !defaultAction.AIRefineEnabled {
		return definition.PluginSettingDefinitionItem{}
	}

	return definition.PluginSettingDefinitionItem{
		Type: definition.PluginSettingDefinitionTypeTable,
		Value: &definition.PluginSettingValueTable{
			Key:          settingKeyDictionary,
			DefaultValue: "[]",
			Title:        "i18n:plugin_dictation_dictionary",
			Tooltip:      "i18n:plugin_dictation_dictionary_tooltip",
			MaxHeight:    260,
			Columns: []definition.PluginSettingValueTableColumn{
				{
					Key:          "phrase",
					Label:        "i18n:plugin_dictation_dictionary_phrase",
					Type:         definition.PluginSettingValueTableColumnTypeText,
					Width:        260,
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

// PressDictationHotkey is called by press-triggered action hotkeys. It starts
// recording if idle and stops if recording.
func (p *DictationPlugin) PressDictationHotkey(ctx context.Context, actionID string) {
	p.sessionMu.Lock()
	if p.isRecording {
		p.sessionMu.Unlock()
		p.stopAndOutput(ctx)
		return
	}
	if p.isStarting {
		p.pendingStop = true
		p.sessionMu.Unlock()
		p.api.Log(ctx, plugin.LogLevelDebug, "dictation: press hotkey during startup, pendingStop set")
		return
	}
	p.sessionMu.Unlock()
	p.startRecording(ctx, actionID)
}

// StartDictation is called by the action hotkey press handler in hold mode.
func (p *DictationPlugin) StartDictation(ctx context.Context, actionID string) {
	p.sessionMu.Lock()
	if p.isRecording || p.isStarting {
		p.sessionMu.Unlock()
		return
	}
	p.sessionMu.Unlock()
	p.startRecording(ctx, actionID)
}

// StopDictation is called by the action hotkey release handler in hold mode.
// If startRecording is still in progress (model loading), it sets a
// pendingStop flag so startRecording can stop immediately after it
// finishes — preventing the overlay from being stuck open.
func (p *DictationPlugin) StopDictation(ctx context.Context, actionID string) {
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

// prepareActionInputContext resolves prompt-declared external inputs before
// recording starts, so the later overlay/focus changes cannot alter context.
func (p *DictationPlugin) prepareActionInputContext(ctx context.Context, action dictationAction) (dictationActionInputContext, bool) {
	if !actionNeedsSelectedText(action) {
		return dictationActionInputContext{}, true
	}

	selected, selectedErr := selection.GetSelected(ctx)
	if selectedErr != nil {
		p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to get selected text for dictation action %s: %s", action.ID, selectedErr.Error()))
		p.api.Notify(ctx, "plugin_dictation_action_selected_text_required")
		return dictationActionInputContext{}, false
	}
	if selected.Type != selection.SelectionTypeText || strings.TrimSpace(selected.Text) == "" {
		p.api.Notify(ctx, "plugin_dictation_action_selected_text_required")
		return dictationActionInputContext{}, false
	}

	return dictationActionInputContext{SelectedText: selected.Text}, true
}

// startRecording initializes the recognizer and audio capture, then shows the overlay.
func (p *DictationPlugin) startRecording(ctx context.Context, actionID string) {
	t0 := time.Now()
	p.api.Log(ctx, plugin.LogLevelDebug, "dictation timing: plugin.startRecording enter")

	p.sessionMu.Lock()
	if p.isRecording || p.isStarting {
		p.sessionMu.Unlock()
		p.api.Log(ctx, plugin.LogLevelDebug, "dictation: startRecording ignored while busy")
		return
	}
	p.isStarting = true
	p.pendingStop = false
	p.sessionMu.Unlock()

	clearStarting := func() {
		p.sessionMu.Lock()
		p.isStarting = false
		p.pendingStop = false
		p.sessionMu.Unlock()
	}

	action, actionFound := p.actionByID(actionID)
	if !actionFound || action.Disabled {
		p.api.Notify(ctx, "plugin_dictation_action_unavailable")
		clearStarting()
		return
	}

	inputContext, ok := p.prepareActionInputContext(ctx, action)
	if !ok {
		clearStarting()
		return
	}

	// Read settings
	deviceID := p.api.GetSetting(ctx, settingKeyInputDevice)
	if deviceID == "" {
		deviceID = inputDeviceSystem
	}
	modelID := p.api.GetSetting(ctx, settingKeyModel)

	if modelID == "" {
		p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_no_model_selected"))
		clearStarting()
		return
	}

	if p.modelManager == nil {
		p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_model_error"))
		clearStarting()
		return
	}

	// Find the model on disk.
	models, err := p.modelManager.ListLocalModels()
	if err != nil {
		p.api.Notify(ctx, fmt.Sprintf("%s: %s", i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_model_error"), err.Error()))
		clearStarting()
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
		clearStarting()
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
		clearStarting()
		return
	}
	deviceID = resolvedDeviceID
	if resolvedDeviceName != "" {
		p.api.SaveSetting(ctx, settingKeyInputDeviceName, resolvedDeviceName, false)
	}

	// Show the overlay immediately so the user gets instant feedback while the
	// native dictation engine and recognition model are being prepared.
	// Lower other audio as soon as the overlay appears. This pauses/ducks
	// other apps' audio but does not affect Wox's own beep sounds.
	p.startVolumeDucking(ctx)

	p.showLoadingOverlay(ctx, "plugin_dictation_loading_model")

	// Create the session with VAD + offline recognizer pools.
	config := p.recognizerConfigForModel(ctx, *selectedModel)
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

	p.runtimeMu.Lock()
	if p.recognizerPool == nil || p.audioCapturePool == nil || p.vadPool == nil {
		p.runtimeMu.Unlock()
		p.api.Log(ctx, plugin.LogLevelError, "dictation start failed: runtime is not initialized")
		p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_start_failed"))
		p.closeDictationOverlay()
		p.stopVolumeDucking(ctx)
		clearStarting()
		return
	}

	if err := session.Start(); err != nil {
		p.runtimeMu.Unlock()
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("dictation start failed: %s", err.Error()))
		p.api.Notify(ctx, fmt.Sprintf("%s: %s", i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_start_failed"), err.Error()))
		p.closeDictationOverlay()
		p.stopVolumeDucking(ctx)
		clearStarting()
		return
	}
	p.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("dictation timing: plugin.sessionStart cost=%dms", time.Since(t0).Milliseconds()))

	p.sessionMu.Lock()
	shouldStop := p.pendingStop
	p.pendingStop = false
	if shouldStop {
		p.isStarting = false
		p.sessionMu.Unlock()
		p.stopVolumeDucking(ctx)
		if _, err := session.Stop(); err != nil {
			p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to stop dictation session during startup cancel: %s", err.Error()))
		}
		p.closeDictationOverlay()
		p.runtimeMu.Unlock()
		p.api.Log(ctx, plugin.LogLevelDebug, "dictation: startup cancelled before recording began")
		return
	}
	p.sessionMu.Unlock()

	// Model loaded and audio capture started. Switch the overlay to
	// "Listening..." and play the start sound to signal the user can speak.
	p.setVoiceOverlayVisible(true)
	p.showDictationOverlay(ctx, p.currentVoiceOverlayActive())

	p.playSoundIfEnabled(ctx, soundStart)

	p.sessionMu.Lock()
	p.session = session
	p.isRecording = true
	p.activeAction = action
	p.activeInputContext = inputContext
	p.isStarting = false
	p.sessionMu.Unlock()
	p.runtimeMu.Unlock()

	p.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("dictation timing: plugin.total cost=%dms", time.Since(t0).Milliseconds()))
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
	action := p.activeAction
	inputContext := p.activeInputContext
	p.session = nil
	p.isRecording = false
	p.activeAction = dictationAction{}
	p.activeInputContext = dictationActionInputContext{}
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

	if action.ID == "" {
		action = newDefaultDictationAction()
	}
	outputText, historyText, usedAI, ok := p.prepareActionOutput(ctx, action, text, inputContext)
	if !ok {
		p.closeDictationOverlay()
		p.playSoundIfEnabled(ctx, soundStop)
		return
	}

	// Persist after action processing so history matches the user-visible
	// result for input/overlay actions and keeps the spoken request for chat.
	// Best-effort: save failures are logged inside the store and do not block
	// the output path.
	p.history.add(ctx, historyText, util.GetSystemTimestamp())

	p.closeDictationOverlay()
	p.playSoundIfEnabled(ctx, soundStop)

	// Wait briefly for the overlay to close and focus to return to the
	// previously focused window.
	time.Sleep(100 * time.Millisecond)
	if err := p.executeActionOutput(ctx, action, outputText, text, inputContext, usedAI); err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to output dictation action %s: %s", action.ID, err.Error()))
		p.api.Notify(ctx, err.Error())
	}
}

func (p *DictationPlugin) prepareActionOutput(ctx context.Context, action dictationAction, rawText string, inputContext dictationActionInputContext) (outputText string, historyText string, usedAI bool, ok bool) {
	if action.Type == dictationActionTypeDefault {
		return p.prepareDefaultActionOutput(ctx, action, rawText)
	}

	dictationText := rawText
	aiRefineSucceeded := false

	// When the user has enabled AI Refine on the default action, refine the
	// transcript before passing it to a custom action's AI prompt so the
	// prompt receives clean, punctuated text instead of raw speech output.
	defaultAction := defaultDictationActionFromSetting(p.api.GetSetting(ctx, settingKeyActions))
	if defaultAction.AIRefineEnabled {
		dictationText, aiRefineSucceeded = p.refineTranscript(ctx, defaultAction, dictationText)
	}

	if action.Output == dictationActionOutputChat {
		return dictationText, dictationText, aiRefineSucceeded, true
	}

	if strings.TrimSpace(action.Prompt) == "" {
		return dictationText, dictationText, aiRefineSucceeded, true
	}

	model, modelOk := parseActionAIModel(ctx, p.api, action.Model)
	if !modelOk {
		p.api.Notify(ctx, "plugin_dictation_action_ai_no_model")
		return "", "", false, false
	}

	// Switch the overlay to the action-processing state so the user can see
	// that the refined transcript is now being handled by the action's AI
	// prompt, distinct from the earlier refinement stage.
	p.showActionProcessingOverlay(ctx)
	prompt := renderDictationActionPrompt(action, dictationText, inputContext)
	if strings.TrimSpace(prompt) == "" {
		return dictationText, dictationText, aiRefineSucceeded, true
	}
	answer, actionErr := p.runPromptWithAI(ctx, model, prompt, aiActionTimeout)
	if actionErr != nil {
		p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("dictation action AI failed: %s", actionErr.Error()))
		if strings.Contains(actionErr.Error(), "timeout") {
			p.api.Notify(ctx, "plugin_dictation_action_ai_timeout")
		} else {
			p.api.Notify(ctx, "plugin_dictation_action_ai_failed")
		}
		return "", "", false, false
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		p.api.Notify(ctx, "plugin_dictation_action_ai_empty")
		return "", "", false, false
	}
	return answer, answer, true, true
}

func (p *DictationPlugin) prepareDefaultActionOutput(ctx context.Context, action dictationAction, rawText string) (outputText string, historyText string, usedAI bool, ok bool) {
	text := rawText
	aiRefineSucceeded := false
	if action.AIRefineEnabled {
		text, aiRefineSucceeded = p.refineTranscript(ctx, action, text)
	}

	return text, text, aiRefineSucceeded, true
}

// refineTranscript sends rawText through the AI refinement model configured on
// the supplied action and returns the refined text plus a flag indicating
// whether refinement succeeded. On failure or when no model is selected it
// notifies the user and returns the original text unchanged.
func (p *DictationPlugin) refineTranscript(ctx context.Context, action dictationAction, rawText string) (string, bool) {
	model, modelOk := parseActionAIModel(ctx, p.api, action.Model)
	if !modelOk {
		p.api.Notify(ctx, "plugin_dictation_ai_no_model")
		return rawText, false
	}
	recentCtx := p.history.recentContext(util.GetSystemTimestamp())
	p.showRefiningOverlay(ctx)
	var phrases []string
	if p.dictionary != nil {
		phrases = p.dictionary.activePhrases()
	}
	refined, refineErr := p.refineWithAI(ctx, model, rawText, recentCtx, phrases)
	if refineErr != nil {
		p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("AI refine failed: %s", refineErr.Error()))
		if strings.Contains(refineErr.Error(), "timeout") {
			p.api.Notify(ctx, "plugin_dictation_ai_timeout")
		} else {
			p.api.Notify(ctx, "plugin_dictation_ai_failed")
		}
		return rawText, false
	}
	if strings.TrimSpace(refined) == "" {
		return rawText, false
	}
	return strings.TrimSpace(refined), true
}

// parseActionAIModel parses the JSON-encoded common.Model stored in an action.
func parseActionAIModel(ctx context.Context, api plugin.API, raw string) (common.Model, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return common.Model{}, false
	}
	var model common.Model
	if err := json.Unmarshal([]byte(raw), &model); err != nil {
		api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to parse dictation action AI model: %s", err.Error()))
		return common.Model{}, false
	}
	if model.Name == "" || model.Provider == "" {
		return common.Model{}, false
	}
	return model, true
}

func (p *DictationPlugin) executeActionOutput(ctx context.Context, action dictationAction, outputText string, rawText string, inputContext dictationActionInputContext, _ bool) error {
	switch action.Output {
	case dictationActionOutputOverlay:
		p.showActionResultOverlay(ctx, outputText)
		return nil
	case dictationActionOutputChat:
		return p.openActionChat(ctx, action, rawText, inputContext)
	default:
		if err := keyboard.SimulateType(outputText); err != nil {
			return fmt.Errorf("%s: %s", i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_type_failed"), err.Error())
		}
		return nil
	}
}

func (p *DictationPlugin) showActionResultOverlay(ctx context.Context, text string) {
	window := buildDictationTextOverlayWindow()
	window.ID = fmt.Sprintf("%s-%s", dictationOverlayResultName, uuid.NewString())
	window.MinWidth = 260
	window.MaxWidth = 720
	window.MaxHeight = 600
	window.Movable = true
	window.CloseOnEscape = true

	textoverlay.Show(textoverlay.Options{
		Window:                   window,
		Closable:                 true,
		Message:                  text,
		FontSize:                 14,
		FollowScroll:             true,
		ShowCopyButton:           true,
		CopyButtonTooltip:        i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_action_copy"),
		CopyButtonSuccessTooltip: i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_action_copied"),
		OnClick: func() bool {
			if err := clipboard.WriteText(text); err != nil {
				p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to copy dictation action overlay result: %s", err.Error()))
				return false
			}
			return true
		},
	})
}

func (p *DictationPlugin) openActionChat(ctx context.Context, action dictationAction, dictationText string, inputContext dictationActionInputContext) error {
	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		return errors.New(i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_action_chat_unavailable"))
	}

	model, modelOk := parseActionAIModel(ctx, p.api, action.Model)
	if !modelOk {
		model = chater.GetDefaultModel(ctx)
	}
	if model.Name == "" || model.Provider == "" {
		return errors.New(i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_action_ai_no_model"))
	}

	message := renderDictationActionPrompt(action, dictationText, inputContext)
	if strings.TrimSpace(message) == "" {
		message = strings.TrimSpace(dictationText)
	}
	if message == "" {
		return errors.New(i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_action_ai_empty"))
	}

	now := util.GetSystemTimestamp()
	chatID := uuid.NewString()
	chatData := common.AIChatData{
		Id:    chatID,
		Title: truncateHistoryTitle(dictationText),
		Model: model,
		Conversations: []common.Conversation{
			{
				Id:        uuid.NewString(),
				Role:      common.ConversationRoleUser,
				Text:      message,
				Timestamp: now,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	chater.Chat(ctx, chatData, 0)
	p.api.ChangeQuery(ctx, common.PlainQuery{
		QueryType:   plugin.QueryTypeInput,
		QueryText:   "chat " + message,
		ContextData: common.ContextData{"ai_chat_active_id": chatID},
	})
	p.api.ShowApp(ctx)
	return nil
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
		Movable:          true,
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
	}

	textoverlay.Show(opts)
}

// showActionProcessingOverlay switches the overlay into a loading state while
// a custom action's AI prompt is being processed by the selected model.
func (p *DictationPlugin) showActionProcessingOverlay(ctx context.Context) {
	window := buildDictationTextOverlayWindow()
	window.PreservePosition = true
	window.MinWidth = 200
	window.MaxWidth = 600
	window.OnClose = func() {
		p.api.Log(util.NewTraceContext(), plugin.LogLevelInfo, "dictation overlay closed during AI action processing")
	}
	opts := textoverlay.Options{
		Window:   window,
		Closable: true,
		Message:  i18n.GetI18nManager().TranslateWox(ctx, "plugin_dictation_action_ai_processing"),
		Loading:  true,
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
//
// phrases is the user's dictionary: words/phrases the AI should recognize and
// spell correctly in the refined output.
func (p *DictationPlugin) refineWithAI(ctx context.Context, model common.Model, rawText string, recentContext []string, phrases []string) (string, error) {
	refineCtx, cancel := context.WithTimeout(ctx, aiRefineTimeout)
	defer cancel()

	systemPrompt := strings.Join([]string{
		"You are a transcription editor. Rewrite the user's dictated text into fluent, coherent, easy-to-understand sentences while preserving the original meaning and language.",
		"Remove filler words (um, uh, like, you know), fix disfluencies, false starts, repeated words, and sentence fragments.",
		"Choose punctuation based on grammar and meaning, not on speech pauses. Merge fragments that belong to the same sentence, and remove punctuation that splits a natural phrase or clause.",
		"Do not add new facts, commands, explanations, quotes, or extra formatting. Output only the refined text.",
	}, " ")

	var userPrompt string
	if len(recentContext) > 0 || len(phrases) > 0 {
		var ctxBuf strings.Builder
		if len(phrases) > 0 {
			ctxBuf.WriteString("The user wants these words/phrases to be recognized and spelled correctly: ")
			ctxBuf.WriteString(strings.Join(phrases, ", "))
			ctxBuf.WriteString("\n\n")
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

// runPromptWithAI sends a user-authored dictation action prompt to the selected
// model and returns the final streamed answer.
func (p *DictationPlugin) runPromptWithAI(ctx context.Context, model common.Model, prompt string, timeout time.Duration) (string, error) {
	aiCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan struct {
		text string
		err  error
	}, 1)

	var accumulated string
	err := p.api.AIChatStream(aiCtx, model, []common.Conversation{
		{
			Role: common.ConversationRoleUser,
			Text: prompt,
		},
	}, common.ChatOptions{
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
	case <-aiCtx.Done():
		return "", fmt.Errorf("AI action timeout")
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
		Movable:          true,
		TakeFocus:        true,
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
	})
}

// closeDictationOverlay removes the recording overlay.
func (p *DictationPlugin) closeDictationOverlay() {
	p.setVoiceOverlayVisible(false)
	dictationoverlay.Close(dictationOverlayName)
}

// releaseRuntime stops any active dictation session and closes cached speech
// resources when the plugin is disabled, unloaded, or uninstalled.
func (p *DictationPlugin) releaseRuntime(ctx context.Context) {
	p.runtimeMu.Lock()
	defer p.runtimeMu.Unlock()

	p.reregisterActionHotkeys(ctx, nil)
	p.stopVolumeDucking(ctx)
	p.closeDictationOverlay()

	p.sessionMu.Lock()
	session := p.session
	p.session = nil
	p.isRecording = false
	p.isStarting = false
	p.pendingStop = false
	p.activeAction = dictationAction{}
	p.activeInputContext = dictationActionInputContext{}
	p.sessionMu.Unlock()

	if session != nil {
		if _, err := session.Stop(); err != nil {
			p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to stop dictation session during unload: %s", err.Error()))
		}
	}

	if p.recognizerPool != nil {
		p.recognizerPool.Close()
		p.recognizerPool = nil
	}
	if p.audioCapturePool != nil {
		p.audioCapturePool.Close()
		p.audioCapturePool = nil
	}
	if p.vadPool != nil {
		p.vadPool.Close()
		p.vadPool = nil
	}
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
	if p.isStarting && session == nil {
		p.pendingStop = true
		p.sessionMu.Unlock()
		p.playSoundIfEnabled(ctx, soundStop)
		return
	}
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

// startVolumeDucking pauses media only when it is already playing, then records
// whether this session needs to restore playback later.
func (p *DictationPlugin) startVolumeDucking(ctx context.Context) {
	p.setMediaPausedForDictation(false)

	enabled := parseBoolSetting(p.api.GetSetting(ctx, settingKeyDuckVolume))
	p.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("dictation: startVolumeDucking, enabled=%t", enabled))
	if !enabled {
		return
	}
	result, err := p.api.InvokePluginCommand(ctx, plugin.PluginCommandRequest{
		PluginId: mediaplayer.PluginID,
		Command:  mediaplayer.PluginCommandPauseIfPlaying,
	})
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to pause media: %s", err.Error()))
		return
	}

	switch result.Message {
	case mediaplayer.PluginCommandResultPaused:
		p.setMediaPausedForDictation(true)
		p.api.Log(ctx, plugin.LogLevelInfo, "dictation: media paused via plugin command")
	case mediaplayer.PluginCommandResultNotPlaying, mediaplayer.PluginCommandResultNoActiveMedia:
		p.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("dictation: media pause skipped: %s", result.Message))
	default:
		p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to pause media: %s", result.Message))
	}
}

// stopVolumeDucking resumes media playback only when this dictation session
// previously paused it.
func (p *DictationPlugin) stopVolumeDucking(ctx context.Context) {
	if !p.consumeMediaPausedForDictation() {
		return
	}
	result, err := p.api.InvokePluginCommand(ctx, plugin.PluginCommandRequest{
		PluginId: mediaplayer.PluginID,
		Command:  mediaplayer.PluginCommandPlay,
	})
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to resume media: %s", err.Error()))
		return
	}
	if result.Message != "" {
		p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to resume media: %s", result.Message))
		return
	}

	p.api.Log(ctx, plugin.LogLevelInfo, "dictation: media resumed via plugin command")
}

func (p *DictationPlugin) setMediaPausedForDictation(paused bool) {
	p.sessionMu.Lock()
	p.mediaPausedForDictation = paused
	p.sessionMu.Unlock()
}

// consumeMediaPausedForDictation returns the recorded media pause state once
// so duplicate stop/cancel paths do not resume playback multiple times.
func (p *DictationPlugin) consumeMediaPausedForDictation() bool {
	p.sessionMu.Lock()
	defer p.sessionMu.Unlock()

	paused := p.mediaPausedForDictation
	p.mediaPausedForDictation = false
	return paused
}

// playSoundIfEnabled plays an embedded audio clip when the playSound setting
// is on. Errors are logged but never propagated so they can't disrupt
// recording or typing.
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
