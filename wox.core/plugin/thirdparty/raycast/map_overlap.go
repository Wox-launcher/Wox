package raycast

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"

	"wox/common"
	"wox/plugin"
	"wox/setting"
)

type ignoredClipboardApp struct {
	Name     string
	Identity string
	Path     string
}

const (
	raycastDictationExtensionID  = "e:r:dictation"
	raycastEmojiExtensionID      = "e:r:emoji-picker"
	raycastScreenshotExtensionID = "e:r:screenshots"
	raycastApplicationsID        = "e:r:applications"
	raycastAIExtensionID         = "e:r:ai"
)

// mapOverlapSettings maps Raycast preferences that already exist as Wox settings.
// Theme, window position, and density use Wox's built-in values. Lists are merged
// with the current snapshot so an import adds rows instead of replacing them.
func mapOverlapSettings(payload Payload, current woxSnapshot) []mappedSetting {
	var rows []mappedSetting
	rows = append(rows, mapAppearance(payload.Settings.General, current)...)
	rows = append(rows, mapWindowPosition(payload.Settings.General, current)...)
	rows = append(rows, mapWindowDensity(payload.Settings.General, current)...)
	rows = append(rows, mapAppDirectories(payload, current)...)
	rows = append(rows, mapMCPServers(payload, current)...)
	rows = append(rows, mapDictationSettings(payload, current)...)
	rows = append(rows, mapSkillDirectories(payload, current)...)
	rows = append(rows, mapEmojiSettings(payload, current)...)
	rows = append(rows, mapScreenshotSettings(payload, current)...)
	return rows
}

func mapAppearance(general map[string]json.RawMessage, current woxSnapshot) []mappedSetting {
	raw, ok := stringSetting(general, "appearance")
	if !ok {
		return nil
	}
	themeID, detailKey, ok := appearanceTheme(raw)
	if !ok {
		return nil
	}
	row := mappedSetting{id: "setting:theme", titleKey: "theme", key: "ThemeId", value: themeID, detailKey: detailKey, selectable: true}
	if current.ThemeKnown && current.ThemeID == themeID {
		row.selectable = false
		row.status = statusExists
	}
	return []mappedSetting{row}
}

// appearanceTheme imports explicit colors only; automatic appearance leaves the Wox theme unchanged.
func appearanceTheme(raw string) (themeID, detailKey string, ok bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "dark":
		return setting.DefaultThemeId, "theme_dark", true
	case "light":
		return setting.DefaultLightThemeId, "theme_light", true
	default:
		return "", "", false
	}
}

func mapWindowPosition(general map[string]json.RawMessage, current woxSnapshot) []mappedSetting {
	raw, ok := stringSetting(general, "windowPresentationMode")
	if !ok {
		return nil
	}
	value, detailKey, ok := windowPosition(raw)
	if !ok {
		return nil
	}
	row := mappedSetting{id: "setting:window_position", titleKey: "window_position", key: "ShowPosition", value: value, detailKey: detailKey, selectable: true}
	if current.ShowPositionKnown && current.ShowPosition == value {
		row.selectable = false
		row.status = statusExists
	}
	return []mappedSetting{row}
}

func windowPosition(raw string) (value, detailKey string, ok bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "mouse-screen", "mouse_screen":
		return string(setting.PositionTypeMouseScreen), "position_mouse_screen", true
	case "active-screen", "active_screen":
		return string(setting.PositionTypeActiveScreen), "position_active_screen", true
	case "last-location", "last_location", "last-position":
		return string(setting.PositionTypeLastLocation), "position_last_location", true
	default:
		return "", "", false
	}
}

func mapWindowDensity(general map[string]json.RawMessage, current woxSnapshot) []mappedSetting {
	raw, ok := stringSetting(general, "windowMode")
	if !ok {
		return nil
	}
	value, detailKey, ok := windowDensity(raw)
	if !ok {
		return nil
	}
	row := mappedSetting{id: "setting:ui_density", titleKey: "ui_density", key: "UiDensity", value: value, detailKey: detailKey, selectable: true}
	if current.UIDensityKnown && current.UIDensity == value {
		row.selectable = false
		row.status = statusExists
	}
	return []mappedSetting{row}
}

