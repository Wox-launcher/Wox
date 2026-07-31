package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxHotkeyRecorderRendersHoldModifierAsText(t *testing.T) {
	recorder, _ := WoxHotkeyRecorder(HotkeyRecorderProps{
		Labels: []string{"Cmd"}, Hold: true, HoldPrefix: "Hold", Theme: Theme{ActionText: woxui.Color{R: 1, A: 255}},
	})
	content := recorder.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Text)
	if content.Value != "Hold Cmd" {
		t.Fatalf("hold modifier label = %q, want Hold Cmd", content.Value)
	}
}

func TestWoxHotkeyRecorderUsesErrorBorder(t *testing.T) {
	errorColor := woxui.Color{R: 220, G: 40, B: 40, A: 255}
	recorder, _ := WoxHotkeyRecorder(HotkeyRecorderProps{Error: true, Focused: true, Theme: Theme{
		ErrorText: errorColor, Cursor: woxui.Color{R: 10, G: 20, B: 30, A: 255},
	}})
	container := recorder.(woxwidget.Container)
	if container.BorderColor != errorColor {
		t.Fatalf("error border = %+v, want %+v", container.BorderColor, errorColor)
	}
}
