package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestFormPanelUsesIntrinsicHeightUpToMaximum(t *testing.T) {
	panel := FormPanel(FormPanelProps{
		Width: 388, MaximumHeight: 420, Rows: []woxwidget.Widget{woxwidget.Container{Width: 360, Height: 44}},
		CancelLabel: "Cancel (Esc)", SaveLabel: "Save (Cmd+Enter)", Theme: woxcomponent.Theme{},
	}).(woxwidget.Container)
	if panel.Height != 0 {
		t.Fatalf("form panel height = %.0f, want intrinsic height", panel.Height)
	}
	column := panel.Child.(woxwidget.Flex)
	if len(column.Children) != 2 {
		t.Fatalf("form panel child count = %d, want body and actions without a title row", len(column.Children))
	}
	body := column.Children[0].(woxwidget.ScrollView)
	if body.Width != 360 || body.Height != 0 || body.MaxHeight != 354 {
		t.Fatalf("form body constraints = width %.0f height %.0f max %.0f, want Flutter 360 wide and intrinsic up to 354", body.Width, body.Height, body.MaxHeight)
	}
	buttons := column.Children[1].(woxwidget.Align).Child.(woxwidget.Flex)
	cancel := buttons.Children[0].(woxwidget.Semantics)
	save := buttons.Children[1].(woxwidget.Semantics)
	if cancel.Label != "Cancel (Esc)" || save.Label != "Save (Cmd+Enter)" {
		t.Fatalf("form action labels = %q / %q", cancel.Label, save.Label)
	}
	for _, child := range buttons.Children {
		container := focusedControlGesture(child).Child.(woxwidget.Container)
		if container.Width != 0 {
			t.Fatalf("form action width = %v, want content-sized", container.Width)
		}
	}
}

func TestFormSwitchFieldUsesAccessibleSwitch(t *testing.T) {
	field := FormSwitchField(FormSwitchFieldProps{ID: "history", Label: "History", Width: 400, Height: 40, LabelWidth: 100, Checked: true, Theme: woxcomponent.Theme{}, OnChange: func(bool) {}})
	row := field.(woxwidget.Container).Child.(woxwidget.Flex)
	controlColumn := row.Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex)
	control := controlColumn.Children[0].(woxwidget.Semantics)
	if control.Role != woxui.AccessibilityRoleCheckBox || !control.Checked {
		t.Fatalf("switch semantics = role %q checked %v", control.Role, control.Checked)
	}
}

func TestFormSelectFieldUsesOutlinedDropdown(t *testing.T) {
	field := FormSelectField(FormSelectFieldProps{ID: "action", Label: "Action", Value: "Paste", Width: 400, Height: 44, LabelWidth: 100, Theme: woxcomponent.Theme{}})
	row := field.(woxwidget.Container).Child.(woxwidget.Flex)
	controlColumn := row.Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex)
	semantics := controlColumn.Children[0].(woxwidget.Semantics)
	gesture := focusedControlGesture(semantics)
	control := gesture.Child.(woxwidget.Container)
	if control.BorderWidth != 1 {
		t.Fatalf("dropdown border width = %.0f, want 1", control.BorderWidth)
	}
	content := control.Child.(woxwidget.Flex)
	if len(content.Children) != 2 {
		t.Fatalf("dropdown child count = %d, want value and indicator", len(content.Children))
	}
}

func TestFormFieldNaturalHeightMeasuresWrappedDescription(t *testing.T) {
	field := FormSelectField(FormSelectFieldProps{
		ID: "action", Label: "Action", Description: "A description that may wrap onto multiple lines",
		Value: "Paste", Width: 240, LabelWidth: 100, Theme: woxcomponent.Theme{},
	})
	container := field.(woxwidget.Container)
	if container.Height != 0 || container.Padding.Bottom != 10 {
		t.Fatalf("natural row geometry = height %.0f bottom %.0f, want intrinsic height with 10px spacing", container.Height, container.Padding.Bottom)
	}
	row := container.Child.(woxwidget.Flex)
	controlColumn := row.Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex)
	description := controlColumn.Children[1].(woxwidget.TextBlock)
	if description.Height != 0 || description.MaxLines != 0 {
		t.Fatalf("natural description limits = height %.0f lines %d, want measured wrapped content", description.Height, description.MaxLines)
	}
}

