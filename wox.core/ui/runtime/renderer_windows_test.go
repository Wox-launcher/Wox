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
