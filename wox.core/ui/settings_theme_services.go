package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"wox/ai"
	"wox/plugin"
	"wox/resource"

	"wox/common"
	"wox/setting"
	"wox/ui/contract"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

// CurrentTheme returns the platform-resolved active theme.
func (s *CoreServices) CurrentTheme(ctx context.Context, sessionID string) (common.Theme, error) {
	return GetUIManager().GetCurrentTheme(uiServiceContext(ctx, sessionID)), nil
}

// Themes returns one resolved theme catalog for the current platform.
func (s *CoreServices) Themes(ctx context.Context, sessionID string, catalog contract.ThemeCatalog) ([]contract.ThemeCatalogItem, error) {
	ctx = uiServiceContext(ctx, sessionID)
	installedIDs := make(map[string]bool)
	for _, theme := range GetUIManager().GetAllThemes(ctx) {
		installedIDs[theme.ThemeId] = true
	}
	var themes []common.Theme
	switch catalog {
	case contract.ThemeCatalogStore:
		manifests := GetStoreManager().GetThemeManifests()
		result := make([]contract.ThemeCatalogItem, 0, len(manifests))
		for _, manifest := range manifests {
			result = append(result, contract.ThemeCatalogItem{
				Theme:        common.Theme{ThemeId: manifest.Id, IsInstalled: installedIDs[manifest.Id]},
				Manifest:     manifest,
				IsUpgradable: GetUIManager().IsThemeUpgradable(manifest.Id, manifest.Version),
			})
		}
		return result, nil
	case contract.ThemeCatalogInstalled:
		themes = GetUIManager().GetAllThemes(ctx)
	default:
		return nil, fmt.Errorf("unsupported theme catalog %q", catalog)
	}

	result := make([]contract.ThemeCatalogItem, len(themes))
	for index, theme := range themes {
		theme.IsInstalled = installedIDs[theme.ThemeId]
		theme.IsSystem = GetUIManager().IsSystemTheme(theme.ThemeId)
		result[index] = contract.ThemeCatalogItem{
			// The installed catalog renders per-theme previews, so packaged assets are loaded here.
			Theme:        GetUIManager().ThemeWithAssets(ctx, theme),
			IsUpgradable: GetUIManager().IsThemeUpgradable(theme.ThemeId, theme.Version),
		}
	}
	return result, nil
}

// OperateTheme performs one theme lifecycle action against core-owned managers.
func (s *CoreServices) OperateTheme(ctx context.Context, sessionID string, themeID string, operation contract.ThemeOperation) error {
	ctx = uiServiceContext(ctx, sessionID)
	switch operation {
	case contract.ThemeOperationInstall:
		manifest, exists := lo.Find(GetStoreManager().GetThemeManifests(), func(item common.StoreThemeManifest) bool {
			return item.Id == themeID
		})
		if !exists {
			return fmt.Errorf("theme %q not found in the store", themeID)
		}
		if err := GetStoreManager().InstallManifest(ctx, manifest); err != nil {
			return fmt.Errorf("install theme %q: %w", themeID, err)
		}
		return nil
	case contract.ThemeOperationUninstall:
		theme, exists := lo.Find(GetUIManager().GetAllThemes(ctx), func(item common.Theme) bool {
			return item.ThemeId == themeID
		})
		if !exists {
			return fmt.Errorf("theme %q is not installed", themeID)
		}
		currentThemeID := setting.GetSettingManager().GetWoxSetting(ctx).ThemeId.Get()
		if err := GetStoreManager().Uninstall(ctx, theme); err != nil {
			return fmt.Errorf("uninstall theme %q: %w", themeID, err)
		}
		if currentThemeID == themeID {
			GetUIManager().ChangeToDefaultTheme(ctx)
			return nil
		}
		if err := resetAutoThemeEndpoints(ctx, themeID); err != nil {
			return err
		}
		if current := GetUIManager().GetThemeById(currentThemeID); current.IsAutoAppearance {
			GetUIManager().ApplyCurrentTheme(ctx)
		}
		return nil
	case contract.ThemeOperationApply:
		theme, exists := lo.Find(GetUIManager().GetAllThemes(ctx), func(item common.Theme) bool {
			return item.ThemeId == themeID
		})
		if !exists {
			return fmt.Errorf("theme %q is not installed", themeID)
		}
		GetUIManager().ChangeTheme(ctx, theme)
		return nil
	default:
		return fmt.Errorf("unsupported theme operation %q", operation)
	}
}

