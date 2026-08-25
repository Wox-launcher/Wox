package definition

import (
	"context"

	"github.com/google/uuid"
)

// PluginSettingValueStatsRow is one read-only label/value pair in a stats card.
type PluginSettingValueStatsRow struct {
	Label string
	Value string
}

// PluginSettingValueStats renders a read-only key/value summary card.
type PluginSettingValueStats struct {
	Key     string
	Title   string
	Tooltip string
	Rows    []PluginSettingValueStatsRow
}

func (p *PluginSettingValueStats) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeStats
}

func (p *PluginSettingValueStats) GetKey() string {
	if p.Key != "" {
		return p.Key
	}
	return uuid.NewString()
}

func (p *PluginSettingValueStats) GetDefaultValue() string {
	return ""
}

func (p *PluginSettingValueStats) Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue {
	copy := *p
	copy.Title = translator(context.Background(), p.Title)
	copy.Tooltip = translator(context.Background(), p.Tooltip)
	copy.Rows = make([]PluginSettingValueStatsRow, len(p.Rows))
	for i := range p.Rows {
		copy.Rows[i].Label = translator(context.Background(), p.Rows[i].Label)
		copy.Rows[i].Value = p.Rows[i].Value
	}
	return &copy
}
