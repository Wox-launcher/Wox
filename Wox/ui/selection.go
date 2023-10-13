package ui

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"wox/util"
	"wox/util/keybd_event"
)

type SelectType string

const (
	SelectTypeText SelectType = "text"
)

type Selection struct {
	Type SelectType
	Data string
}

func GetSelectedText() (Selection, error) {
	kb, err := keybd_event.NewKeyBonding()
	if err != nil {
		return Selection{}, err
	}

	kb.SetKeys(keybd_event.VK_C)
	if strings.ToLower(runtime.GOOS) == "windows" || strings.ToLower(runtime.GOOS) == "linux" {
		kb.HasCTRL(true)
	}
	if strings.ToLower(runtime.GOOS) == "darwin" {
		kb.HasSuper(true)
	}
	err = kb.Launching()
	if err != nil {
		return Selection{}, fmt.Errorf("error send copy command: %w", err)
	}

	data, readErr := util.ClipboardRead()
	if readErr != nil {
		return Selection{}, fmt.Errorf("error read clipboard: %w", readErr)
	}

	if data.Type == util.ClipboardTypeText {
		return Selection{
			Type: SelectTypeText,
			Data: string(data.Data),
		}, nil
	}

	return Selection{}, errors.New("no data in clipboard")
}
