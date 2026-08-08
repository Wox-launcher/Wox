package preview

import (
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ChatPreviewProps contains the typed chat panes and optional catalog drawer.
type ChatPreviewProps struct {
	Width     float32
	Height    float32
	Key       string
	Panel     string
	Header    ChatHeaderProps
	Messages  ChatMessagesProps
	Debug     *ChatDebugProps
	Question  *ChatQuestionProps
	Input     ChatInputProps
	Catalog   *ChatCatalogProps
	OnDismiss func()
}

// ChatPreview builds the chat reading flow and floating catalog layers.
func ChatPreview(props ChatPreviewProps) woxwidget.Widget {
	const headerHeight = float32(52)
	const inputHeight = float32(98)
	innerWidth := max(float32(0), props.Width-20)
	innerHeight := max(float32(0), props.Height-14)
	children := []woxwidget.Widget{ChatHeader(props.Header), ChatMessages(props.Messages)}
	if props.Debug != nil {
		children = append(children, ChatDebug(*props.Debug))
	}
	if props.Question != nil {
		children = append(children, ChatQuestion(*props.Question))
	}
	children = append(children, ChatInput(props.Input))
	layers := []woxwidget.StackChild{{Left: 10, Top: 6, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}}
	if props.Catalog != nil {
		layers = append(layers, woxwidget.StackChild{Child: woxwidget.Gesture{ID: "chat-panel-dismiss-" + props.Key, OnTap: props.OnDismiss, Child: woxwidget.Container{Width: props.Width, Height: props.Height}}})
		left := float32(0)
		top := float32(0)
		if props.Panel != "history" {
			left = 10
			top = 6
			left += (innerWidth - props.Catalog.Width) / 2
			top += max(headerHeight, innerHeight-inputHeight-props.Catalog.Height-6)
		}
		layers = append(layers, woxwidget.StackChild{Left: left, Top: top, Child: ChatCatalog(*props.Catalog)})
	}
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: layers}
}

// ChatHeaderProps contains the current conversation title and header actions.
type ChatHeaderProps struct {
	Width          float32
	Height         float32
	Key            string
	Title          string
	HistoryOpen    bool
	ShowDebug      bool
	DebugOpen      bool
	ShowExit       bool
	ExitLabel      string
	HistoryLabel   string
	HistoryTooltip string
	Theme          woxcomponent.Theme
	OnHistory      func()
	OnDebug        func()
	OnExit         func()
	OnDrag         func()
	OnExitHover    func(bool, string, woxui.Rect)
	OnHistoryHover func(bool, string, woxui.Rect)
}

// ChatHeader builds the compact chat title bar.
func ChatHeader(props ChatHeaderProps) woxwidget.Widget {
	menuBackground := woxui.Color{}
	menuIcon := woxcomponent.MenuGlyph(22, props.Theme.ResultSubtitle)
	if props.HistoryOpen {
		menuBackground = props.Theme.ActionBackground
		menuIcon = woxcomponent.CloseGlyph(20, props.Theme.ResultSubtitle)
	}
	menuHoverBackground := props.Theme.ResultSubtitle
	menuHoverBackground.A = uint8(float32(menuHoverBackground.A) * 0.1)
	if menuBackground.A != 0 {
		menuHoverBackground = menuBackground
	}
	debugWidth := float32(0)
	if props.ShowDebug {
		debugWidth = 48
	}
	exitWidth := float32(0)
	if props.ShowExit {
		exitWidth = 40
	}
	titleWidth := max(float32(60), props.Width-48-debugWidth-exitWidth)
	title := woxwidget.Gesture{ID: "chat-title-drag-" + props.Key, OnDragStart: props.OnDrag, Child: woxwidget.Align{
		Width: titleWidth, Height: 36, Horizontal: 0, Vertical: 0.5,
		Child: woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText},
	}}
	children := []woxwidget.StackChild{
		{Left: 2, Top: 5, Child: woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "chat-history-" + props.Key, Label: props.HistoryLabel, Icon: menuIcon, Width: 36, Height: 36, Radius: 7,
			Background: menuBackground, HoverBackground: menuHoverBackground, FocusRingColor: props.Theme.Cursor, OnTap: props.OnHistory,
			OnHoverAt: func(inside bool, bounds woxui.Rect) {
				if props.OnHistoryHover != nil {
					props.OnHistoryHover(inside, props.HistoryTooltip, bounds)
				}
			},
		})},
		{Left: 44, Top: 5, Child: title},
	}
	if props.ShowDebug {
		debugBackground := woxui.Color{}
		if props.DebugOpen {
			debugBackground = props.Theme.ActionBackground
		}
		children = append(children, woxwidget.StackChild{Left: props.Width - 40 - exitWidth, Top: 9, Child: woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "chat-debug-" + props.Key, Label: "Debug trace", Icon: woxcomponent.DebugGlyph(16, props.Theme.ResultSubtitle),
			Width: 28, Height: 28, Radius: 7, Background: debugBackground, HoverBackground: menuHoverBackground, FocusRingColor: props.Theme.Cursor, OnTap: props.OnDebug,
		})})
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
		children = append(children, woxwidget.StackChild{Left: props.Width - 34, Top: 9, Child: button})
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Child: woxwidget.Stack{Width: props.Width, Height: props.Height, Children: children}}
}

