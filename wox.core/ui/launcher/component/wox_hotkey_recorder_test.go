package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxHotkeyRecorderRendersHoldModifierAsText(t *testing.T) {
	recorder, _ := WoxHotkeyRecorder(HotkeyRecorderProps{
		ID: "hotkey", Labels: []string{"Cmd"}, Hold: true, HoldPrefix: "Hold", Theme: Theme{ActionText: woxui.Color{R: 1, A: 255}},
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
	recorder, _ := WoxHotkeyRecorder(HotkeyRecorderProps{ID: "hotkey", Error: true, Focused: true, Theme: Theme{
		ErrorText: errorColor, Cursor: woxui.Color{R: 10, G: 20, B: 30, A: 255},
	}})
	container := buildHotkeyRecorderForTest(recorder).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	if container.BorderColor != errorColor {
		t.Fatalf("error border = %+v, want %+v", container.BorderColor, errorColor)
	}
}

func TestWoxHotkeyRecorderUsesKeyboardOnlyFocusRing(t *testing.T) {
	cursor := woxui.Color{R: 10, G: 20, B: 30, A: 255}
	recorder, _ := WoxHotkeyRecorder(HotkeyRecorderProps{ID: "hotkey", Focused: true, Theme: Theme{Cursor: cursor}})
	focusable := buildHotkeyRecorderForTest(recorder)
	container := focusable.Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	if container.BorderWidth != 1 || focusable.FocusRingColor != cursor || !focusable.Autofocus {
		t.Fatalf("recorder focus styling = border %.0f ring %+v, want idle border with host focus-visible ring", container.BorderWidth, focusable.FocusRingColor)
	}
}

func TestWoxHotkeyRecorderFocusNodeOwnsRecordingLifecycle(t *testing.T) {
	var focusChanges []bool
	host := woxwidget.NewHost(func(frame woxui.FrameInfo) woxwidget.Widget {
		recorder, _ := WoxHotkeyRecorder(HotkeyRecorderProps{
			ID: "hotkey", Focused: true, Placeholder: "Record", Theme: Theme{},
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
			ID: "hotkey", Focused: true, Placeholder: "Record", Theme: Theme{},
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

func (s *hotkeyRecorderHostServices) Invalidate() error { return nil }

func (s *hotkeyRecorderHostServices) SetTextInputState(state woxui.TextInputState) error { return nil }

func (s *hotkeyRecorderHostServices) SetPointerCursor(cursor woxui.PointerCursor) error { return nil }

func (s *hotkeyRecorderHostServices) UpdateAccessibility(tree woxui.AccessibilityTree, handler woxui.AccessibilityActionHandler) error {
	return nil
}
