package view

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestUsageShareButtonCentersIconAndLabel(t *testing.T) {
	icon := &woxui.Image{}
	button, _ := usageShareButton(UsageSettingsProps{ShareLabel: "Share to X", ShareIcon: icon})
	container := button.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	if container.Padding.Top != container.Padding.Bottom {
		t.Fatalf("share button vertical padding = %v/%v, want centered content", container.Padding.Top, container.Padding.Bottom)
	}
	aligned := container.Child.(woxwidget.Align)
	if aligned.Vertical != 0.5 {
		t.Fatalf("share button vertical alignment = %v, want 0.5", aligned.Vertical)
	}
	content := aligned.Child.(woxwidget.Flex)
	if content.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatal("share button icon and label should share the vertical center line")
	}
}
