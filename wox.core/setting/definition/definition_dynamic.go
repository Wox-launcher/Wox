package definition

import (
	"context"
)

type PluginSettingValueDynamic struct {
	Key string
}

func (p *PluginSettingValueDynamic) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeDynamic
}

func (p *PluginSettingValueDynamic) GetKey() string {
	return p.Key
}

func (p *PluginSettingValueDynamic) GetDefaultValue() string {
	return ""
}

func (p *PluginSettingValueDynamic) Translate(translator func(ctx context.Context, key string) string) {
}
