package woxui

import (
	"errors"
	"reflect"
	"sync"
)

// AccessibilityNodeID remains stable while one retained UI element survives reconciliation.
type AccessibilityNodeID uint64

// AccessibilityRole describes the user-facing purpose of one rendered element.
type AccessibilityRole string

const (
	AccessibilityRoleWindow      AccessibilityRole = "window"
	AccessibilityRoleGroup       AccessibilityRole = "group"
	AccessibilityRoleText        AccessibilityRole = "text"
	AccessibilityRoleHeading     AccessibilityRole = "heading"
	AccessibilityRoleButton      AccessibilityRole = "button"
	AccessibilityRoleTextField   AccessibilityRole = "text_field"
	AccessibilityRoleCheckBox    AccessibilityRole = "checkbox"
	AccessibilityRoleRadioButton AccessibilityRole = "radio_button"
	AccessibilityRoleList        AccessibilityRole = "list"
	AccessibilityRoleListItem    AccessibilityRole = "list_item"
	AccessibilityRoleImage       AccessibilityRole = "image"
	AccessibilityRoleProgressBar AccessibilityRole = "progress_bar"
	AccessibilityRoleLink        AccessibilityRole = "link"
	AccessibilityRoleMenu        AccessibilityRole = "menu"
	AccessibilityRoleMenuItem    AccessibilityRole = "menu_item"
	AccessibilityRoleDialog      AccessibilityRole = "dialog"
	AccessibilityRoleWebView     AccessibilityRole = "web_view"
)

// AccessibilityAction identifies an operation exposed to assistive technology or automation.
type AccessibilityAction string

const (
	AccessibilityActionFocus     AccessibilityAction = "focus"
	AccessibilityActionActivate  AccessibilityAction = "activate"
	AccessibilityActionSetValue  AccessibilityAction = "set_value"
	AccessibilityActionToggle    AccessibilityAction = "toggle"
	AccessibilityActionIncrement AccessibilityAction = "increment"
	AccessibilityActionDecrement AccessibilityAction = "decrement"
	AccessibilityActionScroll    AccessibilityAction = "scroll"
	AccessibilityActionDismiss   AccessibilityAction = "dismiss"
)

// AccessibilityLiveRegion controls how value changes are announced.
type AccessibilityLiveRegion string

const (
	AccessibilityLiveRegionNone      AccessibilityLiveRegion = ""
	AccessibilityLiveRegionPolite    AccessibilityLiveRegion = "polite"
	AccessibilityLiveRegionAssertive AccessibilityLiveRegion = "assertive"
)

// AccessibilityNode is an immutable snapshot of one element in logical client coordinates.
type AccessibilityNode struct {
	ID             AccessibilityNodeID
	ParentID       AccessibilityNodeID
	Children       []AccessibilityNodeID
	AutomationID   string
	Role           AccessibilityRole
	Label          string
	Description    string
	Value          string
	Bounds         Rect
	Actions        []AccessibilityAction
	LiveRegion     AccessibilityLiveRegion
	Enabled        bool
	Focusable      bool
	Focused        bool
	Selected       bool
	Checked        bool
	Expanded       bool
	ReadOnly       bool
	Protected      bool
	Hidden         bool
	NativeBoundary bool
}

// AccessibilityTree is the versioned snapshot consumed by native bridges and test automation.
type AccessibilityTree struct {
	Generation uint64
	RootIDs    []AccessibilityNodeID
	Nodes      []AccessibilityNode
}

// AccessibilityActionHandler applies one action on the UI thread.
type AccessibilityActionHandler func(nodeID AccessibilityNodeID, action AccessibilityAction, value string) error

type accessibilityWindowState struct {
	tree    AccessibilityTree
	handler AccessibilityActionHandler
}

var accessibilityWindows sync.Map

// UpdateAccessibility publishes a new immutable tree for this window.
func (w *Window) UpdateAccessibility(tree AccessibilityTree, handler AccessibilityActionHandler) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	tree = cloneAccessibilityTree(tree)
	previousValue, hadPrevious := accessibilityWindows.Load(w.native)
	accessibilityWindows.Store(w.native, accessibilityWindowState{tree: tree, handler: handler})
	if hadPrevious && accessibilityTreeContentEqual(previousValue.(accessibilityWindowState).tree, tree) {
		return nil
	}
	return updateNativeAccessibility(w.native, tree)
}

// accessibilityTreeContentEqual ignores frame generation so unchanged retained trees do not rebuild native objects.
func accessibilityTreeContentEqual(left AccessibilityTree, right AccessibilityTree) bool {
	left.Generation = 0
	right.Generation = 0
	return reflect.DeepEqual(left, right)
}

