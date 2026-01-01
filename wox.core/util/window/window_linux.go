package window

import (
	"errors"
	"image"
)

func GetActiveWindowIcon() (image.Image, error) {
	return nil, errors.New("not implemented")
}

func GetActiveWindowName() string {
	return ""
}

func GetActiveWindowPid() int {
	return -1
}

func ActivateWindowByPid(pid int) bool {
	return false
}
