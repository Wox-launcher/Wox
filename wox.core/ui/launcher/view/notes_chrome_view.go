package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	NotesSearchRowHeight         = float32(36)
	NotesSearchSectionHeight     = float32(28)
	NotesSearchSectionLead       = float32(12)
	notesSearchSectionTextHeight = float32(16)
	NotesSearchTextHeight        = float32(18)
	notesSearchListGap           = float32(4)
	// WoxTextField's default chrome is 40 logical pixels; a shorter slot overflows the overlay flex.
	notesSearchFieldHeight    = float32(40)
	notesSearchOverlayPadding = float32(12)
	notesSearchOverlayGap     = float32(8)
	notesSearchOverlayTop     = float32(44)
	// NotesToolbarActionsWidth is the four 32-wide title-bar buttons plus gaps
	// and the 6px inset used to place that cluster. Equal macOS title insets
	// used to be 220, which left only 20px at the 460 default width.
	NotesToolbarActionsWidth = float32(140)
	notesFormatButtonSize    = float32(28)
	notesFormatButtonGap     = float32(2)
	notesFormatBarPadLeft    = float32(12)
	notesFormatBarPadRight   = float32(12)
	notesFormatBarStatsGap   = float32(8)
	// NotesFormatBarLeadCount is the heading through link cluster kept visible
	// before the overflow "more" control.
	NotesFormatBarLeadCount = 7
)

// NotesToolbarProps is a presentation snapshot plus stable event callbacks.
type NotesToolbarProps struct {
	SidebarLabel                            string
	SidebarOpen                             bool
	OnSidebar                               func()
	Width                                   float32
	DensityScale                            float32
	Active, Pinned                          bool
	Maximized, FormatVisible                bool
	Platform                                string
	Title, Label                            string
	PinLabel, NewLabel                      string
	FormatLabel, MoreLabel                  string
	Icon                                    *woxui.Image
	Theme                                   woxcomponent.Theme
	OnHover                                 func(bool, string, woxui.Rect)
	OnPin, OnFormat, OnNew, OnMore          func()
	OnDrag, OnMaximize, OnMinimize, OnClose func()
}

