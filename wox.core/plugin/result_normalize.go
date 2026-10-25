package plugin

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"wox/common"
	"wox/common/icons"
	"wox/i18n"
	"wox/setting"
	"wox/util"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

const (
	// Keep the published SDK marker even though the UI calls this a query pin.
	favoriteTailContextDataKey   = "system:favorite"
	favoriteTailContextDataValue = "true"
	scoreTailContextDataKey      = "system:score"
)

// queryResultPinned reports the query-pin tail attached during polish.
func queryResultPinned(result QueryResult) bool {
	for _, tail := range result.Tails {
		if tail.ContextData[favoriteTailContextDataKey] == favoriteTailContextDataValue {
			return true
		}
	}
	return false
}

// pinnedInQueryTail marks a result pinned to the current query.
// ContextData identifies this system tail, and IsSystemTail keeps it out of plugin updates.
func pinnedInQueryTail(ctx context.Context) QueryResultTail {
	return QueryResultTail{
		Type:         QueryResultTailTypeImage,
		Image:        icons.Get(icons.ActionPin),
		Tooltip:      i18n.GetI18nManager().TranslateWox(ctx, "plugin_manager_pin_in_query"),
		ContextData:  common.ContextData{favoriteTailContextDataKey: favoriteTailContextDataValue},
		IsSystemTail: true,
	}
}

func (m *Manager) appendDevScoreTail(ctx context.Context, tails []QueryResultTail, score int64) []QueryResultTail {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if !util.IsDev() || !woxSetting.ShowScoreTail.Get() {
		return tails
	}

	for _, tail := range tails {
		if tail.ContextData[scoreTailContextDataKey] == "true" {
			return tails
		}
	}

	return append(tails, QueryResultTail{
		Type:         QueryResultTailTypeText,
		Text:         fmt.Sprintf("score:%d", score),
		ContextData:  common.ContextData{scoreTailContextDataKey: "true"},
		IsSystemTail: true,
	})
}

// For external plugins (Node.js/Python), create proxy action callbacks
// These callbacks will invoke the host's action method, which will then
// call the actual cached callback in the plugin host
func (m *Manager) attachExternalActionCallbacks(pluginInstance *Instance, actions []QueryResultAction) {
	if proxyCreator, ok := pluginInstance.Plugin.(ActionProxyCreator); ok {
		for actionIndex := range actions {
			if actions[actionIndex].Type == QueryResultActionTypeExecute && actions[actionIndex].Action == nil {
				actions[actionIndex].Action = proxyCreator.CreateActionProxy(actions[actionIndex].Id)
			}
		}
	}

	if proxyCreator, ok := pluginInstance.Plugin.(FormActionProxyCreator); ok {
		for actionIndex := range actions {
			if actions[actionIndex].Type == QueryResultActionTypeForm && actions[actionIndex].OnSubmit == nil {
				actions[actionIndex].OnSubmit = proxyCreator.CreateFormActionProxy(actions[actionIndex].Id)
			}
		}
	}
}

func normalizeQueryResultDragData(dragData *QueryResultDragData) *QueryResultDragData {
	if dragData == nil || dragData.Type != QueryResultDragDataTypeFiles {
		return nil
	}

	files := make([]string, 0, len(dragData.Files))
	for _, rawPath := range dragData.Files {
		cleanPath := filepath.Clean(strings.TrimSpace(rawPath))
		if cleanPath == "." || !filepath.IsAbs(cleanPath) {
			continue
		}
		// Existence is checked when native dragging actually starts. Probing every
		// displayed result adds synchronous filesystem I/O to ordinary searches.
		files = append(files, cleanPath)
	}
	if len(files) == 0 {
		return nil
	}

	return &QueryResultDragData{
		Type:                 QueryResultDragDataTypeFiles,
		Files:                files,
		PreventHideAfterDrag: dragData.PreventHideAfterDrag,
	}
}

// appendActionEnglishAlias makes the default English translation searchable without plugin boilerplate.
func appendActionEnglishAlias(ctx context.Context, pluginInstance *Instance, name string, aliases []string) []string {
	if !strings.HasPrefix(name, "i18n:") {
		return aliases
	}
	englishName := pluginInstance.Metadata.translateEn(ctx, common.I18nString(name))
	if englishName == name || lo.Contains(aliases, englishName) {
		return aliases
	}
	return append(aliases, englishName)
}

// normalizeHotkeyForPlatform converts hotkey strings to platform-specific format
// This function provides better cross-platform hotkey support by handling various
// modifier key aliases and converting them to the appropriate platform format
func normalizeHotkeyForPlatform(hotkey string) string {
	if hotkey == "" {
		return hotkey
	}

	// Convert to lowercase for case-insensitive matching
	normalized := strings.ToLower(strings.TrimSpace(hotkey))

	// Define platform-specific modifier mappings
	var modifierMappings map[string]string

	if util.IsMacOS() {
		// On macOS: convert Windows/Linux style to macOS style
		modifierMappings = map[string]string{
			"win":     "cmd",
			"windows": "cmd",
			"ctrl":    "control", // Keep ctrl as control on macOS
			"control": "control", // Keep explicit control as-is
			"alt":     "option",
		}
	} else {
		// On Windows/Linux: convert macOS style to Windows/Linux style
		modifierMappings = map[string]string{
			"cmd":     "win", // Map cmd to win key on Windows/Linux
			"command": "win",
			"option":  "alt",
			"control": "ctrl", // Keep control as ctrl
			"ctrl":    "ctrl", // Keep ctrl as-is
		}
	}

	// Split hotkey into parts and process each modifier
	parts := strings.Split(normalized, "+")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if replacement, exists := modifierMappings[part]; exists {
			parts[i] = replacement
		} else {
			parts[i] = part
		}
	}

	return strings.Join(parts, "+")
}

