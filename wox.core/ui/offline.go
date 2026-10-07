package ui

import (
	"context"
	"errors"
	"sync"
	"wox/ai"
	"wox/network"
	"wox/plugin"
	"wox/privacy"
	"wox/setting"
	"wox/util"
)

var offlineTransition sync.Mutex

// setOfflineMode persists the preference before changing runtime policy in either direction.
func setOfflineMode(ctx context.Context, settings *setting.WoxSetting, enabled bool) error {
	offlineTransition.Lock()
	defer offlineTransition.Unlock()
	previous := settings.EnableOfflineMode.Get()
	if err := settings.EnableOfflineMode.SetLocal(enabled); err != nil {
		return err
	}
	if err := privacy.RefreshPreservedSettings(settings); err != nil {
		// Restore both persisted copies while leaving the running policy untouched.
		restoreErr := settings.EnableOfflineMode.SetLocal(previous)
		var profileErr error
		if restoreErr == nil {
			profileErr = privacy.RefreshPreservedSettings(settings)
		}
		return errors.Join(err, restoreErr, profileErr)
	}
	manager := plugin.GetPluginManager()
	network.Default.SetOffline(enabled)
	network.Notify()
	ai.ResetMCPClients()
	if enabled {
		manager.SuspendOfflinePlugins(ctx)
	} else if err := manager.ResumeOfflinePlugins(ctx); err != nil {
		// Plugin failures are isolated just as at startup; they must not lock the user offline.
		util.GetLogger().Error(ctx, "failed to restore some plugins after leaving offline mode: "+err.Error())
	}
	return nil
}
