package view

import (
	"strings"

	"wox/common"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const themeAutoPickerRowHeight = float32(64)

// ThemeAutoPickerProps contains the overlay used to replace one Auto theme endpoint.
type ThemeAutoPickerProps struct {
	OverlayWidth      float32
	OverlayHeight     float32
	Window            *woxui.Window
	Theme             woxcomponent.ControlTheme
	Title             string
	SearchPlaceholder string
	CancelLabel       string
	Themes            []ThemeCatalogItem
	OnChoose          func(string)
	OnCancel          func()
}

// ThemeAutoPickerView builds a searchable theme list over the installed catalog.
func ThemeAutoPickerView(props ThemeAutoPickerProps) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: "theme-auto-picker", Type: (*themeAutoPickerState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &themeAutoPickerState{} },
	}
}

type themeAutoPickerState struct {
	queryController  *woxwidget.TextEditingController
	queryFocusNode   *woxwidget.FocusNode
	scrollController *woxwidget.ScrollController
	hovered          int
}

// InitState creates picker-local search and scroll state.
func (s *themeAutoPickerState) InitState(_ woxwidget.StateContext, _ any) {
	s.queryController = woxwidget.NewTextEditingController("")
	s.queryFocusNode = woxwidget.NewFocusNode()
	s.scrollController = woxwidget.NewScrollController(0)
	s.hovered = -1
}

// DidUpdateWidget keeps the filter when the catalog refreshes.
func (s *themeAutoPickerState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

// Build filters installed palette themes for the current query.
func (s *themeAutoPickerState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(ThemeAutoPickerProps)
	visible := filteredAutoPickerThemes(props.Themes, s.queryController.Text())
	if s.hovered >= len(visible) {
		s.hovered = -1
	}
	return buildThemeAutoPickerDialog(context, props, s, visible)
}

// Dispose leaves child retained controls to detach their own host resources.
func (s *themeAutoPickerState) Dispose() {}

func filteredAutoPickerThemes(themes []ThemeCatalogItem, query string) []ThemeCatalogItem {
	query = strings.ToLower(strings.TrimSpace(query))
	visible := make([]ThemeCatalogItem, 0, len(themes))
	for _, theme := range themes {
		if query == "" || strings.Contains(strings.ToLower(theme.Name), query) {
			visible = append(visible, theme)
		}
	}
	return visible
}

func buildThemeAutoPickerDialog(context woxwidget.StateContext, props ThemeAutoPickerProps, state *themeAutoPickerState, visible []ThemeCatalogItem) woxwidget.Widget {
	panelWidth := min(float32(520), max(float32(0), props.OverlayWidth-64))
	panelHeight := min(float32(560), max(float32(0), props.OverlayHeight-56))
	innerWidth := max(float32(0), panelWidth-48)
	innerHeight := max(float32(0), panelHeight-48)
	const titleHeight = float32(36)
	const searchHeight = woxcomponent.SettingsSearchHeight
	listHeight := max(float32(48), innerHeight-titleHeight-searchHeight-SettingsDialogActionsHeight-12)
	title := woxwidget.Container{Width: innerWidth, Height: titleHeight, Child: woxwidget.Text{
		Value: props.Title, Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: props.Theme.Text,
	}}
	search := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: "theme-auto-picker-search", Label: props.SearchPlaceholder, Hint: props.SearchPlaceholder, Width: innerWidth, Height: searchHeight, Radius: 4,
		Padding: woxwidget.Insets{Left: 12, Top: 10, Right: 10, Bottom: 10}, Transparent: true,
		BorderColor: props.Theme.Border, BorderWidth: 1, Style: woxui.TextStyle{Size: 13},
		Controller: state.queryController, FocusNode: state.queryFocusNode, Autofocus: true, MaxLines: 1,
		Window: props.Window, Theme: props.Theme,
		OnChanged: func(string) {
			context.SetState(func() {
				state.hovered = -1
				state.scrollController.JumpTo(0)
			})
		},
	})
	rows := make([]woxwidget.Widget, 0, len(visible))
	for index, theme := range visible {
		item := theme
		background := woxui.Color{}
		if state.hovered == index {
			background = props.Theme.SelectionBackground
		}
		rows = append(rows, woxwidget.Gesture{ID: "theme-auto-pick-" + item.ID, OnTap: func() {
			if props.OnChoose != nil {
				props.OnChoose(item.ID)
			}
		}, OnHover: func(inside bool) {
			context.SetState(func() {
				if inside {
					state.hovered = index
				} else if state.hovered == index {
					state.hovered = -1
				}
			})
		}, Child: woxwidget.Container{Width: innerWidth, Height: themeAutoPickerRowHeight, Color: background, Padding: woxwidget.Insets{Left: 8, Right: 8}, Child: woxwidget.Align{Height: themeAutoPickerRowHeight, Vertical: 0.5, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				themeSwatch(item.PreviewTheme, common.ThemeSwatchSize),
				woxwidget.Text{Value: item.Name, Style: woxui.TextStyle{Size: 14}, Color: props.Theme.Text},
			},
		}}}})
	}
	var list woxwidget.Widget = woxwidget.Align{Width: innerWidth, Height: listHeight, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: props.SearchPlaceholder, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.TextSecondary}}
	if len(rows) > 0 {
		list = woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
			Key: "theme-auto-picker-scroll", Width: innerWidth, Height: listHeight, Controller: state.scrollController,
			Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}, Theme: props.Theme, ThumbColor: props.Theme.Text,
		})
	}
	cancel := woxwidget.Align{Width: innerWidth, Height: SettingsDialogActionsHeight, Horizontal: 1, Vertical: 0.5, Child: woxcomponent.WoxButton(woxcomponent.ButtonProps{
		ID: "theme-auto-picker-cancel", Label: props.CancelLabel, IntrinsicWidth: true, Variant: woxcomponent.ButtonSecondary, OnTap: props.OnCancel, Theme: props.Theme,
	})}
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "theme-auto-picker-dialog", Label: props.Title, Width: panelWidth, Height: panelHeight,
		OverlayWidth: props.OverlayWidth, OverlayHeight: props.OverlayHeight, BackdropID: "theme-auto-picker-backdrop", BackdropAlpha: 210,
		Solid: true, Radius: 16, Padding: woxwidget.UniformInsets(24), Theme: props.Theme, InitialFocus: "theme-auto-picker-search", OnEscape: props.OnCancel,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: []woxwidget.Widget{title, search, list, cancel}},
	})
}