// NotesToolbar composes title-bar chrome from prepared state.
func NotesToolbar(props NotesToolbarProps) woxwidget.Widget {
	width, active, theme := props.Width, props.Active, props.Theme
	const buttonSize = float32(32)
	const iconSize = float32(16)
	hoverBackground := woxcomponent.TitleBarAlpha(theme.ToolbarText, 20)
	button := func(id, label string, icon woxwidget.Widget, disabled bool, action func()) woxwidget.Widget {
		onTap := func() {
			props.OnHover(false, "", woxui.Rect{})
			if action != nil {
				action()
			}
		}
		return woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "notes.toolbar." + id, Label: label, Icon: icon, Width: buttonSize, Height: buttonSize, Radius: 5,
			HoverBackground: hoverBackground, FocusRingColor: theme.Cursor, Disabled: disabled, OnTap: onTap,
			OnHoverAt: func(inside bool, bounds woxui.Rect) { props.OnHover(inside, label, bounds) },
		})
	}
	color := theme.ToolbarText
	pin := woxcomponent.PinGlyph(iconSize, color)
	if props.Pinned {
		pin = woxcomponent.FilledPinGlyph(iconSize, theme.PreviewText)
	}
	right := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 2, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		button("pin", props.PinLabel, pin, false, props.OnPin),
		button("format", props.FormatLabel, woxcomponent.ToolbarToggleGlyph(iconSize, color, props.FormatVisible), false, props.OnFormat),
		button("new", props.NewLabel, woxcomponent.AddGlyph(iconSize, color), false, props.OnNew),
		button("more", props.MoreLabel, woxcomponent.MenuGlyph(iconSize, color), false, props.OnMore),
	}}
	contentRight := woxcomponent.TitleBarChromeWidth(props.Platform, true, true)
	titleLeft, titleRight, titleAlignment := NotesTitleSlot(props.Platform, contentRight)
	if props.OnSidebar != nil {
		if props.Platform == "darwin" {
			titleLeft += 32
		} else {
			titleLeft += 44
		}
	}
	drag := woxwidget.Semantics{AutomationID: "notes.toolbar.drag", Role: woxui.AccessibilityRoleGroup, Label: props.Label, Child: woxwidget.Gesture{
		ID:          "notes.toolbar.drag",
		OnDragStart: props.OnDrag,
		OnDoubleTap: props.OnMaximize,
		Child:       woxwidget.Container{Width: width, Height: NotesToolbarHeight},
	}}
	children := []woxwidget.StackChild{
		{Child: drag},
		{Left: titleLeft, Right: titleRight, StretchWidth: true, Child: woxwidget.Align{Height: NotesToolbarHeight, Horizontal: titleAlignment, Vertical: .5, Child: woxwidget.TextBlock{
			Value: props.Title, MaxLines: 1, ShrinkWrap: true, AlignmentY: 0.5,
			Style: woxui.TextStyle{Size: (woxcomponent.ControlTheme{DensityScale: props.DensityScale}).Scaled(woxcomponent.SidebarTitleFontSize), Weight: woxui.FontWeightSemibold}, Color: theme.PreviewText,
		}}},
		{Right: contentRight + 6, AnchorRight: true, Child: woxwidget.Align{Width: NotesToolbarActionsWidth - 6, Height: NotesToolbarHeight, Vertical: 0.5, Child: right}},
	}
	if props.Platform != "darwin" {
		children = append(children, woxwidget.StackChild{Left: 12, Child: woxwidget.Align{Width: 20, Height: NotesToolbarHeight, Vertical: .5, Child: woxwidget.Image{Source: props.Icon, Width: 20, Height: 20}}})
	}
	children = append(children, woxwidget.StackChild{Child: woxcomponent.WindowCloseChrome(woxcomponent.WindowCloseChromeProps{
		ID: "notes.toolbar.close", Width: width, Platform: props.Platform, Theme: theme.Controls, Active: active, Maximized: props.Maximized,
		OnMinimize: props.OnMinimize, OnMaximize: props.OnMaximize, OnClose: props.OnClose,
	})})
	if props.OnSidebar != nil {
		left := float32(42)
		if props.Platform == "darwin" {
			left = woxcomponent.TitleBarChromeWidth(props.Platform, true, true)
		}
		sidebar := woxcomponent.SidebarToggle(woxcomponent.IconButtonProps{ID: "notes.toolbar.sidebar", Label: props.SidebarLabel, Width: 36, Height: 36, Radius: 7, HoverBackground: woxcomponent.TitleBarAlpha(theme.ResultSubtitle, 25), FocusRingColor: theme.Cursor, OnTap: func() { props.OnHover(false, "", woxui.Rect{}); props.OnSidebar() }, OnHoverAt: func(inside bool, bounds woxui.Rect) { props.OnHover(inside, props.SidebarLabel, bounds) }}, woxcomponent.SidebarTitleIconSize, theme.ResultSubtitle, theme.PreviewText, props.SidebarOpen)
		children = append(children, woxwidget.StackChild{Left: left, Child: woxwidget.Align{Width: 36, Height: NotesToolbarHeight, Vertical: .5, Child: sidebar}})
	}
	content := woxwidget.Stack{Width: width, Height: NotesToolbarHeight, Children: children}
	return woxwidget.Container{Width: width, Height: NotesToolbarHeight, Child: content}
}

// NotesTitleSlot places the window title like other Wox chrome: centered on
// macOS, left-aligned after the app icon on Windows and Linux.
func NotesTitleSlot(goos string, contentRight float32) (left, right, alignment float32) {
	if goos == "darwin" {
		return NotesToolbarActionsWidth, NotesToolbarActionsWidth, 0.5
	}
	return 40, contentRight + NotesToolbarActionsWidth, 0
}

// NotesFormatAction describes one document command in the format bar.
type NotesFormatAction struct {
	ID, Label string
	Active    bool
	OnTap     func()
}

type NotesFormatBarProps struct {
	Width, StatsWidth  float32
	Stats              string
	Markdown, ReadOnly bool
	Actions            []NotesFormatAction
	More               NotesFormatAction
	Theme              woxcomponent.Theme
	OnHover            func(bool, string, woxui.Rect)
}

