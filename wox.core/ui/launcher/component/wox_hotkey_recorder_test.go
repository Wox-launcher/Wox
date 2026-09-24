package component

import (
	"runtime"
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxHotkeyRecorderNormalDensityKeepsAuthoredKeycaps(t *testing.T) {
	for _, scale := range []float32{0, 1} {
		recorder, width := WoxHotkeyRecorder(HotkeyRecorderProps{
			ID: "hotkey", Labels: []string{"Alt", "Space"}, Theme: ControlTheme{DensityScale: scale},
		})
		container := hotkeyRecorderBox(recorder)
		padding := woxwidget.Insets{Left: 8, Top: 4, Right: 8, Bottom: 4}
		if container.Height != 30 || container.Padding != padding || container.Radius != 4 || container.BorderWidth != 1 {
			t.Fatalf("scale %v outline = height %.0f padding %+v radius %.0f border %.0f", scale, container.Height, container.Padding, container.Radius, container.BorderWidth)
		}
		chip := container.Child.(woxwidget.Container)
		row := chip.Child.(woxwidget.Align).Child.(woxwidget.Flex)
		first := row.Children[0].(woxwidget.Stack)
		label := first.Children[2].Child.(woxwidget.Align).Child.(woxwidget.Text)
		if chip.Height != 22 || first.Height != 22 || label.Style.Size != TailFontSize || label.Style.Weight != woxui.FontWeightSemibold {
			t.Fatalf("scale %v keycap = height %.0f/%.0f width %.0f size %.0f weight %v", scale, chip.Height, first.Height, first.Width, label.Style.Size, label.Style.Weight)
		}
		second := row.Children[1].(woxwidget.Stack)
		if second.Width != 47 || row.Gap != 4 || width != container.Width || width != first.Width+second.Width+row.Gap+16 {
			t.Fatalf("scale %v space key = width %.0f gap %.0f reported %.0f", scale, second.Width, row.Gap, width)
		}
	}
}

func TestWoxHotkeyRecorderFollowsInterfaceSize(t *testing.T) {
	comfortable, _ := WoxHotkeyRecorder(HotkeyRecorderProps{
		ID: "hotkey", Labels: []string{"Alt", "Space"}, Theme: ControlTheme{DensityScale: 1.1},
	})
	box := hotkeyRecorderBox(comfortable)
	chip := box.Child.(woxwidget.Container)
	row := chip.Child.(woxwidget.Align).Child.(woxwidget.Flex)
	label := row.Children[0].(woxwidget.Stack).Children[2].Child.(woxwidget.Align).Child.(woxwidget.Text)
	if box.Height != 32 || box.Padding.Left != 9 || box.Padding.Top != 4 || chip.Height != 24 || label.Style.Size != 12 || row.Gap != 4 {
		t.Fatalf("comfortable recorder = outline %.0f pad %+v key %.0f font %.0f gap %.0f", box.Height, box.Padding, chip.Height, label.Style.Size, row.Gap)
	}

	compact, compactWidth := WoxHotkeyRecorder(HotkeyRecorderProps{
		ID: "hotkey", Labels: []string{"J"}, Theme: ControlTheme{DensityScale: 0.9},
	})
	box = hotkeyRecorderBox(compact)
	chip = box.Child.(woxwidget.Container)
	key := chip.Child.(woxwidget.Align).Child.(woxwidget.Flex).Children[0].(woxwidget.Stack)
	label = key.Children[2].Child.(woxwidget.Align).Child.(woxwidget.Text)
	if box.Height != 28 || box.Padding.Left != 7 || chip.Height != 20 || key.Width != 25 || label.Style.Size != 10 || compactWidth != key.Width+14 {
		t.Fatalf("compact recorder = outline %.0f pad %.0f key %.0f/%.0f font %.0f width %.0f", box.Height, box.Padding.Left, chip.Height, key.Width, label.Style.Size, compactWidth)
	}

	placeholder, placeholderWidth := WoxHotkeyRecorder(HotkeyRecorderProps{
		ID: "hotkey", Placeholder: "Record", Theme: ControlTheme{DensityScale: 1},
	})
	box = hotkeyRecorderBox(placeholder)
	text := box.Child.(woxwidget.Align).Child.(woxwidget.Text)
	if box.Height != 30 || box.Child.(woxwidget.Align).Height != 22 || text.Style.Size != SettingsControlFontSize || placeholderWidth != 96 {
		t.Fatalf("normal placeholder = height %.0f/%.0f size %.0f width %.0f", box.Height, box.Child.(woxwidget.Align).Height, text.Style.Size, placeholderWidth)
	}
}

func hotkeyRecorderBox(recorder woxwidget.Widget) woxwidget.Container {
	return buildHotkeyRecorderForTest(recorder).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
}

func TestWoxHotkeyRecorderRendersHoldModifierAsText(t *testing.T) {
	recorder, _ := WoxHotkeyRecorder(HotkeyRecorderProps{
		ID: "hotkey", Labels: []string{"Cmd"}, Hold: true, HoldPrefix: "Hold", Theme: ControlTheme{ControlText: woxui.Color{R: 1, A: 255}},
	})
	focusable := buildHotkeyRecorderForTest(recorder)
	content := focusable.Child.(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Text)
	if content.Value != "Hold Cmd" {
		t.Fatalf("hold modifier label = %q, want Hold Cmd", content.Value)
	}
	if !focusable.UnfocusOnPointerOutside {
		t.Fatal("hotkey recorder should release focus after a pointer press outside")
	}
}

func TestWoxHotkeyRecorderUsesErrorBorder(t *testing.T) {
	errorColor := woxui.Color{R: 220, G: 40, B: 40, A: 255}
	recorder, _ := WoxHotkeyRecorder(HotkeyRecorderProps{ID: "hotkey", Error: true, Focused: true, Theme: ControlTheme{
		Error: errorColor, Focus: woxui.Color{R: 10, G: 20, B: 30, A: 255},
	}})
	container := buildHotkeyRecorderForTest(recorder).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	if container.BorderColor != errorColor {
		t.Fatalf("error border = %+v, want %+v", container.BorderColor, errorColor)
	}
}

func TestWoxHotkeyRecorderUsesKeyboardOnlyFocusRing(t *testing.T) {
	cursor := woxui.Color{R: 10, G: 20, B: 30, A: 255}
	recorder, _ := WoxHotkeyRecorder(HotkeyRecorderProps{ID: "hotkey", Focused: true, Theme: ControlTheme{Focus: cursor}})
	focusable := buildHotkeyRecorderForTest(recorder)
	container := focusable.Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	if container.BorderWidth != 1 || focusable.FocusRingColor != cursor || !focusable.Autofocus {
		t.Fatalf("recorder focus styling = border %.0f ring %+v, want idle border with host focus-visible ring", container.BorderWidth, focusable.FocusRingColor)
	}
}

func TestWoxHotkeyRecorderAddsHoverSurface(t *testing.T) {
	foreground := woxui.Color{R: 210, G: 220, B: 230, A: 255}
	recorder, _ := WoxHotkeyRecorder(HotkeyRecorderProps{ID: "hotkey", Theme: ControlTheme{Text: foreground}})
	stateful := recorder.(woxwidget.Stateful)
	state := &hotkeyRecorderFocusState{hovered: true}
	state.InitState(woxwidget.StateContext{}, stateful.Widget)
	gesture := state.Build(woxwidget.StateContext{}, stateful.Widget).(woxwidget.Focusable).Child.(woxwidget.Gesture)
	container := gesture.Child.(woxwidget.Container)

	if gesture.OnHoverAt == nil {
		t.Fatal("hotkey recorder does not retain hover input")
	}
	if container.Color != controlHoverColor(woxui.Color{}, foreground) || container.BorderColor != withAlpha(foreground, 200) {
		t.Fatalf("hotkey recorder hover = background %#v border %#v", container.Color, container.BorderColor)
	}
}

func TestWoxHotkeyRecorderOutlineFollowsValueText(t *testing.T) {
	subtitle := woxui.Color{R: 255, A: 255}
	foreground := woxui.Color{R: 210, G: 220, B: 230, A: 255}
	recorder, _ := WoxHotkeyRecorder(HotkeyRecorderProps{ID: "hotkey", Placeholder: "Record", Theme: ControlTheme{TextSecondary: subtitle, Text: foreground}})
	container := buildHotkeyRecorderForTest(recorder).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	want := foreground
	want.A = 140
	if container.BorderColor != want {
		t.Fatalf("hotkey recorder border = %#v, want Text %#v", container.BorderColor, want)
	}
	placeholder := container.Child.(woxwidget.Align).Child.(woxwidget.Text)
	if placeholder.Color != foreground {
		t.Fatalf("hotkey recorder placeholder = %#v, want Text", placeholder.Color)
	}
}

func TestWoxHotkeyRecorderFocusNodeOwnsRecordingLifecycle(t *testing.T) {
	var focusChanges []bool
	host := woxwidget.NewHost(func(frame woxui.FrameInfo) woxwidget.Widget {
		recorder, _ := WoxHotkeyRecorder(HotkeyRecorderProps{
			ID: "hotkey", Focused: true, Placeholder: "Record", Theme: ControlTheme{},
			OnFocusChange: func(focused bool) { focusChanges = append(focusChanges, focused) },
		})
		return recorder
	})
	host.AttachServices(&hotkeyRecorderHostServices{})
	displayList := woxui.DisplayList{}
	host.Frame(&displayList, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 80}, PixelSize: woxui.PixelSize{Width: 200, Height: 80}, Scale: 1})
	if len(focusChanges) != 1 || !focusChanges[0] {
		t.Fatalf("initial controlled focus changes = %v, want [true]", focusChanges)
	}

	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 180, Y: 60}})
	if len(focusChanges) != 2 || focusChanges[1] {
		t.Fatalf("outside pointer focus changes = %v, want [true false]", focusChanges)
	}
}

