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
	label := translator(context.Background(), p.Label)
	tooltip := translator(context.Background(), p.Tooltip)
	if label == p.Label && tooltip == p.Tooltip {
		return p
	}
	copy := *p
	copy.Label = label
	copy.Tooltip = tooltip
	return &copy
}
