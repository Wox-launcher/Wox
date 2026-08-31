//go:build windows

package woxui

import (
	"errors"
	"testing"
)

func TestRecoverableRendererErrors(t *testing.T) {
	for _, code := range []uint32{d2dErrRecreateTarget, dxgiErrDeviceRemoved, dxgiErrDeviceHung, dxgiErrDeviceReset, dxgiErrDriverInternalError} {
		if !isRecoverableRendererError(&windowsRendererError{operation: "render", code: code}) {
			t.Fatalf("HRESULT %#x should be recoverable", code)
		}
	}
	if isRecoverableRendererError(&windowsRendererError{operation: "render", code: 0x80070057}) {
		t.Fatal("E_INVALIDARG should not be recoverable")
	}
	if isRecoverableRendererError(errors.New("render failed")) {
		t.Fatal("untyped errors should not be recoverable")
	}
}

func TestWindowsRendererDiagnosticEnvironment(t *testing.T) {
	t.Setenv(windowsRenderTraceEnvironment, "1")
	t.Setenv(windowsForceWARPEnvironment, "1")
	if !windowsRenderTraceEnabled() || !windowsForceWARPEnabled() {
		t.Fatal("Windows renderer diagnostic environment was not enabled")
	}
	t.Setenv(windowsRenderTraceEnvironment, "true")
	t.Setenv(windowsForceWARPEnvironment, "true")
	if windowsRenderTraceEnabled() || windowsForceWARPEnabled() {
		t.Fatal("Windows renderer diagnostic environment should require value 1")
	}
}

func TestSuspendedRendererKeepsPendingState(t *testing.T) {
	renderer := nativeRenderer{width: 640, height: 480}
	if err := renderer.trim(); err != nil {
		t.Fatalf("trim renderer: %v", err)
	}
	if err := renderer.resize(800, 600); err != nil {
		t.Fatalf("resize suspended renderer: %v", err)
	}
	if err := renderer.setFontFamily("Segoe UI Variable"); err != nil {
		t.Fatalf("set suspended renderer font: %v", err)
	}
	if renderer.handle != nil || renderer.width != 800 || renderer.height != 600 || renderer.fontFamily != "Segoe UI Variable" {
		t.Fatalf("suspended renderer state = %+v", renderer)
	}
}
