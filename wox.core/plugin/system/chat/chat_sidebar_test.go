package chat

import (
	"context"
	"encoding/json"
	"testing"

	"wox/common"
)

func TestChatSidebarPreferencePersistsInPreview(t *testing.T) {
	ctx := context.Background()
	chat := newChatStorageTestPlugin(t)
	if chat.chatSidebarOpen(ctx) {
		t.Fatal("sidebar should default to closed")
	}
	for _, open := range []bool{true, false} {
		if err := chat.SetChatSidebarOpen(ctx, open); err != nil {
			t.Fatal(err)
		}
		restarted := &AIChatPlugin{store: chat.store, api: chat.api}
		var preview common.AIChatPreviewData
		result := restarted.getChatPreviewData(ctx, "", nil)
		if err := json.Unmarshal([]byte(result.Preview.PreviewData), &preview); err != nil {
			t.Fatal(err)
		}
		if preview.SidebarOpen != open {
			t.Fatalf("restarted sidebar = %v, want %v", preview.SidebarOpen, open)
		}
	}
}
