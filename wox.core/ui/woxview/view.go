package woxview

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	component "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	widget "wox/ui/widget"
	"wox/util"
)

// BuildProps carries everything a View needs from the embedding feature. woxview never reads
// global theme or window state, so the same tree can render in windows, settings, or previews.
type BuildProps struct {
	Width float32
	// Height <= 0 lets the content choose its natural height (for example inside an outer scroll).
	Height float32
	Theme  component.ControlTheme
	// Window enables text measurement, selection, and IME for text fields and Markdown. It may be nil in tests.
	Window *woxui.Window
	// IDPrefix namespaces retained widget state when several views share one Host.
	IDPrefix string
	// OnEvent receives producer callbacks; args is always a JSON array.
	OnEvent func(handler HandlerID, args json.RawMessage)
	// OnNodeEvent includes node identity. When set it takes precedence over OnEvent.
	OnNodeEvent func(Event)
	OnOpenLink  func(url string)
	// Translate resolves i18n: display strings in the embedding feature's locale.
	Translate func(string) string
}

// Event identifies the keyed node which triggered a producer callback.
type Event struct {
	Handler     HandlerID
	Args        json.RawMessage
	Key         string
	Coalescible bool `json:"-"`
}

// View retains one woxview surface: the current tree and decoded images. Widget-local state such
// as text-field editing survives tree updates through node keys. It is confined to the UI thread.
type View struct {
	tree           *Tree
	theme          component.ControlTheme
	images         imageCache
	fieldValues    map[string]any
	fieldStates    map[string]retainedField
	scrolls        map[string]*widget.ScrollController
	commands       []*pendingCommand
	changes        map[string]*pendingTextChange
	scheduleChange func(time.Duration, func()) func()
	frameContext   widget.StateContext
	prefix         string
	nodes          map[string]*nodeLifetime
	captures       map[string]*exitCapture
	exits          map[string]*exitingNode
	hasExit        bool
	captureOrder   int
	exitBatch      uint64
	eventEpoch     uint64
}

// NewView creates an empty view; Build returns an empty widget until SetTree is called.
func NewView() *View {
	return &View{
		images:      imageCache{entries: map[imageCacheKey]*imageCacheEntry{}},
		fieldValues: map[string]any{}, fieldStates: map[string]retainedField{},
		scrolls: map[string]*widget.ScrollController{}, changes: map[string]*pendingTextChange{},
		scheduleChange: scheduleTextChange, prefix: "woxview",
		nodes: map[string]*nodeLifetime{}, captures: map[string]*exitCapture{}, exits: map[string]*exitingNode{},
	}
}

// SetTree replaces the rendered tree. Retained state is matched by node identity (key or position).
func (v *View) SetTree(tree *Tree) {
	v.reconcilePresence(tree)
	v.tree = tree
	live := map[string]bool{}
	var walk func(*Node, string)
	walk = func(n *Node, id string) {
		if field, ok := n.Props.(fieldOwner); ok {
			live[id] = true
			if state := v.fieldStates[id]; state != nil {
				v.fieldValues[id] = state.receiveField(field)
			}
		}
		for i, child := range n.Children {
			walk(child, nodeID(id, child, i))
		}
	}
	if tree != nil && tree.Root != nil {
		walk(tree.Root, nodeID(v.prefix, tree.Root, 0))
	}
	for id := range v.fieldValues {
		if !live[id] {
			delete(v.fieldValues, id)
			delete(v.fieldStates, id)
		}
	}
	v.reconcileChanges()
}

// Tree returns the current tree, which callers must treat as immutable.
func (v *View) Tree() *Tree { return v.tree }

// HasHandler reports whether the current tree still owns a queued callback.
func (v *View) HasHandler(id HandlerID) bool {
	if v.tree == nil || id == "" {
		return false
	}
	_, ok := v.tree.handlerSet[id]
	return ok
}

