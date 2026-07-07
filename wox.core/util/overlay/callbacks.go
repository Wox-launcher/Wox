package overlay

import "sync"

var clickCallbacks = make(map[string]func() bool)
var clickCallbacksMu sync.RWMutex

var closeCallbacks = make(map[string]func())
var closeCallbacksMu sync.RWMutex

type nativeAttachmentRegistration struct {
	kind    NativeAttachmentKind
	handle  uintptr
	release func()
}

var nativeAttachmentCallbacks = make(map[string]nativeAttachmentRegistration)
var nativeAttachmentCallbacksMu sync.Mutex

// RequestClose closes an overlay as a user action, firing OnClose before the native window is removed.
func RequestClose(id string) {
	RegisterClickCallback(id, nil)
	if cb := takeCloseCallback(id); cb != nil {
		cb()
	}
	Close(id)
}

func invokeCloseCallback(id string) {
	ReleaseNativeAttachment(id)
	RegisterClickCallback(id, nil)
	if cb := takeCloseCallback(id); cb != nil {
		cb()
	}
}

func takeCloseCallback(id string) func() {
	closeCallbacksMu.Lock()
	cb, ok := closeCallbacks[id]
	if ok {
		delete(closeCallbacks, id)
	}
	closeCallbacksMu.Unlock()
	return cb
}

// RegisterCallbacks stores native overlay event handlers for a window ID.
func RegisterCallbacks(id string, onClick func() bool, onClose func()) {
	RegisterClickCallback(id, onClick)
	RegisterCloseCallback(id, onClose)
}

// RegisterClickCallback stores or clears the native click handler for a window ID.
func RegisterClickCallback(id string, onClick func() bool) {
	if onClick != nil {
		clickCallbacksMu.Lock()
		clickCallbacks[id] = onClick
		clickCallbacksMu.Unlock()
	} else {
		clickCallbacksMu.Lock()
		delete(clickCallbacks, id)
		clickCallbacksMu.Unlock()
	}
}

// RegisterCloseCallback stores or clears the native close handler for a window ID.
func RegisterCloseCallback(id string, onClose func()) {
	if onClose != nil {
		closeCallbacksMu.Lock()
		closeCallbacks[id] = onClose
		closeCallbacksMu.Unlock()
	} else {
		closeCallbacksMu.Lock()
		delete(closeCallbacks, id)
		closeCallbacksMu.Unlock()
	}
}

// RegisterNativeAttachment tracks the resource release callback for a native attachment window.
// The returned callback releases the replaced attachment after native code has detached it.
func RegisterNativeAttachment(id string, attachment NativeAttachment) func() {
	if id == "" {
		return nil
	}

	var releaseOld func()
	newRegistration := nativeAttachmentRegistration{
		kind:    attachment.Kind,
		handle:  attachment.Handle,
		release: attachment.OnRelease,
	}
	newActive := attachment.active() && attachment.OnRelease != nil

	nativeAttachmentCallbacksMu.Lock()
	oldRegistration, hasOld := nativeAttachmentCallbacks[id]
	sameAttachment := hasOld && oldRegistration.kind == newRegistration.kind && oldRegistration.handle == newRegistration.handle
	if hasOld && (!newActive || !sameAttachment) {
		releaseOld = oldRegistration.release
		delete(nativeAttachmentCallbacks, id)
	}
	if newActive {
		nativeAttachmentCallbacks[id] = newRegistration
	}
	nativeAttachmentCallbacksMu.Unlock()

	return releaseOld
}

// ReleaseNativeAttachment releases and forgets any native attachment owned by the window ID.
func ReleaseNativeAttachment(id string) {
	if id == "" {
		return
	}

	var release func()
	nativeAttachmentCallbacksMu.Lock()
	if registration, ok := nativeAttachmentCallbacks[id]; ok {
		release = registration.release
		delete(nativeAttachmentCallbacks, id)
	}
	nativeAttachmentCallbacksMu.Unlock()

	if release != nil {
		release()
	}
}
