package window

import (
	"context"
	"errors"
	"fmt"
	"image"
	"time"
	"wox/util"
	"wox/util/clipboard"
)

// WriteClipboardText dispatches publication to the native window's owner thread.
func (w *Window) WriteClipboardText(text string) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.writeClipboardText(text)
}

// WriteClipboardImageFile prepares formats outside the UI dispatcher.
func (w *Window) WriteClipboardImageFile(path string) error {
	return w.writeClipboardImage(func() (*clipboard.PreparedImage, error) { return clipboard.PrepareImageFile(path) })
}

func (w *Window) WriteClipboardImage(source image.Image) error {
	return w.WriteClipboardImageWithPNG(source, nil)
}

// WriteClipboardImageWithPNG lets the clipboard package reuse the caller's encoded image.
func (w *Window) WriteClipboardImageWithPNG(source image.Image, encodedPNG []byte) error {
	return w.writeClipboardImage(func() (*clipboard.PreparedImage, error) { return clipboard.PrepareImage(source, encodedPNG) })
}

// writeClipboardImage holds prepared formats through synchronous dispatch and interrupted window teardown.
func (w *Window) writeClipboardImage(prepare func() (*clipboard.PreparedImage, error)) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	started := time.Now()
	prepared, err := prepare()
	if err != nil {
		return err
	}
	defer prepared.Close()
	preparedAt := time.Now()
	err = w.native.writeClipboardImage(prepared)
	size := prepared.Size()
	util.GetLogger().Debug(context.Background(), fmt.Sprintf(
		"clipboard_image prepareMs=%d commitMs=%d size=%dx%d success=%t",
		preparedAt.Sub(started).Milliseconds(), time.Since(preparedAt).Milliseconds(), size.X, size.Y, err == nil,
	))
	return err
}
