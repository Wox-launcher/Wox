package launcher

import (
	"testing"
	"wox/ui/launcher/view/preview"
	"wox/ui/widget"
)

func TestChatMessageRenderingSharesMarkdownAcrossHosts(t *testing.T) {
	app := &App{previewLayouts: map[string]*textLayoutCache{}}
	for _, key := range []string{"chat-reply", "theme-editor-reply"} {
		props := preview.ChatMessageProps{Key: key, Role: "assistant", Text: "**Heading**\n\n- First\n- Second\n\n`color`"}
		plain := app.prepareChatMessage(props, nil, 400, 1)
		if plain.Markdown == nil || plain.Markdown.OnOpenLink == nil || plain.TextLayout.Size.Height <= 0 {
			t.Fatalf("%s did not use rich reply rendering", key)
		}
		props.TextTrailing = widget.Container{Width: 32, Height: 32}
		withAction := app.prepareChatMessage(props, nil, 400, 1)
		if withAction.Markdown.InlineTrailing == nil || withAction.TextLayout.Size.Height < plain.TextLayout.Size.Height {
			t.Fatal("business action must survive Markdown rendering and cache reuse")
		}
		props.Role = "user"
		if user := app.prepareChatMessage(props, nil, 400, 1); user.Markdown != nil {
			t.Fatal("user input must retain its literal formatting")
		}
	}
}
