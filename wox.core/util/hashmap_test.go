package util

import "testing"

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
