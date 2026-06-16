package tray

// #cgo CFLAGS: -x objective-c
// #cgo LDFLAGS: -framework Cocoa
// #include <stdlib.h>
// void createTray(const char *iconBytes, int length);
// void addMenuItem(const char *title, int tag);
// void addQueryTray(const char *iconBytes, int length, int tag, const char *tooltip, int menuTag, const char *menuTitle);
// void clearQueryTrayIcons();
// void removeTray();
import "C"
import (
	"sync"
	"unsafe"

	"wox/util/mainthread"
)

var (
	trayMu            sync.Mutex
	trayMenuFuncs     = make(map[int]func())
	trayQueryFuncs    = make(map[int]func(ClickRect))
	queryMenuTags     []int
	trayNextTag       int
	leftClickCallback func()
)

//export reportLeftClick
func reportLeftClick() {
	trayMu.Lock()
	callback := leftClickCallback
	trayMu.Unlock()

	if callback != nil {
		callback()
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

//export GoQueryTrayCallback
func GoQueryTrayCallback(tag C.int, x C.double, y C.double, width C.double, height C.double) {
	trayMu.Lock()
	callback, exists := trayQueryFuncs[int(tag)]
	trayMu.Unlock()
	if !exists || callback == nil {
		return
	}

	callback(ClickRect{
		X:      int(x),
		Y:      int(y),
		Width:  int(width),
		Height: int(height),
	})
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

func CreateTray(appIcon []byte, onClick func(), items ...MenuItem) {
	//ensure executed in main thread
	mainthread.Call(func() {
		trayMu.Lock()
		leftClickCallback = onClick
		trayMenuFuncs = make(map[int]func())
		trayQueryFuncs = make(map[int]func(ClickRect))
		queryMenuTags = nil
		trayNextTag = 0
		trayMu.Unlock()

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

	trayMu.Lock()
	trayQueryFuncs = make(map[int]func(ClickRect))
	trayMu.Unlock()
}

func SetQueryIcons(items []QueryIconItem) {
	mainthread.Call(func() {
		C.clearQueryTrayIcons()
	})

	trayMu.Lock()
	trayQueryFuncs = make(map[int]func(ClickRect))
	for _, tag := range queryMenuTags {
		delete(trayMenuFuncs, tag)
	}
	queryMenuTags = nil
	trayMu.Unlock()

	for _, item := range items {
		if len(item.Icon) == 0 || item.Callback == nil {
			continue
		}

		var tag int
		trayMu.Lock()
		tag = trayNextTag
		trayNextTag++
		trayQueryFuncs[tag] = item.Callback
		trayMu.Unlock()

		menuTag := -1
		if item.ContextMenuTitle != "" && item.ContextMenuCallback != nil {
			trayMu.Lock()
			menuTag = trayNextTag
			trayNextTag++
			trayMenuFuncs[menuTag] = item.ContextMenuCallback
			queryMenuTags = append(queryMenuTags, menuTag)
			trayMu.Unlock()
		}

		mainthread.Call(func() {
			iconBytesC := C.CBytes(item.Icon)
			defer C.free(iconBytesC)

			tooltipC := C.CString(item.Tooltip)
			defer C.free(unsafe.Pointer(tooltipC))

			menuTitleC := C.CString(item.ContextMenuTitle)
			defer C.free(unsafe.Pointer(menuTitleC))

			C.addQueryTray((*C.char)(iconBytesC), C.int(len(item.Icon)), C.int(tag), tooltipC, C.int(menuTag), menuTitleC)
		})
	}
}
