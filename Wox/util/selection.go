package util

import (
	"errors"
	"github.com/google/uuid"
	"strings"
	"wox/util/clipboard"
)

var noSelection = errors.New("no selection")

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
	clipboardDataBefore, errBefore := clipboard.Read()
	if errBefore != nil {
		clipboardDataBefore = &clipboard.TextData{Text: uuid.NewString()}
	}

	if SimulateCtrlC() != nil {
		return Selection{}, errors.New("error simulate ctrl c")
	}

	clipboardDataAfter, err := clipboard.Read()
	if err != nil {
		return Selection{}, err
	}

	// check if clipboard data is changed
	if clipboardDataBefore.GetType() == clipboardDataAfter.GetType() {
		if clipboardDataBefore.GetType() == clipboard.ClipboardTypeImage {
			imgBefore := clipboardDataBefore.(*clipboard.ImageData).Image
			imgAfter := clipboardDataAfter.(*clipboard.ImageData).Image
			if imgBefore.Bounds().Eq(imgAfter.Bounds()) {
				return Selection{}, noSelection
			}
		} else {
			if clipboardDataBefore.String() == clipboardDataAfter.String() {
				return Selection{}, noSelection
			}
		}
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