// Build compiles the current tree into widgets sized to props.Width and props.Height.
func (v *View) Build(props BuildProps) widget.Widget {
	v.theme = props.Theme
	prefix := props.IDPrefix
	if prefix == "" {
		prefix = "woxview"
	}
	if prefix != v.prefix {
		// A different namespace also creates fresh retained widgets in the Host.
		v.cancelChanges()
		clear(v.fieldValues)
		clear(v.fieldStates)
		clear(v.scrolls)
		// Commands accepted before the first frame use the default namespace.
		for i := range v.commands {
			v.commands[i].id = prefix + strings.TrimPrefix(v.commands[i].id, v.prefix)
		}
		v.prefix = prefix
		v.clearPresence()
		for _, owner := range v.nodes {
			owner.live = false
		}
		clear(v.nodes)
		v.reconcilePresence(v.tree)
	}
	b := &builder{view: v, props: props}
	v.images.beginBuild()
	var root widget.Widget = widget.Container{}
	if v.tree != nil && v.tree.Root != nil {
		root = b.node(v.tree.Root, nodeID(prefix, v.tree.Root, 0))
	}
	root = widget.Stateful{Key: widget.Key(prefix + "/frame"), Type: (*viewFrameState)(nil),
		Widget: viewFrameConfig{view: v, tree: v.tree, child: root, theme: props.Theme}, CreateState: func() widget.State { return &viewFrameState{} }}
	// The root fills the surface like a React Native root view; unbounded heights stay natural.
	return widget.Container{Width: props.Width, Height: props.Height, Child: widget.Constrained{FillWidth: true, FillHeight: props.Height > 0, Child: root}}
}

type viewFrameConfig struct {
	view  *View
	tree  *Tree
	child widget.Widget
	theme component.ControlTheme
}

// viewFrameState completes image accounting after lazy builders have run during layout.
type viewFrameState struct {
	view    *View
	context widget.StateContext
}

func (s *viewFrameState) InitState(ctx widget.StateContext, config any) {
	s.view = config.(viewFrameConfig).view
	s.context = ctx
	s.view.frameContext = ctx
}
func (s *viewFrameState) DidUpdateWidget(ctx widget.StateContext, _, next any) {
	view := next.(viewFrameConfig).view
	if s.view != view {
		s.view.CancelPending()
		s.view = view
	}
	view.frameContext = ctx
	s.context = ctx
}

// Dispose must not revoke a replacement frame owner when IDPrefix changes in
// the same Host frame: the new owner is mounted before the old one is disposed.
func (s *viewFrameState) Dispose() {
	if s.view.frameContext == s.context {
		s.view.CancelPending()
	}
}
func (*viewFrameState) Build(ctx widget.StateContext, value any) widget.Widget {
	c := value.(viewFrameConfig)
	ctx.PostFrame(c.view.images.endBuild)
	ctx.PostFrame(func() {
		if c.view.tree == c.tree {
			c.view.flushCommands(ctx)
		}
	})
	return c.view.exitSurface(ctx, c.theme, c.child)
}

// builder holds per-Build context. Callbacks capture only immutable props and the stable emit path.
type builder struct {
	view       *View
	props      BuildProps
	form       *formContext
	inScroll   bool
	nodeKey    string
	isRoot     bool
	nodeOwner  *nodeLifetime
	eventOwner *nodeLifetime
}

// translate leaves literal text and protocol data unchanged; hosts own lookup and fallback.
func (b *builder) translate(text string) string {
	if b.props.Translate != nil && strings.HasPrefix(text, "i18n:") {
		return b.props.Translate(text)
	}
	return text
}

func (b *builder) theme() component.ControlTheme { return b.props.Theme }

func (b *builder) node(node *Node, id string) widget.Widget {
	spec, ok := catalog[node.Type]
	if !ok || spec.build == nil {
		return widget.Container{}
	}
	local := *b
	local.nodeKey = node.Key
	local.nodeOwner = b.view.nodes[id]
	local.eventOwner = local.nodeOwner
	local.isRoot = node == b.view.tree.Root
	// Keep the state boundary stable when producers add or remove state-style props.
	return b.captureNode(id, widget.Stateful{Key: widget.Key(id + "/visual-state"), Type: (*nodeVisualState)(nil),
		Widget: nodeVisualConfig{builder: local, node: node, id: id}, CreateState: func() widget.State { return &nodeVisualState{} }})
}

