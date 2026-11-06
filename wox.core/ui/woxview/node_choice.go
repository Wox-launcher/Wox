package woxview

import (
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// ChoiceOption is a stable selectable value in a choice control.
type ChoiceOption struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Disabled bool   `json:"disabled,omitempty"`
}

// ChoiceProps defines a single-select list, dropdown, or radio group.
type ChoiceProps struct {
	Styled
	FieldProps
	Name     string         `json:"name,omitempty"`
	Label    string         `json:"label,omitempty"`
	Value    string         `json:"value,omitempty"`
	Options  []ChoiceOption `json:"options"`
	Disabled bool           `json:"disabled,omitempty"`
	OnChange *Handler       `json:"onChange,omitempty"`
}

func (p *ChoiceProps) fieldName() string { return p.Name }
func (p *ChoiceProps) initialValue() any { return p.Value }
func (p *ChoiceProps) handlers() []namedHandler {
	return append(p.Styled.handlers(), []namedHandler{{"onChange", p.OnChange}}...)
}

// normalizeFieldValue discards a removed option even while its field is offscreen.
func (p *ChoiceProps) normalizeFieldValue(value any) any {
	if value == "" {
		return value
	}
	for _, option := range p.Options {
		if option.Value == value {
			return value
		}
	}
	return p.Value
}

func (p *ChoiceProps) prepare() error {
	if len(p.Options) > 500 {
		return errors.New("options exceeds 500")
	}
	seen := map[string]bool{}
	for _, o := range p.Options {
		if o.Value == "" || seen[o.Value] {
			return fmt.Errorf("option values must be nonempty and unique: %q", o.Value)
		}
		seen[o.Value] = true
	}
	if p.Value != "" && !seen[p.Value] {
		return fmt.Errorf("value %q is not an option", p.Value)
	}
	return p.Style.validate()
}

type choiceConfig struct {
	props *ChoiceProps
	id    string
	kind  NodeType
	theme component.ControlTheme
	view  *View
	emit  func(...any)
}
type choiceState struct {
	echo   *echoState[string]
	open   bool
	token  uint64
	anchor ui.Rect
	frame  ui.Size
}

func buildChoice(b *builder, node *Node, id string) widget.Widget {
	return buildChoiceProps(b, *node.Props.(*ChoiceProps), node.Type, id)
}

// buildChoiceProps shares field retention and translated option presentation with tabs.
func buildChoiceProps(b *builder, props ChoiceProps, kind NodeType, id string) widget.Widget {
	props.Disabled = props.Disabled || b.fieldsDisabled()
	props.Label = b.translate(props.Label)
	props.Error = b.translate(props.Error)
	// The tree is immutable and may be rendered in more than one locale.
	props.Options = append([]ChoiceOption(nil), props.Options...)
	for i := range props.Options {
		props.Options[i].Label = b.translate(props.Options[i].Label)
	}
	p := &props
	c := choiceConfig{props: p, id: id, kind: kind, theme: b.theme(), view: b.view, emit: b.emitChange(p.OnChange)}
	control := p.decorateField(p.Label, id, b.theme(), widget.Stateful{Key: widget.Key(id), Type: (*choiceState)(nil), Widget: c, CreateState: func() widget.State { return &choiceState{} }})
	if kind == NodeSelect {
		return p.Style.wrapControl(b.theme(), control)
	}
	return p.Style.wrapOuter(b.theme(), control)
}
func (s *choiceState) InitState(_ widget.StateContext, value any) {
	c := value.(choiceConfig)
	s.echo = retainEcho(c.view, c.id, c.props.Value, c.props.Revision)
	s.echo.receiveField(c.props)
	c.view.fieldValues[c.id] = s.echo.value
}

// DidUpdateWidget refreshes open menus when their options, theme, or callbacks change.
func (s *choiceState) DidUpdateWidget(ctx widget.StateContext, old, value any) {
	c := value.(choiceConfig)
	s.echo.receiveField(c.props)
	c.view.fieldValues[c.id] = s.echo.value
	if !s.open {
		return
	}
	if c.props.Disabled || ctx.HostOverlayOwner() != widget.Key(c.id) {
		s.dismiss(ctx, c)
		return
	}
	previous, ok := old.(choiceConfig)
	if !ok || !reflect.DeepEqual(previous.props, c.props) || !reflect.DeepEqual(previous.theme, c.theme) {
		s.showMenu(ctx, c, s.anchor)
	}
}
func (*choiceState) Dispose() {}

