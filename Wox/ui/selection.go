package ui

import (
	"fmt"
	"golang.design/x/clipboard"
	"runtime"
	"strings"
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

func GetSelected() (Selection, error) {
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

	clipboardErr := clipboard.Init()
	if clipboardErr != nil {
		return Selection{}, fmt.Errorf("error init clipboard: %w", clipboardErr)
	}
	data := clipboard.Read(clipboard.FmtText)

	return Selection{
		Type: SelectTypeText,
		Data: string(data),
	}, nil
}
