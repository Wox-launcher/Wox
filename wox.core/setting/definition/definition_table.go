package definition

import (
	"context"
	"wox/setting/validator"
)

type PluginSettingValueTableColumnType = string

const (
	PluginSettingValueTableColumnTypeText                   PluginSettingValueTableColumnType = "text"
	PluginSettingValueTableColumnTypeTextList               PluginSettingValueTableColumnType = "textList"
	PluginSettingValueTableColumnTypeCheckbox               PluginSettingValueTableColumnType = "checkbox"
	PluginSettingValueTableColumnTypeDirPath                PluginSettingValueTableColumnType = "dirPath"
	PluginSettingValueTableColumnTypeSelect                 PluginSettingValueTableColumnType = "select"
	PluginSettingValueTableColumnTypeSelectAIModel          PluginSettingValueTableColumnType = "selectAIModel"
	PluginSettingValueTableColumnTypeAIModelStatus          PluginSettingValueTableColumnType = "aiModelStatus"
	PluginSettingValueTableColumnTypeAIMCPServerTools       PluginSettingValueTableColumnType = "aiMCPServerTools"
	PluginSettingValueTableColumnTypeAISelectMCPServerTools PluginSettingValueTableColumnType = "aiSelectMCPServerTools"
	PluginSettingValueTableColumnTypeWoxImage               PluginSettingValueTableColumnType = "woxImage"
)

const (
	PluginSettingValueTableSortOrderAsc  = "asc"
	PluginSettingValueTableSortOrderDesc = "desc"
)

type PluginSettingValueTable struct {
	Key           string
	DefaultValue  string
	Title         string
	Tooltip       string
	Columns       []PluginSettingValueTableColumn
	SortColumnKey string // The key of the column that should be used for sorting
	SortOrder     string // asc or desc

	Style PluginSettingValueStyle
}

type PluginSettingValueTableColumn struct {
	Key           string
	Label         string
	Tooltip       string
	Width         int
	Type          PluginSettingValueTableColumnType
	Validators    []validator.PluginSettingValidator // validators for this setting, every validator should be satisfied
	SelectOptions []PluginSettingValueSelectOption   // Only used when Type is PluginSettingValueTableColumnTypeSelect
	TextMaxLines  int                                // Only used when Type is PluginSettingValueTableColumnTypeText
	HideInTable   bool                               // Hide this column in the table, but still show it in the setting dialog
	HideInUpdate  bool                               // Hide this column in the update/add dialog, but still show it in the table
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

func (p *PluginSettingValueTable) Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue {
	copy := *p
	copy.Title = translator(context.Background(), p.Title)
	// Deep copy Columns
	copy.Columns = make([]PluginSettingValueTableColumn, len(p.Columns))
	for i := range p.Columns {
		copy.Columns[i] = p.Columns[i]
		copy.Columns[i].Label = translator(context.Background(), p.Columns[i].Label)
		copy.Columns[i].Tooltip = translator(context.Background(), p.Columns[i].Tooltip)
	}
	return &copy
}
