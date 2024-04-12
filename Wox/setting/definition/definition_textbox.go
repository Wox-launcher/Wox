package definition

import "context"

type PluginSettingValueTextBox struct {
	Key          string
	Label        string
	Suffix       string
	DefaultValue string
	Tooltip      string

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

func (p *PluginSettingValueTextBox) Translate(translator func(ctx context.Context, key string) string) {
	p.Label = translator(context.Background(), p.Label)
	p.Suffix = translator(context.Background(), p.Suffix)
}
