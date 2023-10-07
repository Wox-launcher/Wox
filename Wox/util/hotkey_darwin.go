//go:build darwin

package util

import (
	"fmt"
	"github.com/samber/lo"
	"golang.design/x/hotkey"
	"strings"
)

// combineKey is a string like "Ctrl+Shift+Space", multiple modifiers and one key, combine examples:
// "command+Space", "Ctrl+Shift+Alt+Space", "ctrl+1"
func (h *Hotkey) parseCombineKey(combineKey string) ([]hotkey.Modifier, hotkey.Key, error) {
	keys := lo.Map(strings.Split(combineKey, "+"), func(item string, index int) string {
		return strings.TrimSpace(item)
	})

	var mods []hotkey.Modifier
	var key hotkey.Key

	for _, k := range keys {
		switch strings.ToLower(k) {
		case "ctrl":
			mods = append(mods, hotkey.ModCtrl)
		case "shift":
			mods = append(mods, hotkey.ModShift)
		case "option":
			mods = append(mods, hotkey.ModOption)
		case "cmd":
			mods = append(mods, hotkey.ModCmd)
		case "command":
			mods = append(mods, hotkey.ModCmd)
		case "a":
			key = hotkey.KeyA
		case "b":
			key = hotkey.KeyB
		case "c":
			key = hotkey.KeyC
		case "d":
			key = hotkey.KeyD
		case "e":
			key = hotkey.KeyE
		case "f":
			key = hotkey.KeyF
		case "g":
			key = hotkey.KeyG
		case "h":
			key = hotkey.KeyH
		case "i":
			key = hotkey.KeyI
		case "j":
			key = hotkey.KeyJ
		case "k":
			key = hotkey.KeyK
		case "l":
			key = hotkey.KeyL
		case "m":
			key = hotkey.KeyM
		case "n":
			key = hotkey.KeyN
		case "o":
			key = hotkey.KeyO
		case "p":
			key = hotkey.KeyP
		case "q":
			key = hotkey.KeyQ
		case "r":
			key = hotkey.KeyR
		case "s":
			key = hotkey.KeyS
		case "t":
			key = hotkey.KeyT
		case "u":
			key = hotkey.KeyU
		case "v":
			key = hotkey.KeyV
		case "w":
			key = hotkey.KeyW
		case "x":
			key = hotkey.KeyX
		case "y":
			key = hotkey.KeyY
		case "z":
			key = hotkey.KeyZ
		case "0":
			key = hotkey.Key0
		case "1":
			key = hotkey.Key1
		case "2":
			key = hotkey.Key2
		case "3":
			key = hotkey.Key3
		case "4":
			key = hotkey.Key4
		case "5":
			key = hotkey.Key5
		case "6":
			key = hotkey.Key6
		case "7":
			key = hotkey.Key7
		case "8":
			key = hotkey.Key8
		case "9":
			key = hotkey.Key9
		case "space":
			key = hotkey.KeySpace
		case "return":
			key = hotkey.KeyReturn
		case "escape":
			key = hotkey.KeyEscape
		case "tab":
			key = hotkey.KeyTab
		case "delete":
			key = hotkey.KeyDelete
		case "left":
			key = hotkey.KeyLeft
		case "right":
			key = hotkey.KeyRight
		case "up":
			key = hotkey.KeyUp
		case "down":
			key = hotkey.KeyDown
		case "f1":
			key = hotkey.KeyF1
		case "f2":
			key = hotkey.KeyF2
		case "f3":
			key = hotkey.KeyF3
		case "f4":
			key = hotkey.KeyF4
		case "f5":
			key = hotkey.KeyF5
		case "f6":
			key = hotkey.KeyF6
		case "f7":
			key = hotkey.KeyF7
		case "f8":
			key = hotkey.KeyF8
		case "f9":
			key = hotkey.KeyF9
		case "f10":
			key = hotkey.KeyF10
		case "f11":
			key = hotkey.KeyF11
		case "f12":
			key = hotkey.KeyF12
		default:
			return nil, 0, fmt.Errorf("invalid key: %s", k)
		}
	}

	return mods, key, nil
}
