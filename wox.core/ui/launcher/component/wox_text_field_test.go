package component

import (
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
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyBackspace, Modifiers: primary, Down: true}) || controller.Text() != "\nbeta" {
		t.Fatalf("primary+Backspace text = %q, want previous word deleted", controller.Text())
	}
	if parentCalls != 0 {
		t.Fatalf("parent shortcut handler called %d times, want standard editing shortcuts retained by the field", parentCalls)
	}
}
