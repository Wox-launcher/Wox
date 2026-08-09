package view

import (
	"fmt"
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const formAppPickerRowHeight = float32(48)

// FormAppCandidate is the display data required for one application picker row.
type FormAppCandidate struct {
	Name     string
	Identity string
	Detail   string
	Icon     *woxui.Image
}

// FormAppPickerProps contains the immutable application catalog and dialog actions.
type FormAppPickerProps struct {
	OverlayWidth      float32
	OverlayHeight     float32
	Window            *woxui.Window
	Theme             woxcomponent.Theme
	Title             string
	SearchPlaceholder string
	LoadingLabel      string
	EmptyLabel        string
	CancelLabel       string
	ConfirmLabel      string
	CancelWidth       float32
	ConfirmWidth      float32
	Candidates        []FormAppCandidate
	SelectedIdentity  string
	Loading           bool
	Error             string
	OnConfirm         func(int)
	OnCancel          func()
}

// FormAppPickerView builds the retained application picker dialog.
func FormAppPickerView(props FormAppPickerProps) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: "form-table-app-picker", Type: (*formAppPickerState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &formAppPickerState{} },
	}
}

type formAppPickerState struct {
	queryController  *woxwidget.TextEditingController
	queryFocusNode   *woxwidget.FocusNode
	scrollController *woxwidget.ScrollController
	selectedIdentity string
	hovered          int
}

// InitState creates dialog-local search, selection, focus, and scroll state.
func (s *formAppPickerState) InitState(_ woxwidget.StateContext, widget any) {
	props := widget.(FormAppPickerProps)
	s.queryController = woxwidget.NewTextEditingController("")
	s.queryFocusNode = woxwidget.NewFocusNode()
	s.scrollController = woxwidget.NewScrollController(0)
	s.selectedIdentity = normalizedFormAppIdentity(props.SelectedIdentity)
	s.hovered = -1
}

// DidUpdateWidget follows a changed committed app while retaining the user's filter.
func (s *formAppPickerState) DidUpdateWidget(_ woxwidget.StateContext, oldWidget, newWidget any) {
	oldProps := oldWidget.(FormAppPickerProps)
	props := newWidget.(FormAppPickerProps)
	if !strings.EqualFold(strings.TrimSpace(oldProps.SelectedIdentity), strings.TrimSpace(props.SelectedIdentity)) {
		s.selectedIdentity = normalizedFormAppIdentity(props.SelectedIdentity)
		s.hovered = -1
		s.scrollController.JumpTo(0)
	}
}

// Build filters the latest candidate catalog without moving committed values into view state.
func (s *formAppPickerState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(FormAppPickerProps)
	visible := filteredFormAppCandidates(props.Candidates, s.queryController.Text())
	if s.hovered >= len(visible) {
		s.hovered = -1
	}
	return buildFormAppPickerDialog(context, props, s, visible)
}

// Dispose leaves child retained controls to detach their own host resources.
func (s *formAppPickerState) Dispose() {}

type visibleFormAppCandidate struct {
	candidate     FormAppCandidate
	originalIndex int
}

func filteredFormAppCandidates(candidates []FormAppCandidate, query string) []visibleFormAppCandidate {
	query = strings.ToLower(strings.TrimSpace(query))
	visible := make([]visibleFormAppCandidate, 0, len(candidates))
	for index, candidate := range candidates {
		if query == "" || strings.Contains(strings.ToLower(candidate.Name), query) || strings.Contains(strings.ToLower(candidate.Identity), query) || strings.Contains(strings.ToLower(candidate.Detail), query) {
			visible = append(visible, visibleFormAppCandidate{candidate: candidate, originalIndex: index})
		}
	}
	return visible
}

func normalizedFormAppIdentity(identity string) string {
	return strings.ToLower(strings.TrimSpace(identity))
}

