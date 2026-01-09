package system

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime/pprof"
	"slices"
	"time"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/ui"
	"wox/util"
	"wox/util/notifier"
	"wox/util/shell"
)

var sysIcon = common.PluginSysIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &SysPlugin{})
}

type SysPlugin struct {
	api      plugin.API
	commands []SysCommand
}

type SysCommand struct {
	ID                     string
	Title                  string
	SubTitle               string
	Icon                   common.WoxImage
	PreventHideAfterAction bool
	Action                 func(ctx context.Context, actionContext plugin.ActionContext)
}

func (r *SysPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "227f7d64-df08-4e35-ad05-98a26d540d06",
		Name:          "i18n:plugin_sys_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_sys_plugin_description",
		Icon:          sysIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureMRU,
			},
		},
	}
}

func (r *SysPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.api = initParams.API
	r.commands = []SysCommand{
		{
			ID:    "lock_computer",
			Title: "i18n:plugin_sys_lock_computer",
			Icon:  common.LockIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if util.IsMacOS() {
					shell.Run("osascript", "-e", "tell application \"System Events\" to keystroke \"q\" using {control down, command down}")
				}
				if util.IsWindows() {
					shell.Run("rundll32.exe", "user32.dll,LockWorkStation")
				}
			},
		},
		{
			ID:    "empty_trash",
			Title: "i18n:plugin_sys_empty_trash",
			Icon:  common.TrashIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if util.IsMacOS() {
					shell.Run("osascript", "-e", "tell application \"Finder\" to empty trash")
				}
				if util.IsWindows() {
					shell.Run("powershell.exe", "-Command", "Clear-RecycleBin -Force")
				}
			},
		},
		{
			ID:    "quit_wox",
			Title: "i18n:plugin_sys_quit_wox",
			Icon:  common.ExitIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				ui.GetUIManager().ExitApp(ctx)
			},
		},
		{
			ID:                     "open_wox_settings",
			Title:                  "i18n:plugin_sys_open_wox_settings",
			PreventHideAfterAction: true,
			Icon:                   common.WoxIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				plugin.GetPluginManager().GetUI().OpenSettingWindow(ctx, common.DefaultSettingWindowContext)
			},
		},
		{
			ID:    "open_system_settings",
			Title: "i18n:plugin_sys_open_system_settings",
			Icon:  common.SettingIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if util.IsMacOS() {
					shell.Run("open", "-a", "System Preferences")
				}
				if util.IsWindows() {
					shell.Run("control.exe", "desk.cpl")
				}
			},
		},
	}

	if util.IsDev() {
		r.commands = append(r.commands, SysCommand{
			Title: "test notification long",
			Icon:  common.CPUProfileIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				img, _ := common.WoxIcon.ToImage()
				notifier.Notify(img, `This is a very long notification message to test the notification system in Wox. 
				If you see this message, the notification system is working properly. 
				You can customize the duration, appearance, and behavior of notifications as needed.
				Enjoy using Wox! 
				`+time.Now().String())
			},
		})

		r.commands = append(r.commands, SysCommand{
			Title: "test notification short",
			Icon:  common.CPUProfileIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				img, _ := common.WoxIcon.ToImage()
				notifier.Notify(img, `This is a very short notification.`+time.Now().String())
			},
		})

		r.commands = append(r.commands, SysCommand{
			ID:    "cpu_profiling",
			Title: "i18n:plugin_sys_performance_cpu_profiling",
			Icon:  common.CPUProfileIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				cpuProfPath := path.Join(util.GetLocation().GetWoxDataDirectory(), "cpu.prof")
				f, err := os.Create(cpuProfPath)
				if err != nil {
					util.GetLogger().Info(ctx, "failed to create cpu profile file: "+err.Error())
					return
				}

				util.GetLogger().Info(ctx, "start cpu profile")
				profileErr := pprof.StartCPUProfile(f)
				if profileErr != nil {
					util.GetLogger().Info(ctx, "failed to start cpu profile: "+profileErr.Error())
					return
				}

				time.AfterFunc(30*time.Second, func() {
					pprof.StopCPUProfile()
					util.GetLogger().Info(ctx, "cpu profile saved to "+cpuProfPath)
				})
			},
		})

		r.commands = append(r.commands, SysCommand{
			ID:    "memory_profiling",
			Title: "i18n:plugin_sys_performance_memory_profiling",
			Icon:  common.CPUProfileIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				memoryProfPath := path.Join(util.GetLocation().GetWoxDataDirectory(), "memory.prof")
				f, err := os.Create(memoryProfPath)
				if err != nil {
					util.GetLogger().Info(ctx, "failed to create memory profile file: "+err.Error())
					return
				}

				util.GetLogger().Info(ctx, "start memory profile")
				profileErr := pprof.WriteHeapProfile(f)
				if profileErr != nil {
					util.GetLogger().Info(ctx, "failed to start memory profile: "+profileErr.Error())
					return
				}

				util.GetLogger().Info(ctx, "memory profile saved to "+memoryProfPath)
			},
		})
	}

	// Clear cache command - available in all environments
	r.commands = append(r.commands, SysCommand{
		ID:       "clear_all_cache",
		Title:    "i18n:plugin_sys_clear_all_cache",
		SubTitle: "i18n:plugin_sys_clear_all_cache_subtitle",
		Icon:     common.NewWoxImageEmoji("🗑️"),
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			location := util.GetLocation()
			cacheDirectory := location.GetCacheDirectory()

			// Remove entire cache directory
			if _, err := os.Stat(cacheDirectory); err == nil {
				removeErr := os.RemoveAll(cacheDirectory)
				if removeErr != nil {
					r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to remove cache directory: %s", removeErr.Error()))
					r.api.Notify(ctx, "i18n:plugin_sys_clear_cache_failed")
					return
				}

				// Recreate the cache directory structure
				os.MkdirAll(location.GetImageCacheDirectory(), 0755)

				r.api.Log(ctx, plugin.LogLevelInfo, "cache directory cleared successfully")
				r.api.Notify(ctx, "i18n:plugin_sys_clear_cache_success")
			}
		},
	})

	r.api.OnMRURestore(ctx, r.handleMRURestore)
}

