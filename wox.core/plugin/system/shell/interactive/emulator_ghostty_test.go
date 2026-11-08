//go:build wox_ghostty && cgo

package interactive

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

var testSize = Size{Columns: 80, Rows: 12, CellWidthPixels: 9, CellHeightPixels: 18}

func TestGhosttyFrameAndKeyEncoding(t *testing.T) {
	model, err := newEmulator(testSize)
	if err != nil {
		t.Fatal(err)
	}
	defer model.close()
	reply := model.write([]byte("\x1b[31m中\x1b[0m\x1b[6n"))
	frame := model.snapshot(testSize)
	if len(frame.Cells) < 2 || frame.Cells[0].Text != "中" || !frame.Cells[0].Wide || !frame.Cells[1].Continuation || frame.Cells[0].Foreground == 0 {
		t.Fatalf("colored wide cell was not rendered: %+v", frame.Cells[:min(2, len(frame.Cells))])
	}
	if string(reply) != "\x1b[1;3R" {
		t.Fatalf("cursor reply = %q", reply)
	}
	if got := string(model.key(Key{Name: "c", Ctrl: true})); got != "\x03" {
		t.Fatalf("Ctrl+C = %q", got)
	}
	if data, safe := model.paste("first\nsecond"); safe || len(data) != 0 {
		t.Fatal("an unbracketed multiline paste would submit a command")
	}
	model.write([]byte("\x1b[?1049h\x1b[H\x1b[?2004hALT"))
	if data, safe := model.paste("hello\nworld"); !safe || string(data) != "\x1b[200~hello\nworld\x1b[201~" {
		t.Fatalf("paste = %q", data)
	}
	if frame := model.snapshot(testSize); frame.Cells[0].Text != "A" {
		t.Fatalf("alternate screen = %+v", frame.Cells[0])
	}
}

// TestInteractivePTY checks that the actual platform shell accepts commands and delivers its last screen.
func TestInteractivePTY(t *testing.T) {
	interpreter := "sh"
	command := "printf '%s%s\\n' WOX_SHELL_ READY; exit"
	if runtime.GOOS == "windows" {
		interpreter = "cmd"
		command = "echo WOX_SHELL_^READY & exit"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := Start(ctx, Options{Interpreter: interpreter, Directory: t.TempDir()}, testSize)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if !session.Text(command) || !session.Key(Key{Name: "enter"}) {
		t.Fatal("shell did not accept input")
	}
	var screen string
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("shell did not exit: %q", screen)
		case _, ok := <-session.Changes():
			frame := session.Frame()
			if frame != nil {
				var builder strings.Builder
				for _, cell := range frame.Cells {
					builder.WriteString(cell.Text)
				}
				screen = builder.String()
				if frame.Exited {
					if frame.Err != nil || !strings.Contains(screen, "WOX_SHELL_READY") {
						t.Fatalf("shell output = %q, exit error = %v", screen, frame.Err)
					}
					return
				}
			}
			if !ok {
				t.Fatalf("session closed without an exit frame: %q", screen)
			}
		}
	}
}

// TestInteractivePTYResizeAndClose exercises native resize and cancellation with a live shell.
func TestInteractivePTYResizeAndClose(t *testing.T) {
	interpreter := "sh"
	if runtime.GOOS == "windows" {
		interpreter = "cmd"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := Start(ctx, Options{Interpreter: interpreter, Directory: t.TempDir()}, testSize)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	size := Size{Columns: 51, Rows: 20, CellWidthPixels: 13, CellHeightPixels: 24}
	session.Resize(size)
	for session.Frame() == nil || session.Frame().Size != size {
		select {
		case <-ctx.Done():
			t.Fatal("shell resize timed out")
		case _, ok := <-session.Changes():
			if !ok {
				t.Fatalf("shell exited before resize: %+v", session.Frame())
			}
		}
	}
	session.Close()
	select {
	case <-session.Done():
	case <-ctx.Done():
		t.Fatal("shell teardown timed out")
	}
}