func TestWoxHotkeyRecorderHandlesSpecialFocusKeys(t *testing.T) {
	var focusChanges []bool
	host := woxwidget.NewHost(func(frame woxui.FrameInfo) woxwidget.Widget {
		recorder, _ := WoxHotkeyRecorder(HotkeyRecorderProps{
			ID: "hotkey", Focused: true, Placeholder: "Record", Theme: ControlTheme{},
			OnFocusChange: func(focused bool) { focusChanges = append(focusChanges, focused) },
		})
		return woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			recorder,
			woxwidget.Focusable{Key: "next", Child: woxwidget.Container{Width: 80, Height: 30}},
		}}
	})
	host.AttachServices(&hotkeyRecorderHostServices{})
	displayList := woxui.DisplayList{}
	host.Frame(&displayList, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 80}, PixelSize: woxui.PixelSize{Width: 200, Height: 80}, Scale: 1})

	if !host.Key(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) {
		t.Fatal("Escape should be consumed by the recorder")
	}
	if len(focusChanges) != 2 || focusChanges[1] {
		t.Fatalf("Escape focus changes = %v, want recorder focus released", focusChanges)
	}

	host.RequestFocus("hotkey")
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}) {
		t.Fatal("Enter should be consumed by the recorder")
	}
	if len(focusChanges) != 4 || focusChanges[3] {
		t.Fatalf("Enter focus changes = %v, want recorder focus released", focusChanges)
	}

	host.RequestFocus("hotkey")
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyTab, Down: true}) || !host.HasFocus("next") {
		t.Fatal("Tab should leave recording and move focus to the next control")
	}
}

