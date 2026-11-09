package ui

import (
	"testing"

	"wox/cloudsync"
	"wox/database"
	"wox/plugin"
	"wox/setting"
	"wox/ui/contract"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRefinementDefaultsRestoreOnlyOnCommandEntry(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&database.PluginSetting{}, &database.Oplog{}))

	const pluginID = "test-plugin"
	store := setting.NewPluginSettingStore(db, pluginID)
	owner := &plugin.Instance{Metadata: plugin.Metadata{Id: pluginID}, Setting: setting.NewPluginSetting(store, nil)}
	require.NoError(t, owner.Setting.SetRefinementDefault("browse", map[string]string{"country": "CN"}))

	query := plugin.Query{Command: "browse"}
	state := prepareRefinementDefaults(contract.RefinementScope{}, &query, owner)
	require.Equal(t, map[string]string{"country": "CN"}, state.SavedValues)
	require.True(t, state.Restored)
	require.Equal(t, map[string]string{"country": "CN"}, query.Refinements)

	query.Refinements["country"] = "JP"
	require.Equal(t, "CN", state.SavedValues["country"])
	require.False(t, prepareRefinementDefaults(state.Scope, &query, owner).Restored)
	require.Equal(t, "JP", query.Refinements["country"])

	explicit := plugin.Query{Command: "browse", Refinements: map[string]string{"country": "DE"}}
	explicitState := prepareRefinementDefaults(contract.RefinementScope{}, &explicit, owner)
	require.False(t, explicitState.Restored)
	require.Equal(t, state.SavedValues, explicitState.SavedValues)
	require.Equal(t, "DE", explicit.Refinements["country"])

	query.Command = "other"
	next := prepareRefinementDefaults(state.Scope, &query, owner)
	require.Nil(t, next.SavedValues)
	require.Empty(t, query.Refinements)
	require.NotNil(t, next.AppliedValues)

	require.NoError(t, store.DeleteAll())
	require.Nil(t, prepareRefinementDefaults(contract.RefinementScope{}, &plugin.Query{Command: "browse"}, owner).SavedValues)
	var oplogs []database.Oplog
	require.NoError(t, db.Where("entity_id = ?", pluginID).Order("id").Find(&oplogs).Error)
	require.Len(t, oplogs, 2)
	require.Equal(t, oplogs[0].Key, oplogs[1].Key)
	require.Equal(t, cloudsync.OpUpsert, oplogs[0].Operation)
	require.Equal(t, cloudsync.OpDelete, oplogs[1].Operation)
}