func windowDensity(raw string) (value, detailKey string, ok bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "compact":
		return string(setting.UiDensityCompact), "density_compact", true
	case "comfortable":
		return string(setting.UiDensityComfortable), "density_comfortable", true
	case "default", "regular", "normal":
		return string(setting.UiDensityNormal), "density_normal", true
	default:
		return "", "", false
	}
}

func mapAppDirectories(payload Payload, current woxSnapshot) []mappedSetting {
	meta, ok := extensionMeta(payload, raycastApplicationsID)
	if !ok {
		return nil
	}
	var scopes []string
	if json.Unmarshal(meta["userSearchScopes"], &scopes) != nil {
		return nil
	}
	currentPaths := append([]string(nil), current.AppDirectories...)
	var added []string
	for _, scope := range scopes {
		path := cleanImportedPath(scope)
		if path == "" || !filepath.IsAbs(path) || containsImportedPath(currentPaths, path) || containsImportedPath(added, path) {
			continue
		}
		added = append(added, path)
	}
	if len(added) == 0 {
		return nil
	}
	merged := mergeImportedPaths(currentPaths, added)
	raw, err := json.Marshal(pathTable(merged))
	if err != nil {
		return nil
	}
	return []mappedSetting{{
		id: "setting:app_directories", titleKey: "app_directory", key: "AppDirectories", value: string(raw),
		detail: strings.Join(added, ", "), pluginID: common.AppPluginID, selectable: true,
	}}
}

func mapMCPServers(payload Payload, current woxSnapshot) []mappedSetting {
	var incoming []common.AIChatMCPServerConfig
	var titles []string
	for _, server := range payload.MCPServers.Servers {
		mapped, ok := mcpServer(server)
		if !ok || mcpExists(current.MCPServers, mapped) {
			continue
		}
		incoming = append(incoming, mapped)
		titles = append(titles, mapped.Name)
	}
	if len(incoming) == 0 {
		return nil
	}
	merged := append(append([]common.AIChatMCPServerConfig{}, current.MCPServers...), incoming...)
	raw, err := json.Marshal(merged)
	if err != nil {
		return nil
	}
	return []mappedSetting{{
		id: "setting:mcp_servers", titleKey: "mcp_server", key: "AIMCPServers", value: string(raw),
		detail: strings.Join(titles, ", "), selectable: true,
	}}
}

func mcpServer(server MCPServer) (common.AIChatMCPServerConfig, bool) {
	switch strings.ToLower(strings.TrimSpace(server.Type)) {
	case "stdio", "":
		command := strings.TrimSpace(server.Command)
		if command == "" {
			return common.AIChatMCPServerConfig{}, false
		}
		name := strings.TrimSpace(server.Title)
		if name == "" {
			name = command
		}
		return common.AIChatMCPServerConfig{Name: name, Type: common.AIChatMCPServerTypeSTDIO, Command: command, Args: compactStrings(server.Arguments)}, true
	case "http", "sse", "streamable-http":
		url := strings.TrimSpace(server.URL)
		if url == "" {
			return common.AIChatMCPServerConfig{}, false
		}
		name := strings.TrimSpace(server.Title)
		if name == "" {
			name = url
		}
		return common.AIChatMCPServerConfig{Name: name, Type: common.AIChatMCPServerTypeStreamableHTTP, Url: url}, true
	default:
		return common.AIChatMCPServerConfig{}, false
	}
}

func mcpExists(current []common.AIChatMCPServerConfig, incoming common.AIChatMCPServerConfig) bool {
	for _, server := range current {
		if strings.EqualFold(strings.TrimSpace(server.Name), incoming.Name) {
			return true
		}
		if incoming.Type == common.AIChatMCPServerTypeSTDIO && strings.EqualFold(server.Command, incoming.Command) && strings.Join(server.Args, "\n") == strings.Join(incoming.Args, "\n") {
			return true
		}
	}
	return false
}

