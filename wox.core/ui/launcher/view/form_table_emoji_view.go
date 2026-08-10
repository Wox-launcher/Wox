package view

import (
	"fmt"
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/emojisearch"
)

const (
	formTableEmojiPanelWidth       = float32(760)
	formTableEmojiPanelHeight      = float32(540)
	formTableEmojiHeaderHeight     = float32(42)
	formTableEmojiSidebarWidth     = float32(150)
	formTableEmojiSidebarRowHeight = float32(40)
	formTableEmojiCellSize         = float32(46)
	formTableEmojiCellGap          = float32(8)
	formTableEmojiSearchLimit      = 100
)

// FormTableEmojiGroup is one translated emoji category shown by the picker.
type FormTableEmojiGroup struct {
	Label  string
	Marker string
	Emojis []string
}

// FormTableEmojiPickerProps contains the immutable catalog and dialog actions.
type FormTableEmojiPickerProps struct {
	OverlayWidth       float32
	OverlayHeight      float32
	Window             *woxui.Window
	Theme              woxcomponent.Theme
	Groups             []FormTableEmojiGroup
	SearchEntries      []emojisearch.Entry
	InitialEmoji       string
	Title              string
	SearchLabel        string
	SearchResultsLabel string
	NoResultsLabel     string
	CloseLabel         string
	SearchIcon         *woxui.Image
	OnChoose           func(string)
	OnCancel           func()
}

// FormTableEmojiPicker builds the searchable sidebar-and-grid emoji dialog.
func FormTableEmojiPicker(props FormTableEmojiPickerProps) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: "form-table-emoji-picker", Type: (*formTableEmojiPickerState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &formTableEmojiPickerState{} },
	}
}

type formTableEmojiPickerState struct {
	group         int
	selected      int
	hoveredGroup  int
	hoveredEmoji  int
	gridActive    bool
	query         *woxwidget.TextEditingController
	sidebarScroll *woxwidget.ScrollController
	gridScroll    *woxwidget.ScrollController
}

// InitState starts at the group holding the current emoji and prepares retained search and scroll state.
func (s *formTableEmojiPickerState) InitState(_ woxwidget.StateContext, widget any) {
	props := widget.(FormTableEmojiPickerProps)
	s.group, s.selected = formTableEmojiInitialSelection(props.Groups, props.InitialEmoji)
	s.hoveredGroup = -1
	s.hoveredEmoji = -1
	s.query = woxwidget.NewTextEditingController("")
	s.sidebarScroll = woxwidget.NewScrollController(0)
	s.gridScroll = woxwidget.NewScrollController(0)
}

// DidUpdateWidget keeps selection valid when the catalog changes.
func (s *formTableEmojiPickerState) DidUpdateWidget(_ woxwidget.StateContext, oldWidget, newWidget any) {
	oldProps := oldWidget.(FormTableEmojiPickerProps)
	props := newWidget.(FormTableEmojiPickerProps)
	if len(oldProps.Groups) != len(props.Groups) && s.group >= len(props.Groups) {
		s.group = 0
		s.selected = 0
	}
}

// Dispose leaves retained controllers to detach from their host.
func (s *formTableEmojiPickerState) Dispose() {}

// Build renders the latest catalog and local filter state.
func (s *formTableEmojiPickerState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(FormTableEmojiPickerProps)
	if len(props.Groups) == 0 {
		return woxwidget.Container{}
	}
	if s.group < 0 || s.group >= len(props.Groups) {
		s.group = 0
	}
	_, emojis := s.visibleEmojis(props)
	if s.selected >= len(emojis) {
		s.selected = max(0, len(emojis)-1)
	}
	return s.buildDialog(context, props)
}

