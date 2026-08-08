package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestFormTableRowNonTextControlsExposeControlledFocus(t *testing.T) {
	focused := 0
	props := FormTableRowFieldProps{
		ID: "field", Label: "Field", Focused: true, Theme: woxcomponent.Theme{},
		OnFocus: func() { focused++ }, OnKey: func(woxui.KeyEvent) bool { return true }, OnTap: func() {},
	}

	checkboxSemantics := formTableRowCheckboxControl(props).(woxwidget.Semantics)
	if checkboxSemantics.AutomationID != "field" || checkboxSemantics.Role != woxui.AccessibilityRoleCheckBox || len(checkboxSemantics.Actions) != 1 || checkboxSemantics.Actions[0] != woxui.AccessibilityActionToggle {
		t.Fatal("checkbox should expose its controlled value to accessibility and automation")
	}
	checkbox := checkboxSemantics.Child.(woxwidget.Focusable)
	if !checkbox.Autofocus || checkbox.OnKey == nil || checkbox.OnFocusChange == nil {
		t.Fatal("checkbox should expose the controlled table-row focus contract")
	}
	checkbox.OnFocusChange(true)

	props.OnChoiceTap = func(woxui.Rect) {}
	selectControl := formTableRowSelectControl(props, 200, 34).(woxwidget.Semantics).Child.(woxwidget.Focusable)
	if !selectControl.Autofocus || selectControl.OnKey == nil || selectControl.OnFocusChange == nil {
		t.Fatal("select should expose the controlled table-row focus contract")
	}
	selectControl.OnFocusChange(true)

	if focused != 2 {
		t.Fatalf("focus callbacks = %d, want both controls to synchronize logical focus", focused)
	}
}

