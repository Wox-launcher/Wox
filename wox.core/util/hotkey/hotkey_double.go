package hotkey

import (
	"context"
	"time"
	"wox/util"

	hook "github.com/robotn/gohook"
	"golang.design/x/hotkey"
)

var initialized = false
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

	util.Go(context.Background(), "double key listener", func() {
		evChan := hook.Start()
		for {
			select {
			case ev := <-evChan:
				if ev.Kind == hook.KeyUp {
					// util.GetLogger().Info(ctx, fmt.Sprintf("hotkey event received, ev: %v", ev.Keycode))
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
			default:
				// avoid 100% cpu usage
				time.Sleep(20 * time.Millisecond)
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
	return nil
}