// decorateNode gives block and inline nodes the same interaction and identity contract.
func (b *builder) decorateNode(node *Node, id string, content widget.Widget) widget.Widget {
	if p, ok := node.Props.(interface{ commonProps() CommonProps }); ok {
		common := p.commonProps()
		if len(common.Shortcuts) > 0 {
			content = b.shortcuts(id, common.Shortcuts, b.isRoot, content)
		}
		content = b.interactions(id, common, content)
		if len(common.ContextMenu) > 0 {
			content = b.contextMenu(id, common.ContextMenu, content)
		}
		if common.Tooltip != "" {
			content = component.WoxTooltip(component.TooltipProps{Key: widget.Key(id + "/tooltip"), Message: b.translate(common.Tooltip), Theme: b.theme(), Child: content})
		}
	}
	if node.Key != "" {
		content = widget.Target{Key: widget.Key(id + "/target"), Child: content}
	}
	if opacity := styleOf(node).Opacity; opacity != nil {
		content = widget.Opacity{Value: *opacity, Child: content}
	}
	return styleOf(node).transform(b.theme(), content)
}

// nodeID derives a stable retained-state identity from the parent path and the node key,
// falling back to type and position so unkeyed siblings keep state while the list is stable.
func nodeID(parent string, node *Node, index int) string {
	if node.Key != "" {
		return parent + "/k:" + url.PathEscape(node.Key) + "/" + string(node.Type)
	}
	return fmt.Sprintf("%s/%s%d", parent, node.Type, index)
}

// emit returns a callback for handler, or nil when the producer did not register one.
func (b *builder) emit(handler *Handler) func(args ...any) {
	return b.eventEmitter(handler, false)
}

// emitChange marks value changes as replaceable while preserving discrete event ordering.
func (b *builder) emitChange(handler *Handler) func(args ...any) {
	return b.eventEmitter(handler, true)
}

// eventEmitter binds the immutable handler and node metadata for one tree build.
func (b *builder) eventEmitter(handler *Handler, coalescible bool) func(args ...any) {
	if handler == nil || (b.props.OnEvent == nil && b.props.OnNodeEvent == nil) {
		return nil
	}
	onEvent, id := b.props.OnEvent, handler.ID
	onNodeEvent, key := b.props.OnNodeEvent, b.nodeKey
	view, epoch, source, owner := b.view, b.view.eventEpoch, b.nodeOwner, b.eventOwner
	return func(args ...any) {
		// Handler IDs may be shared by several nodes. Revoking this node must
		// reject its old callbacks even when another node still owns that ID.
		if source == nil || owner == nil || !source.live || !owner.live || epoch != view.eventEpoch || !owner.handlers[id] {
			return
		}
		if args == nil {
			args = []any{}
		}
		data, err := json.Marshal(args)
		if err != nil {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("woxview: failed to encode event args: %v", err))
			return
		}
		if onNodeEvent != nil {
			onNodeEvent(Event{Handler: id, Args: data, Key: key, Coalescible: coalescible})
		} else {
			onEvent(id, data)
		}
	}
}

// tap adapts emit to a no-argument tap callback.
func (b *builder) tap(handler *Handler) func() {
	emit := b.emit(handler)
	if emit == nil {
		return nil
	}
	return func() { emit() }
}

// children builds natural-size items for non-flex layouts such as Wrap and Grid.
func (b *builder) children(node *Node, id string) []widget.Widget {
	children := make([]widget.Widget, 0, len(node.Children))
	for index, child := range node.Children {
		children = append(children, b.node(child, nodeID(id, child, index)))
	}
	return children
}

// flexChildren builds children for a Flex, wrapping flex-weighted children in Expanded.
// A spacer without a fixed size behaves like flex: 1, matching SwiftUI's Spacer.
func (b *builder) flexChildren(node *Node, id string) []widget.Widget {
	children := make([]widget.Widget, 0, len(node.Children))
	for index, child := range node.Children {
		built := b.node(child, nodeID(id, child, index))
		style := styleOf(child)
		flex := style.Flex
		if spacer, ok := child.Props.(*SpacerProps); ok && flex == 0 && spacer.Size == (Size{}) {
			flex = 1
		}
		if flex > 0 {
			built = widget.Expanded{Flex: flex, Child: built}
		}
		if style.AlignSelf != "" {
			built = widget.AlignSelf{Alignment: (Style{AlignItems: style.AlignSelf}).crossAxis(), Child: built}
		}
		children = append(children, built)
	}
	return children
}

// widthOr returns the explicit style width, otherwise the available width from the layout pass.
func widthOr(style resolvedStyle, available woxui.Size) float32 {
	if style.Width > 0 {
		return style.Width
	}
	if available.Width >= math.MaxFloat32 {
		return 0
	}
	return available.Width
}
