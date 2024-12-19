package tray

// #cgo CFLAGS: -x objective-c
// #cgo LDFLAGS: -framework Cocoa
// #include <stdlib.h>
// void createTray(const char *iconBytes, int length);
// void addMenuItem(const char *title, int tag);
// void removeTray();
import "C"
import (
	"golang.design/x/hotkey/mainthread"
	"sync"
	"unsafe"
)

var (
	trayMu        sync.Mutex
	trayMenuFuncs = make(map[int]func())
	trayNextTag   int
)

//export GoMenuItemCallback
func GoMenuItemCallback(tag C.int) {
	trayMu.Lock()
	defer trayMu.Unlock()

	if fn, exists := trayMenuFuncs[int(tag)]; exists {
		fn()
	}
}

func addMenuItem(title string, callback func()) {
	trayMu.Lock()
	defer trayMu.Unlock()

	tag := trayNextTag
	trayNextTag++

	trayMenuFuncs[tag] = callback

	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))

	C.addMenuItem(cTitle, C.int(tag))
}

func CreateTray(appIcon []byte, items ...MenuItem) {
	//ensure executed in main thread
	mainthread.Call(func() {
		iconBytesC := C.CBytes(appIcon)
		defer C.free(iconBytesC)

		C.createTray((*C.char)(iconBytesC), C.int(len(appIcon)))

		for _, item := range items {
			addMenuItem(item.Title, item.Callback)
		}
	})
}

func RemoveTray() {
	//ensure executed in main thread
	mainthread.Call(func() {
		C.removeTray()
	})
}