func TestWoxHotkeyRecorderAsksOnKeyBeforeConsuming(t *testing.T) {
	var handled []woxui.KeyEvent
	host := woxwidget.NewHost(func(frame woxui.FrameInfo) woxwidget.Widget {
		recorder, _ := WoxHotkeyRecorder(HotkeyRecorderProps{
			ID: "hotkey", Focused: true, Placeholder: "Record", Theme: ControlTheme{},
			OnKey: func(event woxui.KeyEvent) bool {
				handled = append(handled, event)
				return event.Key == woxui.KeyEnter && event.Modifiers.HasPrimary()
			},
		})
		return recorder
	})
	host.AttachServices(&hotkeyRecorderHostServices{})
	displayList := woxui.DisplayList{}
	host.Frame(&displayList, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 80}, PixelSize: woxui.PixelSize{Width: 200, Height: 80}, Scale: 1})

	primary := woxui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		primary = woxui.KeyModifierMeta
	}
	save := woxui.KeyEvent{Key: woxui.KeyEnter, Down: true, Modifiers: primary}
	if !host.Key(save) {
		t.Fatal("OnKey should be able to consume a parent shortcut")
	}
	if !host.HasFocus("hotkey") {
		t.Fatal("a handled parent shortcut must not unfocus the recorder")
	}

	escape := woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}
	if !host.Key(escape) {
		t.Fatal("unhandled Escape should still leave the recorder")
	}
	if host.HasFocus("hotkey") {
		t.Fatal("unhandled Escape should unfocus the recorder")
	}
	if len(handled) != 2 || handled[0] != save || handled[1] != escape {
		t.Fatalf("OnKey events = %+v, want save then escape", handled)
	}
}

func buildHotkeyRecorderForTest(recorder woxwidget.Widget) woxwidget.Focusable {
	stateful := recorder.(woxwidget.Stateful)
	state := &hotkeyRecorderFocusState{}
	state.InitState(woxwidget.StateContext{}, stateful.Widget)
	return state.Build(woxwidget.StateContext{}, stateful.Widget).(woxwidget.Focusable)
}

type hotkeyRecorderHostServices struct{}

func (s *hotkeyRecorderHostServices) MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error) {
	return woxui.TextMetrics{Size: woxui.Size{Width: float32(len([]rune(text))) * style.Size / 2, Height: style.Size}}, nil
}

func (s *hotkeyRecorderHostServices) Invalidate() error               { return nil }
func (s *hotkeyRecorderHostServices) InvalidateRect(woxui.Rect) error { return nil }

func (s *hotkeyRecorderHostServices) SetTextInputState(state woxui.TextInputState) error { return nil }

func (s *hotkeyRecorderHostServices) SetPointerCursor(cursor woxui.PointerCursor) error { return nil }

func (s *hotkeyRecorderHostServices) UpdateAccessibility(tree woxui.AccessibilityTree, handler woxui.AccessibilityActionHandler) error {
	return nil
}
