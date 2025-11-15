package definition

import "context"

type PluginSettingValueCheckBox struct {
	Key          string
	Label        string
	DefaultValue string
	Tooltip      string
	Style        PluginSettingValueStyle
}

func (p *PluginSettingValueCheckBox) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeCheckBox
}

func (p *PluginSettingValueCheckBox) GetKey() string {
	return p.Key
}

func (p *PluginSettingValueCheckBox) GetDefaultValue() string {
	return p.DefaultValue
}

func (p *PluginSettingValueCheckBox) Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue {
	copy := *p
	copy.Label = translator(context.Background(), p.Label)
	return &copy
}
