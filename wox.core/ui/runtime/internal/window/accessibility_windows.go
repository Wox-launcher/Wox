//go:build windows

package window

/*
#cgo CXXFLAGS: -std=c++17
#cgo LDFLAGS: -lole32 -loleaut32 -luiautomationcore
#include <stdlib.h>
#include "native_windows.h"
*/
import "C"

import (
	"encoding/binary"
	"errors"
	"math"
	"unsafe"

	"github.com/lxn/win"
)

// windowsUIARootObjectID is the lParam of WM_GETOBJECT that requests the root provider (UiaRootObjectId).
const windowsUIARootObjectID = int32(-25)

// windowsA11yPayloadRetainLimit bounds the reusable encode buffer so one huge tree cannot pin memory.
const windowsA11yPayloadRetainLimit = 64 << 10

func init() {
	updateNativeAccessibility = updateWindowsAccessibility
	accessibilityWindowFocused = windowsAccessibilityWindowFocused
}

// windowsAccessibilityWindowFocused checks live focus because hidden windows may
// stop rendering before their accessibility snapshot records the focus loss.
func windowsAccessibilityWindowFocused(window *platformWindow, _ AccessibilityTree) bool {
	window.mu.Lock()
	hwnd := window.hwnd
	window.mu.Unlock()
	return hwnd != 0 && win.GetForegroundWindow() == hwnd
}

// markAccessibilityClosing stops all native UIA publication for this window. It must run before
// the provider is disconnected, because disconnecting can re-enter WM_GETOBJECT.
func (w *platformWindow) markAccessibilityClosing() {
	w.mu.Lock()
	w.a11yClosing = true
	w.a11ySynced = false
	w.a11ySyncedHash = 0
	w.mu.Unlock()
}

// accessibilityClosed reports whether native UIA state may no longer be created for this window.
func (w *platformWindow) accessibilityClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.hwnd == 0 || w.a11yClosing
}

func updateWindowsAccessibility(window *platformWindow, tree AccessibilityTree, contentHash uint64) error {
	var updateErr error
	callErr := platformCall(func() {
		updateErr = updateWindowsAccessibilityOnUI(window, tree, contentHash)
	})
	if callErr != nil {
		return callErr
	}
	return updateErr
}

// updateWindowsAccessibilityOnUI keeps provider creation and UIA events on the window's COM STA.
// The native mirror is only maintained while a UIA client can observe it; the first WM_GETOBJECT
// pushes the latest tree otherwise (see windowsAccessibilityObject).
func updateWindowsAccessibilityOnUI(window *platformWindow, tree AccessibilityTree, contentHash uint64) error {
	window.mu.Lock()
	hwnd := window.hwnd
	closing := window.a11yClosing
	window.mu.Unlock()
	if hwnd == 0 || closing {
		return errors.New("woxui: Windows window is closed")
	}
	if C.wox_windows_accessibility_has_active_client(C.uintptr_t(hwnd)) == 0 {
		return nil
	}
	return pushWindowsAccessibilityTreeOnUI(window, hwnd, tree, contentHash)
}

// pushWindowsAccessibilityTreeOnUI sends the whole tree to the native mirror in one call.
func pushWindowsAccessibilityTreeOnUI(window *platformWindow, hwnd win.HWND, tree AccessibilityTree, contentHash uint64) error {
	payload := encodeWindowsAccessibilityTree(window.a11yPayloadBuf[:0], tree)
	traceNativeCall("accessibility update enter hwnd=%#x generation=%d nodes=%d bytes=%d", hwnd, tree.Generation, len(tree.Nodes), len(payload))
	var pointer *C.uint8_t
	if len(payload) > 0 {
		pointer = (*C.uint8_t)(unsafe.Pointer(&payload[0]))
	}
	result := C.wox_windows_accessibility_update(C.uintptr_t(hwnd), C.uint64_t(tree.Generation), pointer, C.int32_t(len(payload)))
	traceNativeCall("accessibility update exit hwnd=%#x generation=%d result=%d", hwnd, tree.Generation, result)
	if cap(payload) > windowsA11yPayloadRetainLimit {
		window.a11yPayloadBuf = nil
	} else {
		window.a11yPayloadBuf = payload[:0]
	}
	if result != 0 {
		return errors.New("woxui: failed to update Windows accessibility tree")
	}
	window.a11ySyncedHash = contentHash
	window.a11ySynced = true
	return nil
}

