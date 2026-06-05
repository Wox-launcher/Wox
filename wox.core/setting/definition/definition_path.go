package definition

import (
	"context"
	"wox/setting/validator"
)

type PluginSettingValuePath struct {
	Key              string
	Label            string
	Suffix           string
	DefaultValue     string
	Tooltip          string
	Validators       []validator.PluginSettingValidator
	IsDirectory      bool     `json:"IsDirectory"`
	AllowedExtensions []string `json:"AllowedExtensions"`
	AllowMultiple    bool     `json:"AllowMultiple"`

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

func (p *PluginSettingValuePath) Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue {
	copy := *p
	copy.Label = translator(context.Background(), p.Label)
	copy.Suffix = translator(context.Background(), p.Suffix)
	copy.Tooltip = translator(context.Background(), p.Tooltip)
	return &copy
}
