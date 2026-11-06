package woxview

import (
	"fmt"
	"reflect"
	"strings"

	"wox/common/icons"
	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// ContextMenuItem declares a local popup action with an optional catalog icon.
type ContextMenuItem struct {
	Label    string   `json:"label"`
	Icon     string   `json:"icon,omitempty"`
	OnPress  *Handler `json:"onPress"`
	Disabled bool     `json:"disabled,omitempty"`
}

// validateCommon checks shared props independently of each component's own prepare method.
func (p CommonProps) validateCommon() error {
	if err := validateShortcuts(p.Shortcuts); err != nil {
		return err
	}
	if len(p.ContextMenu) > 500 {
		return fmt.Errorf("contextMenu exceeds 500 items")
	}
	for i, item := range p.ContextMenu {
		if strings.TrimSpace(item.Label) == "" {
			return fmt.Errorf("contextMenu[%d].label must not be empty", i)
		}
		if item.OnPress == nil {
			return fmt.Errorf("contextMenu[%d].onPress is required", i)
		}
		if item.Icon != "" && icons.Get(item.Icon).ImageData == "" {
			return fmt.Errorf("contextMenu[%d]: unknown icon %q", i, item.Icon)
		}
	}
	return nil
}

type contextMenuConfig struct {
	owner  widget.Key
	items  []component.MenuItem
	source []ContextMenuItem
	theme  component.ControlTheme
	child  widget.Widget
}

type contextMenuState struct {
	open   bool
	anchor ui.Rect
	token  uint64
	frame  ui.Size
}

// contextMenu retains popup lifecycle under its owning node and compiles translated labels.
func (b *builder) contextMenu(id string, items []ContextMenuItem, child widget.Widget) widget.Widget {
	owner := widget.Key(id + "/context-menu")
	c := contextMenuConfig{owner: owner, source: items, theme: b.theme(), child: child}
	for i, item := range items {
		c.items = append(c.items, component.MenuItem{ID: fmt.Sprintf("%s/item:%d", owner, i), Label: b.translate(item.Label), Icon: item.Icon, Disabled: item.Disabled, OnSelect: b.tap(item.OnPress)})
	}
	return widget.Stateful{Key: owner, Type: (*contextMenuState)(nil), Widget: c, CreateState: func() widget.State { return &contextMenuState{} }}
}

func (*contextMenuState) InitState(widget.StateContext, any) {}
func (*contextMenuState) Dispose()                           {}

// DidUpdateWidget replaces stale menu actions and translations before the next overlay layout.
func (s *contextMenuState) DidUpdateWidget(ctx widget.StateContext, old, next any) {
	if !s.open {
		return
	}
	a, b := old.(contextMenuConfig), next.(contextMenuConfig)
	if ctx.HostOverlayOwner() != b.owner {
		s.open = false
		return
	}
	changed := !reflect.DeepEqual(a.source, b.source) || !reflect.DeepEqual(a.theme, b.theme)
	if !changed {
		for i, item := range b.items {
			if item.Label != a.items[i].Label {
				changed = true
				break
			}
		}
	}
	if changed {
		s.show(ctx, b)
	}
}

// show keeps all popup state in Core; activating an item is the only producer round trip.
func (s *contextMenuState) show(ctx widget.StateContext, c contextMenuConfig) {
	s.open, s.frame = true, ctx.HostFrameSize()
	dismiss := func() { s.open = false; ctx.ClearHostOverlay(c.owner, s.token) }
	items := append([]component.MenuItem(nil), c.items...)
	for i := range items {
		action := items[i].OnSelect
		items[i].OnSelect = func() {
			dismiss()
			if action != nil {
				action()
			}
		}
	}
	s.token = ctx.SetHostOverlay(c.owner, component.WoxPopupMenu(component.PopupMenuProps{
		MenuListProps: component.MenuListProps{ID: string(c.owner) + "/menu", Items: items, Width: c.theme.Scaled(240), Theme: c.theme}, Anchor: s.anchor, OnDismiss: dismiss,
	}))
}

// Build observes right clicks through child controls while preserving normal input behavior.
func (s *contextMenuState) Build(ctx widget.StateContext, value any) widget.Widget {
	c := value.(contextMenuConfig)
	ctx.PostFrame(func() {
		if !ctx.Mounted() || !s.open {
			return
		}
		if ctx.HostOverlayOwner() != c.owner {
			s.open = false
			return
		}
		if ctx.HostFrameSize() != s.frame {
			s.show(ctx, c)
		}
	})
	return widget.Gesture{ID: string(c.owner), ObserveDescendants: true, OnSecondaryTapDown: func(p ui.Point) { s.anchor = ui.Rect{X: p.X, Y: p.Y}; s.show(ctx, c) }, Child: widget.Container{Child: c.child}}
}
