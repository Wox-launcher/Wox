package tray

/*
#cgo pkg-config: gtk+-3.0 ayatana-appindicator3-0.1
#cgo LDFLAGS: -pthread

#include <gtk/gtk.h>
#include <libayatana-appindicator/app-indicator.h>

// Function declarations
extern void goMenuItemCallback(int tag);

typedef struct {
    AppIndicator *indicator;
    GtkMenu *menu;
    GMainLoop *loop;
} TrayIcon;

TrayIcon* create_tray();
void set_tray_icon(TrayIcon* tray, const char* icon_data, gsize icon_data_len);
void add_menu_item(TrayIcon* tray, const char* label, int tag);
void cleanup_tray(TrayIcon* tray);
*/
import "C"
import (
	"sync"
	"unsafe"
)

var (
	trayIcon    *C.TrayIcon
	callbacks   = make(map[int]func())
	nextTag     int
	initOnce    sync.Once
	initialized bool
)

//export goMenuItemCallback
func goMenuItemCallback(tag C.int) {
	if callback, ok := callbacks[int(tag)]; ok {
		callback()
	}
}

func CreateTray(appIcon []byte, items ...MenuItem) {
	initOnce.Do(func() {
		if !initialized {
			C.gtk_init(nil, nil)
			initialized = true
		}
	})

	// Create tray
	trayIcon = C.create_tray()

	// Set icon if provided
	if len(appIcon) > 0 {
		iconData := C.CBytes(appIcon)
		defer C.free(iconData)
		C.set_tray_icon(trayIcon, (*C.char)(iconData), C.gsize(len(appIcon)))
	}

	// Add menu items
	for _, item := range items {
		tag := nextTag
		nextTag++
		callbacks[tag] = item.Callback

		cTitle := C.CString(item.Title)
		defer C.free(unsafe.Pointer(cTitle))

		C.add_menu_item(trayIcon, cTitle, C.int(tag))
	}
}

func RemoveTray() {
	if trayIcon != nil {
		C.cleanup_tray(trayIcon)
		trayIcon = nil
	}
	callbacks = make(map[int]func())
}
