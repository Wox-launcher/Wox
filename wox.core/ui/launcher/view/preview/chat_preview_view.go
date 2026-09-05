package preview

import (
	"math"
	"strings"
	"time"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ChatHistoryRowHeight keeps drawer layout and controller scrolling in logical units.
const ChatHistoryRowHeight = float32(38)

const chatCopyFeedbackDuration = 1200 * time.Millisecond

const (
	chatComposerHeight  = float32(98)
	chatQuoteCardHeight = float32(56)
)

// ChatComposerHeight returns the chat input pane height, including a quote card when present.
func ChatComposerHeight(attachmentCount int) float32 {
	return chatComposerHeight + float32(min(attachmentCount, 3))*chatQuoteCardHeight
}

// ChatPreviewProps contains the typed chat panes and optional catalog drawer.
type ChatPreviewProps struct {
	Width     float32
	Height    float32
	Key       string
	Panel     string
	Header    *ChatHeaderProps
	Messages  ChatMessagesProps
	Debug     *ChatDebugProps
	Question  *ChatQuestionProps
	Input     ChatInputProps
	History   *ChatCatalogProps
	Catalog   *ChatCatalogProps
	OnDismiss func()
}

// ChatPreview builds the chat reading flow and floating catalog layers.
func ChatPreview(props ChatPreviewProps) woxwidget.Widget {
	headerHeight := float32(52)
	inputHeight := props.Input.Height
	if inputHeight <= 0 {
		inputHeight = ChatComposerHeight(len(props.Input.Attachments))
	}
	innerWidth := max(float32(0), props.Width-20)
	innerHeight := max(float32(0), props.Height-14)
	children := make([]woxwidget.Widget, 0, 5)
	if props.Header != nil {
		children = append(children, ChatHeader(*props.Header))
	} else {
		headerHeight = 0
	}
	children = append(children, ChatMessages(props.Messages))
	if props.Debug != nil {
		children = append(children, ChatDebug(*props.Debug))
	}
	if props.Question != nil {
		children = append(children, ChatQuestion(*props.Question))
	}
	children = append(children, ChatInput(props.Input))
	history := props.History
	overlay := props.Catalog
	if history == nil && props.Panel == "history" {
		history = props.Catalog
		overlay = nil
	}
	if history != nil {
		contentWidth := max(float32(0), props.Width-history.Width)
		content := woxwidget.Container{Width: contentWidth, Height: props.Height, Padding: woxwidget.Insets{Left: 10, Top: 6, Right: 10, Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
		if overlay == nil {
			return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{ChatCatalog(*history), content}}
		}
		contentInnerWidth := max(float32(0), contentWidth-20)
		contentInnerHeight := max(float32(0), props.Height-14)
		left := 10 + (contentInnerWidth-overlay.Width)/2
		top := 6 + max(headerHeight, contentInnerHeight-inputHeight-overlay.Height-6)
		return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			ChatCatalog(*history),
			woxwidget.Stack{Width: contentWidth, Height: props.Height, Children: []woxwidget.StackChild{
				{Child: content},
				{Child: woxwidget.Gesture{ID: "chat-panel-dismiss-" + props.Key, OnTap: props.OnDismiss, Child: woxwidget.Container{Width: contentWidth, Height: props.Height}}},
				{Left: left, Top: top, Child: ChatCatalog(*overlay)},
			}},
		}}
	}
	layers := []woxwidget.StackChild{{Left: 10, Top: 6, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}}
	if overlay != nil {
		layers = append(layers, woxwidget.StackChild{Child: woxwidget.Gesture{ID: "chat-panel-dismiss-" + props.Key, OnTap: props.OnDismiss, Child: woxwidget.Container{Width: props.Width, Height: props.Height}}})
		left := 10 + (innerWidth-overlay.Width)/2
		top := 6 + max(headerHeight, innerHeight-inputHeight-overlay.Height-6)
		layers = append(layers, woxwidget.StackChild{Left: left, Top: top, Child: ChatCatalog(*overlay)})
	}
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: layers}
}

// ChatHeaderProps contains the current conversation title and header actions.
type ChatHeaderProps struct {
	Width             float32
	Height            float32
	Key               string
	Title             string
	ShowDebug         bool
	DebugOpen         bool
	ShowExit          bool
	ShowOpenWindow    bool
	ExitLabel         string
	OpenWindowLabel   string
	HistoryLabel      string
	HistoryTooltip    string
	Theme             woxcomponent.Theme
	OnHistory         func()
	OnDebug           func()
	OnExit            func()
	OnOpenWindow      func()
	OnDrag            func()
	OnDoubleTap       func()
	OnExitHover       func(bool, string, woxui.Rect)
	OnOpenWindowHover func(bool, string, woxui.Rect)
	OnHistoryHover    func(bool, string, woxui.Rect)
}

// ChatHeader builds the compact chat title bar.
func ChatHeader(props ChatHeaderProps) woxwidget.Widget {
	menuHoverBackground := props.Theme.ResultSubtitle
	menuHoverBackground.A = uint8(float32(menuHoverBackground.A) * 0.1)
	debugWidth := float32(0)
	if props.ShowDebug {
		debugWidth = 48
	}
	exitWidth := float32(0)
	if props.ShowExit {
		exitWidth = 40
	}
	openWindowWidth := float32(0)
	if props.ShowOpenWindow {
		openWindowWidth = 40
	}
	title := woxwidget.Gesture{ID: "chat-title-drag-" + props.Key, OnDragStart: props.OnDrag, OnDoubleTap: props.OnDoubleTap, Child: woxwidget.Align{
		Height: 36, Horizontal: 0, Vertical: 0.5,
		Child: woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText},
	}}
	children := []woxwidget.StackChild{
		{Child: woxwidget.Gesture{ID: "chat-titlebar-drag-" + props.Key, OnDragStart: props.OnDrag, OnDoubleTap: props.OnDoubleTap, Child: woxwidget.Container{Width: props.Width, Height: props.Height}}},
		{Left: 2, Child: woxwidget.Align{Width: 36, Height: props.Height, Vertical: 0.5, Child: woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "chat-history-" + props.Key, Label: props.HistoryLabel, Icon: woxcomponent.SidebarGlyph(20, props.Theme.ResultSubtitle), Width: 36, Height: 36, Radius: 7,
			HoverBackground: menuHoverBackground, FocusRingColor: props.Theme.Cursor, OnTap: props.OnHistory,
			OnHoverAt: func(inside bool, bounds woxui.Rect) {
				if props.OnHistoryHover != nil {
					props.OnHistoryHover(inside, props.HistoryTooltip, bounds)
				}
			},
		})}},
		{Left: 44, Right: 4 + debugWidth + openWindowWidth + exitWidth, StretchWidth: true, Child: woxwidget.Align{Height: props.Height, Vertical: 0.5, Child: title}},
	}
	if props.ShowDebug {
		debugBackground := woxui.Color{}
		if props.DebugOpen {
			debugBackground = props.Theme.ActionBackground
		}
		children = append(children, woxwidget.StackChild{Right: 12 + openWindowWidth + exitWidth, AnchorRight: true, Child: woxwidget.Align{Width: 28, Height: props.Height, Vertical: 0.5, Child: woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "chat-debug-" + props.Key, Label: "Debug trace", Icon: woxcomponent.DebugGlyph(16, props.Theme.ResultSubtitle),
			Width: 28, Height: 28, Radius: 7, Background: debugBackground, HoverBackground: menuHoverBackground, FocusRingColor: props.Theme.Cursor, OnTap: props.OnDebug,
		})}})
	}
	if props.ShowOpenWindow {
		hoverBackground := props.Theme.ResultSubtitle
		hoverBackground.A = uint8(float32(hoverBackground.A) * 0.1)
		button := woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "chat-open-window-" + props.Key, Label: props.OpenWindowLabel, Icon: woxcomponent.OpenWindowGlyph(16, props.Theme.ResultSubtitle), Width: 28, Height: 28, Radius: 14,
			HoverBackground: hoverBackground, FocusRingColor: props.Theme.Cursor, OnTap: props.OnOpenWindow, OnHoverAt: func(inside bool, bounds woxui.Rect) {
				if props.OnOpenWindowHover != nil {
					props.OnOpenWindowHover(inside, props.OpenWindowLabel, bounds)
				}
			},
		})
		children = append(children, woxwidget.StackChild{Right: 6 + exitWidth, AnchorRight: true, Child: woxwidget.Align{Width: 28, Height: props.Height, Vertical: 0.5, Child: button}})
	}
	if props.ShowExit {
		hoverBackground := props.Theme.ResultSubtitle
		hoverBackground.A = uint8(float32(hoverBackground.A) * 0.1)
		button := woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "chat-exit-" + props.Key, Label: props.ExitLabel, Icon: woxcomponent.CloseGlyph(16, props.Theme.ResultSubtitle), Width: 28, Height: 28, Radius: 14,
			HoverBackground: hoverBackground, FocusRingColor: props.Theme.Cursor, OnTap: props.OnExit, OnHoverAt: func(inside bool, bounds woxui.Rect) {
				if props.OnExitHover != nil {
					props.OnExitHover(inside, props.ExitLabel, bounds)
				}
			},
		})
		children = append(children, woxwidget.StackChild{Right: 6, AnchorRight: true, Child: woxwidget.Align{Width: 28, Height: props.Height, Vertical: 0.5, Child: button}})
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Child: woxwidget.Stack{Width: props.Width, Height: props.Height, Children: children}}
}

