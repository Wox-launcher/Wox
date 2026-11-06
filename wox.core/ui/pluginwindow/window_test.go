package pluginwindow

import (
	"math"
	"strings"
	"testing"
	"wox/plugin"
	ui "wox/ui/runtime"
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
	if _, err := prepareContent(plugin.WindowContent{Type: plugin.WindowContentWoxView, Data: "https://example.com"}); err == nil {
		t.Fatal("accepted unsupported content")
	}
	if _, err := prepareContent(plugin.WindowContent{Type: plugin.WindowContentWoxView, Data: strings.Repeat("x", 4*1024*1024+1)}); err == nil {
		t.Fatal("accepted oversized content")
	}
	if _, err := prepareContent(plugin.WindowContent{Type: plugin.WindowContentWoxView, Data: `{"version":1,"root":{"type":"div"}}`}); err == nil {
		t.Fatal("accepted invalid woxview tree")
	}
}

// TestWoxViewUpdateValidation rejects malformed documents before dispatching to the UI thread.
func TestWoxViewUpdateValidation(t *testing.T) {
	if (&session{}).UpdateContent(plugin.WindowContentUpdateOption{Data: "{"}) == nil {
		t.Fatal("accepted malformed update")
	}
}

// TestWindowOnlyAcceptsWoxView rejects former shorthand content before native allocation or mutation.
func TestWindowOnlyAcceptsWoxView(t *testing.T) {
	for _, data := range []string{"", "plain text", "# Markdown", "<p>HTML</p>", `{"html":"<p>HTML</p>"}`} {
		t.Run(data, func(t *testing.T) {
			content := plugin.WindowContent{Type: plugin.WindowContentWoxView, Data: data}
			if _, err := Create("invalid", plugin.CreateWindowOption{Content: content}, nil); err == nil {
				t.Fatal("created window with non-WoxView content")
			}
			s := &session{document: "current", sequence: 7}
			if s.SetContent(content) == nil || s.UpdateContent(plugin.WindowContentUpdateOption{Data: data}) == nil {
				t.Fatal("accepted non-WoxView replacement or update")
			}
			if s.document != "current" || s.sequence != 7 {
				t.Fatal("invalid content mutated the document")
			}
		})
	}
}

// TestWindowRejectsUnsupportedContentTypes checks the discriminator even with a valid WoxView payload.
func TestWindowRejectsUnsupportedContentTypes(t *testing.T) {
	for _, kind := range []plugin.WindowContentType{"", "text", "markdown", "html", "unknown"} {
		t.Run(string(kind), func(t *testing.T) {
			content := plugin.WindowContent{Type: kind, Data: `{"version":1,"root":{"type":"text","props":{"text":"hello"}}}`}
			if _, err := Create("invalid", plugin.CreateWindowOption{Content: content}, nil); err == nil || err.Error() != "unsupported window content type" {
				t.Fatalf("unexpected creation result: %v", err)
			}
			if err := (&session{}).SetContent(content); err == nil || err.Error() != "unsupported window content type" {
				t.Fatalf("unexpected replacement result: %v", err)
			}
		})
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

func TestWindowCenterInWorkArea(t *testing.T) {
	area := ui.Rect{X: -1600, Y: -100, Width: 1600, Height: 860}
	if !windowCenterInWorkArea(ui.Rect{X: -1000, Y: 100, Width: 200, Height: 100}, area) || windowCenterInWorkArea(ui.Rect{X: 100, Y: 100, Width: 200, Height: 100}, area) {
		t.Fatal("display containment did not use logical desktop coordinates")
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
