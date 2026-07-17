package view

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
		left := float32(10)
		top := float32(6)
		if props.Panel != "history" {
			left += (innerWidth - props.Catalog.Width) / 2
			top += max(headerHeight, innerHeight-inputHeight-props.Catalog.Height-6)
		}
		layers = append(layers, woxwidget.StackChild{Left: left, Top: top, Child: ChatCatalog(*props.Catalog)})
	}
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: layers}
}

// ChatHeaderProps contains the current conversation title and header actions.
type ChatHeaderProps struct {
	Width       float32
	Height      float32
	Key         string
	Title       string
	HistoryOpen bool
	ShowDebug   bool
	DebugOpen   bool
	Theme       woxcomponent.Theme
	OnHistory   func()
	OnDebug     func()
}

// ChatHeader builds the compact chat title bar.
func ChatHeader(props ChatHeaderProps) woxwidget.Widget {
	menuBackground := woxui.Color{}
	menuLabel := "☰"
	if props.HistoryOpen {
		menuBackground = props.Theme.ActionBackground
		menuLabel = "×"
	}
	debugWidth := float32(0)
	if props.ShowDebug {
		debugWidth = 48
	}
	titleWidth := max(float32(60), props.Width-48-debugWidth)
	children := []woxwidget.StackChild{
		{Left: 2, Top: 5, Child: woxwidget.Gesture{ID: "chat-history-" + props.Key, OnTap: props.OnHistory, Child: woxwidget.Container{Width: 36, Height: 36, Radius: 7, Color: menuBackground, Padding: woxwidget.Insets{Left: 9, Top: 7}, Child: woxwidget.Text{Value: menuLabel, Style: woxui.TextStyle{Size: 18}, Color: props.Theme.ResultSubtitle}}}},
		{Left: 44, Top: 14, Child: woxwidget.Container{Width: titleWidth, Height: 22, Child: woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText}}},
	}
	if props.ShowDebug {
		children = append(children, woxwidget.StackChild{Left: props.Width - 48, Top: 6, Child: chatHeaderButton("chat-debug-"+props.Key, "Trace", 46, props.DebugOpen, props.Theme, props.OnDebug)})
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
	SelectID string
	DeleteID string
	Title    string
	Subtitle string
	Selected bool
	OnSelect func()
	OnDelete func()
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
	Theme         woxcomponent.Theme
	OnScroll      func(float32)
	OnNew         func()
}

// ChatCatalog builds a floating chat catalog.
func ChatCatalog(props ChatCatalogProps) woxwidget.Widget {
	const rowHeight = float32(44)
	innerWidth := max(float32(0), props.Width-20)
	viewportHeight := max(float32(40), props.Height-42)
	header := woxwidget.Widget(woxwidget.Container{Width: innerWidth, Height: 24, Child: woxwidget.Text{Value: props.Label, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionHeader}})
	if props.ShowNew {
		header = woxwidget.Stack{Width: innerWidth, Height: 24, Children: []woxwidget.StackChild{
			{Top: 5, Child: woxwidget.Container{Width: max(float32(0), innerWidth-54), Height: 18, Child: woxwidget.Text{Value: props.Label, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionHeader}}},
			{Left: innerWidth - 48, Child: chatHeaderButton("chat-new-"+props.Key, "New", 48, false, props.Theme, props.OnNew)},
		}}
	}
	rows := make([]woxwidget.Widget, 0, max(1, len(props.Items)))
	for _, item := range props.Items {
		rows = append(rows, chatCatalogItem(item, innerWidth, rowHeight, props.Theme))
	}
	if len(rows) == 0 {
		rows = append(rows, woxwidget.Container{Width: innerWidth, Height: viewportHeight, Padding: woxwidget.Insets{Left: 10, Top: 18}, Child: woxwidget.TextBlock{Value: props.EmptyMessage, Width: max(float32(0), innerWidth-20), Height: 48, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: props.Theme.ResultSubtitle}})
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Radius: 9, Color: props.Theme.ActionBackground, Padding: woxwidget.Insets{Left: 10, Top: 7, Right: 10, Bottom: 7}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
		header,
		woxwidget.Gesture{ID: "chat-catalog-scroll-" + props.Key, OnScroll: func(delta woxui.Point) {
			if props.OnScroll != nil {
				props.OnScroll(-delta.Y)
			}
		}, Child: woxwidget.ScrollView{Width: innerWidth, Height: viewportHeight, ContentHeight: props.ContentHeight, Offset: props.Scroll, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}},
	}}}
}

