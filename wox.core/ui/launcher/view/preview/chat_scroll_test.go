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
