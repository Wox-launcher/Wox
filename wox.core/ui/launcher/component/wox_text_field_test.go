package component

import (
	"strings"
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestSingleLineTextFieldDefaultsToVerticalCenter(t *testing.T) {
	singleLine := WoxTextField(TextFieldProps{ID: "single"}).(woxwidget.Stateful).Widget.(TextFieldProps)
	if singleLine.TextAlignmentY != 0.5 {
		t.Fatalf("single-line vertical alignment = %v, want 0.5", singleLine.TextAlignmentY)
	}

	multiline := WoxTextField(TextFieldProps{ID: "multi", MaxLines: 2}).(woxwidget.Stateful).Widget.(TextFieldProps)
	if multiline.TextAlignmentY != 0 {
		t.Fatalf("multiline vertical alignment = %v, want 0", multiline.TextAlignmentY)
	}
}

func TestTextFieldHoverUsesSharedOverlayAndSkipsDisabledFields(t *testing.T) {
	base := woxui.Color{R: 20, G: 30, B: 40, A: 255}
	foreground := woxui.Color{R: 220, G: 230, B: 240, A: 255}
	build := func(disabled, disableHover, focused bool) woxwidget.Gesture {
		field := WoxTextField(TextFieldProps{ID: "hover", Width: 200, Height: 40, Background: base, Disabled: disabled, DisableHover: disableHover, Theme: Theme{ResultTitle: foreground}}).(woxwidget.Stateful)
		state := field.CreateState().(*textFieldState)
		state.InitState(woxwidget.StateContext{}, field.Widget)
		state.hovered = true
		state.focusNode.UpdateFocus(focused)
		return state.Build(woxwidget.StateContext{}, field.Widget).(woxwidget.EditableText).Child.(woxwidget.Gesture)
	}

	hovered := build(false, false, false)
	if hovered.OnHoverAt == nil || hovered.Child.(woxwidget.Container).Color != controlHoverColor(base, foreground) {
		t.Fatal("enabled text field did not expose the shared hover treatment")
	}
	disabled := build(true, false, false)
	if disabled.OnHoverAt != nil || disabled.Child.(woxwidget.Container).Color != base {
		t.Fatal("disabled text field should not react to hover")
	}
	optedOut := build(false, true, false)
	if optedOut.OnHoverAt != nil || optedOut.Child.(woxwidget.Container).Color != base {
		t.Fatal("text field with hover disabled should not react to hover")
	}
	focused := build(false, false, true)
	if focused.Child.(woxwidget.Container).Color != base {
		t.Fatal("focused text field should not draw its hover treatment")
	}
}

func TestTextFieldScrolledOffsetConsumesOnlyMovableWheelDeltas(t *testing.T) {
	if next, changed := textFieldScrolledOffset(0, 240, -40); next != 40 || !changed {
		t.Fatalf("scroll down = %.0f, %v, want 40, true", next, changed)
	}
	if next, changed := textFieldScrolledOffset(240, 240, -20); next != 240 || changed {
		t.Fatalf("scroll at bottom = %.0f, %v, want 240, false", next, changed)
	}
	if next, changed := textFieldScrolledOffset(40, 240, 20); next != 20 || !changed {
		t.Fatalf("scroll up = %.0f, %v, want 20, true", next, changed)
	}
}

func TestMultilineTextFieldOwnsStandardEditingShortcutsBeforeParent(t *testing.T) {
	controller := woxwidget.NewTextEditingController("alpha\nbeta")
	controller.SetCaret(5)
	controller.InsertText(" changed")
	parentCalls := 0
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return WoxTextField(TextFieldProps{
			ID: "multi-shortcuts", Width: 200, Height: 80, MaxLines: 4, Controller: controller,
			OnKey: func(woxui.KeyEvent) bool {
				parentCalls++
				return true
			},
		})
	})
	host.AttachServices(&hotkeyRecorderHostServices{})
	displayList := &woxui.DisplayList{}
	host.Frame(displayList, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 80}, PixelSize: woxui.PixelSize{Width: 200, Height: 80}, Scale: 1})
	host.RequestFocus("multi-shortcuts")
	primary := woxui.KeyModifierControl | woxui.KeyModifierMeta

	if !host.Key(woxui.KeyEvent{Key: woxui.Key("a"), Modifiers: primary, Down: true}) || controller.SelectedText() != controller.Text() {
		t.Fatalf("Ctrl+A selection = %q, want all text", controller.SelectedText())
	}
	if !host.Key(woxui.KeyEvent{Key: woxui.Key("z"), Modifiers: primary, Down: true}) || controller.Text() != "alpha\nbeta" {
		t.Fatalf("Ctrl+Z text = %q, want original multiline value", controller.Text())
	}
	word := woxui.KeyModifierControl
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyBackspace, Modifiers: word, Down: true}) || controller.Text() != "\nbeta" {
		t.Fatalf("word+Backspace text = %q, want previous word deleted", controller.Text())
	}
	if parentCalls != 0 {
		t.Fatalf("parent shortcut handler called %d times, want standard editing shortcuts retained by the field", parentCalls)
	}
}

