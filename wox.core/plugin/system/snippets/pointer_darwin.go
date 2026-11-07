package snippets

/*
#include <stdint.h>
typedef struct SnippetMacPointer SnippetMacPointer;
SnippetMacPointer *snippetMacPointerStart(uintptr_t callback);
void snippetMacPointerStop(SnippetMacPointer *pointer);
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
	pointer := C.snippetMacPointerStart(C.uintptr_t(handle))
	if pointer == nil {
		handle.Delete()
		return nil, errors.New("pointer monitoring is unavailable")
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			C.snippetMacPointerStop(pointer)
			handle.Delete()
		})
	}, nil
}

//export woxSnippetMacPointerEvent
func woxSnippetMacPointerEvent(callback C.uintptr_t) {
	cgo.Handle(callback).Value().(func())()
}
