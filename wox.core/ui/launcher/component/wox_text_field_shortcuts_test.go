package component

import (
	"runtime"
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestMultilineDocumentNavigationPrecedesVisualLines covers soft wraps and selection extension.
func TestMultilineDocumentNavigationPrecedesVisualLines(t *testing.T) {
	text := "alpha beta gamma\ntail"
	primary := woxui.KeyModifierControl
	startKey, endKey := woxui.KeyHome, woxui.KeyEnd
	if runtime.GOOS == "darwin" {
		primary = woxui.KeyModifierMeta
		startKey, endKey = woxui.KeyArrowUp, woxui.KeyArrowDown
	}
	measurer := &fakeTextMeasurer{charWidth: 10}
	style := woxui.TextStyle{Size: 12}
	type navigationCase struct {
		key  woxui.Key
		want int
	}
	for _, softWrap := range []bool{false, true} {
		lines := textFieldLines(text, measurer, style, 60, softWrap)
		if softWrap && len(lines) <= 2 {
			t.Fatal("navigation fixture did not produce soft wraps")
		}
		commands := []navigationCase{{startKey, 0}, {endKey, len([]rune(text))}}
		if runtime.GOOS == "darwin" {
			commands = append(commands, navigationCase{woxui.KeyArrowLeft, 0}, navigationCase{woxui.KeyArrowRight, 16})
		}
		for _, extend := range []bool{false, true} {
			for _, test := range commands {
				controller := woxwidget.NewTextEditingController(text)
				controller.SetCaret(8)
				modifiers := primary
				if extend {
					modifiers |= woxui.KeyModifierShift
				}
				handled, changed := handleTextFieldControllerKey(controller, 8, lines, 80, measurer, style, 60, softWrap, woxui.KeyEvent{Key: test.key, Modifiers: modifiers, Down: true})
				selection := controller.State().Selection
				anchor := test.want
				if extend {
					anchor = 8
				}
				if !handled || changed || selection.Focus != test.want || selection.Anchor != anchor {
					t.Fatalf("softWrap=%t extend=%t key=%s handled=%t changed=%t selection=%+v", softWrap, extend, test.key, handled, changed, selection)
				}
			}
		}
	}
}

// TestTextFieldExhaustedCustomRedoDoesNotUndo reproduces a Note with undo history but no redo history.
func TestTextFieldExhaustedCustomRedoDoesNotUndo(t *testing.T) {
	undoCalls, redoCalls := 0, 0
	controller := woxwidget.NewTextEditingController("draft")
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return WoxTextField(TextFieldProps{ID: "redo", Width: 200, Height: 80, MaxLines: 4, Controller: controller,
			OnUndo: func() bool { undoCalls++; return true }, OnRedo: func() bool { redoCalls++; return false },
		})
	})
	host.AttachServices(&hotkeyRecorderHostServices{})
	host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 80}, Scale: 1})
	host.RequestFocus("redo")
	primary := woxui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		primary = woxui.KeyModifierMeta
	}
	if !host.Key(woxui.KeyEvent{Key: "z", Modifiers: primary | woxui.KeyModifierShift, Down: true}) || undoCalls != 0 || redoCalls != 1 || controller.Text() != "draft" {
		t.Fatalf("redo invoked undo=%d redo=%d text=%q", undoCalls, redoCalls, controller.Text())
	}
}

// TestReadOnlyTextFieldDoesNotInvokeMutationCallbacks protects custom document undo and clipboard hooks.
func TestReadOnlyTextFieldDoesNotInvokeMutationCallbacks(t *testing.T) {
	mutations := 0
	controller := woxwidget.NewTextEditingController("read only")
	controller.SelectAll()
	mutate := func() bool { mutations++; return true }
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return WoxTextField(TextFieldProps{ID: "readonly", Width: 200, Height: 80, Controller: controller, ReadOnly: true,
			OnCut: mutate, OnUndo: mutate, OnRedo: mutate, OnPaste: func(string) bool { return mutate() },
		})
	})
	host.AttachServices(&hotkeyRecorderHostServices{})
	host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 80}, Scale: 1})
	host.RequestFocus("readonly")
	primary := woxui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		primary = woxui.KeyModifierMeta
	}
	for _, event := range []woxui.KeyEvent{
		{Key: "x", Modifiers: primary, Down: true},
		{Key: "v", Modifiers: primary, Down: true},
		{Key: "z", Modifiers: primary, Down: true},
		{Key: "z", Modifiers: primary | woxui.KeyModifierShift, Down: true},
		{Key: "y", Modifiers: primary, Down: true},
	} {
		if !host.Key(event) {
			t.Fatalf("read-only field did not consume %+v", event)
		}
	}
	if mutations != 0 || controller.Text() != "read only" {
		t.Fatalf("read-only mutation callbacks=%d text=%q", mutations, controller.Text())
	}
}
