package view

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestUsageShareButtonCentersIconAndLabel(t *testing.T) {
	icon := &woxui.Image{}
	button, _ := usageShareButton(UsageSettingsProps{ShareLabel: "Share to X", ShareIcon: icon})
	container := focusedControlGesture(button).Child.(woxwidget.Container)
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

func TestUsageSummaryHeaderAnchorsShareActionToRight(t *testing.T) {
	header, _ := usageSummaryHeader(UsageSettingsProps{ShareLabel: "Share"}, 600)
	share := header.(woxwidget.Stack).Children[1]
	if !share.AnchorRight || share.Right != 0 {
		t.Fatalf("usage share anchor = %v right %.0f, want true/0", share.AnchorRight, share.Right)
	}
}
