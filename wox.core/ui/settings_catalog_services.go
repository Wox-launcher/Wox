package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"wox/ai"
	"wox/common"
	"wox/plugin"
	appplugin "wox/plugin/system/app"
	"wox/setting"
	"wox/ui/contract"
)

// HotkeyAppCandidates returns platform application identities suitable for exclusion rules.
func (s *CoreServices) HotkeyAppCandidates(ctx context.Context, sessionID string) ([]contract.HotkeyApp, error) {
	apps := appplugin.GetHotkeyAppCandidates(uiServiceContext(ctx, sessionID))
	converted := make([]contract.HotkeyApp, len(apps))
	for index, app := range apps {
		converted[index] = contract.HotkeyApp{Name: app.Name, Identity: app.Identity, Path: app.Path, Icon: app.Icon}
	}
	return converted, nil
}

// IndexedApps returns applications matching the core ignore rule, including distinct shortcuts.
func (s *CoreServices) IndexedApps(ctx context.Context, sessionID string, pattern string) ([]contract.HotkeyApp, error) {
	apps := appplugin.GetIndexedApps(uiServiceContext(ctx, sessionID), pattern)
	converted := make([]contract.HotkeyApp, len(apps))
	for index, app := range apps {
		converted[index] = contract.HotkeyApp{Name: app.Name, Identity: app.Identity, Path: app.Path, Icon: app.Icon}
	}
	return converted, nil
}

// StartHotkeyRecording activates the strongest recorder supported by the current platform.
func (s *CoreServices) StartHotkeyRecording(ctx context.Context, sessionID string, purpose string, allowedKinds []string) (contract.HotkeyRecordingCapability, error) {
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		return contract.HotkeyRecordingCapability{}, errors.New("purpose is required when recording starts")
	}
	if len(allowedKinds) == 0 {
		return contract.HotkeyRecordingCapability{}, errors.New("allowedKinds is required when recording starts")
	}
	ctx = uiServiceContext(ctx, sessionID)
	logger.Info(ctx, fmt.Sprintf("received hotkey recording state from UI: isRecording=true purpose=%s allowedKinds=%v", purpose, allowedKinds))
	capability, err := GetUIManager().PostOnHotkeyRecording(ctx, true, purpose, allowedKinds)
	if err != nil {
		return contract.HotkeyRecordingCapability{}, err
	}
	return contract.HotkeyRecordingCapability{
		RawRecorderAvailable: capability.RawRecorderAvailable,
		FallbackAllowedKinds: append([]string(nil), capability.FallbackAllowedKinds...),
		UnavailableReason:    capability.UnavailableReason,
	}, nil
}

// StopHotkeyRecording releases the process-wide recorder.
func (s *CoreServices) StopHotkeyRecording(ctx context.Context, sessionID string) error {
	ctx = uiServiceContext(ctx, sessionID)
	logger.Info(ctx, "received hotkey recording state from UI: isRecording=false")
	_, err := GetUIManager().PostOnHotkeyRecording(ctx, false, "", nil)
	return err
}

// SubmitHotkeyRecordingCandidate forwards a locally parsed normal combo to the raw recorder path.
func (s *CoreServices) SubmitHotkeyRecordingCandidate(ctx context.Context, sessionID string, hotkey string) error {
	hotkey = strings.TrimSpace(hotkey)
	if hotkey == "" {
		return errors.New("hotkey is required")
	}
	return GetUIManager().PostHotkeyRecordingCandidate(uiServiceContext(ctx, sessionID), hotkey)
}

// CheckHotkeyAvailability checks Wox-owned and operating-system conflicts.
func (s *CoreServices) CheckHotkeyAvailability(ctx context.Context, sessionID string, hotkey string) (contract.HotkeyAvailability, error) {
	hotkey = strings.TrimSpace(hotkey)
	if hotkey == "" {
		return contract.HotkeyAvailability{}, errors.New("hotkey is empty")
	}
	availability := GetUIManager().CheckHotkeyAvailability(uiServiceContext(ctx, sessionID), hotkey)
	return contract.HotkeyAvailability{
		Available: availability.Available, ConflictType: availability.ConflictType, ConflictValue: availability.ConflictValue,
	}, nil
}

