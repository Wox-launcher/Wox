package definition

import (
	"context"
	"wox/common"
	"wox/setting/validator"
)

type PluginSettingValueSelect struct {
	Key          string
	Label        string
	Suffix       string
	DefaultValue string
	Tooltip      string
	IsMulti      bool
	Options      []PluginSettingValueSelectOption
	Validators   []validator.PluginSettingValidator // validators for this setting, every validator should be satisfied

	Style PluginSettingValueStyle
}

const PluginSettingValueSelectOptionValueSelectAll = "all"

type PluginSettingValueSelectOption struct {
	Label string
	Value string
	Icon  common.WoxImage

	// Indicates if this option represents a "Select All" choice,
	// which allows users to select all available options at once.
	// This is useful for multi-select scenarios where users may want to quickly select all options without having to click each one individually.
	// when use selected all, the value will be [PluginSettingValueSelectOptionValueSelectAll], and the plugin should handle this value accordingly (e.g., treat it as selecting all options).
	IsSelectAll bool
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
	copy.Tooltip = translator(context.Background(), p.Tooltip)
	// Deep copy Options
	copy.Options = make([]PluginSettingValueSelectOption, len(p.Options))
	for i := range p.Options {
		copy.Options[i].Label = translator(context.Background(), p.Options[i].Label)
		copy.Options[i].Value = p.Options[i].Value
		copy.Options[i].Icon = p.Options[i].Icon
		copy.Options[i].IsSelectAll = p.Options[i].IsSelectAll
	}
	return &copy
}
