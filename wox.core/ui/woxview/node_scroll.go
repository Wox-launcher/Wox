package woxview

import (
	"errors"

	component "wox/ui/launcher/component"
	widget "wox/ui/widget"
)

// ScrollProps defines the scroll axis and optional trailing-edge notifications.
// EndThreshold is measured in logical units along that axis, with zero meaning the exact end.
type ScrollProps struct {
	BoxProps
	Horizontal   bool     `json:"horizontal,omitempty"`
	StickToEnd   bool     `json:"stickToEnd,omitempty"`
	OnEndReached *Handler `json:"onEndReached,omitempty"`
	EndThreshold Size     `json:"endThreshold,omitempty"`
}

func (p *ScrollProps) handlers() []namedHandler {
	return append(p.Styled.handlers(), []namedHandler{{"onPress", p.OnPress}, {"onEndReached", p.OnEndReached}}...)
}

// prepare rejects invalid distances, including Go-built NaN values.
func (p *ScrollProps) prepare() error {
	if p.EndThreshold.validate() != nil {
		return errors.New("endThreshold must be between 0 and 16384")
	}
	return p.Style.validate()
}

type scrollConfig struct {
	view    *View
	props   *ScrollProps
	id      string
	content widget.Widget
	items   int
	theme   component.ControlTheme
	nested  bool
	onEnd   func()
}

// scrollState keeps protocol notifications separate from the widget's scroll mechanics.
// The end notification rearms on leaving the threshold, changed content size/count, or a new handler.
type scrollState struct {
	view       *View
	id         string
	controller *widget.ScrollController
	lastMax    float32
	lastItems  int
	handler    HandlerID
	notified   bool
}

// buildScroll preserves the original vertical layout by default and tracks nesting for wheel routing.
func buildScroll(b *builder, node *Node, id string) widget.Widget {
	p := node.Props.(*ScrollProps)
	children := *b
	children.inScroll = true
	axis := widget.Vertical
	if p.Horizontal {
		axis = widget.Horizontal
	}
	content := widget.Flex{Axis: axis, Gap: p.Style.Gap.resolve(b.theme()), CrossAxisAlignment: p.Style.crossAxis(), Children: children.flexChildren(node, id)}
	config := scrollConfig{view: b.view, props: p, id: id, content: content, items: len(node.Children), theme: b.theme(), nested: b.inScroll, onEnd: b.tap(p.OnEndReached)}
	style := p.Style.resolve(b.theme())
	// Normally the scroll surface owns viewport dimensions. With a ratio, the
	// outer box sets the viewport constraints before scroll geometry is measured.
	if style.AspectRatio == nil {
		style.Width, style.Height = 0, 0
	}
	return pressable(b, id, &p.BoxProps, style.decorate(b.theme(), widget.Stateful{
		Key: widget.Key(id + "/scroll-state"), Type: (*scrollState)(nil), Widget: config,
		CreateState: func() widget.State { return &scrollState{} },
	}))
}

func (s *scrollState) InitState(_ widget.StateContext, value any) {
	c := value.(scrollConfig)
	s.view, s.id = c.view, c.id
	s.controller = widget.NewScrollController(0)
	s.view.scrolls[s.id] = s.controller
	s.lastMax = -1
}

// DidUpdateWidget starts a fresh range when the same scroll node switches axes.
func (s *scrollState) DidUpdateWidget(_ widget.StateContext, old, next any) {
	if old.(scrollConfig).props.Horizontal != next.(scrollConfig).props.Horizontal {
		s.controller = widget.NewScrollController(0)
		s.view.scrolls[s.id] = s.controller
		s.lastMax, s.notified = -1, false
	}
}

func (s *scrollState) Dispose() {
	if s.view.scrolls[s.id] == s.controller {
		delete(s.view.scrolls, s.id)
	}
}

// Build checks the final controller geometry after layout, never invoking producer code inside layout.
func (s *scrollState) Build(ctx widget.StateContext, value any) widget.Widget {
	c := value.(scrollConfig)
	p := c.props
	ctx.PostFrame(func() {
		if !ctx.Mounted() {
			return
		}
		handler := HandlerID("")
		if p.OnEndReached != nil {
			handler = p.OnEndReached.ID
		}
		maxOffset := s.controller.MaxOffset()
		// Extents are clamped to the viewport, so an added page that still fits
		// must rearm by item count or an initially short list could never fill.
		if maxOffset != s.lastMax || c.items != s.lastItems || handler != s.handler {
			s.notified = false
			s.lastMax, s.handler = maxOffset, handler
			s.lastItems = c.items
		}
		if c.onEnd == nil || maxOffset-s.controller.Offset() > p.EndThreshold.resolve(c.theme) {
			s.notified = false
			return
		}
		if !s.notified {
			s.notified = true
			c.onEnd()
		}
	})
	return component.WoxScrollView(component.ScrollViewProps{
		Key: widget.Key(c.id), Theme: c.theme, Content: c.content, EdgeFade: 12,
		Controller: s.controller, RetainState: true, Horizontal: p.Horizontal,
		PreserveVerticalWheel: c.nested, StickToEnd: p.StickToEnd,
		Width: p.Style.Width.resolve(c.theme), Height: p.Style.Height.resolve(c.theme), FillWidth: p.Style.Width.resolve(c.theme) == 0, FillHeight: p.Style.Height.resolve(c.theme) == 0,
	})
}
