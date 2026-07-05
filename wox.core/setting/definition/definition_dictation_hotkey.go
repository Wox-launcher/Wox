package definition

import (
	"context"
)

// PluginSettingValueDictationHotkey is a Wox-internal setting component for
// recording a global hotkey used by the dictation plugin. Unlike a textbox,
// the Flutter side renders a dedicated hotkey recorder widget that captures
// physical key presses and produces a combine-key string (e.g. "cmd+shift+d").
type PluginSettingValueDictationHotkey struct {
	Key          string
	Label        string
	Tooltip      string
	DefaultValue string
}

func (p *PluginSettingValueDictationHotkey) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeDictationHotkey
}

func (p *PluginSettingValueDictationHotkey) GetKey() string {
	return p.Key
}

func (p *PluginSettingValueDictationHotkey) GetDefaultValue() string {
	return p.DefaultValue
}

func (p *PluginSettingValueDictationHotkey) Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue {
	copy := *p
	copy.Label = translator(context.Background(), p.Label)
	copy.Tooltip = translator(context.Background(), p.Tooltip)
	return &copy
}