// chatHeaderButton applies the compact selected state shared by chat header actions.
func chatHeaderButton(id, label string, width float32, selected bool, theme woxcomponent.Theme, action func()) woxwidget.Widget {
	variant := woxcomponent.ButtonSurface
	if selected {
		variant = woxcomponent.ButtonSelected
	}
	return woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: id, Label: label, Width: width, Height: 34, Radius: 7, FontSize: 10, Variant: variant, OnTap: action, Theme: theme})
}

// ChatCatalogItemProps contains one selectable history, model, or skill entry.
type ChatCatalogItemProps struct {
	SelectID    string
	DeleteID    string
	Kind        string
	Title       string
	Subtitle    string
	DeleteLabel string
	GroupLabel  string
	Selected    bool
	Current     bool
	OnSelect    func()
	OnDelete    func()
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
			{Top: 5, Child: woxwidget.Container{Width: max(float32(0), innerWidth-54), Height: 18, Child: woxwidget.Text{Value: props.Label, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionHeader}}},
			{Left: innerWidth - 68, Child: chatHeaderButton("chat-new-"+props.Key, props.NewLabel, 68, false, props.Theme, props.OnNew)},
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
		rows = append(rows, woxwidget.Container{Width: innerWidth, Height: viewportHeight, Padding: woxwidget.Insets{Left: 10, Top: 18}, Child: woxwidget.TextBlock{Value: props.EmptyMessage, Width: max(float32(0), innerWidth-20), Height: 48, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: props.Theme.ResultSubtitle}})
	}
	border := props.Theme.ResultSubtitle
	border.A = uint8(float32(border.A) * 0.14)
	children = append(children, woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: woxwidget.Key("chat-catalog-scroll-" + props.Key), Width: innerWidth, Height: viewportHeight, ContentHeight: props.ContentHeight,
		Offset: props.Scroll, Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}, ThumbColor: props.Theme.ResultSubtitle, OnScroll: props.OnScroll,
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
			rows = append(rows, woxwidget.Container{Width: innerWidth, Height: 32, Padding: woxwidget.Insets{Left: 4, Top: 10, Bottom: 6}, Child: woxwidget.Text{Value: groupLabel, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText}})
		}
		rows = append(rows, ChatCatalogItem(item, innerWidth, 46, props.Theme))
	}
	if len(props.Items) == 0 {
		rows = append(rows, woxwidget.Container{Width: innerWidth, Height: 40, Padding: woxwidget.Insets{Left: 12, Top: 10}, Child: woxwidget.Text{Value: props.EmptyMessage, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle}})
	}
	scroll := woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: woxwidget.Key("chat-catalog-scroll-" + props.Key), Width: innerWidth, Height: viewportHeight, ContentHeight: props.ContentHeight,
		Offset: props.Scroll, Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}, ThumbColor: props.Theme.ResultSubtitle, OnScroll: props.OnScroll,
	})
	divider := props.Theme.ResultSubtitle
	divider.A = 20
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Container{Width: props.Width, Height: props.Height, Color: props.Theme.ActionBackground, Padding: woxwidget.Insets{Left: 10, Top: 12, Right: 10, Bottom: 12}, Child: scroll}},
		{Left: props.Width - 1, Child: woxwidget.Container{Width: 1, Height: props.Height, Color: divider}},
	}}
}

