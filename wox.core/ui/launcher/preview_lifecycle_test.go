package launcher

import (
	"errors"
	"testing"

	woxui "wox/ui/runtime"
)

func TestSelectedPreviewForLifecycleRejectsStaleQueryResults(t *testing.T) {
	app := &App{
		visible:        true,
		query:          plainQuery{QueryID: "current-query"},
		resultsQueryID: "previous-query",
		results:        []queryResult{{Preview: queryPreview{PreviewType: "text", PreviewData: "stale"}}},
		selected:       0,
	}

	if _, _, visible := app.selectedPreviewForLifecycle(); visible {
		t.Fatal("stale query result kept its preview lifecycle active")
	}
}

func TestSelectedPreviewForLifecycleRejectsPreviewlessResult(t *testing.T) {
	app := &App{
		visible:        true,
		query:          plainQuery{QueryID: "current-query"},
		resultsQueryID: "current-query",
		results:        []queryResult{{Preview: queryPreview{PreviewType: "text"}}},
		selected:       0,
	}

	if _, _, visible := app.selectedPreviewForLifecycle(); visible {
		t.Fatal("result without preview data kept its preview lifecycle active")
	}
}

func TestNativeFilePreviewLifecycleAdvancesGeneration(t *testing.T) {
	app := &App{}
	changed := app.activateNativeFilePreview("first.docx")
	if !changed || app.nativeFilePreviewGeneration != 1 {
		t.Fatalf("first native preview activation = changed %v generation %d", changed, app.nativeFilePreviewGeneration)
	}

	app.deactivateNativeFilePreview()
	if app.nativeFilePreviewGeneration != 2 {
		t.Fatalf("native preview deactivation generation = %d, want 2", app.nativeFilePreviewGeneration)
	}
	changed = app.activateNativeFilePreview("second.docx")
	if !changed || app.nativeFilePreviewGeneration != 3 {
		t.Fatalf("second native preview activation = changed %v generation %d, want generation 3", changed, app.nativeFilePreviewGeneration)
	}
}

func TestNativeFilePreviewIgnoresStaleErrors(t *testing.T) {
	app := &App{nativeFilePreviewPath: "current.docx", nativeFilePreviewGeneration: 2}
	app.setNativeFilePreviewError(1, errors.New("stale preview failure"))
	if app.nativeFilePreviewError != "" || app.nativeFilePreviewGeneration != 2 {
		t.Fatalf("stale native preview error changed state: error %q generation %d", app.nativeFilePreviewError, app.nativeFilePreviewGeneration)
	}
}

func TestNativeFilePreviewDelayedActivationRejectsObsoleteSelection(t *testing.T) {
	app := &App{}
	if !app.scheduleNativeFilePreview("first.docx") {
		t.Fatal("initial native preview should schedule")
	}
	generation := app.nativeFilePreviewGeneration
	if app.nativeFilePreviewPendingPath != "first.docx" || app.nativeFilePreviewTimer == nil {
		t.Fatal("scheduled native preview did not retain its cancellation state")
	}

	app.deactivateNativeFilePreview()
	app.activateScheduledNativeFilePreview("first.docx", generation)
	if app.nativeFilePreviewPath != "" {
		t.Fatalf("obsolete delayed preview became active for %q", app.nativeFilePreviewPath)
	}
}

func TestNativeFilePreviewCoalescesPendingBounds(t *testing.T) {
	app := &App{nativeFilePreviewPath: "document.docx", nativeFilePreviewGeneration: 3}
	first := woxui.Rect{X: 1, Y: 2, Width: 300, Height: 400}
	latest := woxui.Rect{X: 2, Y: 3, Width: 320, Height: 420}
	app.requestNativeFilePreviewBounds("document.docx", 3, first)
	timer := app.nativeFilePreviewBoundsTimer
	if timer == nil {
		t.Fatal("first native preview bounds should schedule one deferred update")
	}
	app.requestNativeFilePreviewBounds("document.docx", 3, latest)
	if app.nativeFilePreviewBoundsTimer != timer {
		t.Fatal("new bounds should update the pending request instead of scheduling another native operation")
	}
	if app.nativeFilePreviewBounds != latest {
		t.Fatalf("pending native preview bounds = %+v, want %+v", app.nativeFilePreviewBounds, latest)
	}
	app.stopNativeFilePreviewTimers()
}
