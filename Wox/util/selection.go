package util

import (
	"errors"
	"strings"
	"time"
	"wox/util/clipboard"
	"wox/util/keyboard"
)

var noSelection = errors.New("no selection")
var lastClipboardChangeTimestamp int64 = 0

type SelectionType string

const (
	SelectionTypeText SelectionType = "text"
	SelectionTypeFile SelectionType = "file"
)

type Selection struct {
	Type SelectionType
	// Only available when Type is SelectionTypeText
	Text string
	// Only available when Type is SelectionTypeFile
	FilePaths []string
}

func InitSelection() {
	clipboard.Watch(func(data clipboard.Data) {
		lastClipboardChangeTimestamp = GetSystemTimestamp()
	})
}

func (s *Selection) String() string {
	switch s.Type {
	case SelectionTypeText:
		return s.Text
	case SelectionTypeFile:
		return strings.Join(s.FilePaths, ";")
	}

	return ""
}

func GetSelected() (Selection, error) {
	simulateStartTimestamp := GetSystemTimestamp()
	if keyboard.SimulateCopy() != nil {
		return Selection{}, errors.New("error simulate ctrl c")
	}

	// wait for clipboard data to be updated
	time.Sleep(200 * time.Millisecond)

	simulateEndTimestamp := GetSystemTimestamp()

	clipboardDataAfter, err := clipboard.ReadFilesAndText()
	if err != nil {
		return Selection{}, err
	}

	// clipboard data must be updated between simulateStartTimestamp and simulateEndTimestamp
	// otherwise, it means that the clipboard data is not updated by the simulated ctrl c
	if lastClipboardChangeTimestamp < simulateStartTimestamp || lastClipboardChangeTimestamp > simulateEndTimestamp {
		return Selection{}, noSelection
	}

	switch clipboardDataAfter.GetType() {
	case clipboard.ClipboardTypeText:
		textData := clipboardDataAfter.(*clipboard.TextData)
		return Selection{
			Type: SelectionTypeText,
			Text: textData.Text,
		}, nil
	case clipboard.ClipboardTypeFile:
		fileData := clipboardDataAfter.(*clipboard.FilePathData)
		return Selection{
			Type:      SelectionTypeFile,
			FilePaths: fileData.FilePaths,
		}, nil
	}

	return Selection{}, errors.New("unknown clipboard type")
}
