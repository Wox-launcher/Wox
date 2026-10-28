//go:build linux && cgo

package selection

import (
	"errors"
	"testing"

	"wox/util/clipboard"
)

func TestHyprlandSelectionUsesNewClipboardOffer(t *testing.T) {
	for _, tc := range []struct {
		name string
		data clipboard.Data
		want Selection
	}{
		{"text", &clipboard.TextData{Text: "repository"}, Selection{Type: SelectionTypeText, Text: "repository"}},
		{"files", &clipboard.FilePathData{FilePaths: []string{"/tmp/example"}}, Selection{Type: SelectionTypeFile, FilePaths: []string{"/tmp/example"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			triggered := false
			capture := func(trigger func() error) (clipboard.Data, error) {
				if err := trigger(); err != nil {
					return nil, err
				}
				return tc.data, nil
			}
			got, err := selectionFromHyprlandCopy(capture, func() error { triggered = true; return nil })
			if err != nil || !triggered || !selectionEquals(got, tc.want) {
				t.Fatalf("selection=%+v triggered=%v err=%v", got, triggered, err)
			}
		})
	}
}

func TestHyprlandSelectionRejectsMissingNewOffer(t *testing.T) {
	captureErr := errors.New("no new offer")
	capture := func(trigger func() error) (clipboard.Data, error) { return nil, captureErr }
	if _, err := selectionFromHyprlandCopy(capture, func() error { return nil }); !errors.Is(err, captureErr) {
		t.Fatalf("error = %v", err)
	}
}