type chatCatalogItemState struct {
	hovered bool
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
	return chatCatalogItem(props.item, props.width, props.height, props.theme, s.hovered, func(inside bool) {
		if inside != s.hovered {
			context.SetState(func() { s.hovered = inside })
		}
	})
}

func (s *chatCatalogItemState) Dispose() {}

// chatCatalogItem renders the shared two-line catalog row and optional delete target.
func chatCatalogItem(item ChatCatalogItemProps, width, height float32, theme woxcomponent.Theme, hovered bool, onHover func(bool)) woxwidget.Widget {
	if item.Kind == "history" || item.Kind == "history-new" {
		return chatHistoryItem(item, width, height, theme, hovered, onHover)
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
	textInset := float32(20)
	if item.OnDelete != nil {
		mainWidth = max(float32(80), width-44)
		rightPadding = 8
		textInset = 18
	}
	if item.OnDelete == nil {
		checkWidth := float32(0)
		if item.Current {
			checkWidth = 28
		}
		titleWidth := min(float32(220), max(float32(100), width*0.42))
		iconColor := theme.PreviewText
		if item.Selected {
			iconColor = theme.SelectedTitle
		}
		icon := woxcomponent.ModelTrainingGlyph(18, iconColor)
		if item.Kind == "skills" {
			icon = woxcomponent.ExtensionGlyph(18, iconColor)
		}
		return woxwidget.Gesture{ID: item.SelectID, OnTap: item.OnSelect, OnHover: onHover, Child: woxwidget.Container{
			Width: width, Height: height, Color: background, Child: woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
				{Left: 14, Top: 10, Child: icon},
				{Left: 42, Top: 11, Child: woxwidget.Container{Width: titleWidth, Height: 18, Child: woxwidget.Text{Value: item.Title, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: iconColor}}},
				{Left: 50 + titleWidth, Top: 11, Child: woxwidget.Container{Width: max(float32(0), width-titleWidth-58-checkWidth), Height: 18, Child: woxwidget.Text{Value: item.Subtitle, Style: woxui.TextStyle{Size: 11}, Color: theme.ResultSubtitle}}},
				{Left: width - checkWidth, Top: 10, Child: woxwidget.Container{Width: checkWidth, Height: 18, Child: woxcomponent.CheckGlyph(18, iconColor)}},
			}},
		}}
	}
	main := woxwidget.Gesture{ID: item.SelectID, OnTap: item.OnSelect, Child: woxwidget.Container{
		Width: mainWidth, Height: height - 4, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 5, Right: rightPadding}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
			woxwidget.Container{Width: max(float32(0), mainWidth-textInset), Height: 16, Child: woxwidget.Text{Value: item.Title, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: theme.PreviewText}},
			woxwidget.Container{Width: max(float32(0), mainWidth-textInset), Height: 14, Child: woxwidget.Text{Value: item.Subtitle, Style: woxui.TextStyle{Size: 9}, Color: theme.ResultSubtitle}},
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
	background := woxui.Color{}
	if hovered || item.Selected {
		background = theme.SelectedBackground
	}
	if item.Kind == "history-new" {
		iconColor := theme.ActionHeader
		iconColor.A = 200
		return woxwidget.Gesture{ID: item.SelectID, OnTap: item.OnSelect, OnHover: onHover, Child: woxwidget.Container{Width: width, Height: height, Radius: 6, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 10, Right: 12, Bottom: 10}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxcomponent.AddGlyph(18, iconColor),
			woxwidget.Text{Value: item.Title, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: theme.PreviewText},
		}}}}
	}
	titleColor := theme.PreviewText
	if item.Selected {
		titleColor = theme.SelectedTitle
	}
	rowHeight := height - 4
	deleteHover := theme.ResultSubtitle
	deleteHover.A = uint8(float32(deleteHover.A) * 0.1)
	row := woxwidget.Gesture{ID: item.SelectID, OnTap: item.OnSelect, OnHover: onHover, Child: woxwidget.Container{Width: width, Height: rowHeight, Radius: 6, Color: background, Child: woxwidget.Stack{Width: width, Height: rowHeight, Children: []woxwidget.StackChild{
		{Left: 12, Top: 13, Child: woxwidget.Container{Width: max(float32(0), width-50), Height: 18, Child: woxwidget.Text{Value: item.Title, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: titleColor}}},
	}}}}
	deleteButton := woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
		ID: item.DeleteID, Label: item.DeleteLabel, Icon: woxcomponent.DeleteGlyph(15, theme.ResultSubtitle), Width: 26, Height: 26, Radius: 13,
		HoverBackground: deleteHover, FocusRingColor: theme.Cursor, OnTap: item.OnDelete,
	})
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Bottom: 4}, Child: woxwidget.Stack{Width: width, Height: rowHeight, Children: []woxwidget.StackChild{
		{Child: row},
		{Left: width - 34, Top: 8, Child: deleteButton},
	}}}
}

