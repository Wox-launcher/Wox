package tray

// #cgo CFLAGS: -x objective-c
// #cgo LDFLAGS: -framework Cocoa
// #include <stdlib.h>
// void createTray(const char *iconBytes, int length);
// void addMenuItem(const char *title, int tag);
// void removeTray();
import "C"
import (
	"sync"
	"unsafe"

	"golang.design/x/hotkey/mainthread"
)

var (
	trayMu        sync.Mutex
	trayMenuFuncs = make(map[int]func())
	trayNextTag   int
	leftClickCallback func()
)

//export reportLeftClick
func reportLeftClick() {
	if leftClickCallback != nil {
		leftClickCallback()
	}
}

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
	dleftClickCallback = onClick
		efer C.free(unsafe.Pointer(cTitle))

	C.addMenuItem(cTitle, C.int(tag))
}

func CreateTray(appIcon []byte, onClick func(), items ...MenuItem) {
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
