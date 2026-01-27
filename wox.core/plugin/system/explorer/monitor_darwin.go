package explorer

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
extern void finderActivatedCallbackCGO(int pid);
void startFinderMonitor();
void stopFinderMonitor();
*/
import "C"

var onFinderActivated func(pid int)

//export finderActivatedCallbackCGO
func finderActivatedCallbackCGO(pid C.int) {
	if onFinderActivated != nil {
		onFinderActivated(int(pid))
	}
}

func StartMonitor(callback func(pid int)) {
	onFinderActivated = callback
	C.startFinderMonitor()
}

func StopMonitor() {
	C.stopFinderMonitor()
	onFinderActivated = nil
}
