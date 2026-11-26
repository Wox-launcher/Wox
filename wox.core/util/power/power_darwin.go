package power

/*
#cgo LDFLAGS: -framework AppKit
void *woxPowerWatch(void);
void woxPowerUnwatch(void *observer);
*/
import "C"

import (
	"context"
	"sync/atomic"
)

var darwinWake atomic.Pointer[func()]

func startPlatformWatcher(ctx context.Context, notify func()) {
	darwinWake.Store(&notify)
	observer := C.woxPowerWatch()
	context.AfterFunc(ctx, func() {
		darwinWake.CompareAndSwap(&notify, nil)
		C.woxPowerUnwatch(observer)
	})
}

//export woxPowerDidWake
func woxPowerDidWake() {
	if notify := darwinWake.Load(); notify != nil {
		(*notify)()
	}
}
