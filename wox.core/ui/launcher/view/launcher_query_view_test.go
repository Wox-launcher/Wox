package view

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestLauncherQueryRemainsFocusableWithoutOwningFocus(t *testing.T) {
	query := LauncherQueryView(LauncherQueryProps{Focused: false, Enabled: true}).(woxwidget.EditableText)
	if query.Disabled {
		t.Fatal("unfocused query was disabled instead of remaining pointer-focusable")
	}
}

func TestLauncherQueryKeepsMinimumEditableAreaBeforeDragOverlay(t *testing.T) {
	tapped := false
	query := LauncherQueryView(LauncherQueryProps{
		Width: 500, Height: 40, TextWidth: 50, Enabled: true,
		OnTapEnd: func() { tapped = true },
	}).(woxwidget.Stack)
	dragArea := query.Children[1]
	if dragArea.Left != 350 {
		t.Fatalf("drag area left = %v, want 350", dragArea.Left)
	}
	dragGesture := dragArea.Child.(woxwidget.Gesture)
	if dragGesture.Cursor != woxui.PointerCursorDefault {
		t.Fatalf("drag area cursor = %v, want default", dragGesture.Cursor)
	}
	dragGesture.OnTap()
	if !tapped {
		t.Fatal("drag area tap did not request query focus")
	}
}
