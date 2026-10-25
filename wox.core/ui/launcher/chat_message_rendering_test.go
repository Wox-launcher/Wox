package launcher

import (
	"testing"
	"wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestChatMessageLayoutsUseHostWindow covers both pop-out chat and Settings AI assistance.
func TestChatMessageLayoutsUseHostWindow(t *testing.T) {
	app := bindTestApp(&App{
		window:   &woxui.Window{},
		previews: previewController{previewLayouts: map[string]*textLayoutCache{}},
	})
	for _, host := range []*woxui.Window{app.window, {}, {}} {
		for _, role := range []string{"user", "assistant"} {
			props := preview.ChatMessageProps{Key: role, Role: role, Text: "Message", Reasoning: "Reasoning"}
			message := app.prepareChatMessage(props, host, 400, 1)
			if message.Window != host || (message.Markdown != nil && message.Markdown.Window != host) {
				t.Fatal("message rendering retained another window")
			}
			for _, key := range []string{"chat-text-" + role, "chat-reasoning-" + role} {
				if role == "assistant" && key == "chat-text-assistant" {
					continue
				}
				if cached := app.previews.previewLayouts[key]; cached == nil || cached.key.window != host {
					t.Fatalf("%s layout did not follow its host window", key)
				}
			}
		}
	}
}

func TestChatMessageRenderingSharesMarkdownAcrossHosts(t *testing.T) {
	app := bindTestApp(&App{previews: previewController{previewLayouts: map[string]*textLayoutCache{}}})
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
