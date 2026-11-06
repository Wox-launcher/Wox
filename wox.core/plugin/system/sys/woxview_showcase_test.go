package sys

import (
	"strings"
	"testing"
	"wox/ui/woxview"
)

// TestWoxViewShowcaseDocuments checks every interactive tree variant against the
// same parser used by plugin windows, including forced resets and removed nodes.
func TestWoxViewShowcaseDocuments(t *testing.T) {
	s := &woxViewShowcase{visible: true}
	for _, state := range []struct {
		visible, changed bool
		revision         uint64
	}{{true, false, 0}, {false, true, 1}, {true, true, 2}} {
		s.visible, s.changed, s.revision = state.visible, state.changed, state.revision
		document := s.document(`{"Handler":"submit","Args":[{"draft":"hello"}]}`)
		tree, err := woxview.Parse([]byte(document))
		if err != nil {
			t.Fatalf("showcase state %+v: %v", state, err)
		}
		if !strings.Contains(document, `"revision":`) || !strings.Contains(document, `"item-39"`) {
			t.Fatalf("showcase is missing revision or virtual rows: %+v", state)
		}
		if state.visible {
			types := map[woxview.NodeType]bool{}
			var visit func(*woxview.Node)
			visit = func(node *woxview.Node) {
				types[node.Type] = true
				for _, child := range node.Children {
					visit(child)
				}
			}
			visit(tree.Root)
			for _, typ := range []woxview.NodeType{woxview.NodeView, woxview.NodeRow, woxview.NodeColumn, woxview.NodeStack, woxview.NodeScroll, woxview.NodeLazyList, woxview.NodeGrid, woxview.NodeSpacer, woxview.NodeText, woxview.NodeSpan, woxview.NodeMarkdown, woxview.NodeImage, woxview.NodeButton, woxview.NodeIconButton, woxview.NodeTextField, woxview.NodeNumberField, woxview.NodeHotkey, woxview.NodeSwitch, woxview.NodeCheckbox, woxview.NodeSlider, woxview.NodeProgress, woxview.NodeLoading, woxview.NodeDivider, woxview.NodeForm, woxview.NodeSubmit, woxview.NodeSelect, woxview.NodeList, woxview.NodeRadioGroup, woxview.NodeTabs, woxview.NodeMultiSelect, woxview.NodeTags, woxview.NodeTag, woxview.NodeBadge, woxview.NodeTable, woxview.NodeCode, woxview.NodeLink} {
				if !types[typ] {
					t.Errorf("showcase has no %s node", typ)
				}
			}
		}
	}
}