// chatHeaderButton applies the compact selected state shared by chat header actions.
func chatHeaderButton(id, label string, selected bool, theme woxcomponent.Theme, action func()) woxwidget.Widget {
	variant := woxcomponent.ButtonSurface
	if selected {
		variant = woxcomponent.ButtonSelected
	}
	return woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: id, Label: label, Radius: 7, FontSize: 10, Variant: variant, OnTap: action, Theme: theme})
}

// ChatCatalogItemProps contains one selectable history, model, or skill entry.
type ChatCatalogItemProps struct {
	SelectID    string
	DeleteID    string
	Kind        string
	Title       string
	Subtitle    string
	DeleteLabel string
	// ConfirmDeleteLabel describes the destructive second activation for assistive technology.
	ConfirmDeleteLabel  string
	GroupLabel          string
	Selected            bool
	Current             bool
	OnSelect            func()
	OnDelete            func()
	deleteFocused       bool
	onDeleteFocusChange func(bool)
}

// ChatCatalogProps contains a typed history, model, or skill catalog.
type ChatCatalogProps struct {
	Width         float32
	Height        float32
	Key           string
	Label         string
	Items         []ChatCatalogItemProps
	EmptyMessage  string
	Scroll        float32
	ContentHeight float32
	ShowNew       bool
	NewLabel      string
	Theme         woxcomponent.Theme
	OnScroll      func(float32)
	OnNew         func()
}

// ChatCatalog builds a floating chat catalog.
func ChatCatalog(props ChatCatalogProps) woxwidget.Widget {
	if props.ShowNew {
		return chatHistoryCatalog(props)
	}
	const rowHeight = float32(38)
	innerWidth := max(float32(0), props.Width-20)
	viewportHeight := max(float32(40), props.Height-14)
	children := make([]woxwidget.Widget, 0, 2)
	header := woxwidget.Widget(nil)
	if props.Label != "" {
		viewportHeight = max(float32(40), viewportHeight-30)
		header = woxwidget.Container{Width: innerWidth, Height: 28, Padding: woxwidget.Insets{Left: 4, Top: 6}, Child: woxwidget.Text{Value: props.Label, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionHeader}}
	}
	if props.ShowNew {
		header = woxwidget.Stack{Width: innerWidth, Height: 28, Children: []woxwidget.StackChild{
			{Top: 5, Right: 54, StretchWidth: true, Child: woxwidget.Container{Height: 18, Child: woxwidget.Text{Value: props.Label, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionHeader}}},
			{AnchorRight: true, Right: 0, Child: chatHeaderButton("chat-new-"+props.Key, props.NewLabel, false, props.Theme, props.OnNew)},
		}}
	}
	if header != nil {
		children = append(children, header)
	}
	rows := make([]woxwidget.Widget, 0, max(1, len(props.Items)))
	groupLabel := ""
	for _, item := range props.Items {
		if item.GroupLabel != "" && item.GroupLabel != groupLabel {
			groupLabel = item.GroupLabel
			rows = append(rows, woxwidget.Container{Width: innerWidth, Height: 28, Padding: woxwidget.Insets{Left: 4, Top: 6}, Child: woxwidget.Text{Value: groupLabel, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionHeader}})
		}
		rows = append(rows, ChatCatalogItem(item, innerWidth, rowHeight, props.Theme))
	}
	if len(rows) == 0 {
		rows = append(rows, woxwidget.Container{Width: innerWidth, Height: viewportHeight, Padding: woxwidget.Insets{Left: 10, Top: 18, Right: 10}, Child: woxwidget.TextBlock{Value: props.EmptyMessage, Height: 48, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: props.Theme.ResultSubtitle}})
	}
	border := props.Theme.ResultSubtitle
	border.A = uint8(float32(border.A) * 0.14)
	children = append(children, woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: woxwidget.Key("chat-catalog-scroll-" + props.Key), Width: innerWidth, Height: viewportHeight, ContentHeight: props.ContentHeight,
		Offset: props.Scroll, Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}, ThumbColor: props.Theme.ResultTitle, OnScroll: props.OnScroll,
	}))
	return woxwidget.Container{Width: props.Width, Height: props.Height, Radius: 9, Color: props.Theme.ActionBackground, BorderColor: border, BorderWidth: 1, Padding: woxwidget.Insets{Left: 10, Top: 7, Right: 10, Bottom: 7}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
}

// chatHistoryCatalog builds Flutter's fixed-width full-height conversation drawer.
func chatHistoryCatalog(props ChatCatalogProps) woxwidget.Widget {
	innerWidth := max(float32(0), props.Width-20)
	viewportHeight := max(float32(40), props.Height-24)
	rows := []woxwidget.Widget{
		ChatCatalogItem(ChatCatalogItemProps{SelectID: "chat-new-" + props.Key, Kind: "history-new", Title: props.NewLabel, OnSelect: props.OnNew}, innerWidth, 38, props.Theme),
		woxwidget.Container{Width: innerWidth, Height: 8},
	}
	groupLabel := ""
	for _, item := range props.Items {
		if item.GroupLabel != "" && item.GroupLabel != groupLabel {
			groupLabel = item.GroupLabel
			rows = append(rows, woxwidget.Container{Width: innerWidth, Height: 32, Padding: woxwidget.Insets{Left: 12, Top: 10, Bottom: 6}, Child: woxwidget.Text{Value: groupLabel, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle}})
		}
		rows = append(rows, ChatCatalogItem(item, innerWidth, ChatHistoryRowHeight, props.Theme))
	}
	if len(props.Items) == 0 {
		rows = append(rows, woxwidget.Container{Width: innerWidth, Height: 40, Padding: woxwidget.Insets{Left: 12, Top: 10}, Child: woxwidget.Text{Value: props.EmptyMessage, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle}})
	}
	thumbColor := props.Theme.ResultSubtitle
	thumbColor.A = uint8(float32(thumbColor.A) * 0.5)
	scroll := woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: woxwidget.Key("chat-catalog-scroll-" + props.Key), Width: innerWidth, Height: viewportHeight, ContentHeight: props.ContentHeight,
		Offset: props.Scroll, Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}, ThumbColor: thumbColor, OnScroll: props.OnScroll,
	})
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 10, Top: 12, Right: 10, Bottom: 12}, Child: scroll}},
		{AnchorRight: true, Child: woxwidget.Container{Width: 1, Height: props.Height, Color: props.Theme.PreviewSplit}},
	}}
}

type chatCatalogItemState struct {
	hovered       bool
	deleteHovered bool
	deleteFocused bool
	deleteConfirm bool
}

type chatCatalogItemWidget struct {
	item   ChatCatalogItemProps
	width  float32
	height float32
	theme  woxcomponent.Theme
}

// ChatCatalogItem retains pointer hover independently for each catalog row.
func ChatCatalogItem(item ChatCatalogItemProps, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: woxwidget.Key(item.SelectID), Type: (*chatCatalogItemState)(nil), Widget: chatCatalogItemWidget{item: item, width: width, height: height, theme: theme},
		CreateState: func() woxwidget.State { return &chatCatalogItemState{} },
	}
}

func (s *chatCatalogItemState) InitState(_ woxwidget.StateContext, _ any) {}

