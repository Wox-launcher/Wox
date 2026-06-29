//go:build !linux || !cgo

package keyboard

func registerGlobalHotkeysLinuxHyprland(specs []GlobalHotkeySpec) (HotkeyRegistration, bool, error) {
	return nil, false, nil
}

func InvokeHyprlandHotkeyCallback(key string) {}

func RegisterHyprlandHotkeyCallback(key string, callback func()) {}