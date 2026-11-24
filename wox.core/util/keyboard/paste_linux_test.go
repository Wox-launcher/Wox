package keyboard

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"slices"
	"testing"
)

func TestLinuxPasteChord(t *testing.T) {
	for _, tc := range []struct {
		name  string
		shift bool
		want  []uint16
	}{
		{name: "standard", want: []uint16{29, 47, 0, 47, 29, 0}},
		{name: "terminal", shift: true, want: []uint16{29, 42, 47, 0, 47, 42, 29, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			defer writer.Close()
			if err := writeCtrlKeyCombo(int(writer.Fd()), evKeyCodeV, tc.shift, nil); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(reader)
			if err != nil || len(data) != len(tc.want)*24 {
				t.Fatalf("read %d event bytes, err=%v", len(data), err)
			}
			var keys []uint16
			value := uint32(evKeyDownValue)
			for event := range slices.Chunk(data, 24) {
				key := binary.LittleEndian.Uint16(event[18:20])
				keys = append(keys, key)
				if key == evSynReportCode {
					value = evKeyUpValue
				} else if got := binary.LittleEndian.Uint32(event[20:24]); got != value {
					t.Fatalf("key %d has value %d, want %d", key, got, value)
				}
			}
			if !slices.Equal(keys, tc.want) {
				t.Fatalf("key sequence %v, want %v", keys, tc.want)
			}
		})
	}
}

func TestLinuxPasteSkipsInputWhenFocusFails(t *testing.T) {
	focusErr := errors.New("wrong window")
	// An invalid FD proves the focus error is returned before any write.
	err := writeCtrlKeyCombo(-1, evKeyCodeV, true, func(int) error { return focusErr })
	if !errors.Is(err, focusErr) {
		t.Fatalf("expected focus error, got %v", err)
	}
}
