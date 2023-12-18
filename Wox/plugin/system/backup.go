package system

import (
	"context"
	"fmt"
	"slices"
	"wox/plugin"
	"wox/setting"
	"wox/util"
)

var backupIcon = plugin.NewWoxImageBase64(`data:image/png;base64,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`)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &BackupPlugin{})
}

type BackupPlugin struct {
	api plugin.API
}

func (c *BackupPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "0feebaec-1a66-45af-9856-566343518638",
		Name:          "Backup and restore Wox settings",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Nodejs",
		Description:   "Backup and restore Wox settings",
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
			Title:    "Backup now",
			SubTitle: "Backup Wox settings",
			Icon:     backupIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "Backup",
					Action: func(actionContext plugin.ActionContext) {
						backupErr := setting.GetSettingManager().Backup(ctx, setting.BackupTypeManual)
						if backupErr != nil {
							c.api.ShowMsg(ctx, "Error", backupErr.Error(), "")
						} else {
							c.api.ShowMsg(ctx, "Success", "Wox settings backed up", "")
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
				Title:    "Error",
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
		backupDummy := backup
		results = append(results, plugin.QueryResult{
			Title:    fmt.Sprintf("#%d", index+1),
			SubTitle: fmt.Sprintf("%s - %s", backupDummy.Type, util.FormatTimestamp(backupDummy.Timestamp)),
			Icon:     backupIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "Restore",
					Action: func(actionContext plugin.ActionContext) {
						restoreErr := setting.GetSettingManager().Restore(ctx, backupDummy.Id)
						if restoreErr != nil {
							c.api.ShowMsg(ctx, "Error", restoreErr.Error(), "")
						} else {
							c.api.ShowMsg(ctx, "Success", "Wox settings restored", "")
						}
					},
				},
			},
		})
	}

	return results
}