func TestProtectedTextFieldRejectsCopyAndCut(t *testing.T) {
	controller := woxwidget.NewTextEditingController("secret")
	controller.SelectAll()
	provider := &memoryClipboard{}
	SetClipboardProvider(provider)
	t.Cleanup(func() { SetClipboardProvider(nil) })

	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return WoxTextField(TextFieldProps{ID: "password", Width: 200, Height: 40, Protected: true, Controller: controller})
	})
	host.AttachServices(&hotkeyRecorderHostServices{})
	displayList := &woxui.DisplayList{}
	host.Frame(displayList, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 40}, PixelSize: woxui.PixelSize{Width: 200, Height: 40}, Scale: 1})
	host.RequestFocus("password")
	primary := woxui.KeyModifierControl | woxui.KeyModifierMeta

	if !host.Key(woxui.KeyEvent{Key: woxui.Key("c"), Modifiers: primary, Down: true}) {
		t.Fatal("Ctrl+C should be handled on protected fields")
	}
	if provider.text != "" {
		t.Fatalf("protected copy wrote %q", provider.text)
	}
	if !host.Key(woxui.KeyEvent{Key: woxui.Key("x"), Modifiers: primary, Down: true}) || controller.Text() != "secret" {
		t.Fatalf("protected cut changed text to %q", controller.Text())
	}
	if provider.text != "" {
		t.Fatalf("protected cut wrote %q", provider.text)
	}
}

func TestSingleLineTextFieldFiltersNewlinesOnPaste(t *testing.T) {
	controller := woxwidget.NewTextEditingController("")
	provider := &memoryClipboard{text: "a\r\nb\nc"}
	SetClipboardProvider(provider)
	t.Cleanup(func() { SetClipboardProvider(nil) })

	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return WoxTextField(TextFieldProps{ID: "single-line", Width: 200, Height: 40, Controller: controller})
	})
	host.AttachServices(&hotkeyRecorderHostServices{})
	displayList := &woxui.DisplayList{}
	host.Frame(displayList, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 40}, PixelSize: woxui.PixelSize{Width: 200, Height: 40}, Scale: 1})
	host.RequestFocus("single-line")
	primary := woxui.KeyModifierControl | woxui.KeyModifierMeta
	if !host.Key(woxui.KeyEvent{Key: woxui.Key("v"), Modifiers: primary, Down: true}) || controller.Text() != "abc" {
		t.Fatalf("paste = %q, want abc", controller.Text())
	}
}

func TestReadOnlyTextFieldAllowsSelectAndCopy(t *testing.T) {
	controller := woxwidget.NewTextEditingController("readable")
	provider := &memoryClipboard{}
	SetClipboardProvider(provider)
	t.Cleanup(func() { SetClipboardProvider(nil) })

	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return WoxTextField(TextFieldProps{ID: "readonly", Width: 200, Height: 40, ReadOnly: true, Controller: controller})
	})
	host.AttachServices(&hotkeyRecorderHostServices{})
	displayList := &woxui.DisplayList{}
	host.Frame(displayList, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 40}, PixelSize: woxui.PixelSize{Width: 200, Height: 40}, Scale: 1})
	host.RequestFocus("readonly")
	primary := woxui.KeyModifierControl | woxui.KeyModifierMeta
	if !host.Key(woxui.KeyEvent{Key: woxui.Key("a"), Modifiers: primary, Down: true}) || controller.SelectedText() != "readable" {
		t.Fatalf("readonly select all = %q", controller.SelectedText())
	}
	if !host.Key(woxui.KeyEvent{Key: woxui.Key("c"), Modifiers: primary, Down: true}) || provider.text != "readable" {
		t.Fatalf("readonly copy = %q", provider.text)
	}
	if host.Key(woxui.KeyEvent{Key: woxui.Key("v"), Modifiers: primary, Down: true}) && controller.Text() != "readable" {
		t.Fatalf("readonly paste mutated text to %q", controller.Text())
	}
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyArrowLeft, Modifiers: woxui.KeyModifierShift, Down: true}) {
		t.Fatal("readonly shift+left should adjust selection")
	}
	if controller.State().Selection.Collapsed() {
		t.Fatal("readonly shift+left should keep a non-empty selection from select-all")
	}
}