func (s *formTableEmojiPickerState) buildDialog(context woxwidget.StateContext, props FormTableEmojiPickerProps) woxwidget.Widget {
	panelWidth := min(formTableEmojiPanelWidth, max(float32(0), props.OverlayWidth-56))
	panelHeight := min(formTableEmojiPanelHeight, max(float32(0), props.OverlayHeight-56))
	innerWidth := max(float32(0), panelWidth-48)
	innerHeight := max(float32(0), panelHeight-48)
	contentHeight := max(float32(0), innerHeight-formTableEmojiHeaderHeight-12)

	searchWidth := min(float32(320), max(float32(180), innerWidth*0.45))
	header := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Expanded{Child: woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText}},
		woxcomponent.WoxSearchField(woxcomponent.SearchFieldProps{
			ID: "form-table-emoji-search", Label: props.SearchLabel, Width: searchWidth, Value: s.query.Text(), Autofocus: true,
			Controller: s.query, SearchIcon: props.SearchIcon, Window: props.Window, Theme: props.Theme,
			OnFocus: func() { context.RequestFocus(woxwidget.Key("form-table-emoji-search")) }, OnClear: func() {
				context.SetState(func() {
					s.query.SetText("", true)
					s.selected = 0
					s.gridActive = false
					s.gridScroll.JumpTo(0)
				})
			}, OnChanged: func(string) {
				context.SetState(func() {
					s.selected = 0
					s.hoveredEmoji = -1
					s.gridActive = false
					s.gridScroll.JumpTo(0)
				})
			}, OnKey: func(event woxui.KeyEvent) bool { return s.handleKey(context, props, event) },
		}),
		woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "form-table-emoji-close", Label: props.CloseLabel, Icon: woxcomponent.CloseGlyph(16, props.Theme.ResultSubtitle),
			Width: 32, Height: 32, Radius: 6, HoverBackground: formTableAlpha(props.Theme.ResultSubtitle, 25), FocusRingColor: props.Theme.Cursor, OnTap: props.OnCancel,
		}),
	}}

	sidebarWidth := min(formTableEmojiSidebarWidth, max(float32(112), innerWidth*0.24))
	sidebar := s.buildSidebar(context, props, sidebarWidth, contentHeight)
	main := woxwidget.Expanded{Child: woxwidget.LayoutBuilder{Build: func(size woxui.Size) woxwidget.Widget {
		return s.buildEmojiGrid(context, props, size.Width, contentHeight)
	}}}
	body := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 20, Children: []woxwidget.Widget{
		sidebar,
		woxwidget.Container{Width: 1, Height: contentHeight, Color: formTableAlpha(props.Theme.PreviewSplit, 150)},
		main,
	}}
	content := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: []woxwidget.Widget{header, body}}
	border := formTableAlpha(props.Theme.ResultSubtitle, 104)
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "form-table-emoji-dialog", Label: props.Title, Width: panelWidth, Height: panelHeight,
		OverlayWidth: props.OverlayWidth, OverlayHeight: props.OverlayHeight, BackdropID: "form-table-emoji-backdrop", BackdropAlpha: 210,
		Radius: 20, Padding: woxwidget.UniformInsets(24), BorderColor: border, BorderWidth: 0.75,
		InitialFocus: "form-table-emoji-search", OnEscape: props.OnCancel, Theme: props.Theme, Child: content,
	})
}

func (s *formTableEmojiPickerState) buildSidebar(context woxwidget.StateContext, props FormTableEmojiPickerProps, width, height float32) woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0, len(props.Groups))
	queryActive := strings.TrimSpace(s.query.Text()) != ""
	for index, group := range props.Groups {
		selected := !queryActive && s.group == index
		background := woxui.Color{}
		foreground := props.Theme.ResultSubtitle
		if selected {
			background = formTableAlpha(props.Theme.ActionSelected, 44)
			foreground = props.Theme.ActionText
		} else if s.hoveredGroup == index {
			background = formTableAlpha(props.Theme.ResultSubtitle, 20)
		}
		groupIndex := index
		activate := func() {
			context.SetState(func() {
				s.group = groupIndex
				s.selected = 0
				s.hoveredEmoji = -1
				s.gridActive = false
				s.query.SetText("", true)
				s.gridScroll.JumpTo(0)
			})
		}
		marker := woxwidget.Align{Width: 26, Height: formTableEmojiSidebarRowHeight, Vertical: 0.5, Child: woxwidget.Text{
			Value: group.Marker, Style: woxui.TextStyle{Size: 16}, Color: foreground,
		}}
		row := woxwidget.Gesture{ID: fmt.Sprintf("form-table-emoji-group-%d", index), OnTap: activate, OnHover: func(inside bool) {
			context.SetState(func() {
				if inside {
					s.hoveredGroup = groupIndex
				} else if s.hoveredGroup == groupIndex {
					s.hoveredGroup = -1
				}
			})
		}, Child: woxwidget.Container{Width: width, Height: formTableEmojiSidebarRowHeight, Radius: 6, Color: background, Padding: woxwidget.Insets{Left: 8, Right: 8}, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				marker,
				woxwidget.TextBlock{Value: group.Label, Width: max(float32(0), width-50), Height: 18, MaxLines: 1, Style: woxui.TextStyle{Size: 12, Weight: boolWeight(selected)}, Color: foreground},
			},
		}}}
		rows = append(rows, woxwidget.Semantics{
			AutomationID: fmt.Sprintf("form-table-emoji-group-%d", index), Role: woxui.AccessibilityRoleMenuItem, Label: group.Label, Selected: selected,
			Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}, OnAction: func(action woxui.AccessibilityAction, _ string) error {
				if action == woxui.AccessibilityActionActivate {
					activate()
				}
				return nil
			}, Child: row,
		})
	}
	return woxwidget.Container{Width: width, Height: height, Child: woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: "form-table-emoji-groups-scroll", Width: width, Height: height, Controller: s.sidebarScroll,
		Content: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: rows}, ThumbColor: props.Theme.ResultSubtitle,
	})}
}