func (s *chatCatalogItemState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

func (s *chatCatalogItemState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(chatCatalogItemWidget)
	props.item.deleteFocused = s.deleteFocused
	props.item.onDeleteFocusChange = func(focused bool) {
		if focused != s.deleteFocused {
			context.SetState(func() {
				s.deleteFocused = focused
				if !focused {
					s.deleteConfirm = false
				}
			})
		}
	}
	return chatCatalogItemWithDeleteState(props.item, props.width, props.height, props.theme, s.hovered, s.deleteHovered, s.deleteConfirm, func(inside bool) {
		if inside != s.hovered {
			context.SetState(func() { s.hovered = inside })
		}
	}, func(inside bool) {
		if inside != s.deleteHovered || !inside && s.deleteConfirm {
			context.SetState(func() { s.setDeleteHovered(inside) })
		}
	}, func() {
		deleteConfirmed := false
		context.SetState(func() { deleteConfirmed = s.advanceDeleteConfirmation() })
		if deleteConfirmed && props.item.OnDelete != nil {
			props.item.OnDelete()
		}
	})
}

func (s *chatCatalogItemState) setDeleteHovered(inside bool) {
	s.deleteHovered = inside
	if !inside {
		s.deleteConfirm = false
	}
}

// advanceDeleteConfirmation requires two consecutive activations before deletion.
func (s *chatCatalogItemState) advanceDeleteConfirmation() bool {
	if !s.deleteConfirm {
		s.deleteConfirm = true
		return false
	}
	s.deleteConfirm = false
	return true
}

func (s *chatCatalogItemState) Dispose() {}

// chatCatalogItem renders the shared two-line catalog row and optional delete target.
func chatCatalogItem(item ChatCatalogItemProps, width, height float32, theme woxcomponent.Theme, hovered bool, onHover func(bool)) woxwidget.Widget {
	return chatCatalogItemWithDeleteState(item, width, height, theme, hovered, false, false, onHover, nil, item.OnDelete)
}

func chatCatalogItemWithDeleteState(item ChatCatalogItemProps, width, height float32, theme woxcomponent.Theme, hovered, deleteHovered, deleteConfirm bool, onHover, onDeleteHover func(bool), onDelete func()) woxwidget.Widget {
	if item.Kind == "history" || item.Kind == "history-new" {
		return chatHistoryItemWithDeleteState(item, width, height, theme, hovered, deleteHovered, deleteConfirm, onHover, onDeleteHover, onDelete)
	}
	background := woxui.Color{}
	if item.Selected {
		background = theme.SelectedBackground
	} else if hovered && item.OnDelete == nil {
		background = theme.SelectedBackground
		background.A = 40
	}
	mainWidth := width
	rightPadding := float32(10)
	if item.OnDelete != nil {
		mainWidth = max(float32(80), width-44)
		rightPadding = 8
	}
	if item.OnDelete == nil {
		iconColor := theme.PreviewText
		if item.Selected {
			iconColor = theme.SelectedTitle
		}
		checkWidth := float32(0)
		check := woxwidget.Widget(nil)
		if item.Current {
			checkWidth = 28
			check = woxcomponent.CheckGlyph(18, iconColor)
		}
		titleWidth := min(float32(220), max(float32(100), width*0.42))
		icon := woxcomponent.ModelTrainingGlyph(18, iconColor)
		if item.Kind == "skills" {
			icon = woxcomponent.ExtensionGlyph(18, iconColor)
		}
		return woxwidget.Gesture{ID: item.SelectID, OnTap: item.OnSelect, OnHover: onHover, Child: woxwidget.Container{
			Width: width, Height: height, Color: background, Child: woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
				{Left: 14, Top: 10, Child: icon},
				{Left: 42, Top: 11, Child: woxwidget.Container{Width: titleWidth, Height: 18, Child: woxwidget.Text{Value: item.Title, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: iconColor}}},
				{Left: 50 + titleWidth, Top: 11, Right: checkWidth + 8, StretchWidth: true, Child: woxwidget.Container{Height: 18, Child: woxwidget.Text{Value: item.Subtitle, Style: woxui.TextStyle{Size: 11}, Color: theme.ResultSubtitle}}},
				{Top: 10, AnchorRight: true, Child: woxwidget.Container{Width: checkWidth, Height: 18, Child: check}},
			}},
		}}
	}
	main := woxwidget.Gesture{ID: item.SelectID, OnTap: item.OnSelect, Child: woxwidget.Container{
		Width: mainWidth, Height: height - 4, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 5, Right: rightPadding}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, CrossAxisAlignment: woxwidget.CrossAxisStretch, Children: []woxwidget.Widget{
			woxwidget.Container{Height: 16, Child: woxwidget.Text{Value: item.Title, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: theme.PreviewText}},
			woxwidget.Container{Height: 14, Child: woxwidget.Text{Value: item.Subtitle, Style: woxui.TextStyle{Size: 9}, Color: theme.ResultSubtitle}},
		}},
	}}
	children := []woxwidget.Widget{main}
	if item.OnDelete != nil {
		hoverBackground := theme.ResultSubtitle
		hoverBackground.A = uint8(float32(hoverBackground.A) * 0.1)
		children = append(children, woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: item.DeleteID, Label: item.DeleteLabel, Icon: woxcomponent.CloseGlyph(14, theme.ResultSubtitle), Width: 40, Height: height - 4, Radius: 7,
			Background: theme.QueryBackground, HoverBackground: hoverBackground, FocusRingColor: theme.Cursor, OnTap: item.OnDelete,
		}))
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Bottom: 4}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, Children: children}}
}

func chatHistoryItem(item ChatCatalogItemProps, width, height float32, theme woxcomponent.Theme, hovered bool, onHover func(bool)) woxwidget.Widget {
	return chatHistoryItemWithDeleteState(item, width, height, theme, hovered, false, false, onHover, nil, item.OnDelete)
}

func chatHistoryItemWithDeleteState(item ChatCatalogItemProps, width, height float32, theme woxcomponent.Theme, hovered, deleteHovered, deleteConfirm bool, onHover, onDeleteHover func(bool), onDelete func()) woxwidget.Widget {
	background := woxui.Color{}
	radius := float32(6)
	selected := hovered || item.Selected
	if selected {
		background = theme.SelectedBackground
	}
	if item.Kind == "history-new" {
		iconColor := theme.ActionHeader
		iconColor.A = 200
		titleColor := theme.PreviewText
		if selected {
			iconColor = theme.SelectedSubtitle
			titleColor = theme.SelectedTitle
		}
		return woxwidget.Gesture{OnHover: onHover, Child: woxcomponent.WoxListItem(woxcomponent.ListItemProps{
			ID: item.SelectID, Label: item.Title, OnTap: item.OnSelect, Width: width, Height: height,
			Background: &background, HoverBackground: &theme.SelectedBackground, Radius: &radius, Theme: theme,
			Padding: woxwidget.Insets{Left: 12, Right: 12},
			Child: woxwidget.Align{Height: height, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				woxcomponent.AddGlyph(18, iconColor),
				woxwidget.Text{Value: item.Title, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: titleColor},
			}}},
		})}
	}
	titleColor := theme.PreviewText
	deleteColor := theme.ResultSubtitle
	if selected {
		titleColor = theme.SelectedTitle
		deleteColor = theme.SelectedSubtitle
	}
	rowHeight := height - 4
	dangerColor := theme.ErrorText
	dangerHover := dangerColor
	dangerHover.A = uint8(float32(dangerHover.A) * 0.16)
	deleteWidth := float32(28)
	deleteLabel := item.DeleteLabel
	deleteBackground := woxui.Color{}
	deleteHoverBackground := dangerHover
	deleteIcon := woxcomponent.DeleteGlyph(15, deleteColor)
	if deleteHovered {
		deleteIcon = woxcomponent.DeleteGlyph(15, dangerColor)
	}
	if deleteConfirm {
		if item.ConfirmDeleteLabel != "" {
			deleteLabel = item.ConfirmDeleteLabel
		}
		deleteBackground = dangerColor
		deleteHoverBackground = dangerColor
		deleteIcon = woxcomponent.CheckGlyph(14, theme.SelectedTitle)
	}
	// Keep the button mounted so keyboard focus can reveal the otherwise quiet action.
	if !hovered && !deleteHovered && !item.deleteFocused && !deleteConfirm {
		deleteIcon = nil
	}
	deleteButton := woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
		ID: item.DeleteID, Label: deleteLabel, Icon: deleteIcon, Width: deleteWidth, Height: 28, Radius: 4,
		Background: deleteBackground, HoverBackground: deleteHoverBackground, FocusRingColor: theme.Cursor, OnFocusChange: item.onDeleteFocusChange, OnTap: onDelete, OnHoverAt: func(inside bool, _ woxui.Rect) {
			if onDeleteHover != nil {
				onDeleteHover(inside)
			}
		},
	})
	row := woxwidget.Gesture{OnHover: onHover, Child: woxcomponent.WoxListItem(woxcomponent.ListItemProps{
		ID: item.SelectID, Label: item.Title, OnTap: item.OnSelect, Selected: item.Selected,
		Width: width, Height: rowHeight, Background: &background, HoverBackground: &theme.SelectedBackground, Radius: &radius, Theme: theme,
		Padding: woxwidget.Insets{Left: 12, Right: deleteWidth + 16},
		Child:   woxwidget.Align{Height: rowHeight, Vertical: 0.5, Child: woxwidget.Text{Value: item.Title, Style: woxui.TextStyle{Size: 13}, Color: titleColor}},
	})}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Bottom: 4}, Child: woxwidget.Stack{Width: width, Height: rowHeight, Children: []woxwidget.StackChild{
		{Child: row},
		{Right: 8, AnchorRight: true, Child: woxwidget.Align{Width: deleteWidth, Height: rowHeight, Vertical: 0.5, Child: deleteButton}},
	}}}
}

