package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
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

func TestUsagePeriodSelectorAddsHoverToUnselectedOption(t *testing.T) {
	theme := woxcomponent.Theme{ResultSubtitle: woxui.Color{R: 220, G: 230, B: 240, A: 255}}
	selector, _ := usagePeriodSelector(UsageSettingsProps{
		Theme:   theme,
		Periods: []UsagePeriod{{ID: "7d", Label: "最近 7 天", Selected: false, OnSelect: func() {}}, {ID: "30d", Label: "最近 30 天", Selected: true}},
	})
	row := selector.(woxwidget.Container).Child.(woxwidget.Flex)
	button := row.Children[0].(woxwidget.Semantics)
	gesture := focusedControlGesture(button)
	if gesture.OnHoverAt == nil {
		t.Fatal("unselected usage period does not expose hover input")
	}
}

func TestUsageSettingsViewUsesSymmetricHorizontalInsets(t *testing.T) {
	page := UsageSettingsView(UsageSettingsProps{Width: 800, Height: 600})
	container := page.(woxwidget.Container)
	if container.Padding.Left != 40 || container.Padding.Right != 40 {
		t.Fatalf("usage page horizontal insets = %.0f/%.0f, want 40/40", container.Padding.Left, container.Padding.Right)
	}
}