// ChatDebugProps contains the laid-out trace and copy action.
type ChatDebugProps struct {
	Width         float32
	Height        float32
	Key           string
	Summary       string
	Value         string
	Layout        woxwidget.TextBlockLayout
	Scroll        float32
	ContentHeight float32
	Theme         woxcomponent.Theme
	OnScroll      func(float32)
	OnCopy        func()
}

// ChatDebug builds the portable JSON trace panel.
func ChatDebug(props ChatDebugProps) woxwidget.Widget {
	innerWidth := max(float32(0), props.Width-20)
	viewportHeight := max(float32(40), props.Height-42)
	header := woxwidget.Stack{Width: innerWidth, Height: 24, Children: []woxwidget.StackChild{
		{Child: woxwidget.Container{Width: max(float32(0), innerWidth-54), Height: 24, Child: woxwidget.Text{Value: props.Summary, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionHeader}}},
		{Left: innerWidth - 48, Child: chatHeaderButton("chat-debug-copy-"+props.Key, "Copy", 48, false, props.Theme, props.OnCopy)},
	}}
	body := woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: woxwidget.Key("chat-debug-scroll-" + props.Key), Width: innerWidth, Height: viewportHeight, ContentHeight: props.ContentHeight,
		Offset: props.Scroll, ThumbColor: props.Theme.ResultSubtitle, OnScroll: props.OnScroll,
		Content: woxwidget.Container{
			Width: innerWidth, Height: props.ContentHeight, Radius: 7, Color: props.Theme.QueryBackground, Padding: woxwidget.Insets{Left: 8, Top: 8, Right: 8, Bottom: 8},
			Child: woxwidget.TextBlock{Value: props.Value, Width: max(float32(20), innerWidth-16), Height: props.Layout.Size.Height, Style: woxui.TextStyle{Size: 10}, LineHeight: 16, Color: props.Theme.PreviewText, Layout: &props.Layout},
		},
	})
	return woxwidget.Container{Width: props.Width, Height: props.Height, Radius: 9, Color: props.Theme.ActionBackground, Padding: woxwidget.Insets{Left: 10, Top: 7, Right: 10, Bottom: 7}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{header, body}}}
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
	Images           []*woxui.Image
	Theme            woxcomponent.Theme
	ShowMeta         bool
	CopyLabel        string
	EditLabel        string
	RetryLabel       string
	OnCopy           func()
	OnEdit           func()
	OnRetry          func()
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
	Scroll          float32
	Theme           woxcomponent.Theme
	OnScroll        func(float32, float32)
}

// ChatMessagesContentHeight returns the shared scroll extent for prepared messages.
func ChatMessagesContentHeight(messages []ChatMessageProps, viewportHeight float32) float32 {
	height := float32(0)
	for _, message := range messages {
		height += chatMessageHeight(message)
	}
	return max(viewportHeight, height)
}