// ChatDebugProps contains the laid-out trace and copy action.
type ChatDebugProps struct {
	Width             float32
	Height            float32
	Key               string
	Summary           string
	Value             string
	Layout            woxwidget.TextBlockLayout
	Scroll            float32
	Theme             woxcomponent.Theme
	OnScroll          func(float32)
	OnGeometryChanged func(viewport, content float32)
	OnCopy            func()
}

// ChatDebug builds the portable JSON trace panel.
func ChatDebug(props ChatDebugProps) woxwidget.Widget {
	header := woxwidget.Stack{Height: 24, Children: []woxwidget.StackChild{
		{Right: 54, StretchWidth: true, Child: woxwidget.Container{Height: 24, Child: woxwidget.Text{Value: props.Summary, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionHeader}}},
		{AnchorRight: true, Right: 0, Child: chatHeaderButton("chat-debug-copy-"+props.Key, "Copy", false, props.Theme, props.OnCopy)},
	}}
	body := woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: woxwidget.Key("chat-debug-scroll-" + props.Key), FillWidth: true, FillHeight: true,
		Offset: props.Scroll, ThumbColor: props.Theme.ResultTitle, OnScroll: props.OnScroll, OnGeometryChanged: props.OnGeometryChanged,
		Content: woxwidget.Constrained{FillWidth: true, Child: woxwidget.Container{
			Radius: 7, Color: props.Theme.QueryBackground, Padding: woxwidget.Insets{Left: 8, Top: 8, Right: 8, Bottom: 8},
			Child: woxwidget.TextBlock{Value: props.Value, Height: props.Layout.Size.Height, Style: woxui.TextStyle{Size: 10}, LineHeight: 16, Color: props.Theme.PreviewText, Layout: &props.Layout},
		}},
	})
	return woxwidget.Container{Width: props.Width, Height: props.Height, Radius: 9, Color: props.Theme.ActionBackground, Padding: woxwidget.Insets{Left: 10, Top: 7, Right: 10, Bottom: 7}, Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 4, CrossAxisAlignment: woxwidget.CrossAxisStretch, Children: []woxwidget.Widget{header, woxwidget.Expanded{Child: body}},
	}}
}

// ChatToolDetailProps contains one labeled tool-call detail value.
type ChatToolDetailProps struct {
	Label  string
	Value  string
	Layout woxwidget.TextBlockLayout
}

// ChatToolCallProps contains one collapsible tool row inside an activity group.
type ChatToolCallProps struct {
	Key           string
	Name          string
	NameWidth     float32
	Duration      string
	DurationWidth float32
	Status        string
	StatusColor   woxui.Color
	Expanded      bool
	Details       []ChatToolDetailProps
	DetailsHeight float32
	OnToggle      func()
}

// ChatAttachmentProps carries reference content and an optional measured message layout.
type ChatAttachmentProps struct {
	Kind   string
	Image  *woxui.Image
	ID     string
	Label  string
	Text   string
	Layout woxwidget.TextBlockLayout
}

// ChatMessageProps contains one prepared conversation and its controller callbacks.
type ChatMessageProps struct {
	Key              string
	AvailableWidth   float32
	Kind             string
	Role             string
	Timestamp        string
	TimestampWidth   float32
	RoundLabel       string
	RoundExpanded    bool
	ToolSummary      string
	ToolSummaryWidth float32
	ToolLeading      string
	ToolStatus       string
	ToolStatusColor  woxui.Color
	Tools            []ChatToolCallProps
	Text             string
	ContentWidth     float32
	TextLayout       woxwidget.TextBlockLayout
	Markdown         *woxcomponent.MarkdownProps
	Reasoning        string
	ReasoningLayout  woxwidget.TextBlockLayout
	ToolText         string
	ToolLayout       woxwidget.TextBlockLayout
	Skills           string
	Attachments      []ChatAttachmentProps
	Images           []*woxui.Image
	Theme            woxcomponent.Theme
	ShowMeta         bool
	Copied           bool
	CopyLabel        string
	CopiedLabel      string
	EditLabel        string
	RetryLabel       string
	OnCopy           func() bool
	OnEdit           func()
	OnRetry          func()
	OnTooltip        func(bool, string, woxui.Rect)
	OnToggleRound    func()
}

// ChatMessagesProps contains typed conversations and scroll geometry.
type ChatMessagesProps struct {
	Width           float32
	Height          float32
	Key             string
	Messages        []ChatMessageProps
	EmptyMessage    string
	EmptyTextWidth  float32
	EmptyTextHeight float32
	ContentHeight   float32
	ExtentRevision  uint64
	Scroll          float32
	Theme           woxcomponent.Theme
	OnScroll        func(float32, float32)
}

// ChatMessagesContentHeight returns the shared scroll extent for prepared messages.
func ChatMessagesContentHeight(messages []ChatMessageProps, viewportHeight float32) float32 {
	height, _ := ChatMessagesScrollMetrics(messages, viewportHeight)
	return height
}

// ChatMessagesScrollMetrics walks messages once for scroll extent and a trusted prefix revision.
func ChatMessagesScrollMetrics(messages []ChatMessageProps, viewportHeight float32) (float32, uint64) {
	height := float32(0)
	revision := uint64(len(messages)) + 1
	for _, message := range messages {
		item := chatMessageHeight(message)
		height += item
		revision = revision*16777619 ^ uint64(math.Float32bits(item))
	}
	return max(viewportHeight, height), revision
}

// ChatMessages builds the scrollable conversation viewport.
func ChatMessages(props ChatMessagesProps) woxwidget.Widget {
	innerWidth := max(float32(0), props.Width)
	innerHeight := max(float32(0), props.Height-14)
	if len(props.Messages) == 0 {
		color := props.Theme.ResultTitle
		color.A = uint8(float32(color.A) * 0.59)
		textWidth := min(max(float32(0), innerWidth-48), props.EmptyTextWidth)
		return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Top: 6, Bottom: 8}, Child: woxwidget.Align{
			Width: innerWidth, Height: innerHeight, Horizontal: 0.5, Vertical: 0.5,
			Child: woxwidget.Container{Width: textWidth, Height: props.EmptyTextHeight, Child: woxwidget.Text{Value: props.EmptyMessage, Style: woxui.TextStyle{Size: 28, Weight: woxui.FontWeightSemibold}, Color: color}},
		}}
	}
	messages := props.Messages
	contentHeight := max(innerHeight, props.ContentHeight)
	maxOffset := max(float32(0), contentHeight-innerHeight)
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Top: 6, Bottom: 8}, Child: woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: woxwidget.Key("chat-message-scroll-" + props.Key), Width: innerWidth, Height: innerHeight, ContentHeight: contentHeight,
		Offset: min(max(float32(0), props.Scroll), maxOffset), Content: woxwidget.LazyList{
			Key: woxwidget.Key("chat-messages-" + props.Key), Width: innerWidth, Viewport: innerHeight, ItemCount: len(messages),
			ExtentRevision: props.ExtentRevision,
			ItemExtentAt:   func(index int) float32 { return chatMessageHeight(messages[index]) },
			ItemKey:        func(index int) woxwidget.Key { return woxwidget.Key(messages[index].Key) },
			ItemBuilder:    func(index int) woxwidget.Widget { return ChatMessage(messages[index], innerWidth) },
		},
		ThumbColor: props.Theme.ResultTitle, OnScroll: func(delta float32) {
			if props.OnScroll != nil {
				props.OnScroll(delta, maxOffset)
			}
		},
		AutomationID: "chat.messages", Label: props.EmptyMessage,
	})}
}

// chatMessageState keeps hover-only metadata out of the launcher controller.
type chatMessageState struct {
	hovered       bool
	actionHovered bool
	copied        bool
	copyAnchor    woxui.Rect
	copyReset     *time.Timer
}

// ChatMessage maps a prepared conversation to the Flutter-aligned reading surface.
func ChatMessage(props ChatMessageProps, width float32) woxwidget.Widget {
	props.AvailableWidth = width
	return woxwidget.Stateful{
		Key: woxwidget.Key("chat-message-" + props.Key), Type: (*chatMessageState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &chatMessageState{} },
	}
}

func (s *chatMessageState) InitState(_ woxwidget.StateContext, _ any) {}

