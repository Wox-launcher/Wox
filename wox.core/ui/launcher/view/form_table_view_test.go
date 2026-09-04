package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestFormTableSideTitleMatchesFormLabel(t *testing.T) {
	field := FormTableField(FormTableFieldProps{
		ID: "actions", Title: "Actions", Width: 720, LabelWidth: 80,
		AddLabel: "Add", Theme: woxcomponent.Theme{ActionText: woxui.Color{R: 240, G: 240, B: 240, A: 255}},
	})
	title := field.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Container).Child.(woxwidget.Text)
	if title.Value != "Actions" || title.Style.Size != 13 || title.Style.Weight != woxui.FontWeightRegular || title.Color.R != 240 {
		t.Fatalf("table title = %+v, want the shared 13 regular form label", title)
	}
}

func TestFormTableSideTitleUsesHeaderWeight(t *testing.T) {
	field := FormTableField(FormTableFieldProps{
		ID: "hotkeys", Title: "Query Hotkeys", Width: 720, LabelWidth: 80,
		HeaderWeight: woxui.FontWeightSemibold, AddLabel: "Add", Theme: woxcomponent.Theme{},
	})
	title := field.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Container).Child.(woxwidget.Text)
	if title.Style.Size != 13 || title.Style.Weight != woxui.FontWeightSemibold {
		t.Fatalf("settings side title = %+v, want 13 semibold", title.Style)
	}
}

// translatedLabelHostServices measures CJK runes at full width like a real font,
// which the halved-width stub understates. The Linux language smoke failure came
// from a Chinese label, so a Latin-only approximation hides the regression.
type translatedLabelHostServices struct {
	actionSearchHostServices
}

func (translatedLabelHostServices) MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error) {
	width := float32(0)
	for _, glyph := range text {
		advance := max(style.Size/2, 1)
		if glyph > 0x2E80 {
			advance = max(style.Size, 1)
		}
		width += advance
	}
	return woxui.TextMetrics{Size: woxui.Size{Width: width, Height: max(style.Size, 1)}}, nil
}

// TestFormTableInlineHeaderKeepsTranslatedActionLabelsVisible guards the header
// against a fixed action reserve. A hardcoded width tuned for the English labels
// clipped the leading icon and gap of every longer translation, which only the
// language smoke case caught because English never overflows.
func TestFormTableInlineHeaderKeepsTranslatedActionLabelsVisible(t *testing.T) {
	icon := &woxui.Image{}
	for _, label := range []struct {
		language string
		add      string
		template string
	}{
		{language: "en_US", add: "Add", template: "From Templates"},
		{language: "zh_CN", add: "添加", template: "从模板添加"},
		{language: "ru_RU", add: "Добавить", template: "Из шаблонов"},
		{language: "pt_BR", add: "Adicionar", template: "A partir de modelos"},
		{language: "ko_KR", add: "추가", template: "템플릿에서"},
	} {
		t.Run(label.language, func(t *testing.T) {
			host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
				return FormTableField(FormTableFieldProps{
					ID: "query-hotkeys", Title: "Query Hotkeys", Description: "Open a configured query from a hotkey.",
					Width: 720, InlineTitle: true, MaxHeight: 300,
					Columns:  []FormTableColumn{{Label: "Name", Width: 120}, {Label: "Hotkey"}},
					AddLabel: label.add, SecondaryLabel: label.template, AddIcon: icon, SecondaryIcon: icon,
					OperationLabel: "Operation", EmptyLabel: "No data", Theme: woxcomponent.Theme{},
				})
			})
			host.AttachServices(translatedLabelHostServices{})
			if err := host.SetRepaintDebugMode(woxwidget.RepaintDebugVerify); err != nil {
				t.Fatal(err)
			}
			frame := woxui.FrameInfo{Size: woxui.Size{Width: 720, Height: 400}, PixelSize: woxui.PixelSize{Width: 720, Height: 400}, Scale: 1}
			host.Frame(&woxui.DisplayList{}, frame)
			if diagnostics := host.Snapshot().Diagnostics; len(diagnostics) != 0 {
				t.Fatalf("%s table header diagnostics = %v, want translated action labels to fit", label.language, diagnostics)
			}
		})
	}
}

