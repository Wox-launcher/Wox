package woxui

import "testing"

func TestAccessibilityTreeContentHashIgnoresGeneration(t *testing.T) {
	tree := AccessibilityTree{Generation: 1, RootIDs: []AccessibilityNodeID{1}, Nodes: []AccessibilityNode{{ID: 1}}}
	changed := cloneAccessibilityTree(tree)
	changed.Generation = 2
	if accessibilityTreeContentHash(tree) != accessibilityTreeContentHash(changed) {
		t.Fatal("generation-only changes should not rebuild native accessibility objects")
	}
}

func TestAccessibilityTreeContentHashCoversEveryNodeField(t *testing.T) {
	base := AccessibilityTree{Nodes: []AccessibilityNode{{}}}
	cases := []struct {
		name   string
		change func(*AccessibilityTree)
	}{
		{name: "root IDs", change: func(tree *AccessibilityTree) { tree.RootIDs = []AccessibilityNodeID{1} }},
		{name: "ID", change: func(tree *AccessibilityTree) { tree.Nodes[0].ID = 1 }},
		{name: "parent ID", change: func(tree *AccessibilityTree) { tree.Nodes[0].ParentID = 1 }},
		{name: "children", change: func(tree *AccessibilityTree) { tree.Nodes[0].Children = []AccessibilityNodeID{1} }},
		{name: "automation ID", change: func(tree *AccessibilityTree) { tree.Nodes[0].AutomationID = "query" }},
		{name: "role", change: func(tree *AccessibilityTree) { tree.Nodes[0].Role = AccessibilityRoleTextField }},
		{name: "label", change: func(tree *AccessibilityTree) { tree.Nodes[0].Label = "Query" }},
		{name: "description", change: func(tree *AccessibilityTree) { tree.Nodes[0].Description = "Description" }},
		{name: "value", change: func(tree *AccessibilityTree) { tree.Nodes[0].Value = "Value" }},
		{name: "bounds x", change: func(tree *AccessibilityTree) { tree.Nodes[0].Bounds.X = 1 }},
		{name: "bounds y", change: func(tree *AccessibilityTree) { tree.Nodes[0].Bounds.Y = 1 }},
		{name: "bounds width", change: func(tree *AccessibilityTree) { tree.Nodes[0].Bounds.Width = 1 }},
		{name: "bounds height", change: func(tree *AccessibilityTree) { tree.Nodes[0].Bounds.Height = 1 }},
		{name: "actions", change: func(tree *AccessibilityTree) {
			tree.Nodes[0].Actions = []AccessibilityAction{AccessibilityActionActivate}
		}},
		{name: "live region", change: func(tree *AccessibilityTree) { tree.Nodes[0].LiveRegion = AccessibilityLiveRegionPolite }},
		{name: "enabled", change: func(tree *AccessibilityTree) { tree.Nodes[0].Enabled = true }},
		{name: "focusable", change: func(tree *AccessibilityTree) { tree.Nodes[0].Focusable = true }},
		{name: "focused", change: func(tree *AccessibilityTree) { tree.Nodes[0].Focused = true }},
		{name: "selected", change: func(tree *AccessibilityTree) { tree.Nodes[0].Selected = true }},
		{name: "checked", change: func(tree *AccessibilityTree) { tree.Nodes[0].Checked = true }},
		{name: "expanded", change: func(tree *AccessibilityTree) { tree.Nodes[0].Expanded = true }},
		{name: "read only", change: func(tree *AccessibilityTree) { tree.Nodes[0].ReadOnly = true }},
		{name: "protected", change: func(tree *AccessibilityTree) { tree.Nodes[0].Protected = true }},
		{name: "hidden", change: func(tree *AccessibilityTree) { tree.Nodes[0].Hidden = true }},
		{name: "native boundary", change: func(tree *AccessibilityTree) { tree.Nodes[0].NativeBoundary = true }},
	}
	baseHash := accessibilityTreeContentHash(base)
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			changed := cloneAccessibilityTree(base)
			test.change(&changed)
			if accessibilityTreeContentHash(changed) == baseHash {
				t.Fatalf("field change did not affect accessibility content hash: %+v", changed)
			}
		})
	}
}
