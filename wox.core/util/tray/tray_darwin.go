package tray

// #cgo CFLAGS: -x objective-c
// #cgo LDFLAGS: -framework Cocoa -framework ImageIO
// #include <stdlib.h>
// void createTray(const char *iconBytes, int length);
// void addMenuItem(const char *title, int tag);
// int addQueryTray(const char *iconBytes, int length, int tag, const char *identifier, const char *tooltip, int menuTag, const char *menuTitle);
// void clearQueryTrayIcons();
// void removeTray();
import "C"
import (
	"context"
	"fmt"
	"sync"
	"unsafe"

	"wox/util"
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

//export reportQueryTrayFallback
func reportQueryTrayFallback(tag C.int) {
	util.GetLogger().Info(context.Background(), fmt.Sprintf("macOS placed query tray icon outside the reliably renderable menu bar area; reused the main tray slot: tag=%d", int(tag)))
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
			identifierC := C.CString(item.Identifier)
			defer C.free(unsafe.Pointer(identifierC))

			tooltipC := C.CString(item.Tooltip)
			defer C.free(unsafe.Pointer(tooltipC))

			menuTitleC := C.CString(item.ContextMenuTitle)
			defer C.free(unsafe.Pointer(menuTitleC))

			iconBytesC := C.CBytes(item.Icon)
			defer C.free(iconBytesC)

			result := C.addQueryTray((*C.char)(iconBytesC), C.int(len(item.Icon)), C.int(tag), identifierC, tooltipC, C.int(menuTag), menuTitleC)
			if result <= 0 {
				util.GetLogger().Warn(context.Background(), fmt.Sprintf("failed to create macOS tray query icon: tag=%d iconBytes=%d nativeResult=%d", tag, len(item.Icon), int(result)))
			}
		})
	}
}
