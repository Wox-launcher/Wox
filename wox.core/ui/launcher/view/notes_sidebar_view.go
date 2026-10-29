package view

import (
	"fmt"
	"strings"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const NotesSidebarRowHeight = woxcomponent.SidebarRowHeight

// NotesSidebarRow is one visible tree row, including pinned shortcuts and section labels.
type NotesSidebarRow struct {
	ID, Title, ParentID                          string
	Group, Expanded, Selected, Shortcut, Section bool
	Depth                                        int
}

// NotesSidebarProps supplies all behavior through callbacks; the view owns no tree state.
type NotesSidebarProps struct {
	Width, Height                                                     float32
	Rows                                                              []NotesSidebarRow
	Theme                                                             woxcomponent.Theme
	Scroll, Horizontal                                                *woxwidget.ScrollController
	Title, RootLabel, NewLabel, GroupLabel, MoreLabel, ToggleAllLabel string
	HasGroups, AnyExpanded                                            bool
	OnNew, OnGroup, OnRootSelect, OnToggleAll                         func()
	OnSelect, OnToggle, OnMenu                                        func(int)
	OnKey                                                             func(int, woxui.KeyEvent) bool
	OnHover                                                           func(bool, string, woxui.Rect)
	OnDragStart                                                       func(int, woxui.Point)
	OnDragUpdate                                                      func(woxui.Point)
	OnDragEnd                                                         func()
	DropIndex                                                         int
	DropInto, DropAfter, DropRoot                                     bool
	Dragging                                                          bool
	SearchLabel, SearchHint, SearchValue, ClearLabel, EmptyLabel      string
	SearchItems                                                       []NotesSearchItem
	SearchIndex                                                       int
	SearchEditor                                                      *woxwidget.TextEditingController
	SearchFocus                                                       *woxwidget.FocusNode
	SearchFocused                                                     bool
	Window                                                            *woxui.Window
	OnSearchChanged                                                   func(string)
	OnSearchClear                                                     func()
	OnSearchKey                                                       func(woxui.KeyEvent) bool
}

// NotesSidebar presents quiet navigation chrome with compact file rows and hover-only actions.
func NotesSidebar(p NotesSidebarProps) woxwidget.Widget {
	s := p.Theme.Controls.Scaled
	rowHeight := s(NotesSidebarRowHeight)
	width := max(float32(0), p.Width-s(16))
	contentWidth := width
	for _, row := range p.Rows {
		contentWidth = max(contentWidth, width+max(float32(0), float32(row.Depth)*s(16)-s(64)))
	}
	rows := make([]woxwidget.Widget, 0, len(p.Rows))
	for index, row := range p.Rows {
		if row.Section {
			rows = append(rows, woxwidget.Container{Width: contentWidth, Height: rowHeight, Padding: woxwidget.Insets{Left: s(8)}, Child: woxwidget.Align{Height: rowHeight, Vertical: .5, Child: woxwidget.Text{Value: row.Title, Style: woxui.TextStyle{Size: s(12), Weight: woxui.FontWeightSemibold}, Color: p.Theme.Controls.TextSecondary}}})
			continue
		}
		rows = append(rows, notesSidebarTreeRow(p, row, index, contentWidth))
	}
	height := max(float32(0), p.Height-s(96))
	horizontal := woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{Key: "notes.sidebar.horizontal", Width: width, Height: height, ContentWidth: contentWidth, Horizontal: true, Controller: p.Horizontal, Theme: p.Theme.Controls, Content: woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{Key: "notes.sidebar.tree", AutomationID: "notes.sidebar.tree", Label: p.Title, Width: contentWidth, Height: height, ContentHeight: float32(len(rows)) * rowHeight, Controller: p.Scroll, Theme: p.Theme.Controls, Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}})})
	rootIndicator := woxui.Color{}
	if p.DropRoot {
		rootIndicator = p.Theme.Controls.Focus
	}
	// Keep the scroll subtree at the same path while the root-drop indicator changes.
	var content woxwidget.Widget = woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: horizontal}, {Child: woxwidget.Container{Width: s(2), Height: height, Color: rootIndicator}},
	}}
	if strings.TrimSpace(p.SearchValue) != "" {
		results := make([]woxwidget.Widget, 0, len(p.SearchItems))
		for i, item := range p.SearchItems {
			selected := i == p.SearchIndex && p.SearchFocused
			background, color := woxui.Color{}, p.Theme.Controls.BodyText
			if selected {
				background, color = p.Theme.Controls.SelectionBackground, p.Theme.Controls.SelectionText
			}
			results = append(results, woxcomponent.WoxListItem(woxcomponent.ListItemProps{ID: "notes.search." + item.ID, Label: item.Label, Width: width, Height: rowHeight, Padding: woxwidget.Insets{Left: s(24), Right: s(8)}, Background: &background, Selected: selected, SkipFocus: true, Theme: p.Theme.Controls, OnTap: item.OnTap, Child: woxwidget.Align{Height: rowHeight, Vertical: .5, Child: woxwidget.TextBlock{Value: item.Label, MaxLines: 1, AlignmentY: .5, Style: woxui.TextStyle{Size: s(13)}, Color: color}}}))
		}
		if len(results) == 0 {
			results = append(results, woxwidget.Container{Width: width, Height: rowHeight, Padding: woxwidget.Insets{Left: s(8)}, Child: woxwidget.Align{Height: rowHeight, Vertical: .5, Child: woxwidget.Text{Value: p.EmptyLabel, Style: woxui.TextStyle{Size: s(12)}, Color: p.Theme.Controls.TextSecondary}}})
		}
		var keepVisible *woxwidget.ScrollRange
		if len(p.SearchItems) > 0 {
			start := float32(p.SearchIndex) * rowHeight
			keepVisible = &woxwidget.ScrollRange{Start: start, End: start + rowHeight}
		}
		content = woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{Key: "notes.search.results", AutomationID: "notes.search.results", Label: p.SearchLabel, Width: width, Height: height, ContentHeight: float32(len(results)) * rowHeight, KeepVisible: keepVisible, Theme: p.Theme.Controls, Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: results}})
	}
	searchHeight, searchInset := s(32), s(4)
	rightPadding := s(12)
	if p.SearchValue != "" {
		rightPadding = s(36)
	}
	input := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: "notes.search", Label: p.SearchLabel, Hint: p.SearchHint, Width: width, Height: searchHeight,
		// Keep the full line inside the compact field instead of using the taller form-field padding.
		Padding: woxwidget.Insets{Left: s(12), Top: searchInset, Right: rightPadding, Bottom: searchInset}, LineHeight: searchHeight - 2*searchInset,
		Background: woxcomponent.TitleBarAlpha(p.Theme.Controls.BodyText, 18), Style: woxui.TextStyle{Size: woxcomponent.SettingsControlFontSize},
		Value: p.SearchValue, Controller: p.SearchEditor, FocusNode: p.SearchFocus, Focused: p.SearchFocused, Window: p.Window, MaxLines: 1, Theme: p.Theme.Controls, OnChanged: p.OnSearchChanged, OnKey: p.OnSearchKey,
	})
	search := woxwidget.Stack{Width: width, Height: searchHeight, Children: []woxwidget.StackChild{{Child: input}}}
	if p.SearchValue != "" && p.OnSearchClear != nil {
		search.Children = append(search.Children, woxwidget.StackChild{Right: s(2), AnchorRight: true, Child: woxwidget.Align{Width: s(28), Height: searchHeight, Horizontal: .5, Vertical: .5, Child: woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "notes.search.clear", Label: p.ClearLabel, Width: s(28), Height: s(28), Radius: s(14), Icon: woxcomponent.CloseGlyph(s(16), p.Theme.Controls.TextSecondary),
			HoverBackground: woxcomponent.TitleBarAlpha(p.Theme.Controls.TextSecondary, 25), FocusRingColor: p.Theme.Controls.Focus, OnTap: p.OnSearchClear,
		})}})
	}
	iconAction := func(id, label string, glyph woxwidget.Widget, action func(), disabled bool) woxwidget.Widget {
		return woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{ID: id, Label: label, Width: s(28), Height: s(32), Radius: s(4), Icon: glyph, Disabled: disabled, HoverBackground: woxcomponent.TitleBarAlpha(p.Theme.Controls.Text, 20), FocusRingColor: p.Theme.Controls.Focus, OnTap: action, OnHoverAt: func(inside bool, bounds woxui.Rect) {
			if p.OnHover != nil && !disabled {
				p.OnHover(inside, label, bounds)
			}
		}})
	}
	var spacer woxwidget.Widget = woxwidget.Container{Height: s(32)}
	if p.OnRootSelect != nil {
		spacer = woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{ID: "notes.sidebar.root", Label: p.RootLabel, Width: max(0, width-s(84)), Height: s(32), FocusRingColor: p.Theme.Controls.Focus, OnTap: p.OnRootSelect})
	}
	allColor := p.Theme.Controls.TextSecondary
	if !p.HasGroups {
		allColor.A = allColor.A / 3
	}
	header := woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Expanded{Child: spacer},
		iconAction("notes.sidebar.group", p.GroupLabel, woxcomponent.FolderGlyph(s(16), p.Theme.Controls.TextSecondary), p.OnGroup, false),
		iconAction("notes.sidebar.new", p.NewLabel, woxcomponent.AddGlyph(s(16), p.Theme.Controls.TextSecondary), p.OnNew, false),
		iconAction("notes.sidebar.toggle-all", p.ToggleAllLabel, woxcomponent.ExpandAllGlyph(s(16), allColor, p.AnyExpanded), p.OnToggleAll, !p.HasGroups),
	}}
	return woxcomponent.SidebarPanel(p.Width, p.Height, woxwidget.UniformInsets(s(8)), p.Theme, woxwidget.Flex{Axis: woxwidget.Vertical, Gap: s(8), Children: []woxwidget.Widget{header, search, content}})
}

