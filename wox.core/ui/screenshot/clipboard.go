package screenshot

import "image"

// publishScreenshotClipboard marks the image pasteable before the capture worker continues with persistence.
// Keeping that work on the same worker preserves native pixels until scene snapshots finish.
func publishScreenshotClipboard(write func(image.Image, []byte) error, source image.Image, png []byte, onReady func()) error {
	if err := write(source, png); err != nil {
		return err
	}
	if onReady != nil {
		onReady()
	}
	return nil
}
