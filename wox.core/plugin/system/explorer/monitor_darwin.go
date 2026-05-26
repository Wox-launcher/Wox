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
	"fmt"
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

type monitorRawKeySubscription struct {
	id       int
	isDialog bool
	once     sync.Once
}

const (
	stateNone monitorState = iota
	stateExplorer
	stateDialog
)

var currentState monitorState = stateNone
var nextRawKeyListenerID = 1
var explorerRawKeyListeners = map[int]ExplorerRawKeyListener{}
var dialogRawKeyListeners = map[int]ExplorerRawKeyListener{}

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
		explorerKeyListener != nil || dialogKeyListener != nil ||
		len(explorerRawKeyListeners) > 0 || len(dialogRawKeyListeners) > 0
	needRawListener := explorerKeyListener != nil || dialogKeyListener != nil ||
		len(explorerRawKeyListeners) > 0 || len(dialogRawKeyListeners) > 0
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

// AddExplorerRawKeyListener registers a raw-key listener for active Finder
// windows without replacing the Explorer plugin's type-to-search listener.
func AddExplorerRawKeyListener(listener ExplorerRawKeyListener) (ExplorerRawKeySubscription, error) {
	return addExplorerRawKeyListener(false, listener)
}

// AddExplorerOpenSaveRawKeyListener registers a raw-key listener for active
// open/save dialogs without replacing the Explorer plugin's listener.
func AddExplorerOpenSaveRawKeyListener(listener ExplorerRawKeyListener) (ExplorerRawKeySubscription, error) {
	return addExplorerRawKeyListener(true, listener)
}

func addExplorerRawKeyListener(isDialog bool, listener ExplorerRawKeyListener) (ExplorerRawKeySubscription, error) {
	if listener == nil {
		return nil, fmt.Errorf("raw key listener is required")
	}

	stateMu.Lock()
	id := nextRawKeyListenerID
	nextRawKeyListenerID++
	if isDialog {
		dialogRawKeyListeners[id] = listener
	} else {
		explorerRawKeyListeners[id] = listener
	}
	stateMu.Unlock()

	if err := checkUpdateMonitorState(); err != nil {
		stateMu.Lock()
		if isDialog {
			delete(dialogRawKeyListeners, id)
		} else {
			delete(explorerRawKeyListeners, id)
		}
		stateMu.Unlock()
		_ = checkUpdateMonitorState()
		return nil, err
	}

	return &monitorRawKeySubscription{id: id, isDialog: isDialog}, nil
}

func (s *monitorRawKeySubscription) Close() error {
	if s == nil {
		return nil
	}

	s.once.Do(func() {
		stateMu.Lock()
		if s.isDialog {
			delete(dialogRawKeyListeners, s.id)
		} else {
			delete(explorerRawKeyListeners, s.id)
		}
		stateMu.Unlock()
		_ = checkUpdateMonitorState()
	})
	return nil
}

func handleExplorerRawKeyEvent(event keyboard.RawKeyEvent) bool {
	if event.Key == keyboard.KeyUnknown {
		return false
	}

	if event.Type == keyboard.EventTypeKeyUp {
		return dispatchRawKeyToAllListeners(event)
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
	explorerRawListeners := copyExplorerRawKeyListeners(explorerRawKeyListeners)
	dialogRawListeners := copyExplorerRawKeyListeners(dialogRawKeyListeners)
	stateMu.RUnlock()

	consume := false
	if state == stateExplorer && explorerListener != nil {
		consume = dispatchRawKeyListeners(event, explorerRawListeners) || consume
		if shouldDispatchTypeToSearch(event) {
			explorerListener(key)
		}
	}
	if state == stateDialog && dialogListener != nil {
		consume = dispatchRawKeyListeners(event, dialogRawListeners) || consume
		if shouldDispatchTypeToSearch(event) {
			dialogListener(key)
		}
	}
	if state == stateExplorer && explorerListener == nil {
		consume = dispatchRawKeyListeners(event, explorerRawListeners) || consume
	}
	if state == stateDialog && dialogListener == nil {
		consume = dispatchRawKeyListeners(event, dialogRawListeners) || consume
	}
	return consume
}

func dispatchRawKeyToAllListeners(event keyboard.RawKeyEvent) bool {
	stateMu.RLock()
	explorerRawListeners := copyExplorerRawKeyListeners(explorerRawKeyListeners)
	dialogRawListeners := copyExplorerRawKeyListeners(dialogRawKeyListeners)
	stateMu.RUnlock()

	consume := dispatchRawKeyListeners(event, explorerRawListeners)
	return dispatchRawKeyListeners(event, dialogRawListeners) || consume
}

func dispatchRawKeyListeners(event keyboard.RawKeyEvent, listeners []ExplorerRawKeyListener) bool {
	consume := false
	for _, listener := range listeners {
		if listener != nil && listener(event) {
			consume = true
		}
	}
	return consume
}

func copyExplorerRawKeyListeners(listeners map[int]ExplorerRawKeyListener) []ExplorerRawKeyListener {
	copied := make([]ExplorerRawKeyListener, 0, len(listeners))
	for _, listener := range listeners {
		copied = append(copied, listener)
	}
	return copied
}

func shouldDispatchTypeToSearch(event keyboard.RawKeyEvent) bool {
	return event.Type == keyboard.EventTypeKeyDown &&
		event.Character != "" &&
		event.Modifiers&(keyboard.ModifierCtrl|keyboard.ModifierAlt|keyboard.ModifierSuper) == 0
}
