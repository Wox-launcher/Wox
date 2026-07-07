package dictation

import (
	"testing"

	"wox/setting/definition"
)

func TestDictationMetadataIncludesDictionaryTable(t *testing.T) {
	metadata := (&DictationPlugin{}).GetMetadata()

	table := findDictionaryTableSetting(t, metadata.SettingDefinitions)
	if table.Title == "" {
		t.Fatalf("dictionary table title must not be empty")
	}

	requiredColumns := map[string]definition.PluginSettingValueTableColumnType{
		"context":       definition.PluginSettingValueTableColumnTypeText,
		"wrongPhrase":   definition.PluginSettingValueTableColumnTypeText,
		"correctPhrase": definition.PluginSettingValueTableColumnTypeText,
	}
	for columnKey, columnType := range requiredColumns {
		column, ok := findTableColumn(table.Columns, columnKey)
		if !ok {
			t.Fatalf("dictionary table missing column %q in %#v", columnKey, table.Columns)
		}
		if column.Type != columnType {
			t.Fatalf("dictionary table column %q type = %q, want %q", columnKey, column.Type, columnType)
		}
	}
	if _, ok := findTableColumn(table.Columns, "enabled"); ok {
		t.Fatalf("dictionary table should not expose an enabled column; users can delete unwanted entries")
	}
}

func findDictionaryTableSetting(t *testing.T, settings definition.PluginSettingDefinitions) *definition.PluginSettingValueTable {
	t.Helper()
	for _, item := range settings {
		table, ok := item.Value.(*definition.PluginSettingValueTable)
		if !ok {
			continue
		}
		if item.Type == definition.PluginSettingDefinitionTypeTable && table.Key == settingKeyDictionary {
			return table
		}
	}
	t.Fatalf("dictation metadata missing dictionary table setting")
	return nil
}

func findTableColumn(columns []definition.PluginSettingValueTableColumn, key string) (definition.PluginSettingValueTableColumn, bool) {
	for _, column := range columns {
		if column.Key == key {
			return column, true
		}
	}
	return definition.PluginSettingValueTableColumn{}, false
}
