package preview

import (
	"testing"

	r "wox/ui/runtime"
	w "wox/ui/widget"
)

func TestChatConversationOwnsHostGeometry(t *testing.T) {
	for _, key := range []string{"chat", "dedicated", "theme-editor-ai"} {
		t.Run(key, func(t *testing.T) {
			props := ChatConversationProps{ChatPreviewProps: ChatPreviewProps{
				Key: key, Width: 400, Height: 500,
				Input:    ChatInputProps{Key: key, Editing: r.TextEditingState{Text: "first\nsecond"}},
				Messages: ChatMessagesProps{EmptyMessage: "Create or refine a theme"},
				Catalog:  &ChatCatalogProps{Label: "Models", ContentHeight: 76},
			}}
			panes := prepareChatConversation(props)
			if panes.Input.Width != 380 || panes.Messages.Width != 380 || panes.Input.Height+panes.Messages.Height+14 != 500 {
				t.Fatalf("inconsistent shared layout: input=%+v messages=%+v", panes.Input, panes.Messages)
			}
			if panes.Catalog.Height != 118 || panes.Catalog.Width != 380 {
				t.Fatalf("catalog must fit two rows: %+v", panes.Catalog)
			}
			props.Footer = w.Text{Value: "Undo"}
			withFooter := prepareChatConversation(props)
			if withFooter.Messages.Height != panes.Messages.Height-40 {
				t.Fatal("business footer must reserve space without changing composer geometry")
			}
			props.Height = 170
			short := prepareChatConversation(props)
			if short.Catalog.Height > short.Messages.Height || short.Messages.Height < 0 {
				t.Fatal("catalog must stay above composer in a short host")
			}
		})
	}
}

func TestChatConversationCommonKeyboard(t *testing.T) {
	sends, selections, moves := 0, 0, 0
	send := func() { sends++ }
	enter := r.KeyEvent{Key: r.KeyEnter, Down: true}
	ChatComposerKey(enter, false, false, send, nil)
	ChatComposerKey(enter, true, false, send, nil)
	ChatComposerKey(enter, false, true, send, nil)
	enter.Composing = true
	ChatComposerKey(enter, false, false, send, nil)
	enter.Composing = false
	enter.Modifiers = r.KeyModifierShift
	if ChatComposerKey(enter, false, false, send, nil) || sends != 1 {
		t.Fatalf("send/IME/newline rules: sends=%d", sends)
	}
	ChatCatalogKey(r.KeyEvent{Key: r.KeyTab, Down: true, Modifiers: r.KeyModifierShift}, func(delta int) { moves += delta }, func() { selections++ }, func() {})
	ChatCatalogKey(r.KeyEvent{Key: r.KeyEnter, Down: true}, func(int) {}, func() { selections++ }, func() {})
	if moves != -1 || selections != 1 {
		t.Fatal("catalog keyboard navigation differs between hosts")
	}
}

func TestSubmitChatMessageConsumesOnlyAcceptedDraft(t *testing.T) {
	draft := "  Keep this question  "
	clear := func() { draft = "" }
	if SubmitChatMessage(draft, func(string) bool { return false }, clear) || draft == "" {
		t.Fatal("validation failure must preserve the draft")
	}
	var sent string
	if !SubmitChatMessage(draft, func(text string) bool { sent = text; return true }, clear) || draft != "" || sent != "Keep this question" {
		t.Fatal("accepted message must be captured and composer cleared immediately")
	}
}

func TestChatMessageTrailingActionReservesTextSpace(t *testing.T) {
	props := ChatConversationProps{ChatPreviewProps: ChatPreviewProps{Width: 300, Height: 400, Messages: ChatMessagesProps{Messages: []ChatMessageProps{{Role: "assistant", Text: "Applied to draft", TextTrailing: w.Container{Width: 32, Height: 32}}}}}}
	panes := prepareChatConversation(props)
	message := panes.Messages.Messages[0]
	if message.TextLayout.Size.Width > message.ContentWidth-38 || chatMessageHeight(message) < 32 {
		t.Fatal("message action overlaps text or falls outside the scroll extent")
	}
}

func TestChatMessageSpacingMatchesScrollExtent(t *testing.T) {
	messages := []ChatMessageProps{{Role: "user", Text: "Question"}, {Role: "assistant", Text: "Answer"}}
	height, _ := ChatMessagesScrollMetrics(messages, 0)
	if height != chatMessageHeight(messages[0])+chatMessageHeight(messages[1])+12 || height != chatMessageExtent(messages, 0)+chatMessageExtent(messages, 1) {
		t.Fatal("message spacing must be included once in both list and scroll geometry")
	}
}

func TestChatConversationUsesSharedUserMessageMeasurement(t *testing.T) {
	message := ChatMessageProps{Role: "user", Images: []*r.Image{nil}}
	panes := prepareChatConversation(ChatConversationProps{ChatPreviewProps: ChatPreviewProps{
		Width: 400, Height: 500, Messages: ChatMessagesProps{Messages: []ChatMessageProps{message}},
	}})
	var wrappingWidth float32
	cached := MeasureChatMessage(message, nil, panes.Messages.Width, func(_ string, text string, style r.TextStyle, width, lineHeight float32) w.TextBlockLayout {
		wrappingWidth = width
		return w.LayoutTextBlock(nil, text, style, width, 0, lineHeight)
	})
	embedded := panes.Messages.Messages[0]
	if embedded.ContentWidth != 82 || cached.ContentWidth != embedded.ContentWidth {
		t.Fatalf("user bubbles must hug content in both cached and embedded hosts: cached=%v embedded=%v", cached.ContentWidth, embedded.ContentWidth)
	}
	if wrappingWidth+24 > (panes.Messages.Width-4)*.82 {
		t.Fatal("text wrapping exceeds the shared user bubble width")
	}
}

func TestChatConversationFlushHorizontal(t *testing.T) {
	panes := prepareChatConversation(ChatConversationProps{ChatPreviewProps: ChatPreviewProps{Width: 400, Height: 500, FlushHorizontal: true}})
	if panes.Messages.Width != 400 || panes.Input.Width != 400 {
		t.Fatal("embedded chat must align with its host controls")
	}
}
