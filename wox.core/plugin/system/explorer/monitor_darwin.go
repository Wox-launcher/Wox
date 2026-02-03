package explorer

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
extern void fileExplorerActivatedCallbackCGO(int pid);
void startFileExplorerMonitor();
void stopFileExplorerMonitor();
*/
import "C"

var onFileExplorerActivated func(pid int)

//export fileExplorerActivatedCallbackCGO
func fileExplorerActivatedCallbackCGO(pid C.int) {
	if onFileExplorerActivated != nil {
		onFileExplorerActivated(int(pid))
	}
}

func StartMonitor(callback func(pid int)) {
	onFileExplorerActivated = callback
	C.startFileExplorerMonitor()
}

func StopMonitor() {
	C.stopFileExplorerMonitor()
	onFileExplorerActivated = nil
}
