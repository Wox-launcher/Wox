package view

import (
	"fmt"
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	formTableEmojiCellSize      = float32(44)
	formTableEmojiCellGap       = float32(8)
	formTableEmojiTitleHeight   = float32(36)
	formTableEmojiTabHeight     = float32(42)
	formTableEmojiActionsHeight = float32(44)
)

// FormTableEmojiGroup is one translated emoji category shown by the picker.
type FormTableEmojiGroup struct {
	Label  string
	Emojis []string
}

// FormTableEmojiPickerProps contains the immutable catalog and dialog actions.
type FormTableEmojiPickerProps struct {
	OverlayWidth  float32
	OverlayHeight float32
	Window        *woxui.Window
	Theme         woxcomponent.Theme
	Groups        []FormTableEmojiGroup
	InitialEmoji  string
	Title         string
	CancelLabel   string
	CancelWidth   float32
	OnChoose      func(string)
	OnCancel      func()
}

// FormTableEmojiPicker builds the Flutter-aligned emoji dialog for woxImage row fields.
func FormTableEmojiPicker(props FormTableEmojiPickerProps) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: "form-table-emoji-picker", Type: (*formTableEmojiPickerState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &formTableEmojiPickerState{} },
	}
}

type formTableEmojiPickerState struct {
	group    int
	selected int
	hovered  int
	scroll   *woxwidget.ScrollController
}

// InitState starts at the group holding the current emoji, or the recommended group.
func (s *formTableEmojiPickerState) InitState(_ woxwidget.StateContext, widget any) {
	props := widget.(FormTableEmojiPickerProps)
	s.group, s.selected = formTableEmojiInitialSelection(props.Groups, props.InitialEmoji)
	s.hovered = -1
	s.scroll = woxwidget.NewScrollController(0)
}

// DidUpdateWidget keeps selection local while the committed value only changes on choose.
func (s *formTableEmojiPickerState) DidUpdateWidget(context woxwidget.StateContext, oldWidget, newWidget any) {
	oldProps := oldWidget.(FormTableEmojiPickerProps)
	props := newWidget.(FormTableEmojiPickerProps)
	if oldProps.Groups != nil && props.Groups != nil && len(oldProps.Groups) != len(props.Groups) {
		if s.group >= len(props.Groups) {
			s.group = 0
		}
	}
}

// Dispose leaves the scroll controller to detach its own host resources.
func (s *formTableEmojiPickerState) Dispose() {}

// Build renders the modal dialog with the current group and selection.
func (s *formTableEmojiPickerState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(FormTableEmojiPickerProps)
	if len(props.Groups) == 0 {
		return woxwidget.Container{}
	}
	if s.group < 0 || s.group >= len(props.Groups) {
		s.group = 0
	}
	emojis := props.Groups[s.group].Emojis
	if s.selected >= len(emojis) {
		s.selected = max(0, len(emojis)-1)
	}
	return s.buildDialog(context, props)
}

func (s *formTableEmojiPickerState) buildDialog(context woxwidget.StateContext, props FormTableEmojiPickerProps) woxwidget.Widget {
	panelWidth := min(float32(660), max(float32(0), props.OverlayWidth-64))
	panelHeight := min(float32(500), max(float32(0), props.OverlayHeight-56))
	innerWidth := max(float32(0), panelWidth-48)
	gridWidth := innerWidth
	columns := formTableEmojiColumns(gridWidth)
	tabHeight := s.groupTabsHeight(props, innerWidth)
	gridHeight := formTableEmojiGridHeight(panelHeight, tabHeight)

	title := woxwidget.Container{Width: innerWidth, Height: formTableEmojiTitleHeight, Child: woxwidget.Text{
		Value: props.Title, Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText,
	}}

	tabs := s.buildGroupTabs(context, props, innerWidth, tabHeight)

	cells := make([]woxwidget.Widget, 0, len(props.Groups[s.group].Emojis))
	for index, emoji := range props.Groups[s.group].Emojis {
		cells = append(cells, s.buildEmojiCell(context, props, index, emoji))
	}
	contentHeight := float32((len(cells)+columns-1)/columns)*(formTableEmojiCellSize+formTableEmojiCellGap) + formTableEmojiCellGap
	grid := woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: "form-table-emoji-scroll", Width: gridWidth, Height: gridHeight, ContentHeight: max(gridHeight, contentHeight),
		Controller: s.scroll, ThumbColor: props.Theme.ResultSubtitle,
		Content: woxwidget.Container{Width: gridWidth, Child: woxwidget.Wrap{Gap: formTableEmojiCellGap, RunGap: formTableEmojiCellGap, Children: cells}},
	})

	actions := woxwidget.Align{
		Width: innerWidth, Height: formTableEmojiActionsHeight, Horizontal: 1, Vertical: 0.5,
		Child: woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "form-table-emoji-cancel", Label: props.CancelLabel, Height: 38, Radius: 4, FontSize: 13, Variant: woxcomponent.ButtonOutline, OnTap: props.OnCancel, Theme: props.Theme}),
	}

	body := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{title, tabs, grid, actions}}
	focusable := woxwidget.Focusable{Key: "form-table-emoji-focus", OnKey: func(event woxui.KeyEvent) bool {
		return s.handleKey(context, props, event)
	}, Child: body}
	border := formTableAlpha(props.Theme.ResultSubtitle, 104)
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "form-table-emoji-dialog", Label: props.Title, Width: panelWidth, Height: panelHeight,
		OverlayWidth: props.OverlayWidth, OverlayHeight: props.OverlayHeight, BackdropID: "form-table-emoji-backdrop", BackdropAlpha: 210,
		Radius: 20, Padding: woxwidget.UniformInsets(24), BorderColor: border, BorderWidth: 0.75,
		InitialFocus: "form-table-emoji-focus", OnEscape: props.OnCancel, Theme: props.Theme,
		Child: focusable,
	})
}

