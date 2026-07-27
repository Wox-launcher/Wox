package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestCloudAccountPlanTooltipForwardsHover(t *testing.T) {
	var gotInside bool
	var gotBounds woxui.Rect
	icon := &woxui.Image{}
	card := cloudAccountCard(CloudAccountProps{
		LoggedIn:   true,
		LabelWidth: 520,
		PlanLabel:  "Plan",
		InfoIcon:   icon,
		OnPlanTooltip: func(inside bool, bounds woxui.Rect) {
			gotInside = inside
			gotBounds = bounds
		},
	}, 830, 162, woxcomponent.Theme{}).(woxwidget.Container)

	column := card.Child.(woxwidget.Flex)
	planRow := column.Children[1].(woxwidget.Container).Child.(woxwidget.Flex)
	planLabel := planRow.Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
	heading := planLabel.Children[0].(woxwidget.Flex)
	tooltip := heading.Children[1].(woxwidget.Semantics).Child.(woxwidget.Gesture)
	bounds := woxui.Rect{X: 12, Y: 18, Width: 14, Height: 14}
	tooltip.OnHoverAt(true, bounds)

	if !gotInside || gotBounds != bounds {
		t.Fatalf("tooltip hover = (%v, %+v), want (true, %+v)", gotInside, gotBounds, bounds)
	}
}

func TestCloudWideFormActionsEndAtContentEdge(t *testing.T) {
	const width = float32(830)
	const labelWidth = float32(520)
	const gap = float32(32)

	accountCard := cloudAccountCard(CloudAccountProps{LoggedIn: true, LabelWidth: labelWidth, SupportLabel: "Support"}, width, 162, woxcomponent.Theme{}).(woxwidget.Container)
	accountColumn := accountCard.Child.(woxwidget.Flex)
	billingRow := accountColumn.Children[2].(woxwidget.Container).Child.(woxwidget.Flex)
	supportValue := billingRow.Children[1].(woxwidget.Container)
	supportRow := supportValue.Child.(woxwidget.Flex)
	supportSpacer := supportRow.Children[0].(woxwidget.Painter)
	if got := labelWidth + gap + supportValue.Width; got != width {
		t.Fatalf("billing row width = %v, want %v", got, width)
	}
	if got := supportSpacer.Width; got+112 != supportValue.Width {
		t.Fatalf("support button right edge = %v, want %v", got+112, supportValue.Width)
	}

	syncCard := cloudSyncCard(CloudSyncProps{LabelWidth: labelWidth, ButtonLabel: "Sync"}, width, woxcomponent.Theme{}).(woxwidget.Container)
	syncRow := syncCard.Child.(woxwidget.Flex)
	syncValue := syncRow.Children[1].(woxwidget.Container)
	if got := labelWidth + gap + syncValue.Width; got != width {
		t.Fatalf("sync row width = %v, want %v", got, width)
	}
	if got := syncValue.Padding.Left; got+64 != syncValue.Width {
		t.Fatalf("sync button right edge = %v, want %v", got+64, syncValue.Width)
	}

	deviceHeader := cloudDeviceHeader(CloudDevicesProps{LabelWidth: labelWidth, RefreshLabel: "Refresh"}, width, woxcomponent.Theme{}).(woxwidget.Container)
	deviceRow := deviceHeader.Child.(woxwidget.Flex)
	refreshValue := deviceRow.Children[1].(woxwidget.Container)
	if got := labelWidth + gap + refreshValue.Width; got != width {
		t.Fatalf("device row width = %v, want %v", got, width)
	}
	if got := refreshValue.Padding.Left; got+88 != refreshValue.Width {
		t.Fatalf("refresh button right edge = %v, want %v", got+88, refreshValue.Width)
	}
}

func TestCloudPlanTooltipOverlayOccupiesOnlyItsVisiblePanel(t *testing.T) {
	overlay, left, top := CloudPlanTooltipOverlay(
		CloudIntroProps{FreeLabel: "Free", ProLabel: "Pro"},
		woxui.Rect{X: 308, Y: 192, Width: 14, Height: 14},
		1152,
		768,
		woxcomponent.Theme{},
	)
	tooltip := overlay.(woxwidget.Semantics)
	panel := tooltip.Child.(woxwidget.Container)
	if panel.Width != 580 || panel.Height != 260 {
		t.Fatalf("tooltip panel = %vx%v, want 580x260", panel.Width, panel.Height)
	}

	window := SettingsWindow(SettingsWindowProps{
		Width: 1152, Height: 768, Platform: "darwin", RailWidth: 240,
		Page: woxwidget.Painter{}, Rail: woxwidget.Painter{}, TitleBar: woxwidget.Painter{},
		Overlay: overlay, OverlayLeft: left, OverlayTop: top,
	}).(woxwidget.Semantics)
	windowContainer := window.Child.(woxwidget.Container)
	stack := windowContainer.Child.(woxwidget.Stack)
	overlayChild := stack.Children[1]
	if overlayChild.Left != left || overlayChild.Top != top {
		t.Fatalf("tooltip placement = (%v, %v), want (%v, %v)", overlayChild.Left, overlayChild.Top, left, top)
	}
	if _, fullWindowOverlay := overlayChild.Child.(woxwidget.Stack); fullWindowOverlay {
		t.Fatal("tooltip must not add a full-window hit-test layer above its hover anchor")
	}
}