// NotesDeleteDialog follows the same title, explanation and action layout as Chat's project confirmation.
type NotesDeleteDialogProps struct {
	Size                                                  woxui.Size
	Theme                                                 woxcomponent.ControlTheme
	Title, Name, Message, Error, CancelLabel, DeleteLabel string
	OnCancel, OnDelete                                    func()
}

// NotesDeleteDialog confirms a note or group deletion before invoking its action.
func NotesDeleteDialog(p NotesDeleteDialogProps) woxwidget.Widget {
	s := p.Theme.Scaled
	width := min(s(460), max(0, p.Size.Width-s(24)))
	inner := max(0, width-s(32))
	actionAxis, actionHeight := woxwidget.Horizontal, s(32)
	if inner < s(300) {
		actionAxis, actionHeight = woxwidget.Vertical, s(72)
	}
	content := []woxwidget.Widget{
		woxwidget.TextBlock{Value: p.Title, Width: inner, Height: s(32), LineHeight: s(32), MaxLines: 1, AlignmentY: .5, Style: woxui.TextStyle{Size: s(woxcomponent.SettingsPageTitleFontSize), Weight: woxui.FontWeightSemibold}, Color: p.Theme.Text},
		woxwidget.TextBlock{Value: p.Name, Width: inner, Height: s(20), LineHeight: s(20), MaxLines: 1, Style: woxui.TextStyle{Size: s(13), Weight: woxui.FontWeightSemibold}, Color: p.Theme.Text},
		woxwidget.TextBlock{Value: p.Message, Width: inner, Height: s(64), LineHeight: s(16), MaxLines: 4, Style: woxui.TextStyle{Size: s(12)}, Color: p.Theme.TextSecondary},
	}
	if p.Error != "" {
		content = append(content, woxwidget.TextBlock{Value: p.Error, Width: inner, Height: s(32), LineHeight: s(16), MaxLines: 2, Style: woxui.TextStyle{Size: s(12)}, Color: p.Theme.Error})
	}
	content = append(content, woxwidget.Align{Width: inner, Height: actionHeight, Horizontal: 1, Child: woxwidget.Flex{Axis: actionAxis, Gap: s(8), CrossAxisAlignment: woxwidget.CrossAxisEnd, Children: []woxwidget.Widget{
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "notes.organize.cancel", Label: p.CancelLabel, Height: s(32), Theme: p.Theme, OnTap: p.OnCancel}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "notes.organize.confirm", Label: p.DeleteLabel, Height: s(32), Theme: p.Theme, Variant: woxcomponent.ButtonPrimary, OnTap: p.OnDelete}),
	}}})
	contentHeight := s(32+20+64+32) + actionHeight + float32(len(content)-1)*s(12)
	if p.Error != "" {
		contentHeight += s(32)
	}
	height := min(contentHeight, max(0, p.Size.Height-s(24)))
	var body woxwidget.Widget = woxwidget.Flex{Axis: woxwidget.Vertical, Gap: s(12), Children: content}
	if height < contentHeight {
		body = woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{Key: "notes.delete.scroll", Width: inner, Height: max(0, height-s(32)), ContentHeight: contentHeight - s(32), Theme: p.Theme, Content: body})
	}
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "notes.organize.dialog", Label: p.Message, Width: width, Height: height, OverlayWidth: p.Size.Width, OverlayHeight: p.Size.Height,
		BackdropAlpha: 160, Theme: p.Theme, Padding: woxwidget.UniformInsets(s(16)), InitialFocus: "notes.organize.cancel",
		OnEscape: p.OnCancel, OnBackdrop: p.OnCancel, Child: body,
	})
}

