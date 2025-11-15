package definition

import (
	"context"
	"wox/setting/validator"
)

type PluginSettingValueSelect struct {
	Key          string
	Label        string
	Suffix       string
	DefaultValue string
	Tooltip      string
	Options      []PluginSettingValueSelectOption
	Validators   []validator.PluginSettingValidator // validators for this setting, every validator should be satisfied

	Style PluginSettingValueStyle
}

type PluginSettingValueSelectOption struct {
	Label string
	Value string
}

func (p *PluginSettingValueSelect) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeSelect
}

func (p *PluginSettingValueSelect) GetKey() string {
	return p.Key
}

func (p *PluginSettingValueSelect) GetDefaultValue() string {
	return p.DefaultValue
}

func (p *PluginSettingValueSelect) Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue {
	copy := *p
	copy.Label = translator(context.Background(), p.Label)
	copy.Suffix = translator(context.Background(), p.Suffix)
	// Deep copy Options
	copy.Options = make([]PluginSettingValueSelectOption, len(p.Options))
	for i := range p.Options {
		copy.Options[i].Label = translator(context.Background(), p.Options[i].Label)
		copy.Options[i].Value = p.Options[i].Value
	}
	return &copy
}