// AccessibilitySnapshot returns a detached copy suitable for automation readers.
func (w *Window) AccessibilitySnapshot() AccessibilityTree {
	if w == nil || w.native == nil {
		return AccessibilityTree{}
	}
	value, ok := accessibilityWindows.Load(w.native)
	if !ok {
		return AccessibilityTree{}
	}
	return cloneAccessibilityTree(value.(accessibilityWindowState).tree)
}

// PerformAccessibilityAction dispatches a native or automation action to the current UI tree.
func (w *Window) PerformAccessibilityAction(nodeID AccessibilityNodeID, action AccessibilityAction, value string) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	valueState, ok := accessibilityWindows.Load(w.native)
	if !ok || valueState.(accessibilityWindowState).handler == nil {
		return errors.New("accessibility action handler is not initialized")
	}
	return valueState.(accessibilityWindowState).handler(nodeID, action, value)
}

func cloneAccessibilityTree(tree AccessibilityTree) AccessibilityTree {
	clone := tree
	clone.RootIDs = append([]AccessibilityNodeID(nil), tree.RootIDs...)
	clone.Nodes = append([]AccessibilityNode(nil), tree.Nodes...)
	for index := range clone.Nodes {
		clone.Nodes[index].Children = append([]AccessibilityNodeID(nil), tree.Nodes[index].Children...)
		clone.Nodes[index].Actions = append([]AccessibilityAction(nil), tree.Nodes[index].Actions...)
	}
	return clone
}

func clearAccessibility(window *platformWindow) {
	if window == nil {
		return
	}
	accessibilityWindows.Delete(window)
	_ = updateNativeAccessibility(window, AccessibilityTree{})
}

const (
	accessibilityStateEnabled uint32 = 1 << iota
	accessibilityStateFocusable
	accessibilityStateFocused
	accessibilityStateSelected
	accessibilityStateChecked
	accessibilityStateExpanded
	accessibilityStateReadOnly
	accessibilityStateProtected
	accessibilityStateHidden
)

const (
	accessibilityActionFocusFlag uint32 = 1 << iota
	accessibilityActionActivateFlag
	accessibilityActionSetValueFlag
	accessibilityActionToggleFlag
	accessibilityActionIncrementFlag
	accessibilityActionDecrementFlag
	accessibilityActionScrollFlag
	accessibilityActionDismissFlag
)

func accessibilityNodeStateFlags(node AccessibilityNode) uint32 {
	var flags uint32
	if node.Enabled {
		flags |= accessibilityStateEnabled
	}
	if node.Focusable {
		flags |= accessibilityStateFocusable
	}
	if node.Focused {
		flags |= accessibilityStateFocused
	}
	if node.Selected {
		flags |= accessibilityStateSelected
	}
	if node.Checked {
		flags |= accessibilityStateChecked
	}
	if node.Expanded {
		flags |= accessibilityStateExpanded
	}
	if node.ReadOnly {
		flags |= accessibilityStateReadOnly
	}
	if node.Protected {
		flags |= accessibilityStateProtected
	}
	if node.Hidden {
		flags |= accessibilityStateHidden
	}
	return flags
}

func accessibilityNodeActionFlags(node AccessibilityNode) uint32 {
	var flags uint32
	for _, action := range node.Actions {
		switch action {
		case AccessibilityActionFocus:
			flags |= accessibilityActionFocusFlag
		case AccessibilityActionActivate:
			flags |= accessibilityActionActivateFlag
		case AccessibilityActionSetValue:
			flags |= accessibilityActionSetValueFlag
		case AccessibilityActionToggle:
			flags |= accessibilityActionToggleFlag
		case AccessibilityActionIncrement:
			flags |= accessibilityActionIncrementFlag
		case AccessibilityActionDecrement:
			flags |= accessibilityActionDecrementFlag
		case AccessibilityActionScroll:
			flags |= accessibilityActionScrollFlag
		case AccessibilityActionDismiss:
			flags |= accessibilityActionDismissFlag
		}
	}
	return flags
}

func performNativeAccessibilityAction(window *platformWindow, nodeID AccessibilityNodeID, action AccessibilityAction, value string) error {
	stateValue, ok := accessibilityWindows.Load(window)
	if !ok || stateValue.(accessibilityWindowState).handler == nil {
		return errors.New("accessibility action handler is not initialized")
	}
	return stateValue.(accessibilityWindowState).handler(nodeID, action, value)
}
