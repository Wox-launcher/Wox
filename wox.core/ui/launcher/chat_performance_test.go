package launcher

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestChatStreamKeepsWidgetIdentity separates content revisions from retained interaction state.
func TestChatStreamKeepsWidgetIdentity(t *testing.T) {
	state := &chatPreviewState{key: "revision-1", resultID: "result", chat: chatData{ID: "chat"}}
	before := snapshotChatPreviewLocked(state).key
	state.key = "revision-2"
	if snapshotChatPreviewLocked(state).key != before {
		t.Fatal("stream update replaced scrollbar identity")
	}
	state.chat.ID = "other"
	if snapshotChatPreviewLocked(state).key == before {
		t.Fatal("different conversation retained the old identity")
	}
}

// TestPreviewTextReplacesStreamVersions guards the shared path used by chat reasoning.
func TestPreviewTextReplacesStreamVersions(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	style := woxui.TextStyle{Size: 11}
	for i := range 200 {
		value := "completed paragraph\n" + fmt.Sprint(i)
		got := app.previewTextLayout("reasoning", value, style, 560, 16)
		if len(app.previewLayouts) != 1 || app.previewLayouts["reasoning"].value != value {
			t.Fatal("stream update retained an obsolete text version")
		}
		reused := app.previewTextLayout("reasoning", value, style, 560, 16)
		if &got.Lines[0] != &reused.Lines[0] {
			t.Fatal("scrolling rebuilt unchanged reasoning")
		}
	}
	got := app.previewTextLayout("reasoning", "replacement", style, 320, 22)
	want := woxwidget.LayoutTextBlock(nil, "replacement", style, 320, 0, 22)
	if got.Size != want.Size || got.LineHeight != want.LineHeight || got.Lines[0] != "replacement" {
		t.Fatal("replacement or changed layout constraints reused stale output")
	}
	for i := range 130 {
		app.previewTextLayout(fmt.Sprint(i), "small", style, 560, 16)
	}
	if len(app.previewLayouts) > 128 {
		t.Fatal("text cache exceeded its entry budget")
	}
	app.previewLayouts["large"] = &textLayoutCache{value: strings.Repeat("x", 2<<20)}
	app.previewTextLayout("next", "small", style, 560, 16)
	if len(app.previewLayouts) != 1 {
		t.Fatal("text cache exceeded its source byte budget")
	}
	snapshot := &chatPreviewSnapshot{chat: chatData{ID: "width", IsStreaming: true, Conversations: []chatConversation{{Role: "assistant", Reasoning: "thinking"}}}}
	for _, width := range []float32{320, 560, 1000} {
		props := app.chatMessagesProps(snapshot, defaultPalette(), width, 500, 1.5)
		if props.Messages[0].ReasoningLayout.ConstraintWidth != width-8 {
			t.Fatal("adapter width disagrees with frameless message padding")
		}
	}
}

// TestCollapsedChatToolsSkipHiddenLayouts covers both disclosure levels and reopening.
func TestCollapsedChatToolsSkipHiddenLayouts(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	palette := defaultPalette()
	item := chatRenderItem{kind: "tool-activity", roundID: "group"}
	for i := range 31 {
		item.tools = append(item.tools, chatConversation{ID: fmt.Sprint(i), Role: "tool", ToolCallInfo: chatToolCallInfo{
			ID: fmt.Sprint(i), Name: "web_fetch", Status: "succeeded", Response: strings.Repeat("Tool response. ", 100),
		}})
	}
	props := app.chatToolActivityProps(item, nil, palette, 560)
	if len(props.Tools) != 0 || len(app.previewLayouts) != 0 || props.ToolSummary == "" {
		t.Fatal("collapsed group prepared hidden tools or lost its summary")
	}
	item.roundExpanded = true
	props = app.chatToolActivityProps(item, nil, palette, 560)
	if len(props.Tools) != 31 || len(app.previewLayouts) != 0 {
		t.Fatal("expanded group must prepare headers without hidden detail layouts")
	}
	props = app.chatToolActivityProps(item, map[string]bool{"0": true}, palette, 560)
	if len(props.Tools[0].Details) == 0 || props.Tools[0].DetailsHeight <= 16 || len(props.Tools[1].Details) != 0 {
		t.Fatal("only the expanded tool must prepare details")
	}
	clear(app.previewLayouts)
	item.roundExpanded = false
	app.chatToolActivityProps(item, map[string]bool{"0": true}, palette, 560)
	if len(app.previewLayouts) != 0 {
		t.Fatal("collapsing the parent still measured an expanded child")
	}
}

