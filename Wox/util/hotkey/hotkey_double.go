package hotkey

import (
	"context"
	"fmt"
	"github.com/robotn/gohook"
	"golang.design/x/hotkey"
	"wox/util"
)

var initialized = false
var endHookChan chan bool
var lastKeyUpTimestamp = util.NewHashMap[uint16, int64]()
var keyCallback = util.NewHashMap[uint16, func()]()

func registerDoubleHotKey(ctx context.Context, modifier hotkey.Modifier, callback func()) error {
	keyCode, err := getModifierKeyCode(ctx, modifier)
	if err != nil {
		return err
	}
	keyCallback.Store(keyCode, callback)

	if initialized {
		return nil
	}
	initialized = true
	endHookChan = make(chan bool)

	util.Go(context.Background(), "double key listener", func() {
		evChan := hook.Start()
		defer hook.End()

		for {
			select {
			case ev := <-evChan:
				if ev.Kind == hook.KeyUp {
					util.GetLogger().Info(ctx, fmt.Sprintf("keycode: %d, rawkeycode: %d", ev.Keycode, ev.Rawcode))
					if cb, callbackExist := keyCallback.Load(ev.Keycode); callbackExist {
						var keyUpMaxInterval int64 = 500
						if v, ok := lastKeyUpTimestamp.Load(ev.Keycode); ok {
							if util.GetSystemTimestamp()-v < keyUpMaxInterval {
								lastKeyUpTimestamp.Delete(ev.Keycode)
								util.Go(context.Background(), "double hotkey callback", func() {
									cb()
								})
							}
						}

						lastKeyUpTimestamp.Store(ev.Keycode, util.GetSystemTimestamp())
					}
				}
			case <-endHookChan:
				util.GetLogger().Info(ctx, fmt.Sprintf("unregister double hotkey event received, exit loop"))
				return
			default:
			}
		}
	})

	return nil
}

func unregisterDoubleHotkey(ctx context.Context, modifier hotkey.Modifier) error {
	keyCode, err := getModifierKeyCode(ctx, modifier)
	if err != nil {
		return err
	}

	keyCallback.Delete(keyCode)
	if keyCallback.Len() > 0 {
		return nil
	}

	endHookChan <- true
	initialized = false
	return nil
}
