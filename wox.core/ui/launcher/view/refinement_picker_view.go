package view

import (
	"fmt"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// RefinementGroupKey identifies the trigger in the bar and anchors its popup.
func RefinementGroupKey(id string) woxwidget.Key {
	return woxwidget.Key("refinement-group-" + id)
}

// refinementDropdown combines the selection and shortcut in a single focusable control.
func refinementDropdown(group RefinementGroup, props RefinementsProps, height float32) (woxwidget.Widget, float32) {
	s := func(n float32) float32 { return scaledLauncherSize(n, props.DensityScale) }
	theme := props.Theme
	titleColor := refinementThemeColor(theme.RefinementTitleColor, theme.ResultSubtitle)
	hotkeyColor := refinementThemeColor(theme.RefinementHotkeyColor, theme.ResultSubtitle)
	border := refinementThemeColor(theme.RefinementBorderColor, refinementColorWithOpacity(theme.ResultSubtitle, 0.24))
	if group.Expanded {
		border = theme.Controls.Focus
	}
	title := group.Title
	if group.Current != "" {
		title += " · " + group.Current
	}
	style := woxui.TextStyle{Size: s(woxcomponent.TailFontSize), Weight: woxui.FontWeightSemibold}
	hotkeyWidth := float32(0)
	if group.Hotkey != "" {
		hotkeyWidth = refinementTextWidth(props.Window, group.Hotkey, style) + s(16) + 1
	}
	labelWidth := min(s(158), refinementTextWidth(props.Window, title, style)+s(2))
	width := max(s(146), labelWidth+s(38)+hotkeyWidth)
	width = min(width, max(s(80), props.Width-s(16)))
	labelWidth = max(0, width-s(38)-hotkeyWidth)
	key := RefinementGroupKey(group.ID)
	control := woxcomponent.Hoverable(key+"/hover", false, func(hovered bool, onHoverAt func(bool, woxui.Rect)) woxwidget.Widget {
		background := refinementThemeColor(theme.RefinementBackgroundColor, refinementColorWithOpacity(theme.ResultTitle, 0.035))
		if hovered {
			background = woxcomponent.ControlHoverColor(background, theme.ResultTitle)
		}
		children := []woxwidget.Widget{
			woxwidget.Container{Width: s(8)},
			woxwidget.TextBlock{Value: title, Width: labelWidth, Height: height, LineHeight: height, MaxLines: 1, AlignmentY: 0.5, Style: style, Color: theme.ResultTitle},
			woxwidget.Container{Width: s(8)},
			woxcomponent.ChevronGlyph(s(14), titleColor, group.Expanded),
			woxwidget.Container{Width: s(8)},
		}
		if group.Hotkey != "" {
			children = append(children,
				woxwidget.Container{Width: 1, Height: s(14), Color: refinementThemeColor(theme.RefinementDividerColor, refinementColorWithOpacity(theme.ResultSubtitle, 0.25))},
				woxwidget.Align{Width: hotkeyWidth - 1, Height: height, Horizontal: 0.5, Vertical: 0.5,
					Child: woxwidget.Text{Value: group.Hotkey, Style: style, Color: hotkeyColor}},
			)
		}
		return woxwidget.Gesture{ID: string(key), OnTap: group.OnTap, OnHoverAt: onHoverAt, Child: woxwidget.Container{
			Width: width, Height: height, Radius: s(7), Color: background, BorderWidth: 1, BorderColor: border,
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children},
		}}
	})
	return woxwidget.Semantics{
		Key: key, AutomationID: string(key), Role: woxui.AccessibilityRoleButton, Label: title, Description: group.Hotkey,
		Expanded: group.Expanded, Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action == woxui.AccessibilityActionActivate && group.OnTap != nil {
				group.OnTap()
			}
			return nil
		},
		Child: woxwidget.Focusable{Key: key, FocusRingColor: theme.Controls.Focus, FocusRingRadius: s(7), OnKey: group.OnKey, Child: control},
	}, width
}

// RefinementPickerProps describes a searchable list anchored to a refinement trigger.
type RefinementPickerProps struct {
	ID, Search, SearchLabel, EmptyLabel string
	Anchor                              woxui.Rect
	Theme                               woxcomponent.Theme
	Window                              *woxui.Window
	DensityScale                        float32
	Count, Cursor                       int
	OptionAt                            func(int) RefinementOption
	OnChanged                           func(string)
	OnKey                               func(woxui.KeyEvent) bool
	OnDismiss                           func()
}

