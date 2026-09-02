package launcher

import (
	"testing"

	"wox/common"
	"wox/setting/definition"
	woxcomponent "wox/ui/launcher/component"
)

func TestNewAISettingsFormMatchesFlutterTableDefinitions(t *testing.T) {
	form := newAISettingsForm(settingsData{})
	if len(form.definitions) != 4 {
		t.Fatalf("AI table count = %d, want 4", len(form.definitions))
	}

	providers := form.definitions[0].Value
	if !providers.InlineTable || providers.SortColumnKey != "Name" {
		t.Fatalf("provider table options = inline %v, sort %q; want inline and Name", providers.InlineTable, providers.SortColumnKey)
	}
	assertFormTableColumnWidths(t, providers.Columns, []int{40, 100, 120, 160, 0})
	if providers.Columns[0].Type != "aiModelStatus" || !providers.Columns[0].HideInUpdate {
		t.Fatalf("provider status column = type %q, hide in update %v", providers.Columns[0].Type, providers.Columns[0].HideInUpdate)
	}

	builtin := form.definitions[1].Value
	if !builtin.InlineTable || builtin.Key != "AIBuiltinTools" || builtin.SortColumnKey != "Name" {
		t.Fatalf("builtin tools table = key %q inline %v sort %q", builtin.Key, builtin.InlineTable, builtin.SortColumnKey)
	}
	assertFormTableColumnWidths(t, builtin.Columns, []int{140, 0, 80})

	mcp := form.definitions[2].Value
	if !mcp.InlineTable || mcp.SortColumnKey != "Name" {
		t.Fatalf("MCP table options = inline %v, sort %q; want inline and Name", mcp.InlineTable, mcp.SortColumnKey)
	}
	assertFormTableColumnWidths(t, mcp.Columns, []int{100, 0, 80, 80, 100, 160, 160, 120})
	if mcp.Columns[5].Key != "Args" || mcp.Columns[5].Type != "textList" {
		t.Fatalf("MCP args column = %+v", mcp.Columns[5])
	}
	if mcp.Columns[7].Key != "Url" || mcp.Columns[7].TextMaxLines != 1 {
		t.Fatalf("MCP url column = %+v", mcp.Columns[7])
	}
	for _, index := range []int{3, 4, 5, 6, 7} {
		if !mcp.Columns[index].HideInTable {
			t.Fatalf("MCP column %s should stay hidden in the table", mcp.Columns[index].Key)
		}
	}
	assertFormTableColumnVisibleWhen(t, mcp.Columns[4], "Type", "stdio")
	assertFormTableColumnVisibleWhen(t, mcp.Columns[5], "Type", "stdio")
	assertFormTableColumnVisibleWhen(t, mcp.Columns[6], "Type", "stdio")
	assertFormTableColumnVisibleWhen(t, mcp.Columns[7], "Type", "streamable-http")

	skills := form.definitions[3].Value
	if !skills.InlineTable || skills.SortColumnKey != "Name" || skills.MaxHeight != 360 {
		t.Fatalf("skills table options = inline %v, sort %q, max height %d; want inline, Name, 360", skills.InlineTable, skills.SortColumnKey, skills.MaxHeight)
	}
	assertFormTableColumnWidths(t, skills.Columns[:3], []int{200, 100, 400})
}

func TestBuiltinSkillViewRowIsReadOnly(t *testing.T) {
	definition := newAISettingsForm(settingsData{}).definitions[3]
	rows := (&App{}).formTableViewRows(definition, nil, []map[string]any{
		{"Name": "wox-plugin-creator", "Builtin": true},
		{"Name": "user-skill"},
	}, woxcomponent.Theme{}, 1)
	if len(rows) != 2 || !rows[1].ReadOnly || rows[0].ReadOnly {
		t.Fatalf("skill view rows = %+v, want only the built-in row read-only", rows)
	}
}

func TestBuiltinToolRowsJSONUsesDisableList(t *testing.T) {
	raw := builtinToolRowsJSON([]aiBuiltinToolInfo{
		{Name: "read", Description: "Read a file"},
		{Name: "bash", Description: "Run a command"},
	}, []string{"bash"})
	rows, err := decodeFormTableRows(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("row count = %d, want 2", len(rows))
	}
	if rows[0]["Name"] != "read" || rows[0]["Enabled"] != true {
		t.Fatalf("read row = %#v", rows[0])
	}
	if rows[1]["Name"] != "bash" || rows[1]["Enabled"] != false {
		t.Fatalf("bash row = %#v", rows[1])
	}
}

func TestMCPServerRowFieldsFollowTransportType(t *testing.T) {
	definition := newAISettingsForm(settingsData{}).definitions[2]
	stdio, _ := formTableRowFields(definition, map[string]any{"Type": "stdio", "Command": "uvx", "Args": []string{"duckduckgo-mcp-server"}})
	assertFormFieldKeys(t, stdio, []string{"Name", "Disabled", "Type", "Command", "Args", "EnvironmentVariables"})
	if stdio.values["Command"] != "uvx" || stdio.values["Args"] != "duckduckgo-mcp-server" {
		t.Fatalf("stdio values = %#v", stdio.values)
	}

	http, _ := formTableRowFields(definition, map[string]any{"Type": "streamable-http", "Url": "https://example.com/mcp"})
	assertFormFieldKeys(t, http, []string{"Name", "Disabled", "Type", "Url"})
	if http.values["Url"] != "https://example.com/mcp" {
		t.Fatalf("http url = %q", http.values["Url"])
	}
	if field := formFieldByKey(http, "Url"); field.Value.MaxLines != 1 {
		t.Fatalf("http url max lines = %d, want 1", field.Value.MaxLines)
	}
}