func TestFormTableRowTextControlLeavesCaretFocusToHost(t *testing.T) {
	changes := []bool{}
	control := formTableRowTextControl(FormTableRowFieldProps{
		ID: "field", Focused: true, State: woxui.TextEditingState{}, Theme: woxcomponent.Theme{},
		OnFocusChange: func(focused bool) { changes = append(changes, focused) },
	}, 240, 34)
	field := control.(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	if field.Focused {
		t.Fatal("table row state must not control the retained text field caret")
	}
	field.OnFocusChange(true)
	field.OnFocusChange(false)
	if len(changes) != 2 || !changes[0] || changes[1] {
		t.Fatalf("focus changes = %v, want Host transitions forwarded unchanged", changes)
	}
}

func TestFormTableRowTextControlPlacesActionBesideInput(t *testing.T) {
	tapped := false
	control := formTableRowTextControl(FormTableRowFieldProps{
		ID: "query", State: woxui.TextEditingState{Text: "ai translate {wox:selected_text}"}, Theme: woxcomponent.Theme{},
		ActionIcon: &woxui.Image{}, ActionLabel: "Test this query", OnActionTap: func() { tapped = true },
	}, 420, 34).(woxwidget.Flex)
	if control.Gap != 8 || len(control.Children) != 2 {
		t.Fatalf("query action layout = children %d gap %.0f, want input plus outside action icon", len(control.Children), control.Gap)
	}
	input := control.Children[0].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	if input.Width != 378 {
		t.Fatalf("input width = %.0f, want room reserved for the outside action icon", input.Width)
	}
	action := control.Children[1].(woxwidget.Semantics)
	if action.AutomationID != "query-action" || action.Label != "Test this query" || action.Role != woxui.AccessibilityRoleButton {
		t.Fatalf("action semantics = %#v", action)
	}
	if err := action.OnAction(woxui.AccessibilityActionActivate, ""); err != nil || !tapped {
		t.Fatalf("action activate err = %v tapped = %v", err, tapped)
	}
}

func TestFormTableRowAppControlMatchesFlutterSelectorLayout(t *testing.T) {
	theme := woxcomponent.Theme{
		ActionSelected: woxui.Color{R: 20, G: 80, B: 140, A: 255},
		ResultSubtitle: woxui.Color{R: 100, G: 110, B: 120, A: 255},
	}
	control := formTableRowAppControl(FormTableRowFieldProps{
		ID: "app", Value: "No app selected", SelectLabel: "Select Apps", SelectWidth: 104, Theme: theme, OnTap: func() {},
	}, 420, 42).(woxwidget.Flex)
	if control.Gap != 10 || len(control.Children) != 2 {
		t.Fatal("app selector should keep Flutter's preview and primary button split layout")
	}
	preview := control.Children[0].(woxwidget.Container)
	if preview.Width != 306 || preview.Radius != 4 || preview.BorderColor.A != 115 {
		t.Fatalf("preview geometry = width %.0f radius %.0f alpha %d", preview.Width, preview.Radius, preview.BorderColor.A)
	}
	emptyText := preview.Child.(woxwidget.Flex).Children[0].(woxwidget.Align)
	if emptyText.Height != 42 || emptyText.Vertical != 0.5 {
		t.Fatalf("empty app alignment = height %.0f vertical %.1f, want full-height center", emptyText.Height, emptyText.Vertical)
	}
	buttonFocus := control.Children[1].(woxwidget.Semantics).Child.(woxwidget.Focusable)
	if buttonFocus.OnFocusChange == nil {
		t.Fatal("app selector button should keep table-row focus synchronized")
	}
	button := buttonFocus.Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	if button.Width != 104 || button.Height != 42 || button.Color != theme.ActionSelected {
		t.Fatal("app selector action should use the Flutter-style primary button")
	}
	selected := formTableRowAppControl(FormTableRowFieldProps{
		ID: "selected-app", Value: "Lightroom Classic", Detail: "/Applications/Lightroom Classic.app", Image: &woxui.Image{},
		SelectLabel: "Select Apps", SelectWidth: 104, Theme: theme, OnTap: func() {},
	}, 420, 42).(woxwidget.Flex)
	selectedPreview := selected.Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
	if len(selectedPreview.Children) != 2 {
		t.Fatalf("selected app preview children = %d, want icon and name only", len(selectedPreview.Children))
	}
	selectedText := selectedPreview.Children[1].(woxwidget.Align)
	name := selectedText.Child.(woxwidget.TextBlock)
	if selectedText.Height != 42 || selectedText.Vertical != 0.5 || name.Value != "Lightroom Classic" || name.Height < name.LineHeight {
		t.Fatal("selected app should render only its vertically centered name")
	}
}

func TestQueryHotkeyEditorHeaderUsesFourEqualPresets(t *testing.T) {
	selected := ""
	demoKind := ""
	header := QueryHotkeyEditorHeader(QueryHotkeyEditorHeaderProps{
		Width: 700, Title: "Add Query Hotkey", Selected: "normal", Description: "Open the launcher.",
		NormalLabel: "Normal", WebPanelLabel: "Preview", SilentLabel: "Silent", CustomLabel: "Custom",
		DemoIcon: &woxui.Image{}, Theme: woxcomponent.Theme{}, OnSelect: func(value string) { selected = value },
		OnDemoHover: func(value string, inside bool, _ woxui.Rect) {
			if inside {
				demoKind = value
			}
		},
	}).(woxwidget.Container)
	content := header.Child.(woxwidget.Flex)
	buttons := content.Children[1].(woxwidget.Flex)
	if len(buttons.Children) != 4 || buttons.Gap != 8 {
		t.Fatalf("preset buttons = %d gap %.0f", len(buttons.Children), buttons.Gap)
	}
	preview := buttons.Children[1].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture)
	preview.OnTap()
	if selected != "web-panel" {
		t.Fatalf("selected preset = %q", selected)
	}
	previewContent := preview.Child.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex)
	previewDemo := previewContent.Children[1].(woxwidget.Semantics).Child.(woxwidget.Gesture)
	previewDemo.OnHoverAt(true, woxui.Rect{})
	if demoKind != "web-panel" {
		t.Fatalf("demo preset = %q", demoKind)
	}
	custom := buttons.Children[3].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture)
	if _, hasDemo := custom.Child.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex); hasDemo {
		t.Fatal("custom preset should not expose a demo trigger")
	}
}

func TestFormTableRowDescriptionPreservesFlutterParagraphs(t *testing.T) {
	description := "Type { to insert variables.\n\nInstall the browser extension."
	height := FormTableRowFieldHeight("textbox", description, 1)
	row := FormTableRowField(FormTableRowFieldProps{Kind: "textbox", Description: description, Width: 500, Height: height, LabelWidth: 80, MaxLines: 1, Theme: woxcomponent.Theme{}}).(woxwidget.Container)
	right := row.Child.(woxwidget.Flex).Children[1].(woxwidget.Flex)
	help := right.Children[1].(woxwidget.TextBlock)
	if help.MaxLines != 3 || help.Height != 54 {
		t.Fatalf("description lines = %d height %.0f", help.MaxLines, help.Height)
	}
}
