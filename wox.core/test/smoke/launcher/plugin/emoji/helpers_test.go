//go:build wox_ui_smoke

package emoji

import (
	"context"
	"strings"
	"testing"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/clipboard"
)

func emojiResults(snapshot woxwidget.AutomationSnapshot) []woxui.AccessibilityNode {
	results := make([]woxui.AccessibilityNode, 0)
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") {
			results = append(results, node)
		}
	}
	return results
}

func emojiResultByLabel(snapshot woxwidget.AutomationSnapshot, labels ...string) (woxui.AccessibilityNode, bool) {
	for _, result := range emojiResults(snapshot) {
		for _, label := range labels {
			if result.Label == label {
				return result, true
			}
		}
	}
	return woxui.AccessibilityNode{}, false
}

func emojiActions(snapshot woxwidget.AutomationSnapshot) []woxui.AccessibilityNode {
	actions := make([]woxui.AccessibilityNode, 0)
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "action-result-") {
			actions = append(actions, node)
		}
	}
	return actions
}

func emojiActionByLabel(snapshot woxwidget.AutomationSnapshot, labels ...string) (woxui.AccessibilityNode, bool) {
	for _, action := range emojiActions(snapshot) {
		for _, label := range labels {
			if action.Label == label {
				return action, true
			}
		}
	}
	return woxui.AccessibilityNode{}, false
}

func waitForClipboardText(t *testing.T, ctx context.Context, expected string) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		text, err := clipboard.ReadText()
		if err == nil && text == expected {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for clipboard text %q: %v", expected, ctx.Err())
		case <-ticker.C:
		}
	}
}