// dismiss only clears this owner's current menu; another popup may have replaced it.
func (s *choiceState) dismiss(ctx widget.StateContext, c choiceConfig) {
	s.open = false
	ctx.ClearHostOverlay(widget.Key(c.id), s.token)
	ctx.Invalidate()
}

// items keeps activation identical for the inline list and floating select menu.
func (s *choiceState) items(ctx widget.StateContext, c choiceConfig) []component.MenuItem {
	items := make([]component.MenuItem, 0, len(c.props.Options))
	for _, o := range c.props.Options {
		items = append(items, component.MenuItem{ID: c.id + "/option:" + base64.RawURLEncoding.EncodeToString([]byte(o.Value)), Label: o.Label, Selected: s.echo.value == o.Value, Disabled: c.props.Disabled || o.Disabled, OnSelect: func() {
			if c.props.Disabled || o.Disabled {
				return
			}
			ctx.SetState(func() { s.echo.local(o.Value); c.view.fieldValues[c.id] = s.echo.value })
			if c.kind == NodeSelect {
				s.dismiss(ctx, c)
			}
			if c.emit != nil {
				c.emit(o.Value)
			}
		}})
	}
	return items
}

// showMenu uses a Host overlay so menu rows never affect parent layout or clipping.
func (s *choiceState) showMenu(ctx widget.StateContext, c choiceConfig, anchor ui.Rect) {
	if c.props.Disabled {
		return
	}
	s.open, s.anchor, s.frame = true, anchor, ctx.HostFrameSize()
	s.token = ctx.SetHostOverlay(widget.Key(c.id), component.WoxPopupMenu(component.PopupMenuProps{
		MenuListProps: component.MenuListProps{ID: c.id + "/menu", Items: s.items(ctx, c), Width: anchor.Width, Theme: c.theme},
		Anchor:        anchor, OnDismiss: func() { s.dismiss(ctx, c) },
	}))
}

// Build keeps select's trigger size stable; lists reuse the same enabled-row navigation.
func (s *choiceState) Build(ctx widget.StateContext, value any) widget.Widget {
	c := value.(choiceConfig)
	p := c.props
	label := p.Label
	for _, o := range p.Options {
		if o.Value == s.echo.value {
			label = o.Label
			break
		}
	}
	ctx.PostFrame(func() {
		if !ctx.Mounted() || !s.open {
			return
		}
		if ctx.HostOverlayOwner() != widget.Key(c.id) {
			s.open = false
			return
		}
		if anchor, ok := ctx.HostBoundsForKey(widget.Key(c.id)); ok && (anchor != s.anchor || ctx.HostFrameSize() != s.frame) {
			s.showMenu(ctx, c, anchor)
		}
	})
	return widget.LayoutBuilder{Build: func(available ui.Size) widget.Widget {
		width := widthOr(p.Style.resolve(c.theme), available)
		if c.kind == NodeRadioGroup || c.kind == NodeTabs {
			return widget.Container{Width: width, Child: component.WoxChoiceGroup(component.ChoiceGroupProps{
				ID: c.id + "/group", Label: p.fieldLabel(p.Label), Items: s.items(ctx, c), Theme: c.theme, Paint: p.Style.controlPaint(c.theme),
			})}
		}
		if c.kind == NodeList {
			return component.WoxMenuList(component.MenuListProps{ID: c.id + "/list", Items: s.items(ctx, c), Width: width, Height: p.Style.Height.resolve(c.theme), Scrollable: p.Style.Height.resolve(c.theme) > 0, Theme: c.theme, Paint: p.Style.controlPaint(c.theme)})
		}
		var tap func()
		if !p.Disabled {
			tap = func() {
				if s.open {
					s.dismiss(ctx, c)
					return
				}
				if anchor, ok := ctx.HostBoundsForKey(widget.Key(c.id)); ok {
					s.showMenu(ctx, c, anchor)
				}
			}
		}
		return component.WoxDropdown(component.DropdownProps{ID: c.id, Label: p.fieldLabel(p.Label), Value: label, Width: width, Foreground: c.theme.Text, Theme: c.theme, Paint: p.Style.controlPaint(c.theme), OnTap: tap, OnKey: func(e ui.KeyEvent) bool {
			if p.Disabled || !e.Down || e.Composing || e.Modifiers != 0 {
				return false
			}
			switch e.Key {
			case ui.KeyEnter, ui.KeySpace, ui.KeyArrowDown, ui.KeyArrowUp:
				tap()
				return true
			}
			return false
		}})
	}}
}

func (p *ChoiceProps) isDisabled() bool { return p.Disabled }
