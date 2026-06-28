//go:build !windows && !darwin

package ui

// On platforms without a native renderer (Linux etc.), NewNativeRenderer
// returns an error. The caller (gpuUIImpl.Init) handles the fallback to the
// WebSocket-based Flutter UI. When a Linux native backend is added, replace
// this file with ui_linux.go + ui_linux.c implementing the same ABI.

func NewNativeRenderer(width, height int, theme Theme, eventCb EventCallback) (NativeRenderer, error) {
	return nil, &WindowError{Op: "create", Err: "native renderer not implemented on this platform"}
}