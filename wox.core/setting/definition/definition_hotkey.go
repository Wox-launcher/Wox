package definition

import (
	"context"
)

// PluginSettingValueHotkey is a Wox-internal hotkey recorder for action forms
// and other core-owned surfaces. It is not part of the public plugin SDK.
type PluginSettingValueHotkey struct {
	Key          string
	Label        string
	Tooltip      string
	DefaultValue string

	Style PluginSettingValueStyle `json:"-"`
}

func (p *PluginSettingValueHotkey) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeHotkey
}

func (p *PluginSettingValueHotkey) GetKey() string {
	return p.Key
}

func (p *PluginSettingValueHotkey) GetDefaultValue() string {
	return p.DefaultValue
}

func (p *PluginSettingValueHotkey) Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue {
	copy := *p
	copy.Label = translator(context.Background(), p.Label)
	copy.Tooltip = translator(context.Background(), p.Tooltip)
	return &copy
}
