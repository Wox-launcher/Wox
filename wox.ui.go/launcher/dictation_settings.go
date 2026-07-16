package launcher

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wox-launcher/wox.ui.go/coreclient"
)

const (
	dictationPluginID                 = "a3f7b8c2-d1e4-4f6a-9b0c-7e2d1a5f8b3e"
	dictationActionsKey               = "actions"
	dictationDefaultHotkeyKey         = "defaultHotkey"
	dictationDefaultAIRefineKey       = "defaultAIRefineEnabled"
	dictationDefaultAIModelKey        = "defaultAIModel"
	dictationDefaultActionInternalKey = "__wox_go_dictation_default_action"
)

func newDictationDefaultAction() map[string]any {
	return map[string]any{
		"id": "default", "type": "default", "name": "i18n:plugin_dictation_default_action_name", "disabled": false,
		"hotkey": "", "output": "input", "model": "", "prompt": "", "aiRefineEnabled": false,
	}
}

func decodeDictationActions(raw string) []map[string]any {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "null" {
		return nil
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil
	}
	return rows
}

func dictationString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func dictationBool(value any) bool {
	if typed, ok := value.(bool); ok {
		return typed
	}
	return strings.EqualFold(dictationString(value), "true")
}

func normalizeDictationDefaultAction(action map[string]any) map[string]any {
	normalized := newDictationDefaultAction()
	for key, value := range action {
		normalized[key] = value
	}
	normalized["id"] = "default"
	normalized["type"] = "default"
	if dictationString(normalized["name"]) == "" {
		normalized["name"] = "i18n:plugin_dictation_default_action_name"
	} else {
		normalized["name"] = dictationString(normalized["name"])
	}
	normalized["disabled"] = false
	normalized["hotkey"] = dictationString(normalized["hotkey"])
	normalized["output"] = "input"
	normalized["model"] = dictationString(normalized["model"])
	normalized["prompt"] = dictationString(normalized["prompt"])
	normalized["aiRefineEnabled"] = dictationBool(normalized["aiRefineEnabled"])
	delete(normalized, "enabled")
	return normalized
}

func normalizeDictationCustomAction(action map[string]any) map[string]any {
	normalized := make(map[string]any, len(action)+2)
	for key, value := range action {
		if key != formTableRowIDKey {
			normalized[key] = value
		}
	}
	if dictationString(normalized["id"]) == "" {
		normalized["id"] = coreclient.NewID()
	} else {
		normalized["id"] = dictationString(normalized["id"])
	}
	normalized["type"] = "custom"
	normalized["name"] = dictationString(normalized["name"])
	normalized["disabled"] = dictationBool(normalized["disabled"])
	normalized["hotkey"] = dictationString(normalized["hotkey"])
	output := dictationString(normalized["output"])
	switch output {
	case "overlay", "chat":
		normalized["output"] = output
	default:
		normalized["output"] = "input"
	}
	normalized["model"] = dictationString(normalized["model"])
	normalized["prompt"] = dictationString(normalized["prompt"])
	delete(normalized, "enabled")
	delete(normalized, "aiRefineEnabled")
	return normalized
}

// applyDictationFormCompatibility presents the default action as normal fields while keeping custom rows in the table.
func applyDictationFormCompatibility(plugin pluginSettingsPlugin, values map[string]string) {
	if plugin.ID != dictationPluginID {
		return
	}
	defaultAction := newDictationDefaultAction()
	customActions := make([]map[string]any, 0)
	for _, action := range decodeDictationActions(plugin.Setting.Settings[dictationActionsKey]) {
		if dictationString(action["type"]) == "default" || dictationString(action["id"]) == "default" {
			defaultAction = normalizeDictationDefaultAction(action)
			continue
		}
		customActions = append(customActions, normalizeDictationCustomAction(action))
	}
	encodedDefault, _ := json.Marshal(defaultAction)
	encodedCustom, _ := json.Marshal(customActions)
	values[dictationDefaultActionInternalKey] = string(encodedDefault)
	values[dictationActionsKey] = string(encodedCustom)
	values[dictationDefaultHotkeyKey] = dictationString(defaultAction["hotkey"])
	values[dictationDefaultAIRefineKey] = fmt.Sprintf("%t", dictationBool(defaultAction["aiRefineEnabled"]))
	values[dictationDefaultAIModelKey] = dictationString(defaultAction["model"])
}

func preserveDictationCompatibilityValues(pluginID string, target, source map[string]string) {
	if pluginID != dictationPluginID {
		return
	}
	for _, key := range []string{dictationDefaultActionInternalKey, dictationActionsKey, dictationDefaultHotkeyKey, dictationDefaultAIRefineKey, dictationDefaultAIModelKey} {
		target[key] = source[key]
	}
}

// rewriteDictationSaveValues merges staged default fields and custom rows back into the single core-owned actions setting.
func rewriteDictationSaveValues(pluginID string, current, initial, changed map[string]string) error {
	if pluginID != dictationPluginID {
		return nil
	}
	specialKeys := []string{dictationDefaultHotkeyKey, dictationDefaultAIRefineKey, dictationDefaultAIModelKey, dictationActionsKey}
	specialChanged := false
	for _, key := range specialKeys {
		if current[key] != initial[key] {
			specialChanged = true
		}
		delete(changed, key)
	}
	if !specialChanged {
		return nil
	}
	defaultAction := newDictationDefaultAction()
	if raw := current[dictationDefaultActionInternalKey]; raw != "" {
		_ = json.Unmarshal([]byte(raw), &defaultAction)
	}
	defaultAction = normalizeDictationDefaultAction(defaultAction)
	defaultAction["hotkey"] = strings.TrimSpace(current[dictationDefaultHotkeyKey])
	defaultAction["aiRefineEnabled"] = strings.EqualFold(current[dictationDefaultAIRefineKey], "true")
	defaultAction["model"] = strings.TrimSpace(current[dictationDefaultAIModelKey])
	customRows := decodeDictationActions(current[dictationActionsKey])
	actions := make([]map[string]any, 0, len(customRows)+1)
	actions = append(actions, defaultAction)
	for _, row := range customRows {
		actions = append(actions, normalizeDictationCustomAction(row))
	}
	encoded, err := json.Marshal(actions)
	if err != nil {
		return err
	}
	changed[dictationActionsKey] = string(encoded)
	return nil
}
