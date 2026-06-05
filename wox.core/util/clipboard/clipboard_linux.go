package clipboard

import "image"

// readClipboardContentType is not implemented on Linux.
func readClipboardContentType() Type {
	return ""
}

func readText() (string, error) {
	return "", notImplement
}

func readFilePaths() ([]string, error) {
	return nil, notImplement
}

func readImage() (image.Image, error) {
	return nil, notImplement
}

func writeTextData(text string) error {
	return notImplement
}

func writeFilePaths(filePaths []string) error {
	return notImplement
}

func writeImageData(img image.Image) error {
	return notImplement
}

func writeImageBytes(pngData []byte, dibData []byte) error {
	return notImplement
}

func isClipboardChanged() bool {
	return false
}

func buildWatchSnapshot() string {
	return ""
}
