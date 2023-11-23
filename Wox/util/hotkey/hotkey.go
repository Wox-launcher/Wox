package hotkey

import (
	"context"
	"fmt"
	"golang.design/x/hotkey"
	"wox/util"
)

type Hotkey struct {
	combineKey string

	//normal key
	hk             *hotkey.Hotkey
	unregisterChan chan bool

	//double key
	isDoubleKey       bool
	doubleKeyModifier hotkey.Modifier
}

func (h *Hotkey) Register(ctx context.Context, combineKey string, callback func()) error {
	modifiers, key, parseErr := h.parseCombineKey(combineKey)
	if parseErr != nil {
		return parseErr
	}

	h.combineKey = combineKey

	if len(modifiers) == 2 && modifiers[0] == modifiers[1] {
		util.GetLogger().Info(ctx, fmt.Sprintf("register double hotkey: %s", combineKey))
		h.isDoubleKey = true
		h.doubleKeyModifier = modifiers[0]
		return registerDoubleHotKey(ctx, modifiers[0], callback)
	}

	return h.registerNormalKey(ctx, modifiers, key, callback)
}

func (h *Hotkey) registerNormalKey(ctx context.Context, modifiers []hotkey.Modifier, key hotkey.Key, callback func()) error {
	newHk := hotkey.New(modifiers, key)
	err := newHk.Register()
	if err != nil {
		return err
	}

	h.Unregister(ctx)
	h.unregisterChan = make(chan bool)
	h.hk = newHk
	util.GetLogger().Info(ctx, fmt.Sprintf("register normal hotkey: %s", h.combineKey))

	util.Go(ctx, "", func() {
		for {
			select {
			case <-h.hk.Keyup():
				util.Go(ctx, "normal hotkey callback", func() {
					if callback != nil {
						callback()
					}
				})
			case <-h.unregisterChan:
				util.GetLogger().Error(ctx, "unregister normal hotkey event received, exit loop")
				return
			}
		}
	})

	return nil
}

func (h *Hotkey) Unregister(ctx context.Context) {
	if h.isDoubleKey {
		util.GetLogger().Info(ctx, fmt.Sprintf("unregister double hotkey: %s", h.combineKey))
		unregisterDoubleHotkey(ctx, h.doubleKeyModifier)
		return
	}

	if h.hk == nil && h.unregisterChan == nil {
		return
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("unregister normal hotkey: %s", h.combineKey))
	if h.unregisterChan != nil {
		h.unregisterChan <- true
		close(h.unregisterChan)
	}

	if h.hk != nil {
		h.hk.Unregister()
	}
}
