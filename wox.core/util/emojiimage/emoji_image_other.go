//go:build !darwin && !windows

package emojiimage

import (
	"errors"
	"image"
)

// Render reports that native color emoji rendering is unavailable.
func Render(emoji string, size int) (image.Image, error) {
	return nil, errors.New("native emoji rendering not supported on this platform")
}
