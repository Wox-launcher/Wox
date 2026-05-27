package explorer

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
extern void fileExplorerActivatedCallbackCGO(int pid, int isFileDialog, int x, int y, int w, int h);
extern void fileExplorerDeactivatedCallbackCGO();
int getCurrentFinderWindowRect(int *x, int *y, int *w, int *h);
int refreshFileExplorerMonitorState();
void startFileExplorerMonitor();
void stopFileExplorerMonitor();
*/
import "C"

import (
	"strings"
	"sync"
	"wox/util/keyboard"
)

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

	rawKeySubscription keyboard.RawKeySubscription
)

// stateMu protects Explorer/dialog state shared by native activation callbacks
// and the raw-key listener path.
var stateMu sync.RWMutex

type monitorState int

const (
	stateNone monitorState = iota
	stateExplorer
	stateDialog
)

var currentState monitorState = stateNone

//export fileExplorerActivatedCallbackCGO
func fileExplorerActivatedCallbackCGO(pid C.int, isFileDialog C.int, x, y, w, h C.int) {
	isDialog := int(isFileDialog) == 1
	rectX, rectY, rectW, rectH := int(x), int(y), int(w), int(h)
	var deactivated func()
	var activated func(pid int)

	stateMu.Lock()
	if isDialog {
		if currentState == stateExplorer {
			explorerActive = false
			deactivated = explorerDeactivatedCallback
		}
		currentState = stateDialog
		dialogActive = true
		dialogRectX = rectX
		dialogRectY = rectY
		dialogRectW = rectW
		dialogRectH = rectH
		activated = dialogActivatedCallback
	} else {
		if currentState == stateDialog {
			dialogActive = false
			deactivated = dialogDeactivatedCallback
		}
		currentState = stateExplorer
		explorerActive = true
		explorerRectX = rectX
		explorerRectY = rectY
		explorerRectW = rectW
		explorerRectH = rectH
		activated = explorerActivatedCallback
	}
	stateMu.Unlock()

	if deactivated != nil {
		deactivated()
	}
	if activated != nil {
		activated(int(pid))
	}
}

//export fileExplorerDeactivatedCallbackCGO
func fileExplorerDeactivatedCallbackCGO() {
	var deactivated func()

	stateMu.Lock()
	if currentState == stateExplorer {
		explorerActive = false
		deactivated = explorerDeactivatedCallback
	}
	if currentState == stateDialog {
		dialogActive = false
		deactivated = dialogDeactivatedCallback
	}
	currentState = stateNone
	stateMu.Unlock()

	if deactivated != nil {
		deactivated()
	}
}

func checkUpdateMonitorState() error {
	stateMu.RLock()
	needMonitor := explorerActivatedCallback != nil || explorerDeactivatedCallback != nil ||
		dialogActivatedCallback != nil || dialogDeactivatedCallback != nil ||
		explorerKeyListener != nil || dialogKeyListener != nil
	needRawListener := explorerKeyListener != nil || dialogKeyListener != nil
	stateMu.RUnlock()

	if needMonitor {
		C.startFileExplorerMonitor()
	} else {
		C.stopFileExplorerMonitor()
		stateMu.Lock()
		currentState = stateNone
		explorerActive = false
		dialogActive = false
		stateMu.Unlock()
	}

	if needRawListener {
		if rawKeySubscription == nil {
			subscription, err := keyboard.AddRawKeyListener(handleExplorerRawKeyEvent)
			if err == nil {
				rawKeySubscription = subscription
			} else {
				return err
			}
		}
		return nil
	}

	if rawKeySubscription != nil {
		_ = rawKeySubscription.Close()
		rawKeySubscription = nil
	}
	return nil
}

func StartExplorerMonitor(activated func(pid int), deactivated func(), keyListener func(string)) {
	stateMu.Lock()
	explorerActivatedCallback = activated
	explorerDeactivatedCallback = deactivated
	explorerKeyListener = keyListener
	stateMu.Unlock()
	_ = checkUpdateMonitorState()
}