func mapDictationSettings(payload Payload, current woxSnapshot) []mappedSetting {
	meta, ok := extensionMeta(payload, raycastDictationExtensionID)
	if !ok {
		return nil
	}
	var rows []mappedSetting
	if raw, exists := meta["sounds"]; exists {
		var sounds bool
		if json.Unmarshal(raw, &sounds) == nil {
			row := boolMappedSetting("setting:dictation_sound", "dictation_sound", "playSound", sounds, current.DictationSoundKnown, current.DictationSound)
			row.pluginID = common.DictationPluginID
			rows = append(rows, row)
		}
	}
	if raw, exists := meta["muteWhileRecording"]; exists {
		var mute bool
		if json.Unmarshal(raw, &mute) == nil {
			row := boolMappedSetting("setting:dictation_pause_media", "dictation_pause_media", "duckVolume", mute, current.DictationDuckKnown, current.DictationDuck)
			row.pluginID = common.DictationPluginID
			rows = append(rows, row)
		}
	}
	var phrases []string
	if json.Unmarshal(meta["customVocabulary"], &phrases) == nil {
		if row, ok := dictationVocabulary(phrases, current); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func dictationVocabulary(phrases []string, current woxSnapshot) (mappedSetting, bool) {
	var added []string
	seen := map[string]struct{}{}
	for _, phrase := range current.DictationPhrases {
		seen[normalizePhrase(phrase)] = struct{}{}
	}
	for _, phrase := range phrases {
		phrase = strings.TrimSpace(phrase)
		key := normalizePhrase(phrase)
		if phrase == "" || key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		added = append(added, phrase)
	}
	if len(added) == 0 {
		return mappedSetting{}, false
	}
	type entry struct {
		ID     string `json:"id"`
		Phrase string `json:"phrase"`
	}
	merged := make([]entry, 0, len(current.DictationPhrases)+len(added))
	for _, phrase := range current.DictationPhrases {
		phrase = strings.TrimSpace(phrase)
		if phrase == "" {
			continue
		}
		merged = append(merged, entry{ID: "existing:" + normalizePhrase(phrase), Phrase: phrase})
	}
	for _, phrase := range added {
		merged = append(merged, entry{ID: "raycast:" + normalizePhrase(phrase), Phrase: phrase})
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return mappedSetting{}, false
	}
	return mappedSetting{
		id: "setting:dictation_vocabulary", titleKey: "dictation_vocabulary", key: "dictionary", value: string(raw),
		detailKey: "dictation_phrases", detail: strconv.Itoa(len(added)), pluginID: common.DictationPluginID, selectable: true,
	}, true
}

// mapSkillDirectories adds Raycast skill folders as local AI skill roots.
// Wox discovers SKILL.md bundles by walking each stored path.
func mapSkillDirectories(payload Payload, current woxSnapshot) []mappedSetting {
	meta, ok := extensionMeta(payload, raycastAIExtensionID)
	if !ok {
		return nil
	}
	var directories []string
	if json.Unmarshal(meta["skillDirectories"], &directories) != nil {
		return nil
	}
	var added []string
	for _, directory := range directories {
		path := cleanImportedPath(directory)
		if path == "" || !filepath.IsAbs(path) || skillDirectoryExists(current.Skills, path) || containsImportedPath(added, path) {
			continue
		}
		added = append(added, path)
	}
	if len(added) == 0 {
		return nil
	}
	merged := append([]common.Skill{}, current.Skills...)
	for _, path := range added {
		merged = append(merged, common.Skill{Path: path, Enabled: true, Source: "local", SourceName: "Local"})
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return nil
	}
	return []mappedSetting{{
		id: "setting:ai_skills", titleKey: "ai_skill_directory", key: "AISkills", value: string(raw),
		detail: strings.Join(added, ", "), selectable: true,
	}}
}

func skillDirectoryExists(skills []common.Skill, path string) bool {
	for _, skill := range skills {
		if sameImportedPath(skill.Path, path) {
			return true
		}
	}
	return false
}

func mapEmojiSettings(payload Payload, current woxSnapshot) []mappedSetting {
	var rows []mappedSetting
	if meta, ok := extensionMeta(payload, raycastEmojiExtensionID); ok {
		if raw, exists := meta["primaryAction"]; exists {
			var action string
			if json.Unmarshal(raw, &action) == nil {
				action = strings.ToLower(strings.TrimSpace(action))
				if action == "copy" || action == "paste" {
					row := mappedSetting{id: "setting:emoji_action", titleKey: "emoji_action", key: "primary_action", value: action, detailKey: "emoji_" + action, pluginID: common.EmojiPluginID, selectable: true}
					if current.EmojiActionKnown && current.EmojiAction == action {
						row.selectable = false
						row.status = statusExists
					}
					rows = append(rows, row)
				}
			}
		}
	}
	if row, ok := emojiHistory(payload, current); ok {
		rows = append(rows, row)
	}
	if row, ok := emojiKeywords(payload, current); ok {
		rows = append(rows, row)
	}
	return rows
}

func emojiHistory(payload Payload, current woxSnapshot) (mappedSetting, bool) {
	seen := map[string]struct{}{}
	for _, emoji := range current.EmojiUsed {
		seen[emoji] = struct{}{}
	}
	type usage struct {
		Emoji    string `json:"emoji"`
		UseCount int    `json:"useCount"`
	}
	merged := make([]usage, 0, len(current.EmojiUsed))
	for _, emoji := range current.EmojiUsed {
		if emoji != "" {
			merged = append(merged, usage{Emoji: emoji, UseCount: 1})
		}
	}
	var added []string
	for _, entry := range payload.Emoji.Emojis {
		emoji := strings.TrimSpace(entry.Symbol)
		if emoji == "" {
			continue
		}
		if _, exists := seen[emoji]; exists {
			continue
		}
		seen[emoji] = struct{}{}
		added = append(added, emoji)
		merged = append(merged, usage{Emoji: emoji, UseCount: 1})
	}
	if len(added) == 0 {
		return mappedSetting{}, false
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return mappedSetting{}, false
	}
	return mappedSetting{
		id: "setting:emoji_history", titleKey: "emoji_history", key: "frequentlyUsed", value: string(raw),
		detail: strings.Join(added, " "), pluginID: common.EmojiPluginID, selectable: true,
	}, true
}

func emojiKeywords(payload Payload, current woxSnapshot) (mappedSetting, bool) {
	descriptions := map[string][]string{}
	for emoji, words := range current.EmojiKeywords {
		descriptions[emoji] = append([]string(nil), words...)
	}
	changed := false
	for _, entry := range payload.Emoji.Emojis {
		emoji := strings.TrimSpace(entry.Symbol)
		if emoji == "" {
			continue
		}
		for _, word := range entry.CustomKeywords {
			word = strings.TrimSpace(word)
			if word == "" || containsPhrase(descriptions[emoji], word) {
				continue
			}
			descriptions[emoji] = append(descriptions[emoji], word)
			changed = true
		}
	}
	if !changed {
		return mappedSetting{}, false
	}
	raw, err := json.Marshal(descriptions)
	if err != nil {
		return mappedSetting{}, false
	}
	return mappedSetting{
		id: "setting:emoji_keywords", titleKey: "emoji_keywords", key: "customDescriptions", value: string(raw),
		pluginID: common.EmojiPluginID, selectable: true,
	}, true
}

func mapScreenshotSettings(payload Payload, current woxSnapshot) []mappedSetting {
	meta, ok := extensionMeta(payload, raycastScreenshotExtensionID)
	if !ok {
		return nil
	}
	var rows []mappedSetting
	if raw, exists := meta["textRecognitionEnabled"]; exists {
		var enabled bool
		if json.Unmarshal(raw, &enabled) == nil {
			row := boolMappedSetting("setting:screenshot_ocr", "screenshot_ocr", "ocr_enabled", enabled, current.ScreenshotOCRKnown, current.ScreenshotOCR)
			row.pluginID = common.ScreenshotPluginID
			rows = append(rows, row)
		}
	}
	if raw, exists := meta["storageDuration"]; exists {
		var duration string
		if json.Unmarshal(raw, &duration) == nil {
			if days, ok := raycastHistoryDays(duration); ok && !(current.ScreenshotDaysKnown && current.ScreenshotDays == days) {
				rows = append(rows, mappedSetting{
					id: "setting:screenshot_retention", titleKey: "screenshot_retention", key: "retention_days", value: strconv.Itoa(days),
					detailKey: "clipboard_days", detail: strconv.Itoa(days), pluginID: common.ScreenshotPluginID, selectable: true,
				})
			}
		}
	}
	return rows
}

func mapIgnoredClipboardApps(payload Payload, apps []plugin.DesktopApp, current woxSnapshot) []mappedSetting {
	meta, ok := extensionMeta(payload, raycastClipboardExtensionID)
	if !ok {
		return nil
	}
	var exported []struct {
		ID string `json:"raycastAppId"`
	}
	if json.Unmarshal(meta["disabledApplications"], &exported) != nil || len(exported) == 0 {
		return nil
	}
	type appRef struct {
		Name     string `json:"Name"`
		Identity string `json:"Identity"`
		Path     string `json:"Path"`
	}
	type row struct {
		App appRef `json:"App"`
	}
	merged := make([]row, 0, len(current.IgnoredClipboardApps)+len(exported))
	seen := map[string]struct{}{}
	for _, app := range current.IgnoredClipboardApps {
		identity := strings.ToLower(strings.TrimSpace(app.Identity))
		if identity == "" {
			continue
		}
		seen[identity] = struct{}{}
		merged = append(merged, row{App: appRef{Name: app.Name, Identity: app.Identity, Path: app.Path}})
	}
	var names []string
	for _, item := range exported {
		name, identity, path, resolved := resolveIgnoredApp(item.ID, apps)
		if !resolved {
			continue
		}
		identityKey := strings.ToLower(identity)
		if _, exists := seen[identityKey]; exists {
			continue
		}
		seen[identityKey] = struct{}{}
		merged = append(merged, row{App: appRef{Name: name, Identity: identity, Path: path}})
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return nil
	}
	return []mappedSetting{{
		id: "setting:clipboard_ignored_apps", titleKey: "clipboard_ignored_apps", key: "ignored_applications", value: string(raw),
		detail: strings.Join(names, ", "), pluginID: common.ClipboardPluginID, selectable: true,
	}}
}

// resolveIgnoredApp turns a Raycast application id into a Wox ignored-app row.
// Windows executables use their file name as the process identity. Other paths
// match an installed app by name, which covers macOS .app paths on Windows.
func resolveIgnoredApp(raw string, apps []plugin.DesktopApp) (name, identity, path string, ok bool) {
	path = cleanImportedPath(raw)
	if path == "" {
		return "", "", "", false
	}
	if filepath.IsAbs(path) && isExecutablePath(path) {
		return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), strings.ToLower(filepath.Base(path)), path, true
	}
	wanted := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	for _, app := range apps {
		if !strings.EqualFold(strings.TrimSpace(app.Name), wanted) {
			continue
		}
		identityPath := app.Executable
		if strings.TrimSpace(identityPath) == "" {
			identityPath = app.Path
		}
		identity = strings.ToLower(filepath.Base(identityPath))
		if identity == "" || identity == "." {
			continue
		}
		return app.Name, identity, app.Path, true
	}
	return "", "", "", false
}

func isExecutablePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".exe", ".bat", ".cmd", ".com":
		return true
	default:
		return false
	}
}

func extensionMeta(payload Payload, id string) (map[string]json.RawMessage, bool) {
	for _, extension := range payload.Settings.InternalExtensions {
		if extension.ID != id || len(extension.Meta) == 0 {
			continue
		}
		var meta map[string]json.RawMessage
		if json.Unmarshal(extension.Meta, &meta) != nil {
			return nil, false
		}
		return meta, true
	}
	return nil, false
}

func compactStrings(values []string) []string {
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func normalizePhrase(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func containsPhrase(values []string, phrase string) bool {
	for _, value := range values {
		if normalizePhrase(value) == normalizePhrase(phrase) {
			return true
		}
	}
	return false
}