func TestTextFieldLinesSoftWrapPreservesOffsets(t *testing.T) {
	lines := textFieldLines("hello world", nil, woxui.TextStyle{Size: 12}, 0, true)
	if len(lines) != 1 || lines[0].text != "hello world" {
		t.Fatalf("without window soft wrap should stay hard-only, got %#v", lines)
	}
	hard := textFieldLines("a\nb", nil, woxui.TextStyle{}, 0, false)
	if len(hard) != 2 || hard[0].text != "a" || hard[1].text != "b" {
		t.Fatalf("hard lines = %#v", hard)
	}
}

func TestTextFieldLinesSoftWrapUsesGraphemeClusters(t *testing.T) {
	measurer := &fakeTextMeasurer{charWidth: 10}
	style := woxui.TextStyle{Size: 12}

	// Width 70 fits "hello w" so the break prefers the space after "hello ".
	spaceWrapped := textFieldLines("hello world", measurer, style, 70, true)
	if len(spaceWrapped) != 2 || spaceWrapped[0].text != "hello " || spaceWrapped[1].text != "world" {
		t.Fatalf("space wrap = %#v", spaceWrapped)
	}
	if spaceWrapped[0].start != 0 || spaceWrapped[0].end != 6 || spaceWrapped[1].start != 6 || spaceWrapped[1].end != 11 {
		t.Fatalf("space wrap offsets = %#v", spaceWrapped)
	}

	cjk := textFieldLines("中文测试内容", measurer, style, 30, true)
	if len(cjk) < 2 {
		t.Fatalf("cjk wrap expected multiple lines, got %#v", cjk)
	}
	for _, line := range cjk {
		if line.end < line.start {
			t.Fatalf("invalid offsets %#v", line)
		}
		if string([]rune("中文测试内容")[line.start:line.end]) != line.text {
			t.Fatalf("line text/offset mismatch %#v", line)
		}
	}

	emoji := "✈️✈️✈️"
	emojiLines := textFieldLines(emoji, measurer, style, 25, true)
	if len(emojiLines) < 2 {
		t.Fatalf("emoji wrap expected multiple lines, got %#v", emojiLines)
	}
	for _, line := range emojiLines {
		spans := woxui.GraphemeSpans(line.text)
		joined := ""
		for _, span := range spans {
			joined += span.Text
		}
		if joined != line.text {
			t.Fatalf("emoji line split a grapheme: %#v", line)
		}
		runes := []rune(emoji)
		if line.text != string(runes[line.start:line.end]) {
			t.Fatalf("emoji offsets mismatch %#v", line)
		}
	}

	longWord := textFieldLines("supercalifragilistic", measurer, style, 40, true)
	if len(longWord) < 2 {
		t.Fatalf("long word wrap expected multiple lines, got %#v", longWord)
	}
	narrow := textFieldLines("ab", measurer, style, 5, true)
	if len(narrow) != 2 || narrow[0].text != "a" || narrow[1].text != "b" {
		t.Fatalf("narrow wrap = %#v", narrow)
	}
}

func (m *fakeTextMeasurer) MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error) {
	width := m.charWidth
	if width <= 0 {
		width = 10
	}
	return woxui.TextMetrics{Size: woxui.Size{Width: float32(len([]rune(text))) * width, Height: style.Size}}, nil
}

func TestEditableTextExposesSelectionSemantics(t *testing.T) {
	controller := woxwidget.NewTextEditingController("hello")
	controller.SetSelection(1, 4)
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return WoxTextField(TextFieldProps{ID: "semantic", Width: 200, Height: 40, Controller: controller})
	})
	host.AttachServices(&hotkeyRecorderHostServices{})
	displayList := &woxui.DisplayList{}
	host.Frame(displayList, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 40}, PixelSize: woxui.PixelSize{Width: 200, Height: 40}, Scale: 1})
	tree := host.Snapshot().Tree
	var found bool
	for _, node := range tree.Nodes {
		if node.AutomationID != "semantic" {
			continue
		}
		found = true
		if !node.HasTextSelection || node.SelectionStart != 1 || node.SelectionEnd != 4 {
			t.Fatalf("selection semantics = %#v", node)
		}
		if !containsAccessibilityAction(node.Actions, woxui.AccessibilityActionSelectAll) || !containsAccessibilityAction(node.Actions, woxui.AccessibilityActionCopy) {
			t.Fatalf("actions = %v", node.Actions)
		}
	}
	if !found {
		t.Fatal("semantic text field node missing")
	}
}

func TestMultilineTextFieldHomeEndUsesVisualLine(t *testing.T) {
	controller := woxwidget.NewTextEditingController("alpha\nbeta")
	controller.SetCaret(8)
	lines := textFieldLines(controller.Text(), nil, woxui.TextStyle{}, 0, false)
	handled, changed := handleTextFieldControllerKey(controller, 4, lines, 80, nil, woxui.TextStyle{}, 0, false, woxui.KeyEvent{Key: woxui.KeyHome, Down: true})
	if !handled || changed || controller.State().Selection.Focus != 6 {
		t.Fatalf("visual home = handled %v changed %v focus %d", handled, changed, controller.State().Selection.Focus)
	}
	handled, changed = handleTextFieldControllerKey(controller, 4, lines, 80, nil, woxui.TextStyle{}, 0, false, woxui.KeyEvent{Key: woxui.KeyEnd, Down: true})
	if !handled || changed || controller.State().Selection.Focus != 10 {
		t.Fatalf("visual end = handled %v changed %v focus %d", handled, changed, controller.State().Selection.Focus)
	}
}

