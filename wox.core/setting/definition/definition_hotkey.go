package definition

import (
	"context"
)

type HotkeyScope string

const (
	HotkeyScopeLocal  HotkeyScope = "local"
	HotkeyScopeGlobal HotkeyScope = "global"
)

// PluginSettingValueHotkey opts into automatic registration only with global scope.
type PluginSettingValueHotkey struct {
	Key                string
	Label              string
	Tooltip            string
	DefaultValue       string
	Scope              HotkeyScope
	AllowedHotkeyKinds []string

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
