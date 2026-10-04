package common

import (
	"github.com/disintegration/imaging"
	"image"
)

// ImageThumbnailSize bounds physical cache pixels; UI owns the logical size and chrome.
const ImageThumbnailSize = 96

// NewImageThumbnail preserves the source aspect ratio without baking in theme-dependent decoration.
func NewImageThumbnail(source image.Image) image.Image {
	return imaging.Fit(source, ImageThumbnailSize, ImageThumbnailSize, imaging.Lanczos)
}
