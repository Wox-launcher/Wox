package interactive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"
)

const (
	OpenActionID = "open-interactive-shell"
	MaxColumns   = 240
	MaxRows      = 100
	MaxPaste     = 64 * 1024
)

type Options struct {
	Interpreter string
	Directory   string
}

type Size struct {
	Columns, Rows                     int
	CellWidthPixels, CellHeightPixels int
}

type Cell struct {
	Text                   string
	Foreground, Background uint32 // RGBA, with zero denoting the theme default.
	Wide, Continuation     bool
	Bold, Italic, Inverse  bool
	Underline              bool
}

// Frame is an immutable, row-major snapshot. It remains readable after the process exits.
type Frame struct {
	Size                    Size
	Cells                   []Cell
	CursorColumn, CursorRow int
	CursorVisible           bool
	PasteBlocked            bool
	Exited                  bool
	Err                     error
}

type Key struct {
	Name             string
	Shift, Ctrl, Alt bool
	Repeat           bool
}

type input struct {
	text   string
	key    Key
	paste  bool
	scroll int
}

type transport struct {
	output *os.File
	input  io.WriteCloser
	resize func(Size) error
	wait   func() error
	stop   func(canceled bool)
}

type emulator interface {
	close()
	write([]byte) []byte
	key(Key) []byte
	paste(string) ([]byte, bool)
	resize(Size) error
	scroll(int)
	snapshot(Size) Frame
}

// Session owns one PTY and emulator. The UI exchanges immutable frames and ordered input.
type Session struct {
	cancel  context.CancelFunc
	ctx     context.Context
	input   chan input
	sizes   atomic.Pointer[Size]
	resized chan struct{}
	frame   atomic.Pointer[Frame]
	changes chan struct{}
	done    chan struct{}
}

// Start creates a fresh shell after the first viewport size is known.
func Start(ctx context.Context, options Options, size Size) (*Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !size.valid() {
		return nil, errors.New("invalid shell viewport size")
	}
	model, err := newEmulator(size)
	if err != nil {
		return nil, err
	}
	pty, err := openPTY(options, size)
	if err != nil {
		model.close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	s := &Session{ctx: ctx, cancel: cancel, input: make(chan input, 64), resized: make(chan struct{}, 1), changes: make(chan struct{}, 1), done: make(chan struct{})}
	go s.run(pty, model, size)
	return s, nil
}

func (s *Session) Close()                   { s.cancel() }
func (s *Session) Frame() *Frame            { return s.frame.Load() }
func (s *Session) Changes() <-chan struct{} { return s.changes }
func (s *Session) Done() <-chan struct{}    { return s.done }

func (s *Session) Text(text string) bool { return s.send(input{text: text}) }
func (s *Session) Key(key Key) bool      { return s.send(input{key: key}) }
func (s *Session) Paste(text string) bool {
	return len(text) <= MaxPaste && s.send(input{text: text, paste: true})
}
func (s *Session) Scroll(rows int) bool { return s.send(input{scroll: rows}) }

// Resize replaces the pending size, keeping native layout changes off the UI event loop.
func (s *Session) Resize(size Size) {
	if !size.valid() {
		return
	}
	s.sizes.Store(&size)
	select {
	case s.resized <- struct{}{}:
	default:
	}
}

func (size Size) valid() bool {
	return size.Columns >= 1 && size.Columns <= MaxColumns && size.Rows >= 1 && size.Rows <= MaxRows &&
		size.CellWidthPixels >= 1 && size.CellWidthPixels <= 65535/size.Columns &&
		size.CellHeightPixels >= 1 && size.CellHeightPixels <= 65535/size.Rows
}

func (s *Session) send(value input) bool {
	select {
	case <-s.ctx.Done():
		return false
	default:
	}
	select {
	case s.input <- value:
		return true
	default:
		return false
	}
}

func (s *Session) publish(frame Frame) {
	s.frame.Store(&frame)
	select {
	case s.changes <- struct{}{}:
	default:
	}
}

// run serializes libghostty calls. PTY reads and writes progress independently so a full pipe
// cannot deadlock a child waiting for its input or a ConPTY waiting to flush its final frame.
func (s *Session) run(pty transport, model emulator, size Size) {
	defer close(s.done)
	defer close(s.changes)
	defer model.close()

	output := make(chan []byte, 8)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		defer close(output)
		defer pty.output.Close()
		var buffer [16 * 1024]byte
		for {
			count, err := pty.output.Read(buffer[:])
			if count > 0 {
				select {
				case output <- append([]byte(nil), buffer[:count]...):
				case <-s.ctx.Done():
				}
			}
			if err != nil {
				return
			}
		}
	}()

	writes := make(chan []byte, 64)
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		for {
			select {
			case <-s.ctx.Done():
				return
			case data := <-writes:
				if _, err := pty.input.Write(data); err != nil {
					return
				}
			}
		}
	}()

	exited := make(chan error, 1)
	go func() { exited <- pty.wait() }()
	result := make(chan error, 1)
	cleaned := make(chan struct{})
	go func() {
		defer close(cleaned)
		var err error
		select {
		case err = <-exited:
			pty.stop(false)
		case <-s.ctx.Done():
			pty.stop(true)
			err = <-exited
		}
		<-readDone
		_ = pty.input.Close()
		result <- err
	}()
	defer func() {
		s.cancel()
		<-cleaned
		<-writeDone
	}()

	write := func(data []byte) bool {
		if len(data) == 0 {
			return true
		}
		select {
		case writes <- data:
			return true
		default:
			// Do not silently lose a control sequence or half of a user command.
			s.cancel()
			frame := model.snapshot(size)
			frame.Exited, frame.Err = true, errors.New("shell input queue is full")
			s.publish(frame)
			return false
		}
	}

	s.publish(model.snapshot(size))
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()
	dirty := false
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if dirty {
				s.publish(model.snapshot(size))
				dirty = false
			}
		case bytes, ok := <-output:
			if !ok {
				frame := model.snapshot(size)
				frame.Exited = true
				frame.Err = <-result
				s.publish(frame)
				return
			}
			if !write(model.write(bytes)) {
				return
			}
			dirty = true
		case <-s.resized:
			next := *s.sizes.Load()
			if next != size {
				err := pty.resize(next)
				if err == nil {
					err = model.resize(next)
				}
				if err != nil {
					frame := model.snapshot(size)
					frame.Exited, frame.Err = true, fmt.Errorf("resize shell viewport: %w", err)
					s.publish(frame)
					return
				}
				size, dirty = next, true
			}
		case value := <-s.input:
			var data []byte
			switch {
			case value.scroll != 0:
				model.scroll(value.scroll)
				dirty = true
			case value.paste:
				var safe bool
				data, safe = model.paste(value.text)
				if !safe {
					frame := model.snapshot(size)
					frame.PasteBlocked = true
					s.publish(frame)
					dirty = false
				}
			case value.key.Name != "":
				data = model.key(value.key)
			default:
				data = []byte(value.text)
			}
			if !write(data) {
				return
			}
		}
	}
}