const maxRefinementPickerRows = 6

// RefinementPickerHeight caps the popup while keeping its search field fixed.
func RefinementPickerHeight(count int, scale float32) float32 {
	return scaledLauncherSize(46, scale) + float32(min(max(count, 1), maxRefinementPickerRows))*scaledLauncherSize(30, scale)
}

// RefinementPickerView overlays a compact searchable menu without moving the result or preview panes.
func RefinementPickerView(props RefinementPickerProps, size woxui.Size) (woxwidget.Widget, woxui.Rect) {
	s := func(n float32) float32 { return scaledLauncherSize(n, props.DensityScale) }
	gap, padding, searchHeight, rowHeight := s(3), s(6), s(30), s(30)
	searchInset := s(4)
	anchor := props.Anchor
	theme := props.Theme.Controls
	room := max(anchor.Y, size.Height-anchor.Y-anchor.Height) - 2*gap
	height := min(RefinementPickerHeight(props.Count, props.DensityScale), room)
	listHeight := max(0, height-searchHeight-2*padding-s(4))
	width := min(anchor.Width, max(0, size.Width-2*gap))
	inner := max(0, width-2*padding)
	if inner <= 0 || listHeight <= 0 {
		return nil, woxui.Rect{}
	}
	id := "refinement-picker-" + props.ID
	scroll := woxcomponent.ScrollViewProps{
		Key: woxwidget.Key(id + "/options"), Width: inner, Height: listHeight, ContentHeight: float32(props.Count) * rowHeight,
		Theme: theme, RetainState: true, ReserveScrollbarSpace: float32(props.Count)*rowHeight > listHeight,
	}
	if props.Cursor >= 0 {
		start := float32(props.Cursor) * rowHeight
		scroll.KeepVisible = &woxwidget.ScrollRange{Start: start, End: start + rowHeight}
	}
	rowWidth := scroll.ContentViewportWidth()
	scroll.Content = woxwidget.LazyList{Key: woxwidget.Key(id + "/rows"), Width: rowWidth, ItemCount: props.Count, ItemExtent: rowHeight,
		ItemBuilder: func(index int) woxwidget.Widget {
			return refinementPickerRow(id, props.OptionAt(index), index == props.Cursor, rowWidth, rowHeight, props)
		}}
	var list woxwidget.Widget = woxcomponent.WoxScrollView(scroll)
	if props.Count == 0 {
		list = woxwidget.Align{Width: inner, Height: listHeight, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
			Value: props.EmptyLabel, Style: woxui.TextStyle{Size: s(11)}, Color: theme.TextSecondary,
		}}
	}
	search := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: id + "/search", Label: props.SearchLabel, Hint: props.SearchLabel, Value: props.Search, Autofocus: true, Focused: true,
		Width: inner, Height: searchHeight, Radius: s(5), MaxLines: 1,
		Padding: woxwidget.Insets{Left: s(25), Top: searchInset, Right: s(7), Bottom: searchInset}, LineHeight: searchHeight - 2*searchInset,
		Background: theme.InputBackground, BorderColor: theme.Border, BorderWidth: 1, FocusRingColor: theme.Focus,
		Style: woxui.TextStyle{Size: s(12)}, Window: props.Window, Theme: theme, OnChanged: props.OnChanged,
	})
	menuBounds := woxwidget.AnchoredOverlayBounds(anchor, woxui.Size{Width: width, Height: height}, size, gap, gap)
	menu := woxwidget.Container{
		Width: width, Height: height, Padding: woxwidget.UniformInsets(padding), Radius: s(8), Floating: true,
		Color: theme.Surface, BorderColor: theme.Border, BorderWidth: 1, Child: woxwidget.Flex{
			Axis: woxwidget.Vertical, Gap: s(4), Children: []woxwidget.Widget{
				woxwidget.Stack{Width: inner, Height: searchHeight, Children: []woxwidget.StackChild{
					{Child: search},
					{Left: s(7), Child: woxwidget.Align{Width: s(13), Height: searchHeight, Vertical: 0.5, Child: woxcomponent.SearchGlyph(s(13), theme.TextSecondary)}},
				}}, list,
			},
		},
	}
	// Leave the trigger strip uncovered so another dropdown can be opened in one click.
	backdrop := func(height float32) woxwidget.Widget {
		return woxwidget.Gesture{ID: id + "/dismiss", OnTap: props.OnDismiss, OnScroll: func(woxui.Point) {},
			Child: woxwidget.Container{Width: size.Width, Height: max(0, height)}}
	}
	below := anchor.Y + anchor.Height
	return woxwidget.FocusScope{Key: woxwidget.Key(id), Modal: true, OnKey: props.OnKey, Child: woxwidget.Stack{
		Width: size.Width, Height: size.Height, Children: []woxwidget.StackChild{
			{Child: backdrop(anchor.Y)},
			{Top: below, Child: backdrop(size.Height - below)},
			{Left: menuBounds.X, Top: menuBounds.Y, Child: menu},
		},
	}}, menuBounds
}