// convertActionIcons normalizes each distinct source once within a plugin response.
func convertActionIcons(ctx context.Context, actions []QueryResultAction, pluginInstance *Instance, cache map[common.WoxImage]common.WoxImage) int {
	convertedCount := 0
	for index := range actions {
		convertedCount += convertActionImage(&actions[index].Icon, pluginInstance, cache, ctx)
		convertedCount += convertActionImage(&actions[index].TailIcon, pluginInstance, cache, ctx)
	}
	return convertedCount
}

func convertActionImage(image *common.WoxImage, pluginInstance *Instance, cache map[common.WoxImage]common.WoxImage, ctx context.Context) int {
	if image == nil || image.IsEmpty() {
		return 0
	}
	source := *image
	if converted, found := cache[source]; found {
		*image = converted
		return 0
	}
	converted := pluginInstance.ConvertIcon(ctx, source)
	*image = converted
	if cache != nil {
		cache[source] = converted
	}
	return 1
}

// normalizeResultActionDefaults supplies identity and type before callbacks are attached.
func normalizeResultActionDefaults(actions []QueryResultAction) {
	for actionIndex := range actions {
		if actions[actionIndex].Id == "" {
			actions[actionIndex].Id = uuid.NewString()
		}
		if actions[actionIndex].Icon.IsEmpty() {
			// set default action icon if not present
			actions[actionIndex].Icon = icons.Get(icons.ActionExecute)
		}
		if actions[actionIndex].Type == "" {
			if len(actions[actionIndex].Form) > 0 || actions[actionIndex].OnSubmit != nil {
				actions[actionIndex].Type = QueryResultActionTypeForm
			} else {
				actions[actionIndex].Type = QueryResultActionTypeExecute
			}
		}
	}
}

// translateResultActions keeps names, English search aliases and form labels consistent.
func (m *Manager) translateResultActions(ctx context.Context, pluginInstance *Instance, actions []QueryResultAction) {
	// translate action names
	for actionIndex := range actions {
		actions[actionIndex].SearchAliases = appendActionEnglishAlias(ctx, pluginInstance, actions[actionIndex].Name, actions[actionIndex].SearchAliases)
		actions[actionIndex].Name = m.translatePlugin(ctx, pluginInstance, actions[actionIndex].Name)
		if actions[actionIndex].Type == QueryResultActionTypeForm {
			for definitionIndex := range actions[actionIndex].Form {
				if actions[actionIndex].Form[definitionIndex].Value != nil {
					actions[actionIndex].Form[definitionIndex].Value = actions[actionIndex].Form[definitionIndex].Value.Translate(pluginInstance.API.GetTranslation)
				}
			}
		}
	}
}

// ensureDefaultResultAction preserves explicit defaults and promotes the first action otherwise.
func ensureDefaultResultAction(actions []QueryResultAction) {
	defaultActionCount := lo.CountBy(actions, func(item QueryResultAction) bool {
		return item.IsDefault
	})
	if defaultActionCount == 0 && len(actions) > 0 {
		actions[0].IsDefault = true
		actions[0].Hotkey = "Enter"
	}
}

// normalizeResultActionHotkeys applies the platform mapping after default selection.
func normalizeResultActionHotkeys(actions []QueryResultAction) {
	for actionIndex := range actions {
		if actions[actionIndex].IsDefault && actions[actionIndex].Hotkey == "" {
			actions[actionIndex].Hotkey = "Enter"
		}
		actions[actionIndex].Hotkey = normalizeHotkeyForPlatform(actions[actionIndex].Hotkey)
	}
}

// translateResultTails preserves supplied IDs and translates text and tooltips.
func (m *Manager) translateResultTails(ctx context.Context, pluginInstance *Instance, tails []QueryResultTail) {
	// translate tail text and assign IDs if not present
	for i := range tails {
		if tails[i].Id == "" {
			tails[i].Id = uuid.NewString()
		}
		if tails[i].Type == QueryResultTailTypeText {
			tails[i].Text = m.translatePlugin(ctx, pluginInstance, tails[i].Text)
		}
		if tails[i].Tooltip != "" {
			tails[i].Tooltip = m.translatePlugin(ctx, pluginInstance, tails[i].Tooltip)
		}
	}
}

// convertResultTailImages returns the number of image tails for polish diagnostics.
func convertResultTailImages(ctx context.Context, pluginInstance *Instance, tails []QueryResultTail) int {
	count := 0
	for i := range tails {
		if tails[i].Type == QueryResultTailTypeImage {
			count++
			tails[i].Image = pluginInstance.ConvertIcon(ctx, tails[i].Image)
		}
	}
	return count
}

// appendPinnedResultTail uses query-specific pins; MRU ranking never inherits them.
func (m *Manager) appendPinnedResultTail(ctx context.Context, pluginInstance *Instance, query Query, result QueryResult, tails []QueryResultTail) []QueryResultTail {
	if query.Env.IsMRU || !setting.GetSettingManager().IsPinedResult(ctx, resultIdentityHash(pluginInstance.Metadata.Id, result), query.RawQuery) {
		return tails
	}
	for _, tail := range tails {
		if tail.ContextData[favoriteTailContextDataKey] == favoriteTailContextDataValue {
			return tails
		}
	}
	return append(tails, pinnedInQueryTail(ctx))
}
