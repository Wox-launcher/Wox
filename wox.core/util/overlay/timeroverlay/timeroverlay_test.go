package timeroverlay

import (
	"testing"

	"wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestTimerSizeExpandsOnceDetailsAreVisible(t *testing.T) {
	compact := timerSize(woxui.Size{Width: 60, Height: 24}, woxui.Size{}, false, true)
	expanded := timerSize(woxui.Size{Width: 60, Height: 24}, woxui.Size{Width: 90, Height: 14}, true, true)
	if expanded.Width <= compact.Width || expanded.Height <= compact.Height {
		t.Fatalf("expanded size = %+v, compact size = %+v", expanded, compact)
	}
}

func TestTimerSizeLeavesTextMeasurementSlack(t *testing.T) {
	countdown := woxui.Size{Width: 60, Height: 24}
	compact := timerSize(countdown, woxui.Size{}, false, true)
	textWidth := compact.Width - timerHorizontalPadding*2
	if textWidth < countdown.Width+timerTextSlack {
		t.Fatalf("timer text width = %v, want at least %v", textWidth, countdown.Width+timerTextSlack)
	}
}

func TestTimerHoverChangesOnlyOnBoundaryTransitions(t *testing.T) {
	if !nextTimerHovered(false, woxui.PointerEnter) {
		t.Fatal("pointer enter should expand the timer")
	}
	if nextTimerHovered(true, woxui.PointerLeave) {
		t.Fatal("pointer leave should collapse the timer")
	}
	if nextTimerHovered(false, woxui.PointerMove) {
		t.Fatal("queued pointer move after leave should not expand the timer")
	}
}

func TestTimerOverlayExposesCountdownAndHoverControls(t *testing.T) {
	opts := Options{Countdown: "01:30", Note: "Tea", Closable: true}
	compact := buildTimerOverlay(opts, woxui.FrameInfo{Size: woxui.Size{Width: 180, Height: 60}}, false).(woxwidget.Container)
	compactStack := compact.Child.(woxwidget.Stack)
	compactChildren := compactStack.Children[0].Child.(woxwidget.Align).Child.(woxwidget.Flex).Children
	countdown := compactChildren[0].(woxwidget.Semantics)
	if countdown.AutomationID != "timer-overlay-countdown" || countdown.Value != "01:30" || countdown.Role != woxui.AccessibilityRoleText || !countdown.ReadOnly {
		t.Fatalf("countdown semantics = %#v", countdown)
	}
	if len(compactStack.Children) != 1 {
		t.Fatalf("compact overlay children = %d, want countdown only", len(compactStack.Children))
	}

	expanded := buildTimerOverlay(opts, woxui.FrameInfo{Size: woxui.Size{Width: 220, Height: 84}}, true).(woxwidget.Container)
	expandedStack := expanded.Child.(woxwidget.Stack)
	expandedChildren := expandedStack.Children[0].Child.(woxwidget.Align).Child.(woxwidget.Flex).Children
	note := expandedChildren[1].(woxwidget.Semantics)
	if note.AutomationID != "timer-overlay-note" || note.Value != "Tea" || note.Role != woxui.AccessibilityRoleText || !note.ReadOnly {
		t.Fatalf("note semantics = %#v", note)
	}
	closeButton := expandedStack.Children[1].Child.(woxwidget.Stateful)
	closeProps := closeButton.Widget.(component.IconButtonProps)
	if closeProps.ID != "timer-overlay-close" || closeProps.OnTap == nil {
		t.Fatalf("close button = %#v", closeProps)
	}
}
