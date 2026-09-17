package preview

import (
	"strings"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ChatConversationProps is the host contract for a complete embedded conversation.
// Hosts supply data and actions; pane geometry and the composer toolbar belong here.
type ChatConversationProps struct {
	ChatPreviewProps
	Footer woxwidget.Widget
	// PrepareMessages supports cached rich messages and tool rounds in AI Chat.
	// It runs before building widgets, never inside a cached Boundary.Build.
	PrepareMessages func(width, height float32) ChatMessagesProps
	PrepareCatalog  func(width, available float32) *ChatCatalogProps
}

// ChatConversation is shared by launcher chat, dedicated chat and theme assistance.
func ChatConversation(props ChatConversationProps) woxwidget.Widget {
	panes := prepareChatConversation(props)
	conversation := ChatPreview(panes)
	if props.Footer == nil {
		return conversation
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{
		conversation, woxwidget.Align{Width: props.Width, Height: 32, Horizontal: 1, Child: props.Footer},
	}}
}

// prepareChatConversation measures all common panes in the host window's logical units.
func prepareChatConversation(props ChatConversationProps) ChatPreviewProps {
	p := props.ChatPreviewProps
	if props.Footer != nil {
		p.Height = max(float32(0), p.Height-40)
	}
	width := p.Width
	if p.History != nil {
		width -= p.History.Width
	}
	if !p.FlushHorizontal {
		width -= 20
	}
	width = max(float32(0), width)
	p.Input.Width = width
	p.Input.Height = ChatComposerHeightForAttachments(p.Input.Attachments, ChatComposerVisibleLines(p.Input.Editing.Text, width, p.Input.Window, p.Input.RichRuns))
	height := max(float32(0), p.Height-14-p.Input.Height)
	if p.Header != nil {
		height -= p.Header.Height
	}
	if p.Question != nil {
		height -= p.Question.Height
	}
	if p.Debug != nil {
		height -= p.Debug.Height
	}
	height = max(float32(0), height)
	if props.PrepareMessages != nil {
		p.Messages = props.PrepareMessages(width, height)
	} else {
		// Copy before measuring: callers may retain their message snapshots across frames.
		p.Messages.Messages = append([]ChatMessageProps(nil), p.Messages.Messages...)
		for i := range p.Messages.Messages {
			p.Messages.Messages[i] = MeasureChatMessage(p.Messages.Messages[i], p.Input.Window, width, nil)
		}
		p.Messages.ContentHeight, p.Messages.ExtentRevision = ChatMessagesScrollMetrics(p.Messages.Messages, height-14)
		style, lineHeight := p.Messages.EmptyTextStyle, p.Messages.EmptyLineHeight
		if style.Size <= 0 {
			style = woxui.TextStyle{Size: 28, Weight: woxui.FontWeightSemibold}
		}
		if lineHeight <= 0 {
			lineHeight = 36
		}
		p.Messages.EmptyTextWidth = max(float32(0), width-24)
		layout := woxwidget.LayoutTextBlock(p.Input.Window, p.Messages.EmptyMessage, style, p.Messages.EmptyTextWidth, 0, lineHeight)
		p.Messages.EmptyTextHeight, p.Messages.EmptyTextLayout = layout.Size.Height, &layout
	}
	p.Messages.Width, p.Messages.Height = width, height
	hostKey := p.Input.OnKey
	p.Input.OnKey = func(event woxui.KeyEvent) bool {
		if hostKey != nil && hostKey(event) {
			return true
		}
		return ChatComposerKey(event, p.Input.Sending, p.Input.Disabled || p.Input.Importing, p.Input.OnSend, func(delta float32) {
			if p.Messages.OnScroll != nil {
				p.Messages.OnScroll(delta, max(float32(0), p.Messages.ContentHeight-max(float32(0), height-14)))
			}
		})
	}
	if props.PrepareCatalog != nil {
		p.Catalog = props.PrepareCatalog(width, height)
	}
	if p.Catalog != nil {
		catalog := *p.Catalog
		catalog.Width = width
		catalog.Height = ChatCatalogHeight(catalog.ContentHeight, catalog.Label != "", height)
		p.Catalog = &catalog
	}
	return p
}

// ChatCatalogHeight fits short lists to their content and caps long command palettes.
func ChatCatalogHeight(contentHeight float32, hasTitle bool, available float32) float32 {
	height := max(float32(40), contentHeight) + 14
	if hasTitle {
		height += 28
	}
	return max(float32(0), min(height, 310, available))
}

// ChatCatalogKey handles command navigation consistently across conversation hosts.
func ChatCatalogKey(event woxui.KeyEvent, move func(int), activate, dismiss func()) bool {
	if !event.Down || event.Composing {
		return false
	}
	switch event.Key {
	case woxui.KeyEscape:
		dismiss()
	case woxui.KeyEnter:
		activate()
	case woxui.KeyArrowUp:
		move(-1)
	case woxui.KeyArrowDown, woxui.KeyTab:
		delta := 1
		if event.Key == woxui.KeyTab && event.Modifiers&woxui.KeyModifierShift != 0 {
			delta = -1
		}
		move(delta)
	default:
		return false
	}
	return true
}

// ChatComposerKey leaves composition and Shift+Enter to the native text editor.
func ChatComposerKey(event woxui.KeyEvent, sending, disabled bool, send func(), scrollPage func(float32)) bool {
	if !event.Down || event.Composing {
		return false
	}
	if event.Key == woxui.KeyEnter && event.Modifiers&woxui.KeyModifierShift == 0 {
		if !sending && !disabled && send != nil {
			send()
		}
		return true
	}
	if scrollPage != nil && (event.Key == woxui.KeyPageUp || event.Key == woxui.KeyPageDown) {
		delta := float32(-240)
		if event.Key == woxui.KeyPageDown {
			delta = 240
		}
		scrollPage(delta)
		return true
	}
	return false
}

// SubmitChatMessage consumes the composer only after the host accepts the turn.
// Request completion must never clear a newer draft or restore an already sent message.
func SubmitChatMessage(text string, accept func(string) bool, clearDraft func()) bool {
	if !accept(strings.TrimSpace(text)) {
		return false
	}
	clearDraft()
	return true
}

// ChatMessageTextWidth is the shared wrapping limit for plain text, reasoning and Markdown.
func ChatMessageTextWidth(width float32, role string) float32 {
	if role == "user" {
		return max(float32(24), (width-4)*.82-24)
	}
	return max(float32(24), width-8)
}

// MeasureChatMessage owns message geometry for every host; layout optionally supplies a cache.
func MeasureChatMessage(props ChatMessageProps, window *woxui.Window, width float32, layout func(string, string, woxui.TextStyle, float32, float32) woxwidget.TextBlockLayout) ChatMessageProps {
	if layout == nil {
		layout = func(_ string, text string, style woxui.TextStyle, width, lineHeight float32) woxwidget.TextBlockLayout {
			return woxwidget.LayoutTextBlock(window, text, style, width, 0, lineHeight)
		}
	}
	textWidth := ChatMessageTextWidth(width, props.Role)
	bodyWidth := textWidth
	if props.TextTrailing != nil {
		bodyWidth = max(float32(0), bodyWidth-38)
	}
	if props.Markdown == nil {
		props.TextLayout = layout("chat-text-"+props.Key, props.Text, woxui.TextStyle{Size: 13}, bodyWidth, 19)
	}
	props.ReasoningLayout = layout("chat-reasoning-"+props.Key, props.Reasoning, woxui.TextStyle{Size: 11}, textWidth, 16)
	props.ContentWidth = textWidth
	if props.Role == "user" {
		props.ContentWidth = 0
		for _, line := range props.TextLayout.Lines {
			if metrics, err := window.MeasureText(line, woxui.TextStyle{Size: 13}); err == nil {
				props.ContentWidth = max(props.ContentWidth, metrics.Size.Width)
			}
		}
		if props.Skills != "" {
			if metrics, err := window.MeasureText(props.Skills, woxui.TextStyle{Size: 10}); err == nil {
				props.ContentWidth = max(props.ContentWidth, metrics.Size.Width)
			}
		}
		if len(props.Images) > 0 {
			props.ContentWidth = max(props.ContentWidth, float32(len(props.Images))*82+float32(len(props.Images)-1)*8)
		}
		if len(props.Attachments) > 0 {
			props.ContentWidth = textWidth
		}
		props.ContentWidth = min(textWidth, props.ContentWidth)
	}
	return props
}
