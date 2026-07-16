package woxui

import "testing"

func TestAccessibilityTreeContentEqualIgnoresGeneration(t *testing.T) {
	left := AccessibilityTree{
		Generation: 1,
		RootIDs:    []AccessibilityNodeID{1},
		Nodes: []AccessibilityNode{{
			ID: 1, AutomationID: "query", Role: AccessibilityRoleTextField,
			Label: "Query", Actions: []AccessibilityAction{AccessibilityActionSetValue},
		}},
	}
	right := cloneAccessibilityTree(left)
	right.Generation = 2
	if !accessibilityTreeContentEqual(left, right) {
		t.Fatal("generation-only changes should not rebuild native accessibility objects")
	}
	right.Nodes[0].Focused = true
	if accessibilityTreeContentEqual(left, right) {
		t.Fatal("semantic state changes must reach the native accessibility bridge")
	}
}
