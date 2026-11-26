//go:build linux && cgo

package selection

import "wox/util/clipboard"

func captureWaylandSelection(trigger func() error) (Selection, error) {
	return selectionFromWaylandCopy(clipboard.CaptureNextWaylandCopy, trigger)
}

// selectionFromWaylandCopy converts only the newly observed clipboard offer.
func selectionFromWaylandCopy(capture func(func() error) (clipboard.Data, error), trigger func() error) (Selection, error) {
	data, err := capture(trigger)
	if err != nil {
		return Selection{}, err
	}
	switch selected := data.(type) {
	case *clipboard.TextData:
		return Selection{Type: SelectionTypeText, Text: selected.Text}, nil
	case *clipboard.FilePathData:
		return Selection{Type: SelectionTypeFile, FilePaths: selected.FilePaths}, nil
	default:
		return Selection{}, noSelection
	}
}
