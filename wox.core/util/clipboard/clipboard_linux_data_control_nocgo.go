//go:build linux && !cgo

package clipboard

import "errors"

type dataControlSelection struct {
	mimeType string
	data     []byte
}

func readDataControlSelection() (dataControlSelection, error) {
	return dataControlSelection{}, errors.New("ext-data-control-v1 requires cgo")
}