// SaveTheme validates and installs one user-edited theme draft.
func (s *CoreServices) SaveTheme(ctx context.Context, sessionID string, name string, theme common.Theme, overwrite bool) (common.Theme, error) {
	ctx = uiServiceContext(ctx, sessionID)
	name = strings.TrimSpace(name)
	if name == "" {
		return common.Theme{}, fmt.Errorf("theme name is empty")
	}
	if theme.AppBackgroundColor == "" {
		return common.Theme{}, fmt.Errorf("theme data is empty")
	}
	if len(theme.AssetFiles) == 0 {
		theme.AssetFiles = GetUIManager().ThemeWithAssets(ctx, GetUIManager().GetThemeById(theme.ThemeId)).AssetFiles
	}
	if overwrite {
		if strings.TrimSpace(theme.ThemeId) == "" {
			return common.Theme{}, fmt.Errorf("theme id is empty")
		}
		if GetUIManager().IsSystemTheme(theme.ThemeId) {
			return common.Theme{}, fmt.Errorf("can't overwrite system theme")
		}
	} else {
		theme.ThemeId = uuid.NewString()
	}
	theme.ThemeName = name
	if strings.TrimSpace(theme.ThemeAuthor) == "" {
		theme.ThemeAuthor = "Wox Launcher"
	}
	if strings.TrimSpace(theme.ThemeUrl) == "" {
		theme.ThemeUrl = "https://github.com/Wox-launcher/Wox"
	}
	if strings.TrimSpace(theme.Version) == "" {
		theme.Version = "1.0.0"
	}
	theme.IsSystem = false
	theme.IsInstalled = true
	theme.IsAutoAppearance = false
	theme.DarkThemeId = ""
	theme.LightThemeId = ""
	if !theme.HasAuthoredStyles() {
		theme.Windows = nil
		theme.MacOS = nil
		theme.Linux = nil
	}
	if err := GetStoreManager().Install(ctx, theme); err != nil {
		return common.Theme{}, fmt.Errorf("save theme: %w", err)
	}
	return theme, nil
}

// SaveAutoTheme persists one user Auto theme document without applying it.
func (s *CoreServices) SaveAutoTheme(ctx context.Context, sessionID string, name string, lightThemeID string, darkThemeID string, themeID string, overwrite bool) (common.Theme, error) {
	ctx = uiServiceContext(ctx, sessionID)
	installed := installedThemeIndex(ctx)
	if err := validateAutoThemeDraft(name, lightThemeID, darkThemeID, installed); err != nil {
		return common.Theme{}, err
	}
	template := autoThemeTemplate(ctx)
	if overwrite {
		if strings.TrimSpace(themeID) == "" {
			return common.Theme{}, fmt.Errorf("theme id is empty")
		}
		if GetUIManager().IsSystemTheme(themeID) {
			return common.Theme{}, fmt.Errorf("can't overwrite system theme")
		}
		existing := GetUIManager().GetThemeById(themeID)
		if existing.ThemeId == "" {
			return common.Theme{}, fmt.Errorf("theme %q is not installed", themeID)
		}
		if !existing.IsAutoAppearance {
			return common.Theme{}, fmt.Errorf("theme %q is not an auto theme", themeID)
		}
		template = existing
	} else {
		themeID = uuid.NewString()
	}
	theme := composeUserAutoTheme(themeID, name, template, lightThemeID, darkThemeID)
	if err := GetStoreManager().PersistInstalled(ctx, theme); err != nil {
		return common.Theme{}, fmt.Errorf("save auto theme: %w", err)
	}
	return theme, nil
}

func installedThemeIndex(ctx context.Context) map[string]common.Theme {
	themes := map[string]common.Theme{}
	for _, theme := range GetUIManager().GetAllThemes(ctx) {
		themes[theme.ThemeId] = theme
	}
	return themes
}

func autoThemeTemplate(ctx context.Context) common.Theme {
	if theme := GetUIManager().GetThemeById(setting.DefaultAutoThemeId); theme.ThemeId != "" {
		return theme
	}
	return common.Theme{SchemaVersion: 2, MinWoxVersion: "2.4.3", AppBackgroundColor: "#232933BF"}
}

// validateAutoThemeDraft checks the name and light/dark endpoints for one Auto theme.
func validateAutoThemeDraft(name, lightThemeID, darkThemeID string, themes map[string]common.Theme) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("theme name is empty")
	}
	if err := validateAutoThemeEndpoint(lightThemeID, themes); err != nil {
		return err
	}
	return validateAutoThemeEndpoint(darkThemeID, themes)
}

func validateAutoThemeEndpoint(themeID string, themes map[string]common.Theme) error {
	themeID = strings.TrimSpace(themeID)
	if themeID == "" {
		return fmt.Errorf("auto theme endpoint is empty")
	}
	theme, ok := themes[themeID]
	if !ok {
		return fmt.Errorf("theme %q is not installed", themeID)
	}
	if theme.IsAutoAppearance {
		return fmt.Errorf("auto themes cannot be used as light or dark variants")
	}
	return nil
}

