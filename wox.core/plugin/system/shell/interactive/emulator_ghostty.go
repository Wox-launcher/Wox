//go:build wox_ghostty && cgo

package interactive

/*
#cgo CFLAGS: -DGHOSTTY_STATIC
#cgo windows CFLAGS: -I${SRCDIR}/.build/ghostty-1.0.0-windows-amd64/include
#cgo linux CFLAGS: -I${SRCDIR}/.build/ghostty-1.0.0-linux-amd64/include
#cgo darwin,amd64 CFLAGS: -I${SRCDIR}/.build/ghostty-1.0.0-darwin-amd64/include
#cgo darwin,arm64 CFLAGS: -I${SRCDIR}/.build/ghostty-1.0.0-darwin-arm64/include
#cgo windows LDFLAGS: ${SRCDIR}/.build/ghostty-1.0.0-windows-amd64/woxghostty.a -lntdll
#cgo linux LDFLAGS: ${SRCDIR}/.build/ghostty-1.0.0-linux-amd64/woxghostty.a -lm
#cgo darwin,amd64 LDFLAGS: ${SRCDIR}/.build/ghostty-1.0.0-darwin-amd64/woxghostty.a
#cgo darwin,arm64 LDFLAGS: ${SRCDIR}/.build/ghostty-1.0.0-darwin-arm64/woxghostty.a
#include "ghostty.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"
	"unsafe"
)

const Available = true

type ghosttyEmulator struct {
	model *C.WoxGhostty
	cells []C.WoxGhosttyCell
}

// newEmulator checks the staged library before allocating its handles.
func newEmulator(size Size) (emulator, error) {
	if version := C.GoString(C.ghostty_version()); version != "1.0.0" {
		return nil, fmt.Errorf("unexpected libghostty-vt version %s", version)
	}
	model := C.wox_ghostty_new(C.uint16_t(size.Columns), C.uint16_t(size.Rows))
	if model == nil {
		return nil, errors.New("create Ghostty terminal")
	}
	e := &ghosttyEmulator{model: model}
	if err := e.resize(size); err != nil {
		e.close()
		return nil, err
	}
	return e, nil
}

func (e *ghosttyEmulator) close()          { C.wox_ghostty_free(e.model) }
func (e *ghosttyEmulator) scroll(rows int) { C.wox_ghostty_scroll(e.model, C.int(rows)) }
func (e *ghosttyEmulator) scrollBottom()   { C.wox_ghostty_scroll_bottom(e.model) }
func (e *ghosttyEmulator) beginCommand()   { C.wox_ghostty_begin_command(e.model) }

// commandOutput copies a bounded, valid UTF-8 tail of the rendered command output.
func (e *ghosttyEmulator) commandOutput() string {
	var length C.size_t
	output := C.wox_ghostty_command_output(e.model, &length)
	if output == nil {
		return ""
	}
	defer C.ghostty_free(nil, output, length)
	data := unsafe.Slice((*byte)(output), int(length))
	start := max(0, len(data)-64*1024)
	for start < len(data) && !utf8.RuneStart(data[start]) {
		start++
	}
	return string(data[start:])
}

func (e *ghosttyEmulator) resize(size Size) error {
	result := C.wox_ghostty_resize(e.model, C.uint16_t(size.Columns), C.uint16_t(size.Rows), C.uint32_t(size.CellWidthPixels), C.uint32_t(size.CellHeightPixels))
	if result != C.GHOSTTY_SUCCESS {
		return fmt.Errorf("Ghostty resize: %d", result)
	}
	return nil
}

func (e *ghosttyEmulator) write(bytes []byte) []byte {
	if len(bytes) == 0 {
		return nil
	}
	var length C.size_t
	reply := C.wox_ghostty_write(e.model, (*C.uint8_t)(&bytes[0]), C.size_t(len(bytes)), &length)
	if length == 0 {
		return nil
	}
	return C.GoBytes(unsafe.Pointer(reply), C.int(length))
}

func (e *ghosttyEmulator) paste(text string) ([]byte, bool) {
	if len(text) == 0 {
		return nil, true
	}
	data := []byte(text)
	output := make([]byte, len(data)+16)
	length := C.wox_ghostty_paste(e.model, (*C.char)(unsafe.Pointer(&data[0])), C.size_t(len(data)), (*C.uint8_t)(&output[0]), C.size_t(len(output)))
	if length == ^C.size_t(0) {
		return nil, false
	}
	return output[:int(length)], true
}

func (e *ghosttyEmulator) key(event Key) []byte {
	key, codepoint := ghosttyKey(event.Name)
	if key == 0 {
		return nil
	}
	var mods C.GhosttyMods
	if event.Shift {
		mods |= C.GHOSTTY_MODS_SHIFT
	}
	if event.Ctrl {
		mods |= C.GHOSTTY_MODS_CTRL
	}
	if event.Alt {
		mods |= C.GHOSTTY_MODS_ALT
	}
	var output [64]byte
	length := C.wox_ghostty_key(e.model, key, codepoint, mods, C.bool(event.Repeat), (*C.uint8_t)(&output[0]), C.size_t(len(output)))
	return output[:int(length)]
}

// ghosttyKey maps Wox's semantic key names to Ghostty's physical key identifiers.
func ghosttyKey(name string) (C.GhosttyKey, C.uint32_t) {
	if len(name) == 1 {
		char := name[0]
		if char >= 'a' && char <= 'z' {
			return C.GhosttyKey(C.GHOSTTY_KEY_A + int(char-'a')), C.uint32_t(char)
		}
		if char >= '0' && char <= '9' {
			return C.GhosttyKey(C.GHOSTTY_KEY_DIGIT_0 + int(char-'0')), C.uint32_t(char)
		}
		return ghosttyKeys[name], C.uint32_t(char)
	}
	if name == "space" {
		return C.GHOSTTY_KEY_SPACE, ' '
	}
	if len(name) >= 2 && name[0] == 'f' {
		if number, err := strconv.Atoi(name[1:]); err == nil && number >= 1 && number <= 12 {
			return C.GhosttyKey(C.GHOSTTY_KEY_F1 + number - 1), 0
		}
	}
	return ghosttyKeys[name], 0
}

var ghosttyKeys = map[string]C.GhosttyKey{
	"backspace": C.GHOSTTY_KEY_BACKSPACE, "tab": C.GHOSTTY_KEY_TAB,
	"enter": C.GHOSTTY_KEY_ENTER, "escape": C.GHOSTTY_KEY_ESCAPE, "space": C.GHOSTTY_KEY_SPACE,
	"page-up": C.GHOSTTY_KEY_PAGE_UP, "page-down": C.GHOSTTY_KEY_PAGE_DOWN,
	"home": C.GHOSTTY_KEY_HOME, "end": C.GHOSTTY_KEY_END,
	"arrow-up": C.GHOSTTY_KEY_ARROW_UP, "arrow-down": C.GHOSTTY_KEY_ARROW_DOWN,
	"arrow-left": C.GHOSTTY_KEY_ARROW_LEFT, "arrow-right": C.GHOSTTY_KEY_ARROW_RIGHT,
	"insert": C.GHOSTTY_KEY_INSERT, "delete": C.GHOSTTY_KEY_DELETE,
	"[": C.GHOSTTY_KEY_BRACKET_LEFT, "]": C.GHOSTTY_KEY_BRACKET_RIGHT, "\\": C.GHOSTTY_KEY_BACKSLASH,
	"-": C.GHOSTTY_KEY_MINUS, "/": C.GHOSTTY_KEY_SLASH, ".": C.GHOSTTY_KEY_PERIOD,
	",": C.GHOSTTY_KEY_COMMA, ";": C.GHOSTTY_KEY_SEMICOLON, "=": C.GHOSTTY_KEY_EQUAL,
}

// snapshot copies C bridge data into an immutable Go frame before handing it to the launcher.
func (e *ghosttyEmulator) snapshot(size Size) Frame {
	count := size.Columns * size.Rows
	if cap(e.cells) < count {
		e.cells = make([]C.WoxGhosttyCell, count)
	}
	e.cells = e.cells[:count]
	cursor := C.GhosttyRenderStateCursor{size: C.sizeof_GhosttyRenderStateCursor}
	frame := Frame{Size: size}
	if !bool(C.wox_ghostty_frame(e.model, &e.cells[0], C.uint16_t(size.Columns), C.uint16_t(size.Rows), &cursor)) {
		return frame
	}
	frame.CursorColumn, frame.CursorRow = int(cursor.viewport_x), int(cursor.viewport_y)
	frame.CursorVisible = bool(cursor.visible) && bool(cursor.viewport_has_value)
	frame.Cells = make([]Cell, count)
	for i, cell := range e.cells {
		frame.Cells[i] = Cell{
			Text:       C.GoStringN((*C.char)(unsafe.Pointer(&e.cells[i].text[0])), C.int(cell.length)),
			Foreground: uint32(cell.fg), Background: uint32(cell.bg),
			Wide:         cell.width == C.GHOSTTY_CELL_WIDE_WIDE,
			Continuation: cell.width == C.GHOSTTY_CELL_WIDE_SPACER_TAIL || cell.width == C.GHOSTTY_CELL_WIDE_SPACER_HEAD,
			Bold:         cell.style&C.GHOSTTY_STYLE_FLAG_BOLD != 0, Italic: cell.style&C.GHOSTTY_STYLE_FLAG_ITALIC != 0,
			Inverse:   cell.style&C.GHOSTTY_STYLE_FLAG_INVERSE != 0,
			Underline: cell.style&C.GHOSTTY_STYLE_UNDERLINE_MASK != 0,
		}
	}
	return frame
}
