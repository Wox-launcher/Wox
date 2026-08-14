//go:build windows && wox_automation

package woxui

/*
#include "renderer_windows.h"
*/
import "C"

import "errors"

// SimulateRendererDeviceRemoved makes the next rendered frame report DXGI_ERROR_DEVICE_REMOVED.
func (w *Window) SimulateRendererDeviceRemoved() error {
	if w == nil || w.native == nil || w.native.renderer == nil {
		return errors.New("window renderer is not initialized")
	}
	result := C.wox_renderer_simulate_device_removed(w.native.renderer.handle)
	if result < 0 {
		return hresultError("simulate renderer device removal", result)
	}
	return w.native.invalidate()
}