// NotesFormatBar lays out prepared commands without reading editor state.
func NotesFormatBar(props NotesFormatBarProps) woxwidget.Widget {
	width, theme, stats := props.Width, props.Theme, props.Stats
	item := func(action NotesFormatAction) woxwidget.Widget {
		id, label := action.ID, action.Label
		onTap := func() {
			props.OnHover(false, "", woxui.Rect{})
			if action.OnTap != nil {
				action.OnTap()
			}
		}
		iconColor := theme.ToolbarText
		if action.Active && theme.Cursor.A != 0 {
			iconColor = theme.Cursor
		}
		return woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "notes.format." + id, Label: label, Icon: woxcomponent.FormatGlyph(id, 16, iconColor),
			Width: notesFormatButtonSize, Height: notesFormatButtonSize, Radius: 6, HoverBackground: woxcomponent.TitleBarAlpha(theme.ToolbarText, 20),
			Selected: action.Active, SelectedBackground: woxcomponent.TitleBarAlpha(theme.ToolbarText, 40),
			FocusRingColor: theme.Cursor, Disabled: props.ReadOnly, OnTap: onTap,
			OnHoverAt: func(inside bool, bounds woxui.Rect) { props.OnHover(inside, label, bounds) },
		})
	}
	items := make([]woxwidget.Widget, 0, len(props.Actions))
	for _, action := range props.Actions {
		items = append(items, item(action))
	}
	inner := max(float32(0), width-notesFormatBarPadLeft-notesFormatBarPadRight)
	tools := notesFormatToolsWidth(len(items))
	statsWidth := props.StatsWidth
	// Drop the count before collapsing marks. The 13-button row is 388 wide; at
	// the 460 default that already leaves too little room for the localized count.
	showStats := props.Markdown || inner >= tools+notesFormatBarStatsGap+statsWidth
	if !props.Markdown && inner < tools {
		items = append(items[:NotesFormatBarLeadCount], item(props.More))
		tools = notesFormatToolsWidth(len(items))
		showStats = inner >= tools+notesFormatBarStatsGap+statsWidth
	}
	row := []woxwidget.Widget{}
	if showStats {
		row = append(row, woxwidget.Semantics{
			Key: "notes.format.stats", AutomationID: "notes.format.stats", Role: woxui.AccessibilityRoleText, Label: stats,
			Child: woxwidget.Text{Value: stats, Style: woxui.TextStyle{Size: 11}, Color: theme.ResultSubtitle},
		})
	}
	if !props.Markdown {
		row = append(row, woxwidget.Expanded{Child: woxwidget.Align{
			Height: NotesFormatBarHeight, Horizontal: 1, Vertical: .5,
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: notesFormatButtonGap, Children: items},
		}})
	}
	return woxwidget.Container{Width: width, Height: NotesFormatBarHeight,
		Padding: woxwidget.Insets{Left: notesFormatBarPadLeft, Right: notesFormatBarPadRight},
		Child:   woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: notesFormatBarStatsGap, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: row}}
}

// notesFormatToolsWidth is the shrink-wrapped width of count format buttons plus gaps.
func notesFormatToolsWidth(count int) float32 {
	if count <= 0 {
		return 0
	}
	return float32(count)*notesFormatButtonSize + float32(count-1)*notesFormatButtonGap
}

// NotesSearchItem contains only the row's display data and activation callback.
type NotesSearchItem struct {
	ID, Label, Updated, Section string
	OnTap                       func()
}

type NotesSearchProps struct {
	Size                                       woxui.Size
	Items                                      []NotesSearchItem
	Selected                                   int
	Label, Hint, EmptyLabel, CloseLabel, Value string
	Editor                                     *woxwidget.TextEditingController
	Focus                                      *woxwidget.FocusNode
	Window                                     *woxui.Window
	Theme                                      woxcomponent.Theme
	OnChanged                                  func(string)
	OnKey                                      func(woxui.KeyEvent) bool
	OnDismiss                                  func()
}

