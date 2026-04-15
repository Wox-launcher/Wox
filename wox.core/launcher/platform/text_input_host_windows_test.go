//go:build windows

package platform

import (
	"context"
	"testing"

	"github.com/lxn/win"
)

func TestWindowsTextInputHostTracksLifecycleAndState(t *testing.T) {
	t.Parallel()

	host := NewWindowsTextInputHost()
	state := TextInputState{
		QueryBox: QueryBoxState{
			Visible:      true,
			Text:         "hello",
			Placeholder:  DefaultQueryBoxPlaceholder,
			HasFocus:     true,
			CaretVisible: true,
			Frame: Rect{
				X:      180,
				Y:      210,
				Width:  420,
				Height: 48,
			},
		},
		SelectionStart: 5,
		SelectionEnd:   5,
	}

	if err := host.Start(context.Background()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	if !host.started {
		t.Fatal("Start should mark the Windows text input host as started")
	}

	if host.hostWindow == 0 {
		t.Fatal("Start should create a Win32 host window")
	}

	if host.editControl == 0 {
		t.Fatal("Start should create a Win32 edit control")
	}

	if err := host.UpdateState(context.Background(), state); err != nil {
		t.Fatalf("UpdateState returned error: %v", err)
	}

	if host.state.QueryBox.Text != state.QueryBox.Text {
		t.Fatal("UpdateState should store the latest query-box text")
	}

	var rect win.RECT
	if !win.GetWindowRect(host.hostWindow, &rect) {
		t.Fatal("UpdateState should allow reading the native host window rect")
	}

	if int(rect.Left) != state.QueryBox.Frame.X || int(rect.Top) != state.QueryBox.Frame.Y {
		t.Fatalf("unexpected host window position: left=%d top=%d", rect.Left, rect.Top)
	}

	if int(rect.Right-rect.Left) != state.QueryBox.Frame.Width || int(rect.Bottom-rect.Top) != state.QueryBox.Frame.Height {
		t.Fatalf("unexpected host window size: width=%d height=%d", rect.Right-rect.Left, rect.Bottom-rect.Top)
	}

	if !win.IsWindowVisible(host.hostWindow) {
		t.Fatal("visible query-box state should show the native host window")
	}

	if !win.IsWindowVisible(host.editControl) {
		t.Fatal("visible query-box state should show the native edit control")
	}

	if err := host.Focus(context.Background()); err != nil {
		t.Fatalf("Focus returned error: %v", err)
	}

	if !host.focused {
		t.Fatal("Focus should mark the Windows text input host as focused")
	}

	if err := host.Blur(context.Background()); err != nil {
		t.Fatalf("Blur returned error: %v", err)
	}

	if host.focused {
		t.Fatal("Blur should clear the focused flag")
	}

	if win.IsWindowVisible(host.hostWindow) {
		t.Fatal("Blur should hide the native host window")
	}

	if win.IsWindowVisible(host.editControl) {
		t.Fatal("Blur should hide the native edit control")
	}

	if err := host.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	if host.started {
		t.Fatal("Stop should clear the started flag")
	}

	if host.hostWindow != 0 || host.editControl != 0 {
		t.Fatal("Stop should destroy the Win32 resources")
	}
}

func TestNewDefaultTextInputHostReturnsWindowsHost(t *testing.T) {
	t.Parallel()

	host := NewDefaultTextInputHost()
	if _, ok := host.(*WindowsTextInputHost); !ok {
		t.Fatal("default Windows text input host should use the Windows-specific implementation")
	}
}
