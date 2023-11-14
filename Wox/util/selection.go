package util

import (
	"errors"
	"wox/util/clipboard"
)

func GetSelected() (clipboard.Data, error) {
	if SimulateCtrlC() != nil {
		return nil, errors.New("error simulate ctrl c")
	}

	return clipboard.Read()
}
