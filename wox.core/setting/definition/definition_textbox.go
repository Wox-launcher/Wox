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

	Style PluginSettingValueStyle `json:"-"` // Deprecated: ignored on load so Wox keeps setting layouts consistent.
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
	label := translator(context.Background(), p.Label)
	suffix := translator(context.Background(), p.Suffix)
	tooltip := translator(context.Background(), p.Tooltip)
	if label == p.Label && suffix == p.Suffix && tooltip == p.Tooltip {
		return p
	}
	copy := *p
	copy.Label = label
	copy.Suffix = suffix
	copy.Tooltip = tooltip
	return &copy
}
