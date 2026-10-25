package launcher

import (
	"testing"

	"wox/plugin"
	woxui "wox/ui/runtime"
)

func TestDragOutStatusMapsNativeOutcomes(t *testing.T) {
	cases := []struct {
		status woxui.FileDragStatus
		want   plugin.DragOutStatus
		ok     bool
	}{
		{woxui.FileDragStatusSuccess, plugin.DragOutStatusSuccess, true},
		{woxui.FileDragStatusCancel, plugin.DragOutStatusCancel, true},
		{woxui.FileDragStatusCancelInSource, plugin.DragOutStatusCancelInSource, true},
		{woxui.FileDragStatusPending, "", false},
	}
	for _, testCase := range cases {
		got, ok := dragOutStatus(testCase.status)
		if ok != testCase.ok || got != testCase.want {
			t.Fatalf("status %v = (%q, %v), want (%q, %v)", testCase.status, got, ok, testCase.want, testCase.ok)
		}
	}
}

func TestHideLauncherAfterResultDragDefaultsToHide(t *testing.T) {
	if !hideLauncherAfterResultDrag(woxui.FileDragStatusSuccess, false) {
		t.Fatal("successful drag should hide unless the result opts out")
	}
	if !hideLauncherAfterResultDrag(woxui.FileDragStatusCancel, false) {
		t.Fatal("cancelled drag outside Wox should hide unless the result opts out")
	}
	if hideLauncherAfterResultDrag(woxui.FileDragStatusSuccess, true) {
		t.Fatal("PreventHideAfterDrag should keep Wox visible after success")
	}
	if hideLauncherAfterResultDrag(woxui.FileDragStatusCancel, true) {
		t.Fatal("PreventHideAfterDrag should keep Wox visible after cancel")
	}
	if hideLauncherAfterResultDrag(woxui.FileDragStatusCancelInSource, false) {
		t.Fatal("dropping back onto Wox should keep the launcher visible")
	}
}

func TestOnFocusSkipsHideOnBlurDuringResultFileDrag(t *testing.T) {
	app := bindTestApp(&App{
		visible:    true,
		resultDrag: resultDragController{resultFileDragActive: true},
	})
	app.show.HideOnBlur = true
	app.onFocus(woxui.FocusEvent{Active: false})
	if !app.visible || !app.resultDrag.resultFileDragActive {
		t.Fatal("hide-on-blur must not run while a result file drag is active")
	}

	app.onFocus(woxui.FocusEvent{Active: true})
	if !app.resultDrag.resultFileDragActive {
		t.Fatal("focusing during an OS file drag must not end the hide-on-blur suppress")
	}
}

func TestResultDragHoldEndsOnInteractionWithoutNativeBlur(t *testing.T) {
	app := bindTestApp(&App{
		visible:    true,
		resultDrag: resultDragController{resultFileDragActive: true},
	})
	app.show.HideOnBlur = true
	// Input during the native drag must not lift the visibility protection.
	app.resultDrag.releaseResultFileDragHold()
	if !app.resultDrag.resultFileDragActive {
		t.Fatal("input during drag released visibility protection")
	}
	app.resultDrag.finishResultFileDrag(woxui.FileDragStatusSuccess, pendingResultFileDrag{preventHide: true})
	// Native drag loops can swallow every blur event. Focus bouncing alone must
	// not dismiss Wox, and a later click/key must still restore normal behavior.
	app.onFocus(woxui.FocusEvent{Active: true})
	if !app.resultDrag.resultFileDragHeld || !app.visible {
		t.Fatal("post-drag activation released visibility protection")
	}
	app.resultDrag.releaseResultFileDragHold()
	if app.resultDrag.resultFileDragActive || app.resultDrag.resultFileDragHeld {
		t.Fatal("user interaction did not restore normal blur behavior")
	}
}

func TestResultDragDeliversCapturedCallback(t *testing.T) {
	var got plugin.DragOutEvent
	app := bindTestApp(&App{})
	app.resultDrag.finishResultFileDrag(woxui.FileDragStatusSuccess, pendingResultFileDrag{
		resultID: "result", files: []string{"file"}, preventHide: true,
		notify: func(event plugin.DragOutEvent) { got = event },
	})
	if got.ResultId != "result" || got.Status != plugin.DragOutStatusSuccess || len(got.Files) != 1 || got.Files[0] != "file" {
		t.Fatalf("drag callback = %#v", got)
	}
}
