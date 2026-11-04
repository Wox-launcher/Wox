package definition

import (
	"context"
	"wox/setting/validator"
)

type PluginSettingValueFilePath struct {
	Key          string
	Label        string
	DefaultValue string
	Tooltip      string
	Extensions   []string // File extensions shown by the native picker, without dots.
	Validators   []validator.PluginSettingValidator

	Style PluginSettingValueStyle `json:"-"` // Deprecated: ignored on load so Wox keeps setting layouts consistent.
}

func (p *PluginSettingValueFilePath) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeFilePath
}

func (p *PluginSettingValueFilePath) GetKey() string {
	return p.Key
}

func (p *PluginSettingValueFilePath) GetDefaultValue() string {
	return p.DefaultValue
}

func (p *PluginSettingValueFilePath) Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue {
	copy := *p
	copy.Label = translator(context.Background(), p.Label)
	copy.Tooltip = translator(context.Background(), p.Tooltip)
	return &copy
}
