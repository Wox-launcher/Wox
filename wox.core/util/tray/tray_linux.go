package tray

/*
#cgo pkg-config: gtk+-3.0 ayatana-appindicator3-0.1
#cgo LDFLAGS: -pthread

#include <gtk/gtk.h>
#include <libayatana-appindicator/app-indicator.h>

extern void goMenuItemCallback(int tag);
extern void goTrayMenuItemAdded(int tag, char* label);
extern void goTrayMenuItemActivated(int tag);

typedef struct TrayIcon TrayIcon;

TrayIcon* create_tray();
void set_tray_icon(TrayIcon* tray, const char* icon_data, gsize icon_data_len);
void add_menu_item(TrayIcon* tray, const char* label, int tag);
void show_tray(TrayIcon* tray);
void cleanup_tray(TrayIcon* tray);
*/
import "C"
import (
	"context"
	"fmt"
	"sync"
	"unsafe"
	"wox/util"
)

var (
	trayMu            sync.RWMutex
	trayIcon          *C.TrayIcon
	callbacks         = make(map[int]func())
	nextTag           int
	leftClickCallback func()
)

//export reportLeftClick
func reportLeftClick() {
	trayMu.RLock()
	callback := leftClickCallback
	trayMu.RUnlock()
	if callback != nil {
		go callback()
	}
}

//export goMenuItemCallback
func goMenuItemCallback(tag C.int) {
	trayMu.RLock()
	callback, ok := callbacks[int(tag)]
	trayMu.RUnlock()
	util.GetLogger().Info(context.Background(), fmt.Sprintf("Wox tray Go callback: tag=%d found=%t", int(tag), ok && callback != nil))
	if ok && callback != nil {
		go callback()
	}
}

//export goTrayMenuItemAdded
func goTrayMenuItemAdded(tag C.int, label *C.char) {
	util.GetLogger().Info(context.Background(), fmt.Sprintf("Wox tray menu item added: tag=%d label=%s", int(tag), C.GoString(label)))
}

//export goTrayMenuItemActivated
func goTrayMenuItemActivated(tag C.int) {
	util.GetLogger().Info(context.Background(), fmt.Sprintf("Wox tray menu activate: tag=%d", int(tag)))
}

func CreateTray(appIcon []byte, onClick func(), items ...MenuItem) {
	trayMu.Lock()
	leftClickCallback = onClick
	callbacks = make(map[int]func(), len(items))
	nextTag = 0
	trayMu.Unlock()

	trayIcon = C.create_tray()
	if trayIcon == nil {
		return
	}

	// Set icon if provided
	if len(appIcon) > 0 {
		iconData := C.CBytes(appIcon)
		defer C.free(iconData)
		C.set_tray_icon(trayIcon, (*C.char)(iconData), C.gsize(len(appIcon)))
	}

	// Add menu items
	for _, item := range items {
		trayMu.Lock()
		tag := nextTag
		nextTag++
		callbacks[tag] = item.Callback
		trayMu.Unlock()

		cTitle := C.CString(item.Title)
		defer C.free(unsafe.Pointer(cTitle))

		C.add_menu_item(trayIcon, cTitle, C.int(tag))
	}

	C.show_tray(trayIcon)
}

func RemoveTray() {
	if trayIcon != nil {
		C.cleanup_tray(trayIcon)
		trayIcon = nil
	}
	trayMu.Lock()
	callbacks = make(map[int]func())
	leftClickCallback = nil
	trayMu.Unlock()
}

func SetQueryIcons(items []QueryIconItem) {
	// Linux query tray icons are not implemented yet.
}
