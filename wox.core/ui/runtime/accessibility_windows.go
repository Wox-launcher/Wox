//go:build windows

package woxui

/*
#cgo CXXFLAGS: -std=c++17
#cgo LDFLAGS: -lole32 -loleaut32 -luiautomationcore
#include <stdlib.h>
#include "native_windows.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

func init() {
	updateNativeAccessibility = updateWindowsAccessibility
}

func updateWindowsAccessibility(window *platformWindow, tree AccessibilityTree) error {
	window.mu.Lock()
	hwnd := window.hwnd
	window.mu.Unlock()
	if hwnd == 0 {
		return errors.New("woxui: Windows window is closed")
	}
	owner := C.uintptr_t(hwnd)
	if C.wox_windows_accessibility_begin(owner, C.uint64_t(tree.Generation)) != 0 {
		return errors.New("woxui: failed to begin Windows accessibility update")
	}
	for _, node := range tree.Nodes {
		children := make([]C.uint64_t, len(node.Children))
		for index := range node.Children {
			children[index] = C.uint64_t(node.Children[index])
		}
		var childPointer *C.uint64_t
		if len(children) > 0 {
			childPointer = (*C.uint64_t)(unsafe.Pointer(&children[0]))
		}
		automationID := C.CString(node.AutomationID)
		role := C.CString(string(node.Role))
		label := C.CString(node.Label)
		description := C.CString(node.Description)
		value := C.CString(node.Value)
		liveRegion := 0
		if node.LiveRegion == AccessibilityLiveRegionPolite {
			liveRegion = 1
		} else if node.LiveRegion == AccessibilityLiveRegionAssertive {
			liveRegion = 2
		}
		result := C.wox_windows_accessibility_add_node(
			owner,
			C.uint64_t(node.ID),
			C.uint64_t(node.ParentID),
			childPointer,
			C.int32_t(len(children)),
			automationID,
			role,
			label,
			description,
			value,
			C.float(node.Bounds.X),
			C.float(node.Bounds.Y),
			C.float(node.Bounds.Width),
			C.float(node.Bounds.Height),
			C.uint32_t(accessibilityNodeStateFlags(node)),
			C.uint32_t(accessibilityNodeActionFlags(node)),
			C.int32_t(liveRegion),
		)
		C.free(unsafe.Pointer(automationID))
		C.free(unsafe.Pointer(role))
		C.free(unsafe.Pointer(label))
		C.free(unsafe.Pointer(description))
		C.free(unsafe.Pointer(value))
		if result != 0 {
			return errors.New("woxui: failed to add Windows accessibility node")
		}
	}
	if C.wox_windows_accessibility_end(owner) != 0 {
		return errors.New("woxui: failed to commit Windows accessibility update")
	}
	return nil
}

func windowsAccessibilityObject(hwnd uintptr, wParam uintptr, lParam uintptr) uintptr {
	return uintptr(C.wox_windows_accessibility_get_object(C.uintptr_t(hwnd), C.uintptr_t(wParam), C.uintptr_t(lParam)))
}

func removeWindowsAccessibility(hwnd uintptr) {
	C.wox_windows_accessibility_remove(C.uintptr_t(hwnd))
}

//export woxGoWindowsAccessibilityAction
func woxGoWindowsAccessibilityAction(owner C.uintptr_t, nodeID C.uint64_t, action *C.char, value *C.char) C.int32_t {
	windowValue, ok := nativeWindows.Load(uintptr(owner))
	if !ok {
		return 0
	}
	window := windowValue.(*platformWindow)
	var actionErr error
	callErr := platformCall(func() {
		actionErr = performNativeAccessibilityAction(window, AccessibilityNodeID(nodeID), AccessibilityAction(C.GoString(action)), C.GoString(value))
	})
	if callErr != nil || actionErr != nil {
		return 0
	}
	return 1
}
