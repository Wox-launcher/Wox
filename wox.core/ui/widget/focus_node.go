package widget

import "sync"

// FocusNode is a stable, window-scoped handle for requesting and observing widget focus.
type FocusNode struct {
	mu         sync.RWMutex
	host       *Host
	key        Key
	focused    bool
	attachment *FocusAttachment
}

// FocusAttachment owns one State-to-Host focus binding.
type FocusAttachment struct {
	node *FocusNode
}

// Detach removes only this specific binding so disposing a stale element cannot
// clobber a newer element that already re-attached the same FocusNode.
func (a *FocusAttachment) Detach() {
	if a == nil || a.node == nil {
		return
	}
	a.node.detach(a)
	a.node = nil
}

// NewFocusNode creates an unattached focus handle.
func NewFocusNode() *FocusNode {
	return &FocusNode{}
}

func (n *FocusNode) attach(host *Host, key Key) *FocusAttachment {
	if n == nil {
		return nil
	}
	attachment := &FocusAttachment{node: n}
	n.mu.Lock()
	n.attachment = attachment
	n.host = host
	n.key = key
	n.focused = host != nil && host.isFocusedKey(key)
	n.mu.Unlock()
	return attachment
}

func (n *FocusNode) detach(attachment *FocusAttachment) {
	if n == nil || attachment == nil {
		return
	}
	n.mu.Lock()
	// Reparenting mounts the replacement state before sweeping the old element;
	// ignoring stale attachments keeps the live binding and focus state intact.
	if n.attachment == attachment {
		n.attachment = nil
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

// MoveFocus asks the attached Host to focus the previous or next control.
func (n *FocusNode) MoveFocus(reverse bool) bool {
	if n == nil {
		return false
	}
	n.mu.RLock()
	host := n.host
	key := n.key
	n.mu.RUnlock()
	if host == nil || key == "" || !host.isFocusedKey(key) {
		return false
	}
	return host.moveFocus(reverse)
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
