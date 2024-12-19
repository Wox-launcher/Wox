package definition

import (
	"context"
	"wox/setting/validator"
)

type PluginSettingValueSelectAiModel struct {
	Key          string
	Label        string
	Suffix       string
	DefaultValue string
	Tooltip      string
	Validators   []validator.PluginSettingValidator // validators for this setting, every validator should be satisfied

	Style PluginSettingValueStyle
}

func (p *PluginSettingValueSelectAiModel) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeSelectAIModel
}

func (p *PluginSettingValueSelectAiModel) GetKey() string {
	return p.Key
}

func (p *PluginSettingValueSelectAiModel) GetDefaultValue() string {
	return p.DefaultValue
}

func (p *PluginSettingValueSelectAiModel) Translate(translator func(ctx context.Context, key string) string) {
	p.Label = translator(context.Background(), p.Label)
	p.Suffix = translator(context.Background(), p.Suffix)
}
