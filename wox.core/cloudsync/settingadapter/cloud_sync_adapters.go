package settingadapter

import (
	"context"
	"wox/cloudsync"
	"wox/setting"
)

type CloudSyncOplogNotifier struct{}

func NewCloudSyncOplogNotifier() *CloudSyncOplogNotifier {
	return &CloudSyncOplogNotifier{}
}

func (n *CloudSyncOplogNotifier) Changes() <-chan struct{} {
	return cloudsync.SubscribeOplogChanges()
}

type CloudSyncPluginExclusionProvider struct{}

func NewCloudSyncPluginExclusionProvider() *CloudSyncPluginExclusionProvider {
	return &CloudSyncPluginExclusionProvider{}
}

func (p *CloudSyncPluginExclusionProvider) DisabledPluginIDs(ctx context.Context) []string {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if woxSetting == nil {
		return nil
	}
	return woxSetting.CloudSyncDisabledPlugins.Get()
}
