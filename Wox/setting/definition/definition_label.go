package definition

import (
	"context"
	"github.com/google/uuid"
)

type PluginSettingValueLabel struct {
	Content string
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

func (p *PluginSettingValueLabel) Translate(translator func(ctx context.Context, key string) string) {
	p.Content = translator(context.Background(), p.Content)
}