// encodeWindowsAccessibilityTree appends the tree in the layout read by wox_windows_accessibility_update:
// uint32 node count, then per node id, parent id, child count and ids, bounds, state and action flags,
// live region, and five length-prefixed UTF-8 strings (automation id, role, label, description, value).
func encodeWindowsAccessibilityTree(buffer []byte, tree AccessibilityTree) []byte {
	order := binary.LittleEndian
	appendString := func(buffer []byte, value string) []byte {
		buffer = order.AppendUint32(buffer, uint32(len(value)))
		return append(buffer, value...)
	}
	buffer = order.AppendUint32(buffer, uint32(len(tree.Nodes)))
	for _, node := range tree.Nodes {
		buffer = order.AppendUint64(buffer, uint64(node.ID))
		buffer = order.AppendUint64(buffer, uint64(node.ParentID))
		buffer = order.AppendUint32(buffer, uint32(len(node.Children)))
		for _, child := range node.Children {
			buffer = order.AppendUint64(buffer, uint64(child))
		}
		buffer = order.AppendUint32(buffer, math.Float32bits(node.Bounds.X))
		buffer = order.AppendUint32(buffer, math.Float32bits(node.Bounds.Y))
		buffer = order.AppendUint32(buffer, math.Float32bits(node.Bounds.Width))
		buffer = order.AppendUint32(buffer, math.Float32bits(node.Bounds.Height))
		buffer = order.AppendUint32(buffer, accessibilityNodeStateFlags(node))
		buffer = order.AppendUint32(buffer, accessibilityNodeActionFlags(node))
		liveRegion := int32(0)
		if node.LiveRegion == AccessibilityLiveRegionPolite {
			liveRegion = 1
		} else if node.LiveRegion == AccessibilityLiveRegionAssertive {
			liveRegion = 2
		}
		buffer = order.AppendUint32(buffer, uint32(liveRegion))
		buffer = appendString(buffer, node.AutomationID)
		buffer = appendString(buffer, string(node.Role))
		buffer = appendString(buffer, node.Label)
		buffer = appendString(buffer, node.Description)
		buffer = appendString(buffer, node.Value)
	}
	return buffer
}

func windowsAccessibilityObject(hwnd uintptr, wParam uintptr, lParam uintptr) uintptr {
	// A closing window must fall through to DefWindowProc without touching native provider state.
	if windowValue, ok := nativeWindows.Load(hwnd); ok && windowValue.(*platformWindow).accessibilityClosed() {
		return 0
	}
	if int32(lParam) == windowsUIARootObjectID {
		syncWindowsAccessibilityForQuery(hwnd)
	}
	traceNativeCall("accessibility get_object enter hwnd=%#x wParam=%#x lParam=%#x", hwnd, wParam, lParam)
	result := uintptr(C.wox_windows_accessibility_get_object(C.uintptr_t(hwnd), C.uintptr_t(wParam), C.uintptr_t(lParam)))
	traceNativeCall("accessibility get_object exit hwnd=%#x result=%#x", hwnd, result)
	return result
}

// syncWindowsAccessibilityForQuery fills the native mirror before UIA's first read when updates were
// skipped for lack of a client. It runs on the UI thread inside WM_GETOBJECT.
func syncWindowsAccessibilityForQuery(hwnd uintptr) {
	windowValue, ok := nativeWindows.Load(hwnd)
	if !ok {
		return
	}
	window := windowValue.(*platformWindow)
	if window.accessibilityClosed() {
		return
	}
	stateValue, ok := accessibilityWindows.Load(window)
	if !ok {
		return
	}
	state := stateValue.(accessibilityWindowState)
	if window.a11ySynced && state.contentHash == window.a11ySyncedHash {
		return
	}
	_ = pushWindowsAccessibilityTreeOnUI(window, win.HWND(hwnd), state.tree, state.contentHash)
}

func removeWindowsAccessibility(hwnd uintptr) {
	traceNativeCall("accessibility remove enter hwnd=%#x", hwnd)
	C.wox_windows_accessibility_remove(C.uintptr_t(hwnd))
	traceNativeCall("accessibility remove exit hwnd=%#x", hwnd)
}

//export woxGoWindowsAccessibilityAction
func woxGoWindowsAccessibilityAction(owner C.uintptr_t, nodeID C.uint64_t, action *C.char, value *C.char) C.int32_t {
	windowValue, ok := nativeWindows.Load(uintptr(owner))
	if !ok {
		return 0
	}
	window := windowValue.(*platformWindow)
	if window.accessibilityClosed() {
		return 0
	}
	actionValue := AccessibilityAction(C.GoString(action))
	textValue := C.GoString(value)
	postErr := platformPost(func() {
		_ = performNativeAccessibilityAction(window, AccessibilityNodeID(nodeID), actionValue, textValue)
	})
	if postErr != nil {
		return 0
	}
	return 1
}
