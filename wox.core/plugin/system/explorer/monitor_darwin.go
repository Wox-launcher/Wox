package explorer

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
extern void fileExplorerActivatedCallbackCGO(int pid, int isFileDialog, int x, int y, int w, int h);
extern void fileExplorerDeactivatedCallbackCGO();
extern void fileExplorerKeyDownCallbackCGO(char key);
int getCurrentFinderWindowRect(int *x, int *y, int *w, int *h);
void startFileExplorerMonitor();
void stopFileExplorerMonitor();
*/
import "C"

var (
	explorerActivatedCallback   func(pid int)
	explorerDeactivatedCallback func()
	dialogActivatedCallback     func(pid int)
	dialogDeactivatedCallback   func()
	explorerKeyListener         func(key string)
	dialogKeyListener           func(key string)

	explorerActive bool
	explorerRectX  int
	explorerRectY  int
	explorerRectW  int
	explorerRectH  int

	dialogActive bool
	dialogRectX  int
	dialogRectY  int
	dialogRectW  int
	dialogRectH  int
)

type monitorState int

const (
	stateNone monitorState = iota
	stateExplorer
	stateDialog
)

var currentState monitorState = stateNone

//export fileExplorerKeyDownCallbackCGO
func fileExplorerKeyDownCallbackCGO(key C.char) {
	k := string(rune(key))
	if currentState == stateExplorer && explorerKeyListener != nil {
		explorerKeyListener(k)
		return
	}

	if currentState == stateDialog && dialogKeyListener != nil {
		dialogKeyListener(k)
	}
}

//export fileExplorerActivatedCallbackCGO
func fileExplorerActivatedCallbackCGO(pid C.int, isFileDialog C.int, x, y, w, h C.int) {
	isDialog := int(isFileDialog) == 1
	rectX, rectY, rectW, rectH := int(x), int(y), int(w), int(h)

	if isDialog {
		if currentState == stateExplorer {
			explorerActive = false
			if explorerDeactivatedCallback != nil {
				explorerDeactivatedCallback()
			}
		}
		currentState = stateDialog
		dialogActive = true
		dialogRectX = rectX
		dialogRectY = rectY
		dialogRectW = rectW
		dialogRectH = rectH
		if dialogActivatedCallback != nil {
			dialogActivatedCallback(int(pid))
		}
		return
	}

	if currentState == stateDialog {
		dialogActive = false
		if dialogDeactivatedCallback != nil {
			dialogDeactivatedCallback()
		}
	}
	currentState = stateExplorer
	explorerActive = true
	explorerRectX = rectX
	explorerRectY = rectY
	explorerRectW = rectW
	explorerRectH = rectH
	if explorerActivatedCallback != nil {
		explorerActivatedCallback(int(pid))
	}
}

//export fileExplorerDeactivatedCallbackCGO
func fileExplorerDeactivatedCallbackCGO() {
	if currentState == stateExplorer {
		explorerActive = false
		if explorerDeactivatedCallback != nil {
			explorerDeactivatedCallback()
		}
	}
	if currentState == stateDialog {
		dialogActive = false
		if dialogDeactivatedCallback != nil {
			dialogDeactivatedCallback()
		}
	}
	currentState = stateNone
}

func checkUpdateMonitorState() {
	needMonitor := explorerActivatedCallback != nil || explorerDeactivatedCallback != nil ||
		dialogActivatedCallback != nil || dialogDeactivatedCallback != nil ||
		explorerKeyListener != nil || dialogKeyListener != nil

	if needMonitor {
		C.startFileExplorerMonitor()
		return
	}

	C.stopFileExplorerMonitor()
	currentState = stateNone
	explorerActive = false
	dialogActive = false
}

func StartExplorerMonitor(activated func(pid int), deactivated func(), keyListener func(string)) {
	explorerActivatedCallback = activated
	explorerDeactivatedCallback = deactivated
	explorerKeyListener = keyListener
	checkUpdateMonitorState()
}

func StopExplorerMonitor() {
	explorerActivatedCallback = nil
	explorerDeactivatedCallback = nil
	explorerKeyListener = nil
	if currentState == stateExplorer {
		currentState = stateNone
		explorerActive = false
	}
	checkUpdateMonitorState()
}

func GetActiveExplorerRect() (int, int, int, int, bool) {
	if explorerActive {
		var x, y, w, h C.int
		if int(C.getCurrentFinderWindowRect(&x, &y, &w, &h)) == 1 {
			explorerRectX = int(x)
			explorerRectY = int(y)
			explorerRectW = int(w)
			explorerRectH = int(h)
			return explorerRectX, explorerRectY, explorerRectW, explorerRectH, true
		}
		explorerActive = false
		if currentState == stateExplorer {
			currentState = stateNone
		}
		return 0, 0, 0, 0, false
	}
	return 0, 0, 0, 0, false
}

func StartExplorerOpenSaveMonitor(activated func(pid int), deactivated func(), keyListener func(string)) {
	dialogActivatedCallback = activated
	dialogDeactivatedCallback = deactivated
	dialogKeyListener = keyListener
	checkUpdateMonitorState()
}

func StopExplorerOpenSaveMonitor() {
	dialogActivatedCallback = nil
	dialogDeactivatedCallback = nil
	dialogKeyListener = nil
	if currentState == stateDialog {
		currentState = stateNone
		dialogActive = false
	}
	checkUpdateMonitorState()
}

func GetActiveDialogRect() (int, int, int, int, bool) {
	if dialogActive {
		return dialogRectX, dialogRectY, dialogRectW, dialogRectH, true
	}
	return 0, 0, 0, 0, false
}