func (r *SysPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	for _, command := range r.commands {
		translatedTitle := i18n.GetI18nManager().TranslateWox(ctx, command.Title)
		isTitleMatch, titleScore := plugin.IsStringMatchScore(ctx, translatedTitle, query.Search)
		if !isTitleMatch {
			translatedTitleEnUs := i18n.GetI18nManager().TranslateWoxEnUs(ctx, command.Title)
			isTitleMatch, titleScore = plugin.IsStringMatchScore(ctx, translatedTitleEnUs, query.Search)
		}

		if isTitleMatch {
			results = append(results, plugin.QueryResult{
				Title:    command.Title,
				SubTitle: command.SubTitle,
				Score:    titleScore,
				Icon:     command.Icon,
				Actions: []plugin.QueryResultAction{
					{
						Name:                   "i18n:plugin_sys_execute",
						Icon:                   common.ExecuteRunIcon,
						Action:                 command.Action,
						PreventHideAfterAction: command.PreventHideAfterAction,
						ContextData:            common.ContextData{"commandId": command.ID},
					},
				},
			})
		}
	}

	pluginSettingsFormat := i18n.GetI18nManager().TranslateWox(ctx, "plugin_sys_open_plugin_settings")
	for _, instance := range plugin.GetPluginManager().GetPluginInstances() {
		pluginName := instance.GetName(ctx)
		title := fmt.Sprintf(pluginSettingsFormat, pluginName)
		isNameMatch, matchScore := plugin.IsStringMatchScore(ctx, title, query.Search)
		isTriggerKeywordMatch := slices.Contains(instance.GetTriggerKeywords(), query.Search)
		if isNameMatch || isTriggerKeywordMatch {
			pluginIcon := common.SettingIcon
			iconImg, parseErr := common.ParseWoxImage(instance.Metadata.Icon)
			if parseErr == nil {
				pluginIcon = common.ConvertRelativePathToAbsolutePath(ctx, iconImg, instance.PluginDirectory)
			}

			results = append(results, plugin.QueryResult{
				Title: title,
				Score: matchScore,
				Icon:  pluginIcon,
				Actions: []plugin.QueryResultAction{
					{
						Name: "i18n:plugin_sys_execute",
						Icon: common.ExecuteRunIcon,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							plugin.GetPluginManager().GetUI().OpenSettingWindow(ctx, common.SettingWindowContext{
								Path:  "/plugin/setting",
								Param: pluginName,
							})
						},
						PreventHideAfterAction: true,
					},
				},
			})
		}
	}

	return
}

func (r *SysPlugin) handleMRURestore(ctx context.Context, mruData plugin.MRUData) (*plugin.QueryResult, error) {
	commandID := mruData.ContextData["commandId"]
	if commandID == "" {
		return nil, fmt.Errorf("empty commandId in context data")
	}

	// Find the command by ID
	var foundCommand *SysCommand
	for _, command := range r.commands {
		if command.ID == commandID {
			foundCommand = &command
			break
		}
	}

	if foundCommand == nil {
		return nil, fmt.Errorf("system command no longer exists: %s", commandID)
	}

	result := &plugin.QueryResult{
		Title:    foundCommand.Title,
		SubTitle: foundCommand.SubTitle,
		Icon:     foundCommand.Icon,
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "i18n:plugin_sys_execute",
				Action:                 foundCommand.Action,
				PreventHideAfterAction: foundCommand.PreventHideAfterAction,
				ContextData:            mruData.ContextData,
			},
		},
	}

	return result, nil
}