// AIProviders returns built-in provider metadata used by AI settings.
func (s *CoreServices) AIProviders(_ context.Context, _ string) ([]contract.AIProvider, error) {
	providers := ai.GetAllProviders()
	converted := make([]contract.AIProvider, len(providers))
	for index, provider := range providers {
		converted[index] = contract.AIProvider{Name: string(provider.Name), Icon: provider.Icon, DefaultHost: provider.DefaultHost}
	}
	return converted, nil
}

// AIModels resolves models from every configured provider.
func (s *CoreServices) AIModels(ctx context.Context, sessionID string) ([]contract.AIModel, error) {
	ctx = uiServiceContext(ctx, sessionID)
	models := getAIModels(ctx)
	converted := make([]contract.AIModel, len(models))
	for index, model := range models {
		converted[index] = contract.AIModel{Name: model.Name, Provider: string(model.Provider), ProviderAlias: model.ProviderAlias}
	}
	return converted, nil
}

// AICommandTemplates returns the translated template catalog owned by core.
func (s *CoreServices) AICommandTemplates(ctx context.Context, sessionID string) ([]contract.AICommandTemplate, error) {
	ctx = uiServiceContext(ctx, sessionID)
	templates := ai.GetStoreManager().GetCommands(ctx)
	converted := make([]contract.AICommandTemplate, len(templates))
	for index, template := range templates {
		converted[index] = contract.AICommandTemplate{
			ID: template.Id, Category: template.Category, Name: template.Name, Description: template.Description,
			Command: template.Command, Prompt: template.Prompt, ThinkingMode: template.ThinkingMode, DefaultAction: template.DefaultAction, Vision: template.Vision,
		}
	}
	return converted, nil
}

// DefaultAIModel returns the chat plugin's configured template-install model.
func (s *CoreServices) DefaultAIModel(ctx context.Context, sessionID string) (contract.AIModel, error) {
	ctx = uiServiceContext(ctx, sessionID)
	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		return contract.AIModel{}, errors.New("ai chat plugin not found")
	}
	model := chater.GetDefaultModel(ctx)
	return contract.AIModel{Name: model.Name, Provider: string(model.Provider), ProviderAlias: model.ProviderAlias}, nil
}

// AISkills returns the discovered skill catalog from the active AI chat plugin.
func (s *CoreServices) AISkills(ctx context.Context, sessionID string) ([]contract.AISkill, error) {
	ctx = uiServiceContext(ctx, sessionID)
	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		return nil, errors.New("ai chat plugin not found")
	}
	skills := chater.GetAllSkills(ctx)
	converted := make([]contract.AISkill, len(skills))
	for index, skill := range skills {
		converted[index] = contract.AISkill{
			ID: skill.Id, Name: skill.Name, Description: skill.Description, Path: skill.Path, ManifestPath: skill.ManifestPath,
			Source: skill.Source, SourceName: skill.SourceName, SourceURL: skill.SourceUrl, Error: skill.Error, Enabled: skill.Enabled,
		}
	}
	return converted, nil
}

// CloneAISkills discovers skills from one remote repository.
func (s *CoreServices) CloneAISkills(ctx context.Context, sessionID string, sourceURL string) ([]contract.AISkill, error) {
	if strings.TrimSpace(sourceURL) == "" {
		return nil, errors.New("url is required")
	}
	stubs, err := ai.DiscoverRemoteSkills(uiServiceContext(ctx, sessionID), sourceURL)
	if err != nil {
		return nil, err
	}
	skills := make([]contract.AISkill, len(stubs))
	for index, stub := range stubs {
		skills[index] = contract.AISkill{
			Name: stub.Name, Description: stub.Description, Path: stub.Path, ManifestPath: stub.ManifestPath,
			Source: "remote", SourceName: "Remote", SourceURL: stub.SourceUrl, Error: stub.Error, Enabled: true,
		}
	}
	return skills, nil
}

// getAIModels resolves configured providers while tolerating one provider failure.
func getAIModels(ctx context.Context) []common.Model {
	results := make([]common.Model, 0)
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	for _, providerSetting := range woxSetting.AIProviders.Get() {
		provider, err := ai.NewProvider(ctx, providerSetting)
		if err != nil {
			logger.Error(ctx, "failed to create AI provider "+string(providerSetting.Name)+": "+err.Error())
			continue
		}
		models, err := provider.Models(ctx)
		if err != nil {
			logger.Error(ctx, "failed to get models for provider "+string(providerSetting.Name)+": "+err.Error())
			continue
		}
		results = append(results, models...)
	}
	return results
}
