package widget

import (
	"fmt"
	"reflect"
	"sync/atomic"
)

// State owns mutable data and lifecycle hooks for one retained Stateful widget.
type State interface {
	InitState(context StateContext, widget any)
	DidUpdateWidget(context StateContext, oldWidget, newWidget any)
	Build(context StateContext, widget any) Widget
	Dispose()
}

// StateContext connects retained state to its owning Host without exposing element internals.
type StateContext struct {
	element *stateElement
}

// Mounted reports whether the state still belongs to a live element tree.
func (c StateContext) Mounted() bool {
	return c.element != nil && c.element.mounted.Load()
}

// Invalidate schedules a new frame for the stateful widget's owning Host.
func (c StateContext) Invalidate() {
	if !c.Mounted() || c.element.tree == nil || c.element.tree.host == nil {
		return
	}
	c.element.tree.host.invalidate()
}

// SetState applies a local mutation and schedules a new frame while the element remains mounted.
func (c StateContext) SetState(update func()) {
	if !c.Mounted() {
		return
	}
	if update != nil {
		update()
	}
	c.Invalidate()
}

// BindFocusNode attaches a stable focus handle to this element's window-specific Host.
func (c StateContext) BindFocusNode(node *FocusNode, key Key) *FocusAttachment {
	if node == nil || !c.Mounted() || c.element.tree == nil {
		return nil
	}
	host := c.element.tree.host
	node.attach(host, key)
	return &FocusAttachment{node: node, host: host, key: key}
}

// RequestFocus asks the owning Host to focus a laid-out descendant by stable key.
func (c StateContext) RequestFocus(key Key) bool {
	return c.Mounted() && c.element.tree != nil && c.element.tree.host != nil && c.element.tree.host.RequestFocus(key)
}

// PostFrame schedules retained work after the current layout, focus reconciliation, and paint pass.
func (c StateContext) PostFrame(callback func()) {
	if callback == nil || !c.Mounted() || c.element.tree == nil || c.element.tree.host == nil {
		return
	}
	c.element.tree.host.postFrame = append(c.element.tree.host.postFrame, callback)
}

// Stateful retains State by widget type and Key while its immutable configuration is rebuilt.
type Stateful struct {
	Key Key
	// Type supplies stable element identity when multiple widgets share one config type.
	Type        any
	Widget      any
	CreateState func() State
}

func (w Stateful) layout(ctx context, available constraints) *node {
	if ctx.elements == nil {
		return w.layoutEphemeral(ctx, available)
	}
	if w.Key == "" || w.CreateState == nil {
		ctx.elements.diagnostics = append(ctx.elements.diagnostics, "stateful widgets require a key and state factory")
		return &node{key: w.Key, kind: "stateful"}
	}
	element := ctx.elements.reconcile(ctx.element, w)
	if element == nil || element.state == nil {
		return &node{key: w.Key, kind: "stateful"}
	}
	child := element.state.Build(StateContext{element: element}, w.Widget)
	if child == nil {
		return &node{key: w.Key, kind: "stateful"}
	}
	childNode := child.layout(ctx.withElement(element), available)
	if childNode.key == "" {
		childNode.key = w.Key
	}
	if childNode.kind == "" {
		childNode.kind = "stateful"
	}
	return childNode
}

func (w Stateful) layoutEphemeral(ctx context, available constraints) *node {
	if w.CreateState == nil {
		return &node{key: w.Key, kind: "stateful"}
	}
	state := w.CreateState()
	if state == nil {
		return &node{key: w.Key, kind: "stateful"}
	}
	temporary := &stateElement{tree: ctx.elements, key: w.Key, widget: w.Widget, state: state}
	temporary.mounted.Store(true)
	stateContext := StateContext{element: temporary}
	state.InitState(stateContext, w.Widget)
	child := state.Build(stateContext, w.Widget)
	if child == nil {
		state.Dispose()
		temporary.mounted.Store(false)
		return &node{key: w.Key, kind: "stateful"}
	}
	childNode := child.layout(ctx, available)
	state.Dispose()
	temporary.mounted.Store(false)
	return childNode
}

