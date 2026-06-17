package main

import (
	"context"
	"strings"
	"wox/account"
	"wox/cloudsync"
	"wox/cloudsync/settingadapter"
	"wox/setting"
	"wox/ui"
	"wox/updater"
	"wox/util"
)

const defaultCloudSyncBaseURL = "https://sync.woxlauncher.com"

func initCloudSync(ctx context.Context) {
	baseURL := resolveCloudSyncBaseURL(ctx)
	accountService := account.NewService(baseURL)
	account.SetService(accountService)
	accountService.StartTokenRefresh(ctx)
	deviceProvider := cloudsync.NewFileDeviceProvider("")

	client, err := cloudsync.NewCloudSyncHTTPClient(cloudsync.CloudSyncHTTPClientConfig{
		BaseURL:        baseURL,
		AuthProvider:   accountService,
		DeviceProvider: deviceProvider,
		AppVersion:     updater.CURRENT_VERSION,
		Platform:       util.GetCurrentPlatform(),
	})
	if err != nil {
		util.GetLogger().Error(ctx, "cloud sync init failed: "+err.Error())
		return
	}

	keyManager := cloudsync.NewKeyManager(cloudsync.KeyManagerConfig{
		KeyClient:      client,
		DeviceProvider: deviceProvider,
	})

	manager := cloudsync.NewCloudSyncManager(cloudsync.DefaultCloudSyncConfig(), cloudsync.CloudSyncDependencies{
		Client:            client,
		Crypto:            cloudsync.NewAesGcmCrypto(keyManager),
		DeviceProvider:    deviceProvider,
		Applier:           settingadapter.NewLocalSettingApplier(),
		OplogStore:        cloudsync.NewDefaultOplogStore(),
		Notifier:          settingadapter.NewCloudSyncOplogNotifier(),
		ExclusionProvider: settingadapter.NewCloudSyncPluginExclusionProvider(),
		SettingReloader:   ui.GetUIManager().GetUI(ctx),
	})

	service := &cloudsync.Service{
		Manager:        manager,
		Client:         client,
		KeyManager:     keyManager,
		DeviceProvider: deviceProvider,
	}
	cloudsync.SetService(service)
}

// startCloudSyncManagerIfReady starts background sync after plugins and UI are
// ready enough to apply install-list records safely.
func startCloudSyncManagerIfReady(ctx context.Context) {
	service := cloudsync.GetService()
	accountService := account.GetService()
	if service == nil || service.Manager == nil || service.KeyManager == nil || accountService == nil {
		return
	}

	accountStatus := accountService.Status(ctx)
	if accountStatus.LoggedIn && accountStatus.SyncEligible && accountStatus.SyncEnabled && service.KeyManager.GetStatus(ctx).Available {
		service.StartManager(ctx)
	}
}

// resolveCloudSyncBaseURL applies the local development override while keeping
// the official sync endpoint as the normal production default.
func resolveCloudSyncBaseURL(ctx context.Context) string {
	settingManager := setting.GetSettingManager()
	if settingManager == nil {
		return defaultCloudSyncBaseURL
	}

	configuredURL := strings.TrimSpace(settingManager.GetWoxSetting(ctx).CloudSyncServerUrl.Get())
	if configuredURL == "" {
		return defaultCloudSyncBaseURL
	}

	return strings.TrimRight(configuredURL, "/")
}
