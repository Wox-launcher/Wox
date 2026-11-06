package component

import (
	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// ChoiceGroupProps presents equal-width segments or wrapping selectable tags.
// The caller owns selection; navigation only moves focus until activation.
type ChoiceGroupProps struct {
	ID    string
	Label string
	Items []MenuItem
	Theme ControlTheme
	Chips bool
	Paint *ControlPaint
}

func WoxChoiceGroup(p ChoiceGroupProps) widget.Widget {
	return widget.Semantics{AutomationID: p.ID, Role: ui.AccessibilityRoleGroup, Label: p.Label,
		Child: widget.Stateful{Key: widget.Key(p.ID), Type: (*choiceGroupState)(nil), Widget: p, CreateState: func() widget.State { return &choiceGroupState{} }}}
}

type choiceGroupState struct{}

func (*choiceGroupState) InitState(widget.StateContext, any)            {}
func (*choiceGroupState) DidUpdateWidget(widget.StateContext, any, any) {}
func (*choiceGroupState) Dispose()                                      {}

// Build shares enabled-item navigation with menus and keeps all sizes in logical units.
func (*choiceGroupState) Build(ctx widget.StateContext, value any) widget.Widget {
	p := value.(ChoiceGroupProps)
	children := make([]widget.Widget, 0, len(p.Items))
	for _, item := range p.Items {
		onKey := func(e ui.KeyEvent) bool {
			if e.Key == ui.KeyArrowLeft {
				e.Key = ui.KeyArrowUp
			} else if e.Key == ui.KeyArrowRight {
				e.Key = ui.KeyArrowDown
			}
			next, handled := menuNavigation(p.Items, item.ID, e)
			if next != "" {
				ctx.RequestFocus(widget.Key(next))
			}
			return handled
		}
		if p.Chips {
			foreground := p.Theme.Text
			if item.Selected {
				foreground = p.Theme.SelectionText
			}
			if item.Disabled {
				foreground = p.Theme.TextSecondary
			}
			foreground = p.Paint.foreground(foreground)
			// Reserve the check slot even when unselected so toggling cannot move adjacent tags.
			var check widget.Widget = widget.Container{Width: p.Theme.Scaled(16), Height: p.Theme.Scaled(16)}
			if item.Selected {
				check = IconGlyph("control.check", p.Theme.Scaled(16), foreground)
			}
			children = append(children, WoxListItem(ListItemProps{
				ID: item.ID, Label: item.Label, Height: p.Theme.Scaled(SettingsControlHeight),
				Selected: item.Selected, Disabled: item.Disabled, Theme: p.Theme, OnTap: item.OnSelect, OnKey: onKey, Paint: p.Paint.choicePaint(item.Selected),
				Padding: widget.Insets{Left: p.Theme.Scaled(4), Right: p.Theme.Scaled(4)},
				Child: widget.Flex{Axis: widget.Horizontal, Gap: p.Theme.Scaled(4), CrossAxisAlignment: widget.CrossAxisCenter,
					Children: []widget.Widget{check, WoxTagWithPaint(item.Label, foreground, p.Theme, false, p.Paint.choicePaint(item.Selected).withPart(item.ID))}},
			}))
		} else {
			children = append(children, widget.Expanded{Child: widget.LayoutBuilder{Build: func(available ui.Size) widget.Widget {
				return WoxSegmentedButton(SegmentedButtonProps{ID: item.ID, Label: item.Label, Width: available.Width,
					Selected: item.Selected, Disabled: item.Disabled, Theme: p.Theme, OnTap: item.OnSelect, OnKey: onKey, Paint: p.Paint.choicePaint(item.Selected)})
			}}})
		}
	}
	if p.Chips {
		return widget.Wrap{Gap: p.Theme.Scaled(4), RunGap: p.Theme.Scaled(4), Children: children}
	}
	return widget.Flex{Axis: widget.Horizontal, Gap: p.Theme.Scaled(4), Children: children}
}
