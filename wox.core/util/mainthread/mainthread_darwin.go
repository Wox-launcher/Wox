//go:build darwin

package mainthread

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>
#import <Dispatch/Dispatch.h>

extern void os_main(void);
extern void wakeupMainThread(void);
extern void dispatchMainFuncs(void);

static bool isMainThread() {
	return [NSThread isMainThread];
}
*/
import "C"

import (
	"os"
	"runtime"
)

type callRequest struct {
	fn   func()
	done chan struct{}
}

var funcQ = make(chan callRequest, 64)

func init() {
	runtime.LockOSThread()
}

func Init(main func()) {
	go func() {
		main()
		os.Exit(0)
	}()

	C.os_main()
}

// Call executes f on the Cocoa main thread and blocks until f returns.
// Do not use it as a fire-and-forget dispatch helper.
func Call(f func()) {
	if f == nil {
		return
	}

	if C.isMainThread() {
		f()
		return
	}

	done := make(chan struct{})
	funcQ <- callRequest{fn: f, done: done}
	C.wakeupMainThread()
	<-done
}

//export dispatchMainFuncs
func dispatchMainFuncs() {
	for {
		select {
		case req := <-funcQ:
			if req.fn != nil {
				req.fn()
			}
			if req.done != nil {
				close(req.done)
			}
		default:
			return
		}
	}
}