// NotesSearchOverlay builds the browser from its immutable row snapshot.
func NotesSearchOverlay(props NotesSearchProps) woxwidget.Widget {
	size, theme := props.Size, props.Theme
	width := max(float32(260), size.Width-24)
	// Keep the panel below the title-bar inset so the overlay flex stays inside the window.
	height := min(float32(300), max(float32(0), size.Height-notesSearchOverlayTop))
	items := props.Items
	rows := make([]woxwidget.Widget, 0, len(items)+6)
	contentHeight := float32(0)
	var keepVisible *woxwidget.ScrollRange
	appendSearchRow := func(child woxwidget.Widget, height float32, selected bool) {
		if len(rows) > 0 {
			contentHeight += notesSearchListGap
		}
		if selected {
			keepVisible = &woxwidget.ScrollRange{Start: contentHeight, End: contentHeight + height}
		}
		rows = append(rows, child)
		contentHeight += height
	}
	section := ""
	for index, summary := range items {
		label := summary.Section
		if label != section {
			first := section == ""
			section = label
			headerHeight := NotesSearchSectionHeight
			if !first {
				headerHeight += NotesSearchSectionLead
			}
			appendSearchRow(NotesSearchSection(label, width-notesSearchOverlayPadding*2, theme, first), headerHeight, false)
		}
		selected := index == props.Selected
		appendSearchRow(NotesSearchRow(summary, width-notesSearchOverlayPadding*2, theme, selected), NotesSearchRowHeight, selected)
	}
	if len(rows) == 0 {
		empty := woxwidget.Container{Width: width - notesSearchOverlayPadding*2, Height: 48, Child: woxwidget.Align{Width: width - notesSearchOverlayPadding*2, Height: 48, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: props.EmptyLabel, Style: woxui.TextStyle{Size: 12}, Color: theme.ResultSubtitle}}}
		appendSearchRow(empty, 48, false)
	}
	innerWidth := width - notesSearchOverlayPadding*2
	scrollHeight := height - notesSearchOverlayPadding*2 - notesSearchFieldHeight - notesSearchOverlayGap
	search := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: "notes.search", Label: props.Label, Hint: props.Hint,
		Width: innerWidth, Height: notesSearchFieldHeight, Style: woxui.TextStyle{Size: 13}, Value: props.Value, Controller: props.Editor,
		FocusNode: props.Focus, Focused: true, Autofocus: true, MaxLines: 1, Window: props.Window, Theme: theme.Controls,
		OnChanged: props.OnChanged, OnKey: props.OnKey,
	})
	results := woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: "notes.search.results", AutomationID: "notes.search.results", Label: props.Label,
		Width: innerWidth, Height: scrollHeight, ContentHeight: contentHeight, KeepVisible: keepVisible,
		Content: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: notesSearchListGap, Children: rows},
		Theme:   theme.Controls, ThumbColor: theme.ResultSubtitle,
	})
	panel := woxwidget.Container{Width: width, Height: height, Radius: 10, Floating: true, Color: theme.ActionBackground, BorderColor: theme.PreviewSplit, BorderWidth: 1, Padding: woxwidget.UniformInsets(notesSearchOverlayPadding), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: notesSearchOverlayGap, Children: []woxwidget.Widget{
		search, results,
	}}}
	return NotesOverlayScrim(size, panel, 12, notesSearchOverlayTop, props.CloseLabel, props.OnDismiss)
}

// NotesSearchRow renders one prepared note summary.
func NotesSearchRow(item NotesSearchItem, width float32, theme woxcomponent.Theme, selected bool) woxwidget.Widget {
	radius := float32(6)
	background := theme.ActionBackground
	titleColor := theme.ActionText
	metaColor := theme.ResultSubtitle
	if selected {
		background = theme.SelectedBackground
		titleColor = theme.SelectedTitle
		metaColor = theme.SelectedSubtitle
	}
	hoverBackground := woxcomponent.ControlHoverColor(theme.ActionBackground, theme.ActionText)
	return woxcomponent.WoxListItem(woxcomponent.ListItemProps{
		ID: "notes.search." + item.ID, Label: item.Label, Width: width, Height: NotesSearchRowHeight, Radius: &radius,
		Background: &background, HoverBackground: &hoverBackground, Selected: selected, SkipFocus: true,
		Padding: woxwidget.Insets{Left: 8, Right: 8}, Theme: theme.Controls,
		OnTap: item.OnTap,
		Child: NotesSearchRowChild(item.Label, item.Updated, titleColor, metaColor),
	})
}

