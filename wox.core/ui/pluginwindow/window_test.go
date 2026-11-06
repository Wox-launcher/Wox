package pluginwindow

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"wox/plugin"
	ui "wox/ui/runtime"
	"wox/ui/woxview"
)

// TestWindowPresentation preserves overlay defaults and explicit application-window options.
func TestWindowPresentation(t *testing.T) {
	for _, presentation := range []plugin.WindowPresentation{"", plugin.WindowPresentationOverlay} {
		options := plugin.WindowOptions{Presentation: presentation}
		role, err := normalizePresentation(&options)
		if err != nil || role != ui.WindowRoleUtility || options.Focusable != nil || options.Topmost != nil {
			t.Fatalf("overlay defaults changed: %+v, %v", options, err)
		}
	}
	options := plugin.WindowOptions{Presentation: plugin.WindowPresentationWindow}
	role, err := normalizePresentation(&options)
	if err != nil || role != ui.WindowRoleApplication || !boolValue(options.Focusable, false) || boolValue(options.Topmost, true) {
		t.Fatalf("application defaults: %+v, %v", options, err)
	}
	yes, no := true, false
	options.Focusable, options.Topmost = &no, &yes
	if _, err := normalizePresentation(&options); err != nil || *options.Focusable || !*options.Topmost {
		t.Fatal("explicit options overwritten")
	}
	if _, err := normalizePresentation(&plugin.WindowOptions{Presentation: "unknown"}); err == nil {
		t.Fatal("accepted unknown presentation")
	}
}

// TestWindowInputValidation checks untrusted geometry and content before native allocation.
func TestWindowInputValidation(t *testing.T) {
	for _, value := range []float64{0, -1, math.NaN(), math.Inf(1), 16385} {
		if validateGeometry(value, 100, plugin.WindowAnchorCenter, 0, 0) == nil {
			t.Fatalf("accepted width %v", value)
		}
	}
	if validateGeometry(360, 160, plugin.WindowAnchorCenter, -500, -80) != nil {
		t.Fatal("negative desktop offsets rejected")
	}
	if validateGeometry(360, 160, "unknown", 0, 0) == nil {
		t.Fatal("accepted unknown anchor")
	}
	if _, err := prepareContent(plugin.WindowContent{Type: "url"}); err == nil {
		t.Fatal("accepted unsupported content")
	}
	if _, err := prepareContent(plugin.WindowContent{Type: plugin.WindowContentHTML, Data: strings.Repeat("x", 4*1024*1024+1)}); err == nil {
		t.Fatal("accepted oversized content")
	}
	if _, err := prepareContent(plugin.WindowContent{Type: plugin.WindowContentWoxView, Data: `{"version":1,"root":{"type":"div"}}`}); err == nil {
		t.Fatal("accepted invalid woxview tree")
	}
}

// TestTextAndMarkdownBecomeWoxViewTrees keeps the shorthand types on the single native render path.
func TestTextAndMarkdownBecomeWoxViewTrees(t *testing.T) {
	text, err := prepareContent(plugin.WindowContent{Type: plugin.WindowContentText, Data: "hello"})
	if err != nil || text.Root.Type != woxview.NodeScroll || text.Root.Children[0].Props.(*woxview.TextProps).Text != "hello" || !text.Root.Children[0].Props.(*woxview.TextProps).Selectable {
		t.Fatalf("text tree: %+v, %v", text, err)
	}
	markdown, err := prepareContent(plugin.WindowContent{Type: plugin.WindowContentMarkdown, Data: "# Title"})
	if err != nil || markdown.Root.Children[0].Type != woxview.NodeMarkdown {
		t.Fatalf("markdown tree: %+v, %v", markdown, err)
	}
	if html, err := prepareContent(plugin.WindowContent{Type: plugin.WindowContentHTML, Data: "<p></p>"}); err != nil || html != nil {
		t.Fatal("html must not produce a woxview tree")
	}
}

// TestWoxViewUpdateRequiresWoxViewDocument rejects updates that would silently change a text or HTML window.
func TestWoxViewUpdateRequiresWoxViewDocument(t *testing.T) {
	if (&session{}).UpdateContent(plugin.WindowContentUpdateOption{Data: "{"}) == nil {
		t.Fatal("accepted malformed update")
	}
}

