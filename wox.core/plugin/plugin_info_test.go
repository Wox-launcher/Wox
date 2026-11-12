package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"wox/common"
	"wox/database"
	"wox/setting"
	"wox/util"

	"github.com/stretchr/testify/require"
)

func TestGetPluginInfoUsesLiveRuntime(t *testing.T) {
	initPluginManagerLoadTest(t)
	ctx := t.Context()
	manager := newManager()
	host := &fakeHost{started: true}
	store := setting.NewPluginSettingStore(database.GetDB(), "example")
	require.NoError(t, store.Set("TriggerKeywords", []string{"custom"}))
	instance := &Instance{
		Metadata:        Metadata{Id: "example", Name: "Example", Version: "1.2.3", Icon: "relative:images/icon.png", TriggerKeywords: []string{"default"}},
		PluginDirectory: t.TempDir(),
		Host:            host,
		Setting:         setting.NewPluginSetting(store, nil),
	}
	instance.setRuntimeTriggerKeywords([]string{"runtime"})
	instance.beginInitCycle()
	require.True(t, manager.lifecycle.appendPluginInstance(instance))
	option := GetPluginInfoOption{PluginId: " EXAMPLE "}
	require.False(t, manager.getPluginInfo(ctx, option).Ready)
	instance.finishInit(false, errors.New("failed init"))
	require.False(t, manager.getPluginInfo(ctx, option).Ready)
	instance.finishInit(true, nil)
	info := manager.getPluginInfo(ctx, option)
	require.True(t, info.Installed && info.Enabled && info.Ready)
	require.Equal(t, "Example", info.Name)
	require.Equal(t, "1.2.3", info.Version)
	require.NotNil(t, info.Icon)
	require.Equal(t, common.WoxImageTypeAbsolutePath, info.Icon.ImageType)
	require.Equal(t, filepath.Join(instance.PluginDirectory, "images", "icon.png"), filepath.Clean(info.Icon.ImageData))
	require.Equal(t, []string{"custom", "runtime"}, info.TriggerKeywords)
	info.TriggerKeywords[0] = "changed"
	require.Equal(t, []string{"custom", "runtime"}, instance.GetTriggerKeywords())
	host.started = false
	require.False(t, manager.getPluginInfo(ctx, option).Ready)
}

func TestGetPluginInfoFindsUnloadedPackage(t *testing.T) {
	initPluginManagerLoadTest(t)
	manager := newManager()
	dir := filepath.Join(util.GetLocation().GetPluginDirectory(), "example@1.2.3")
	require.NoError(t, os.Mkdir(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"Id":"example","Name":"Example","Version":"1.2.3","Runtime":"nodejs","TriggerKeywords":["example"]}`), 0o600))
	settings := setting.NewPluginSettingStore(database.GetDB(), "example")
	require.NoError(t, settings.Set("Disabled", true))
	require.NoError(t, settings.Set("TriggerKeywords", []string{"custom"}))

	info := manager.getPluginInfo(t.Context(), GetPluginInfoOption{PluginId: "example"})
	require.Empty(t, info.ErrMsg)
	require.True(t, info.Installed)
	require.False(t, info.Enabled || info.Ready)
	require.Equal(t, "1.2.3", info.Version)
	require.Equal(t, []string{"custom"}, info.TriggerKeywords)
	require.Empty(t, manager.GetPluginInstances())
	require.False(t, manager.GetLoadedPluginInfo(t.Context(), "example").Installed)

	unknown := manager.getPluginInfo(t.Context(), GetPluginInfoOption{PluginId: "unknown"})
	require.False(t, unknown.Installed)
	require.Empty(t, unknown.ErrMsg)
}

func TestQueryHandoffContextStaysWithRecipient(t *testing.T) {
	query := Query{
		Type:           QueryTypeInput,
		TriggerKeyword: "custom",
		ContextData:    common.ContextData{QueryContextDataRecipientKey: "target", "filePath": "image.png"},
	}
	require.Equal(t, common.ContextData{"filePath": "image.png"}, query.contextDataForPlugin("TARGET"))
	require.Empty(t, query.contextDataForPlugin("other"))
	require.False(t, query.HasScope())
	require.Equal(t, "target", query.ContextData[QueryContextDataRecipientKey])
	query.TriggerKeyword = ""
	require.Empty(t, query.contextDataForPlugin("target"))
}