type elementIdentity struct {
	key        Key
	occurrence int
}

type stateElement struct {
	tree       *elementTree
	parent     *stateElement
	key        Key
	widgetType reflect.Type
	widget     any
	state      State
	children   map[elementIdentity]*stateElement
	seenKeys   map[Key]int
	seenAt     uint64
	preparedAt uint64
	mounted    atomic.Bool
}

type elementTree struct {
	host        *Host
	root        *stateElement
	generation  uint64
	diagnostics []string
}

// newElementTree creates the retained state root owned by one Host window.
func newElementTree(host *Host) *elementTree {
	tree := &elementTree{host: host}
	tree.root = &stateElement{tree: tree, children: map[elementIdentity]*stateElement{}, seenKeys: map[Key]int{}}
	tree.root.mounted.Store(true)
	return tree
}

func (t *elementTree) beginFrame() {
	t.generation++
	t.diagnostics = nil
	t.root.seenAt = t.generation
}

// reconcile reuses state for the same keyed widget type and replaces incompatible state.
func (t *elementTree) reconcile(parent *stateElement, widget Stateful) *stateElement {
	if parent == nil {
		parent = t.root
	}
	if parent.preparedAt != t.generation {
		parent.preparedAt = t.generation
		clear(parent.seenKeys)
	}
	occurrence := parent.seenKeys[widget.Key]
	parent.seenKeys[widget.Key] = occurrence + 1
	if occurrence > 0 {
		t.diagnostics = append(t.diagnostics, fmt.Sprintf("duplicate stateful widget key %q under %q", widget.Key, parent.key))
	}
	identity := elementIdentity{key: widget.Key, occurrence: occurrence}
	widgetType := reflect.TypeOf(widget.Type)
	if widgetType == nil {
		widgetType = reflect.TypeOf(widget.Widget)
	}
	element := parent.children[identity]
	if element != nil && element.widgetType != widgetType {
		t.disposeElement(element)
		delete(parent.children, identity)
		element = nil
	}
	if element == nil {
		state := widget.CreateState()
		if state == nil {
			t.diagnostics = append(t.diagnostics, fmt.Sprintf("stateful widget %q returned nil state", widget.Key))
			return nil
		}
		element = &stateElement{
			tree: t, parent: parent, key: widget.Key, widgetType: widgetType, widget: widget.Widget, state: state,
			children: map[elementIdentity]*stateElement{}, seenKeys: map[Key]int{}, seenAt: t.generation,
		}
		element.mounted.Store(true)
		parent.children[identity] = element
		state.InitState(StateContext{element: element}, widget.Widget)
		return element
	}
	oldWidget := element.widget
	element.widget = widget.Widget
	element.seenAt = t.generation
	element.state.DidUpdateWidget(StateContext{element: element}, oldWidget, widget.Widget)
	return element
}

func (t *elementTree) endFrame() []string {
	t.sweep(t.root)
	return append([]string(nil), t.diagnostics...)
}

// sweep disposes retained subtrees that were not rebuilt in the current frame.
func (t *elementTree) sweep(parent *stateElement) {
	for identity, child := range parent.children {
		if child.seenAt != t.generation {
			t.disposeElement(child)
			delete(parent.children, identity)
			continue
		}
		t.sweep(child)
	}
}

// dispose releases every retained state when its Host window is destroyed.
func (t *elementTree) dispose() {
	for identity, child := range t.root.children {
		t.disposeElement(child)
		delete(t.root.children, identity)
	}
	t.root.mounted.Store(false)
}

func (t *elementTree) disposeElement(element *stateElement) {
	if element == nil || !element.mounted.Load() {
		return
	}
	for identity, child := range element.children {
		t.disposeElement(child)
		delete(element.children, identity)
	}
	element.state.Dispose()
	element.mounted.Store(false)
}
