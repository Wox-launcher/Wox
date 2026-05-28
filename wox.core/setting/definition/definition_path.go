package definition

import (
	"context"
	"wox/setting/validator"
)

type PluginSettingValuePath struct {
	Key          string
	Label        string
	Suffix       string
	DefaultValue string
	Tooltip      string
	Validators   []validator.PluginSettingValidator

	Style PluginSettingValueStyle
}

func (p *PluginSettingValuePath) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypePath
}

func (p *PluginSettingValuePath) GetKey() string {
	return p.Key
}

func (p *PluginSettingValuePath) GetDefaultValue() string {
	return p.DefaultValue
}

func (p *PluginSettingValuePath) Translate(translator func(ctx context.Context, key string) string) {
	p.Label = translator(context.Background(), p.Label)
	p.Suffix = translator(context.Background(), p.Suffix)
	p.Tooltip = translator(context.Background(), p.Tooltip)
}
