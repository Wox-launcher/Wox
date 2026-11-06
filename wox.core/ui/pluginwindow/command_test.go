package pluginwindow

import (
	"encoding/json"
	"strings"
	"testing"

	"wox/plugin"
	ui "wox/ui/runtime"
	"wox/ui/widget"
	"wox/ui/woxview"
)

// TestContentCommandDocumentLifecycle exercises the real adapter and retained Host without native allocation.
func TestContentCommandDocumentLifecycle(t *testing.T) {
	content := plugin.WindowContent{Type: plugin.WindowContentWoxView, Data: `{"version":1,"root":{"type":"column","children":[{"type":"text-field","key":"draft","props":{"value":"before"}},{"type":"text-field","key":"disabled","props":{"disabled":true}}]}}`}
	tree, err := prepareContent(content)
	if err != nil {
		t.Fatal(err)
	}
	flag := true
	var events []plugin.WindowEvent
	s := &session{document: "doc", options: plugin.WindowOptions{Focusable: &flag, HideChrome: &flag}, view: woxview.NewView(), emit: func(e plugin.WindowEvent) { events = append(events, e) }}
	s.view.SetTree(tree)
	s.host = widget.NewHost(s.build)
	s.host.AttachServices(translationTestServices{})
	defer s.host.Dispose()
	render := func() {
		for i := 0; i < 3; i++ {
			s.host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 300, Height: 200}, PixelSize: ui.PixelSize{Width: 450, Height: 300}, Scale: 1.5})
		}
	}
	option := plugin.WindowContentCommandOption{DocumentID: "old", Command: plugin.WindowContentCommandSelectAll, Key: "draft"}
	if err := s.enqueueContentCommand(option); err == nil || err.Error() != "document replaced" {
		t.Fatalf("stale document guard: %v", err)
	}
	option.DocumentID = "doc"
	// Ready callbacks may issue commands before the first namespaced build.
	if err := s.enqueueContentCommand(option); err != nil {
		t.Fatal(err)
	}
	render()
	s.host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "after"})
	render()
	found := false
	for _, node := range s.host.Snapshot().Tree.Nodes {
		if node.AutomationID == "plugin-window/doc/column0/k:draft/text-field/field" {
			found = node.Value == "after"
		}
	}
	if !found || len(events) != 0 {
		t.Fatalf("first-frame selection failed: %+v", events)
	}
	option.Key = "disabled"
	if err := s.enqueueContentCommand(option); err != nil {
		t.Fatal(err)
	}
	render()
	if len(events) != 1 || events[0].Type != plugin.WindowEventError || events[0].DocumentID != "doc" || !strings.Contains(events[0].ErrMsg, "no enabled control") {
		t.Fatalf("deferred error: %+v", events)
	}
	var detail plugin.WindowContentCommandOption
	if json.Unmarshal(events[0].Data, &detail) != nil || detail.Key != "disabled" || detail.Command != plugin.WindowContentCommandSelectAll {
		t.Fatalf("missing command failure metadata: %s", events[0].Data)
	}
	option.Key = "draft"
	if err := s.enqueueContentCommand(option); err != nil {
		t.Fatal(err)
	}
	// This is the document/View replacement performed by setContent; the old
	// queue must not apply even though the new tree reuses the same node key.
	s.document, s.view = "new-doc", woxview.NewView()
	s.view.SetTree(tree)
	render()
	if s.host.FocusedKey() != "" || len(events) != 1 {
		t.Fatalf("old command crossed documents: focus=%s, events=%+v", s.host.FocusedKey(), events)
	}
	option.DocumentID = ""
	s.options.Focusable = nil
	if err := s.enqueueContentCommand(option); err == nil || !strings.Contains(err.Error(), "focusable") {
		t.Fatalf("nonfocusable window: %v", err)
	}
	option.Command = plugin.WindowContentCommandScrollTo
	if err := s.enqueueContentCommand(option); err != nil {
		t.Fatalf("scrolling nonfocusable window: %v", err)
	}
	s.view = nil
	if err := s.enqueueContentCommand(option); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("missing content command: %v", err)
	}
	s.closed = true
	if err := s.enqueueContentCommand(option); err == nil || err.Error() != "window closed" {
		t.Fatalf("closed command: %v", err)
	}
}