func TestFormModelFieldUsesCompactAnchoredDropdown(t *testing.T) {
	var openedAt woxui.Rect
	field := FormModelField(FormModelFieldProps{
		ID: "dictation-model", Label: "Recognition model", Value: "Qwen3-ASR 0.6B",
		Width: 720, Height: 44, LabelWidth: 120, Theme: woxcomponent.Theme{}, OnTap: func(anchor woxui.Rect) { openedAt = anchor },
	})
	row := field.(woxwidget.Container).Child.(woxwidget.Flex)
	controlColumn := row.Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex)
	semantics := controlColumn.Children[0].(woxwidget.Semantics)
	gesture := focusedControlGesture(semantics)
	control := gesture.Child.(woxwidget.Container)
	if control.Height != 34 || control.BorderWidth != 1 {
		t.Fatalf("model trigger geometry = height %.0f border %.0f, want Flutter 34/1", control.Height, control.BorderWidth)
	}
	if gesture.OnTap != nil || gesture.OnTapBounds == nil {
		t.Fatal("model trigger should open a field-anchored dropdown")
	}
	anchor := woxui.Rect{X: 300, Y: 140, Width: 580, Height: 34}
	gesture.OnTapBounds(anchor)
	if openedAt != anchor {
		t.Fatalf("model menu anchor = %#v, want %#v", openedAt, anchor)
	}
}

func TestFormHotkeyFieldStartsAtMeasuredControlColumn(t *testing.T) {
	field := FormHotkeyField(FormHotkeyFieldProps{
		ID: "dictation-hotkey", Label: "Hotkey", Description: "Press or hold a modifier",
		Width: 720, Height: 64, LabelWidth: 120, Theme: woxcomponent.Theme{},
	})
	container := field.(woxwidget.Container)
	if container.Height != 64 {
		t.Fatalf("hotkey row height = %.0f, want tooltip content plus bottom spacing at 64", container.Height)
	}
	row := container.Child.(woxwidget.Flex)
	if row.Gap != 12 {
		t.Fatalf("hotkey label gap = %.0f, want Flutter 12", row.Gap)
	}
	label := row.Children[0].(woxwidget.Container)
	if label.Width != 120 {
		t.Fatalf("hotkey label width = %.0f, want measured width 120", label.Width)
	}
	controlColumn := row.Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex)
	control := controlColumn.Children[0].(woxwidget.Stack)
	if control.Children[0].Left != 0 {
		t.Fatalf("hotkey recorder left = %.0f, want start of control column", control.Children[0].Left)
	}
	description := controlColumn.Children[1].(woxwidget.TextBlock)
	if description.Value != "Press or hold a modifier" {
		t.Fatalf("hotkey description = %q", description.Value)
	}
}

func TestFormHotkeyFieldCanAlignRecorderToTheRightOfItsControlColumn(t *testing.T) {
	field := FormHotkeyField(FormHotkeyFieldProps{
		ID: "onboarding-hotkey", Label: "Hotkey", Description: "Show or hide Wox", Labels: []string{"Alt", "Space"},
		Width: 720, Height: 62, LabelWidth: 132, AlignRecorderRight: true, Theme: woxcomponent.Theme{},
	})
	container := field.(woxwidget.Container)
	row := container.Child.(woxwidget.Flex)
	controlColumn := row.Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex)
	control := controlColumn.Children[0].(woxwidget.Stack)
	if !control.Children[0].AnchorRight || control.Children[0].Right != 0 {
		t.Fatalf("hotkey recorder geometry = %#v, want right-anchored control", control.Children[0])
	}
	description := controlColumn.Children[1].(woxwidget.TextBlock)
	if description.Value != "Show or hide Wox" {
		t.Fatalf("hotkey description = %q, want description below right control", description.Value)
	}
}

func TestFormHotkeyFieldUsesFlutterSettingsLayout(t *testing.T) {
	field := FormHotkeyField(FormHotkeyFieldProps{
		ID: "main-hotkey", Label: "Hotkey", Description: "Show or hide Wox",
		Width: 1120, LabelWidth: 550, SettingsLayout: true, Recording: true, Status: "Press any key", Theme: woxcomponent.Theme{},
	})
	container := field.(woxwidget.Container)
	row := container.Child.(woxwidget.Flex)
	label := row.Children[0].(woxwidget.Container)
	if label.Width != 550 || row.Gap != 32 {
		t.Fatalf("settings label geometry = width %.0f gap %.0f, want Flutter 550/32", label.Width, row.Gap)
	}
	labelColumn := label.Child.(woxwidget.Flex)
	description := labelColumn.Children[1].(woxwidget.Text)
	if description.Value != "Show or hide Wox" {
		t.Fatalf("settings description = %q, want it below the label", description.Value)
	}
	controlArea := row.Children[1].(woxwidget.Stack)
	if controlArea.Width != 534 || !controlArea.Children[0].AnchorRight || controlArea.Children[0].Right != 2 {
		t.Fatalf("settings recorder geometry = width %.0f right anchored %v inset %.0f, want 534/true/2", controlArea.Width, controlArea.Children[0].AnchorRight, controlArea.Children[0].Right)
	}
	hint := controlArea.Children[1]
	if hint.Left != -582 {
		t.Fatalf("settings hint left = %.0f, want Flutter-style overflow to page edge at -582", hint.Left)
	}
	hintClip := hint.Child.(woxwidget.Clip)
	if hintClip.Width != 1006 {
		t.Fatalf("settings hint clip width = %.0f, want the same 12px gap used by right-side hints", hintClip.Width)
	}
}

