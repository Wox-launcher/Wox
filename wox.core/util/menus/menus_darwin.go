package menus

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa -framework AppKit
#include <stdlib.h>

char** getMenuItems(int pid, int* count);
void performMenuAction(int pid, const char* title);
*/
import "C"
import (
	"errors"
	"unsafe"
)

func GetAppMenuTitles(pid int) ([]string, error) {
	var count C.int
	cItems := C.getMenuItems(C.int(pid), &count)
	defer C.free(unsafe.Pointer(cItems))

	if count == 0 {
		return nil, errors.New("no menu items found")
	}
	if cItems == nil {
		return nil, errors.New("failed to get menu items")
	}

	items := make([]string, int(count))
	cItemsSlice := (*[1 << 30]*C.char)(unsafe.Pointer(cItems))[:count:count]

	for i := 0; i < int(count); i++ {
		items[i] = C.GoString(cItemsSlice[i])
		C.free(unsafe.Pointer(cItemsSlice[i]))
	}

	return items, nil
}

func ExecuteActiveAppMenu(pid int, title string) {
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))

	C.performMenuAction(C.int(pid), cTitle)
}
