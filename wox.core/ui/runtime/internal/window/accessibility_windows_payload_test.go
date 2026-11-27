//go:build windows

package window

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/lxn/win"
)

// TestEncodeWindowsAccessibilityTreeLayout pins the binary layout shared with the native decoder.
func TestEncodeWindowsAccessibilityTreeLayout(t *testing.T) {
	tree := AccessibilityTree{Generation: 3, Nodes: []AccessibilityNode{{
		ID: 7, ParentID: 0, Children: []AccessibilityNodeID{8, 9},
		AutomationID: "launcher.ok", Role: AccessibilityRoleButton, Label: "纭畾", Description: "desc", Value: "v",
		Bounds: Rect{X: 1.5, Y: 2.5, Width: 30, Height: 40}, Actions: []AccessibilityAction{AccessibilityActionActivate},
		LiveRegion: AccessibilityLiveRegionPolite, Enabled: true,
	}}}
	buffer := encodeWindowsAccessibilityTree(nil, tree)
	order := binary.LittleEndian
	position := 0
	readU32 := func() uint32 {
		value := order.Uint32(buffer[position:])
		position += 4
		return value
	}
	readU64 := func() uint64 {
		value := order.Uint64(buffer[position:])
		position += 8
		return value
	}
	readString := func() string {
		length := int(readU32())
		value := string(buffer[position : position+length])
		position += length
		return value
	}
	if count := readU32(); count != 1 {
		t.Fatalf("node count = %d, want 1", count)
	}
	if id, parent, children := readU64(), readU64(), readU32(); id != 7 || parent != 0 || children != 2 {
		t.Fatalf("header = %d/%d/%d", id, parent, children)
	}
	if first, second := readU64(), readU64(); first != 8 || second != 9 {
		t.Fatalf("children = %d,%d", first, second)
	}
	for _, want := range []float32{1.5, 2.5, 30, 40} {
		if got := math.Float32frombits(readU32()); got != want {
			t.Fatalf("bounds component = %v, want %v", got, want)
		}
	}
	if state, action, live := readU32(), readU32(), readU32(); state != accessibilityNodeStateFlags(tree.Nodes[0]) || action != accessibilityNodeActionFlags(tree.Nodes[0]) || live != 1 {
		t.Fatalf("flags = %d/%d/%d", state, action, live)
	}
	for _, want := range []string{"launcher.ok", "button", "纭畾", "desc", "v"} {
		if got := readString(); got != want {
			t.Fatalf("string = %q, want %q", got, want)
		}
	}
	if position != len(buffer) {
		t.Fatalf("decoded %d of %d bytes", position, len(buffer))
	}

	// Reusing the buffer must produce the identical encoding.
	if again := encodeWindowsAccessibilityTree(buffer[:0], tree); string(again) != string(buffer) {
		t.Fatal("reused buffer encoded a different payload")
	}
}

// TestSyncWindowsAccessibilityForQueryIgnoresUnknownWindow keeps WM_GETOBJECT safe for HWNDs without state.
func TestSyncWindowsAccessibilityForQueryIgnoresUnknownWindow(t *testing.T) {
	syncWindowsAccessibilityForQuery(0xdeadbeef)
}

// registerTestAccessibilityWindow publishes a synthetic window so UIA query paths can run without a real HWND.
func registerTestAccessibilityWindow(t *testing.T, hwnd uintptr, tree AccessibilityTree) *platformWindow {
	t.Helper()
	window := &platformWindow{hwnd: win.HWND(hwnd)}
	nativeWindows.Store(hwnd, window)
	accessibilityWindows.Store(window, accessibilityWindowState{tree: tree, contentHash: accessibilityTreeContentHash(tree)})
	t.Cleanup(func() {
		removeWindowsAccessibility(hwnd)
		accessibilityWindows.Delete(window)
		nativeWindows.Delete(hwnd)
	})
	return window
}

// TestWindowsAccessibilityQuerySkipsClosingWindow ensures a re-entrant WM_GETOBJECT during WM_DESTROY
// cannot republish the snapshot into native state.
func TestWindowsAccessibilityQuerySkipsClosingWindow(t *testing.T) {
	const hwnd = uintptr(0xdead0001)
	tree := AccessibilityTree{Generation: 1, Nodes: []AccessibilityNode{{ID: 1, Label: "root"}}}
	window := registerTestAccessibilityWindow(t, hwnd, tree)

	window.markAccessibilityClosing()
	removeWindowsAccessibility(hwnd)
	syncWindowsAccessibilityForQuery(hwnd)
	if window.a11ySynced {
		t.Fatal("query republished the snapshot after the window started closing")
	}
	if result := windowsAccessibilityObject(hwnd, 0, uintptr(uint32(0xffffffe7))); result != 0 {
		t.Fatalf("closing window returned provider %#x, want 0", result)
	}
	if window.a11ySynced {
		t.Fatal("WM_GETOBJECT republished the snapshot after the window started closing")
	}
}

// TestWindowsAccessibilityQuerySyncsSameGenerationDifferentContent covers publishers that keep a
// fixed Generation while content changes.
func TestWindowsAccessibilityQuerySyncsSameGenerationDifferentContent(t *testing.T) {
	const hwnd = uintptr(0xdead0002)
	first := AccessibilityTree{Generation: 1, Nodes: []AccessibilityNode{{ID: 1, Label: "first"}}}
	second := AccessibilityTree{Generation: 1, Nodes: []AccessibilityNode{{ID: 1, Label: "second"}}}
	hashFirst, hashSecond := accessibilityTreeContentHash(first), accessibilityTreeContentHash(second)
	if hashFirst == hashSecond {
		t.Fatal("test trees must differ in content")
	}
	window := registerTestAccessibilityWindow(t, hwnd, first)

	if err := pushWindowsAccessibilityTreeOnUI(window, win.HWND(hwnd), first, hashFirst); err != nil {
		t.Fatal(err)
	}
	if !window.a11ySynced || window.a11ySyncedHash != hashFirst {
		t.Fatalf("after push synced=%t hash=%d, want first hash", window.a11ySynced, window.a11ySyncedHash)
	}
	accessibilityWindows.Store(window, accessibilityWindowState{tree: second, contentHash: hashSecond})
	syncWindowsAccessibilityForQuery(hwnd)
	if window.a11ySyncedHash != hashSecond {
		t.Fatal("query skipped the update although content changed under the same generation")
	}
	syncWindowsAccessibilityForQuery(hwnd)
	if window.a11ySyncedHash != hashSecond {
		t.Fatal("unchanged content must remain synced")
	}
}
