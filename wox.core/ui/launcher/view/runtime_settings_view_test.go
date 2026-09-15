package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestRuntimeStatusColumnsFitThreeCardsOnDefaultPage(t *testing.T) {
	width := runtimeStatusThreeColumnMinWidth()
	if width != 800 {
		t.Fatalf("default runtime content width = %.0f, want 800", width)
	}
	if runtimeStatusColumns(width) != 3 {
		t.Fatalf("default runtime columns = %d, want 3 so Node.js, Python, and Script share one row", runtimeStatusColumns(width))
	}
	statuses := []RuntimeStatus{{Runtime: "NODEJS"}, {Runtime: "PYTHON"}, {Runtime: "SCRIPT"}}
	if got := runtimeStatusGridHeight(statuses, width); got != 168 {
		t.Fatalf("default runtime grid height = %.0f, want one 168-high row", got)
	}
	grid := runtimeStatusGrid(RuntimeSettingsProps{Statuses: statuses}, width, 168).(woxwidget.Grid)
	if grid.Columns != 3 || grid.CellWidth != (width-24)/3 {
		t.Fatalf("default runtime grid = %d columns at %.0f, want 3 narrower cards", grid.Columns, grid.CellWidth)
	}
	if runtimeStatusColumns(width-1) != 2 {
		t.Fatalf("runtime columns below the default page = %d, want 2", runtimeStatusColumns(width-1))
	}
}

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
		content := page.Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps).Content.(woxwidget.Container).Child.(woxwidget.Flex)
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
	pill := title.Children[1].(woxwidget.Flex).Children[0].(woxwidget.Container)
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

// TestRuntimeRunningHostOffersRestart verifies a healthy host can apply a saved path without restarting Wox.
func TestRuntimeRunningHostOffersRestart(t *testing.T) {
	calls := 0
	status := RuntimeStatus{Runtime: "NODEJS", StatusCode: "running", RestartLabel: "Restart", OnRestart: func() { calls++ }}
	if height := runtimeStatusGridHeight([]RuntimeStatus{status}, 800); height != 168 {
		t.Fatalf("running host card height = %v, want compact running card", height)
	}
	for _, props := range []RuntimeSettingsProps{{}, {Editing: true}, {Saving: true}, {Restarting: true}, {Refreshing: true}} {
		tooltip := ""
		props.OnTooltip = func(inside bool, text string, _ woxui.Rect) {
			if inside {
				tooltip = text
			} else {
				tooltip = ""
			}
		}
		card := runtimeStatusCard(props, status, 360, 168).(woxwidget.Container)
		column := card.Child.(woxwidget.Flex)
		header := column.Children[0].(woxwidget.Flex)
		title := header.Children[1].(woxwidget.Expanded).Child.(woxwidget.Container).Child.(woxwidget.Flex)
		row := title.Children[1].(woxwidget.Flex)
		button := row.Children[1].(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
		if button.ID != "runtime-restart-NODEJS" {
			t.Fatalf("unexpected action: %s", button.ID)
		}
		disabled := props.Editing || props.Saving || props.Restarting || props.Refreshing
		if button.Disabled != disabled {
			t.Fatalf("restart disabled = %v, want %v", button.Disabled, disabled)
		}
		button.OnHoverAt(true, woxui.Rect{})
		if tooltip != "Restart" {
			t.Fatalf("tooltip = %q", tooltip)
		}
		if !button.Disabled {
			button.OnTap()
			if tooltip != "" {
				t.Fatal("restart should dismiss tooltip")
			}
		}
	}
	if calls != 1 {
		t.Fatalf("restart calls = %d, want only idle action enabled", calls)
	}
}

// TestRuntimePathEditorOffersSave verifies editing replaces Clear with a keyboard-accessible save action.
func TestRuntimePathEditorOffersSave(t *testing.T) {
	saves, clears := 0, 0
	var idleInputWidth float32
	for _, editing := range []bool{false, true} {
		row := runtimeExecutableSettingRow(RuntimeSettingsProps{Labels: RuntimeSettingsLabels{Save: "Save", Clear: "Clear"}}, RuntimeSettingRow{
			ID: "node", Focused: editing, OnSave: func() { saves++ }, OnClear: func() { clears++ },
		}, 800, 72).(woxwidget.Gesture)
		field := row.Child.(woxwidget.Container).Child.(woxwidget.Container)
		controls := field.Child.(woxwidget.Flex).Children[1].(woxwidget.Flex)
		input := controls.Children[0].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
		if !editing {
			idleInputWidth = input.Width
		} else if input.Width != idleInputWidth {
			t.Fatalf("editing changed input width from %v to %v", idleInputWidth, input.Width)
		}
		button := controls.Children[2].(woxwidget.Semantics)
		expected := "node-clear"
		if editing {
			expected = "node-save"
		}
		if button.AutomationID != expected {
			t.Fatalf("action = %s, want %s", button.AutomationID, expected)
		}
		button.Child.(woxwidget.Focusable).OnKey(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true})
	}
	if saves != 1 || clears != 1 {
		t.Fatalf("actions: save=%d clear=%d", saves, clears)
	}
}
