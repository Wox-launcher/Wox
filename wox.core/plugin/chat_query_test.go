package plugin

import (
	"context"
	"path/filepath"
	"testing"
	"wox/common"
	"wox/database"
	"wox/setting"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TestBuildAIChatQueryFollowsKeywordChanges exercises routing after each live
// settings edit, including when another plugin owns the old chat keyword.
func TestBuildAIChatQueryFollowsKeywordChanges(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "settings.db")), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&database.PluginSetting{}, &database.Oplog{}))
	instance := &Instance{
		Metadata: Metadata{Id: common.AIChatPluginID, TriggerKeywords: []string{"chat"}},
		Setting:  setting.NewPluginSetting(setting.NewPluginSettingStore(db, common.AIChatPluginID), nil),
	}
	other := &Instance{Metadata: Metadata{Id: "other-plugin"}}
	manager := &Manager{instances: []*Instance{instance, other}}
	ctx := context.Background()
	contextData := common.ContextData{"ai_chat_active_id": "saved-chat", "ai_chat_attachments": `[{"Kind":"quote","Text":"selected text"}]`}

	for _, test := range []struct {
		name     string
		keywords []string
		want     string
	}{
		{name: "default", want: "chat"},
		{name: "renamed", keywords: []string{"ai"}, want: "ai"},
		{name: "renamed again", keywords: []string{"ask"}, want: "ask"},
		{name: "multiple", keywords: []string{"*", "talk", "ai"}, want: "talk"},
		{name: "global only", keywords: []string{"*"}},
		{name: "reset", want: "chat"},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, instance.Setting.TriggerKeywords.SetLocal(test.keywords))
			other.Metadata.TriggerKeywords = []string{"*"}
			if test.want != "chat" {
				other.Metadata.TriggerKeywords = append(other.Metadata.TriggerKeywords, "chat")
			}
			for _, text := range []string{"", "  selected text\n中文  "} {
				query := manager.BuildAIChatQuery(ctx, text, contextData)
				require.Equal(t, QueryTypeInput, query.QueryType)
				require.Equal(t, contextData, query.ContextData)
				require.Empty(t, query.QuerySelection)
				if test.want == "" {
					require.Equal(t, text, query.QueryText)
					require.Equal(t, common.AIChatPluginID, query.QueryScope.Identity())
					scoped := Query{Type: query.QueryType, Scope: query.QueryScope}
					require.True(t, manager.canOperateQuery(ctx, instance, scoped))
					require.False(t, manager.canOperateQuery(ctx, other, scoped))
					continue
				}

				require.Equal(t, test.want+" "+text, query.QueryText)
				require.True(t, query.QueryScope.IsEmpty())
				parsed, owner := newQueryInputWithPlugins(query.QueryText, manager.GetPluginInstances())
				require.Same(t, instance, owner)
				require.True(t, manager.canOperateQuery(ctx, instance, parsed))
				require.False(t, manager.canOperateQuery(ctx, other, parsed))
			}
		})
	}
}

// TestBuildAIChatQueryWithoutPluginKeepsScope prevents an unavailable chat
// entry point from sending its text through global search.
func TestBuildAIChatQueryWithoutPluginKeepsScope(t *testing.T) {
	manager := &Manager{}
	query := manager.BuildAIChatQuery(context.Background(), "selected text", nil)
	require.Equal(t, "selected text", query.QueryText)
	require.Equal(t, common.AIChatPluginID, query.QueryScope.Identity())
	require.False(t, manager.canOperateQuery(context.Background(), &Instance{
		Metadata: Metadata{Id: "other-plugin", TriggerKeywords: []string{"*", "chat"}},
	}, Query{Type: query.QueryType, Scope: query.QueryScope}))
}
