package tray

/*
#cgo LDFLAGS: -lshell32
#include <windows.h>

extern void init(char *iconPath, char *tooltip);
extern void addMenuItem(unsigned int id, char *title);
extern void addQueryTrayIcon(unsigned int id, char *iconPath, char *tooltip, unsigned int menuId, char *menuTitle);
extern void clearQueryTrayIcons();
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
	"runtime"
	"sync"
	"unsafe"
)

// menuCallbacks maps menu item IDs to their callbacks.
var trayMu sync.RWMutex
var menuCallbacks = make(map[uint32]func())
var queryIconCallbacks = make(map[uint32]func(ClickRect))
var leftClickCallback func()
var mainIconTempPath string
var queryIconTempPaths []string
var queryIconMenuIDs []uint32
var nextQueryIconID uint32 = 1000
var nextQueryMenuID uint32 = 10000

//export reportClick
func reportClick(menuId C.UINT_PTR) {
	trayMu.RLock()
	callback, exists := menuCallbacks[uint32(menuId)]
	trayMu.RUnlock()
	if exists && callback != nil {
		callback()
	}
}

//export reportLeftClick
func reportLeftClick() {
	trayMu.RLock()
	callback := leftClickCallback
	trayMu.RUnlock()
	if callback != nil {
		callback()
	}
}

//export reportQueryClick
func reportQueryClick(iconId C.UINT_PTR, x C.int, y C.int, width C.int, height C.int) {
	trayMu.RLock()
	callback, exists := queryIconCallbacks[uint32(iconId)]
	trayMu.RUnlock()
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

// initializes the system tray icon and menu.
func CreateTray(appIcon []byte, onClick func(), items ...MenuItem) {
	trayMu.Lock()
	leftClickCallback = onClick
	menuCallbacks = make(map[uint32]func(), len(items))
	queryIconMenuIDs = nil
	if mainIconTempPath != "" {
		_ = os.Remove(mainIconTempPath)
		mainIconTempPath = ""
	}
	trayMu.Unlock()

	temp, _ := os.CreateTemp("", "app.ico")
	temp.Write(appIcon)
	temp.Close()
	iconPath := temp.Name()

	trayMu.Lock()
	mainIconTempPath = iconPath
	trayMu.Unlock()

	ready := make(chan struct{})
	itemsCopy := append([]MenuItem(nil), items...)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		iconPathC := C.CString(iconPath)
		defer C.free(unsafe.Pointer(iconPathC))
		tooltipC := C.CString("Wox")
		defer C.free(unsafe.Pointer(tooltipC))

		C.init(iconPathC, tooltipC)

		trayMu.Lock()
		for _, item := range itemsCopy {
			title := C.CString(item.Title)
			menuId := C.nextMenuId
			C.addMenuItem(menuId, title)
			C.free(unsafe.Pointer(title))

			menuCallbacks[uint32(menuId)] = item.Callback
			C.nextMenuId++
		}
		trayMu.Unlock()

		close(ready)
		C.runMessageLoop()
	}()

	<-ready
}

// RemoveTray removes the system tray icon.
func RemoveTray() {
	clearQueryIconsInternal()
	C.removeTrayIcon()

	trayMu.Lock()
	leftClickCallback = nil
	menuCallbacks = make(map[uint32]func())
	if mainIconTempPath != "" {
		_ = os.Remove(mainIconTempPath)
		mainIconTempPath = ""
	}
	trayMu.Unlock()
}

// ShowMenu displays the system tray menu. This can be called in response to a user action.
func ShowMenu() {
	C.showMenu()
}

func SetQueryIcons(items []QueryIconItem) {
	clearQueryIconsInternal()

	trayMu.Lock()
	queryIconCallbacks = make(map[uint32]func(ClickRect))
	nextQueryIconID = 1000
	nextQueryMenuID = 10000
	trayMu.Unlock()

	for _, item := range items {
		if len(item.Icon) == 0 || item.Callback == nil {
			continue
		}

		temp, err := os.CreateTemp("", "wox-query-tray-*.ico")
		if err != nil {
			continue
		}
		_, _ = temp.Write(item.Icon)
		_ = temp.Close()

		trayMu.Lock()
		queryIconTempPaths = append(queryIconTempPaths, temp.Name())
		trayMu.Unlock()

		var menuID uint32
		if item.ContextMenuTitle != "" && item.ContextMenuCallback != nil {
			trayMu.Lock()
			menuID = nextQueryMenuID
			nextQueryMenuID++
			menuCallbacks[menuID] = item.ContextMenuCallback
			queryIconMenuIDs = append(queryIconMenuIDs, menuID)
			trayMu.Unlock()
		}

		iconPathC := C.CString(temp.Name())
		tooltipC := C.CString(item.Tooltip)
		menuTitleC := C.CString(item.ContextMenuTitle)
		C.addQueryTrayIcon(C.uint(nextQueryIconID), iconPathC, tooltipC, C.uint(menuID), menuTitleC)
		C.free(unsafe.Pointer(iconPathC))
		C.free(unsafe.Pointer(tooltipC))
		C.free(unsafe.Pointer(menuTitleC))

		trayMu.Lock()
		queryIconCallbacks[nextQueryIconID] = item.Callback
		nextQueryIconID++
		trayMu.Unlock()
	}
}

func clearQueryIconsInternal() {
	C.clearQueryTrayIcons()

	trayMu.Lock()
	tempPaths := queryIconTempPaths
	queryIconTempPaths = nil
	queryIconCallbacks = make(map[uint32]func(ClickRect))
	menuIDs := queryIconMenuIDs
	queryIconMenuIDs = nil
	trayMu.Unlock()

	for _, tempPath := range tempPaths {
		_ = os.Remove(tempPath)
	}

	trayMu.Lock()
	for _, menuID := range menuIDs {
		delete(menuCallbacks, menuID)
	}
	trayMu.Unlock()
}