// NotesSearchRowChild centers the title and updated time in one search result row.
func NotesSearchRowChild(label, updated string, titleColor, metaColor woxui.Color) woxwidget.Widget {
	return woxwidget.Align{Height: NotesSearchRowHeight, Vertical: .5, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter,
		Children: []woxwidget.Widget{
			woxwidget.Expanded{Child: woxwidget.TextBlock{
				Value: label, Height: NotesSearchTextHeight, LineHeight: NotesSearchTextHeight, MaxLines: 1, AlignmentY: 0.5,
				Style: woxui.TextStyle{Size: 13}, Color: titleColor,
			}},
			woxwidget.TextBlock{
				Value: updated, Height: NotesSearchTextHeight, LineHeight: NotesSearchTextHeight, MaxLines: 1, AlignmentY: 0.5, ShrinkWrap: true,
				Style: woxui.TextStyle{Size: woxcomponent.TailFontSize}, Color: metaColor,
			},
		},
	}}
}

// NotesSearchSection renders a muted section label, with extra lead-in space after the first group.
func NotesSearchSection(label string, width float32, theme woxcomponent.Theme, first bool) woxwidget.Widget {
	top := float32(0)
	if !first {
		top = NotesSearchSectionLead
	}
	return woxwidget.Container{Width: width, Height: NotesSearchSectionHeight + top, Padding: woxwidget.Insets{Left: 8, Top: top}, Child: woxwidget.Align{Height: NotesSearchSectionHeight, Vertical: .5, Child: woxwidget.TextBlock{
		Value: label, Height: notesSearchSectionTextHeight, LineHeight: notesSearchSectionTextHeight, MaxLines: 1, AlignmentY: 0.5,
		Style: woxui.TextStyle{Size: woxcomponent.SettingsSectionTitleFontSize, Weight: woxui.FontWeightSemibold}, Color: theme.ActionHeader,
	}}}
}

// NotesMenuRow renders a prepared menu action.
func NotesMenuRow(id, label string, width float32, theme woxcomponent.Theme, icon woxwidget.Widget, action func()) woxwidget.Widget {
	scale := theme.Controls.Scaled
	radius := scale(6)
	background := theme.ActionBackground
	hoverBackground := woxcomponent.ControlHoverColor(background, theme.ActionText)
	children := []woxwidget.Widget{}
	if icon != nil {
		children = append(children, woxwidget.Align{Width: scale(16), Height: scale(32), Vertical: .5, Horizontal: .5, Child: icon})
	}
	children = append(children, woxwidget.Expanded{Child: woxwidget.TextBlock{Value: label, MaxLines: 1, ShrinkWrap: true, AlignmentY: .5, Style: woxui.TextStyle{Size: scale(13)}, Color: theme.ActionText}})
	return woxcomponent.WoxListItem(woxcomponent.ListItemProps{
		ID: "notes.menu." + id, Label: label, Width: width - scale(12), Height: scale(32), Radius: &radius,
		Background: &background, HoverBackground: &hoverBackground, SkipFocus: true,
		OnTap: action, Theme: theme.Controls,
		Padding: woxwidget.Insets{Left: scale(8), Right: scale(8)},
		Child:   woxwidget.Align{Height: scale(32), Vertical: .5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: scale(8), CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children}},
	})
}

// NotesMenuSeparator separates related actions without adding an interactive row.
func NotesMenuSeparator(width float32, theme woxcomponent.Theme) woxwidget.Widget {
	s := theme.Controls.Scaled
	return woxwidget.Align{Width: width - s(12), Height: s(9), Vertical: .5, Child: woxwidget.Container{Width: width - s(12), Height: 1, Color: theme.Controls.Border}}
}

// NotesOverlayScrim places a dismissible panel above the note.
func NotesOverlayScrim(size woxui.Size, panel woxwidget.Widget, left, top float32, closeLabel string, dismiss func()) woxwidget.Widget {
	return woxwidget.Stack{Width: size.Width, Height: size.Height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Semantics{AutomationID: "notes.overlay.dismiss", Role: woxui.AccessibilityRoleButton, Label: closeLabel, Child: woxwidget.Gesture{ID: "notes.overlay.dismiss", OnTap: dismiss, Child: woxwidget.Container{Width: size.Width, Height: size.Height, Color: woxui.Color{A: 70}}}}},
		{Left: left, Top: top, Child: panel},
	}}
}