// notesSidebarTreeRow keeps a stable interaction tree while revealing secondary actions on hover.
func notesSidebarTreeRow(p NotesSidebarProps, row NotesSidebarRow, index int, width float32) woxwidget.Widget {
	s := p.Theme.Controls.Scaled
	height := s(NotesSidebarRowHeight)
	id := NotesSidebarRowKey(row)
	root := row.Group && row.ID == ""
	indent := float32(row.Depth) * s(16)
	return woxcomponent.Hoverable(woxwidget.Key(id+".hover"), false, func(hovered bool, onHover func(bool, woxui.Rect)) woxwidget.Widget {
		controls := p.Theme.Controls
		foreground := controls.BodyText
		selected := row.Selected && !row.Group
		if selected {
			foreground = controls.SelectionText
		}
		variant := woxcomponent.ButtonText
		if selected {
			variant = woxcomponent.ButtonSelected
		}
		hoverBackground := woxcomponent.TitleBarAlpha(controls.Text, 12)
		if selected {
			hoverBackground = controls.SelectionBackground
		} else if p.Dragging {
			hoverBackground = woxui.Color{}
		}
		var icon *woxui.Image
		padding := woxwidget.Insets{Left: indent + s(8), Right: s(28)}
		fontSize, weight := s(13), woxui.FontWeightRegular
		if root {
			padding.Left = s(8)
			controls.Text = controls.TextSecondary
			fontSize, weight = s(12), woxui.FontWeightSemibold
		} else if row.Group {
			padding.Left += s(16)
			icon = woxcomponent.FolderGlyph(s(16), foreground).(woxwidget.Image).Source
		}
		button := woxcomponent.ButtonProps{ID: id, Label: row.Title, Width: width, Height: height, Radius: s(4), Padding: padding, AlignLeading: true, FontSize: fontSize, FontWeight: weight, Icon: icon, IconSize: s(16), IconGap: s(8), Selected: selected, Variant: variant, Theme: controls, OnTap: func() { p.OnSelect(index) }, OnKey: func(event woxui.KeyEvent) bool { return p.OnKey(index, event) }, OnSecondaryTapDown: func(woxui.Point) { p.OnMenu(index) }}
		button.HoverBackground = &hoverBackground
		if !row.Shortcut && !root {
			if p.Dragging {
				button.Cursor = woxui.PointerCursorGrab
			}
			button.OnPanStart = func(point woxui.Point) { p.OnDragStart(index, point) }
			button.OnPanUpdate, button.OnPanEnd = p.OnDragUpdate, p.OnDragEnd
		}
		var disclosure, menu woxwidget.Widget
		if row.Group && !root {
			disclosure = woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{ID: id + ".toggle", Label: row.Title, Width: s(20), Height: height, Icon: woxcomponent.DisclosureChevronGlyph(s(12), controls.TextSecondary, row.Expanded), FocusRingColor: controls.Focus, OnTap: func() { p.OnToggle(index) }})
		}
		if !root {
			var idle woxwidget.Widget
			if !hovered || p.Dragging {
				idle = woxwidget.Painter{Width: s(16), Height: s(16)}
			}
			menu = woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{ID: id + ".more", Label: p.MoreLabel, Width: s(28), Height: height, Icon: woxcomponent.MoreHorizontalGlyph(s(16), controls.TextSecondary), IdleIcon: idle, FocusRingColor: controls.Focus, OnTap: func() { p.OnMenu(index) }})
		}
		lineColor, outlineColor := woxui.Color{}, woxui.Color{}
		lineTop := float32(0)
		if index == p.DropIndex {
			if p.DropInto {
				outlineColor = controls.Focus
			} else {
				lineColor = controls.Focus
				if p.DropAfter {
					lineTop = height - s(2)
				}
			}
		}
		// Keep feedback in sibling paint layers so pointer capture survives hover and expansion frames.
		return woxwidget.Gesture{ID: id + ".surface", CoverHover: true, OnHoverAt: onHover, Child: woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
			{Child: woxcomponent.WoxButton(button)},
			{Left: indent + s(4), Child: disclosure},
			{Right: 0, AnchorRight: true, Child: menu},
			{Child: woxwidget.Container{Width: width, Height: height, Radius: s(4), BorderWidth: 1, BorderColor: outlineColor}},
			{Top: lineTop, Child: woxwidget.Container{Width: width, Height: s(2), Color: lineColor}},
		}}}
	})
}

// NotesSidebarLayout places the tree beside the editor.
func NotesSidebarLayout(size woxui.Size, sidebar, body woxwidget.Widget, sidebarWidth, inset float32, background woxui.Color) woxwidget.Widget {
	if sidebar == nil {
		return body
	}
	return woxcomponent.SidebarSplit(woxwidget.Container{Width: sidebarWidth, Height: size.Height, Color: background, Padding: woxwidget.Insets{Left: inset, Right: inset, Top: NotesToolbarHeight + inset, Bottom: inset}, Child: sidebar}, body)
}

// NotesSidebarRowKey stays stable when expanding ancestors changes visible row indices.
func NotesSidebarRowKey(row NotesSidebarRow) string {
	return fmt.Sprintf("notes.sidebar.row.%t.%t.%s", row.Group, row.Shortcut, row.ID)
}
