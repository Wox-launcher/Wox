package util

import (
	"errors"
	"fmt"
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
	if SimulateCtrlC() != nil {
		return Selection{}, errors.New("error simulate ctrl c")
	}

	data, readErr := ClipboardRead()
	if readErr != nil {
		return Selection{}, fmt.Errorf("error read clipboard: %w", readErr)
	}

	if data.Type == ClipboardTypeText {
		return Selection{
			Type: SelectTypeText,
			Data: string(data.Data),
		}, nil
	}

	return Selection{}, errors.New("no data in clipboard")
}