// NotesMenuOverlay composes rows into the title-bar menu.
func NotesMenuOverlay(size woxui.Size, width float32, rows []woxwidget.Widget, theme woxcomponent.Theme, closeLabel string, dismiss func()) woxwidget.Widget {
	s := theme.Controls.Scaled
	contentHeight := float32(0)
	for _, row := range rows {
		if separator, ok := row.(woxwidget.Align); ok {
			contentHeight += separator.Height
		} else {
			contentHeight += s(32)
		}
	}
	panelHeight := min(contentHeight+s(12), max(float32(0), size.Height-s(16)))
	var content woxwidget.Widget = woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}
	if contentHeight > panelHeight-s(12) {
		content = woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{Key: "notes.menu.scroll", Width: max(float32(0), width-s(12)), Height: max(float32(0), panelHeight-s(12)), ContentHeight: contentHeight, Theme: theme.Controls, Content: content})
	}
	panel := woxwidget.Container{Width: width, Height: panelHeight, Radius: s(9), Floating: true, Color: theme.ActionBackground, BorderColor: theme.PreviewSplit, BorderWidth: 1, Padding: woxwidget.UniformInsets(s(6)), Child: content}
	top := min(float32(40), max(s(8), size.Height-panelHeight-s(8)))
	return NotesOverlayScrim(size, panel, max(s(8), size.Width-width-s(8)), top, closeLabel, dismiss)
}

type NotesLinkProps struct {
	Size                                              woxui.Size
	Label, CancelLabel, ApplyLabel, CloseLabel, Value string
	Editor                                            *woxwidget.TextEditingController
	Focus                                             *woxwidget.FocusNode
	Window                                            *woxui.Window
	Theme                                             woxcomponent.Theme
	OnKey                                             func(woxui.KeyEvent) bool
	OnDismiss, OnApply                                func()
}

// NotesLinkOverlay composes the link editor and its actions.
func NotesLinkOverlay(props NotesLinkProps) woxwidget.Widget {
	size, theme := props.Size, props.Theme
	width := min(float32(340), size.Width-32)
	field := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{ID: "notes.link", Label: props.Label, Hint: "https://", Width: width - 20, Height: 36, Style: woxui.TextStyle{Size: 13}, Value: props.Value, Controller: props.Editor, FocusNode: props.Focus, Focused: true, Autofocus: true, Window: props.Window, Theme: theme.Controls, OnKey: props.OnKey})
	buttons := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, MainAxisAlignment: woxwidget.MainAxisEnd, Children: []woxwidget.Widget{
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "notes.link.cancel", Label: props.CancelLabel, Width: 70, Theme: theme.Controls, OnTap: props.OnDismiss}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "notes.link.apply", Label: props.ApplyLabel, Width: 70, Theme: theme.Controls, Variant: woxcomponent.ButtonPrimary, OnTap: props.OnApply}),
	}}
	panel := woxwidget.Container{Width: width, Height: 100, Radius: 10, Floating: true, Color: theme.ActionBackground, BorderColor: theme.PreviewSplit, BorderWidth: 1, Padding: woxwidget.UniformInsets(10), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{field, buttons}}}
	return NotesOverlayScrim(size, panel, (size.Width-width)/2, 52, props.CloseLabel, props.OnDismiss)
}

// NotesStatus renders save progress or a retryable error.
func NotesStatus(width float32, text, retryLabel string, color woxui.Color, theme woxcomponent.Theme, retry func()) woxwidget.Widget {
	children := []woxwidget.Widget{woxwidget.Expanded{Child: woxwidget.Text{Value: text, Style: woxui.TextStyle{Size: 11}, Color: color}}}
	if retry != nil {
		children = append(children, woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "notes.retry", Label: retryLabel, Width: 52, FontSize: 11, Theme: theme.Controls, OnTap: retry}))
	}
	return woxwidget.Container{Width: width, Height: NotesStatusHeight, Padding: woxwidget.Insets{Left: 12, Right: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children}}
}

// NotesTooltipOverlay stacks a prepared tooltip above other transient panels.
func NotesTooltipOverlay(size woxui.Size, overlay, tooltip woxwidget.Widget, left, top float32) woxwidget.Widget {
	if tooltip == nil {
		return overlay
	}
	layer := woxwidget.StackChild{Left: left, Top: top, Child: tooltip}
	if overlay == nil {
		return woxwidget.Stack{Width: size.Width, Height: size.Height, Children: []woxwidget.StackChild{layer}}
	}
	return woxwidget.Stack{Width: size.Width, Height: size.Height, Children: []woxwidget.StackChild{
		{Child: overlay},
		layer,
	}}
}
