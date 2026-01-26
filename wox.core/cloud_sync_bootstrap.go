package main

import (
	"context"
	"os"
	"strings"
	"wox/cloudsync"
	"wox/cloudsync/settingadapter"
	"wox/updater"
	"wox/util"
)

const (
	cloudSyncBaseURLEnv = "WOX_CLOUD_SYNC_URL"
	cloudSyncTokenEnv   = "WOX_CLOUD_SYNC_TOKEN"
)

func initCloudSync(ctx context.Context) {
	baseURL := strings.TrimSpace(os.Getenv(cloudSyncBaseURLEnv))
	if baseURL == "" {
		return
	}

	token := strings.TrimSpace(os.Getenv(cloudSyncTokenEnv))
	authProvider := cloudsync.StaticAuthProvider{Token: token}
	deviceProvider := cloudsync.NewFileDeviceProvider("")

	client, err := cloudsync.NewCloudSyncHTTPClient(cloudsync.CloudSyncHTTPClientConfig{
		BaseURL:        baseURL,
		AuthProvider:   authProvider,
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
	})

	service := &cloudsync.Service{
		Manager:        manager,
		Client:         client,
		KeyManager:     keyManager,
		DeviceProvider: deviceProvider,
	}
	cloudsync.SetService(service)

	if keyManager.GetStatus(ctx).Available {
		manager.Start(ctx)
	}
}