// chatCatalogItem renders the shared two-line catalog row and optional delete target.
func chatCatalogItem(item ChatCatalogItemProps, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	background := theme.QueryBackground
	if item.OnDelete != nil {
		background = woxui.Color{}
	}
	if item.Selected {
		background = theme.SelectedBackground
	}
	mainWidth := width
	rightPadding := float32(10)
	textInset := float32(20)
	if item.OnDelete != nil {
		mainWidth = max(float32(80), width-44)
		rightPadding = 8
		textInset = 18
	}
	main := woxwidget.Gesture{ID: item.SelectID, OnTap: item.OnSelect, Child: woxwidget.Container{
		Width: mainWidth, Height: height - 4, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 5, Right: rightPadding}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
			woxwidget.Container{Width: max(float32(0), mainWidth-textInset), Height: 16, Child: woxwidget.Text{Value: item.Title, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: theme.PreviewText}},
			woxwidget.Container{Width: max(float32(0), mainWidth-textInset), Height: 14, Child: woxwidget.Text{Value: item.Subtitle, Style: woxui.TextStyle{Size: 9}, Color: theme.ResultSubtitle}},
		}},
	}}
	children := []woxwidget.Widget{main}
	if item.OnDelete != nil {
		children = append(children, woxwidget.Gesture{ID: item.DeleteID, OnTap: item.OnDelete, Child: woxwidget.Container{
			Width: 40, Height: height - 4, Radius: 7, Color: theme.QueryBackground, Padding: woxwidget.Insets{Left: 15, Top: 12}, Child: woxwidget.Text{Value: "×", Style: woxui.TextStyle{Size: 14}, Color: theme.ResultSubtitle},
		}})
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Bottom: 4}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, Children: children}}
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
	body := woxwidget.Gesture{ID: "chat-debug-scroll-" + props.Key, OnScroll: func(delta woxui.Point) {
		if props.OnScroll != nil {
			props.OnScroll(-delta.Y)
		}
	}, Child: woxwidget.ScrollView{Width: innerWidth, Height: viewportHeight, ContentHeight: props.ContentHeight, Offset: props.Scroll, Child: woxwidget.Container{
		Width: innerWidth, Height: props.ContentHeight, Radius: 7, Color: props.Theme.QueryBackground, Padding: woxwidget.Insets{Left: 8, Top: 8, Right: 8, Bottom: 8},
		Child: woxwidget.TextBlock{Value: props.Value, Width: max(float32(20), innerWidth-16), Height: props.Layout.Size.Height, Style: woxui.TextStyle{Size: 10}, LineHeight: 16, Color: props.Theme.PreviewText, Layout: &props.Layout},
	}}}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Radius: 9, Color: props.Theme.ActionBackground, Padding: woxwidget.Insets{Left: 10, Top: 7, Right: 10, Bottom: 7}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{header, body}}}
}

// ChatMessageProps contains one prepared conversation and its controller callbacks.
type ChatMessageProps struct {
	Key             string
	Role            string
	Timestamp       string
	Text            string
	TextLayout      woxwidget.TextBlockLayout
	Reasoning       string
	ReasoningLayout woxwidget.TextBlockLayout
	ToolText        string
	ToolLayout      woxwidget.TextBlockLayout
	Skills          string
	Images          []*woxui.Image
	Theme           woxcomponent.Theme
	OnCopy          func()
	OnEdit          func()
	OnRetry         func()
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
	for index, message := range messages {
		height += chatMessageHeight(message)
		if index > 0 {
			height += 10
		}
	}
	return max(viewportHeight, height)
}

