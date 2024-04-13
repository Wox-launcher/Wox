package definition

import "context"

type PluginSettingValueTableColumnType = string

const (
	PluginSettingValueTableColumnTypeText     PluginSettingValueTableColumnType = "text"
	PluginSettingValueTableColumnTypeTextList PluginSettingValueTableColumnType = "textList"
	PluginSettingValueTableColumnTypeCheckbox PluginSettingValueTableColumnType = "checkbox"
	PluginSettingValueTableColumnTypeDirPath  PluginSettingValueTableColumnType = "dirPath"
	PluginSettingValueTableColumnTypeSelect   PluginSettingValueTableColumnType = "select"
	PluginSettingValueTableColumnTypeWoxImage PluginSettingValueTableColumnType = "woxImage"
)

type PluginSettingValueTable struct {
	Key          string
	DefaultValue string
	Title        string
	Tooltip      string
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
	TextMaxLines  int                              // Only used when Type is PluginSettingValueTableColumnTypeText
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
	p.Title = translator(context.Background(), p.Title)
	for i := range p.Columns {
		p.Columns[i].Label = translator(context.Background(), p.Columns[i].Label)
		p.Columns[i].Tooltip = translator(context.Background(), p.Columns[i].Tooltip)
	}
}
