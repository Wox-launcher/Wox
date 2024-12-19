package tray

/*
#cgo LDFLAGS: -lshell32
#include <windows.h>

extern void init(char *iconPath, char *tooltip);
extern void addMenuItem(unsigned int id, char *title);
extern void removeTrayIcon();
extern void showMenu();
extern unsigned int nextMenuId;
extern LPVOID GetLastErrorToText(DWORD error);
extern DWORD my_CreateIconFromResourceEx(
    PBYTE pbIconBits,
    DWORD cbIconBits,
    BOOL  fIcon,
    DWORD dwVersion,
    int  cxDesired,
    int  cyDesired,
    UINT uFlags );
extern void runMessageLoop();  // 声明 C 函数
*/
import "C"
import (
	"os"
	"time"
	"unsafe"

	"golang.design/x/hotkey/mainthread"
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
	temp, _ := os.CreateTemp("", "app.ico")
	temp.Write(appIcon)
	temp.Close()
	iconPath := temp.Name()
	iconPathC := C.CString(iconPath)
	defer C.free(unsafe.Pointer(iconPathC))
	tooltipC := C.CString("Wox")
	defer C.free(unsafe.Pointer(tooltipC))

	C.init(iconPathC, tooltipC)

	for _, item := range items {
		title := C.CString(item.Title)
		defer C.free(unsafe.Pointer(title))
		menuId := C.nextMenuId
		C.addMenuItem(menuId, title)
		menuCallbacks[uint32(menuId)] = item.Callback
		C.nextMenuId++
	}

	go func() {
		time.Sleep(time.Second)
		mainthread.Call(func() {
			C.runMessageLoop()
		})
	}()
}

// RemoveTray removes the system tray icon.
func RemoveTray() {
	C.removeTrayIcon()
}

// ShowMenu displays the system tray menu. This can be called in response to a user action.
func ShowMenu() {
	C.showMenu()
}
