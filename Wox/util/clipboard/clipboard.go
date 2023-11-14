package clipboard

import (
	"errors"
	"fmt"
	"image"
	"strings"
)

var noDataErr = errors.New("no such data")
var notImplement = errors.New("not implemented")

type Type string

const (
	ClipboardTypeText  Type = "text"
	ClipboardTypeImage Type = "image"
	ClipboardTypeFile  Type = "file"
)

type Data interface {
	GetType() Type
	String() string
}

func Read() (Data, error) {
	filePaths, fileErr := readFilePaths()
	if fileErr == nil {
		return &FilePathData{
			FilePaths: filePaths,
		}, nil
	}

	imageData, imgErr := readImage()
	if imgErr == nil {
		return &ImageData{
			Image: imageData,
		}, nil
	}

	textData, txtErr := readText()
	if txtErr == nil {
		return &TextData{
			Text: textData,
		}, nil
	}

	return nil, noDataErr
}

func Write(data Data) error {
	if data.GetType() == ClipboardTypeText {
		return writeTextData(data.String())
	}

	return errors.New("not implemented")
}

func WriteText(text string) error {
	return Write(&TextData{
		Text: text,
	})
}

type TextData struct {
	Text string
}

func (t *TextData) GetType() Type {
	return ClipboardTypeText
}

func (t *TextData) String() string {
	return t.Text
}

type FilePathData struct {
	FilePaths []string
}

func (f *FilePathData) GetType() Type {
	return ClipboardTypeFile
}

func (t *FilePathData) String() string {
	return strings.Join(t.FilePaths, ";")
}

type ImageData struct {
	Image image.Image
}

func (i *ImageData) GetType() Type {
	return ClipboardTypeImage
}

func (i *ImageData) String() string {
	b := i.Image.Bounds()
	return fmt.Sprintf("image(%dx%d)", b.Dx(), b.Dy())
}
