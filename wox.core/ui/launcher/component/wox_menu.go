package component

import (
	"strings"
	"unicode/utf8"

	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// MenuItem supplies stable identity and a local action for a menu or selectable list.
type MenuItem struct {
	ID, Label, Icon    string
	Disabled, Selected bool
	OnSelect           func()
}

// MenuListProps shares row appearance and navigation between inline lists and popups.
type MenuListProps struct {
	ID            string
	Items         []MenuItem
	Theme         ControlTheme
	Width, Height float32
	Scrollable    bool
	Autofocus     bool
	Paint         *ControlPaint
}

// WoxMenuList keeps navigation within enabled rows and leaves value changes to activation.
func WoxMenuList(p MenuListProps) widget.Widget {
	return widget.Stateful{Key: widget.Key(p.ID), Type: (*menuListState)(nil), Widget: p, CreateState: func() widget.State { return &menuListState{} }}
}

type menuListState struct {
	active       string
	focusPending bool
}

// InitState starts on the selected enabled row, falling back to the first enabled row.
func (s *menuListState) InitState(_ widget.StateContext, value any) {
	p := value.(MenuListProps)
	s.active = initialMenuItem(p.Items)
	s.focusPending = p.Autofocus
}

func (*menuListState) Dispose() {}

// DidUpdateWidget preserves navigation by ID across producer reorders and option removals.
func (s *menuListState) DidUpdateWidget(_ widget.StateContext, _, value any) {
	p := value.(MenuListProps)
	for _, item := range p.Items {
		if item.ID == s.active && !item.Disabled {
			return
		}
	}
	s.active = initialMenuItem(p.Items)
	s.focusPending = p.Autofocus
}

// initialMenuItem returns the preferred enabled row without selecting a value on the caller's behalf.
func initialMenuItem(items []MenuItem) string {
	first := ""
	for _, item := range items {
		if item.Disabled {
			continue
		}
		if first == "" {
			first = item.ID
		}
		if item.Selected {
			return item.ID
		}
	}
	return first
}

// menuNavigation interprets unmodified navigation and cycles matching initial letters.
func menuNavigation(items []MenuItem, current string, e ui.KeyEvent) (string, bool) {
	if !e.Down || e.Composing || e.Modifiers & ^ui.KeyModifierShift != 0 || len(items) == 0 {
		return "", false
	}
	start := -1
	for i, item := range items {
		if item.ID == current {
			start = i
			break
		}
	}
	direction := 1
	match := ""
	switch e.Key {
	case ui.KeyArrowUp:
		direction = -1
	case ui.KeyArrowDown:
	case ui.KeyHome:
		start = -1
	case ui.KeyEnd:
		start = 0
		direction = -1
	default:
		if utf8.RuneCountInString(string(e.Key)) != 1 {
			return "", false
		}
		match = strings.ToLower(string(e.Key))
	}
	for step := 1; step <= len(items); step++ {
		index := (start + direction*step + len(items)*2) % len(items)
		item := items[index]
		if !item.Disabled && (match == "" || strings.HasPrefix(strings.ToLower(item.Label), match)) {
			return item.ID, true
		}
	}
	return "", true
}

// Build uses each real row's focus target so Host scrolling keeps keyboard navigation visible.
func (s *menuListState) Build(ctx widget.StateContext, value any) widget.Widget {
	p := value.(MenuListProps)
	ctx.PostFrame(func() {
		if s.focusPending && ctx.Mounted() {
			s.focusPending = false
			ctx.RequestFocus(widget.Key(s.active))
		}
	})
	return widget.LayoutBuilder{Build: func(available ui.Size) widget.Widget {
		width := p.Width
		if width <= 0 {
			width = available.Width
		}
		width = min(width, available.Width)
		rowHeight := p.Theme.Scaled(SettingsControlHeight)
		rows := make([]widget.Widget, 0, len(p.Items))
		for _, item := range p.Items {
			foreground := p.Theme.Text
			if item.Selected {
				foreground = p.Theme.SelectionText
			}
			if item.Disabled {
				foreground = p.Theme.TextSecondary
			}
			foreground = p.Paint.foreground(foreground)
			label := widget.TextBlock{Value: item.Label, MaxLines: 1, Style: ui.TextStyle{Size: p.Theme.Scaled(SettingsControlFontSize)}, Color: foreground, ShrinkWrap: true}
			var content widget.Widget = label
			if item.Icon != "" {
				content = widget.Flex{Axis: widget.Horizontal, Gap: p.Theme.Scaled(8), CrossAxisAlignment: widget.CrossAxisCenter, Children: []widget.Widget{IconGlyph(item.Icon, p.Theme.Scaled(16), foreground), widget.Expanded{Child: label}}}
			}
			rows = append(rows, WoxListItem(ListItemProps{ID: item.ID, Label: item.Label, Width: width, Height: rowHeight, Selected: item.Selected, Disabled: item.Disabled,
				Padding: widget.Insets{Left: p.Theme.Scaled(8), Right: p.Theme.Scaled(8)}, Theme: p.Theme, Child: widget.Align{Vertical: 0.5, Child: content}, OnTap: item.OnSelect, Paint: p.Paint.choicePaint(item.Selected),
				OnKey: func(e ui.KeyEvent) bool {
					next, handled := menuNavigation(p.Items, item.ID, e)
					if next != "" {
						s.active = next
						ctx.RequestFocus(widget.Key(next))
					}
					return handled
				},
			}))
		}
		var list widget.Widget = widget.Flex{Axis: widget.Vertical, Children: rows}
		if p.Scrollable {
			height := min(rowHeight*float32(len(rows)), available.Height)
			if p.Height > 0 {
				height = min(height, p.Height)
			}
			list = WoxScrollView(ScrollViewProps{Key: widget.Key(p.ID + "/scroll"), Width: width, Height: height, Content: list, Theme: p.Theme, RetainState: true})
		}
		return list
	}}
}

// PopupMenuProps positions a keyboard-modal menu in window-local logical coordinates.
type PopupMenuProps struct {
	MenuListProps
	Anchor    ui.Rect
	OnDismiss func()
}

// WoxPopupMenu supplies the floating surface, outside-click dismissal, and Escape handling.
// Host restores the previous focus when this modal overlay is removed.
func WoxPopupMenu(p PopupMenuProps) widget.Widget {
	return widget.LayoutBuilder{Build: func(available ui.Size) widget.Widget {
		list := p.MenuListProps
		list.Scrollable, list.Autofocus = true, true
		if list.Height <= 0 {
			list.Height = p.Theme.Scaled(320)
		}
		margin := p.Theme.Scaled(4)
		menu := widget.AnchoredOverlay{Anchor: p.Anchor, Gap: margin, Margin: margin, Child: widget.Container{
			Floating: true, Color: p.Theme.Surface, BorderColor: p.Theme.Border, BorderWidth: 1, Radius: p.Theme.Scaled(6),
			Padding: widget.Insets{Top: margin, Bottom: margin}, Child: WoxMenuList(list),
		}}
		return widget.FocusScope{Key: widget.Key(p.ID + "/modal"), Modal: true, OnKey: func(e ui.KeyEvent) bool {
			if e.Down && !e.Composing && (e.Key == ui.KeyEscape || e.Key == ui.KeyTab) {
				if p.OnDismiss != nil {
					p.OnDismiss()
				}
				return true
			}
			return false
		}, Child: widget.Stack{Width: available.Width, Height: available.Height, Children: []widget.StackChild{
			{Child: widget.Gesture{ID: p.ID + "/dismiss", OnTap: p.OnDismiss, OnSecondaryTapDown: func(ui.Point) {
				if p.OnDismiss != nil {
					p.OnDismiss()
				}
			}, Child: widget.Container{Width: available.Width, Height: available.Height}}},
			{Child: menu},
		}}}
	}}
}