func (s *formTableEmojiPickerState) buildEmojiGrid(context woxwidget.StateContext, props FormTableEmojiPickerProps, width, height float32) woxwidget.Widget {
	label, emojis := s.visibleEmojis(props)
	headerHeight := float32(28)
	gridHeight := max(float32(0), height-headerHeight-8)
	countColor := formTableAlpha(props.Theme.ResultSubtitle, 170)
	header := woxwidget.Container{Width: width, Height: headerHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText},
		woxwidget.Text{Value: fmt.Sprintf("%d", len(emojis)), Style: woxui.TextStyle{Size: 11}, Color: countColor},
	}}}
	if len(emojis) == 0 {
		return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{
			header,
			woxwidget.Align{Width: width, Height: gridHeight, Horizontal: 0.5, Vertical: 0.42, Child: woxwidget.Text{Value: props.NoResultsLabel, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle}},
		}}
	}
	columns := formTableEmojiColumns(width)
	cells := make([]woxwidget.Widget, 0, len(emojis))
	for index, emoji := range emojis {
		cells = append(cells, s.buildEmojiCell(context, props, index, emoji))
	}
	grid := woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: "form-table-emoji-scroll", Width: width, Height: gridHeight, Controller: s.gridScroll, ThumbColor: props.Theme.ResultSubtitle,
		Content: woxwidget.Grid{Width: width, Columns: columns, CellWidth: formTableEmojiCellSize, CellHeight: formTableEmojiCellSize,
			ColumnGap: formTableEmojiCellGap, RowGap: formTableEmojiCellGap, Children: cells},
	})
	return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{header, grid}}
}

func (s *formTableEmojiPickerState) visibleEmojis(props FormTableEmojiPickerProps) (string, []string) {
	query := strings.ToLower(strings.TrimSpace(s.query.Text()))
	if query == "" {
		return props.Groups[s.group].Label, props.Groups[s.group].Emojis
	}
	if len(props.SearchEntries) > 0 {
		entries := emojisearch.Filter(props.SearchEntries, query, formTableEmojiSearchLimit)
		matches := make([]string, len(entries))
		for index, entry := range entries {
			matches[index] = entry.Emoji
		}
		return props.SearchResultsLabel, matches
	}
	seen := make(map[string]struct{})
	matches := make([]string, 0, 64)
	for _, group := range props.Groups {
		matchGroup := strings.Contains(strings.ToLower(group.Label), query)
		for _, emoji := range group.Emojis {
			if !matchGroup && !strings.Contains(emoji, query) {
				continue
			}
			if _, exists := seen[emoji]; exists {
				continue
			}
			seen[emoji] = struct{}{}
			matches = append(matches, emoji)
		}
	}
	return props.SearchResultsLabel, matches
}

