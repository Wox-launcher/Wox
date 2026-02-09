package tray

/*
#cgo LDFLAGS: -lshell32
#include <windows.h>

extern void init(char *iconPath, char *tooltip);
extern void addMenuItem(unsigned int id, char *title);
extern void addQueryTrayIcon(unsigned int id, char *iconPath, char *tooltip);
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
	"time"
	"unsafe"

	"golang.design/x/hotkey/mainthread"
)

// menuCallbacks maps menu item IDs to their callbacks.
var menuCallbacks = make(map[uint32]func())
var queryIconCallbacks = make(map[uint32]func(ClickRect))
var leftClickCallback func()
var mainIconTempPath string
var queryIconTempPaths []string
var nextQueryIconID uint32 = 1000

//export reportClick
func reportClick(menuId C.UINT_PTR) {
	if callback, exists := menuCallbacks[uint32(menuId)]; exists {
		callback()
	}
}

//export reportLeftClick
func reportLeftClick() {
	if leftClickCallback != nil {
		leftClickCallback()
	}
}

//export reportQueryClick
func reportQueryClick(iconId C.UINT_PTR, x C.int, y C.int, width C.int, height C.int) {
	callback, exists := queryIconCallbacks[uint32(iconId)]
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
	leftClickCallback = onClick
	if mainIconTempPath != "" {
		_ = os.Remove(mainIconTempPath)
		mainIconTempPath = ""
	}
	temp, _ := os.CreateTemp("", "app.ico")
	temp.Write(appIcon)
	temp.Close()
	iconPath := temp.Name()
	mainIconTempPath = iconPath
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
	clearQueryIconsInternal()
	C.removeTrayIcon()
	if mainIconTempPath != "" {
		_ = os.Remove(mainIconTempPath)
		mainIconTempPath = ""
	}
}

// ShowMenu displays the system tray menu. This can be called in response to a user action.
func ShowMenu() {
	C.showMenu()
}

func SetQueryIcons(items []QueryIconItem) {
	clearQueryIconsInternal()
	queryIconCallbacks = make(map[uint32]func(ClickRect))
	nextQueryIconID = 1000

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
		queryIconTempPaths = append(queryIconTempPaths, temp.Name())

		iconPathC := C.CString(temp.Name())
		tooltipC := C.CString(item.Tooltip)
		C.addQueryTrayIcon(C.uint(nextQueryIconID), iconPathC, tooltipC)
		C.free(unsafe.Pointer(iconPathC))
		C.free(unsafe.Pointer(tooltipC))

		queryIconCallbacks[nextQueryIconID] = item.Callback
		nextQueryIconID++
	}
}

func clearQueryIconsInternal() {
	C.clearQueryTrayIcons()
	for _, tempPath := range queryIconTempPaths {
		_ = os.Remove(tempPath)
	}
	queryIconTempPaths = nil
	queryIconCallbacks = make(map[uint32]func(ClickRect))
}
