package filesearch

import "testing"

func TestResolveLastIndexElapsedMsPrefersLiveRunThenPersisted(t *testing.T) {
	if got := resolveLastIndexElapsedMs(1200, 8000); got != 1200 {
		t.Fatalf("live elapsed = %d, want the in-flight run", got)
	}
	if got := resolveLastIndexElapsedMs(0, 8000); got != 8000 {
		t.Fatalf("idle elapsed = %d, want the persisted full-index duration", got)
	}
	if got := resolveLastIndexElapsedMs(0, 0); got != 0 {
		t.Fatalf("empty elapsed = %d, want 0", got)
	}
}
