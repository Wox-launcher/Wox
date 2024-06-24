package window

import "errors"
import "image"

func GetActiveWindowIcon() (image.Image, error) {
	return nil, errors.New("not implemented")
}

func GetActiveWindowName() string {
	return ""
}

func GetActiveWindowPid() int {
	return -1
}
