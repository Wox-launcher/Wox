package launcher

import (
	"fmt"
	"testing"

	"wox/plugin"
	woxui "wox/ui/runtime"
)

// TestQueryPreviewWindowSize exercises automatic sizing and restoration through native window bounds.
func TestQueryPreviewWindowSize(t *testing.T) {
	err := woxui.Run(func() error {
		window, err := woxui.Open(woxui.WindowOptions{Title: "Wox preview bounds test", Size: woxui.Size{Width: 800, Height: 300}})
		if err != nil {
			return err
		}
		defer window.Close()
		app := New(false, nil)
		defer app.cancel()
		app.window = window
		app.uiCall = nil
		app.show.WindowWidth = 800
		app.querySession.query = newInputQuery("g wox")
		plain := []queryResult{{ID: "plain"}}
		app.applyResults(app.querySession.query.QueryID, plain, &queryLayout{}, nil, nil, 0, true)
		normal, err := window.Bounds()
		if err != nil {
			return err
		}
		ratio := 0.0
		layout := fromCoreQueryLayout(plugin.QueryLayout{ResultPreviewWidthRatio: &ratio, WindowWidth: 640, WindowHeight: 300})
		results := []queryResult{{ID: "page", Preview: queryPreview{PreviewType: "webview", PreviewData: `{"url":"https://example.com"}`}}}
		for _, complete := range []bool{false, true} {
			app.applyResults(app.querySession.query.QueryID, results, &layout, nil, nil, 0, complete)
			bounds, err := window.Bounds()
			if err != nil {
				return err
			}
			if absFloat32(bounds.Width-640) >= 1 || absFloat32(bounds.Height-300) >= 1 || app.webView.webViewFullscreen {
				return fmt.Errorf("automatic preview size: complete=%v bounds=%+v manual=%v", complete, bounds, app.webView.webViewFullscreen)
			}
		}
		app.applyResults(app.querySession.query.QueryID, plain, &queryLayout{}, nil, nil, 0, true)
		restored, err := window.Bounds()
		if err != nil {
			return err
		}
		if !launcherBoundsEffectivelyEqual(normal, restored) {
			return fmt.Errorf("normal size not restored: before=%+v after=%+v", normal, restored)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestLauncherBoundsKeepsNewerPreview reproduces a result arriving before the hotkey caller resumes.
func TestLauncherBoundsKeepsNewerPreview(t *testing.T) {
	err := woxui.Run(func() error {
		window, err := woxui.Open(woxui.WindowOptions{Title: "Wox bounds test", Size: woxui.Size{Width: 800, Height: 571}})
		if err != nil {
			return err
		}
		defer window.Close()
		app := bindTestApp(&App{
			window:       window,
			querySession: queryController{editor: woxui.NewTextEditor("chat ")},
			appearance:   appearanceController{palette: defaultPalette()},
		})
		injected := false
		var previewBounds woxui.Rect
		app.uiCall = func(fn func()) error {
			fn()
			if injected {
				return nil
			}
			injected = true
			// The old implementation yielded after its empty-results snapshot, so
			// this newer preview resize was subsequently overwritten by that snapshot.
			app.resultList.results = []queryResult{{ID: "chat", Preview: queryPreview{PreviewType: "chat", PreviewData: "{}"}}}
			app.resultList.layout = queryLayout{ChatMode: true}
			if err := app.applyWindowBounds(); err != nil {
				return err
			}
			previewBounds, err = window.Bounds()
			return err
		}
		if err := app.applyWindowBoundsAtShowPosition(); err != nil {
			return err
		}
		finalBounds, err := window.Bounds()
		if err != nil {
			return err
		}
		if previewBounds.Height < 200 || !launcherBoundsEffectivelyEqual(finalBounds, previewBounds) {
			return fmt.Errorf("newer preview bounds lost: preview=%+v final=%+v", previewBounds, finalBounds)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestGridBoundsIgnoreHiddenPreview checks the same resize path used after query results arrive.
func TestGridBoundsIgnoreHiddenPreview(t *testing.T) {
	err := woxui.Run(func() error {
		window, err := woxui.Open(woxui.WindowOptions{Title: "Wox grid bounds test", Size: woxui.Size{Width: 800, Height: 599}})
		if err != nil {
			return err
		}
		defer window.Close()
		app := bindTestApp(&App{
			window:       window,
			querySession: queryController{editor: woxui.NewTextEditor("drop ")},
			appearance:   appearanceController{palette: defaultPalette()},
			resultList: resultController{
				layout:  queryLayout{GridLayout: &gridLayout{Columns: 5, ShowTitle: true}},
				results: []queryResult{{ID: "file"}},
			},
		})
		if err := app.applyWindowBounds(); err != nil {
			return err
		}
		withoutPreview, err := window.Bounds()
		if err != nil {
			return err
		}
		app.resultList.results[0].Preview = queryPreview{PreviewType: "file", PreviewData: "file.exe"}
		if err := app.applyWindowBounds(); err != nil {
			return err
		}
		withHiddenPreview, err := window.Bounds()
		if err != nil {
			return err
		}
		if !launcherBoundsEffectivelyEqual(withoutPreview, withHiddenPreview) {
			return fmt.Errorf("hidden grid preview expanded window: without=%+v with=%+v", withoutPreview, withHiddenPreview)
		}
		app.resultList.layout = queryLayout{}
		if err := app.applyWindowBounds(); err != nil {
			return err
		}
		withVisiblePreview, err := window.Bounds()
		if err != nil {
			return err
		}
		if withVisiblePreview.Height <= withHiddenPreview.Height {
			return fmt.Errorf("visible list preview lost its height: grid=%+v list=%+v", withHiddenPreview, withVisiblePreview)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
