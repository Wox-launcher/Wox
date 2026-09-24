package preview

import "testing"

// TestChatScrollState exercises the same interaction state used by both chat surfaces.
func TestChatScrollState(t *testing.T) {
	var state ChatScrollState
	if state.Position(500) != 500 {
		t.Fatal("new conversation must follow")
	}
	state.Scroll(-100, 500)
	if state.Position(600) != 400 {
		t.Fatal("stream growth moved manual scrollback")
	}
	state.Scroll(190, 600)
	if state.Position(800) != 800 {
		t.Fatal("returning near bottom did not resume follow")
	}
	state.Scroll(-200, 800)
	state.SetExtent(900)
	state.ScrollPage(240)
	if state.Position(1000) != 840 {
		t.Fatal("page navigation ignored current extent")
	}
	state.FollowLatest()
	if state.Position(1200) != 1200 {
		t.Fatal("new request did not resume follow")
	}
}

func TestChatScrollStateShortOverflowStaysScrolledUp(t *testing.T) {
	var state ChatScrollState
	if state.Position(24.8) != 24.8 {
		t.Fatal("short conversation must still follow the latest message")
	}
	state.Scroll(-40, 24.8)
	if state.Position(24.8) != 0 {
		t.Fatal("scrolling a short overflow snapped back to the bottom")
	}
	state.Scroll(-10, 24.8)
	if state.Position(24.8) != 0 {
		t.Fatal("further upward scrolling left the top")
	}
	state.Scroll(40, 24.8)
	if state.Position(30) != 30 {
		t.Fatal("returning to the bottom of a short overflow did not resume follow")
	}
}
