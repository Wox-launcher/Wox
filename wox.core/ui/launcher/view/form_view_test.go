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
	if control.Height != woxcomponent.SettingsControlHeight || control.BorderWidth != 1 {
		t.Fatalf("model trigger geometry = height %.0f border %.0f, want standard %.0f/1", control.Height, control.BorderWidth, woxcomponent.SettingsControlHeight)
	}
	if gesture.OnTap != nil || gesture.OnTapBounds == nil {
		t.Fatal("model trigger should open a field-anchored dropdown")
	}
	anchor := woxui.Rect{X: 300, Y: 140, Width: 580, Height: woxcomponent.SettingsControlHeight}
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
	recorder := control.Children[0].Child.(woxwidget.Align)
	if control.Children[0].Left != 0 || control.Children[0].Top != 0 || recorder.Height != woxcomponent.SettingsControlHeight || recorder.Vertical != 0.5 {
		t.Fatalf("hotkey recorder alignment = position %#v child %#v", control.Children[0], recorder)
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

func TestFormHotkeyFieldShowsRegistrationErrorWhenNotRecording(t *testing.T) {
	field := FormHotkeyField(FormHotkeyFieldProps{
		ID: "onboarding-hotkey", Label: "Hotkey", Description: "Show or hide Wox", Labels: []string{"Alt", "Space"},
		Status: "Used by another application", Error: true, Width: 720, Height: 62, LabelWidth: 132, AlignRecorderRight: true, Theme: woxcomponent.Theme{},
	})
	container := field.(woxwidget.Container)
	row := container.Child.(woxwidget.Flex)
	controlColumn := row.Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex)
	control := controlColumn.Children[0].(woxwidget.Stack)
	if len(control.Children) != 2 {
		t.Fatalf("hotkey control children = %d, want recorder and registration error", len(control.Children))
	}
	status := control.Children[1].Child.(woxwidget.Align).Child.(woxwidget.Clip).Child.(woxwidget.Text)
	if status.Value != "Used by another application" {
		t.Fatalf("hotkey registration error = %q", status.Value)
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
	hintAlignment := hint.Child.(woxwidget.Align)
	if hint.Left != -582 || hint.Top != 0 || hintAlignment.Height != woxcomponent.SettingsControlHeight || hintAlignment.Vertical != 0.5 {
		t.Fatalf("settings hint geometry = position %#v child %#v", hint, hintAlignment)
	}
	hintClip := hintAlignment.Child.(woxwidget.Clip)
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

func TestFormTextFieldBrowseButtonSharesOneControlRow(t *testing.T) {
	field := FormTextField(FormTextFieldProps{
		ID: "cwd", Label: "Directory", Width: 400, LabelWidth: 100,
		OnBrowse: func() {}, BrowseLabel: "Browse",
		Theme: woxcomponent.Theme{ResultSubtitle: woxui.Color{A: 190}, ResultTitle: woxui.Color{A: 255}},
	})
	row := field.(woxwidget.Container).Child.(woxwidget.Flex)
	controlColumn := row.Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex)
	valueRow := controlColumn.Children[0].(woxwidget.Flex)
	if valueRow.Axis != woxwidget.Horizontal {
		t.Fatalf("browse layout axis = %v, want one horizontal control row", valueRow.Axis)
	}
	input := valueRow.Children[0].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	browse := valueRow.Children[1].(woxwidget.Semantics)
	if browse.Label != "Browse" || browse.AutomationID != "cwd-browse" {
		t.Fatalf("browse button = %+v, want outline Browse", browse)
	}
	stateful := browse.Child.(woxwidget.Focusable).Child.(woxwidget.Stateful)
	container := stateful.CreateState().Build(woxwidget.StateContext{}, stateful.Widget).(woxwidget.Gesture).Child.(woxwidget.Container)
	browseWidth := formBrowseButtonWidth("Browse")
	if input.Width+8+container.Width != 288 || container.Width != browseWidth {
		t.Fatalf("dirPath input/browse widths = %.0f/%.0f, want one full control row of 288", input.Width, container.Width)
	}
	if container.BorderWidth != 1 {
		t.Fatalf("browse border width = %v, want outline", container.BorderWidth)
	}
}

func TestFormStatsFieldUsesQuietCardLayout(t *testing.T) {
	theme := woxcomponent.Theme{
		QueryBackground: woxui.Color{R: 40, G: 40, B: 44, A: 255},
		PreviewSplit:    woxui.Color{R: 80, G: 80, B: 84, A: 255},
		ResultTitle:     woxui.Color{R: 250, G: 250, B: 250, A: 255},
		ResultSubtitle:  woxui.Color{R: 160, G: 160, B: 164, A: 255},
	}
	field := FormStatsField(FormStatsFieldProps{
		Width: 420, Title: "Index Stats",
		Rows:  []FormStatsRow{{Label: "Disk Usage", Value: "29.4 MB"}, {Label: "Files", Value: "130,945"}},
		Theme: theme,
	})
	semantics := field.(woxwidget.Semantics)
	if semantics.Role != woxui.AccessibilityRoleGroup || semantics.Label != "Index Stats" {
		t.Fatalf("stats semantics = role %q label %q", semantics.Role, semantics.Label)
	}
	wrapper := semantics.Child.(woxwidget.Container)
	if wrapper.Padding.Top != 16 || wrapper.Padding.Bottom != 12 {
		t.Fatalf("stats outer padding = %+v, want 16/12 section spacing", wrapper.Padding)
	}
	card := wrapper.Child.(woxwidget.Container)
	if card.Radius != 8 || card.BorderWidth != 1 || card.Color != theme.QueryBackground || card.BorderColor != theme.PreviewSplit {
		t.Fatalf("stats card chrome = radius %.0f border %.0f fill %#v stroke %#v", card.Radius, card.BorderWidth, card.Color, card.BorderColor)
	}
	column := card.Child.(woxwidget.Flex)
	title := column.Children[0].(woxwidget.Text)
	if title.Value != "Index Stats" || title.Style.Size != woxcomponent.SettingsLabelFontSize || title.Style.Weight != woxui.FontWeightSemibold {
		t.Fatalf("stats title = %+v", title)
	}
	rows := column.Children[1].(woxwidget.Flex)
	if rows.Gap != 8 || len(rows.Children) != 2 {
		t.Fatalf("stats rows = gap %.0f count %d", rows.Gap, len(rows.Children))
	}
	first := rows.Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
	if first.Children[0].(woxwidget.Text).Value != "Disk Usage" || first.Children[2].(woxwidget.Text).Value != "29.4 MB" {
		t.Fatalf("first stats row = %#v", first.Children)
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
