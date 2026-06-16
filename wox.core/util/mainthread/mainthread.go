//go:build !darwin

package mainthread

import "runtime"

type callRequest struct {
	fn   func()
	done chan struct{}
}

var funcQ = make(chan callRequest)

func init() {
	runtime.LockOSThread()
}

func Init(main func()) {
	go main()

	for f := range funcQ {
		if f.fn != nil {
			f.fn()
		}
		close(f.done)
	}
}

// Call executes f on the main thread and blocks until f returns.
// Do not use it as a fire-and-forget dispatch helper.
func Call(f func()) {
	done := make(chan struct{})
	funcQ <- callRequest{fn: f, done: done}
	<-done
}
