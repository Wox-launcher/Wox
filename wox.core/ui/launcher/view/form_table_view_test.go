package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestFormTableRowNonTextControlsExposeControlledFocus(t *testing.T) {
	focused := 0
	props := FormTableRowFieldProps{
		ID: "field", Label: "Field", Focused: true, Theme: woxcomponent.Theme{},
		OnFocus: func() { focused++ }, OnKey: func(woxui.KeyEvent) bool { return true }, OnTap: func() {},
	}

	checkbox := formTableRowCheckboxControl(props).(woxwidget.Focusable)
	if !checkbox.Autofocus || checkbox.OnKey == nil || checkbox.OnFocusChange == nil {
		t.Fatal("checkbox should expose the controlled table-row focus contract")
	}
	checkbox.OnFocusChange(true)

	props.OnChoiceTap = func(woxui.Rect) {}
	selectControl := formTableRowSelectControl(props, 200, 34).(woxwidget.Semantics).Child.(woxwidget.Focusable)
	if !selectControl.Autofocus || selectControl.OnKey == nil || selectControl.OnFocusChange == nil {
		t.Fatal("select should expose the controlled table-row focus contract")
	}
	selectControl.OnFocusChange(true)

	if focused != 2 {
		t.Fatalf("focus callbacks = %d, want both controls to synchronize logical focus", focused)
	}
}
