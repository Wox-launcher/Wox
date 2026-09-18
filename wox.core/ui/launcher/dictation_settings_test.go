package launcher

import "testing"

func TestApplyDictationFormCompatibilityReadsAIRefineFromActions(t *testing.T) {
	values := map[string]string{
		dictationDefaultAIRefineKey: "false",
		dictationActionsKey:         `[{"id":"default","type":"default","name":"i18n:plugin_dictation_default_action_name","hotkey":"hold:left_ctrl+left_shift","output":"input","aiRefineEnabled":true}]`,
	}
	plugin := pluginSettingsPlugin{
		ID:      dictationPluginID,
		Setting: pluginSettingsData{Settings: map[string]string{dictationActionsKey: values[dictationActionsKey]}},
	}

	applyDictationFormCompatibility(plugin, values)

	if values[dictationDefaultAIRefineKey] != "true" {
		t.Fatalf("AI Polish = %q, want true from the default action", values[dictationDefaultAIRefineKey])
	}
}
