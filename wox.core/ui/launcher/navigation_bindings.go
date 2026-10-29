package launcher

import (
	"runtime"

	"wox/setting"
	woxui "wox/ui/runtime"
)

// navigationDirection resolves the platform's optional result navigation shortcuts.
func navigationDirection(mode setting.NavigationBindingMode, event woxui.KeyEvent) woxui.Key {
	return navigationDirectionForPlatform(mode, event, runtime.GOOS)
}

func navigationDirectionForPlatform(mode setting.NavigationBindingMode, event woxui.KeyEvent, platform string) woxui.Key {
	modifier := woxui.KeyModifierAlt
	if platform == "darwin" {
		modifier = woxui.KeyModifierControl
	}
	if !event.Down || event.Composing || event.Modifiers != modifier {
		return ""
	}
	switch mode {
	case setting.NavigationBindingModeEmacs:
		switch event.Key {
		case "p":
			return woxui.KeyArrowUp
		case "n":
			return woxui.KeyArrowDown
		case "b":
			return woxui.KeyArrowLeft
		case "f":
			return woxui.KeyArrowRight
		}
	case setting.NavigationBindingModeVim:
		switch event.Key {
		case "k":
			return woxui.KeyArrowUp
		case "j":
			return woxui.KeyArrowDown
		case "h":
			return woxui.KeyArrowLeft
		case "l":
			return woxui.KeyArrowRight
		}
	}
	return ""
}