// formTableEmojiGridHeight keeps the dialog body inside the padded panel so the
// action row never overflows, matching the shared dialog layout contract.
func formTableEmojiGridHeight(panelHeight, tabHeight float32) float32 {
	innerHeight := max(float32(0), panelHeight-48)
	return max(float32(80), innerHeight-formTableEmojiTitleHeight-tabHeight-formTableEmojiActionsHeight-30)
}

// groupTabsHeight accounts for translated labels wrapping onto a second row.
func (s *formTableEmojiPickerState) groupTabsHeight(props FormTableEmojiPickerProps, width float32) float32 {
	total := float32(0)
	for _, group := range props.Groups {
		labelWidth := float32(60)
		if props.Window != nil {
			if metrics, err := props.Window.MeasureText(group.Label, woxui.TextStyle{Size: 12}); err == nil {
				labelWidth = metrics.Size.Width + 28
			}
		}
		total += max(float32(52), labelWidth) + 8
	}
	rows := max(1, int(total)/max(1, int(width+8)))
	return float32(rows)*formTableEmojiTabHeight + float32(rows-1)*6
}

func (s *formTableEmojiPickerState) buildGroupTabs(context woxwidget.StateContext, props FormTableEmojiPickerProps, width, height float32) woxwidget.Widget {
	chips := make([]woxwidget.Widget, 0, len(props.Groups))
	for index, group := range props.Groups {
		labelWidth := float32(60)
		if props.Window != nil {
			if metrics, err := props.Window.MeasureText(group.Label, woxui.TextStyle{Size: 12}); err == nil {
				labelWidth = metrics.Size.Width + 28
			}
		}
		labelWidth = max(float32(52), labelWidth)
		selected := s.group == index
		background := props.Theme.ActionBackground
		foreground := props.Theme.ResultSubtitle
		if selected {
			background = props.Theme.ActionSelected
			background.A = uint8(float32(background.A) * 0.5)
			foreground = props.Theme.ActionText
		} else if s.hovered == index {
			background.A = uint8(float32(background.A) * 0.5)
		}
		activate := func() {
			context.SetState(func() {
				s.group = index
				s.selected = 0
				s.hovered = -1
				s.scroll.JumpTo(0)
			})
		}
		chips = append(chips, woxwidget.Gesture{ID: fmt.Sprintf("form-table-emoji-tab-%d", index), OnHover: func(inside bool) {
			if inside && s.hovered != index {
				context.SetState(func() { s.hovered = index })
			} else if !inside && s.hovered == index {
				context.SetState(func() { s.hovered = -1 })
			}
		}, OnTap: activate, Child: woxwidget.Container{
			Width: labelWidth, Height: height, Radius: 20, Color: background, Padding: woxwidget.UniformInsets(10),
			BorderColor: formTableAlpha(props.Theme.ResultSubtitle, 90), BorderWidth: 1, Child: woxwidget.Align{Width: labelWidth - 20, Height: height - 20, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
				Value: group.Label, Style: woxui.TextStyle{Size: 12}, Color: foreground,
			}},
		}})
	}
	return woxwidget.Wrap{Gap: 8, RunGap: 6, Children: chips}
}

