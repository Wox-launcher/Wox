package hotkey

import "C"
import (
	"context"
	"fmt"
)

var initialized = false

func onKey(keyCode int) {
	fmt.Print(keyCode)
}

func InitHotkey() {
	if initialized {
		return
	}

	initialized = true
	//registerKeyboardListener()
}

func Register(ctx context.Context, combineKey string, callback func()) error {
	return nil
}
