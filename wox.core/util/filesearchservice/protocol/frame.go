// Package protocol defines the bounded, versioned file-fact stream used by the NTFS service.
// It deliberately has no dependency on indexing, search, plugin policy, or native volume APIs.
package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	Version         uint16 = 3
	HeaderSize             = 32
	MaxPayload             = 1 << 20
	TargetBatchSize        = 256 << 10
)

type Kind uint16

const (
	Enumerate Kind = iota + 1
	Subscribe
	Replay
	Begin
	Records
	Checkpoint
	End
	ResetRequired
	Failure
)

// Frame payload is borrowed until the next Reader.Read. Sequence is monotonically
// increasing within a session; the connection owner validates the expected session.
type Frame struct {
	Kind     Kind
	Session  uint64
	Sequence uint64
	Payload  []byte
}

type Reader struct {
	source io.Reader
	buffer []byte
}

func NewReader(source io.Reader) *Reader { return &Reader{source: source} }

// Read checks the header before allocating and tolerates fragmented byte-pipe reads.
func (r *Reader) Read() (Frame, error) {
	var header [HeaderSize]byte
	if _, err := io.ReadFull(r.source, header[:]); err != nil {
		return Frame{}, err
	}
	if string(header[:4]) != "WFX3" || binary.LittleEndian.Uint16(header[4:]) != Version {
		return Frame{}, errors.New("unsupported file fact protocol")
	}
	kind := Kind(binary.LittleEndian.Uint16(header[6:]))
	size := binary.LittleEndian.Uint32(header[8:])
	if kind < Enumerate || kind > Failure || size > MaxPayload || binary.LittleEndian.Uint32(header[12:]) != 0 {
		return Frame{}, errors.New("invalid file fact frame")
	}
	if cap(r.buffer) < int(size) {
		r.buffer = make([]byte, int(size))
	}
	r.buffer = r.buffer[:int(size)]
	if _, err := io.ReadFull(r.source, r.buffer); err != nil {
		return Frame{}, err
	}
	return Frame{Kind: kind, Session: binary.LittleEndian.Uint64(header[16:]), Sequence: binary.LittleEndian.Uint64(header[24:]), Payload: r.buffer}, nil
}

// Write never assumes a pipe write accepts the whole batch, and does not buffer a stream.
func Write(destination io.Writer, frame Frame) error {
	if frame.Kind < Enumerate || frame.Kind > Failure || len(frame.Payload) > MaxPayload {
		return fmt.Errorf("invalid file fact frame")
	}
	var header [HeaderSize]byte
	copy(header[:], "WFX3")
	binary.LittleEndian.PutUint16(header[4:], Version)
	binary.LittleEndian.PutUint16(header[6:], uint16(frame.Kind))
	binary.LittleEndian.PutUint32(header[8:], uint32(len(frame.Payload)))
	binary.LittleEndian.PutUint64(header[16:], frame.Session)
	binary.LittleEndian.PutUint64(header[24:], frame.Sequence)
	if err := writeAll(destination, header[:]); err != nil {
		return err
	}
	return writeAll(destination, frame.Payload)
}

func writeAll(destination io.Writer, data []byte) error {
	for len(data) != 0 {
		n, err := destination.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(data) {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