// ChatMessages builds the scrollable conversation viewport.
func ChatMessages(props ChatMessagesProps) woxwidget.Widget {
	innerWidth := max(float32(0), props.Width)
	innerHeight := max(float32(0), props.Height-14)
	if len(props.Messages) == 0 {
		color := props.Theme.ResultTitle
		color.A = uint8(float32(color.A) * 0.59)
		textWidth := min(max(float32(0), innerWidth-48), props.EmptyTextWidth)
		left := max(float32(24), (innerWidth-textWidth)/2)
		top := max(float32(0), (innerHeight-props.EmptyTextHeight)/2)
		return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Top: 6, Bottom: 8}, Child: woxwidget.Stack{Width: innerWidth, Height: innerHeight, Children: []woxwidget.StackChild{
			{Left: left, Top: top, Child: woxwidget.Container{Width: textWidth, Height: props.EmptyTextHeight, Child: woxwidget.Text{Value: props.EmptyMessage, Style: woxui.TextStyle{Size: 28, Weight: woxui.FontWeightSemibold}, Color: color}}},
		}}}
	}
	rows := make([]woxwidget.Widget, 0, len(props.Messages))
	for _, message := range props.Messages {
		rows = append(rows, ChatMessage(message, innerWidth))
	}
	contentHeight := max(innerHeight, props.ContentHeight)
	maxOffset := max(float32(0), contentHeight-innerHeight)
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Top: 6, Bottom: 8}, Child: woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: woxwidget.Key("chat-message-scroll-" + props.Key), Width: innerWidth, Height: innerHeight, ContentHeight: contentHeight,
		Offset: min(max(float32(0), props.Scroll), maxOffset), Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		ThumbColor: props.Theme.ResultSubtitle, OnScroll: func(delta float32) {
			if props.OnScroll != nil {
				props.OnScroll(delta, maxOffset)
			}
		},
		AutomationID: "chat.messages", Label: props.EmptyMessage,
	})}
}

// chatMessageState keeps hover-only metadata out of the launcher controller.
type chatMessageState struct {
	hovered bool
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
	width := props.AvailableWidth
	return chatMessageContent(props, width, s.hovered, func(inside bool) {
		if inside != s.hovered {
			context.SetState(func() { s.hovered = inside })
		}
	})
}

func (s *chatMessageState) Dispose() {}

