package settingadapter

import (
	"context"
	"wox/setting"
)

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
