package tooltip

import (
	"testing"
	"time"
)

func TestTooltipTrackingCloseWaitsForCursorToEnter(t *testing.T) {
	shouldClose, seenInside := tooltipTrackingClose(false, false, false, true, 0, false)
	if shouldClose || seenInside {
		t.Fatalf("close=%v seen=%v, want to keep a tooltip that the OS cursor has not entered", shouldClose, seenInside)
	}

	shouldClose, seenInside = tooltipTrackingClose(false, true, true, true, 0, false)
	if shouldClose || !seenInside {
		t.Fatalf("close=%v seen=%v, want to arm tracking after the cursor enters", shouldClose, seenInside)
	}

	shouldClose, seenInside = tooltipTrackingClose(true, false, false, true, 0, false)
	if !shouldClose || !seenInside {
		t.Fatalf("close=%v seen=%v, want to dismiss after the cursor leaves", shouldClose, seenInside)
	}

	shouldClose, seenInside = tooltipTrackingClose(false, false, false, false, tooltipOwnerLeaveGrace, false)
	if shouldClose || seenInside {
		t.Fatalf("close=%v seen=%v, want to ignore a missing cursor sample", shouldClose, seenInside)
	}
}

func TestTooltipTrackingCloseDismissesLeftoverAfterOwnerMove(t *testing.T) {
	shouldClose, seenInside := tooltipTrackingClose(false, false, true, true, 0, false)
	if shouldClose || seenInside {
		t.Fatalf("close=%v seen=%v, want to keep the first post-show samples that miss the trigger", shouldClose, seenInside)
	}

	shouldClose, seenInside = tooltipTrackingClose(false, false, true, true, tooltipOwnerLeaveGrace, false)
	if !shouldClose || seenInside {
		t.Fatalf("close=%v seen=%v, want to dismiss a delayed tooltip after the pointer moved on in the owner window", shouldClose, seenInside)
	}

	shouldClose, seenInside = tooltipTrackingClose(false, false, false, true, tooltipOwnerLeaveGrace+time.Second, false)
	if shouldClose || seenInside {
		t.Fatalf("close=%v seen=%v, want to keep an automation hover whose OS cursor never entered the owner", shouldClose, seenInside)
	}
}

func TestTooltipTrackingCloseKeepsSyntheticHoverOverIdleOwner(t *testing.T) {
	shouldClose, seenInside := tooltipTrackingClose(false, false, true, true, tooltipOwnerLeaveGrace, true)
	if shouldClose || seenInside {
		t.Fatalf("close=%v seen=%v, want to keep a synthetic hover when the OS cursor rests on the owner", shouldClose, seenInside)
	}
}