func (s *formTableEmojiPickerState) buildEmojiCell(context woxwidget.StateContext, props FormTableEmojiPickerProps, index int, emoji string) woxwidget.Widget {
	selected := s.selected == index
	background := formTableAlpha(props.Theme.ResultSubtitle, 8)
	if selected {
		background = formTableAlpha(props.Theme.ActionSelected, 36)
	} else if s.hoveredEmoji == index {
		background = formTableAlpha(props.Theme.ResultSubtitle, 22)
	}
	activate := func() {
		if props.OnChoose != nil {
			props.OnChoose(emoji)
		}
	}
	cellIndex := index
	cell := woxwidget.Gesture{ID: fmt.Sprintf("form-table-emoji-cell-%d", index), OnHover: func(inside bool) {
		context.SetState(func() {
			if inside {
				s.hoveredEmoji = cellIndex
			} else if s.hoveredEmoji == cellIndex {
				s.hoveredEmoji = -1
			}
		})
	}, OnTap: activate, Child: woxwidget.Container{
		Width: formTableEmojiCellSize, Height: formTableEmojiCellSize, Radius: 8, Color: background,
		BorderColor: formTableEmojiCellBorder(props.Theme, selected), BorderWidth: 1, Child: woxwidget.Align{
			Width: formTableEmojiCellSize - 2, Height: formTableEmojiCellSize - 2, Horizontal: 0.5, Vertical: 0.5,
			Child: woxwidget.Text{Value: emoji, Style: woxui.TextStyle{Size: 24}, Color: props.Theme.ActionText},
		},
	}}
	return woxwidget.Semantics{
		Key: woxwidget.Key(fmt.Sprintf("form-table-emoji-cell-%d", index)), AutomationID: fmt.Sprintf("form-table-emoji-cell-%d", index),
		Role: woxui.AccessibilityRoleMenuItem, Label: emoji, Selected: selected, Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action == woxui.AccessibilityActionActivate {
				activate()
			}
			return nil
		}, Child: cell,
	}
}

// handleKey moves from search into the grid without taking ordinary text-editing arrows.
func (s *formTableEmojiPickerState) handleKey(context woxwidget.StateContext, props FormTableEmojiPickerProps, event woxui.KeyEvent) bool {
	if !event.Down || event.Composing || len(props.Groups) == 0 {
		return false
	}
	_, emojis := s.visibleEmojis(props)
	if event.Key == woxui.KeyEscape {
		if props.OnCancel != nil {
			props.OnCancel()
		}
		return true
	}
	if len(emojis) == 0 {
		return false
	}
	mainWidth := max(float32(0), min(formTableEmojiPanelWidth, props.OverlayWidth-56)-48-formTableEmojiSidebarWidth-21-20)
	columns := formTableEmojiColumns(mainWidth)
	if !s.gridActive {
		if event.Key != woxui.KeyArrowDown {
			return false
		}
		context.SetState(func() {
			s.gridActive = true
			s.selected = min(max(0, s.selected), len(emojis)-1)
			s.ensureSelectedVisible(columns)
		})
		return true
	}
	switch event.Key {
	case woxui.KeyArrowLeft:
		s.moveSelection(context, -1, len(emojis), columns)
	case woxui.KeyArrowRight:
		s.moveSelection(context, 1, len(emojis), columns)
	case woxui.KeyArrowUp:
		s.moveSelection(context, -columns, len(emojis), columns)
	case woxui.KeyArrowDown:
		s.moveSelection(context, columns, len(emojis), columns)
	case woxui.KeyEnter, woxui.KeySpace:
		if s.selected >= 0 && s.selected < len(emojis) && props.OnChoose != nil {
			props.OnChoose(emojis[s.selected])
		}
	default:
		return false
	}
	return true
}

func (s *formTableEmojiPickerState) moveSelection(context woxwidget.StateContext, delta, count, columns int) {
	context.SetState(func() {
		s.selected = (s.selected + delta + count) % count
		s.ensureSelectedVisible(columns)
	})
}

func (s *formTableEmojiPickerState) ensureSelectedVisible(columns int) {
	row := s.selected / max(1, columns)
	s.gridScroll.EnsureVisible(float32(row)*(formTableEmojiCellSize+formTableEmojiCellGap), float32(row+1)*(formTableEmojiCellSize+formTableEmojiCellGap)+formTableEmojiCellGap)
}

func formTableEmojiColumns(width float32) int {
	return max(6, min(10, int(width)/int(formTableEmojiCellSize+formTableEmojiCellGap)))
}

func formTableEmojiInitialSelection(groups []FormTableEmojiGroup, initialEmoji string) (int, int) {
	initialEmoji = strings.TrimSpace(initialEmoji)
	if initialEmoji == "" || strings.HasPrefix(initialEmoji, "{") {
		return 0, 0
	}
	for group, current := range groups {
		for index, emoji := range current.Emojis {
			if emoji == initialEmoji {
				return group, index
			}
		}
	}
	return 0, 0
}

func formTableEmojiCellBorder(theme woxcomponent.Theme, selected bool) woxui.Color {
	if selected {
		return theme.Cursor
	}
	return formTableAlpha(theme.ResultSubtitle, 44)
}

func boolWeight(enabled bool) woxui.FontWeight {
	if enabled {
		return woxui.FontWeightSemibold
	}
	return woxui.FontWeightRegular
}
