package definition

import "context"

type PluginSettingValueTableColumnType = string

const (
	PluginSettingValueTableColumnTypeText     PluginSettingValueTableColumnType = "text"
	PluginSettingValueTableColumnTypeCheckbox PluginSettingValueTableColumnType = "checkbox"
	PluginSettingValueTableColumnTypeDirPath  PluginSettingValueTableColumnType = "dirPath"
	PluginSettingValueTableColumnTypeSelect   PluginSettingValueTableColumnType = "select"
	PluginSettingValueTableColumnTypeWoxImage PluginSettingValueTableColumnType = "woxImage"
)

type PluginSettingValueTable struct {
	Key          string
	DefaultValue string
	EnableFilter bool
	Columns      []PluginSettingValueTableColumn

	Style PluginSettingValueStyle
}

type PluginSettingValueTableColumn struct {
	Key           string
	Label         string
	Tooltip       string
	Width         int
	Type          PluginSettingValueTableColumnType
	SelectOptions []PluginSettingValueSelectOption // Only used when Type is PluginSettingValueTableColumnTypeSelect
}

func (p *PluginSettingValueTable) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeNewLine
}

func (p *PluginSettingValueTable) GetKey() string {
	return p.Key
}

func (p *PluginSettingValueTable) GetDefaultValue() string {
	return p.DefaultValue
}

func (p *PluginSettingValueTable) Translate(translator func(ctx context.Context, key string) string) {
	for i := range p.Columns {
		p.Columns[i].Label = translator(context.Background(), p.Columns[i].Label)
	}
}