func formTableOperationIconButton(action woxwidget.Widget) woxcomponent.IconButtonProps {
	stateful := action.(woxwidget.Stateful)
	if props, ok := stateful.Widget.(woxcomponent.IconButtonProps); ok {
		return props
	}
	return stateful.CreateState().Build(woxwidget.StateContext{}, stateful.Widget).(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
}

func TestFormTableDisabledBlocksMutatingActions(t *testing.T) {
	props := FormTableFieldProps{ID: "commands", AddLabel: "Add", Disabled: true, Theme: woxcomponent.Theme{}}
	add := formTableAddButton(props).(woxwidget.Semantics)
	edit := formTableIconButton(props, "commands-row-0-edit", "Edit", nil, props.Disabled, func() {}).(woxwidget.Semantics)
	if !add.Disabled || len(add.Actions) != 0 || !edit.Disabled || len(edit.Actions) != 0 {
		t.Fatal("disabled table must not expose mutating actions")
	}
}

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
	if _, ok := checkbox.Child.(woxwidget.Stateful); !ok {
		t.Fatalf("checkbox control = %T, want shared hoverable checkbox state", checkbox.Child)
	}
	checkbox.OnFocusChange(true)

	props.OnChoiceTap = func(woxui.Rect) {}
	selectControl := formTableRowSelectControl(props, 200, woxcomponent.SettingsControlHeight).(woxwidget.Semantics).Child.(woxwidget.Focusable)
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
	}, 240, woxcomponent.SettingsControlHeight)
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
	}, 420, woxcomponent.SettingsControlHeight).(woxwidget.Flex)
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
	if control.Gap != 10 || control.CrossAxisAlignment != woxwidget.CrossAxisCenter || len(control.Children) != 2 {
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
	button := focusedControlGesture(control.Children[1]).Child.(woxwidget.Container)
	if button.Width != 0 || button.Height != 32 || button.Color != theme.ActionSelected {
		t.Fatal("app selector action should use a content-sized Flutter-style primary button")
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

func TestFormTableRowEditorActionsSizeToTranslatedLabels(t *testing.T) {
	editor := FormTableRowEditor(FormTableRowEditorProps{
		Width: 700, Height: 400, CancelLabel: "Cancel", SaveLabel: "Save", Theme: woxcomponent.Theme{},
	}).(woxwidget.Flex)
	footer := editor.Children[len(editor.Children)-1].(woxwidget.Container)
	if footer.Height != FormTableRowEditorFooterHeight || footer.Padding.Top != SettingsDialogActionsHeight-settingsDialogActionHeight {
		t.Fatalf("row editor footer = height %v padding %+v, want shared action height plus top spacing", footer.Height, footer.Padding)
	}
	actions := footer.Child.(woxwidget.Align)
	if actions.Horizontal != 1 || actions.Width != 700 {
		t.Fatalf("row editor actions alignment = horizontal %v width %v", actions.Horizontal, actions.Width)
	}
	buttons := actions.Child.(woxwidget.Flex)
	if len(buttons.Children) != 2 || buttons.Gap != 12 {
		t.Fatalf("row editor actions = %d gap %.0f", len(buttons.Children), buttons.Gap)
	}
	for _, child := range buttons.Children {
		container := focusedControlGesture(child).Child.(woxwidget.Container)
		if container.Width != 0 {
			t.Fatalf("action button width = %v, want content-sized Cancel/Save like Flutter", container.Width)
		}
	}
}

func TestQueryHotkeyEditorHeaderUsesFourEqualPresets(t *testing.T) {
	selected := ""
	demoKind := ""
	openedLink := ""
	header := QueryHotkeyEditorHeader(QueryHotkeyEditorHeaderProps{
		Width: 700, Title: "Add Query Hotkey", Selected: "web-panel", Description: "Open the launcher. [Learn more](https://example.com)",
		NormalLabel: "Normal", WebPanelLabel: "Preview", SilentLabel: "Silent", CustomLabel: "Custom",
		DemoIcon: &woxui.Image{}, Theme: woxcomponent.Theme{}, OnSelect: func(value string) { selected = value },
		OnOpenLink: func(target string) { openedLink = target },
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
	preview := focusedControlGesture(buttons.Children[1])
	preview.OnTap()
	if selected != "web-panel" {
		t.Fatalf("selected preset = %q", selected)
	}
	for _, button := range buttons.Children {
		content := focusedControlGesture(button).Child.(woxwidget.Container).Child.(woxwidget.Align).Child
		if _, hasTrailing := content.(woxwidget.Flex); hasTrailing {
			t.Fatal("preset button should not contain a demo trigger")
		}
	}
	description := content.Children[2].(woxwidget.Container).Child.(woxwidget.Flex)
	paragraph := description.Children[0].(woxwidget.Wrap)
	link := paragraph.Children[len(paragraph.Children)-2].(woxwidget.Semantics).Child.(woxwidget.Gesture)
	link.OnTap()
	if openedLink != "https://example.com" {
		t.Fatalf("opened link = %q", openedLink)
	}
	previewDemo := paragraph.Children[len(paragraph.Children)-1].(woxwidget.Semantics).Child.(woxwidget.Gesture)
	demoContainer := previewDemo.Child.(woxwidget.Container)
	if demoContainer.Padding.Left != 6 {
		t.Fatalf("demo leading gap = %.0f, want 6", demoContainer.Padding.Left)
	}
	previewDemo.OnHoverAt(true, woxui.Rect{})
	if demoKind != "web-panel" {
		t.Fatalf("demo preset = %q", demoKind)
	}
}

func TestQueryHotkeyEditorHeaderHidesDemoForCustomPreset(t *testing.T) {
	header := QueryHotkeyEditorHeader(QueryHotkeyEditorHeaderProps{
		Width: 700, Selected: "custom", Description: "Tune all options.", DemoIcon: &woxui.Image{}, Theme: woxcomponent.Theme{},
	}).(woxwidget.Container)
	content := header.Child.(woxwidget.Flex)
	description := content.Children[2].(woxwidget.Container).Child.(woxwidget.Flex)
	paragraph := description.Children[0].(woxwidget.Wrap)
	if _, hasDemo := paragraph.Children[len(paragraph.Children)-1].(woxwidget.Semantics); hasDemo {
		t.Fatal("custom preset should not append a demo trigger")
	}
}

func TestFormTableRowDescriptionWrapsLongPlainText(t *testing.T) {
	description := "填写名称、通配规则或绝对路径。不含路径分隔符的规则匹配任意路径片段；相对路径匹配搜索目录内的路径；绝对路径（如 D:\\Games\\Cache 或 /Users/me/Downloads）会排除该文件夹及其内容。"
	width := float32(500)
	labelWidth := float32(80)
	controlWidth := width - labelWidth - 10
	height := FormTableRowFieldHeightFor("textbox", description, "", 1, false, controlWidth)
	singleLine := FormTableRowFieldHeightFor("textbox", "短说明", "", 1, false, controlWidth)
	if height <= singleLine {
		t.Fatalf("wrapped height = %.0f, want more than single-line height %.0f", height, singleLine)
	}

	row := FormTableRowField(FormTableRowFieldProps{
		Kind: "textbox", Description: description, Width: width, Height: height, LabelWidth: labelWidth, MaxLines: 1, Theme: woxcomponent.Theme{},
	}).(woxwidget.Container)
	right := row.Child.(woxwidget.Flex).Children[1].(woxwidget.Flex)
	help := right.Children[1].(woxwidget.TextBlock)
	if help.MaxLines < 2 {
		t.Fatalf("description max lines = %d, want wrapping so the full tip stays visible", help.MaxLines)
	}
	if help.Height < float32(help.MaxLines)*18 {
		t.Fatalf("description height = %.0f, want room for %d wrapped lines", help.Height, help.MaxLines)
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

func TestFormTableHotkeyStatusUsesRemainingControlWidth(t *testing.T) {
	control := formTableRowControl(FormTableRowFieldProps{
		ID: "hotkey", Kind: "hotkey", Recording: true, RecordingStatus: "Press a key", Theme: woxcomponent.Theme{},
	}, 400, 40).(woxwidget.Flex)
	if _, ok := control.Children[1].(woxwidget.Expanded); !ok {
		t.Fatalf("hotkey recording status slot = %T, want Expanded", control.Children[1])
	}
}

func TestFormTableRowMarkdownDescriptionReservesWrappedHeight(t *testing.T) {
	description := "The query when the hotkey is triggered. Type { to insert dynamic variables.\n\nUsing the active browser URL variable requires the Wox Chrome extension: [Install Wox Chrome extension](https://chromewebstore.google.com/detail/wox/bjbkdpjdnagiongdfemjhepkkglnailh)"
	plainHeight := FormTableRowFieldHeight("textbox", description, 1)
	markdownHeight := FormTableRowFieldHeightFor("textbox", description, "", 1, true, 500)
	if markdownHeight <= plainHeight {
		t.Fatalf("markdown height = %.0f, want more than newline-only height %.0f so the tip cannot overlap the next field", markdownHeight, plainHeight)
	}
	row := FormTableRowField(FormTableRowFieldProps{
		Kind: "textbox", Description: description, DescriptionMarkdown: true, Width: 580, Height: markdownHeight, LabelWidth: 80, MaxLines: 1, Theme: woxcomponent.Theme{},
	}).(woxwidget.Container)
	right := row.Child.(woxwidget.Flex).Children[1].(woxwidget.Flex)
	help := right.Children[1].(woxwidget.Container)
	if help.Height < 54 {
		t.Fatalf("markdown help height = %.0f, want room for wrapped chrome-extension tip", help.Height)
	}
	markdown := help.Child.(woxwidget.Flex)
	if markdown.Gap != formTableMarkdownDescriptionGap {
		t.Fatalf("markdown block gap = %.0f, want compact form gap %.0f", markdown.Gap, formTableMarkdownDescriptionGap)
	}
}

func TestFormTableRowEditorUsesFlutterFieldGap(t *testing.T) {
	editor := FormTableRowEditor(FormTableRowEditorProps{
		Width: 500, Height: 320, Title: "Add",
		Rows:          []woxwidget.Widget{woxwidget.Container{Height: 40}, woxwidget.Container{Height: 40}},
		ContentHeight: 400, CancelLabel: "Cancel", SaveLabel: "Save", Theme: woxcomponent.Theme{},
	}).(woxwidget.Flex)
	body := editor.Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	content := body.Content.(woxwidget.Flex)
	if content.Gap != formTableRowFieldGap {
		t.Fatalf("row gap = %.0f, want Flutter bottom padding %.0f", content.Gap, formTableRowFieldGap)
	}
}

func TestFormTableRowFieldRendersInlineValidationError(t *testing.T) {
	errorMessage := "Value cannot be empty"
	height := FormTableRowFieldHeightWithError("textbox", "Website keyword.", errorMessage, 1)
	withoutError := FormTableRowFieldHeight("textbox", "Website keyword.", 1)
	if height <= withoutError {
		t.Fatalf("error height = %.0f, want more than %.0f without error", height, withoutError)
	}
	row := FormTableRowField(FormTableRowFieldProps{
		Kind: "textbox", Label: "Keyword", Description: "Website keyword.", Error: errorMessage,
		Width: 500, Height: height, LabelWidth: 80, MaxLines: 1, Theme: woxcomponent.Theme{ErrorText: woxui.Color{R: 255, A: 255}},
	}).(woxwidget.Container)
	right := row.Child.(woxwidget.Flex).Children[1].(woxwidget.Flex)
	if len(right.Children) != 3 {
		t.Fatalf("right children = %d, want control, description, and error", len(right.Children))
	}
	errorText := right.Children[2].(woxwidget.TextBlock)
	if errorText.Value != errorMessage || errorText.Color.R != 255 {
		t.Fatalf("inline error = %#v", errorText)
	}
}

func formTableGridFlex(t *testing.T, grid woxwidget.Widget) woxwidget.Flex {
	t.Helper()
	frame, ok := grid.(woxwidget.Stack)
	if !ok || len(frame.Children) < 2 {
		t.Fatalf("table grid = %T, want a stacked outer frame", grid)
	}
	body, ok := frame.Children[0].Child.(woxwidget.Flex)
	if !ok {
		t.Fatalf("table grid body = %T, want header/body flex", frame.Children[0].Child)
	}
	return body
}

func formTableHeaderLabel(props FormTableFieldProps) woxwidget.TextBlock {
	cell := formTableHeaderCell(props, FormTableColumn{Label: "Name"}, 120, 0).(woxwidget.Container)
	return cell.Child.(woxwidget.Align).Child.(woxwidget.Flex).Children[0].(woxwidget.TextBlock)
}

func TestFormTableColumnTitleStaysRegular(t *testing.T) {
	regular := formTableHeaderLabel(FormTableFieldProps{ID: "plugin-commands", Theme: woxcomponent.Theme{}})
	if regular.Style.Weight != woxui.FontWeightRegular {
		t.Fatalf("plugin table column title weight = %v, want regular", regular.Style.Weight)
	}

	settings := formTableHeaderLabel(FormTableFieldProps{
		ID: "query-hotkeys", HeaderWeight: woxui.FontWeightSemibold, Theme: woxcomponent.Theme{},
	})
	if settings.Style.Weight != woxui.FontWeightRegular {
		t.Fatalf("settings table column title weight = %v, want regular", settings.Style.Weight)
	}
}

func TestFormTableUsesCollapsedGridLines(t *testing.T) {
	theme := woxcomponent.Theme{
		PreviewSplit: woxui.Color{R: 80, G: 90, B: 100, A: 200},
		ResultTitle:  woxui.Color{R: 240, G: 240, B: 240, A: 255},
	}
	props := FormTableFieldProps{
		ID: "hotkeys", Width: 400, Height: tableSurfaceHeaderHeight + tableSurfaceRowHeight*2,
		Columns: []FormTableColumn{{Label: "Name", Width: 120}, {Label: "Hotkey", Width: 120}},
		Rows: []FormTableRow{
			{Index: 0, Cells: []FormTableCell{{Text: "one"}, {Text: "ctrl"}}},
			{Index: 1, Cells: []FormTableCell{{Text: "two"}, {Text: "alt"}}},
		},
		Theme: theme,
	}

	frame := buildFormTableGrid(props, props.Width, props.Height, newFormTableGridState()).(woxwidget.Stack)
	if len(frame.Children) != 2 {
		t.Fatalf("table frame children = %d, want content plus one outer stroke", len(frame.Children))
	}
	outline := frame.Children[1].Child.(woxwidget.Container)
	if outline.BorderWidth != tableSurfaceBorderWidth || outline.BorderColor != theme.PreviewSplit || outline.Color.A != 0 {
		t.Fatalf("outer table stroke = %#v, want a single 1px PreviewSplit frame", outline)
	}

	headerCell := formTableHeaderCell(props, props.Columns[0], 130, 0).(woxwidget.Container)
	if headerCell.BorderWidth != 0 || headerCell.RightBorderWidth != tableSurfaceBorderWidth || headerCell.BottomBorderWidth != tableSurfaceBorderWidth {
		t.Fatalf("header separator = full %.0f right %.0f bottom %.0f, want collapsed right+bottom", headerCell.BorderWidth, headerCell.RightBorderWidth, headerCell.BottomBorderWidth)
	}
	operationHeader := formTableHeaderCell(props, FormTableColumn{Label: "Operation"}, 130, len(props.Columns)).(woxwidget.Container)
	if operationHeader.RightBorderWidth != 0 || operationHeader.BottomBorderWidth != tableSurfaceBorderWidth {
		t.Fatalf("last header separator = right %.0f bottom %.0f, want bottom only", operationHeader.RightBorderWidth, operationHeader.BottomBorderWidth)
	}

	firstBody := formTableDataCellAt(props, 0, 0, props.Rows[0].Cells[0], 130, false).(woxwidget.Container)
	if firstBody.BorderWidth != 0 || firstBody.RightBorderWidth != tableSurfaceBorderWidth || firstBody.BottomBorderWidth != tableSurfaceBorderWidth {
		t.Fatalf("body separator = full %.0f right %.0f bottom %.0f, want collapsed right+bottom", firstBody.BorderWidth, firstBody.RightBorderWidth, firstBody.BottomBorderWidth)
	}
	lastBody := formTableDataCellAt(props, 1, 1, props.Rows[1].Cells[1], 130, true).(woxwidget.Container)
	if lastBody.RightBorderWidth != tableSurfaceBorderWidth || lastBody.BottomBorderWidth != 0 {
		t.Fatalf("last data separator = right %.0f bottom %.0f, want trailing only before the operation column", lastBody.RightBorderWidth, lastBody.BottomBorderWidth)
	}
	lastOperation := formTableOperationCell(props, props.Rows[1], 130, true).(woxwidget.Container)
	if lastOperation.BorderWidth != 0 || lastOperation.RightBorderWidth != 0 || lastOperation.BottomBorderWidth != 0 {
		t.Fatalf("last operation separator = %#v, want the outer frame to own that corner", lastOperation)
	}

	empty := formTableEmptyState(FormTableFieldProps{EmptyLabel: "None", Theme: theme}, 240, tableSurfaceEmptyHeight).(woxwidget.Container)
	if empty.BorderWidth != 0 || empty.RightBorderWidth != 0 || empty.BottomBorderWidth != 0 {
		t.Fatalf("empty state = %#v, want fill only so it does not double the header or frame", empty)
	}
	emptyLabel := empty.Child.(woxwidget.Align).Child.(woxwidget.Flex).Children[1].(woxwidget.Align).Child.(woxwidget.Text)
	if emptyLabel.Color != newTableSurfaceStyle(theme).headerText {
		t.Fatalf("empty label = %#v, want table header text so ResultSubtitle cannot restyle it", emptyLabel.Color)
	}
}

func TestFormTableScrollbarUsesValueText(t *testing.T) {
	title := woxui.Color{R: 240, G: 244, B: 248, A: 255}
	props := FormTableFieldProps{
		ID: "hotkeys", Width: 400, Height: tableSurfaceHeaderHeight + tableSurfaceRowHeight,
		Columns: []FormTableColumn{{Label: "Name", Width: 120}, {Label: "Hotkey", Width: 120}},
		Rows: []FormTableRow{
			{Index: 0, Cells: []FormTableCell{{Text: "one"}, {Text: "ctrl"}}},
			{Index: 1, Cells: []FormTableCell{{Text: "two"}, {Text: "alt"}}},
		},
		Theme: woxcomponent.Theme{ResultTitle: title, ResultSubtitle: woxui.Color{R: 255, A: 255}},
	}
	body := formTableGridFlex(t, buildFormTableGrid(props, props.Width, props.Height, newFormTableGridState())).Children[1]
	scroll := body.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	if scroll.ThumbColor != title {
		t.Fatalf("table scrollbar = %#v, want ResultTitle so ResultSubtitle cannot restyle it", scroll.ThumbColor)
	}
}
