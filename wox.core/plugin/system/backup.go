package system

import (
	"context"
	"fmt"
	"slices"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting"
	"wox/util"
)

var backupIcon = common.PluginBackupIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &BackupPlugin{})
}

type BackupPlugin struct {
	api plugin.API
}

func (c *BackupPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "0feebaec-1a66-45af-9856-566343518638",
		Name:          "i18n:plugin_backup_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_backup_plugin_description",
		Icon:          backupIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"restore",
			"backup",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (c *BackupPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API
}

func (c *BackupPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	if query.TriggerKeyword == "restore" {
		return c.restore(ctx, query)
	} else {
		return c.backup(ctx, query)
	}
}

func (c *BackupPlugin) backup(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	return []plugin.QueryResult{
		{
			Title:    "i18n:plugin_backup_now",
			SubTitle: "i18n:plugin_backup_subtitle",
			Icon:     backupIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_backup_action",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						backupErr := setting.GetSettingManager().Backup(ctx, setting.BackupTypeManual)
						if backupErr != nil {
							c.api.Notify(ctx, backupErr.Error())
						} else {
							c.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_backup_success"))
						}
					},
				},
			},
		},
	}
}

func (c *BackupPlugin) restore(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	backups, err := setting.GetSettingManager().FindAllBackups(ctx)
	if err != nil {
		return []plugin.QueryResult{
			{
				Title:    "i18n:plugin_backup_error",
				SubTitle: err.Error(),
				Icon:     backupIcon,
			},
		}
	}

	//sort backups by timestamp desc
	slices.SortFunc(backups, func(i, j setting.Backup) int {
		return int(j.Timestamp - i.Timestamp)
	})

	var results []plugin.QueryResult
	for index, backup := range backups {
		results = append(results, plugin.QueryResult{
			Title:    fmt.Sprintf("#%d", index+1),
			SubTitle: fmt.Sprintf("%s - %s", backup.Type, util.FormatTimestamp(backup.Timestamp)),
			Icon:     backupIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_backup_restore",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						restoreErr := setting.GetSettingManager().Restore(ctx, backup.Id)
						if restoreErr != nil {
							c.api.Notify(ctx, restoreErr.Error())
						} else {
							c.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_backup_restore_success"))
						}
					},
				},
			},
		})
	}

	return results
}
