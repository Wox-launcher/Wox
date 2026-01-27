package explorer

/*
extern void finderActivatedCallbackCGO(int pid);
void startFinderMonitor();
void stopFinderMonitor();
*/
import "C"

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
