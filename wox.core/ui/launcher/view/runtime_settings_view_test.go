package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxwidget "wox/ui/widget"
)

func TestRuntimeLabelWidthIncludesButtonPadding(t *testing.T) {
	if width := runtimeLabelWidth("浏览", 62, 96); width != 66 {
		t.Fatalf("runtime label width = %v, want 66", width)
	}
}

func TestRuntimeExecutableSettingUsesAlignedSettingsTextField(t *testing.T) {
	row := runtimeExecutableSettingRow(RuntimeSettingsProps{}, RuntimeSettingRow{ID: "python", Title: "Python"}, 800, 72).(woxwidget.Gesture)
	target := row.Child.(woxwidget.Container)
	field := target.Child.(woxwidget.Container)
	controls := field.Child.(woxwidget.Flex).Children[1].(woxwidget.Flex)
	input := controls.Children[0].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	if input.TextAlignmentY != 0.5 {
		t.Fatalf("runtime input vertical alignment = %v, want 0.5", input.TextAlignmentY)
	}
}

func TestRuntimeLoadingDoesNotAddStatusText(t *testing.T) {
	for _, statuses := range [][]RuntimeStatus{nil, {{Runtime: "PYTHON"}}} {
		page := buildRuntimeSettingsView(RuntimeSettingsProps{Width: 1000, Height: 700, Loading: true, Statuses: statuses}).(woxwidget.Container)
		content := page.Child.(woxwidget.ScrollView).Child.(woxwidget.Flex)
		if len(content.Children) != 6 {
			t.Fatalf("runtime page children while loading = %d, want 6 without a loading message", len(content.Children))
		}
	}
}

func TestRuntimeStatusCardUsesRemainingHeaderWidth(t *testing.T) {
	card := runtimeStatusCard(RuntimeSettingsProps{}, RuntimeStatus{DisplayName: "Python", Version: "3.13"}, 360, 168).(woxwidget.Container)
	column := card.Child.(woxwidget.Flex)
	header := column.Children[0].(woxwidget.Flex)
	title := header.Children[1].(woxwidget.Expanded).Child.(woxwidget.Container).Child.(woxwidget.Flex)
	name := title.Children[0].(woxwidget.Flex).Children[0]
	if _, ok := name.(woxwidget.Expanded); !ok {
		t.Fatalf("runtime name slot = %T, want Expanded", name)
	}
}

func TestRuntimeStatusPillCentersLabel(t *testing.T) {
	card := runtimeStatusCard(RuntimeSettingsProps{}, RuntimeStatus{DisplayName: "Node.js", StatusLabel: "运行中"}, 360, 168).(woxwidget.Container)
	column := card.Child.(woxwidget.Flex)
	header := column.Children[0].(woxwidget.Flex)
	title := header.Children[1].(woxwidget.Expanded).Child.(woxwidget.Container).Child.(woxwidget.Flex)
	pill := title.Children[1].(woxwidget.Container)
	align := pill.Child.(woxwidget.Align)
	if pill.Padding.Left != 8 || pill.Padding.Right != 8 {
		t.Fatalf("status pill padding = %+v, want symmetric 8px insets", pill.Padding)
	}
	if align.Horizontal != 0.5 || align.Vertical != 0.5 || align.Width != pill.Width-16 {
		t.Fatalf("status pill alignment = %#v, want centered in the pill", align)
	}
}

func TestRuntimeStatusCardShowsRefreshNextToInstall(t *testing.T) {
	card := runtimeStatusCard(RuntimeSettingsProps{}, RuntimeStatus{
		Runtime: "PYTHON", DisplayName: "Python", Actionable: true,
		InstallLabel: "Install Python", OnInstall: func() {},
		RefreshLabel: "Refresh", OnRefresh: func() {},
	}, 360, 224).(woxwidget.Container)
	ids := runtimeCardButtonIDs(card)
	if len(ids) != 2 || ids[0] != "runtime-install-PYTHON" || ids[1] != "runtime-refresh-PYTHON" {
		t.Fatalf("runtime card buttons = %v, want install then refresh", ids)
	}
}

func TestRuntimeStatusCardOmitsRefreshWhenNotProvided(t *testing.T) {
	card := runtimeStatusCard(RuntimeSettingsProps{}, RuntimeStatus{
		Runtime: "PYTHON", DisplayName: "Python", Actionable: true,
		InstallLabel: "Install Python", OnInstall: func() {},
	}, 360, 224).(woxwidget.Container)
	ids := runtimeCardButtonIDs(card)
	if len(ids) != 1 || ids[0] != "runtime-install-PYTHON" {
		t.Fatalf("runtime card buttons = %v, want install only", ids)
	}
}

func runtimeCardButtonIDs(card woxwidget.Container) []string {
	column := card.Child.(woxwidget.Flex)
	row := column.Children[len(column.Children)-1].(woxwidget.Container).Child.(woxwidget.Flex)
	ids := make([]string, 0, len(row.Children))
	for _, child := range row.Children {
		if semantics, ok := child.(woxwidget.Semantics); ok {
			ids = append(ids, semantics.AutomationID)
		}
	}
	return ids
}
