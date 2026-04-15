//go:build windows

package platform

import (
	"context"
	"syscall"
	"testing"
	"time"
	"unsafe"
	"wox/common"
	launchertheme "wox/launcher/theme"

	"github.com/lxn/win"
)

func TestWindowsNativeShellHostTracksWindowLifecycle(t *testing.T) {
	host := NewWindowsNativeShellHost()
	textInput := host.TextInputHost()
	ctx := context.Background()

	if err := host.Start(ctx, StartOptions{
		Appearance: WindowAppearance{
			Transparent:    true,
			Acrylic:        true,
			RoundedCorners: true,
		},
	}); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer func() {
		_ = host.Stop(context.Background())
	}()

	if host.controller.windowHandle == 0 {
		t.Fatal("Start should create a native shell window")
	}

	if host.controller.editControl == 0 {
		t.Fatal("Start should create a native edit control")
	}

	if !host.SupportsEmbeddedTextInput(ctx) {
		t.Fatal("native shell host should support embedded text input")
	}

	request := ShowRequest{
		ShowContext: common.ShowContext{
			WindowWidth: 800,
			WindowPosition: &common.WindowPosition{
				X: 100,
				Y: 120,
			},
		},
		Theme: launchertheme.DefaultPaintTheme(),
	}
	if err := host.Show(ctx, request); err != nil {
		t.Fatalf("Show returned error: %v", err)
	}

	if !host.IsVisible(ctx) {
		t.Fatal("Show should mark the native shell as visible")
	}

	if host.NativeWindowHandle(ctx) == 0 {
		t.Fatal("Show should expose a stable native window handle")
	}

	var rect win.RECT
	if !win.GetWindowRect(host.controller.windowHandle, &rect) {
		t.Fatal("Show should allow reading the native shell rect")
	}

	if int(rect.Left) != 100 || int(rect.Top) != 120 {
		t.Fatalf("unexpected shell origin: left=%d top=%d", rect.Left, rect.Top)
	}

	if int(rect.Right-rect.Left) != 800 || int(rect.Bottom-rect.Top) != defaultShellHeight {
		t.Fatalf("unexpected shell size: width=%d height=%d", rect.Right-rect.Left, rect.Bottom-rect.Top)
	}

	state := TextInputState{
		QueryBox: QueryBoxState{
			Visible:      true,
			Text:         "hello",
			Placeholder:  DefaultQueryBoxPlaceholder,
			HasFocus:     true,
			CaretVisible: true,
			Frame: Rect{
				X:      124,
				Y:      140,
				Width:  752,
				Height: 48,
			},
		},
		SelectionStart: 5,
		SelectionEnd:   5,
	}
	if err := textInput.UpdateState(ctx, state); err != nil {
		t.Fatalf("UpdateState returned error: %v", err)
	}

	if !win.IsWindowVisible(host.controller.editControl) {
		t.Fatal("visible query box should show the embedded edit control")
	}

	if win.GetParent(host.controller.editControl) != host.controller.windowHandle {
		t.Fatal("embedded edit control should be parented to the native shell window")
	}

	if err := host.Hide(ctx); err != nil {
		t.Fatalf("Hide returned error: %v", err)
	}

	if host.IsVisible(ctx) {
		t.Fatal("Hide should clear the visible state")
	}
}

func TestWindowsNativeShellTextInputEmitsUserEditChanges(t *testing.T) {
	host := NewWindowsNativeShellHost()
	textInput := host.TextInputHost()
	ctx := context.Background()

	if err := host.Start(ctx, StartOptions{
		Appearance: WindowAppearance{
			Transparent:    true,
			Acrylic:        true,
			RoundedCorners: true,
		},
	}); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer func() {
		_ = host.Stop(context.Background())
	}()

	observed := make(chan TextInputState, 4)
	if err := textInput.SetChangeHandler(ctx, func(ctx context.Context, state TextInputState) {
		observed <- state
	}); err != nil {
		t.Fatalf("SetChangeHandler returned error: %v", err)
	}

	if err := host.Show(ctx, ShowRequest{
		ShowContext: common.ShowContext{WindowWidth: 800},
		Theme:       launchertheme.DefaultPaintTheme(),
	}); err != nil {
		t.Fatalf("Show returned error: %v", err)
	}

	if err := textInput.UpdateState(ctx, TextInputState{
		QueryBox: QueryBoxState{
			Visible:     true,
			Text:        "",
			Placeholder: DefaultQueryBoxPlaceholder,
			Frame: Rect{
				X:      24,
				Y:      20,
				Width:  752,
				Height: 48,
			},
		},
	}); err != nil {
		t.Fatalf("UpdateState returned error: %v", err)
	}

	win.SendMessage(host.controller.editControl, win.WM_SETTEXT, 0, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("typed from edit"))))

	select {
	case state := <-observed:
		if state.QueryBox.Text != "typed from edit" {
			t.Fatalf("unexpected observed text: %q", state.QueryBox.Text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for text input change event")
	}
}
