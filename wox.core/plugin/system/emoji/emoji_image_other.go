//go:build !darwin && !windows

package emoji

import (
	"errors"
	"image"
)

func getNativeEmojiImage(emoji string, size int) (image.Image, error) {
	return nil, errors.New("native emoji rendering not supported on this platform")
}
