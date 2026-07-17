package widget

import "sync"

// FocusNode is a stable, window-scoped handle for requesting and observing widget focus.
type FocusNode struct {
	mu      sync.RWMutex
	host    *Host
	key     Key
	focused bool
}

// FocusAttachment owns one State-to-Host focus binding.
type FocusAttachment struct {
	node *FocusNode
	host *Host
	key  Key
}

// Detach removes this binding without disturbing a node that has since moved to another Host.
func (a *FocusAttachment) Detach() {
	if a == nil || a.node == nil {
		return
	}
	a.node.detach(a.host, a.key)
	a.node = nil
	a.host = nil
	a.key = ""
}

// NewFocusNode creates an unattached focus handle.
func NewFocusNode() *FocusNode {
	return &FocusNode{}
}

func (n *FocusNode) attach(host *Host, key Key) {
	if n == nil {
		return
	}
	n.mu.Lock()
	n.host = host
	n.key = key
	n.focused = host != nil && host.isFocusedKey(key)
	n.mu.Unlock()
}

func (n *FocusNode) detach(host *Host, key Key) {
	if n == nil {
		return
	}
	n.mu.Lock()
	if n.host == host && n.key == key {
		n.host = nil
		n.key = ""
		n.focused = false
	}
	n.mu.Unlock()
}

// UpdateFocus records a focus transition reported by the Host-owned EditableText.
func (n *FocusNode) UpdateFocus(focused bool) {
	if n == nil {
		return
	}
	n.mu.Lock()
	n.focused = focused
	n.mu.Unlock()
}

// RequestFocus asks the attached Host to focus this node.
func (n *FocusNode) RequestFocus() bool {
	if n == nil {
		return false
	}
	n.mu.RLock()
	host := n.host
	key := n.key
	n.mu.RUnlock()
	return host != nil && key != "" && host.RequestFocus(key)
}

// Unfocus releases focus only when this node still owns the Host focus target.
func (n *FocusNode) Unfocus() {
	if n == nil {
		return
	}
	n.mu.RLock()
	host := n.host
	key := n.key
	n.mu.RUnlock()
	if host != nil && key != "" {
		host.clearFocusForKey(key)
	}
}

// HasFocus reports the latest focus state published by the attached Host.
func (n *FocusNode) HasFocus() bool {
	if n == nil {
		return false
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.focused
}
