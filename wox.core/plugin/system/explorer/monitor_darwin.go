package explorer

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
extern void fileExplorerActivatedCallbackCGO(int pid, int isFileDialog);
extern void fileExplorerDeactivatedCallbackCGO();
void startFileExplorerMonitor();
void stopFileExplorerMonitor();
*/
import "C"

var (
	explorerActivatedCallback   func(pid int)
	explorerDeactivatedCallback func()
)

//export fileExplorerActivatedCallbackCGO
func fileExplorerActivatedCallbackCGO(pid C.int, isFileDialog C.int) {
	// macOS implemetation currently only supports Finder (isFileDialog=0)
	if int(isFileDialog) == 0 && explorerActivatedCallback != nil {
		explorerActivatedCallback(int(pid))
	}
}

//export fileExplorerDeactivatedCallbackCGO
func fileExplorerDeactivatedCallbackCGO() {
	if explorerDeactivatedCallback != nil {
		explorerDeactivatedCallback()
	}
}

func StartExplorerMonitor(activated func(pid int), deactivated func()) {
	explorerActivatedCallback = activated
	explorerDeactivatedCallback = deactivated
	C.startFileExplorerMonitor()
}

func StopExplorerMonitor() {
	explorerActivatedCallback = nil
	explorerDeactivatedCallback = nil
	C.stopFileExplorerMonitor()
}

func StartExplorerOpenSaveMonitor(activated func(pid int), deactivated func()) {
	// Not implemented for macOS yet
}

func StopExplorerOpenSaveMonitor() {
	// Not implemented for macOS yet
}
