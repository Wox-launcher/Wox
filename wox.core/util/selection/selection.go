package selection

import (
	"context"
	"errors"
	"strings"
	"time"
	"wox/util"
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
		lastClipboardChangeTimestamp = util.GetSystemTimestamp()
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

func (s *Selection) IsEmpty() bool {
	switch s.Type {
	case SelectionTypeText:
		return s.Text == ""
	case SelectionTypeFile:
		return s.FilePaths == nil || len(s.FilePaths) == 0
	}

	return false
}

func getSelectedByClipboard(ctx context.Context) (Selection, error) {
	simulateStartTimestamp := util.GetSystemTimestamp()
	if keyboard.SimulateCopy() != nil {
		return Selection{}, errors.New("error simulate ctrl c")
	}

	// loop to wait for clipboard data to be updated, so that we can get the clipboard data as soon as possible if the clipboard data is updated
	// because sometimes clipboard data is not updated immediately after simulated ctrl c, small text data is updated immediately, but large file data is not
	var clipboardDataAfter clipboard.Data
	loopTimes := 10
	for i := 0; i < loopTimes; i++ {
		isLastLoop := loopTimes-1 == i

		// wait for clipboard data to be updated
		time.Sleep(50 * time.Millisecond)

		clipboardData, err := clipboard.ReadFilesAndText()
		if err != nil {
			if isLastLoop {
				return Selection{}, err
			} else {
				continue
			}
		}

		// clipboard data must be updated between simulateStartTimestamp and simulateEndTimestamp
		// otherwise, it means that the clipboard data is not updated by the simulated ctrl c
		if lastClipboardChangeTimestamp < simulateStartTimestamp {
			if isLastLoop {
				return Selection{}, noSelection
			} else {
				continue
			}
		}

		clipboardDataAfter = clipboardData
		break
	}

	if clipboardDataAfter == nil {
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
