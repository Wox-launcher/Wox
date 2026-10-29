package tray

/*
#cgo LDFLAGS: -lshell32 -lole32 -luuid -lgdi32 -ladvapi32
#include <windows.h>

extern void init(char *iconPath, char *tooltip);
extern void addMenuItem(unsigned int id, char *title);
extern void addMenuSeparator();
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
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"unsafe"
	"wox/util"
	"wox/util/appearance"
)

//export trayMenuIsDark
func trayMenuIsDark() C.int {
	if appearance.IsDark() {
		return 1
	}
	return 0
}

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
func reportQueryClick(iconId C.UINT_PTR, x C.int, y C.int, width C.int, height C.int, displayID *C.char) {
	trayMu.RLock()
	callback, exists := queryIconCallbacks[uint32(iconId)]
	trayMu.RUnlock()
	if !exists || callback == nil {
		return
	}

	callback(ClickRect{
		DisplayID: C.GoString(displayID),
		X:         int(x),
		Y:         int(y),
		Width:     int(width),
		Height:    int(height),
	})
}

// initializes the system tray icon and menu.
func CreateTray(appIcon []byte, onClick func(), items ...MenuItem) {
	iconPath, err := writeTrayIcon(appIcon, "wox-tray-*.ico")
	if err != nil {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("create tray icon: %v", err))
		return
	}
	trayMu.Lock()
	leftClickCallback = onClick
	menuCallbacks = make(map[uint32]func(), len(items))
	queryIconMenuIDs = nil
	if mainIconTempPath != "" {
		_ = os.Remove(mainIconTempPath)
		mainIconTempPath = ""
	}
	trayMu.Unlock()

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
		for i, item := range itemsCopy {
			if i > 0 && item.SeparatorBefore {
				C.addMenuSeparator()
			}
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

// writeTrayIcon publishes a usable path only after all writes succeed. An
// unwritable temporary directory must not panic and abort launcher startup.
func writeTrayIcon(icon []byte, pattern string) (string, error) {
	file, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("create temporary icon: %w", err)
	}
	_, writeErr := file.Write(icon)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("save temporary icon: write=%v close=%v", writeErr, closeErr)
	}
	return file.Name(), nil
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

		iconPath, err := writeTrayIcon(item.Icon, "wox-query-tray-*.ico")
		if err != nil {
			util.GetLogger().Error(context.Background(), fmt.Sprintf("create query tray icon: %v", err))
			continue
		}

		trayMu.Lock()
		queryIconTempPaths = append(queryIconTempPaths, iconPath)
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

		iconPathC := C.CString(iconPath)
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
