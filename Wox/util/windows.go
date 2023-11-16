package util

import (
	"fmt"
	"github.com/go-vgo/robotgo"
	"wox/util/keyboard"
)

func GetActiveWindowHash() string {
	activePid := robotgo.GetPid()
	activeTitle := robotgo.GetTitle()
	return Md5([]byte(fmt.Sprintf("%s%d", activeTitle, activePid)))
}

func SimulateCtrlC() error {
	kb, err := keyboard.NewKeyBonding()
	if err != nil {
		return err
	}

	kb.SetKeys(keyboard.VK_C)
	if IsWindows() || IsLinux() {
		kb.HasCTRL(true)
	}
	if IsMacOS() {
		kb.HasSuper(true)
	}
	err = kb.Launching()
	if err != nil {
		return err
	}

	return nil
}

func SimulateCtrlV() error {
	kb, err := keyboard.NewKeyBonding()
	if err != nil {
		return err
	}

	kb.SetKeys(keyboard.VK_V)
	if IsWindows() || IsLinux() {
		kb.HasCTRL(true)
	}
	if IsMacOS() {
		kb.HasSuper(true)
	}
	err = kb.Launching()
	if err != nil {
		return err
	}

	return nil
}
