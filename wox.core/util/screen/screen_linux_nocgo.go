//go:build linux && !cgo

package screen

import "errors"

var errLinuxScreenCGODisabled = errors.New("linux screen enumeration requires cgo")

func GetMouseScreen() Size {
	return Size{}
}

func GetActiveScreen() Size {
	return Size{}
}

func listDisplays() ([]Display, error) {
	return nil, errLinuxScreenCGODisabled
}

func GetX11PointerPosition() (int, int, error) {
	return 0, 0, errLinuxScreenCGODisabled
}