func (s *chatMessageState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

func (s *chatMessageState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(ChatMessageProps)
	props.Copied = s.copied
	if original := props.OnCopy; original != nil {
		props.OnCopy = func() bool {
			if !original() {
				return false
			}
			s.confirmCopied(context, props.OnTooltip, props.CopiedLabel)
			return true
		}
	}
	width := props.AvailableWidth
	return chatMessageContent(props, width, s.hovered || s.actionHovered, func(inside bool) {
		if inside != s.hovered {
			context.SetState(func() { s.hovered = inside })
		}
	}, func(inside bool, bounds woxui.Rect) {
		if inside {
			s.copyAnchor = bounds
		}
		if inside != s.actionHovered {
			context.SetState(func() { s.actionHovered = inside })
		}
	})
}

func (s *chatMessageState) confirmCopied(context woxwidget.StateContext, tooltip func(bool, string, woxui.Rect), label string) {
	if s.copyReset != nil {
		s.copyReset.Stop()
	}
	context.SetState(func() { s.copied = true })
	if tooltip != nil && label != "" && (s.copyAnchor.Width > 0 || s.copyAnchor.Height > 0) {
		tooltip(true, label, s.copyAnchor)
	}
	s.copyReset = time.AfterFunc(chatCopyFeedbackDuration, func() {
		_ = woxui.Call(func() {
			if !context.Mounted() {
				return
			}
			context.SetState(func() { s.copied = false })
			if tooltip != nil {
				tooltip(false, "", woxui.Rect{})
			}
		})
	})
}

func (s *chatMessageState) Dispose() {
	if s.copyReset != nil {
		s.copyReset.Stop()
		s.copyReset = nil
	}
}

// chatMessageContent builds the message body while its retained owner supplies hover state.
func chatMessageContent(props ChatMessageProps, width float32, hovered bool, onHover func(bool), onActionHover func(bool, woxui.Rect)) woxwidget.Widget {
	if props.Kind == "round" {
		disclosure := woxcomponent.KeyboardArrowRightGlyph(16, props.Theme.ResultSubtitle)
		if props.RoundExpanded {
			disclosure = woxcomponent.KeyboardArrowDownGlyph(16, props.Theme.ResultSubtitle)
		}
		return woxwidget.Gesture{ID: "chat-round-" + props.Key, OnTap: props.OnToggleRound, Child: woxwidget.Container{
			Width: width, Height: 30, Padding: woxwidget.Insets{Left: 4, Top: 7, Right: 2, Bottom: 7}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				disclosure,
				woxwidget.Text{Value: props.RoundLabel, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle},
			}},
		}}
	}
	if props.Kind == "tool-activity" {
		return chatToolActivity(props, width)
	}
	cardWidth := max(float32(0), width-4)
	left := float32(2)
	background := woxui.Color{}
	textColor := props.Theme.PreviewText
	role := strings.ToUpper(props.Role)
	if props.Role == "user" {
		cardWidth = min(cardWidth*0.82, max(float32(24), props.ContentWidth+24))
		left = max(float32(2), width-cardWidth-2)
		background = props.Theme.SelectedBackground
		textColor = props.Theme.SelectedTitle
		role = "YOU"
	} else if props.Role == "assistant" {
		role = "WOX"
	} else if props.Role == "tool" {
		role = "TOOL"
		background = props.Theme.ActionBackground
	} else if props.Role == "system" {
		role = "SYSTEM"
		background = props.Theme.ActionBackground
	}
	if role == "" {
		role = "MESSAGE"
	}

	innerWidth := max(float32(24), cardWidth-24)
	actions, _ := chatMessageActions(props, hovered, onActionHover)
	hasActions := len(actions) > 0
	showRoleHeader := props.Role == "tool" || props.Role == "system" || props.ToolText != ""
	children := make([]woxwidget.Widget, 0, 6)
	var footer woxwidget.Widget
	meta := role
	if props.Timestamp != "" {
		meta += "  " + props.Timestamp
	}
	if showRoleHeader {
		headerChildren := []woxwidget.Widget{woxwidget.Expanded{Child: woxwidget.Container{Height: 18, Child: woxwidget.Text{Value: meta, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle}}}}
		if hasActions {
			headerChildren = append(headerChildren, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6, Children: actions})
		}
		children = append(children, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: headerChildren})
	}
	for _, attachment := range props.Attachments {
		children = append(children, chatAttachmentCard(attachment, innerWidth, props.Theme, "", nil, true))
	}
	if props.ToolText != "" {
		children = append(children, woxwidget.TextBlock{Value: props.ToolText, Width: innerWidth, Height: props.ToolLayout.Size.Height, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: textColor, Layout: &props.ToolLayout})
	} else {
		if props.Reasoning != "" {
			reasoningColor := textColor
			reasoningColor.A = 120
			reasoning := woxwidget.TextBlock{Value: props.Reasoning, Width: innerWidth, Height: props.ReasoningLayout.Size.Height, Style: woxui.TextStyle{Size: 11}, LineHeight: 16, Color: reasoningColor, Layout: &props.ReasoningLayout}
			if props.Text != "" {
				children = append(children, woxwidget.Container{Width: innerWidth, Height: props.ReasoningLayout.Size.Height + 3, Padding: woxwidget.Insets{Bottom: 3}, Child: reasoning})
			} else {
				children = append(children, reasoning)
			}
		}
		if props.Text != "" {
			if props.Markdown != nil {
				children = append(children, woxcomponent.WoxMarkdown(*props.Markdown))
			} else {
				children = append(children, woxwidget.TextBlock{Value: props.Text, Width: innerWidth, Height: props.TextLayout.Size.Height, Style: woxui.TextStyle{Size: 13}, LineHeight: 19, Color: textColor, Layout: &props.TextLayout})
			}
		}
	}
	if props.Skills != "" {
		children = append(children, woxwidget.Container{Width: innerWidth, Height: 18, Child: woxwidget.Text{Value: props.Skills, Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ResultSubtitle}})
	}
	if len(props.Images) > 0 {
		imageChildren := make([]woxwidget.Widget, 0, len(props.Images))
		for _, image := range props.Images {
			var child woxwidget.Widget = woxwidget.Container{Width: 82, Height: 82, Radius: 8, Color: props.Theme.ActionBackground, Padding: woxwidget.Insets{Left: 13, Top: 31}, Child: woxwidget.Text{Value: "Image", Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ResultSubtitle}}
			if image != nil {
				child = woxwidget.Image{Source: image, Width: 82, Height: 82}
			}
			imageChildren = append(imageChildren, child)
		}
		children = append(children, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: imageChildren})
	}
	if !showRoleHeader && props.ShowMeta {
		metaColor := props.Theme.ResultSubtitle
		if !hovered {
			metaColor.A = 0
		}
		footerChildren := []woxwidget.Widget{
			woxwidget.Text{Value: props.Timestamp, Style: woxui.TextStyle{Size: 11}, Color: metaColor},
			woxwidget.Container{Width: 10},
			woxwidget.Painter{Width: 4, Height: 4, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
				displayList.FillRoundedRect(bounds, 2, metaColor)
			}},
			woxwidget.Container{Width: 10},
		}
		if hasActions {
			footerChildren = append(footerChildren, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: actions})
		} else if reserved := chatMessageActionWidth(props); reserved > 0 {
			footerChildren = append(footerChildren, woxwidget.Container{Width: reserved})
		}
		footer = woxwidget.Container{Height: 18, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: footerChildren}}
	}

	cardHeight := chatMessageHeight(props)
	bodyHeight := cardHeight
	if footer != nil {
		bodyHeight -= 21
	}
	padding := woxwidget.Insets{Top: 3, Right: 4, Bottom: 3}
	radius := float32(0)
	if props.Role == "user" {
		padding = woxwidget.Insets{Left: 12, Top: 8, Right: 12, Bottom: 8}
		radius = 8
	}
	body := woxwidget.Container{Width: cardWidth, Height: bodyHeight, Radius: radius, Color: background, Padding: padding, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 3, Children: children}}
	cardChildren := []woxwidget.Widget{body}
	if footer != nil {
		cardChildren = append(cardChildren, footer)
	}
	crossAxisAlignment := woxwidget.CrossAxisStart
	if props.Role == "user" {
		crossAxisAlignment = woxwidget.CrossAxisEnd
	}
	card := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 3, CrossAxisAlignment: crossAxisAlignment, Children: cardChildren}
	return woxwidget.Gesture{ID: "chat-message-hover-" + props.Key, OnHover: onHover, Child: woxwidget.Stack{Width: width, Height: cardHeight, Children: []woxwidget.StackChild{{Left: left, Child: card}}}}
}

