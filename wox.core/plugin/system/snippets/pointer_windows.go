package snippets

/*
#include <stdint.h>
typedef struct SnippetPointer SnippetPointer;
SnippetPointer *snippetPointerStart(uintptr_t callback);
void snippetPointerStop(SnippetPointer *pointer);
*/
import "C"

import (
	"errors"
	"runtime/cgo"
	"sync"
)

// startPointerObserver invalidates pending replacements when the user clicks or scrolls.
func startPointerObserver(callback func()) (func(), error) {
	handle := cgo.NewHandle(callback)
	pointer := C.snippetPointerStart(C.uintptr_t(handle))
	if pointer == nil {
		handle.Delete()
		return nil, errors.New("pointer monitoring is unavailable")
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			C.snippetPointerStop(pointer)
			handle.Delete()
		})
	}, nil
}

//export woxSnippetPointerEvent
func woxSnippetPointerEvent(callback C.uintptr_t) {
	cgo.Handle(callback).Value().(func())()
}
