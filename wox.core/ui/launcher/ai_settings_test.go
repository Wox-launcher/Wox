package launcher

import (
	"testing"

	"wox/common"
	"wox/setting/definition"
)

func TestNewAISettingsFormMatchesFlutterTableDefinitions(t *testing.T) {
	form := newAISettingsForm(settingsData{})
	if len(form.definitions) != 3 {
		t.Fatalf("AI table count = %d, want 3", len(form.definitions))
	}

	providers := form.definitions[0].Value
	if !providers.InlineTable || providers.SortColumnKey != "Name" {
		t.Fatalf("provider table options = inline %v, sort %q; want inline and Name", providers.InlineTable, providers.SortColumnKey)
	}
	assertFormTableColumnWidths(t, providers.Columns, []int{40, 100, 120, 160, 0})
	if providers.Columns[0].Type != "aiModelStatus" || !providers.Columns[0].HideInUpdate {
		t.Fatalf("provider status column = type %q, hide in update %v", providers.Columns[0].Type, providers.Columns[0].HideInUpdate)
	}

	mcp := form.definitions[1].Value
	if !mcp.InlineTable || mcp.SortColumnKey != "Name" {
		t.Fatalf("MCP table options = inline %v, sort %q; want inline and Name", mcp.InlineTable, mcp.SortColumnKey)
	}
	assertFormTableColumnWidths(t, mcp.Columns, []int{100, 50, 80, 80, 100, 160, 120})

	skills := form.definitions[2].Value
	if !skills.InlineTable || skills.SortColumnKey != "Name" || skills.MaxHeight != 360 {
		t.Fatalf("skills table options = inline %v, sort %q, max height %d; want inline, Name, 360", skills.InlineTable, skills.SortColumnKey, skills.MaxHeight)
	}
	assertFormTableColumnWidths(t, skills.Columns[:3], []int{200, 100, 400})
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
