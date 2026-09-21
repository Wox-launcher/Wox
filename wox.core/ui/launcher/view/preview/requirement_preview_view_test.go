package preview

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestRequirementPreviewMessageRendersMarkdownLinks(t *testing.T) {
	opened := ""
	view := RequirementPreviewView(RequirementPreviewProps{
		Width: 420, Height: 280, Title: "Spotify needs configuration",
		Message: "Create an app at [Spotify Dashboard](https://developer.spotify.com/dashboard).",
		Theme:   woxcomponent.Theme{}, OnOpenLink: func(target string) { opened = target },
	})
	root := view.(woxwidget.Container)
	column := root.Child.(woxwidget.Flex)
	message := column.Children[1].(woxwidget.Container).Child.(woxwidget.Flex)
	paragraph := message.Children[0].(woxwidget.Wrap)
	for _, child := range paragraph.Children {
		if link, ok := child.(woxwidget.Semantics); ok && link.Role == woxui.AccessibilityRoleLink {
			_ = link.OnAction(woxui.AccessibilityActionActivate, "")
			break
		}
	}
	if opened != "https://developer.spotify.com/dashboard" {
		t.Fatalf("opened link = %q, want the requirement message target", opened)
	}
}

func TestRequirementPreviewExposesSaveError(t *testing.T) {
	const message = "failed to persist setting"
	view := RequirementPreviewView(RequirementPreviewProps{
		Width: 420, Height: 280, Title: "Query Requirement Smoke needs configuration",
		Error: message, Theme: woxcomponent.Theme{},
	})
	errorNode, ok := findSemanticsByAutomationID(view, "requirement-form-error")
	if !ok {
		t.Fatal("requirement save error is missing requirement-form-error")
	}
	if errorNode.Role != woxui.AccessibilityRoleText || errorNode.Label != message || errorNode.Value != message {
		t.Fatalf("requirement save error = %#v, want labeled text %q", errorNode, message)
	}
	if errorNode.LiveRegion != woxui.AccessibilityLiveRegionPolite {
		t.Fatalf("requirement save error live region = %q, want polite", errorNode.LiveRegion)
	}
}

// findSemanticsByAutomationID walks a preview widget tree for one stable automation node.
func findSemanticsByAutomationID(widget woxwidget.Widget, automationID string) (woxwidget.Semantics, bool) {
	switch node := widget.(type) {
	case woxwidget.Semantics:
		if node.AutomationID == automationID {
			return node, true
		}
		return findSemanticsByAutomationID(node.Child, automationID)
	case woxwidget.Keyed:
		return findSemanticsByAutomationID(node.Child, automationID)
	case woxwidget.Container:
		return findSemanticsByAutomationID(node.Child, automationID)
	case woxwidget.Expanded:
		return findSemanticsByAutomationID(node.Child, automationID)
	case woxwidget.Align:
		return findSemanticsByAutomationID(node.Child, automationID)
	case woxwidget.ScrollView:
		return findSemanticsByAutomationID(node.Child, automationID)
	case woxwidget.Flex:
		for _, child := range node.Children {
			if found, ok := findSemanticsByAutomationID(child, automationID); ok {
				return found, true
			}
		}
	}
	return woxwidget.Semantics{}, false
}