// chatToolActivity builds Flutter's grouped two-level tool disclosure.
func chatToolActivity(props ChatMessageProps, width float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-4)
	statusWidth := float32(0)
	if props.ToolStatus == "failed" {
		statusWidth = 22
	}
	titleWidth := min(max(float32(0), innerWidth-46-statusWidth), props.ToolSummaryWidth)
	leadingColor := props.Theme.ResultSubtitle
	if props.ToolStatus == "failed" {
		leadingColor = props.ToolStatusColor
	}
	leading := woxcomponent.TerminalGlyph(16, leadingColor)
	if props.ToolLeading == "search" {
		leading = woxcomponent.SearchGlyph(16, leadingColor)
	} else if props.ToolLeading == "document" {
		leading = woxcomponent.ArticleGlyph(16, leadingColor)
	} else if props.ToolLeading == "extension" {
		leading = woxcomponent.ExtensionGlyph(16, leadingColor)
	}
	disclosure := woxcomponent.KeyboardArrowRightGlyph(16, props.Theme.ResultSubtitle)
	if props.RoundExpanded {
		disclosure = woxcomponent.KeyboardArrowDownGlyph(16, props.Theme.ResultSubtitle)
	}
	headerChildren := []woxwidget.Widget{
		leading,
		woxwidget.Container{Width: 8},
		woxwidget.Container{Width: titleWidth, Height: 16, Child: woxwidget.Text{Value: props.ToolSummary, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle}},
	}
	if props.ToolStatus == "failed" {
		headerChildren = append(headerChildren, woxwidget.Container{Width: 8}, chatToolStatusGlyph(props.ToolStatus, props.ToolStatusColor))
	}
	headerChildren = append(headerChildren, woxwidget.Container{Width: 6}, disclosure)
	header := woxwidget.Gesture{ID: "chat-tool-activity-" + props.Key, OnTap: props.OnToggleRound, Child: woxwidget.Container{Width: innerWidth, Height: 28, Padding: woxwidget.Insets{Left: 2, Top: 6, Right: 2, Bottom: 6}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 0, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: headerChildren,
	}}}
	children := []woxwidget.Widget{header}
	if props.RoundExpanded {
		toolWidth := max(float32(0), innerWidth-24)
		toolChildren := make([]woxwidget.Widget, 0, len(props.Tools))
		for _, tool := range props.Tools {
			toolChildren = append(toolChildren, woxwidget.Container{Width: toolWidth, Height: chatToolCallHeight(tool) + 6, Padding: woxwidget.Insets{Bottom: 6}, Child: chatToolCall(tool, toolWidth, props.Theme)})
		}
		children = append(children, woxwidget.Container{Width: innerWidth, Height: chatToolActivityDetailsHeight(props.Tools) + 6, Padding: woxwidget.Insets{Left: 24, Top: 2, Bottom: 4}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: toolChildren}})
	}
	return woxwidget.Container{Width: width, Height: chatMessageHeight(props), Padding: woxwidget.Insets{Left: 2, Top: 3, Right: 2, Bottom: 3}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
}

// chatToolCall builds one tool badge and its optional details panel.
func chatToolCall(tool ChatToolCallProps, width float32, theme woxcomponent.Theme) woxwidget.Widget {
	innerWidth := max(float32(0), width-4)
	nameWidth := min(max(float32(0), innerWidth-tool.DurationWidth-76), tool.NameWidth)
	disclosure := woxcomponent.KeyboardArrowRightGlyph(16, theme.ResultSubtitle)
	if tool.Expanded {
		disclosure = woxcomponent.KeyboardArrowDownGlyph(16, theme.ResultSubtitle)
	}
	header := woxwidget.Gesture{ID: "chat-tool-call-" + tool.Key, OnTap: tool.OnToggle, Child: woxwidget.Container{Width: width, Height: 28, Padding: woxwidget.Insets{Left: 2, Top: 6, Right: 2, Bottom: 6}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxcomponent.ToolGlyph(16, theme.ResultSubtitle),
			woxwidget.Container{Width: 8},
			woxwidget.Container{Width: nameWidth, Height: 16, Child: woxwidget.Text{Value: tool.Name, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: theme.ResultSubtitle}},
			woxwidget.Container{Width: 8},
			woxwidget.Container{Width: tool.DurationWidth, Height: 16, Child: woxwidget.Text{Value: tool.Duration, Style: woxui.TextStyle{Size: 11}, Color: theme.ResultSubtitle}},
			woxwidget.Container{Width: 8},
			chatToolStatusGlyph(tool.Status, tool.StatusColor),
			woxwidget.Container{Width: 6},
			disclosure,
		},
	}}}
	if !tool.Expanded {
		return header
	}
	detailWidth := max(float32(0), width-24)
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		header,
		woxwidget.Container{Width: width, Height: tool.DetailsHeight + 14, Padding: woxwidget.Insets{Left: 24, Top: 10, Bottom: 4}, Child: chatToolDetails(tool, detailWidth, theme)},
	}}
}

// chatToolDetails builds Flutter's bordered label/value detail card.
func chatToolDetails(tool ChatToolCallProps, width float32, theme woxcomponent.Theme) woxwidget.Widget {
	panelColor := theme.ActionBackground
	panelColor.A = 15
	borderColor := theme.ActionBackground
	borderColor.A = 40
	children := make([]woxwidget.Widget, 0, len(tool.Details))
	for _, detail := range tool.Details {
		valueHeight := detail.Layout.Size.Height + 12
		children = append(children, woxwidget.Container{Height: detail.Layout.Size.Height + 40, Padding: woxwidget.Insets{Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, CrossAxisAlignment: woxwidget.CrossAxisStretch, Children: []woxwidget.Widget{
			woxwidget.Container{Height: 16, Child: woxwidget.Text{Value: detail.Label, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: theme.ResultSubtitle}},
			woxwidget.Container{Height: valueHeight, Color: woxui.Color{A: 20}, BorderColor: woxui.Color{A: 10}, BorderWidth: 1, Padding: woxwidget.Insets{Left: 6, Top: 6, Right: 6, Bottom: 6}, Child: woxwidget.TextBlock{Value: detail.Value, Height: detail.Layout.Size.Height, Style: woxui.TextStyle{Size: 11}, LineHeight: 16, Color: theme.PreviewText, Layout: &detail.Layout}},
		}}})
	}
	return woxwidget.Container{Width: width, Height: tool.DetailsHeight, Radius: 8, Color: panelColor, BorderColor: borderColor, BorderWidth: 1, Padding: woxwidget.Insets{Left: 8, Top: 8, Right: 8, Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, CrossAxisAlignment: woxwidget.CrossAxisStretch, Children: children}}
}

func chatToolStatusGlyph(status string, color woxui.Color) woxwidget.Widget {
	switch status {
	case "succeeded":
		return woxcomponent.CheckCircleGlyph(14, color)
	case "failed":
		return woxcomponent.ErrorGlyph(14, color)
	case "streaming":
		return woxcomponent.PlayArrowGlyph(14, color)
	case "running":
		return woxcomponent.RefreshGlyph(14, color)
	default:
		return woxcomponent.HourglassGlyph(14, color)
	}
}

func chatToolCallHeight(tool ChatToolCallProps) float32 {
	if !tool.Expanded {
		return 28
	}
	return 42 + tool.DetailsHeight
}

func chatToolActivityDetailsHeight(tools []ChatToolCallProps) float32 {
	height := float32(0)
	for _, tool := range tools {
		height += chatToolCallHeight(tool) + 6
	}
	return height
}

// chatMessageActions builds the available controller-owned message actions.
func chatMessageActions(props ChatMessageProps, visible bool, onHover func(bool, woxui.Rect)) ([]woxwidget.Widget, float32) {
	if !visible {
		return nil, 0
	}
	hoverBackground := props.Theme.ResultSubtitle
	hoverBackground.A = uint8(float32(hoverBackground.A) * 0.1)
	actions := make([]woxwidget.Widget, 0, 2)
	width := float32(0)
	appendAction := func(name, label string, actionWidth float32, icon woxwidget.Widget, action func()) {
		if action == nil {
			return
		}
		if len(actions) > 0 {
			width += 8
		}
		width += actionWidth
		actions = append(actions, woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "chat-" + name + "-" + props.Key, Label: label, Icon: icon,
			Width: actionWidth, Height: 18, Radius: 5, HoverBackground: hoverBackground, FocusRingColor: props.Theme.Cursor, OnTap: action,
			OnHoverAt: func(inside bool, bounds woxui.Rect) {
				if onHover != nil {
					onHover(inside, bounds)
				}
				if name == "copy" && props.OnTooltip != nil {
					props.OnTooltip(inside, label, bounds)
				}
			},
		}))
	}
	copyLabel := props.CopyLabel
	copyIcon := woxcomponent.CopyGlyph(14, props.Theme.ResultSubtitle)
	if props.Copied {
		copyIcon = woxcomponent.CheckGlyph(14, props.Theme.ResultSubtitle)
		if props.CopiedLabel != "" {
			copyLabel = props.CopiedLabel
		}
	}
	var onCopy func()
	if props.OnCopy != nil {
		onCopy = func() { _ = props.OnCopy() }
	}
	appendAction("copy", copyLabel, 18, copyIcon, onCopy)
	appendAction("edit", props.EditLabel, 18, woxcomponent.EditGlyph(14, props.Theme.ResultSubtitle), props.OnEdit)
	appendAction("retry", props.RetryLabel, 18, woxcomponent.RefreshGlyph(14, props.Theme.ResultSubtitle), props.OnRetry)
	return actions, width
}

