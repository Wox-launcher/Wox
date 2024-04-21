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

	h.isDoubleKey = false
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

	currentKey := h.combineKey
	util.GetLogger().Info(ctx, fmt.Sprintf("register normal hotkey: %s", currentKey))

	util.Go(ctx, "", func() {
		for {
			select {
			case <-h.hk.Keyup():
				util.Go(ctx, fmt.Sprintf("normal hotkey (%s) callback", currentKey), func() {
					if callback != nil {
						callback()
					}
				})
			case <-h.unregisterChan:
				util.GetLogger().Error(ctx, fmt.Sprintf("unregister normal hotkey (%s) event received, exit loop", currentKey))
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
		//make sure unregisterChan is not closed and then close it
		select {
		case <-h.unregisterChan:
		default:
			close(h.unregisterChan)
		}
	}

	if h.hk != nil {
		unregisterErr := h.hk.Unregister()
		if unregisterErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to unregister hotkey: %s", unregisterErr.Error()))
		}
	}
}

func IsHotkeyAvailable(ctx context.Context, hotkeyStr string) (isAvailable bool) {
	isAvailable = false
	hk := Hotkey{}
	registerErr := hk.Register(ctx, hotkeyStr, func() {})
	if registerErr == nil {
		isAvailable = true
		hk.Unregister(ctx)
	}

	return
}
