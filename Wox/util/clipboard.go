package util

import (
	"context"
	"errors"
	"golang.design/x/clipboard"
)

var watchCallbacks []func(ClipboardData)

type ClipboardType string

const (
	ClipboardTypeText  = "text"
	ClipboardTypeImage = "image"
)

type ClipboardData struct {
	Type ClipboardType
	Data []byte
}

func ClipboardInit() error {
	clipboardErr := clipboard.Init()
	if clipboardErr != nil {
		return clipboardErr
	}

	watchTextChan := clipboard.Watch(context.Background(), clipboard.FmtText)
	Go(context.Background(), "watch text clipboard", func() {
		for {
			select {
			case textData := <-watchTextChan:
				if textData != nil {
					for _, callback := range watchCallbacks {
						callbackDummy := callback
						Go(context.Background(), "clipboard text watch callback", func() {
							callbackDummy(ClipboardData{
								Type: ClipboardTypeText,
								Data: textData,
							})
						})
					}
				}
			}
		}
	})

	watchImageChan := clipboard.Watch(context.Background(), clipboard.FmtImage)
	Go(context.Background(), "watch image clipboard", func() {
		for {
			select {
			case imgData := <-watchImageChan:
				if imgData != nil {
					for _, callback := range watchCallbacks {
						callbackDummy := callback
						Go(context.Background(), "clipboard image watch callback", func() {
							callbackDummy(ClipboardData{
								Type: ClipboardTypeImage,
								Data: imgData,
							})
						})
					}
				}
			}
		}
	})

	return nil
}

func ClipboardRead() (ClipboardData, error) {
	data := clipboard.Read(clipboard.FmtText)
	if data != nil {
		return ClipboardData{
			Type: ClipboardTypeText,
			Data: data,
		}, nil
	}

	data = clipboard.Read(clipboard.FmtImage)
	if data != nil {
		return ClipboardData{
			Type: ClipboardTypeImage,
			Data: data,
		}, nil
	}

	return ClipboardData{}, errors.New("no data in clipboard")
}

func ClipboardWrite(data ClipboardData) {
	switch data.Type {
	case ClipboardTypeText:
		clipboard.Write(clipboard.FmtText, data.Data)
	case ClipboardTypeImage:
		clipboard.Write(clipboard.FmtImage, data.Data)
	}
}

func ClipboardWriteText(text string) {
	ClipboardWrite(ClipboardData{
		Type: ClipboardTypeText,
		Data: []byte(text),
	})
}

func ClipboardWatch(callback func(data ClipboardData)) {
	watchCallbacks = append(watchCallbacks, callback)
}
