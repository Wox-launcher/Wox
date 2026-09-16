package launcher

import (
	"strings"
	"testing"
	"wox/setting/definition"
)

func TestFormTableRowEditorSectionsKeepUngroupedFieldsFirst(t *testing.T) {
	definition := formDefinition{Value: formDefinitionValue{
		Groups: []formTableGroup{
			{Key: "advanced", Title: "Advanced", CollapsedByDefault: true},
			{Key: "webview", Title: "WebView", CollapsedByDefault: true},
		},
	}}
	fields := []formDefinition{
		{Value: formDefinitionValue{Key: "Keyword"}},
		{Value: formDefinitionValue{Key: "InjectCSS", Group: "webview"}},
		{Value: formDefinitionValue{Key: "Title"}},
		{Value: formDefinitionValue{Key: "IsFallback", Group: "advanced"}},
		{Value: formDefinitionValue{Key: "Missing", Group: "unknown"}},
	}
	sections := formTableRowEditorSections(definition, fields, defaultFormTableCollapsedGroups(definition), nil)
	if len(sections) != 3 {
		t.Fatalf("sections = %d, want 3", len(sections))
	}
	if sections[0].Group.Key != "" || fieldKeys(sections[0]) != "Keyword,Title,Missing" {
		t.Fatalf("ungrouped = %#v", sections[0])
	}
	if sections[1].Group.Key != "advanced" || !sections[1].Collapsed || fieldKeys(sections[1]) != "IsFallback" {
		t.Fatalf("advanced = %#v", sections[1])
	}
	if sections[2].Group.Key != "webview" || !sections[2].Collapsed || fieldKeys(sections[2]) != "InjectCSS" {
		t.Fatalf("webview = %#v", sections[2])
	}
}

func TestFormTableRowEditorSectionsOmitEmptyGroups(t *testing.T) {
	definition := formDefinition{Value: formDefinitionValue{
		Groups: []formTableGroup{{Key: "webview", Title: "WebView"}},
	}}
	sections := formTableRowEditorSections(definition, []formDefinition{{Value: formDefinitionValue{Key: "Keyword"}}}, nil, nil)
	if len(sections) != 1 || sections[0].Group.Key != "" || fieldKeys(sections[0]) != "Keyword" {
		t.Fatalf("sections = %#v", sections)
	}
}

func TestFromCoreFormDefinitionMapsTableGroups(t *testing.T) {
	converted, ok := fromCoreFormDefinition(definition.PluginSettingDefinitionItem{
		Type: definition.PluginSettingDefinitionTypeTable,
		Value: &definition.PluginSettingValueTable{
			Key: "webSearches",
			Groups: []definition.PluginSettingValueTableGroup{
				{Key: "webview", Title: "i18n:plugin_websearch_group_webview", CollapsedByDefault: true},
			},
			Columns: []definition.PluginSettingValueTableColumn{
				{Key: "Keyword", Type: definition.PluginSettingValueTableColumnTypeText},
				{Key: "WebViewWidth", Type: definition.PluginSettingValueTableColumnTypeText, Group: "webview", EmptyAsZero: true},
			},
		},
	})
	if !ok || len(converted.Value.Groups) != 1 || converted.Value.Groups[0].Key != "webview" || !converted.Value.Groups[0].CollapsedByDefault {
		t.Fatalf("groups = %#v", converted.Value.Groups)
	}
	if converted.Value.Columns[1].Group != "webview" || !converted.Value.Columns[1].EmptyAsZero {
		t.Fatalf("column = %#v", converted.Value.Columns[1])
	}
	field, _ := formTableColumnDefinition(converted.Value.Columns[1], nil)
	if field.Value.Group != "webview" {
		t.Fatalf("mapped field group = %q", field.Value.Group)
	}
}

func TestBeginFormTableRowEditCollapsesDefaultGroups(t *testing.T) {
	definition := formDefinition{Type: "table", Value: formDefinitionValue{
		Key: "webSearches",
		Groups: []formTableGroup{
			{Key: "webview", Title: "WebView", CollapsedByDefault: true},
		},
		Columns: []formTableColumn{
			{Key: "Keyword", Type: "text"},
			{Key: "InjectCSS", Type: "text", Group: "webview"},
		},
	}}
	target := newFormFieldsState([]formDefinition{definition}, map[string]string{"webSearches": `[{"Keyword":"g"}]`}, true)
	app := &App{
		launcherTableEditor: &formTableEditorState{
			target: &target, definition: definition, rows: []map[string]any{{"Keyword": "g"}}, selected: 0, rowIndex: -1, deletePending: -1,
		},
	}
	app.beginFormTableRowEdit(0, false, false)
	if !app.launcherTableEditor.collapsedGroups["webview"] {
		t.Fatalf("collapsed = %#v", app.launcherTableEditor.collapsedGroups)
	}
	app.toggleFormTableRowGroup("webview")
	if app.launcherTableEditor.collapsedGroups["webview"] {
		t.Fatal("toggle should expand a collapsed group")
	}
}

func TestExpandFormTableGroupsForErrors(t *testing.T) {
	state := &formTableEditorState{
		definition: formDefinition{Value: formDefinitionValue{
			Columns: []formTableColumn{{Key: "InjectCSS", Group: "webview"}},
			Groups:  []formTableGroup{{Key: "webview", Title: "WebView", CollapsedByDefault: true}},
		}},
		collapsedGroups: map[string]bool{"webview": true},
		fieldErrors:     map[string]string{"InjectCSS": "required"},
	}
	expandFormTableGroupsForErrors(state)
	if state.collapsedGroups["webview"] {
		t.Fatal("group with a field error should expand")
	}
}

func fieldKeys(section formTableRowEditorSection) string {
	keys := make([]string, 0, len(section.Fields))
	for _, field := range section.Fields {
		keys = append(keys, field.Definition.Value.Key)
	}
	return strings.Join(keys, ",")
}
