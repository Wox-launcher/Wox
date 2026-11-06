package launcher

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"wox/plugin"
	"wox/ui/contract"
	ui "wox/ui/runtime"
	"wox/ui/widget"
	"wox/ui/woxview"
)

type previewEventTestServices struct {
	contract.Services
	events    chan plugin.WoxViewPreviewEvent
	lifecycle chan plugin.WoxViewPreviewEventType
}

// slowPreviewServices holds the first discrete callback while the UI keeps producing edits.
type slowPreviewServices struct {
	previewEventTestServices
	started, release chan struct{}
}

func (s *slowPreviewServices) DispatchWoxViewPreviewEvent(ctx context.Context, session, query, result, document string, event plugin.WoxViewPreviewEvent) error {
	if event.Handler == "block" {
		close(s.started)
		select {
		case <-s.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return s.previewEventTestServices.DispatchWoxViewPreviewEvent(ctx, session, query, result, document, event)
}

// TestSlowWoxViewPreviewReceivesLatestChanges exercises the real worker and discrete-event barriers.
func TestSlowWoxViewPreviewReceivesLatestChanges(t *testing.T) {
	services := &slowPreviewServices{previewEventTestServices: previewEventTestServices{events: make(chan plugin.WoxViewPreviewEvent, 8)}, started: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	defer release.Do(func() { close(services.release) })
	app := New(false, services)
	defer app.cancel()
	defer app.previews.clearWoxViewPreview()
	app.uiCall = nil
	app.sessionID, app.previews.sessionID, app.visible = "session", "session", true
	result := queryResult{ID: "row", QueryID: "query"}
	preview := queryPreview{WoxViewID: "doc", PreviewData: `{"version":1,"root":{"type":"column","children":[{"type":"button","props":{"onPress":{"$handler":"block"}}},{"type":"slider","props":{"onChange":{"$handler":"change"}}},{"type":"button","props":{"onPress":{"$handler":"submit"}}}]}}`}
	app.buildWoxViewPreview(result, "key", preview, uiPalette{}, 200, 100)
	queue := app.previews.woxViewEvents
	queue.Push(woxview.Event{Handler: "block", Args: []byte(`[]`)})
	select {
	case <-services.started:
	case <-time.After(3 * time.Second):
		t.Fatal("callback did not start")
	}
	for batch := range 2 {
		for i := range 1000 {
			if !queue.Push(woxview.Event{Handler: "change", Key: "slider", Args: []byte(fmt.Sprintf("[%d]", batch*1000+i)), Coalescible: true}) {
				t.Fatal("replaceable edits overflowed")
			}
		}
		if batch == 0 {
			queue.Push(woxview.Event{Handler: "submit", Args: []byte(`[]`)})
		}
	}
	release.Do(func() { close(services.release) })
	for _, want := range []struct{ handler, args string }{{"block", "[]"}, {"change", "[999]"}, {"submit", "[]"}, {"change", "[1999]"}} {
		select {
		case got := <-services.events:
			if got.Handler != want.handler || string(got.Args) != want.args {
				t.Fatalf("event=%+v, want=%+v", got, want)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("missing %+v", want)
		}
	}
	app.previews.clearWoxViewPreview()
	queue.Push(woxview.Event{Handler: "change", Args: []byte(`[2000]`), Coalescible: true})
	if _, ok := queue.Pop(); ok {
		t.Fatal("disposed preview retained events")
	}
}

// TestWoxViewPreviewReusesValidatedTree checks the complete update-to-build handoff.
func TestWoxViewPreviewReusesValidatedTree(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	defer app.previews.clearWoxViewPreview()
	app.uiCall = nil
	app.resultList.results = []queryResult{{ID: "row", Preview: queryPreview{PreviewData: "old"}}}
	preview := plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeWoxView, PreviewData: `{"version":1,"root":{"type":"text","props":{"text":"updated"}}}`}
	accepted, err := app.UpdateResult(context.Background(), plugin.UpdatableResult{Id: "row", Preview: &preview})
	if err != nil || !accepted {
		t.Fatalf("update=%v err=%v", accepted, err)
	}
	prepared := app.resultList.results[0].Preview
	if prepared.woxViewTree == nil {
		t.Fatal("validated tree discarded before UI handoff")
	}
	app.buildWoxViewPreview(app.resultList.results[0], "key", prepared, uiPalette{}, 200, 100)
	if app.previews.woxView.Tree() != prepared.woxViewTree {
		t.Fatal("UI reparsed the accepted document")
	}
	preview.PreviewData = `{"version":1,"root":{"type":"missing"}}`
	accepted, err = app.UpdateResult(context.Background(), plugin.UpdatableResult{Id: "row", Preview: &preview})
	if err != nil || accepted || app.resultList.results[0].Preview.woxViewTree != prepared.woxViewTree {
		t.Fatal("invalid update replaced the accepted document")
	}
}

func (s *previewEventTestServices) DispatchWoxViewPreviewEvent(_ context.Context, session, query, result, document string, event plugin.WoxViewPreviewEvent) error {
	if session == "session" && query == "query" && result == "row" && document == "doc" {
		if event.Type != "" {
			if s.lifecycle != nil {
				s.lifecycle <- event.Type
			}
		} else if s.events != nil {
			s.events <- event
		}
	}
	return nil
}

// TestWoxViewPreviewClick sends a native widget event through the scoped preview worker.
func TestWoxViewPreviewClick(t *testing.T) {
	services := &previewEventTestServices{events: make(chan plugin.WoxViewPreviewEvent, 1)}
	app := New(false, services)
	defer app.cancel()
	app.uiCall = nil
	app.sessionID = "session"
	app.previews.sessionID = "session"
	app.visible = true
	result := queryResult{ID: "row", QueryID: "query"}
	preview := queryPreview{PreviewType: "woxview", WoxViewID: "doc", PreviewData: `{"version":1,"root":{"type":"column","children":[{"type":"button","key":"add","props":{"label":"Add","onPress":{"$handler":"h"},"style":{"width":100,"height":40}}}]}}`}
	host := widget.NewHost(func(ui.FrameInfo) widget.Widget {
		return app.buildWoxViewPreview(result, "key", preview, uiPalette{}, 200, 100)
	})
	host.AttachServices(formTableHostServices{})
	defer host.Dispose()
	host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 200, Height: 100}, PixelSize: ui.PixelSize{Width: 200, Height: 100}, Scale: 1})
	host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 50, Y: 20}})
	host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 50, Y: 20}})
	select {
	case event := <-services.events:
		if event.Handler != "h" || string(event.Args) != "[]" || event.Key != "add" {
			t.Fatalf("unexpected event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("preview click was not delivered")
	}
	app.previews.deactivatePreviewTypes("")
}

// TestWoxViewPreviewVisibility sends ordered transitions once per mount, including final disposal.
func TestWoxViewPreviewVisibility(t *testing.T) {
	services := &previewEventTestServices{lifecycle: make(chan plugin.WoxViewPreviewEventType, 8)}
	app := New(false, services)
	defer app.cancel()
	app.uiCall = nil
	app.sessionID, app.previews.sessionID = "session", "session"
	app.visible = true
	result := queryResult{ID: "row", QueryID: "query"}
	preview := queryPreview{WoxViewID: "doc", PreviewData: `{"version":1,"root":{"type":"text","props":{"text":"hello"}}}`}
	app.buildWoxViewPreview(result, "key", preview, uiPalette{}, 200, 100)
	app.buildWoxViewPreview(result, "key", preview, uiPalette{}, 200, 100)
	app.previews.clearWoxViewPreview()
	app.previews.clearWoxViewPreview()
	app.buildWoxViewPreview(result, "key", preview, uiPalette{}, 200, 100)
	app.disposeControllers()
	for _, want := range []plugin.WoxViewPreviewEventType{plugin.WoxViewPreviewEventShow, plugin.WoxViewPreviewEventHide, plugin.WoxViewPreviewEventShow, plugin.WoxViewPreviewEventHide} {
		select {
		case got := <-services.lifecycle:
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("missing %q", want)
		}
	}
	select {
	case extra := <-services.lifecycle:
		t.Fatalf("duplicate lifecycle event %q", extra)
	default:
	}
}

// TestHiddenWoxViewDoesNotStartWork avoids starting producer tasks during hidden repaints.
func TestHiddenWoxViewDoesNotStartWork(t *testing.T) {
	services := &previewEventTestServices{lifecycle: make(chan plugin.WoxViewPreviewEventType, 2)}
	app := New(false, services)
	defer app.cancel()
	app.sessionID, app.previews.sessionID = "session", "session"
	result := queryResult{ID: "row", QueryID: "query"}
	preview := queryPreview{WoxViewID: "doc", PreviewData: `{"version":1,"root":{"type":"text"}}`}
	app.buildWoxViewPreview(result, "key", preview, uiPalette{}, 200, 100)
	if app.previews.woxViewShown {
		t.Fatal("hidden preview was marked visible")
	}
	app.visible = true
	app.buildWoxViewPreview(result, "key", preview, uiPalette{}, 200, 100)
	app.previews.clearWoxViewPreview()
	for _, want := range []plugin.WoxViewPreviewEventType{plugin.WoxViewPreviewEventShow, plugin.WoxViewPreviewEventHide} {
		select {
		case got := <-services.lifecycle:
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("missing %q", want)
		}
	}
}

// TestWoxViewPreviewRejectsInvalidUpdates keeps the previous handler registry on malformed trees.
func TestWoxViewPreviewRejectsInvalidUpdates(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	for _, preview := range []plugin.WoxPreview{
		{PreviewType: plugin.WoxPreviewTypeWoxView, WoxViewID: "doc", PreviewData: `{"version":1,"root":{"type":"missing"}}`},
		{PreviewType: plugin.WoxPreviewTypeWoxView, PreviewData: `{"version":1,"root":{"type":"button","props":{"onPress":{"$handler":"h"}}}}`},
	} {
		accepted, err := app.UpdateResult(context.Background(), plugin.UpdatableResult{Id: "row", Preview: &preview})
		if err != nil || accepted {
			t.Fatalf("accepted malformed preview: %v, %v", accepted, err)
		}
	}
}

// TestWoxViewPreviewLifecycle checks selection isolation, updates, and invalid event trees.
func TestWoxViewPreviewLifecycle(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	p := &app.previews
	p.initWoxViewForTest(t, app, "query/result", `{"version":1,"root":{"type":"line-chart","props":{"values":[1,2]}}}`)
	first := p.woxView
	p.initWoxViewForTest(t, app, "query/result", `{"version":1,"root":{"type":"line-chart","props":{"values":[2,3]}}}`)
	if p.woxView != first {
		t.Fatal("tree update replaced the view")
	}
	app.buildWoxViewPreview(queryResult{}, "query/result", queryPreview{PreviewData: `{"version":1,"root":{"type":"button","props":{"onPress":{"$handler":"h"}}}}`}, uiPalette{}, 300, 200)
	if p.woxView != first || p.woxViewData == `{"version":1,"root":{"type":"button","props":{"onPress":{"$handler":"h"}}}}` {
		t.Fatal("invalid interactive tree replaced the current view")
	}
	p.initWoxViewForTest(t, app, "query/other", `{"version":1,"root":{"type":"text","props":{"text":"other"}}}`)
	if p.woxView == first {
		t.Fatal("different result reused the previous view")
	}
	p.deactivatePreviewTypes("")
	if p.woxView != nil || p.woxViewKey != "" || p.woxViewData != "" {
		t.Fatal("deactivation retained the document")
	}
}

func (p *previewController) initWoxViewForTest(t *testing.T, app *App, key, data string) {
	t.Helper()
	app.buildWoxViewPreview(queryResult{}, key, queryPreview{PreviewData: data}, uiPalette{}, 300, 200)
	if p.woxView == nil || p.woxViewKey != key || p.woxViewData != data {
		t.Fatal("failed to install WoxView preview")
	}
}
