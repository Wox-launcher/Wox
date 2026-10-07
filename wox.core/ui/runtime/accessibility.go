package woxui

import (
	window "wox/ui/runtime/internal/window"
)

// AccessibilityNodeID remains stable while one retained UI element survives reconciliation.
type AccessibilityNodeID = window.AccessibilityNodeID

// AccessibilityRole describes the user-facing purpose of one rendered element.
type AccessibilityRole = window.AccessibilityRole

const (
	AccessibilityRoleWindow      = window.AccessibilityRoleWindow
	AccessibilityRoleGroup       = window.AccessibilityRoleGroup
	AccessibilityRoleText        = window.AccessibilityRoleText
	AccessibilityRoleHeading     = window.AccessibilityRoleHeading
	AccessibilityRoleButton      = window.AccessibilityRoleButton
	AccessibilityRoleTextField   = window.AccessibilityRoleTextField
	AccessibilityRoleCheckBox    = window.AccessibilityRoleCheckBox
	AccessibilityRoleRadioButton = window.AccessibilityRoleRadioButton
	AccessibilityRoleList        = window.AccessibilityRoleList
	AccessibilityRoleListItem    = window.AccessibilityRoleListItem
	AccessibilityRoleImage       = window.AccessibilityRoleImage
	AccessibilityRoleProgressBar = window.AccessibilityRoleProgressBar
	AccessibilityRoleSlider      = window.AccessibilityRoleSlider
	AccessibilityRoleLink        = window.AccessibilityRoleLink
	AccessibilityRoleMenu        = window.AccessibilityRoleMenu
	AccessibilityRoleMenuItem    = window.AccessibilityRoleMenuItem
	AccessibilityRoleDialog      = window.AccessibilityRoleDialog
)

// AccessibilityAction identifies an operation exposed to assistive technology or automation.
type AccessibilityAction = window.AccessibilityAction

const (
	AccessibilityActionFocus     = window.AccessibilityActionFocus
	AccessibilityActionActivate  = window.AccessibilityActionActivate
	AccessibilityActionSetValue  = window.AccessibilityActionSetValue
	AccessibilityActionToggle    = window.AccessibilityActionToggle
	AccessibilityActionIncrement = window.AccessibilityActionIncrement
	AccessibilityActionDecrement = window.AccessibilityActionDecrement
	AccessibilityActionScroll    = window.AccessibilityActionScroll
	AccessibilityActionDismiss   = window.AccessibilityActionDismiss
	AccessibilityActionSelectAll = window.AccessibilityActionSelectAll
	AccessibilityActionCopy      = window.AccessibilityActionCopy
	AccessibilityActionCut       = window.AccessibilityActionCut
	AccessibilityActionPaste     = window.AccessibilityActionPaste
)

// AccessibilityLiveRegion controls how value changes are announced.
type AccessibilityLiveRegion = window.AccessibilityLiveRegion

const (
	AccessibilityLiveRegionNone      = window.AccessibilityLiveRegionNone
	AccessibilityLiveRegionPolite    = window.AccessibilityLiveRegionPolite
	AccessibilityLiveRegionAssertive = window.AccessibilityLiveRegionAssertive
)

// AccessibilityNode is an immutable snapshot of one element in logical client coordinates.
type AccessibilityNode = window.AccessibilityNode

// AccessibilityTextLine is one painted wrap line, including hanging indent in logical units.
type AccessibilityTextLine = window.AccessibilityTextLine

// AccessibilityTree is the versioned snapshot consumed by native bridges and test automation.
type AccessibilityTree = window.AccessibilityTree

// AccessibilityUpdate is reserved for a future incremental native accessibility path.
// Nothing currently publishes or consumes it; UpdateAccessibility still takes a full tree.
type AccessibilityUpdate = window.AccessibilityUpdate

// DiffAccessibilityTrees builds a full snapshot or a node-level delta against the previous tree.
// Host does not call this: native UpdateAccessibility still consumes a full tree, so an unused
// delta would only add CPU. Keep the helper for a future incremental native path.
func DiffAccessibilityTrees(previous, next AccessibilityTree, forceFull bool) AccessibilityUpdate {
	return window.DiffAccessibilityTrees(previous, next, forceFull)
}

// AccessibilityActionHandler applies one action on the UI thread.
type AccessibilityActionHandler = window.AccessibilityActionHandler

// SelectedTextFromFocusedWindow returns selected text from a focused Wox editor.
// handled is true when a Wox window owns focus, even if the caret is collapsed,
// so callers skip OS selection capture against our own UI.
func SelectedTextFromFocusedWindow() (text string, handled bool) {
	return window.SelectedTextFromFocusedWindow()
}
