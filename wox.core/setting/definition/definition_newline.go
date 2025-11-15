package definition

import (
	"context"

	"github.com/google/uuid"
)

type PluginSettingValueNewLine struct {
	Style PluginSettingValueStyle
}

func (p *PluginSettingValueNewLine) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeNewLine
}

func (p *PluginSettingValueNewLine) GetKey() string {
	return uuid.NewString()
}

func (p *PluginSettingValueNewLine) GetDefaultValue() string {
	return ""
}

func (p *PluginSettingValueNewLine) Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue {
	return p
}
