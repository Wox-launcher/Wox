package system

import (
	"context"
	"testing"
	"wox/common"
	"wox/database"
	"wox/setting"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newChatStorageTestPlugin(t *testing.T) *AIChatPlugin {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&database.PluginSetting{}, &database.Oplog{}); err != nil {
		t.Fatal(err)
	}
	return &AIChatPlugin{api: &chatTestAPI{}, store: setting.NewPluginSettingStore(db, common.AIChatPluginID)}
}

// TestChatStorageKeepsSummariesInMemoryAndBodiesOnDisk covers the per-chat key layout.
func TestChatStorageKeepsSummariesInMemoryAndBodiesOnDisk(t *testing.T) {
	ctx := context.Background()
	chatPlugin := newChatStorageTestPlugin(t)
	older := common.AIChatData{Id: "older", Title: "Older", UpdatedAt: 10, Model: common.Model{Name: "m1"}, Conversations: []common.Conversation{{Id: "c1", Role: common.ConversationRoleUser, Text: "hello"}}}
	newer := common.AIChatData{Id: "newer", Title: "Newer", UpdatedAt: 20, Conversations: []common.Conversation{{Id: "c2", Role: common.ConversationRoleUser, Text: "hi"}}, IsStreaming: true}
	chatPlugin.saveChat(ctx, older)
	chatPlugin.saveChat(ctx, newer)

	summaries := chatPlugin.chatSummaries()
	if len(summaries) != 2 || summaries[0].Id != "newer" || summaries[1].Id != "older" {
		t.Fatalf("summaries = %+v, want newest first", summaries)
	}
	for _, summary := range summaries {
		if !summary.IsSummary || summary.Conversations != nil {
			t.Fatalf("summary index must not hold conversations: %+v", summary)
		}
	}

	loaded, ok := chatPlugin.GetChat(ctx, "older")
	if !ok || len(loaded.Conversations) != 1 || loaded.Conversations[0].Text != "hello" || loaded.Model.Name != "m1" {
		t.Fatalf("GetChat = %+v ok=%v, want full body from store", loaded, ok)
	}
	if loaded.IsSummary || loaded.IsStreaming {
		t.Fatalf("loaded chat carries transient flags: %+v", loaded)
	}

	// A fresh plugin instance rebuilds the index from the store without the bodies.
	reloaded := &AIChatPlugin{api: &chatTestAPI{}, store: chatPlugin.store}
	index, err := reloaded.loadChatSummaries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(index) != 2 || index[0].Id != "newer" || index[0].Conversations != nil {
		t.Fatalf("reloaded index = %+v", index)
	}

	if !chatPlugin.DeleteChat(ctx, "older") {
		t.Fatal("DeleteChat must report an existing chat")
	}
	if _, ok := chatPlugin.GetChat(ctx, "older"); ok {
		t.Fatal("deleted chat still loads from store")
	}
	if len(chatPlugin.chatSummaries()) != 1 {
		t.Fatal("deleted chat still in summary index")
	}
	if chatPlugin.DeleteChat(ctx, "older") {
		t.Fatal("deleting twice must report false")
	}

	// While a stream is running, GetChat serves its live state rather than the last save.
	chatPlugin.activeChatCancels.Store("newer", &activeAIChatCancel{cancel: func() {}, live: func() common.AIChatData {
		live := newer
		live.Conversations = append(append([]common.Conversation(nil), newer.Conversations...), common.Conversation{Id: "c3", Role: common.ConversationRoleAssistant, Text: "streaming"})
		return live
	}})
	if live, ok := chatPlugin.GetChat(ctx, "newer"); !ok || !live.IsStreaming || len(live.Conversations) != 2 || live.Conversations[1].Text != "streaming" {
		t.Fatalf("GetChat during stream = %+v ok=%v, want live conversations", live, ok)
	}
	chatPlugin.activeChatCancels.Delete("newer")
	if stored, ok := chatPlugin.GetChat(ctx, "newer"); !ok || stored.IsStreaming || len(stored.Conversations) != 1 {
		t.Fatalf("GetChat after stream = %+v ok=%v, want persisted copy", stored, ok)
	}

	// A synced upsert carries its payload; a synced delete carries an empty value. Neither
	// path re-reads the store, so a read failure can never masquerade as a deletion.
	chatPlugin.handleChatSettingChanged(ctx, "remote", `{"Id":"remote","Title":"Remote","UpdatedAt":99}`)
	if summaries := chatPlugin.chatSummaries(); len(summaries) != 2 || summaries[0].Id != "remote" || !summaries[0].IsSummary {
		t.Fatalf("remote upsert not indexed: %+v", summaries)
	}
	chatPlugin.handleChatSettingChanged(ctx, "remote", "not json")
	if len(chatPlugin.chatSummaries()) != 2 {
		t.Fatal("invalid synced payload must leave the index untouched")
	}
	chatPlugin.handleChatSettingChanged(ctx, "remote", "")
	if len(chatPlugin.chatSummaries()) != 1 {
		t.Fatal("remote delete must drop the summary")
	}

	keys, err := chatPlugin.store.ListByPrefix(aiChatSettingKeyPrefix)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("stored keys = %v, want only chat:newer", keys)
	}
	if _, ok := keys[chatSettingKey("newer")]; !ok {
		t.Fatalf("stored keys = %v, want chat:newer", keys)
	}
}
