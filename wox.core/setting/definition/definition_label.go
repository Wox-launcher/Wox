package definition

import (
	"context"

	"github.com/google/uuid"
)

type PluginSettingValueLabel struct {
	Content string
	Tooltip string
	Style   PluginSettingValueStyle
}

func (p *PluginSettingValueLabel) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeLabel
}

func (p *PluginSettingValueLabel) GetKey() string {
	return uuid.NewString()
}

func (p *PluginSettingValueLabel) GetDefaultValue() string {
	return ""
}

func (p *PluginSettingValueLabel) Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue {
	copy := *p
	copy.Content = translator(context.Background(), p.Content)
	return &copy
}
