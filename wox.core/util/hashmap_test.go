package util

import (
	"sync"
	"testing"
)

// TestHashMapComputeSerializesUpdates covers concurrent read-modify-write without lost updates.
func TestHashMapComputeSerializesUpdates(t *testing.T) {
	h := NewHashMap[string, int]()
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 100 {
				h.Compute("count", func(value int, _ bool) (int, bool) { return value + 1, true })
			}
		}()
	}
	workers.Wait()
	if got := h.MustLoad("count"); got != 800 {
		t.Fatalf("count = %d, want 800", got)
	}
}

// TestHashMapComputeConditionalRemoval ensures stale owners cannot delete a replacement value.
func TestHashMapComputeConditionalRemoval(t *testing.T) {
	h := NewHashMap[string, int]()
	h.Compute("key", func(value int, exists bool) (int, bool) {
		if exists || value != 0 {
			t.Fatal("missing key should provide its zero value")
		}
		return 1, true
	})
	h.Store("key", 2)
	h.Compute("key", func(value int, exists bool) (int, bool) { return value, exists && value != 1 })
	if got := h.MustLoad("key"); got != 2 {
		t.Fatalf("replacement = %d, want 2", got)
	}
	h.Compute("key", func(value int, exists bool) (int, bool) { return value, exists && value != 2 })
	if h.Len() != 0 {
		t.Fatal("matching value was not removed")
	}
	h.Compute("missing", func(value int, exists bool) (int, bool) { return value, false })
	if h.Len() != 0 {
		t.Fatal("removing a missing key created an entry")
	}
}

func TestHashMapToMapReturnsAllocatedMap(t *testing.T) {
	h := NewHashMap[string, int]()
	h.Store("a", 1)

	m := h.ToMap()
	if m == nil {
		t.Fatal("expected non-nil map")
	}
	if v, ok := m["a"]; !ok || v != 1 {
		t.Fatalf("unexpected map content: %+v", m)
	}
}

func TestHashMapToMapReturnsCopy(t *testing.T) {
	h := NewHashMap[string, int]()
	h.Store("a", 1)

	m := h.ToMap()
	m["b"] = 2

	if h.Exist("b") {
		t.Fatal("expected ToMap result to be independent from HashMap internal storage")
	}
}