// BenchmarkChatRecording replays local chat data without checking private conversations into the repository.
func BenchmarkChatRecording(b *testing.B) {
	path := os.Getenv("WOX_SMOKE_CHAT_RECORDING")
	if path == "" {
		b.Skip("set WOX_SMOKE_CHAT_RECORDING to an AIChatPreviewData JSON file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	var data chatPreviewData
	if err := json.Unmarshal(raw, &data); err != nil {
		b.Fatal(err)
	}
	app := New(false, nil)
	defer app.cancel()
	snapshot := &chatPreviewSnapshot{chat: data.ActiveChat}
	palette := defaultPalette()
	app.chatMessagesProps(snapshot, palette, 560, 500, 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.chatMessagesProps(snapshot, palette, 560, 500, 1)
	}
}

// TestChatMarkdownCache covers reuse, layout invalidation, bounds and conversation lifetime.
func TestChatMarkdownCache(t *testing.T) {
	cache := chatMarkdownCache{}
	measures := 0
	props := woxcomponent.MarkdownProps{Width: 400, FontSize: 13, ResolveImage: func(string) (*woxui.Image, string) {
		measures++
		return nil, ""
	}}
	value := "**Answer**\n\n![image](https://example.com/image.png)"
	layout := chatMarkdownLayoutKey{width: 400, scale: 1}
	_, original := cache.measure("reply", value, layout, props)
	_, reused := cache.measure("reply", value, layout, props)
	if measures != 1 || reused != original {
		t.Fatalf("unchanged history was measured again: calls=%d size=%v want=%v", measures, reused, original)
	}
	for _, changed := range []chatMarkdownLayoutKey{
		{width: 500, scale: 1},
		{width: 500, scale: 1.5},
		{width: 500, scale: 2},
		{width: 500, scale: 2, font: "Arial"},
		{width: 500, scale: 2, font: "Arial", images: 1},
		{width: 500, scale: 2, font: "Arial", images: 1, window: &woxui.Window{}},
	} {
		before := measures
		props.Width = changed.width
		cache.measure("reply", value, changed, props)
		cache.measure("reply", value, changed, props)
		if measures != before+1 {
			t.Fatalf("layout change must measure once: %+v, calls=%d", changed, measures-before)
		}
		layout = changed
	}
	for i := range 100 {
		before := measures
		text := fmt.Sprintf("%s\n\nToken %d", value, i)
		cache.measure("reply", text, layout, props)
		if measures != before+1 || len(cache.entries) != 1 || cache.bytes != len(text) {
			t.Fatal("stream update failed to replace the previous cached version")
		}
	}
	cache = chatMarkdownCache{}
	for i := range chatMarkdownCacheEntries + 10 {
		cache.measure(fmt.Sprint(i), "small", layout, props)
	}
	if len(cache.entries) != chatMarkdownCacheEntries || cache.bytes > chatMarkdownCacheBytes {
		t.Fatal("entry bound was exceeded")
	}
	cache = chatMarkdownCache{}
	large := strings.Repeat("a", chatMarkdownCacheBytes/4)
	for i := range 5 {
		cache.measure(fmt.Sprint(i), large, layout, props)
	}
	if cache.bytes != chatMarkdownCacheBytes || len(cache.entries) != 4 {
		t.Fatal("source byte bound was exceeded")
	}
	app := New(false, nil)
	defer app.cancel()
	app.chatMarkdown = cache
	app.chatMarkdown.chatID = "old"
	app.chatMessagesProps(&chatPreviewSnapshot{chat: chatData{ID: "new"}}, defaultPalette(), 400, 300, 1)
	if len(app.chatMarkdown.entries) != 0 || app.chatMarkdown.chatID != "new" {
		t.Fatal("switching conversations retained the old cache")
	}
	app.chatMarkdown = cache
	app.deactivateChatPreview()
	if app.chatMarkdown.entries != nil || app.chatMarkdown.bytes != 0 {
		t.Fatal("deactivation retained Markdown documents")
	}
	app.chatMarkdown = cache
	app.resetChatPreview()
	if app.chatMarkdown.entries != nil {
		t.Fatal("reset retained Markdown documents")
	}
}

// BenchmarkChatMessagesPreparation includes work done before LazyList selects visible rows.
func BenchmarkChatMessagesPreparation(b *testing.B) {
	app := New(false, nil)
	defer app.cancel()
	snapshot := &chatPreviewSnapshot{chat: chatData{ID: "benchmark", IsStreaming: true}}
	for i := range 200 {
		snapshot.chat.Conversations = append(snapshot.chat.Conversations, chatConversation{
			ID: fmt.Sprint(i), Role: "assistant", Text: fmt.Sprintf("**Reply %d**\n\n%s", i, strings.Repeat("Historical paragraph. ", 20)),
		})
	}
	palette := defaultPalette()
	app.chatMessagesProps(snapshot, palette, 800, 500, 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		snapshot.chat.Conversations[199].Text = fmt.Sprintf("Streaming answer %d", i)
		app.chatMessagesProps(snapshot, palette, 800, 500, 1)
	}
}

// BenchmarkChatReasoningPreparation measures a growing plain-text reasoning message, not Markdown.
func BenchmarkChatReasoningPreparation(b *testing.B) {
	app := New(false, nil)
	defer app.cancel()
	base := strings.Repeat("def weather_icon(code):\n    return '晴天 ☀️' if code == 0 else '阴天 ☁️'\n\n", 150)
	snapshot := &chatPreviewSnapshot{chat: chatData{ID: "reasoning", IsStreaming: true, Conversations: []chatConversation{{ID: "active", Role: "assistant", Reasoning: base}}}}
	palette := defaultPalette()
	app.chatMessagesProps(snapshot, palette, 560, 500, 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		snapshot.chat.Conversations[0].Reasoning = base + fmt.Sprint(i)
		app.chatMessagesProps(snapshot, palette, 560, 500, 1)
	}
}
