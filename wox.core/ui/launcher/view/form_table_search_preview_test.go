package view

import (
	"strings"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestFormTableSearchKeepsSourceIndexesAndFullText(t *testing.T) {
	props := FormTableFieldProps{
		SearchColumnKey: "Pattern",
		Columns:         []FormTableColumn{{Key: "Name"}, {Key: "Pattern"}},
		Rows: []FormTableRow{
			{Index: 7, Cells: []FormTableCell{{Text: "first"}, {Text: "short…", SearchText: strings.Repeat("a", 90) + "needle"}}},
			{Index: 2, Cells: []FormTableCell{{Text: "needle"}, {Text: "other"}}},
		},
	}
	filtered := filterFormTableRows(props, "needle")
	if len(filtered.Rows) != 1 || filtered.Rows[0].Index != 7 || filtered.Columns[0].Key != "Pattern" || filtered.Rows[0].Cells[0].Text != "short…" {
		t.Fatalf("filtered table = %+v", filtered)
	}
	if props.Columns[0].Key != "Name" || props.Rows[0].Cells[0].Text != "first" {
		t.Fatal("filter mutated source props")
	}
}

func TestFormTableSearchStateIsScopedToPlugin(t *testing.T) {
	first := FormTableField(FormTableFieldProps{ID: "table", StateKey: "plugin-a:rules", EnableSearch: true}).(woxwidget.Stateful)
	second := FormTableField(FormTableFieldProps{ID: "table", StateKey: "plugin-b:rules", EnableSearch: true}).(woxwidget.Stateful)
	if first.Key == second.Key {
		t.Fatal("plugin tables share retained search state")
	}
}

func TestFormTableCellIconsScopeTooltipAndPreserveText(t *testing.T) {
	var tooltip string
	props := FormTableFieldProps{ID: "table-a", OnTooltip: func(_ bool, text string, _ woxui.Rect) { tooltip = text }}
	icons := formTableCellIcons(props, 2, 1, FormTableCell{Icons: []FormTableCellIcon{{Tooltip: "/apps/notes"}}}).(woxwidget.Flex)
	trigger := icons.Children[0].(woxwidget.Gesture)
	trigger.OnHoverAt(true, woxui.Rect{})
	if trigger.ID != "table-a-row-2-cell-1-icon-0" || tooltip != "/apps/notes" {
		t.Fatalf("tooltip target = %+v, text = %q", trigger, tooltip)
	}
}

func TestFormTableSearchIconAppearsBeforeAdd(t *testing.T) {
	props := FormTableFieldProps{
		ID: "ignore-rules", Title: "Ignore rules", Width: 720, InlineTitle: true,
		EnableSearch: true, SearchLabel: "Search", AddLabel: "Add",
		SearchIcon: &woxui.Image{}, Theme: woxcomponent.Theme{},
	}
	actions := formTableHeaderActions(props).(woxwidget.Flex)
	if len(actions.Children) != 2 {
		t.Fatalf("header actions = %d, want search then add", len(actions.Children))
	}
	search := actions.Children[0].(woxwidget.Stateful)
	if string(search.Key) != "ignore-rules-search" {
		t.Fatalf("search button key = %q", search.Key)
	}
}

func TestFormTableSearchFieldAppearsWhenOpen(t *testing.T) {
	field := formTableField(FormTableFieldProps{
		ID: "ignore-rules", Title: "Ignore rules", Width: 720, InlineTitle: true,
		EnableSearch: true, SearchOpen: true, SearchPlaceholder: "Filter...",
		SearchIcon: &woxui.Image{}, AddLabel: "Add", Theme: woxcomponent.Theme{},
	})
	children := field.(woxwidget.Container).Child.(woxwidget.Flex).Children
	if len(children) < 3 {
		t.Fatalf("inline children = %d, want header, search, grid", len(children))
	}
	search := children[1].(woxwidget.Container)
	if search.Height != woxcomponent.SettingsSearchHeight {
		t.Fatalf("search height = %v, want %v", search.Height, woxcomponent.SettingsSearchHeight)
	}
}

func TestFormTablePatternPreviewDefaultsCheckboxesOn(t *testing.T) {
	props := FormTablePatternPreviewProps{
		Width: 600, Theme: woxcomponent.Theme{},
		OnToggle: func(string, bool) {},
	}
	row := formTablePatternPreviewRow(props, FormTablePatternPreviewApp{Key: "path:chrome", Name: "Chrome", Checked: true}, 600)
	checkbox := row.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Align).Child.(woxwidget.Semantics)
	if checkbox.Role != woxui.AccessibilityRoleCheckBox || !checkbox.Checked || checkbox.Label != "Chrome" {
		t.Fatalf("preview checkbox = %+v", checkbox)
	}
}

func TestFormTablePatternPreviewPutsTitleOnTheLeft(t *testing.T) {
	preview := FormTablePatternPreview(FormTablePatternPreviewProps{
		Width: 600, LabelWidth: 140, Title: "Apps",
		EmptyLabel: "No indexed apps match this pattern", Theme: woxcomponent.Theme{},
	})
	row := preview.(woxwidget.Container).Child.(woxwidget.Flex)
	if row.Axis != woxwidget.Horizontal || len(row.Children) != 2 {
		t.Fatalf("preview row = %+v, want label then list", row)
	}
	label := row.Children[0].(woxwidget.Container).Child.(woxwidget.TextBlock)
	if label.Value != "Apps" {
		t.Fatalf("label = %q", label.Value)
	}
	list := row.Children[1].(woxwidget.Flex).Children[1].(woxwidget.Container)
	empty := list.Child.(woxwidget.Align).Child.(woxwidget.Text)
	if empty.Value != "No indexed apps match this pattern" {
		t.Fatalf("empty = %q", empty.Value)
	}
}

func TestFormTableRowMatchesSearch(t *testing.T) {
	if !formTableRowMatchesSearch([]string{"Chrome.lnk", "C:\\Desktop"}, "chrm", false) {
		t.Fatal("expected fuzzy match")
	}
	if formTableRowMatchesSearch([]string{"Notes"}, "chrome", false) {
		t.Fatal("unexpected match")
	}
	if !formTableRowMatchesSearch([]string{"卸载程序"}, "xiezai", true) {
		t.Fatal("expected pinyin match when enabled")
	}
	if formTableRowMatchesSearch([]string{"卸载程序"}, "xiezai", false) {
		t.Fatal("pinyin should stay off when the setting is disabled")
	}
}