// chatMessageActionWidth reserves the Flutter toolbar width while hover metadata is hidden.
func chatMessageActionWidth(props ChatMessageProps) float32 {
	count := 0
	if props.OnCopy != nil {
		count++
	}
	for _, action := range []func(){props.OnEdit, props.OnRetry} {
		if action != nil {
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return float32(count*18 + (count-1)*8)
}

// chatMessageHeight derives the card extent from the visible message sections.
func chatMessageHeight(props ChatMessageProps) float32 {
	if props.Kind == "round" {
		return 30
	}
	if props.Kind == "tool-activity" {
		height := float32(34)
		if props.RoundExpanded {
			height += chatToolActivityDetailsHeight(props.Tools) + 6
		}
		return height
	}
	height := float32(0)
	parts := 0
	add := func(value float32) {
		if value <= 0 {
			return
		}
		height += value
		parts++
	}
	showRoleHeader := props.Role == "tool" || props.Role == "system" || props.ToolText != ""
	if showRoleHeader {
		add(18)
	}
	for _, attachment := range props.Attachments {
		add(chatAttachmentHeight(attachment))
	}
	if props.ToolText != "" {
		add(props.ToolLayout.Size.Height)
	} else {
		if props.Reasoning != "" {
			add(props.ReasoningLayout.Size.Height)
			if props.Text != "" {
				height += 3
			}
		}
		if props.Text != "" {
			add(props.TextLayout.Size.Height)
		}
	}
	if props.Skills != "" {
		add(18)
	}
	if len(props.Images) > 0 {
		add(82)
	}
	if !showRoleHeader && props.ShowMeta {
		add(18)
	}
	verticalPadding := float32(6)
	if props.Role == "user" {
		verticalPadding = 16
	}
	return height + float32(max(0, parts-1))*3 + verticalPadding
}

// ChatInputProps contains the committed input value and toolbar state.
type ChatInputProps struct {
	Width               float32
	Height              float32
	Key                 string
	Editing             woxui.TextEditingState
	Focused             bool
	Hint                string
	Window              *woxui.Window
	Model               string
	ModelWidth          float32
	Status              string
	StatusColor         woxui.Color
	ActionLabel         string
	Sending             bool
	Attachments         []ChatAttachmentProps
	QuoteDismissLabel   string
	RichRuns            []woxcomponent.TextFieldRichRun
	AtomicTokens        []woxcomponent.TextFieldTokenRange
	Theme               woxcomponent.Theme
	OnFocus             func()
	OnChanged           func(string)
	OnKey               func(woxui.KeyEvent) bool
	OnModels            func()
	OnSend              func()
	OnDismissAttachment func(string)
}

type chatModelSelectorState struct {
	hovered bool
}

// ChatModelSelector builds Flutter's compact model chip with retained hover feedback.
func ChatModelSelector(props ChatInputProps) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: woxwidget.Key("chat-models-" + props.Key), Type: (*chatModelSelectorState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &chatModelSelectorState{} },
	}
}

func (s *chatModelSelectorState) InitState(_ woxwidget.StateContext, _ any) {}

func (s *chatModelSelectorState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

func (s *chatModelSelectorState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(ChatInputProps)
	id := "chat-models-" + props.Key
	background := woxui.Color{}
	if s.hovered {
		background = props.Theme.SelectedBackground
		background.A = 40
	}
	iconColor := props.Theme.ResultTitle
	iconColor.A = 180
	arrowColor := props.Theme.ResultTitle
	arrowColor.A = 140
	child := woxwidget.Container{Width: props.ModelWidth, Height: 20, Radius: 4, Color: background, Padding: woxwidget.Insets{Left: 4, Right: 4}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 0, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxcomponent.ModelTrainingGlyph(16, iconColor),
			woxwidget.Container{Width: 5},
			woxwidget.Expanded{Child: woxwidget.Align{Height: 20, Vertical: 0.5, Child: woxwidget.Text{Value: props.Model, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultTitle}}},
			woxwidget.Container{Width: 4},
			woxcomponent.KeyboardArrowDownGlyph(14, arrowColor),
		},
	}}
	key := woxwidget.Key(id)
	gesture := woxwidget.Gesture{ID: id, OnTap: props.OnModels, OnHover: func(inside bool) {
		if inside != s.hovered {
			context.SetState(func() { s.hovered = inside })
		}
	}, Child: child}
	return woxwidget.Semantics{Key: key, AutomationID: id, Role: woxui.AccessibilityRoleButton, Label: props.Model,
		Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}, Child: woxwidget.Focusable{Key: key, FocusRingColor: props.Theme.Cursor, FocusRingRadius: 4, OnKey: func(event woxui.KeyEvent) bool {
			if event.Key != woxui.KeyEnter && event.Key != woxui.KeySpace {
				return false
			}
			if event.Down && props.OnModels != nil {
				props.OnModels()
			}
			return true
		}, Child: gesture}}
}

func (s *chatModelSelectorState) Dispose() {}

// ChatInput builds the multiline editor card and send toolbar.
func ChatInput(props ChatInputProps) woxwidget.Widget {
	const toolbarHeight = float32(42)
	quoteHeight := float32(min(len(props.Attachments), 3)) * chatQuoteCardHeight
	cardHeight := max(float32(78), props.Height-14)
	editorHeight := max(float32(36), cardHeight-toolbarHeight-quoteHeight-1)
	input := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: "chat-input-" + props.Key, Label: props.Hint, Hint: props.Hint, Width: props.Width, Height: editorHeight,
		Padding: woxwidget.Insets{Left: 14, Top: 8, Right: 14, Bottom: 7}, Background: props.Theme.QueryBackground,
		Style: woxui.TextStyle{Size: 13}, Value: props.Editing.Text, Focused: props.Focused, MaxLines: 5, Window: props.Window, Theme: props.Theme,
		RichRuns: props.RichRuns, AtomicTokens: props.AtomicTokens,
		OnChanged: props.OnChanged, OnKey: props.OnKey, OnFocusChange: func(focused bool) {
			if focused && props.OnFocus != nil {
				props.OnFocus()
			}
		},
	})
	divider := props.Theme.ResultSubtitle
	divider.A = uint8(float32(divider.A) * 0.14)
	modelButton := ChatModelSelector(props)
	variant := woxcomponent.ButtonPrimary
	if props.Sending {
		variant = woxcomponent.ButtonSurface
	}
	statusLeft := props.ModelWidth + 18
	statusWidth := max(float32(0), props.Width-statusLeft-100)
	toolbarChildren := []woxwidget.StackChild{
		{Left: 8, Child: woxwidget.Align{Width: props.ModelWidth, Height: toolbarHeight, Vertical: 0.5, Child: modelButton}},
		{Right: 8, StretchWidth: true, Child: woxwidget.Align{Height: toolbarHeight, Horizontal: 1, Vertical: 0.5, Child: woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "chat-send-" + props.Key, Label: props.ActionLabel, Radius: 7, Variant: variant, OnTap: props.OnSend, Theme: props.Theme})}},
	}
	if props.Status != "" && statusWidth > 30 {
		toolbarChildren = append(toolbarChildren, woxwidget.StackChild{Left: statusLeft, Child: woxwidget.Align{Width: statusWidth, Height: toolbarHeight, Vertical: 0.5, Child: woxwidget.Text{Value: props.Status, Style: woxui.TextStyle{Size: 9}, Color: props.StatusColor}}})
	}
	cardChildren := make([]woxwidget.Widget, 0, 4)
	attachmentCards := make([]woxwidget.Widget, 0, len(props.Attachments))
	for _, attachment := range props.Attachments {
		var dismiss func()
		if props.OnDismissAttachment != nil {
			dismiss = func() { props.OnDismissAttachment(attachment.ID) }
		}
		attachmentCards = append(attachmentCards, chatAttachmentCard(attachment, props.Width, props.Theme, props.QuoteDismissLabel, dismiss, false))
	}
	if len(attachmentCards) > 3 {
		// A large selection must not push the editor and Send button out of the window.
		cardChildren = append(cardChildren, woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
			Key: woxwidget.Key("chat-attachments-" + props.Key), Width: props.Width, Height: quoteHeight,
			ContentHeight: float32(len(attachmentCards)) * chatQuoteCardHeight, ThumbColor: props.Theme.ResultSubtitle,
			Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: attachmentCards},
		}))
	} else {
		cardChildren = append(cardChildren, attachmentCards...)
	}
	cardChildren = append(cardChildren,
		input,
		woxwidget.Container{Width: props.Width, Height: 1, Color: divider},
		woxwidget.Stack{Width: props.Width, Height: toolbarHeight, Children: toolbarChildren},
	)
	card := woxwidget.Container{Width: props.Width, Height: cardHeight, Radius: 9, Color: props.Theme.QueryBackground, BorderColor: divider, BorderWidth: 1, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: cardChildren}}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Top: 6, Bottom: 8}, Child: card}
}

// chatAttachmentHeight matches the sent-card geometry used by the message scroll list.
func chatAttachmentHeight(attachment ChatAttachmentProps) float32 {
	switch attachment.Kind {
	case "image":
		return 144
	case "file":
		return chatQuoteCardHeight
	default:
		return attachment.Layout.Size.Height + 36
	}
}

