package util

import (
	"context"
	"fmt"
	"golang.design/x/hotkey"
)

type Hotkey struct {
	hk             *hotkey.Hotkey
	unregisterChan chan bool
	combineKey     string
}

func (h *Hotkey) Register(ctx context.Context, combineKey string, callback func()) error {
	modifiers, key, parseErr := h.parseCombineKey(combineKey)
	if parseErr != nil {
		return parseErr
	}
	newHk := hotkey.New(modifiers, key)
	err := newHk.Register()
	if err != nil {
		return err
	}

	h.Unregister(ctx)
	h.unregisterChan = make(chan bool)
	h.hk = newHk
	h.combineKey = combineKey
	GetLogger().Info(ctx, fmt.Sprintf("register hotkey: %s", h.combineKey))

	Go(ctx, "", func() {
		for {
			select {
			case <-h.hk.Keyup():
				Go(ctx, "hotkey callback", func() {
					if callback != nil {
						callback()
					}
				})
			case <-h.unregisterChan:
				GetLogger().Error(ctx, "unregister hotkey event received, exit loop")
				return
			}
		}
	})

	return nil
}

func (h *Hotkey) Unregister(ctx context.Context) {
	if h.hk == nil && h.unregisterChan == nil {
		return
	}

	GetLogger().Info(ctx, fmt.Sprintf("unregister hotkey: %s", h.combineKey))
	if h.unregisterChan != nil {
		h.unregisterChan <- true
		close(h.unregisterChan)
	}

	if h.hk != nil {
		h.hk.Unregister()
	}
}
