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
	historyStore := cloudsync.NewDefaultCloudSyncHistoryStore()

	manager := cloudsync.NewCloudSyncManager(cloudsync.DefaultCloudSyncConfig(), cloudsync.CloudSyncDependencies{
		Client:            client,
		Crypto:            cloudsync.NewAesGcmCrypto(keyManager),
		DeviceProvider:    deviceProvider,
		Applier:           settingadapter.NewLocalSettingApplier(),
		OplogStore:        cloudsync.NewDefaultOplogStore(),
		Snapshotter:       settingadapter.NewLocalSnapshotter(),
		Notifier:          settingadapter.NewCloudSyncOplogNotifier(),
		ProgressNotifier:  cloudSyncUIProgressNotifier{},
		ExclusionProvider: settingadapter.NewCloudSyncPluginExclusionProvider(),
		SettingReloader:   cloudSyncUISettingReloader{},
		HistoryStore:      historyStore,
	})

	service := &cloudsync.Service{
		Manager:        manager,
		Client:         client,
		KeyManager:     keyManager,
		DeviceProvider: deviceProvider,
		HistoryStore:   historyStore,
	}
	cloudsync.SetService(service)
}

type cloudSyncUIProgressNotifier struct{}

// CloudSyncProgressChanged forwards transient sync progress over the existing UI websocket channel.
func (cloudSyncUIProgressNotifier) CloudSyncProgressChanged(ctx context.Context, progress cloudsync.CloudSyncProgress) {
	ui.GetUIManager().GetUI(ctx).CloudSyncProgressChanged(ctx, progress)
}

type cloudSyncUISettingReloader struct{}

func (cloudSyncUISettingReloader) ReloadSetting(ctx context.Context) {
	ui.GetUIManager().GetUI(ctx).ReloadSetting(ctx)
}

func (cloudSyncUISettingReloader) ReloadSettingPlugins(ctx context.Context) {
	ui.GetUIManager().GetUI(ctx).ReloadSettingPlugins(ctx)
}

func (cloudSyncUISettingReloader) ReloadSettingThemes(ctx context.Context) {
	ui.GetUIManager().GetUI(ctx).ReloadSettingThemes(ctx)
}

func (cloudSyncUISettingReloader) ApplyCurrentTheme(ctx context.Context) {
	ui.GetUIManager().ApplyCurrentTheme(ctx)
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
