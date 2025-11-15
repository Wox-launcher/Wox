package definition

import (
	"context"
	"wox/setting/validator"
)

type PluginSettingValueSelectAIModel struct {
	Key          string
	Label        string
	Suffix       string
	DefaultValue string
	Tooltip      string
	Validators   []validator.PluginSettingValidator // validators for this setting, every validator should be satisfied

	Style PluginSettingValueStyle
}

func (p *PluginSettingValueSelectAIModel) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeSelectAIModel
}

func (p *PluginSettingValueSelectAIModel) GetKey() string {
	return p.Key
}

func (p *PluginSettingValueSelectAIModel) GetDefaultValue() string {
	return p.DefaultValue
}

func (p *PluginSettingValueSelectAIModel) Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue {
	copy := *p
	copy.Label = translator(context.Background(), p.Label)
	copy.Suffix = translator(context.Background(), p.Suffix)
	return &copy
}
