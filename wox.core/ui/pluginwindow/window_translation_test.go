package pluginwindow

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"wox/plugin"
	ui "wox/ui/runtime"
	widget "wox/ui/widget"
	"wox/ui/woxview"
)

// TestWindowMarksChangesWithoutLeakingQueueMetadata verifies the UI-to-plugin queue capability boundary.
func TestWindowMarksChangesWithoutLeakingQueueMetadata(t *testing.T) {
	content := plugin.WindowContent{Type: plugin.WindowContentWoxView, Data: `{"version":1,"root":{"type":"text-field","key":"draft","props":{"label":"Draft","onChange":{"$handler":"edit"}}}}`}
	tree, err := prepareContent(content)
	if err != nil {
		t.Fatal(err)
	}
	hideChrome := true
	var events []plugin.WindowEvent
	s := &session{content: content, document: "doc", options: plugin.WindowOptions{HideChrome: &hideChrome}, view: woxview.NewView(), emit: func(e plugin.WindowEvent) { events = append(events, e) }}
	s.view.SetTree(tree)
	host := widget.NewHost(s.build)
	host.AttachServices(translationTestServices{})
	defer host.Dispose()
	host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 300, Height: 200}, Scale: 1})
	host.FocusAutomationID("plugin-window/doc/k:draft/text-field/field")
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "typed"})
	if len(events) != 1 || events[0].CoalesceKey == "" {
		t.Fatal("value change not marked")
	}
	wire, _ := json.Marshal(events[0])
	if strings.Contains(string(wire), "Coalesce") {
		t.Fatalf("queue metadata leaked: %s", wire)
	}
	var data woxview.Event
	if err := json.Unmarshal(events[0].Data, &data); err != nil {
		t.Fatal(err)
	}
	if data.Key != "draft" || data.Handler != "edit" || string(data.Args) != `["typed"]` {
		t.Fatalf("event %+v", data)
	}
}

// TestWindowWoxViewEventKeepsDocumentAndNodeKey verifies the actual window adapter payload.
func TestWindowWoxViewEventKeepsDocumentAndNodeKey(t *testing.T) {
	content := plugin.WindowContent{Type: plugin.WindowContentWoxView, Data: `{"version":1,"root":{"type":"button","key":"row-17","props":{"label":"Open","onPress":{"$handler":"open"}}}}`}
	tree, err := prepareContent(content)
	if err != nil {
		t.Fatal(err)
	}
	hideChrome := true
	var events []plugin.WindowEvent
	s := &session{content: content, document: "doc-1", options: plugin.WindowOptions{HideChrome: &hideChrome}, view: woxview.NewView(), emit: func(e plugin.WindowEvent) { events = append(events, e) }}
	s.view.SetTree(tree)
	host := widget.NewHost(s.build)
	host.AttachServices(translationTestServices{})
	defer host.Dispose()
	host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 300, Height: 200}, Scale: 1})
	if !host.FocusAutomationID("plugin-window/doc-1/k:row-17/button") || !host.Key(ui.KeyEvent{Key: ui.KeyEnter, Down: true}) {
		t.Fatal("button not interactive")
	}
	if len(events) != 1 || events[0].DocumentID != "doc-1" || events[0].Type != plugin.WindowEventWoxView {
		t.Fatalf("events %+v", events)
	}
	var data woxview.Event
	if err := json.Unmarshal(events[0].Data, &data); err != nil {
		t.Fatal(err)
	}
	if data.Key != "row-17" || data.Handler != "open" || string(data.Args) != "[]" {
		t.Fatalf("payload %+v", data)
	}
}

type translationTestServices struct{}

func (translationTestServices) MeasureText(text string, style ui.TextStyle) (ui.TextMetrics, error) {
	return ui.TextMetrics{Size: ui.Size{Width: float32(len([]rune(text))) * max(style.Size/2, 1), Height: max(style.Size, 1)}}, nil
}
func (translationTestServices) Invalidate() error                         { return nil }
func (translationTestServices) InvalidateRect(ui.Rect) error              { return nil }
func (translationTestServices) SetTextInputState(ui.TextInputState) error { return nil }
func (translationTestServices) SetPointerCursor(ui.PointerCursor) error   { return nil }
func (translationTestServices) UpdateAccessibility(ui.AccessibilityTree, ui.AccessibilityActionHandler) error {
	return nil
}

// TestWindowPassesTranslationOnlyToDeclarativeContent guards the literal shorthand content contract.
func TestWindowPassesTranslationOnlyToDeclarativeContent(t *testing.T) {
	for _, kind := range []plugin.WindowContentType{plugin.WindowContentWoxView, plugin.WindowContentText} {
		content := plugin.WindowContent{Type: kind, Data: "i18n:label"}
		if kind == plugin.WindowContentWoxView {
			content.Data = `{"version":1,"root":{"type":"button","props":{"label":"i18n:label"}}}`
		}
		tree, err := prepareContent(content)
		if err != nil {
			t.Fatal(err)
		}
		hideChrome := true
		calls := 0
		s := &session{content: content, document: "doc", options: plugin.WindowOptions{HideChrome: &hideChrome}, view: woxview.NewView(), translate: func(text string) string {
			calls++
			if text != "i18n:label" {
				t.Fatalf("unexpected translation key %q", text)
			}
			return "Translated label"
		}}
		s.view.SetTree(tree)
		host := widget.NewHost(s.build)
		host.AttachServices(translationTestServices{})
		host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 300, Height: 200}, PixelSize: ui.PixelSize{Width: 300, Height: 200}, Scale: 1})
		want := "i18n:label"
		if kind == plugin.WindowContentWoxView {
			want = "Translated label"
		}
		found := false
		for _, n := range host.Snapshot().Tree.Nodes {
			if n.Label == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: missing displayed label %q", kind, want)
		}
		if kind == plugin.WindowContentText && calls != 0 {
			t.Fatal("literal text was translated")
		}
	}
}

// TestWindowValidationPreservesStructuredErrorAndText checks validation before native allocation.
func TestWindowValidationPreservesStructuredErrorAndText(t *testing.T) {
	_, err := Create("invalid", plugin.CreateWindowOption{Content: plugin.WindowContent{Type: plugin.WindowContentWoxView, Data: `{"version":1,"root":{"type":"text","props":{"typo":"x"}}}`}}, nil)
	var validation *woxview.ValidationError
	if !errors.As(err, &validation) || validation.Path != "root/text.props" {
		t.Fatalf("lost structured error: %v", err)
	}
	if err.Error() != `root/text props: json: unknown field "typo"` {
		t.Fatalf("changed diagnostic: %v", err)
	}
}
