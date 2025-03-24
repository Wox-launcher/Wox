package entity

import (
	"fmt"
	"testing"
	"wox/util"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
)

func TestWoxImage_ToImage(t *testing.T) {
	errIcon := NewWoxImageBase64(`data:image/png;base64,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`)
	overlayIcon := NewWoxImageEmoji("🚫")
	icon := errIcon.OverlayFullPercentage(overlayIcon, 1)

	img, err := icon.ToImage()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
		return
	}

	util.GetLocation().Init()
	path := fmt.Sprintf("%s/%s.png", util.GetLocation().GetImageCacheDirectory(), uuid.NewString())
	imaging.Save(img, path)
	t.Log(path)
}

func TestWoxImage_Emoji(t *testing.T) {
	util.GetLocation().Init()
	emojiImg := NewWoxImageEmoji("😀")
	img, err := emojiImg.ToImage()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
		return
	}

	path := fmt.Sprintf("%s/%s.png", util.GetLocation().GetImageCacheDirectory(), uuid.NewString())
	imaging.Save(img, path)
	t.Log(path)
}
