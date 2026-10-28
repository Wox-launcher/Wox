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

// CaptureNextWaylandCopy requires the native data-control client.
func CaptureNextWaylandCopy(trigger func() error) (Data, error) {
	return nil, errors.New("ext-data-control-v1 requires cgo")
}