func StopExplorerMonitor() {
	stateMu.Lock()
	explorerActivatedCallback = nil
	explorerDeactivatedCallback = nil
	explorerKeyListener = nil
	if currentState == stateExplorer {
		currentState = stateNone
		explorerActive = false
	}
	stateMu.Unlock()
	_ = checkUpdateMonitorState()
}

func GetActiveExplorerRect() (int, int, int, int, bool) {
	stateMu.RLock()
	isActive := explorerActive
	stateMu.RUnlock()

	if isActive {
		var x, y, w, h C.int
		if int(C.getCurrentFinderWindowRect(&x, &y, &w, &h)) == 1 {
			stateMu.Lock()
			explorerRectX = int(x)
			explorerRectY = int(y)
			explorerRectW = int(w)
			explorerRectH = int(h)
			rectX, rectY, rectW, rectH := explorerRectX, explorerRectY, explorerRectW, explorerRectH
			stateMu.Unlock()
			return rectX, rectY, rectW, rectH, true
		}
		stateMu.Lock()
		explorerActive = false
		if currentState == stateExplorer {
			currentState = stateNone
		}
		stateMu.Unlock()
		return 0, 0, 0, 0, false
	}
	return 0, 0, 0, 0, false
}

func StartExplorerOpenSaveMonitor(activated func(pid int), deactivated func(), keyListener func(string)) {
	stateMu.Lock()
	dialogActivatedCallback = activated
	dialogDeactivatedCallback = deactivated
	dialogKeyListener = keyListener
	stateMu.Unlock()
	_ = checkUpdateMonitorState()
}

func StopExplorerOpenSaveMonitor() {
	stateMu.Lock()
	dialogActivatedCallback = nil
	dialogDeactivatedCallback = nil
	dialogKeyListener = nil
	if currentState == stateDialog {
		currentState = stateNone
		dialogActive = false
	}
	stateMu.Unlock()
	_ = checkUpdateMonitorState()
}

func GetActiveDialogRect() (int, int, int, int, bool) {
	stateMu.RLock()
	defer stateMu.RUnlock()
	if dialogActive {
		return dialogRectX, dialogRectY, dialogRectW, dialogRectH, true
	}
	return 0, 0, 0, 0, false
}

// AddExplorerRawKeyListener is intentionally unsupported on macOS. Finder has
// native Quick Look, so Wox does not add a Selection Space-key listener there.
func AddExplorerRawKeyListener(listener ExplorerRawKeyListener) (ExplorerRawKeySubscription, error) {
	return nil, nil
}

// AddExplorerOpenSaveRawKeyListener is intentionally unsupported on macOS.
func AddExplorerOpenSaveRawKeyListener(listener ExplorerRawKeyListener) (ExplorerRawKeySubscription, error) {
	return nil, nil
}

func handleExplorerRawKeyEvent(event keyboard.RawKeyEvent) bool {
	if event.Key == keyboard.KeyUnknown {
		return false
	}

	if event.Type == keyboard.EventTypeKeyUp {
		return false
	}

	// Refresh native monitor state on each key so dialog text fields immediately
	// stop participating in Explorer type-to-search.
	if int(C.refreshFileExplorerMonitorState()) == 0 {
		return false
	}

	key := strings.ToLower(event.Character)
	stateMu.RLock()
	state := currentState
	explorerListener := explorerKeyListener
	dialogListener := dialogKeyListener
	stateMu.RUnlock()

	if state == stateExplorer && explorerListener != nil {
		if shouldDispatchTypeToSearch(event) {
			explorerListener(key)
		}
	}
	if state == stateDialog && dialogListener != nil {
		if shouldDispatchTypeToSearch(event) {
			dialogListener(key)
		}
	}
	return false
}

func shouldDispatchTypeToSearch(event keyboard.RawKeyEvent) bool {
	return event.Type == keyboard.EventTypeKeyDown &&
		event.Character != "" &&
		event.Modifiers&(keyboard.ModifierCtrl|keyboard.ModifierAlt|keyboard.ModifierSuper) == 0
}