// composeUserAutoTheme builds a persistable Auto document from a system template.
func composeUserAutoTheme(id, name string, template common.Theme, lightThemeID, darkThemeID string) common.Theme {
	theme := template
	theme.ThemeId = id
	theme.ThemeName = strings.TrimSpace(name)
	theme.IsSystem = false
	theme.IsInstalled = true
	theme.IsAutoAppearance = true
	theme.LightThemeId = lightThemeID
	theme.DarkThemeId = darkThemeID
	if strings.TrimSpace(theme.ThemeAuthor) == "" {
		theme.ThemeAuthor = "Wox Launcher"
	}
	if strings.TrimSpace(theme.ThemeUrl) == "" {
		theme.ThemeUrl = "https://github.com/Wox-launcher/Wox"
	}
	if strings.TrimSpace(theme.Version) == "" {
		theme.Version = "1.0.0"
	}
	if theme.SchemaVersion == 0 {
		theme.SchemaVersion = 2
	}
	theme.Windows = nil
	theme.MacOS = nil
	theme.Linux = nil
	return theme
}

func resetAutoThemeEndpoints(ctx context.Context, removedThemeID string) error {
	for _, theme := range GetUIManager().GetAllThemes(ctx) {
		if !theme.IsAutoAppearance || GetUIManager().IsSystemTheme(theme.ThemeId) {
			continue
		}
		stored := GetUIManager().GetThemeById(theme.ThemeId)
		lightID, darkID, changed := replaceRemovedAutoEndpoints(removedThemeID, stored.LightThemeId, stored.DarkThemeId)
		if !changed {
			continue
		}
		stored.LightThemeId = lightID
		stored.DarkThemeId = darkID
		if err := GetStoreManager().PersistInstalled(ctx, stored); err != nil {
			return fmt.Errorf("update auto theme %q: %w", stored.ThemeId, err)
		}
	}
	return nil
}

func replaceRemovedAutoEndpoints(removedID, lightID, darkID string) (string, string, bool) {
	changed := false
	if lightID == removedID {
		lightID = setting.DefaultLightThemeId
		changed = true
	}
	if darkID == removedID {
		darkID = setting.DefaultDarkThemeId
		changed = true
	}
	return lightID, darkID, changed
}

// SuggestThemeEdits runs a text-only request through the configured AI provider without persisting themes.
func (s *CoreServices) SuggestThemeEdits(ctx context.Context, sessionID string, model common.Model, conversations []common.Conversation, onProgress common.ChatStreamFunc) (string, error) {
	ctx = uiServiceContext(ctx, sessionID)
	provider, err := plugin.GetPluginManager().GetAIProvider(ctx, model.Provider, model.ProviderAlias)
	if err != nil {
		return "", err
	}
	// Load the shipped skill directly: the draft editor has no filesystem tools and must not depend on skill discovery or user configuration.
	skill, err := resource.AIFS.ReadFile("ai/skills/wox-theme-creator/SKILL.md")
	if err != nil {
		return "", fmt.Errorf("load theme creator skill: %w", err)
	}
	messages := []common.Conversation{{Role: common.ConversationRoleSystem, Text: string(skill) + "\nYou are embedded in the theme editor. Apply this skill's design guidance to the supplied editable properties. Repository inspection, file creation and publishing are unavailable. The following editor instructions define the JSON patch output format."}}
	messages = append(messages, conversations...)
	stream, err := provider.ChatStream(ctx, model, messages, common.EmptyChatOptions)
	if err != nil {
		return "", err
	}
	return readThemeSuggestion(ctx, stream, onProgress)
}

// readThemeSuggestion follows the same no-content sentinel contract as AI Chat.
func readThemeSuggestion(ctx context.Context, stream ai.ChatStream, onProgress common.ChatStreamFunc) (string, error) {
	result := ""
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		chunk, err := stream.Receive(ctx)
		if errors.Is(err, ai.ChatStreamNoContentErr) {
			continue
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if chunk.Data != "" {
			result = chunk.Data
		}
		if len(result) > 128*1024 || len(chunk.Reasoning) > 128*1024 {
			return "", fmt.Errorf("theme response exceeds size limit")
		}
		if chunk.Status == common.ChatStreamStatusError {
			return "", fmt.Errorf("%s", chunk.Data)
		}
		if onProgress != nil {
			onProgress(chunk)
		}
		if chunk.Status == common.ChatStreamStatusFinished || chunk.Status == common.ChatStreamStatusStreamed {
			break
		}
	}
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("empty theme response")
	}
	return result, nil
}
