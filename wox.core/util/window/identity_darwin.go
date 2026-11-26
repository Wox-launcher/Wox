//go:build darwin

package window

/*
#include <stdlib.h>

int getActiveWindowIdentity(unsigned int *windowID, char **name);
*/
import "C"

import (
	"context"
	"strconv"
	"unsafe"
)

// GetActiveWindowIdentity captures the window target before Wox takes focus.
func GetActiveWindowIdentity(_ context.Context) (pid int, id string, name string) {
	var windowID C.uint
	var appName *C.char
	pid = int(C.getActiveWindowIdentity(&windowID, &appName))
	if windowID != 0 {
		id = strconv.FormatUint(uint64(windowID), 10)
	}
	if appName != nil {
		defer C.free(unsafe.Pointer(appName))
		name = C.GoString(appName)
	}
	return pid, id, name
}
