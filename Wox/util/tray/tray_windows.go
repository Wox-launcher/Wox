package tray

/*
#cgo LDFLAGS: -lshell32
#include <windows.h>

extern void init(HICON icon);
extern void addMenuItem(unsigned int id, char *title);
extern void removeTrayIcon();
extern void showMenu();
extern unsigned int nextMenuId;
*/
import "C"
import (
	"unsafe"
)

// menuCallbacks maps menu item IDs to their callbacks.
var menuCallbacks = make(map[uint32]func())

//export reportClick
func reportClick(menuId C.UINT_PTR) {
	if callback, exists := menuCallbacks[uint32(menuId)]; exists {
		callback()
	}
}

// initializes the system tray icon and menu.
func CreateTray(appIcon []byte, items ...MenuItem) {
	icon := C.CreateIconFromResourceEx((*C.BYTE)(unsafe.Pointer(&appIcon[0])), C.DWORD(len(appIcon)), C.TRUE, 0x30000, 32, 32, C.LR_DEFAULTCOLOR)
	C.init(icon)

	for _, item := range items {
		title := C.CString(item.Title)
		defer C.free(unsafe.Pointer(title))
		menuId := C.nextMenuId
		C.addMenuItem(menuId, title)
		menuCallbacks[uint32(menuId)] = item.Callback
		C.nextMenuId++
	}
	//
	//go func() {
	//	time.Sleep(time.Second)
	//	mainthread.Call(func() {
	//		var msg C.MSG
	//		for C.GetMessage(&msg, nil, 0, 0) > 0 {
	//			C.TranslateMessage(&msg)
	//			C.DispatchMessage(&msg)
	//		}
	//	})
	//}()
}

// RemoveTray removes the system tray icon.
func RemoveTray() {
	C.removeTrayIcon()
}

// ShowMenu displays the system tray menu. This can be called in response to a user action.
func ShowMenu() {
	C.showMenu()
}
