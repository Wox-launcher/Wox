package clipboard

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"
	"time"
)

var noDataErr = errors.New("no such data")
var notImplement = errors.New("not implemented")
var watchList = make([]func(Data), 0)
var isWatching = false
var WatchIntervalMillisecond = 500

type Type string

const (
	ClipboardTypeText  Type = "text"
	ClipboardTypeImage Type = "image"
	ClipboardTypeFile  Type = "file"
)

type Data interface {
	GetType() Type
	String() string
	MarshalJSON() ([]byte, error)
	UnmarshalJSON([]byte) error
}

func Read() (Data, error) {
	imageData, imgErr := readImage()
	if imgErr == nil {
		return &ImageData{
			Image: imageData,
		}, nil
	}

	return ReadFilesAndText()
}

func ReadFilesAndText() (Data, error) {
	filePaths, fileErr := readFilePaths()
	if fileErr == nil {
		return &FilePathData{
			FilePaths: filePaths,
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

func Watch(cb func(Data)) {
	if !isWatching {
		isWatching = true
		go func() {
			for {
				time.Sleep(time.Millisecond * time.Duration(WatchIntervalMillisecond))
				watchChange()
			}
		}()
	}

	watchList = append(watchList, cb)
}

func watchChange() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("failed to watch clipboard change: %s", err)
		}
	}()

	if isClipboardChanged() {
		data, err := Read()
		if err != nil {
			fmt.Printf("clipboard changed, but failed to read clipboard data: %s", err)
			return
		}

		for _, cb := range watchList {
			cbDummy := cb
			go func() {
				defer func() {
					if err1 := recover(); err1 != nil {
						fmt.Printf("failed to execute clipboard change: %s", err1)
					}
				}()

				cbDummy(data)
			}()
		}
	}
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

func (t *TextData) MarshalJSON() ([]byte, error) {
	var mapData = make(map[string]string)
	mapData["text"] = t.Text
	mapData["type"] = string(t.GetType())
	return json.Marshal(mapData)
}

func (t *TextData) UnmarshalJSON(data []byte) error {
	var mapData = make(map[string]string)
	err := json.Unmarshal(data, &mapData)
	if err != nil {
		return err
	}

	t.Text = mapData["text"]
	return nil
}

type FilePathData struct {
	FilePaths []string
}

func (f *FilePathData) GetType() Type {
	return ClipboardTypeFile
}

func (f *FilePathData) String() string {
	return strings.Join(f.FilePaths, ";")
}

func (f *FilePathData) MarshalJSON() ([]byte, error) {
	var mapData = make(map[string]string)
	mapData["filePaths"] = strings.Join(f.FilePaths, "``")
	mapData["type"] = string(f.GetType())
	return json.Marshal(mapData)
}

func (f *FilePathData) UnmarshalJSON(data []byte) error {
	var mapData = make(map[string]string)
	err := json.Unmarshal(data, &mapData)
	if err != nil {
		return err
	}

	f.FilePaths = strings.Split(mapData["filePaths"], "``")
	return nil
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

func (i *ImageData) MarshalJSON() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := png.Encode(buf, i.Image)
	if err != nil {
		return nil, err
	}

	var mapData = make(map[string]string)
	mapData["type"] = string(i.GetType())
	mapData["image"] = string(buf.Bytes())
	return json.Marshal(mapData)
}

func (i *ImageData) UnmarshalJSON(data []byte) error {
	var mapData = make(map[string]string)
	err := json.Unmarshal(data, &mapData)
	if err != nil {
		return err
	}

	imageBytes := []byte(mapData["image"])
	imageReader := bytes.NewReader(imageBytes)
	img, _, err := image.Decode(imageReader)
	if err != nil {
		return err
	}

	i.Image = img
	return nil
}
