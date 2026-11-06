package woxview

import (
	"errors"

	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// LazyListProps defines fixed logical row extents; only visible keyed items are built.
type LazyListProps struct {
	BoxProps
	ItemHeight   Size     `json:"itemHeight"`
	StickToEnd   bool     `json:"stickToEnd,omitempty"`
	OnEndReached *Handler `json:"onEndReached,omitempty"`
}

func (p *LazyListProps) handlers() []namedHandler {
	return append(p.Styled.handlers(), []namedHandler{{"onPress", p.OnPress}, {"onEndReached", p.OnEndReached}}...)
}

// prepare requires a usable fixed extent so layout never measures the entire collection.
func (p *LazyListProps) prepare() error {
	if p.ItemHeight.validate() != nil || p.ItemHeight.base() <= 0 {
		return errors.New("itemHeight must be greater than 0 and at most 16384")
	}
	return p.Style.validate()
}

// buildLazyList shares scrolling and edge notification policy with ordinary scroll nodes.
func buildLazyList(b *builder, node *Node, id string) widget.Widget {
	p := node.Props.(*LazyListProps)
	children := *b
	children.inScroll = true
	content := widget.LazyList{
		Key: widget.Key(id + "/items"), ItemCount: len(node.Children), ItemExtent: p.ItemHeight.resolve(b.theme()),
		ItemKey: func(index int) widget.Key { return widget.Key(node.Children[index].Key) },
		ItemBuilder: func(index int) widget.Widget {
			child := node.Children[index]
			// Constrain and clip each row to the declared extent so measured content
			// cannot diverge from the index used to find visible items.
			return widget.LayoutBuilder{Build: func(available ui.Size) widget.Widget {
				return widget.Clip{Width: available.Width, Height: p.ItemHeight.resolve(b.theme()), Child: children.node(child, nodeID(id, child, index))}
			}}
		},
	}
	scroll := &ScrollProps{BoxProps: p.BoxProps, StickToEnd: p.StickToEnd, OnEndReached: p.OnEndReached}
	config := scrollConfig{view: b.view, props: scroll, id: id, content: content, items: len(node.Children), theme: b.theme(), nested: b.inScroll, onEnd: b.tap(p.OnEndReached)}
	style := p.Style.resolve(b.theme())
	if style.AspectRatio == nil {
		style.Width, style.Height = 0, 0
	}
	return pressable(b, id, &p.BoxProps, style.decorate(b.theme(), widget.Stateful{
		Key: widget.Key(id + "/scroll-state"), Type: (*scrollState)(nil), Widget: config,
		CreateState: func() widget.State { return &scrollState{} },
	}))
}