func buildFormAppPickerDialog(context woxwidget.StateContext, props FormAppPickerProps, state *formAppPickerState, visible []visibleFormAppCandidate) woxwidget.Widget {
	panelWidth := min(float32(808), max(float32(0), props.OverlayWidth-64))
	panelHeight := min(float32(640), max(float32(0), props.OverlayHeight-56))
	innerWidth := max(float32(0), panelWidth-48)
	innerHeight := max(float32(0), panelHeight-48)
	const titleHeight = float32(36)
	const searchHeight = float32(42)
	const actionsHeight = float32(62)
	errorHeight := float32(0)
	if props.Error != "" {
		errorHeight = 28
	}
	listHeight := max(float32(48), innerHeight-titleHeight-searchHeight-actionsHeight-errorHeight-12)

	title := woxwidget.Container{Width: innerWidth, Height: titleHeight, Child: woxwidget.Text{
		Value: props.Title, Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText,
	}}
	search := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: "form-table-app-search", Label: props.SearchPlaceholder, Hint: props.SearchPlaceholder, Width: innerWidth, Height: searchHeight, Radius: 4,
		Padding: woxwidget.Insets{Left: 12, Top: 11, Right: 10, Bottom: 10}, Transparent: true,
		BorderColor: formAppPickerAlpha(props.Theme.ResultSubtitle, 170), BorderWidth: 1,
		Style: woxui.TextStyle{Size: 13}, Controller: state.queryController, FocusNode: state.queryFocusNode, Autofocus: true, MaxLines: 1,
		Window: props.Window, Theme: props.Theme, OnKey: func(event woxui.KeyEvent) bool { return state.handleKey(context, props, visible, event) },
		OnChanged: func(string) {
			context.SetState(func() {
				state.hovered = -1
				state.scrollController.JumpTo(0)
			})
		},
	})
	content := []woxwidget.Widget{title, search}
	if props.Error != "" {
		content = append(content, woxwidget.Container{Width: innerWidth, Height: errorHeight, Padding: woxwidget.Insets{Top: 10}, Child: woxwidget.TextBlock{
			Value: props.Error, Width: innerWidth, Height: 16, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle,
		}})
	}
	content = append(content, woxwidget.Container{Width: innerWidth, Height: 12}, formAppPickerList(context, props, state, visible, innerWidth, listHeight))

	confirm := func() { state.confirm(props) }
	actions := woxwidget.Align{Width: innerWidth, Height: 38, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "form-table-app-cancel", Label: props.CancelLabel, Height: 38, Radius: 4, FontSize: 13, Variant: woxcomponent.ButtonOutline, OnTap: props.OnCancel, Theme: props.Theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "form-table-app-confirm", Label: props.ConfirmLabel, Height: 38, Radius: 4, FontSize: 13, Variant: woxcomponent.ButtonPrimary, OnTap: confirm, Theme: props.Theme}),
	}}}
	content = append(content, woxwidget.Container{Width: innerWidth, Height: actionsHeight, Padding: woxwidget.Insets{Top: 12}, Child: actions})
	border := formAppPickerAlpha(props.Theme.PreviewSplit, 230)
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "form-table-app-dialog", Label: props.Title, Width: panelWidth, Height: panelHeight,
		OverlayWidth: props.OverlayWidth, OverlayHeight: props.OverlayHeight, BackdropID: "form-table-app-backdrop", BackdropAlpha: 210,
		Radius: 20, Padding: woxwidget.UniformInsets(24), BorderColor: border, BorderWidth: 1,
		InitialFocus: "form-table-app-search", OnEscape: props.OnCancel, Theme: props.Theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: content},
	})
}

func formAppPickerList(context woxwidget.StateContext, props FormAppPickerProps, state *formAppPickerState, visible []visibleFormAppCandidate, width, height float32) woxwidget.Widget {
	var body woxwidget.Widget
	if props.Loading {
		body = formAppPickerMessage(props.LoadingLabel, props.Theme, width, height)
	} else if len(visible) == 0 {
		body = formAppPickerMessage(props.EmptyLabel, props.Theme, width, height)
	} else {
		rows := make([]woxwidget.Widget, 0, len(visible)*2-1)
		for index, item := range visible {
			rows = append(rows, formAppPickerRow(context, props, state, item, index, width))
			if index < len(visible)-1 {
				rows = append(rows, woxwidget.Container{Width: width, Height: 1, Color: formAppPickerAlpha(props.Theme.PreviewSplit, 128)})
			}
		}
		contentHeight := float32(len(visible))*formAppPickerRowHeight + float32(len(visible)-1)
		body = woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
			Key: "form-table-app-scroll", Width: width, Height: height, ContentHeight: max(height, contentHeight), Controller: state.scrollController,
			Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}, ThumbColor: props.Theme.ResultSubtitle,
		})
	}
	border := woxwidget.Container{Width: width, Height: height, Radius: 12, BorderColor: formAppPickerAlpha(props.Theme.PreviewSplit, 230), BorderWidth: 1}
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{{Child: body}, {Child: border}}}
}

func formAppPickerMessage(value string, theme woxcomponent.Theme, width, height float32) woxwidget.Widget {
	return woxwidget.Align{Width: width, Height: height, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
		Value: value, Style: woxui.TextStyle{Size: 13}, Color: theme.ResultSubtitle,
	}}
}

