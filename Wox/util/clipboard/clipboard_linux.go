package clipboard

import "image"

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

func writeImageData(img image.Image) error {
	return notImplement
}

func isClipboardChanged() bool {
	return false
}
