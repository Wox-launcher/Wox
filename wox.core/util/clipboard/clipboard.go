package clipboard

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"
	"sync/atomic"
	"time"
	"wox/util"
)

var noDataErr = errors.New("no such data")
var notImplement = errors.New("not implemented")
var watchList = make([]func(Data), 0)
var isWatching = false
var WatchIntervalMillisecond = 250

// lastWriteTimestamp tracks the last time Wox wrote to the clipboard (UnixMilli).
// Used to prevent the polling loop from self-triggering on our own writes.
var lastWriteTimestamp atomic.Int64

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
	lastWriteTimestamp.Store(time.Now().UnixMilli())
	if data.GetType() == ClipboardTypeText {
		return writeTextData(data.String())
	}
	if data.GetType() == ClipboardTypeImage {
		return writeImageData(data.(*ImageData).Image)
	}

	return errors.New("not implemented")
}

func WriteImageBytes(pngData []byte, dibData []byte) error {
	lastWriteTimestamp.Store(time.Now().UnixMilli())
	return writeImageBytes(pngData, dibData)
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
			util.GetLogger().Error(context.Background(), fmt.Sprintf("clipboard: watchChange panic: %v", err))
		}
	}()

	if !isClipboardChanged() {
		return
	}

	// Skip changes caused by our own writes to prevent self-triggering.
	// This handles the race where the polling goroutine detects a sequence number
	// change before the write goroutine updates lastSeqNum.
	if time.Now().UnixMilli()-lastWriteTimestamp.Load() < 200 {
		return
	}

	// Debounce: wait briefly to let the clipboard settle.
	// When the user rapidly copies items, this avoids opening the clipboard
	// while the source application is still writing, reducing lock contention.
	time.Sleep(50 * time.Millisecond)

	// If the clipboard changed again during the debounce window, skip this read.
	// The next polling cycle will pick up the latest change.
	if isClipboardChanged() {
		return
	}

	start := time.Now()
	data, err := Read()
	if err != nil {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("clipboard: changed but failed to read: %v", err))
		return
	}

	if d := time.Since(start); d > 200*time.Millisecond {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("clipboard: Read took %s (type=%s)", d.String(), data.GetType()))
	}

	for _, cb := range watchList {
		go func() {
			defer func() {
				if err1 := recover(); err1 != nil {
					util.GetLogger().Error(context.Background(), fmt.Sprintf("clipboard: callback panic: %v", err1))
				}
			}()

			cb(data)
		}()
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

	return json.Marshal(base64.StdEncoding.EncodeToString(buf.Bytes()))
}

func (i *ImageData) UnmarshalJSON(data []byte) error {
	var base64ImgData string
	unmarshalErr := json.Unmarshal(data, &base64ImgData)
	if unmarshalErr != nil {
		return unmarshalErr
	}

	decodeBytes, err := base64.StdEncoding.DecodeString(base64ImgData)
	if err != nil {
		return err
	}

	img, decodeErr := png.Decode(bytes.NewReader(decodeBytes))
	if decodeErr != nil {
		return decodeErr
	}

	i.Image = img
	return nil
}
