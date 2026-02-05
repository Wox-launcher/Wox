package explorer

/*
extern void fileExplorerActivatedCallbackCGO(int pid, int isFileDialog, int x, int y, int w, int h);
extern void fileExplorerDeactivatedCallbackCGO();
extern void fileExplorerKeyDownCallbackCGO(char key);
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

	// Internal state to track explorer window
	explorerActive bool
	explorerRectX  int
	explorerRectY  int
	explorerRectW  int
	explorerRectH  int

	// Internal state to track dialog window
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
	} else if currentState == stateDialog && dialogKeyListener != nil {
		dialogKeyListener(k)
	}
}

//export fileExplorerActivatedCallbackCGO
func fileExplorerActivatedCallbackCGO(pid C.int, isFileDialog C.int, x, y, w, h C.int) {
	isDialog := int(isFileDialog) == 1
	// newPid := int(pid) // Unused now
	rectX, rectY, rectW, rectH := int(x), int(y), int(w), int(h)

	if isDialog {
		// Transition: Anything -> Dialog
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
	} else {
		// Transition: Anything -> Explorer
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
	if explorerActivatedCallback != nil || explorerDeactivatedCallback != nil ||
		dialogActivatedCallback != nil || dialogDeactivatedCallback != nil ||
		explorerKeyListener != nil || dialogKeyListener != nil {
		C.startFileExplorerMonitor()
	} else {
		C.stopFileExplorerMonitor()
		currentState = stateNone
		explorerActive = false
		dialogActive = false
	}
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

	// If currently in explorer state, trigger deactivation because we are stopping monitoring
	if currentState == stateExplorer {
		// We don't call the callback because we just cleared it,
		// but we should reset state if this was the active one.
		// Actually, if we stop monitoring, the user probably expects no more callbacks.
		currentState = stateNone
		explorerActive = false
	}

	checkUpdateMonitorState()
}

func GetActiveExplorerRect() (int, int, int, int, bool) {
	if explorerActive {
		return explorerRectX, explorerRectY, explorerRectW, explorerRectH, true
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