func TestTextFieldContextMenuUsesHostOverlay(t *testing.T) {
	controller := woxwidget.NewTextEditingController("hello")
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return WoxTextField(TextFieldProps{ID: "menu-field", Width: 200, Height: 40, Controller: controller})
	})
	host.AttachServices(&hotkeyRecorderHostServices{})
	displayList := &woxui.DisplayList{}
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 40}, PixelSize: woxui.PixelSize{Width: 200, Height: 40}, Scale: 1}
	host.Frame(displayList, frame)
	host.RequestFocus("menu-field")
	host.Pointer(woxui.PointerEvent{
		Kind: woxui.PointerDown, Button: woxui.PointerButtonSecondary,
		Position: woxui.Point{X: 24, Y: 20},
	})
	host.Frame(displayList, frame)
	if !host.HasOverlay() {
		t.Fatal("secondary tap should open a host overlay context menu")
	}
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) {
		t.Fatal("escape should dismiss the context menu")
	}
	host.Frame(displayList, frame)
	if host.HasOverlay() {
		t.Fatal("escape should clear the host overlay")
	}
}

func TestPreferredColumnSurvivesShortLines(t *testing.T) {
	controller := woxwidget.NewTextEditingController("abcdef\nab\nxyzxyz")
	controller.SetCaret(5)
	lines := textFieldLines(controller.Text(), nil, woxui.TextStyle{}, 0, false)
	handled, changed := handleTextFieldControllerKey(controller, 8, lines, 80, nil, woxui.TextStyle{}, 0, false, woxui.KeyEvent{Key: woxui.KeyArrowDown, Down: true})
	if !handled || changed {
		t.Fatalf("arrow down = handled %v changed %v", handled, changed)
	}
	if focus := controller.State().Selection.Focus; focus != len([]rune("abcdef\nab")) {
		t.Fatalf("focus on short line = %d, want end of second line", focus)
	}
	handled, changed = handleTextFieldControllerKey(controller, 8, textFieldLines(controller.Text(), nil, woxui.TextStyle{}, 0, false), 80, nil, woxui.TextStyle{}, 0, false, woxui.KeyEvent{Key: woxui.KeyArrowDown, Down: true})
	if !handled || changed {
		t.Fatalf("second arrow down = handled %v changed %v", handled, changed)
	}
	if focus := controller.State().Selection.Focus; focus != len([]rune("abcdef\nab\nxyzxy")) {
		t.Fatalf("preferred column restore = %d, want column 5 on third line", focus)
	}
}

type memoryClipboard struct {
	text string
}

func (m *memoryClipboard) ReadText() (string, error) { return m.text, nil }
func (m *memoryClipboard) WriteText(text string) error {
	m.text = text
	return nil
}

type fakeTextMeasurer struct {
	charWidth float32
}

func containsAccessibilityAction(actions []woxui.AccessibilityAction, want woxui.AccessibilityAction) bool {
	for _, action := range actions {
		if action == want {
			return true
		}
	}
	return false
}

func TestFilterHelperUsedBySingleLineContract(t *testing.T) {
	if got := woxui.FilterSingleLineNewlines("keep\nme"); got != "keepme" {
		t.Fatalf("filter = %q", got)
	}
	got := woxui.FilterSingleLineNewlines("ok")
	if strings.Contains(got, "\n") {
		t.Fatalf("unexpected newline in %q", got)
	}
}

func TestTextFieldCompositionPreservesAndShiftsRichDecorations(t *testing.T) {
	checkbox := TextFieldRichRun{Start: 0, End: 1, Checkbox: true}
	following := TextFieldRichRun{Start: 5, End: 6, Checkbox: true}
	state := woxui.TextEditingState{Text: "☐ abc☐", Selection: woxui.TextSelection{Anchor: 2, Focus: 2}, Composition: "中文"}
	runs := textFieldCompositionRichRuns(state, []TextFieldRichRun{checkbox, following})
	if len(runs) != 2 || !runs[0].Checkbox || runs[0].Start != 0 || runs[0].End != 1 {
		t.Fatalf("checkbox before composition changed: %#v", runs)
	}
	if !runs[1].Checkbox || runs[1].Start != 7 || runs[1].End != 8 {
		t.Fatalf("checkbox after composition was not shifted: %#v", runs)
	}
}
