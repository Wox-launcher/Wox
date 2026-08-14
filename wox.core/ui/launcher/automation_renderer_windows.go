//go:build windows && wox_automation

package launcher

import (
	"errors"

	woxui "wox/ui/runtime"
)

// SimulateAutomationRendererDeviceRemoved injects a recoverable device-loss HRESULT into the active window.
func (a *App) SimulateAutomationRendererDeviceRemoved() error {
	_, window, _ := a.automationSurface()
	if window == nil {
		return errors.New("active automation window is not initialized")
	}
	var faultErr error
	if err := woxui.Call(func() {
		faultErr = window.SimulateRendererDeviceRemoved()
	}); err != nil {
		return err
	}
	return faultErr
}