func formAppPickerRow(context woxwidget.StateContext, props FormAppPickerProps, state *formAppPickerState, item visibleFormAppCandidate, index int, width float32) woxwidget.Widget {
	selected := state.selectedIdentity != "" && normalizedFormAppIdentity(item.candidate.Identity) == state.selectedIdentity
	background := woxui.Color{}
	if selected {
		background = formAppPickerAlpha(props.Theme.ActionSelected, 46)
	} else if state.hovered == index {
		background = formAppPickerAlpha(props.Theme.ActionSelected, 15)
	}
	activate := func() {
		context.SetState(func() {
			state.selectedIdentity = normalizedFormAppIdentity(item.candidate.Identity)
			state.scrollController.EnsureVisible(float32(index)*formAppPickerRowHeight, float32(index+1)*formAppPickerRowHeight)
		})
	}
	contentWidth := max(float32(0), width-24-48)
	children := []woxwidget.Widget{
		woxwidget.Align{Width: 48, Height: formAppPickerRowHeight, Vertical: 0.5, Child: formAppPickerRadio(selected, props.Theme)},
	}
	if item.candidate.Icon != nil {
		children = append(children, woxwidget.Align{Width: 40, Height: formAppPickerRowHeight, Vertical: 0.5, Child: woxwidget.Image{Source: item.candidate.Icon, Width: 28, Height: 28, Fit: woxwidget.ImageFitContain}})
		contentWidth -= 40
	}
	weight := woxui.FontWeightRegular
	if selected {
		weight = woxui.FontWeightSemibold
	}
	children = append(children, woxwidget.Container{Width: contentWidth, Height: formAppPickerRowHeight, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
			woxwidget.TextBlock{Value: item.candidate.Name, Width: contentWidth, Height: 18, LineHeight: 18, MaxLines: 1, Style: woxui.TextStyle{Size: 13, Weight: weight}, Color: props.Theme.ActionText},
			woxwidget.TextBlock{Value: item.candidate.Detail, Width: contentWidth, Height: 15, LineHeight: 15, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle},
		},
	}})
	key := woxwidget.Key(fmt.Sprintf("form-table-app-%d", item.originalIndex))
	row := woxwidget.Gesture{ID: string(key), OnTap: activate, OnHover: func(inside bool) {
		context.SetState(func() {
			if inside {
				state.hovered = index
			} else if state.hovered == index {
				state.hovered = -1
			}
		})
	}, Child: woxwidget.Container{Width: width, Height: formAppPickerRowHeight, Color: background, Padding: woxwidget.Insets{Left: 12, Right: 12}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: children}}}
	return woxwidget.Semantics{
		Key: key, AutomationID: string(key), Role: woxui.AccessibilityRoleRadioButton, Label: item.candidate.Name,
		Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}, Selected: selected, Checked: selected,
		OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action == woxui.AccessibilityActionActivate || action == woxui.AccessibilityActionToggle {
				activate()
			}
			return nil
		},
		Child: row,
	}
}

func formAppPickerRadio(selected bool, theme woxcomponent.Theme) woxwidget.Widget {
	color := formAppPickerAlpha(theme.ResultSubtitle, 191)
	if selected {
		color = theme.ActionSelected
	}
	return woxwidget.Painter{Width: 20, Height: 20, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		displayList.StrokeRoundedRect(bounds, 10, 1.5, color)
		if selected {
			displayList.FillRoundedRect(woxui.Rect{X: bounds.X + 6, Y: bounds.Y + 6, Width: 8, Height: 8}, 4, color)
		}
	}}
}

func (s *formAppPickerState) confirm(props FormAppPickerProps) {
	index := -1
	for candidateIndex, candidate := range props.Candidates {
		if s.selectedIdentity != "" && normalizedFormAppIdentity(candidate.Identity) == s.selectedIdentity {
			index = candidateIndex
			break
		}
	}
	if props.OnConfirm != nil {
		props.OnConfirm(index)
	}
}

func (s *formAppPickerState) handleKey(context woxwidget.StateContext, props FormAppPickerProps, visible []visibleFormAppCandidate, event woxui.KeyEvent) bool {
	if !event.Down || event.Composing {
		return false
	}
	switch event.Key {
	case woxui.KeyEscape:
		if props.OnCancel != nil {
			props.OnCancel()
		}
		return true
	case woxui.KeyArrowUp, woxui.KeyArrowDown:
		if len(visible) == 0 {
			return true
		}
		current := -1
		for index, item := range visible {
			if normalizedFormAppIdentity(item.candidate.Identity) == s.selectedIdentity {
				current = index
				break
			}
		}
		next := 0
		if current < 0 && event.Key == woxui.KeyArrowUp {
			next = len(visible) - 1
		} else if current >= 0 {
			delta := -1
			if event.Key == woxui.KeyArrowDown {
				delta = 1
			}
			next = (current + delta + len(visible)) % len(visible)
		}
		context.SetState(func() {
			s.selectedIdentity = normalizedFormAppIdentity(visible[next].candidate.Identity)
			s.hovered = -1
			s.scrollController.EnsureVisible(float32(next)*formAppPickerRowHeight, float32(next+1)*formAppPickerRowHeight)
		})
		return true
	case woxui.KeyEnter:
		s.confirm(props)
		return true
	}
	return false
}

func formAppPickerAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}
