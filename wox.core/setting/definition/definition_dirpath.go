package definition

import (
	"context"
	"wox/setting/validator"
)

type PluginSettingValueDirPath struct {
	Key          string
	Label        string
	DefaultValue string
	Tooltip      string
	Validators   []validator.PluginSettingValidator

	Style PluginSettingValueStyle `json:"-"` // Deprecated: ignored on load so Wox keeps setting layouts consistent.
}

func (p *PluginSettingValueDirPath) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeDirPath
}

func (p *PluginSettingValueDirPath) GetKey() string {
	return p.Key
}

func (p *PluginSettingValueDirPath) GetDefaultValue() string {
	return p.DefaultValue
}

func (p *PluginSettingValueDirPath) Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue {
	copy := *p
	copy.Label = translator(context.Background(), p.Label)
	copy.Tooltip = translator(context.Background(), p.Tooltip)
	return &copy
}
