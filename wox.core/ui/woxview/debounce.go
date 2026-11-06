package woxview

import (
	"time"

	ui "wox/ui/runtime"
)

// pendingTextChange belongs to the document rather than the visible field, so
// virtualization cannot silently lose an edit waiting for its trailing edge.
type pendingTextChange struct {
	value    string
	handler  HandlerID
	revision uint64
	delay    int
	emit     func(args ...any)
	stop     func()
}

// scheduleTextChange only posts from the timer goroutine. All retained state and
// producer callbacks stay on the UI thread, including cancellation races.
func scheduleTextChange(delay time.Duration, callback func()) func() {
	timer := time.AfterFunc(delay, func() { _ = ui.Post(callback) })
	return func() { timer.Stop() }
}

// textChanged updates only delivery timing; the editor and form store have
// already received the edit synchronously before this method is called.
func (v *View) textChanged(c textFieldConfig, text string) {
	v.cancelChange(c.id)
	if c.onChange == nil {
		return
	}
	if c.props.ChangeDebounce == 0 {
		c.onChange(text)
		return
	}
	pending := &pendingTextChange{value: text, handler: c.props.OnChange.ID, revision: c.props.Revision, delay: c.props.ChangeDebounce, emit: c.onChange}
	v.changes[c.id] = pending
	pending.stop = v.scheduleChange(time.Duration(pending.delay)*time.Millisecond, func() {
		// Stop may race an already-posted timer callback. Identity, not timer
		// cancellation alone, decides whether this edit is still deliverable.
		if v.changes[c.id] == pending {
			v.flushChange(c.id)
		}
	})
}

// cancelChange invalidates the identity before stopping a possibly posted timer.
func (v *View) cancelChange(id string) {
	if pending := v.changes[id]; pending != nil {
		delete(v.changes, id)
		if pending.stop != nil {
			pending.stop()
		}
	}
}

// flushChange emits once, before blur or submit can produce a later event.
func (v *View) flushChange(id string) {
	if pending := v.changes[id]; pending != nil {
		v.cancelChange(id)
		pending.emit(pending.value)
	}
}

// flushFormChanges delivers pending edits in tree order before the form snapshot.
func (v *View) flushFormChanges(node *Node, id string) {
	v.flushChange(id)
	for i, child := range node.Children {
		v.flushFormChanges(child, nodeID(id, child, i))
	}
}

// reconcileChanges revokes delayed work on removal, reset, handler replacement,
// delay changes, disabling, or an authoritative value change, even offscreen.
func (v *View) reconcileChanges() {
	if len(v.changes) == 0 {
		return
	}
	live := map[string]bool{}
	var walk func(*Node, string, bool)
	walk = func(node *Node, id string, disabled bool) {
		if form, ok := node.Props.(*FormProps); ok {
			disabled = disabled || form.Disabled
		}
		if pending := v.changes[id]; pending != nil {
			if field, ok := node.Props.(*TextFieldProps); ok {
				live[id] = !disabled && !field.Disabled && field.OnChange != nil && field.OnChange.ID == pending.handler && field.Revision == pending.revision && field.ChangeDebounce == pending.delay && v.fieldValues[id] == pending.value
			}
		}
		for i, child := range node.Children {
			walk(child, nodeID(id, child, i), disabled)
		}
	}
	if v.tree != nil && v.tree.Root != nil {
		walk(v.tree.Root, nodeID(v.prefix, v.tree.Root, 0), false)
	}
	for id := range v.changes {
		if !live[id] {
			v.cancelChange(id)
		}
	}
}

func (v *View) cancelChanges() {
	for id := range v.changes {
		v.cancelChange(id)
	}
}

// CancelPending releases document-owned deferred work on replacement or unmount.
func (v *View) CancelPending() {
	v.cancelChanges()
	v.commands = nil
	v.clearPresence()
}
