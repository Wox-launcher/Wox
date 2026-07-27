package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestSharedEditStateBeginEndMutualExclusion(t *testing.T) {
	s := newSharedEditState()
	if !s.Begin("general", "LangCode") {
		t.Fatal("first Begin should succeed")
	}
	if s.Begin("plugins", "PluginFilter") {
		t.Fatal("second Begin from different owner should fail")
	}
	if !s.Begin("general", "LangCode") {
		t.Fatal("re-Begin from same owner should succeed")
	}
	s.End()
	if !s.Begin("plugins", "PluginFilter") {
		t.Fatal("Begin after End should succeed")
	}
}

func TestSharedEditStateStateZeroWhenIdle(t *testing.T) {
	s := newSharedEditState()
	key, editing, picker := s.State()
	if key != "" || picker != nil {
		t.Fatalf("idle state should be zero, got key=%q picker=%v", key, picker)
	}
	if editing.Text != "" {
		t.Fatalf("idle editor text should be empty, got %q", editing.Text)
	}
	_ = woxui.TextEditingState{} // silence unused import if needed
}
