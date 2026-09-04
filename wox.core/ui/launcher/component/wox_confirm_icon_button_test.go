package component

import (
	"testing"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestConfirmIconButtonGeometryAndCancellation(t *testing.T) {
	props := ConfirmIconButtonProps{ID: "delete", Label: "Delete", ConfirmLabel: "Confirm delete", Theme: Theme{ErrorText: woxui.Color{R: 200, A: 255}}}
	cancelled := false
	button := confirmIconButtonWithState(props, true, func(inside bool) { cancelled = !inside }, nil).(woxwidget.Stateful).Widget.(IconButtonProps)
	if button.Width != SettingsCompactControlHeight || button.Height != SettingsCompactControlHeight || button.Label != props.ConfirmLabel || button.Background != props.Theme.ErrorText {
		t.Fatalf("confirmation = %+v", button)
	}
	button.OnFocusChange(false)
	if !cancelled {
		t.Fatal("blur did not cancel confirmation")
	}
	cancelled = false
	if !button.OnKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) || !cancelled {
		t.Fatal("Escape did not cancel confirmation")
	}
}

func TestFormTableDeleteRequiresTwoActivations(t *testing.T) {
	state := &confirmIconButtonState{}
	if state.advanceConfirmation() {
		t.Fatal("first delete activation must only enter confirmation")
	}
	if !state.confirm {
		t.Fatal("first delete activation did not retain confirmation state")
	}
	if !state.advanceConfirmation() {
		t.Fatal("second delete activation must confirm deletion")
	}
	if state.confirm {
		t.Fatal("confirmed deletion must clear confirmation state")
	}
}

func TestFormTableDeleteConfirmationClearsOnMouseLeave(t *testing.T) {
	state := &confirmIconButtonState{hovered: true, confirm: true}
	state.setHovered(false)
	if state.hovered || state.confirm {
		t.Fatalf("delete state after mouse leave = hovered %v confirm %v, want both cleared", state.hovered, state.confirm)
	}
}