func (s *formTableEmojiPickerState) buildEmojiCell(context woxwidget.StateContext, props FormTableEmojiPickerProps, index int, emoji string) woxwidget.Widget {
	selected := s.selected == index
	background := woxui.Color{}
	if selected {
		background = props.Theme.ActionSelected
		background.A = uint8(float32(background.A) * 0.5)
	} else if s.hovered == index {
		background = props.Theme.ActionSelected
		background.A = 24
	}
	activate := func() {
		if props.OnChoose != nil {
			props.OnChoose(emoji)
		}
	}
	cell := woxwidget.Gesture{ID: fmt.Sprintf("form-table-emoji-cell-%d", index), OnHover: func(inside bool) {
		if inside && s.hovered != index {
			context.SetState(func() { s.hovered = index })
		} else if !inside && s.hovered == index {
			context.SetState(func() { s.hovered = -1 })
		}
	}, OnTap: activate, Child: woxwidget.Container{
		Width: formTableEmojiCellSize, Height: formTableEmojiCellSize, Radius: 10, Color: background,
		BorderColor: formTableEmojiCellBorder(props.Theme, selected), BorderWidth: 1, Child: woxwidget.Align{Width: formTableEmojiCellSize - 2, Height: formTableEmojiCellSize - 2, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
			Value: emoji, Style: woxui.TextStyle{Size: 24}, Color: props.Theme.ActionText,
		}},
	}}
	return woxwidget.Semantics{
		Key: woxwidget.Key(fmt.Sprintf("form-table-emoji-cell-%d", index)), AutomationID: fmt.Sprintf("form-table-emoji-cell-%d", index),
		Role: woxui.AccessibilityRoleMenuItem, Label: emoji, Selected: selected,
		Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action == woxui.AccessibilityActionActivate {
				activate()
			}
			return nil
		}, Child: cell,
	}
}

// handleKey owns modal navigation while the emoji grid has focus.
func (s *formTableEmojiPickerState) handleKey(context woxwidget.StateContext, props FormTableEmojiPickerProps, event woxui.KeyEvent) bool {
	if len(props.Groups) == 0 || s.group < 0 || s.group >= len(props.Groups) {
		return false
	}
	emojis := props.Groups[s.group].Emojis
	if len(emojis) == 0 {
		return false
	}
	columns := formTableEmojiColumns(max(float32(0), min(float32(660), props.OverlayWidth-64)-48))
	switch event.Key {
	case woxui.KeyEscape:
		if props.OnCancel != nil {
			props.OnCancel()
		}
		return true
	case woxui.KeyArrowLeft:
		context.SetState(func() {
			s.selected = (s.selected - 1 + len(emojis)) % len(emojis)
			s.ensureSelectedVisible(columns, len(emojis))
		})
		return true
	case woxui.KeyArrowRight:
		context.SetState(func() {
			s.selected = (s.selected + 1) % len(emojis)
			s.ensureSelectedVisible(columns, len(emojis))
		})
		return true
	case woxui.KeyArrowUp:
		context.SetState(func() {
			s.selected = (s.selected - columns + len(emojis)) % len(emojis)
			s.ensureSelectedVisible(columns, len(emojis))
		})
		return true
	case woxui.KeyArrowDown:
		context.SetState(func() {
			s.selected = (s.selected + columns) % len(emojis)
			s.ensureSelectedVisible(columns, len(emojis))
		})
		return true
	case woxui.KeyEnter, woxui.KeySpace:
		if s.selected >= 0 && s.selected < len(emojis) && props.OnChoose != nil {
			props.OnChoose(emojis[s.selected])
		}
		return true
	default:
		return false
	}
}

func (s *formTableEmojiPickerState) ensureSelectedVisible(columns, count int) {
	row := s.selected / max(1, columns)
	s.scroll.EnsureVisible(float32(row)*(formTableEmojiCellSize+formTableEmojiCellGap), float32(row+1)*(formTableEmojiCellSize+formTableEmojiCellGap)+formTableEmojiCellGap)
}

// formTableEmojiColumns mirrors Flutter's adaptive cross-axis grid count.
func formTableEmojiColumns(width float32) int {
	return max(6, min(12, int(width)/int(formTableEmojiCellSize+formTableEmojiCellGap)))
}

// formTableEmojiInitialSelection finds the group and index of the current emoji.
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
		return theme.ActionSelected
	}
	return formTableAlpha(theme.ResultSubtitle, 60)
}
