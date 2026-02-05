package explorer

/*
extern void fileExplorerActivatedCallbackCGO(int pid, int isFileDialog);
extern void fileExplorerDeactivatedCallbackCGO();
void startFileExplorerMonitor();
void stopFileExplorerMonitor();
*/
import "C"

var (
	explorerActivatedCallback   func(pid int)
	explorerDeactivatedCallback func()
	dialogActivatedCallback     func(pid int)
	dialogDeactivatedCallback   func()
)

type monitorState int

const (
	stateNone monitorState = iota
	stateExplorer
	stateDialog
)

var currentState monitorState = stateNone

//export fileExplorerActivatedCallbackCGO
func fileExplorerActivatedCallbackCGO(pid C.int, isFileDialog C.int) {
	isDialog := int(isFileDialog) == 1
	newPid := int(pid)

	if isDialog {
		// Transition: Anything -> Dialog
		if currentState == stateExplorer && explorerDeactivatedCallback != nil {
			explorerDeactivatedCallback()
		}
		currentState = stateDialog
		if dialogActivatedCallback != nil {
			dialogActivatedCallback(newPid)
		}
	} else {
		// Transition: Anything -> Explorer
		if currentState == stateDialog && dialogDeactivatedCallback != nil {
			dialogDeactivatedCallback()
		}
		currentState = stateExplorer
		if explorerActivatedCallback != nil {
			explorerActivatedCallback(newPid)
		}
	}
}

//export fileExplorerDeactivatedCallbackCGO
func fileExplorerDeactivatedCallbackCGO() {
	if currentState == stateExplorer && explorerDeactivatedCallback != nil {
		explorerDeactivatedCallback()
	}
	if currentState == stateDialog && dialogDeactivatedCallback != nil {
		dialogDeactivatedCallback()
	}
	currentState = stateNone
}

func checkUpdateMonitorState() {
	if explorerActivatedCallback != nil || dialogActivatedCallback != nil {
		C.startFileExplorerMonitor()
	} else {
		C.stopFileExplorerMonitor()
		currentState = stateNone
	}
}

func StartExplorerMonitor(activated func(pid int), deactivated func()) {
	explorerActivatedCallback = activated
	explorerDeactivatedCallback = deactivated
	checkUpdateMonitorState()
}

func StopExplorerMonitor() {
	explorerActivatedCallback = nil
	explorerDeactivatedCallback = nil // Fixed: Clear deactivated callback

	// If currently in explorer state, trigger deactivation because we are stopping monitoring
	if currentState == stateExplorer {
		// We don't call the callback because we just cleared it,
		// but we should reset state if this was the active one.
		// Actually, if we stop monitoring, the user probably expects no more callbacks.
		currentState = stateNone
	}

	checkUpdateMonitorState()
}

func StartExplorerOpenSaveMonitor(activated func(pid int), deactivated func()) {
	dialogActivatedCallback = activated
	dialogDeactivatedCallback = deactivated
	checkUpdateMonitorState()
}

func StopExplorerOpenSaveMonitor() {
	dialogActivatedCallback = nil
	dialogDeactivatedCallback = nil

	if currentState == stateDialog {
		currentState = stateNone
	}

	checkUpdateMonitorState()
}
