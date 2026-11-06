package woxview

import (
	"slices"
	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// richTextState owns inline feedback by document node identity. All wrapped
// fragments share this state; selection and pointer routing remain in the editor.
type richTextState struct{ spans map[string]*nodeVisualState }

func (s *richTextState) InitState(widget.StateContext, any) { s.spans = map[string]*nodeVisualState{} }
func (*richTextState) Dispose()                             {}

// DidUpdateWidget forgets removed keys and removed observers, including focus
// changes that happened while a span had no state-style props.
func (s *richTextState) DidUpdateWidget(_ widget.StateContext, _, next any) {
	c := next.(nodeVisualConfig)
	live := map[string]bool{}
	for i, child := range c.node.Children {
		id := nodeID(c.id, child, i)
		live[id] = true
		if state := s.spans[id]; state != nil {
			props := child.Props.(*SpanProps)
			state.receiveEnter(c.builder.view, props.CommonProps)
			if props.InteractionStyles.empty() {
				state.hovered, state.pressed, state.focused = false, false, false
			}
		}
	}
	for id := range s.spans {
		if !live[id] {
			delete(s.spans, id)
		}
	}
}

// Build resolves immutable run paints and connects fragment observations to the
// paragraph state so one update repaints selectable and nonselectable runs alike.
func (s *richTextState) Build(ctx widget.StateContext, value any) widget.Widget {
	c := value.(nodeVisualConfig)
	b, p := c.builder, c.node.Props.(*TextProps)
	theme := b.theme()
	style := p.Style.resolve(theme)
	runs := b.richTextRuns(c.node, c.id)
	patches := make([]StateStyle, len(c.node.Children))
	for i, child := range c.node.Children {
		props := child.Props.(*SpanProps)
		run := &runs[i+1]
		state := s.spans[run.ID]
		if state == nil {
			state = &nodeVisualState{}
			state.receiveEnter(b.view, props.CommonProps)
			s.spans[run.ID] = state
		}
		if props.InteractionStyles.empty() {
			continue
		}
		disabled := props.OnPress != nil && run.OnPress == nil
		if disabled {
			state.pressed = false
		}
		patch := state.patch(props.InteractionStyles, disabled)
		patches[i] = patch
		if patch.Color != nil {
			run.Color = patch.Color.resolve(theme, run.Color)
		}
		if patch != (StateStyle{}) {
			run.DecoratePaint = inlineStatePaint(patch, theme)
		}
		change := func(current *bool, next bool) {
			if *current != next {
				ctx.SetState(func() { *current = next })
			}
		}
		run.OnHoverChange = func(next bool) { change(&state.hovered, next) }
		run.OnPressChange = func(next bool) { change(&state.pressed, next && !disabled) }
		decorate, id := run.Decorate, run.ID
		run.Decorate = func(content widget.Widget) widget.Widget {
			return widget.FocusObserver{Key: widget.Key(id + "/style-focus"), OnChange: func(next bool) { change(&state.focused, next) }, Child: decorate(content)}
		}
	}
	for i := range c.node.Children {
		runs[i+1].DecoratePaint = b.captureInline(runs[i+1].ID, runs[i+1].DecoratePaint)
		id, view := runs[i+1].ID, b.view
		owner := view.nodes[id]
		if exit := c.node.Children[i].Props.(*SpanProps).Exit; exit != nil && exit.duration() > 0 {
			runs[i+1].OnSelectionPaint = func(paint *ui.PaintSnapshot) { view.capturePaint(id, owner, paint) }
		}
	}
	// Each inline node owns one timeline, even when wrapping produces many fragments.
	var animate func(int, []component.RichTextRun) widget.Widget
	animate = func(index int, current []component.RichTextRun) widget.Widget {
		if index == len(c.node.Children) {
			return component.WoxRichText(component.RichTextProps{ID: c.id, Runs: current, Style: style.textStyle(component.ControlTheme{}, 14), Color: style.Color.resolve(theme, theme.Text),
				LineHeight: style.LineHeight, MaxLines: style.MaxLines, Centered: style.TextAlign == TextAlignCenter, Selectable: p.Selectable, Window: b.props.Window, Theme: theme})
		}
		props := c.node.Children[index].Props.(*SpanProps)
		state := s.spans[current[index+1].ID]
		if props.Transition == nil && props.Animation == nil && state.enter == nil {
			return animate(index+1, current)
		}
		return b.transitionStyle(patches[index].apply(Style{}), props.Transition, current[index+1].ID, func(sample Style) widget.Widget {
			return state.enterStyle(ctx, &b, current[index+1].ID, sample, func(sample Style) widget.Widget {
				return b.loopAnimation(props.Animation, current[index+1].ID, func(progress float32) widget.Widget {
					next := slices.Clone(current)
					patch := patches[index]
					animated := animationStyle(sample, props.Animation, progress)
					patch.Opacity, patch.BackgroundColor = animated.Opacity, &animated.BackgroundColor
					base := b.captureInline(current[index+1].ID, inlineStatePaint(patch, theme))
					next[index+1].DecoratePaint = inlineAnimationPaint(base, props.Animation, progress, theme.Dark, animated.transformMatrix(theme))
					return animate(index+1, next)
				})
			})
		})
	}
	return animate(0, runs)
}

// inlineStatePaint reuses widget box painting around each measured fragment.
// Selection keeps its native highlight and text color; opacity still applies to
// the glyphs. State backgrounds/borders cannot obscure the selected range.
func inlineStatePaint(patch StateStyle, theme component.ControlTheme) func(*ui.DisplayList, ui.Rect, bool, func()) {
	style := patch.apply(Style{}).resolve(theme)
	box := style.box(theme, nil)
	return func(list *ui.DisplayList, bounds ui.Rect, selected bool, draw func()) {
		if style.Opacity != nil && *style.Opacity < 1 {
			if *style.Opacity <= 0 {
				return
			}
			list.BeginOpacity(bounds, *style.Opacity)
			defer list.EndOpacity(bounds, *style.Opacity)
		}
		if !selected {
			box.PaintBackground(list, bounds)
		}
		draw()
		if !selected {
			box.PaintForeground(list, bounds)
		}
	}
}