// chatAttachmentCard renders compact file references and aspect-preserving image thumbnails.
func chatAttachmentCard(attachment ChatAttachmentProps, width float32, theme woxcomponent.Theme, dismissLabel string, dismiss func(), sent bool) woxwidget.Widget {
	if attachment.Kind != "image" && attachment.Kind != "file" {
		return chatQuoteCard(attachment, width, theme, dismissLabel, dismiss, sent)
	}
	height := chatQuoteCardHeight
	thumbnailSize := float32(36)
	if sent {
		height = chatAttachmentHeight(attachment)
		if attachment.Kind == "image" {
			thumbnailSize = min(float32(128), max(float32(36), width/3))
		}
	}
	textColor, secondaryColor := theme.PreviewText, theme.ResultSubtitle
	if sent {
		textColor, secondaryColor = theme.SelectedTitle, theme.SelectedSubtitle
	}
	textWidth := max(float32(0), width-thumbnailSize-24)
	if dismiss != nil {
		textWidth = max(float32(0), textWidth-36)
	}
	content := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
		woxwidget.TextBlock{Value: attachment.Label, Width: textWidth, Height: 16, MaxLines: 1, Style: woxui.TextStyle{Size: woxcomponent.SettingsHelpFontSize}, LineHeight: 16, Color: textColor},
		woxwidget.TextBlock{Value: attachment.Text, Width: textWidth, Height: 28, MaxLines: 2, Style: woxui.TextStyle{Size: woxcomponent.CompactButtonFontSize}, LineHeight: 14, Color: secondaryColor},
	}}
	children := []woxwidget.Widget{
		woxwidget.Image{Source: attachment.Image, Width: thumbnailSize, Height: thumbnailSize, Fit: woxwidget.ImageFitContain, Radius: 4},
		woxwidget.Expanded{Child: content},
	}
	if dismiss != nil {
		children = append(children, woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "chat-attachment-dismiss-" + attachment.ID, Label: dismissLabel, Icon: woxcomponent.CloseGlyph(14, secondaryColor),
			Width: 28, Height: 28, Radius: 14, HoverBackground: chatIconHoverBackground(theme), FocusRingColor: theme.Cursor, OnTap: dismiss,
		}))
	}
	return woxwidget.Semantics{AutomationID: "chat-attachment-" + attachment.ID, Role: woxui.AccessibilityRoleGroup, Label: attachment.Label + ": " + attachment.Text,
		Child: woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 8, Top: 4, Right: 8, Bottom: 4},
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children},
		},
	}
}

// chatIconHoverBackground uses the quiet ResultSubtitle wash shared by chat icon actions.
func chatIconHoverBackground(theme woxcomponent.Theme) woxui.Color {
	hover := theme.ResultSubtitle
	hover.A = uint8(float32(hover.A) * 0.1)
	return hover
}

// chatQuoteCard shares the reference treatment between the composer and sent messages.
// Sent messages preserve line breaks and show the full reference; drafts stay compact.
func chatQuoteCard(attachment ChatAttachmentProps, width float32, theme woxcomponent.Theme, dismissLabel string, dismiss func(), sent bool) woxwidget.Widget {
	height := chatQuoteCardHeight
	textHeight := float32(28)
	textWidth := max(float32(0), width-68)
	value := chatQuotePreviewText(attachment.Text)
	var layout *woxwidget.TextBlockLayout
	maxLines := 2
	lineHeight := float32(14)
	fontSize := woxcomponent.CompactButtonFontSize
	if sent {
		height = attachment.Layout.Size.Height + 36
		textHeight = attachment.Layout.Size.Height
		textWidth = max(float32(0), width-32)
		value = attachment.Text
		layout = &attachment.Layout
		maxLines = 0
		lineHeight = 18
		fontSize = woxcomponent.SettingsHelpFontSize
	}
	labelColor := theme.ResultSubtitle
	textColor := theme.PreviewText
	if sent {
		labelColor = theme.SelectedSubtitle
		textColor = theme.SelectedTitle
	}
	text := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
		woxwidget.Text{Value: attachment.Label, Style: woxui.TextStyle{Size: woxcomponent.CompactButtonFontSize, Weight: woxui.FontWeightSemibold}, Color: labelColor},
		woxwidget.TextBlock{Value: value, Width: textWidth, Height: textHeight, MaxLines: maxLines, Style: woxui.TextStyle{Size: fontSize}, LineHeight: lineHeight, Color: textColor, Layout: layout},
	}}
	rowChildren := []woxwidget.Widget{
		woxcomponent.FormatGlyph("quote", 16, woxcomponent.DocumentListMarkerColor),
		woxwidget.Expanded{Child: text},
	}
	if dismiss != nil {
		rowChildren = append(rowChildren, woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "chat-quote-dismiss-" + attachment.ID, Label: dismissLabel, Icon: woxcomponent.CloseGlyph(14, labelColor),
			Width: 28, Height: 28, Radius: 14, HoverBackground: chatIconHoverBackground(theme), FocusRingColor: theme.Cursor, OnTap: dismiss,
		}))
	}
	return woxwidget.Semantics{
		AutomationID: "chat-quote-" + attachment.ID, Role: woxui.AccessibilityRoleGroup, Label: attachment.Label + ": " + attachment.Text,
		Child: woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 8, Top: 6, Right: 0, Bottom: 6},
			LeftBorderColor: woxcomponent.DocumentListMarkerColor, LeftBorderWidth: 2,
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: rowChildren},
		},
	}
}

// chatQuotePreviewText collapses selected text into a compact composer preview.
func chatQuotePreviewText(quote string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(quote)), " ")
}

// ChatQuestionOptionProps contains one ask-user option.
type ChatQuestionOptionProps struct {
	ID       string
	Label    string
	Selected bool
	OnSelect func()
}

// ChatQuestionInputProps contains the optional committed free-form answer.
type ChatQuestionInputProps struct {
	ID        string
	Height    float32
	Editing   woxui.TextEditingState
	Focused   bool
	Window    *woxui.Window
	OnFocus   func()
	OnChanged func(string)
	OnKey     func(woxui.KeyEvent) bool
}

// ChatQuestionProps contains the typed ask-user options and actions.
type ChatQuestionProps struct {
	Width    float32
	Height   float32
	Question string
	Options  []ChatQuestionOptionProps
	Input    *ChatQuestionInputProps
	Theme    woxcomponent.Theme
	OnCancel func()
	OnSubmit func()
}

// ChatQuestion builds the inline ask-user panel.
func ChatQuestion(props ChatQuestionProps) woxwidget.Widget {
	innerWidth := max(float32(0), props.Width-24)
	children := []woxwidget.Widget{woxwidget.Container{Width: innerWidth, Height: 34, Child: woxwidget.TextBlock{Value: props.Question, Width: innerWidth, Height: 34, MaxLines: 2, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, LineHeight: 17, Color: props.Theme.PreviewText}}}
	for _, option := range props.Options {
		background := props.Theme.QueryBackground
		if option.Selected {
			background = props.Theme.SelectedBackground
		}
		children = append(children, woxwidget.Gesture{ID: option.ID, OnTap: option.OnSelect, Child: woxwidget.Container{
			Width: innerWidth, Height: 40, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 10, Right: 10}, Child: woxwidget.Align{Height: 40, Vertical: 0.5, Child: woxwidget.Text{Value: option.Label, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.PreviewText}},
		}})
	}
	if props.Input != nil {
		children = append(children, woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
			ID: props.Input.ID, Label: "Answer", Hint: "Type an answer…", Width: innerWidth, Height: props.Input.Height,
			Radius: 7, Padding: woxwidget.Insets{Left: 10, Top: 8, Right: 10, Bottom: 8}, Background: props.Theme.QueryBackground,
			Style: woxui.TextStyle{Size: 12}, Value: props.Input.Editing.Text, Focused: props.Input.Focused, MaxLines: 4,
			Window: props.Input.Window, Theme: props.Theme, OnChanged: props.Input.OnChanged, OnKey: props.Input.OnKey,
			OnFocusChange: func(focused bool) {
				if focused && props.Input.OnFocus != nil {
					props.Input.OnFocus()
				}
			},
		}))
	}
	children = append(children, woxwidget.Align{Width: innerWidth, Height: 32, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "chat-question-cancel", Label: "Cancel", Variant: woxcomponent.ButtonSurface, OnTap: props.OnCancel, Theme: props.Theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "chat-question-submit", Label: "Submit", Variant: woxcomponent.ButtonPrimary, OnTap: props.OnSubmit, Theme: props.Theme}),
	}}})
	return woxwidget.Container{Width: props.Width, Height: props.Height, Radius: 9, Color: props.Theme.ActionBackground, Padding: woxwidget.Insets{Left: 12, Top: 8, Right: 12, Bottom: 8}, Child: woxwidget.Clip{
		Width: innerWidth, Height: max(float32(0), props.Height-16), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: children},
	}}
}