func TestFormHotkeyFieldShrinksSettingsLabelToKeepRecorderVisible(t *testing.T) {
	field := FormHotkeyField(FormHotkeyFieldProps{
		ID: "main-hotkey", Label: "Hotkey", Description: "Show or hide Wox", Labels: []string{"Super", "Space"},
		Width: 676, LabelWidth: 550, SettingsLayout: true, Theme: woxcomponent.Theme{},
	})
	container := field.(woxwidget.Container)
	row := container.Child.(woxwidget.Flex)
	label := row.Children[0].(woxwidget.Container)
	controlArea := row.Children[1].(woxwidget.Stack)
	if label.Width != 360 || controlArea.Width != 280 {
		t.Fatalf("narrow settings label/control widths = %.0f/%.0f, want 360/280", label.Width, controlArea.Width)
	}
	if !controlArea.Children[0].AnchorRight || controlArea.Children[0].Right != 2 {
		t.Fatalf("narrow settings recorder geometry = %#v, want right anchored with 2px inset", controlArea.Children[0])
	}
}

func TestFormAIModelFieldUsesFlutterProviderAndModelProportions(t *testing.T) {
	field := FormAIModelField(FormAIModelFieldProps{
		ID: "default-model", Label: "Default model", Provider: "deepseek", Model: "deepseek-v4-flash",
		ModelsAvailable: true, Width: 920, Height: 44, LabelWidth: 180, Theme: woxcomponent.Theme{},
	})
	stateful := field.(woxwidget.Stateful)
	state := &formAIModelFieldState{}
	state.InitState(woxwidget.StateContext{}, stateful.Widget)
	built := state.Build(woxwidget.StateContext{}, stateful.Widget).(woxwidget.Container)
	row := built.Child.(woxwidget.Flex)
	controlColumn := row.Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex)
	controls := controlColumn.Children[0].(woxwidget.Flex)
	if len(controls.Children) != 3 {
		t.Fatalf("AI model control count = %d, want provider, model, and edit", len(controls.Children))
	}
	provider := focusedControlGesture(controls.Children[0]).Child.(woxwidget.Container)
	model := focusedControlGesture(controls.Children[1]).Child.(woxwidget.Container)
	if model.Width < provider.Width*1.9 || model.Width > provider.Width*2.1 {
		t.Fatalf("AI model widths provider/model = %.0f/%.0f, want Flutter 1:2 proportions", provider.Width, model.Width)
	}
	if controls.Gap != 8 {
		t.Fatalf("AI model selector gap = %.0f, want 8", controls.Gap)
	}
}

func TestFormTextFieldKeepsSuffixOutsideInput(t *testing.T) {
	field := FormTextField(FormTextFieldProps{ID: "days", Label: "Days", Suffix: "天", Width: 400, Height: 44, LabelWidth: 100, MaxLines: 1, Theme: woxcomponent.Theme{}})
	row := field.(woxwidget.Container).Child.(woxwidget.Flex)
	controlColumn := row.Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex)
	valueRow := controlColumn.Children[0].(woxwidget.Flex)
	suffix := valueRow.Children[1].(woxwidget.Align).Child.(woxwidget.Text)
	if suffix.Value != "天" {
		t.Fatalf("suffix = %q, want 天", suffix.Value)
	}
}

func TestFormTextFieldUsesMeasuredActionLabelWidth(t *testing.T) {
	field := FormTextField(FormTextFieldProps{ID: "content", Label: "内容", Width: 360, LabelWidth: 60, MaxLines: 8, Theme: woxcomponent.Theme{}})
	row := field.(woxwidget.Container).Child.(woxwidget.Flex)
	label := row.Children[0].(woxwidget.Container)
	controlColumn := row.Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex)
	input := controlColumn.Children[0].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	if label.Width != 60 || input.Width != 288 {
		t.Fatalf("form label/input widths = %.0f/%.0f, want measured 60 and expanded 288", label.Width, input.Width)
	}
}