// refinementPickerRow keeps keyboard highlight distinct from the selected value.
func refinementPickerRow(id string, option RefinementOption, cursor bool, width, height float32, props RefinementPickerProps) woxwidget.Widget {
	s := func(n float32) float32 { return scaledLauncherSize(n, props.DensityScale) }
	theme := props.Theme.Controls
	key := woxwidget.Key(id + "/option/" + option.Value)
	label := option.Label
	if option.Count != nil {
		label = fmt.Sprintf("%s (%d)", label, *option.Count)
	}
	return woxwidget.Semantics{
		Key: key, AutomationID: string(key), Role: woxui.AccessibilityRoleButton, Label: label, Selected: option.Selected,
		Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action == woxui.AccessibilityActionActivate {
				option.OnTap()
			}
			return nil
		},
		Child: woxcomponent.Hoverable(key+"/hover", false, func(hovered bool, onHoverAt func(bool, woxui.Rect)) woxwidget.Widget {
			background, foreground := woxui.Color{}, theme.Text
			if cursor {
				background = theme.SelectionBackground
				if theme.SelectionText.A != 0 {
					foreground = theme.SelectionText
				}
			} else if hovered {
				background = theme.Hover
			}
			children := make([]woxwidget.Widget, 0, 3)
			textWidth := max(0, width-s(30))
			if option.Icon != nil {
				children = append(children, woxwidget.Image{Source: option.Icon, Width: s(14), Height: s(14)})
				textWidth = max(0, textWidth-s(20))
			}
			children = append(children, woxwidget.TextBlock{Value: label, Width: textWidth, Height: height, LineHeight: height,
				MaxLines: 1, AlignmentY: 0.5, Style: woxui.TextStyle{Size: s(12)}, Color: foreground})
			if option.Selected {
				children = append(children, woxcomponent.CheckGlyph(s(12), theme.Accent))
			}
			return woxwidget.Gesture{ID: string(key), OnTap: option.OnTap, OnHoverAt: onHoverAt, Child: woxwidget.Container{
				Width: width, Height: height, Radius: s(5), Color: background, Padding: woxwidget.Insets{Left: s(6), Right: s(6)},
				Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: s(6), CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children},
			}}
		}),
	}
}

type refinementLayoutObserver struct{}

func (*refinementLayoutObserver) InitState(woxwidget.StateContext, any)            {}
func (*refinementLayoutObserver) DidUpdateWidget(woxwidget.StateContext, any, any) {}
func (*refinementLayoutObserver) Dispose()                                         {}

func (*refinementLayoutObserver) Build(ctx woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(refinementLayoutProps)
	if props.afterLayout != nil {
		ctx.PostFrame(props.afterLayout)
	}
	return props.child
}

type refinementLayoutProps struct {
	child       woxwidget.Widget
	afterLayout func()
}

// ObserveRefinementLayout resolves focus and popup geometry after the retained bar has been laid out.
func ObserveRefinementLayout(child woxwidget.Widget, afterLayout func()) woxwidget.Widget {
	return woxwidget.Stateful{Key: "refinement-layout", Type: (*refinementLayoutObserver)(nil),
		Widget:      refinementLayoutProps{child: child, afterLayout: afterLayout},
		CreateState: func() woxwidget.State { return &refinementLayoutObserver{} }}
}
