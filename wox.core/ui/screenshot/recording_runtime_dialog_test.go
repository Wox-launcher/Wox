package screenshot

import (
	"context"
	"errors"
	"testing"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/ffmpeg"
)

// TestRecordingFormatToggleAfterBlur reproduces the native blur-before-pointer ordering at mixed scales and origins.
func TestRecordingFormatToggleAfterBlur(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		state := &recordingToolbarState{expandedBounds: Rect{X: -400 * scale, Y: 180 * scale}, formatRect: Rect{X: 200 * scale, Y: 10 * scale, Width: 40 * scale, Height: 40 * scale}}
		pointer := Point{X: state.expandedBounds.X + state.formatRect.X + 20*scale, Y: state.expandedBounds.Y + state.formatRect.Y + 20*scale}
		state.platform.cursorPosition = func() *Point { return &pointer }
		menu := &recordingFormatMenu{owner: state}
		state.formatMenu = menu
		menu.focusChanged(woxui.FocusEvent{Active: false})
		if state.formatMenu == nil {
			t.Fatal("blur consumed the pending trigger click")
		}
		state.toolbarPointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: state.formatRect.X + 20*scale, Y: state.formatRect.Y + 20*scale}})
		if state.formatMenu != nil {
			t.Fatal("second trigger click did not dismiss menu")
		}
		state.formatMenu = menu
		pointer.X += 400 * scale
		menu.focusChanged(woxui.FocusEvent{Active: false})
		if state.formatMenu != nil {
			t.Fatal("outside blur did not dismiss menu")
		}
	}
}

// TestRecordingPlaybackToolbarGeometry keeps format, save, and cancel in equal non-overlapping slots.
func TestRecordingPlaybackToolbarGeometry(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2.5} {
		state := &recordingToolbarState{session: &recordingSession{state: recordingStateSave, config: recordingSessionConfig{Now: time.Now}}}
		frame := FrameInfo{Size: Size{Width: (recordingToolbarWidth - 48) * scale, Height: recordingToolbarHeight * scale}}
		state.drawToolbar(&DisplayList{}, frame)
		for _, rect := range []Rect{state.formatRect, state.finishRect, state.cancelRect} {
			if rect.Width != 40*scale || rect.X+rect.Width > frame.Size.Width {
				t.Fatalf("invalid toolbar slot: %+v", rect)
			}
		}
		if state.formatRect.X+state.formatRect.Width >= state.finishRect.X || state.finishRect.X+state.finishRect.Width >= state.cancelRect.X {
			t.Fatal("toolbar buttons overlap")
		}
	}
}

// TestRuntimeConsentAndCancellation ensures opening or dismissing never downloads, and duplicate confirmation is ignored.
func TestRuntimeConsentAndCancellation(t *testing.T) {
	callbacks := make(chan func(), 8)
	started := make(chan struct{}, 2)
	state := &recordingToolbarState{}
	dialog := &recordingRuntimeDialog{owner: state, dispatch: func(callback func()) error { callbacks <- callback; return nil }}
	dialog.install = func(ctx context.Context, progress func(ffmpeg.Progress)) (string, error) {
		started <- struct{}{}
		<-ctx.Done()
		return "", ctx.Err()
	}
	state.runtimeDialog = dialog
	dialog.host = woxwidget.NewHost(dialog.build)
	dialog.host.AttachServices(&screenshotTestSurface{})
	dialog.host.Frame(&DisplayList{}, FrameInfo{Size: Size{Width: 460, Height: 264}, Scale: 2})
	if len(started) != 0 {
		t.Fatal("rendering consent started a download")
	}
	dialog.confirm()
	dialog.confirm()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("confirmation did not start installation")
	}
	dialog.close()
	select {
	case callback := <-callbacks:
		callback()
	case <-time.After(time.Second):
		t.Fatal("cancellation did not stop worker")
	}
	if state.runtimeDialog != nil || !dialog.closed || dialog.failed || len(started) != 0 {
		t.Fatal("cancelled or duplicate installation changed the UI")
	}
	dialog.confirm()
	if len(started) != 0 {
		t.Fatal("closed dialog restarted installation")
	}
}

// TestRuntimeRetryAndLateSuccess ensures failures remain retryable and completed workers cannot resume a cancelled capture.
func TestRuntimeRetryAndLateSuccess(t *testing.T) {
	callbacks := make(chan func(), 8)
	state := &recordingToolbarState{}
	dialog := &recordingRuntimeDialog{owner: state, dispatch: func(callback func()) error { callbacks <- callback; return nil }}
	attempt := 0
	dialog.install = func(context.Context, func(ffmpeg.Progress)) (string, error) {
		attempt++
		if attempt == 1 {
			return "", errors.New("offline")
		}
		return "/installed/ffmpeg", nil
	}
	state.runtimeDialog = dialog
	dialog.confirm()
	select {
	case callback := <-callbacks:
		callback()
	case <-time.After(time.Second):
		t.Fatal("missing failure callback")
	}
	if !dialog.failed || dialog.busy || dialog.closed {
		t.Fatal("failure is not retryable")
	}
	state.cancelled = true
	dialog.confirm()
	select {
	case callback := <-callbacks:
		callback()
	case <-time.After(time.Second):
		t.Fatal("missing success callback")
	}
	if !dialog.closed || state.runtimeDialog != nil || state.session != nil {
		t.Fatal("completion resumed a cancelled capture")
	}
}

// TestRuntimeEscapeFromToolbar preserves the capture when the toolbar takes focus behind the consent dialog.
func TestRuntimeEscapeFromToolbar(t *testing.T) {
	state := &recordingToolbarState{}
	dialog := &recordingRuntimeDialog{owner: state}
	state.runtimeDialog = dialog
	dialog.host = woxwidget.NewHost(dialog.build)
	dialog.host.AttachServices(&screenshotTestSurface{})
	dialog.host.Frame(&DisplayList{}, FrameInfo{Size: Size{Width: 460, Height: 264}})
	host := &screenshotEditorWindowHost{state: &screenshotEditorOverlayState{recordingUI: state}}
	if !host.key(KeyEvent{Down: true, Key: KeyEscape}) || !dialog.closed || state.cancelled {
		t.Fatal("Escape did not dismiss only the runtime dialog")
	}
}
