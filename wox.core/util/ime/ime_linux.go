package ime

import "wox/util"

func SwitchInputMethodABC() error {
	if util.IsHyprlandSession() {
		return switchInputMethodABCHyprland()
	}
	return nil
}