func TestUpdateRejectsStaleDocumentAndSequence(t *testing.T) {
	s := &session{document: "new", sequence: 3}
	for _, o := range []plugin.WindowContentUpdateOption{
		{DocumentID: "old", Sequence: 4}, {DocumentID: "new", Sequence: 3}, {DocumentID: "new", Sequence: 0},
	} {
		if s.validateUpdate(o) == nil {
			t.Fatalf("accepted stale update %+v", o)
		}
	}
	if err := s.validateUpdate(plugin.WindowContentUpdateOption{DocumentID: "new", Sequence: 4}); err != nil {
		t.Fatal(err)
	}
}

// TestDocumentMessageIsolation exercises authentication and one ready notification per document.
func TestDocumentMessageIsolation(t *testing.T) {
	var events []plugin.WindowEvent
	s := &session{document: "current", content: plugin.WindowContent{Type: plugin.WindowContentHTML}, emit: func(e plugin.WindowEvent) { events = append(events, e) }}
	s.message(`{"Token":"old","Type":"message","Data":{"private":1}}`)
	s.message(`{"Token":"current","Type":"message","Data":`)
	s.message(`{"Token":"current","Type":"ready"}`)
	s.message(`{"Token":"current","Type":"ready"}`)
	s.message(`{"Token":"current","Type":"message","Data":{"snake_case":false,"__proto__":{"x":1}}}`)
	if len(events) != 2 || events[0].Type != plugin.WindowEventReady || events[1].Type != plugin.WindowEventMessage {
		t.Fatalf("events: %+v", events)
	}
	var data map[string]any
	if json.Unmarshal(events[1].Data, &data) != nil || data["snake_case"] != false || data["__proto__"] == nil {
		t.Fatal("message data changed")
	}
	s.closed = true
	s.message(`{"Token":"current","Type":"message","Data":null}`)
	if len(events) != 2 {
		t.Fatal("closed window delivered message")
	}
}

// TestBridgeHTMLIncludesDecodedScript verifies the bridge in both normal and packed builds.
func TestBridgeHTMLIncludesDecodedScript(t *testing.T) {
	html := bridgeHTML("<!doctype html><html><body>Content</body></html>", "document-123")
	if !strings.HasPrefix(html, "<!doctype html><script>") || !strings.Contains(html, `const token = "document-123";`) || !strings.Contains(html, `Object.defineProperty(window, "woxWindow"`) || strings.Contains(html, "__WOX_DOCUMENT_TOKEN__") {
		t.Fatal("bridge was not inserted with the document token")
	}
}

func TestWindowCenterInWorkArea(t *testing.T) {
	area := ui.Rect{X: -1600, Y: -100, Width: 1600, Height: 860}
	if !windowCenterInWorkArea(ui.Rect{X: -1000, Y: 100, Width: 200, Height: 100}, area) || windowCenterInWorkArea(ui.Rect{X: 100, Y: 100, Width: 200, Height: 100}, area) {
		t.Fatal("display containment did not use logical desktop coordinates")
	}
}

// TestContentDragKeepsClicksAndStartsNativeDragging checks the pointer arbitration used by HTML windows.
func TestContentDragKeepsClicksAndStartsNativeDragging(t *testing.T) {
	var d contentDrag
	var forwarded []ui.PointerEvent
	starts := 0
	forward := func(e ui.PointerEvent) bool { forwarded = append(forwarded, e); return true }
	drag := func() { starts++ }
	down := ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 10, Y: 10}}
	up := ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 10, Y: 10}}
	d.pointer(down, forward, drag)
	if len(forwarded) != 0 {
		t.Fatal("press forwarded before click or drag was known")
	}
	d.pointer(up, forward, drag)
	if len(forwarded) != 2 || forwarded[0].Kind != ui.PointerDown || forwarded[1].Kind != ui.PointerUp || starts != 0 {
		t.Fatalf("click: forwarded=%v starts=%d", forwarded, starts)
	}
	forwarded = nil
	d.pointer(down, forward, drag)
	d.pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 15, Y: 10}}, forward, drag)
	if len(forwarded) != 0 || starts != 1 {
		t.Fatalf("drag: forwarded=%v starts=%d", forwarded, starts)
	}
}

func TestHideChromeDoesNotChangeClosePermission(t *testing.T) {
	hidden := true
	s := session{options: plugin.WindowOptions{HideChrome: &hidden}}
	if s.chromeHeight() != 0 {
		t.Fatal("hidden chrome reserved space")
	}
	s.options.HideChrome = nil
	if s.chromeHeight() != 30 {
		t.Fatal("default chrome was not restored")
	}
}