// chatMessageContent builds the message body while its retained owner supplies hover state.
func chatMessageContent(props ChatMessageProps, width float32, hovered bool, onHover func(bool)) woxwidget.Widget {
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
	actions, actionWidth := chatMessageActions(props, hovered, onHover)
	hasActions := len(actions) > 0
	showRoleHeader := props.Role == "tool" || props.Role == "system" || props.ToolText != ""
	children := make([]woxwidget.Widget, 0, 6)
	var footer woxwidget.Widget
	meta := role
	if props.Timestamp != "" {
		meta += "  " + props.Timestamp
	}
	if showRoleHeader {
		metaWidth := innerWidth
		if hasActions {
			metaWidth = max(float32(0), innerWidth-actionWidth-8)
		}
		headerChildren := []woxwidget.StackChild{{Child: woxwidget.Container{Width: metaWidth, Height: 18, Child: woxwidget.Text{Value: meta, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle}}}}
		if hasActions {
			headerChildren = append(headerChildren, woxwidget.StackChild{Left: innerWidth - actionWidth, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6, Children: actions}})
		}
		children = append(children, woxwidget.Stack{Width: innerWidth, Height: 18, Children: headerChildren})
	}
	if props.ToolText != "" {
		children = append(children, woxwidget.TextBlock{Value: props.ToolText, Width: innerWidth, Height: props.ToolLayout.Size.Height, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: textColor, Layout: &props.ToolLayout})
	} else {
		if props.Reasoning != "" {
			reasoningColor := textColor
			reasoningColor.A = 120
			reasoning := woxwidget.TextBlock{Value: props.Reasoning, Width: innerWidth, Height: props.ReasoningLayout.Size.Height, Style: woxui.TextStyle{Size: 11}, LineHeight: 15.4, Color: reasoningColor, Layout: &props.ReasoningLayout}
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
		reservedActionWidth := chatMessageActionWidth(props)
		footerWidth := props.TimestampWidth + 24 + reservedActionWidth
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
		}
		footer = woxwidget.Align{Width: footerWidth, Height: 18, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: footerChildren}}
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
	innerWidth := max(float32(0), width-16)
	children := make([]woxwidget.Widget, 0, len(tool.Details))
	for _, detail := range tool.Details {
		valueHeight := detail.Layout.Size.Height + 12
		children = append(children, woxwidget.Container{Width: innerWidth, Height: detail.Layout.Size.Height + 40, Padding: woxwidget.Insets{Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
			woxwidget.Container{Width: innerWidth, Height: 16, Child: woxwidget.Text{Value: detail.Label, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: theme.ResultSubtitle}},
			woxwidget.Container{Width: innerWidth, Height: valueHeight, Color: woxui.Color{A: 20}, BorderColor: woxui.Color{A: 10}, BorderWidth: 1, Padding: woxwidget.Insets{Left: 6, Top: 6, Right: 6, Bottom: 6}, Child: woxwidget.TextBlock{Value: detail.Value, Width: max(float32(0), innerWidth-12), Height: detail.Layout.Size.Height, Style: woxui.TextStyle{Size: 11}, LineHeight: 16, Color: theme.PreviewText, Layout: &detail.Layout}},
		}}})
	}
	return woxwidget.Container{Width: width, Height: tool.DetailsHeight, Radius: 8, Color: panelColor, BorderColor: borderColor, BorderWidth: 1, Padding: woxwidget.Insets{Left: 8, Top: 8, Right: 8, Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
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
func chatMessageActions(props ChatMessageProps, visible bool, onHover func(bool)) ([]woxwidget.Widget, float32) {
	if !visible {
		return nil, 0
	}
	actions := make([]woxwidget.Widget, 0, 2)
	width := float32(0)
	appendAction := func(name, label string, actionWidth float32, action func()) {
		if action == nil {
			return
		}
		if len(actions) > 0 {
			width += 8
		}
		width += actionWidth
		icon := woxcomponent.CopyGlyph(14, props.Theme.ResultSubtitle)
		if name == "edit" {
			icon = woxcomponent.EditGlyph(14, props.Theme.ResultSubtitle)
		} else if name == "retry" {
			icon = woxcomponent.RefreshGlyph(14, props.Theme.ResultSubtitle)
		}
		actions = append(actions, woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "chat-" + name + "-" + props.Key, Label: label, Icon: icon,
			Width: actionWidth, Height: 18, Radius: 5, HoverBackground: props.Theme.ActionBackground, FocusRingColor: props.Theme.Cursor, OnTap: action,
			OnHoverAt: func(inside bool, _ woxui.Rect) { onHover(inside) },
		}))
	}
	appendAction("copy", props.CopyLabel, 18, props.OnCopy)
	appendAction("edit", props.EditLabel, 18, props.OnEdit)
	appendAction("retry", props.RetryLabel, 18, props.OnRetry)
	return actions, width
}

