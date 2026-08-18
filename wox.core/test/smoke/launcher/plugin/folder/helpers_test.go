//go:build wox_ui_smoke

package folder

import (
	"path/filepath"
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// folderResults returns the result nodes exposed by the current launcher generation.
func folderResults(snapshot woxwidget.AutomationSnapshot) []woxui.AccessibilityNode {
	results := make([]woxui.AccessibilityNode, 0)
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") {
			results = append(results, node)
		}
	}
	return results
}

// folderChildren filters launcher results to direct children of one filesystem path.
func folderChildren(snapshot woxwidget.AutomationSnapshot, parent string) []woxui.AccessibilityNode {
	parent = filepath.Clean(parent)
	children := make([]woxui.AccessibilityNode, 0)
	for _, result := range folderResults(snapshot) {
		if filepath.Clean(filepath.Dir(result.Description)) == parent {
			children = append(children, result)
		}
	}
	return children
}

// folderResultByPath resolves a dynamic launcher result through its path subtitle.
func folderResultByPath(snapshot woxwidget.AutomationSnapshot, path string) (woxui.AccessibilityNode, bool) {
	path = filepath.Clean(path)
	for _, result := range folderResults(snapshot) {
		if filepath.Clean(result.Description) == path {
			return result, true
		}
	}
	return woxui.AccessibilityNode{}, false
}
