package definition

import (
	"context"
	"wox/setting/validator"
)

type PluginSettingValueTextBox struct {
	Key          string
	Label        string
	Suffix       string
	DefaultValue string
	Tooltip      string
	MaxLines     int                                // max lines for textbox, default 1
	Validators   []validator.PluginSettingValidator // validators for this setting, every validator should be satisfied

	Style PluginSettingValueStyle
}

func (p *PluginSettingValueTextBox) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeTextBox
}

func (p *PluginSettingValueTextBox) GetKey() string {
	return p.Key
}

func (p *PluginSettingValueTextBox) GetDefaultValue() string {
	return p.DefaultValue
}

func (p *PluginSettingValueTextBox) Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue {
	copy := *p
	copy.Label = translator(context.Background(), p.Label)
	copy.Suffix = translator(context.Background(), p.Suffix)
	return &copy
}
