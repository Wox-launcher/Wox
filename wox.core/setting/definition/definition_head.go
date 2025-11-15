package definition

import (
	"context"

	"github.com/google/uuid"
)

type PluginSettingValueHead struct {
	Content string
	Tooltip string
	Style   PluginSettingValueStyle
}

func (p *PluginSettingValueHead) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeHead
}

func (p *PluginSettingValueHead) GetKey() string {
	return uuid.NewString()
}

func (p *PluginSettingValueHead) GetDefaultValue() string {
	return ""
}

func (p *PluginSettingValueHead) Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue {
	return p
}