// chatMessageActionWidth reserves the Flutter toolbar width while hover metadata is hidden.
func chatMessageActionWidth(props ChatMessageProps) float32 {
	count := 0
	for _, action := range []func(){props.OnCopy, props.OnEdit, props.OnRetry} {
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
	Width       float32
	Height      float32
	Key         string
	Editing     woxui.TextEditingState
	Focused     bool
	Hint        string
	Window      *woxui.Window
	Model       string
	ModelWidth  float32
	Status      string
	StatusColor woxui.Color
	ActionLabel string
	Sending     bool
	Theme       woxcomponent.Theme
	OnFocus     func()
	OnChanged   func(string)
	OnKey       func(woxui.KeyEvent) bool
	OnModels    func()
	OnSend      func()
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
	contentWidth := max(float32(0), props.ModelWidth-8)
	child := woxwidget.Container{Width: props.ModelWidth, Height: 20, Radius: 4, Color: background, Padding: woxwidget.Insets{Left: 4, Right: 4}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 0, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxcomponent.ModelTrainingGlyph(16, iconColor),
			woxwidget.Container{Width: 5},
			woxwidget.Align{Width: max(float32(0), contentWidth-39), Height: 20, Vertical: 0.5, Child: woxwidget.Text{Value: props.Model, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultTitle}},
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
	cardHeight := max(float32(78), props.Height-14)
	editorHeight := max(float32(36), cardHeight-toolbarHeight-1)
	input := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: "chat-input-" + props.Key, Label: props.Hint, Hint: props.Hint, Width: props.Width, Height: editorHeight,
		Padding: woxwidget.Insets{Left: 14, Top: 8, Right: 14, Bottom: 7}, Background: props.Theme.QueryBackground,
		Style: woxui.TextStyle{Size: 13}, Value: props.Editing.Text, Focused: props.Focused, MaxLines: 5, Window: props.Window, Theme: props.Theme,
		OnChanged: props.OnChanged, OnKey: props.OnKey, OnFocusChange: func(focused bool) {
			if focused && props.OnFocus != nil {
				props.OnFocus()
			}
		},
	})
	divider := props.Theme.ResultSubtitle
	divider.A = uint8(float32(divider.A) * 0.14)
	modelButton := ChatModelSelector(props)
	label := "↵  " + props.ActionLabel
	variant := woxcomponent.ButtonPrimary
	if props.Sending {
		label = props.ActionLabel
		variant = woxcomponent.ButtonSurface
	}
	statusLeft := props.ModelWidth + 18
	statusWidth := max(float32(0), props.Width-statusLeft-100)
	toolbarChildren := []woxwidget.StackChild{
		{Left: 8, Child: woxwidget.Align{Width: props.ModelWidth, Height: toolbarHeight, Vertical: 0.5, Child: modelButton}},
		{Left: props.Width - 90, Top: 6, Child: woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "chat-send-" + props.Key, Label: label, Width: 82, Height: 30, Radius: 7, Size: woxcomponent.ButtonCompact, Variant: variant, OnTap: props.OnSend, Theme: props.Theme})},
	}
	if props.Status != "" && statusWidth > 30 {
		toolbarChildren = append(toolbarChildren, woxwidget.StackChild{Left: statusLeft, Top: 14, Child: woxwidget.Container{Width: statusWidth, Height: 16, Child: woxwidget.Text{Value: props.Status, Style: woxui.TextStyle{Size: 9}, Color: props.StatusColor}}})
	}
	card := woxwidget.Container{Width: props.Width, Height: cardHeight, Radius: 9, Color: props.Theme.QueryBackground, BorderColor: divider, BorderWidth: 1, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		input,
		woxwidget.Container{Width: props.Width, Height: 1, Color: divider},
		woxwidget.Stack{Width: props.Width, Height: toolbarHeight, Children: toolbarChildren},
	}}}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Top: 6, Bottom: 8}, Child: card}
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
			Width: innerWidth, Height: 40, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 11, Right: 10}, Child: woxwidget.Text{Value: option.Label, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.PreviewText},
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
	children = append(children, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Painter{Width: max(float32(0), innerWidth-160), Height: 30},
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "chat-question-cancel", Label: "Cancel", Width: 76, Height: 30, Variant: woxcomponent.ButtonSurface, OnTap: props.OnCancel, Theme: props.Theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "chat-question-submit", Label: "Submit", Width: 76, Height: 30, Variant: woxcomponent.ButtonPrimary, OnTap: props.OnSubmit, Theme: props.Theme}),
	}})
	return woxwidget.Container{Width: props.Width, Height: props.Height, Radius: 9, Color: props.Theme.ActionBackground, Padding: woxwidget.Insets{Left: 12, Top: 8, Right: 12, Bottom: 8}, Child: woxwidget.Clip{
		Width: innerWidth, Height: max(float32(0), props.Height-16), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: children},
	}}
}
