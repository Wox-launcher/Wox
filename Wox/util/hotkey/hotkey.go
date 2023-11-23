package hotkey

import "C"
import (
	"context"
	"fmt"
	hook "github.com/robotn/gohook"
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

	go func() {
		evChan := hook.Start()
		defer hook.End()

		for ev := range evChan {
			fmt.Println("hook: ", ev)
		}
	}()
}

func Register(ctx context.Context, combineKey string, callback func()) error {
	return nil
}
