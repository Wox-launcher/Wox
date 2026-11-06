package launcher

import (
	"context"
	"strings"
	"testing"
	"time"

	"wox/plugin"
	ui "wox/ui/runtime"
	"wox/ui/widget"
	"wox/ui/woxview"
)

type stockRemotePreviewServices struct{ preview plugin.WoxPreview }

func (s stockRemotePreviewServices) ResultPreview(context.Context, string, string, string, string) (plugin.WoxPreview, error) {
	return s.preview, nil
}

// Large interactive previews travel through the deferred preview cache.
func TestRemoteWoxViewPreviewRetainsDocumentIdentity(t *testing.T) {
	data := `{"version":1,"root":{"type":"column","children":[{"type":"text","props":{"text":"` + strings.Repeat("x", 1200) + `"}},{"type":"button","props":{"label":"Back","onPress":{"$handler":"back"}}},{"type":"line-chart","props":{"values":[1,2,3]}}]}}`
	loaded := plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeWoxView, PreviewData: data, WoxViewID: "document-1"}
	path := "/preview?sessionId=session&queryId=query&id=result"
	controller := remotePreviewController{
		isDestroyed:     func() bool { return false },
		services:        stockRemotePreviewServices{preview: loaded},
		lifecycleCtx:    context.Background(),
		runOnUI:         func(_ string, apply func()) error { apply(); return nil },
		onReady:         func() {},
		remotePreviews:  map[string]queryPreview{},
		previewRequests: map[string]bool{},
	}
	controller.loadRemotePreview(path, queryPreview{PreviewType: "remote", PreviewData: path})
	result := controller.resolvePreview(queryPreview{PreviewType: "remote", PreviewData: path})
	if result.WoxViewID != loaded.WoxViewID || result.PreviewType != loaded.PreviewType || result.PreviewData != data {
		t.Fatalf("deferred preview lost identity or content: %+v", result)
	}
	tree, err := woxview.Parse([]byte(result.PreviewData))
	if err != nil || !tree.HasHandlers() {
		t.Fatalf("invalid interactive WoxView: %v", err)
	}
}

// TestRemoteWoxViewClick verifies the resolved document still routes interactive events.
func TestRemoteWoxViewClick(t *testing.T) {
	data := `{"version":1,"root":{"type":"column","children":[{"type":"button","key":"add","props":{"label":"Add","onPress":{"$handler":"h"},"style":{"width":100,"height":40}}},{"type":"text","props":{"text":"` + strings.Repeat("x", 1200) + `"}}]}}`
	path := "/preview?sessionId=session&queryId=query&id=row"
	controller := remotePreviewController{
		isDestroyed:  func() bool { return false },
		services:     stockRemotePreviewServices{preview: plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeWoxView, PreviewData: data, WoxViewID: "doc"}},
		lifecycleCtx: context.Background(),
		runOnUI:      func(_ string, apply func()) error { apply(); return nil },
		onReady:      func() {}, remotePreviews: map[string]queryPreview{}, previewRequests: map[string]bool{},
	}
	controller.loadRemotePreview(path, queryPreview{PreviewType: "remote", PreviewData: path})
	preview := controller.resolvePreview(queryPreview{PreviewType: "remote", PreviewData: path})
	services := &previewEventTestServices{events: make(chan plugin.WoxViewPreviewEvent, 1)}
	app := New(false, services)
	defer app.cancel()
	app.uiCall = nil
	app.sessionID, app.previews.sessionID = "session", "session"
	app.visible = true
	result := queryResult{ID: "row", QueryID: "query"}
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
		if event.Handler != "h" {
			t.Fatalf("unexpected event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("remote preview click was not delivered")
	}
	app.previews.deactivatePreviewTypes("")
}
