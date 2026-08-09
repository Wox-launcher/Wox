//go:build wox_ui_smoke && darwin

package hotkey

/*
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
#include <stdint.h>

int activateApplicationForSmoke(int pid);
int terminateApplicationForSmoke(int pid);
int frontmostApplicationPidForSmoke(void);
int postKeyboardChordForSmoke(uint16_t modifierKeyCode, uint64_t flags, uint16_t keyCode);
*/
import "C"

func activateDarwinApplication(pid int) bool {
	return C.activateApplicationForSmoke(C.int(pid)) != 0
}

func terminateDarwinApplication(pid int) bool {
	return C.terminateApplicationForSmoke(C.int(pid)) != 0
}

func frontmostDarwinApplicationPID() int {
	return int(C.frontmostApplicationPidForSmoke())
}

func postDarwinKeyboardChord(modifierKeyCode uint16, flags uint64, keyCode uint16) bool {
	return C.postKeyboardChordForSmoke(C.uint16_t(modifierKeyCode), C.uint64_t(flags), C.uint16_t(keyCode)) != 0
}
