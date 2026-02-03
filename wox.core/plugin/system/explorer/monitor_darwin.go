package explorer

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
extern void fileExplorerActivatedCallbackCGO(int pid);
extern void fileExplorerDeactivatedCallbackCGO();
void startFileExplorerMonitor();
void stopFileExplorerMonitor();
*/
import "C"

var onFileExplorerActivated func(pid int)
var onFileExplorerDeactivated func()

//export fileExplorerActivatedCallbackCGO
func fileExplorerActivatedCallbackCGO(pid C.int) {
	if onFileExplorerActivated != nil {
		onFileExplorerActivated(int(pid))
	}
}

//export fileExplorerDeactivatedCallbackCGO
func fileExplorerDeactivatedCallbackCGO() {
	if onFileExplorerDeactivated != nil {
		onFileExplorerDeactivated()
	}
}

func StartExplorerMonitor(activated func(pid int), deactivated func()) {
	onFileExplorerActivated = activated
	onFileExplorerDeactivated = deactivated
	C.startFileExplorerMonitor()
}

func StopExplorerMonitor() {
	C.stopFileExplorerMonitor()
	onFileExplorerActivated = nil
	onFileExplorerDeactivated = nil
}