func TestMCPServerRowFieldsKeepHiddenValuesWhenTypeChanges(t *testing.T) {
	definition := newAISettingsForm(settingsData{}).definitions[2]
	fields, _ := formTableRowFields(definition, map[string]any{
		"Name": "ddg-search", "Type": "stdio", "Command": "uvx", "Args": []string{"duckduckgo-mcp-server"},
	})
	state := &formTableEditorState{definition: definition, rowForm: &fields}
	fields.values["Type"] = "streamable-http"
	applyFormTableRowVisibleFieldsLocked(state)
	assertFormFieldKeys(t, *state.rowForm, []string{"Name", "Disabled", "Type", "Url"})
	if state.rowForm.values["Command"] != "uvx" || state.rowForm.values["Args"] != "duckduckgo-mcp-server" {
		t.Fatalf("hidden stdio values should stay, got %#v", state.rowForm.values)
	}

	state.rowForm.values["Type"] = "stdio"
	applyFormTableRowVisibleFieldsLocked(state)
	assertFormFieldKeys(t, *state.rowForm, []string{"Name", "Disabled", "Type", "Command", "Args", "EnvironmentVariables"})
	if field := formFieldByKey(*state.rowForm, "Command"); field.Type != "textbox" || field.Value.MaxLines != 1 {
		t.Fatalf("stdio command field = %+v", field)
	}
}

func assertFormTableColumnVisibleWhen(t *testing.T, column formTableColumn, key, value string) {
	t.Helper()
	if column.VisibleWhen.Key != key || len(column.VisibleWhen.Values) != 1 || column.VisibleWhen.Values[0] != value {
		t.Fatalf("column %s visible when = %+v, want %s=%s", column.Key, column.VisibleWhen, key, value)
	}
}

func assertFormFieldKeys(t *testing.T, fields formFieldsState, want []string) {
	t.Helper()
	got := make([]string, 0, len(fields.definitions))
	for _, definition := range fields.definitions {
		got = append(got, definition.Value.Key)
	}
	if len(got) != len(want) {
		t.Fatalf("field keys = %v, want %v", got, want)
	}
	for index, key := range want {
		if got[index] != key {
			t.Fatalf("field keys = %v, want %v", got, want)
		}
	}
}

func formFieldByKey(fields formFieldsState, key string) formDefinition {
	for _, definition := range fields.definitions {
		if definition.Value.Key == key {
			return definition
		}
	}
	return formDefinition{}
}

func assertFormTableColumnWidths(t *testing.T, columns []formTableColumn, want []int) {
	t.Helper()
	if len(columns) != len(want) {
		t.Fatalf("column count = %d, want %d", len(columns), len(want))
	}
	for index, width := range want {
		if columns[index].Width != width {
			t.Fatalf("column %d width = %d, want %d", index, columns[index].Width, width)
		}
	}
}

// TestApplyAIProviderCatalogLockedCarriesIcons guards the Flutter contract that the AI
// provider name dropdown shows each provider icon next to its label.
func TestApplyAIProviderCatalogLockedCarriesIcons(t *testing.T) {
	form := newAISettingsForm(settingsData{})
	providers := []aiProviderInfo{
		{Name: "openai", Icon: woxImage{ImageType: "url", ImageData: "https://example.com/openai.svg"}, DefaultHost: "https://api.openai.com"},
		{Name: "ollama", DefaultHost: "http://localhost:11434"},
	}
	applyAIProviderCatalogLocked(&form, providers)

	var options []formOption
	for _, column := range form.definitions[0].Value.Columns {
		if column.Key == "Name" {
			options = column.SelectOptions
		}
	}
	if len(options) != 2 {
		t.Fatalf("provider options = %d, want 2", len(options))
	}
	if options[0].Value != "openai" || options[0].Icon.ImageType != "url" || options[0].Icon.ImageData != "https://example.com/openai.svg" {
		t.Fatalf("openai option icon not carried: %+v", options[0])
	}
	if options[1].Value != "ollama" || options[1].Icon.ImageType != "" {
		t.Fatalf("ollama option should have an empty icon: %+v", options[1])
	}
}

// TestFromCoreSelectOptionsCarriesIcons keeps plugin select options on the same
// icon contract as the built-in AI provider dropdown.
func TestFromCoreSelectOptionsCarriesIcons(t *testing.T) {
	converted := fromCoreSelectOptions([]definition.PluginSettingValueSelectOption{
		{Label: "OpenAI", Value: "openai", Icon: common.WoxImage{ImageType: "url", ImageData: "https://example.com/openai.svg"}},
		{Label: "Local", Value: "local"},
	})
	if len(converted) != 2 {
		t.Fatalf("converted options = %d, want 2", len(converted))
	}
	if converted[0].Icon.ImageType != "url" || converted[0].Icon.ImageData != "https://example.com/openai.svg" {
		t.Fatalf("first option icon not carried: %+v", converted[0])
	}
	if converted[1].Icon.ImageType != "" {
		t.Fatalf("second option should have an empty icon: %+v", converted[1])
	}
}