// ChatMessages builds the scrollable conversation viewport.
func ChatMessages(props ChatMessagesProps) woxwidget.Widget {
	innerWidth := max(float32(0), props.Width-20)
	innerHeight := max(float32(0), props.Height-16)
	if len(props.Messages) == 0 {
		color := props.Theme.ResultTitle
		color.A = uint8(float32(color.A) * 0.59)
		textWidth := min(max(float32(0), innerWidth-48), props.EmptyTextWidth)
		left := max(float32(24), (innerWidth-textWidth)/2)
		top := max(float32(0), (innerHeight-props.EmptyTextHeight)/2)
		return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 10, Top: 8, Right: 10, Bottom: 8}, Child: woxwidget.Stack{Width: innerWidth, Height: innerHeight, Children: []woxwidget.StackChild{
			{Left: left, Top: top, Child: woxwidget.Container{Width: textWidth, Height: props.EmptyTextHeight, Child: woxwidget.Text{Value: props.EmptyMessage, Style: woxui.TextStyle{Size: 28, Weight: woxui.FontWeightSemibold}, Color: color}}},
		}}}
	}
	rows := make([]woxwidget.Widget, 0, len(props.Messages))
	for _, message := range props.Messages {
		rows = append(rows, ChatMessage(message, innerWidth))
	}
	contentHeight := max(innerHeight, props.ContentHeight)
	maxOffset := max(float32(0), contentHeight-innerHeight)
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 10, Top: 8, Right: 10, Bottom: 8}, Child: woxwidget.Gesture{
		ID: "chat-message-scroll-" + props.Key, OnScroll: func(delta woxui.Point) {
			if props.OnScroll != nil {
				props.OnScroll(-delta.Y, maxOffset)
			}
		}, Child: woxwidget.ScrollView{Width: innerWidth, Height: innerHeight, ContentHeight: contentHeight, Offset: min(max(float32(0), props.Scroll), maxOffset), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: rows}},
	}}
}

// ChatMessage maps a prepared conversation to one portable card.
func ChatMessage(props ChatMessageProps, width float32) woxwidget.Widget {
	cardWidth := width
	left := float32(0)
	background := props.Theme.QueryBackground
	textColor := props.Theme.PreviewText
	role := strings.ToUpper(props.Role)
	if props.Role == "user" {
		cardWidth = max(float32(180), width*0.82)
		left = max(float32(0), width-cardWidth)
		background = props.Theme.SelectedBackground
		if background.A > 190 {
			background.A = 190
		}
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

	innerWidth := max(float32(40), cardWidth-24)
	actions, actionWidth := chatMessageActions(props)
	hasActions := len(actions) > 0
	showRoleHeader := props.Role == "tool" || props.Role == "system" || props.ToolText != ""
	children := make([]woxwidget.Widget, 0, 6)
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
			reasoningWidth := max(float32(20), innerWidth-16)
			children = append(children, woxwidget.Container{Width: innerWidth, Height: props.ReasoningLayout.Size.Height + 12, Radius: 7, Color: props.Theme.ActionBackground, Padding: woxwidget.Insets{Left: 8, Top: 6, Right: 8, Bottom: 6}, Child: woxwidget.TextBlock{
				Value: props.Reasoning, Width: reasoningWidth, Height: props.ReasoningLayout.Size.Height, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: props.Theme.ResultSubtitle, Layout: &props.ReasoningLayout,
			}})
		}
		if props.Text != "" {
			children = append(children, woxwidget.TextBlock{Value: props.Text, Width: innerWidth, Height: props.TextLayout.Size.Height, Style: woxui.TextStyle{Size: 13}, LineHeight: 19, Color: textColor, Layout: &props.TextLayout})
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
	if !showRoleHeader && (props.Timestamp != "" || hasActions) {
		metaWidth := innerWidth
		if hasActions {
			metaWidth = max(float32(0), innerWidth-actionWidth-8)
		}
		footerChildren := []woxwidget.StackChild{{Top: 3, Child: woxwidget.Container{Width: metaWidth, Height: 15, Child: woxwidget.Text{Value: props.Timestamp, Style: woxui.TextStyle{Size: 9}, Color: props.Theme.ResultSubtitle}}}}
		if hasActions {
			footerChildren = append(footerChildren, woxwidget.StackChild{Left: innerWidth - actionWidth, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6, Children: actions}})
		}
		children = append(children, woxwidget.Stack{Width: innerWidth, Height: 18, Children: footerChildren})
	}

	cardHeight := chatMessageHeight(props)
	card := woxwidget.Container{Width: cardWidth, Height: cardHeight, Radius: 10, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 10, Right: 12, Bottom: 10}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: children}}
	return woxwidget.Stack{Width: width, Height: cardHeight, Children: []woxwidget.StackChild{{Left: left, Child: card}}}
}

