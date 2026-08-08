//go:build darwin

package woxui

/*
#cgo CFLAGS: -fblocks -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "native_darwin.h"
*/
import "C"

import (
	"errors"
	"runtime/cgo"
	"unsafe"
)

func init() {
	updateNativeAccessibility = updateDarwinAccessibility
}

func updateDarwinAccessibility(window *platformWindow, tree AccessibilityTree) error {
	native, err := window.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_accessibility_begin(native, C.uint64_t(tree.Generation)) != 0 {
		return errors.New("woxui: failed to begin macOS accessibility update")
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
		result := C.wox_darwin_accessibility_add_node(
			native,
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
			return errors.New("woxui: failed to add macOS accessibility node")
		}
	}
	if C.wox_darwin_accessibility_end(native) != 0 {
		return errors.New("woxui: failed to commit macOS accessibility update")
	}
	return nil
}

//export woxGoDarwinAccessibilityAction
func woxGoDarwinAccessibilityAction(context C.uintptr_t, nodeID C.uint64_t, action *C.char, value *C.char) C.int32_t {
	window := cgo.Handle(context).Value().(*platformWindow)
	var actionErr error
	callErr := platformCall(func() {
		actionErr = performNativeAccessibilityAction(window, AccessibilityNodeID(nodeID), AccessibilityAction(C.GoString(action)), C.GoString(value))
	})
	if callErr != nil || actionErr != nil {
		return 0
	}
	return 1
}
