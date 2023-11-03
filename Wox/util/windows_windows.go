package util

import (
	"github.com/disintegration/imaging"
	"github.com/fcjr/geticon"
	"github.com/go-vgo/robotgo"
	"image"
)

func GetActiveWindowIcon() (image.Image, error) {
	activePid := robotgo.GetPid()
	icon, err := geticon.FromPid(uint32(activePid))
	if err != nil {
		return nil, err
	}

	thumbnail := imaging.Thumbnail(icon, 60, 60, imaging.Lanczos)
	return thumbnail, nil
}
