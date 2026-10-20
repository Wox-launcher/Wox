//go:build windows || darwin || linux

package window

/*
#include <stdint.h>
*/
import "C"

import (
	"encoding/json"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var scriptCallbacks sync.Map
var scriptCallbackID atomic.Uint64

// registerScriptCallback uses numeric handles so native callbacks never retain Go pointers.
func registerScriptCallback(done func(string, error)) uint64 {
	id := scriptCallbackID.Add(1)
	scriptCallbacks.Store(id, done)
	time.AfterFunc(30*time.Second, func() { finishScriptCallback(id, "", errors.New("native script evaluation timed out")) })
	return id
}

func finishScriptCallback(id uint64, value string, err error) {
	if callback, ok := scriptCallbacks.LoadAndDelete(id); ok {
		callback.(func(string, error))(value, err)
	}
}

//export woxGoWebViewScriptResult
func woxGoWebViewScriptResult(id C.uint64_t, value *C.char, failure *C.char) {
	var err error
	if failure != nil {
		err = errors.New(C.GoString(failure))
	}
	result := C.GoString(value)
	if runtime.GOOS == "windows" && err == nil {
		var decoded string
		if json.Unmarshal([]byte(result), &decoded) == nil {
			result = decoded
		}
	}
	finishScriptCallback(uint64(id), result, err)
}
