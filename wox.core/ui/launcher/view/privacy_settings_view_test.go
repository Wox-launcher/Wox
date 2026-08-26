package view

import (
	"testing"

	woxwidget "wox/ui/widget"
)

func TestPrivacyViewSampleButtonUsesSymmetricPadding(t *testing.T) {
	page := PrivacySettingsView(PrivacySettingsProps{
		Width: 900, Height: 400, ViewSampleLabel: "View data sample", TelemetryTitle: "Anonymous Usage Statistics",
	})
	fields := page.(woxwidget.Container).Child.(woxwidget.Flex)
	telemetry := fields.Children[2].(woxwidget.Container).Child.(woxwidget.Flex)
	controls := telemetry.Children[1].(woxwidget.Align).Child.(woxwidget.Flex)
	button := focusedControlGesture(controls.Children[0]).Child.(woxwidget.Container)
	if button.Padding.Left != 12 || button.Padding.Right != 12 {
		t.Fatalf("view sample padding = %+v, want the shared 12px button insets", button.Padding)
	}
}