// chatMessageActions builds the available controller-owned message actions.
func chatMessageActions(props ChatMessageProps) ([]woxwidget.Widget, float32) {
	actions := make([]woxwidget.Widget, 0, 2)
	width := float32(0)
	appendAction := func(name, label string, actionWidth float32, action func()) {
		if action == nil {
			return
		}
		if len(actions) > 0 {
			width += 6
		}
		width += actionWidth
		actions = append(actions, woxwidget.Gesture{ID: "chat-" + name + "-" + props.Key, OnTap: action, Child: woxwidget.Container{
			Width: actionWidth, Height: 18, Radius: 6, Color: props.Theme.ActionBackground, Padding: woxwidget.Insets{Left: 7, Top: 3}, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText},
		}})
	}
	appendAction("copy", "Copy", 38, props.OnCopy)
	appendAction("edit", "Edit", 34, props.OnEdit)
	appendAction("retry", "Retry", 40, props.OnRetry)
	return actions, width
}

// chatMessageHeight derives the card extent from the visible message sections.
func chatMessageHeight(props ChatMessageProps) float32 {
	height := float32(0)
	parts := 0
	add := func(value float32) {
		if value <= 0 {
			return
		}
		height += value
		parts++
	}
	hasActions := props.OnCopy != nil || props.OnEdit != nil || props.OnRetry != nil
	showRoleHeader := props.Role == "tool" || props.Role == "system" || props.ToolText != ""
	if showRoleHeader {
		add(18)
	}
	if props.ToolText != "" {
		add(props.ToolLayout.Size.Height)
	} else {
		if props.Reasoning != "" {
			add(props.ReasoningLayout.Size.Height + 12)
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
	if !showRoleHeader && (props.Timestamp != "" || hasActions) {
		add(18)
	}
	return height + float32(max(0, parts-1))*6 + 20
}

// ChatInputProps contains the controlled editor and toolbar state.
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
	Sending     bool
	Theme       woxcomponent.Theme
	OnCaret     func(int)
	OnModels    func()
	OnSend      func()
}

// ChatInput builds the multiline editor card and send toolbar.
func ChatInput(props ChatInputProps) woxwidget.Widget {
	const toolbarHeight = float32(42)
	cardHeight := max(float32(78), props.Height-14)
	editorHeight := max(float32(36), cardHeight-toolbarHeight-1)
	input := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: "chat-input-" + props.Key, Label: props.Hint, Hint: props.Hint, Width: props.Width, Height: editorHeight,
		Padding: woxwidget.Insets{Left: 14, Top: 8, Right: 14, Bottom: 7}, Background: props.Theme.QueryBackground,
		Style: woxui.TextStyle{Size: 13}, State: props.Editing, Focused: props.Focused, MaxLines: 5, Window: props.Window, Theme: props.Theme,
		ControllerManagedFocus: true, OnCaret: props.OnCaret,
	})
	divider := props.Theme.ResultSubtitle
	divider.A = uint8(float32(divider.A) * 0.14)
	modelButton := woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "chat-models-" + props.Key, Label: props.Model + "  ▾", Width: props.ModelWidth, Height: 34, Radius: 5, Variant: woxcomponent.ButtonSurface, OnTap: props.OnModels, Theme: props.Theme})
	label := "Send"
	variant := woxcomponent.ButtonPrimary
	if props.Sending {
		label = "Stop"
		variant = woxcomponent.ButtonSurface
	}
	statusLeft := props.ModelWidth + 18
	statusWidth := max(float32(0), props.Width-statusLeft-100)
	toolbarChildren := []woxwidget.StackChild{
		{Left: 8, Top: 4, Child: modelButton},
		{Left: props.Width - 90, Top: 6, Child: woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "chat-send-" + props.Key, Label: label, Width: 82, Height: 30, Radius: 7, Variant: variant, OnTap: props.OnSend, Theme: props.Theme})},
	}
	if props.Status != "" && statusWidth > 30 {
		toolbarChildren = append(toolbarChildren, woxwidget.StackChild{Left: statusLeft, Top: 14, Child: woxwidget.Container{Width: statusWidth, Height: 16, Child: woxwidget.Text{Value: props.Status, Style: woxui.TextStyle{Size: 9}, Color: props.StatusColor}}})
	}
	card := woxwidget.Container{Width: props.Width, Height: cardHeight, Radius: 9, Color: props.Theme.QueryBackground, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
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

// ChatQuestionInputProps contains the optional controlled free-form answer.
type ChatQuestionInputProps struct {
	ID      string
	Height  float32
	Editing woxui.TextEditingState
	Focused bool
	Window  *woxui.Window
	OnCaret func(int)
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
			Style: woxui.TextStyle{Size: 12}, State: props.Input.Editing, Focused: props.Input.Focused, MaxLines: 4,
			Window: props.Input.Window, Theme: props.Theme, ControllerManagedFocus: true, OnCaret: props.Input.OnCaret,
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
