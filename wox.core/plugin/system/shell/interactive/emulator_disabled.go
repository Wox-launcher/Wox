//go:build !wox_ghostty || !cgo

package interactive

import "errors"

const Available = false

func newEmulator(Size) (emulator, error) {
	return nil, errors.New("interactive shell support was not built")
}
