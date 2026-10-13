package protocol

import (
	"bytes"
	"encoding/binary"
	"io"
	"reflect"
	"testing"
	"testing/iotest"
)

// TestFragmentedRoundTrip exercises a byte pipe that splits every frame arbitrarily.
func TestFragmentedRoundTrip(t *testing.T) {
	entries := []Entry{{ID: 12, ParentID: 5, USN: 100, Flags: SizeKnown, Size: 42, Name: "项目 report.txt"}, {ID: 13, ParentID: 5, USN: 101, Flags: Deleted}}
	var payload []byte
	for _, entry := range entries {
		var err error
		payload, err = AppendEntry(payload, entry)
		if err != nil {
			t.Fatal(err)
		}
	}
	var stream bytes.Buffer
	if err := Write(&stream, Frame{Kind: Records, Session: 4, Sequence: 9, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	reader := NewReader(iotest.OneByteReader(&stream))
	frame, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if frame.Session != 4 || frame.Sequence != 9 || frame.Kind != Records {
		t.Fatalf("frame: %+v", frame)
	}
	var got []Entry
	if err := VisitEntries(frame.Payload, func(entry Entry) error { got = append(got, entry); return nil }); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, entries) {
		t.Fatalf("entries: %+v", got)
	}
	if _, err := reader.Read(); err != io.EOF {
		t.Fatalf("stream end: %v", err)
	}
}

// TestFrameBounds rejects untrusted lengths before a body buffer is allocated.
func TestFrameBounds(t *testing.T) {
	var valid bytes.Buffer
	if err := Write(&valid, Frame{Kind: End}); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func([]byte){
		func(b []byte) { binary.LittleEndian.PutUint32(b[8:], MaxPayload+1) },
		func(b []byte) { binary.LittleEndian.PutUint16(b[4:], Version+1) },
		func(b []byte) { b[12] = 1 },
		func(b []byte) { binary.LittleEndian.PutUint16(b[6:], 65535) },
	} {
		data := bytes.Clone(valid.Bytes())
		mutate(data)
		reader := NewReader(bytes.NewReader(data))
		if _, err := reader.Read(); err == nil || cap(reader.buffer) != 0 {
			t.Fatal("invalid header allocated or decoded")
		}
	}
}

type stalledWriter struct{}

func (stalledWriter) Write([]byte) (int, error) { return 0, nil }

func TestStalledWriterFails(t *testing.T) {
	if err := Write(stalledWriter{}, Frame{Kind: End}); err != io.ErrShortWrite {
		t.Fatalf("stalled write: %v", err)
	}
}

func TestBoundaryRoundTrip(t *testing.T) {
	want := Boundary{Volume: `C:\`, RootID: 5, JournalID: 99, NextUSN: 123}
	data, err := EncodeBoundary(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeBoundary(data)
	if err != nil || got != want {
		t.Fatalf("boundary: %+v %v", got, err)
	}
}

// FuzzRecords verifies all sizes are bounded even when payloads bypass the frame reader.
func FuzzRecords(f *testing.F) {
	data, _ := AppendEntry(nil, Entry{ID: 2, ParentID: 1, Name: "test.txt"})
	f.Add(data)
	f.Add([]byte{0, 1, 2})
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = VisitEntries(data, func(entry Entry) error {
			encoded, err := AppendEntry(nil, entry)
			if err != nil {
				t.Fatal(err)
			}
			if len(encoded) > MaxPayload {
				t.Fatal("oversized accepted record")
			}
			return nil
		})
	})
}
