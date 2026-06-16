package sys

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime/pprof"
	"slices"
	"strconv"
	"strings"
	"time"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/ui"
	"wox/updater"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/notifier"

	"github.com/google/uuid"
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
	Aliases                []string
	SupportedOS            []string
	IsAvailable            func() bool
	BuildContextData       func(query plugin.Query) common.ContextData
	BuildTitle             func(ctx context.Context, query plugin.Query) string
	BuildSubTitle          func(ctx context.Context, query plugin.Query) string
	ActionName             string
	ActionIcon             common.WoxImage
	PreventHideAfterAction bool
	Action                 func(ctx context.Context, actionContext plugin.ActionContext)
}

const (
	sysCommandIDContextKey        = "commandId"
	sysCommandConfirmedContextKey = "confirmed"
	sysCommandVolumeContextKey    = "volume"
)

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
		Commands: r.getMetadataCommands(),
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

func (r *SysPlugin) getMetadataCommands() []plugin.MetadataCommand {
	commands := r.buildCommands()
	metadataCommands := make([]plugin.MetadataCommand, 0, len(commands))
	for _, command := range commands {
		if command.ID == "" || !r.isCommandAvailable(command) {
			continue
		}
		metadataCommands = append(metadataCommands, plugin.MetadataCommand{
			Command:     command.ID,
			Description: common.I18nString(command.Title),
		})
	}
	return metadataCommands
}

func (r *SysPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.api = initParams.API
	r.commands = r.buildCommands()

	if util.IsDev() {
		r.commands = append(r.commands, r.buildDevCommands()...)
	}

	r.api.OnMRURestore(ctx, r.handleMRURestore)
}

func (r *SysPlugin) buildCommands() []SysCommand {
	return []SysCommand{
		{
			ID:          "lock_computer",
			Title:       "i18n:plugin_sys_lock_computer",
			Icon:        common.LockIcon,
			Aliases:     []string{"lock screen", "lock computer", "锁屏", "锁定"},
			SupportedOS: []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux},
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.runSystemAction(ctx, "lock_computer", runLockCommand)
			},
		},
		{
			ID:          "empty_trash",
			Title:       "i18n:plugin_sys_empty_trash",
			Icon:        common.TrashIcon,
			Aliases:     []string{"empty recycle bin", "trash", "recycle bin", "清空回收站", "清空废纸篓"},
			SupportedOS: []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux},
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.runSystemAction(ctx, "empty_trash", runEmptyTrashCommand)
			},
		},
		{
			ID:      "quit_wox",
			Title:   "i18n:plugin_sys_quit_wox",
			Icon:    common.ExitIcon,
			Aliases: []string{"exit wox", "quit", "退出", "退出 wox"},
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				ui.GetUIManager().ExitApp(ctx)
			},
		},
		{
			ID:                     "shutdown_computer",
			Title:                  "i18n:plugin_sys_shutdown_computer",
			Icon:                   common.ExitIcon,
			Aliases:                []string{"shutdown", "shut down", "power off", "关机", "关闭电脑"},
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.handleConfirmedSystemCommand(ctx, actionContext, "shutdown_computer")
			},
		},
		{
			ID:                     "restart_computer",
			Title:                  "i18n:plugin_sys_restart_computer",
			Icon:                   common.UpdateIcon,
			Aliases:                []string{"restart", "reboot", "重启", "重启电脑"},
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.handleConfirmedSystemCommand(ctx, actionContext, "restart_computer")
			},
		},
		{
			ID:                     "open_wox_settings",
			Title:                  "i18n:plugin_sys_open_wox_settings",
			PreventHideAfterAction: true,
			Icon:                   common.WoxIcon,
			Aliases:                []string{"settings", "wox settings", "打开设置", "wox 设置"},
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				plugin.GetPluginManager().GetUI().OpenSettingWindow(ctx, common.DefaultSettingWindowContext)
			},
		},
		{
			ID:         "copy_wox_version",
			Title:      "i18n:plugin_sys_copy_wox_version",
			SubTitle:   updater.CURRENT_VERSION,
			Icon:       common.CopyIcon,
			Aliases:    []string{"version", "copy version", "复制版本", "wox version"},
			ActionName: "i18n:plugin_sys_copy",
			ActionIcon: common.CopyIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if err := clipboard.WriteText(updater.CURRENT_VERSION); err != nil {
					r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to copy Wox version: %s", err.Error()))
				}
			},
		},
		{
			ID:          "open_system_settings",
			Title:       "i18n:plugin_sys_open_system_settings",
			Icon:        common.SettingIcon,
			Aliases:     []string{"system settings", "settings app", "control panel", "打开系统设置", "系统设置"},
			SupportedOS: []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux},
			IsAvailable: isOpenSystemSettingsCommandAvailable,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.runSystemAction(ctx, "open_system_settings", runOpenSystemSettingsCommand)
			},
		},
		{
			ID:               "set-volume",
			Title:            "i18n:plugin_sys_set_volume",
			SubTitle:         "i18n:plugin_sys_set_volume_subtitle",
			Icon:             sysVolumeIcon,
			Aliases:          []string{"set volume", "volume", "音量", "设置音量"},
			SupportedOS:      []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux},
			IsAvailable:      isVolumeCommandAvailable,
			BuildContextData: buildSetVolumeContextData,
			BuildTitle:       buildSetVolumeTitle,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				percent, ok := parseVolumeContext(actionContext.ContextData)
				if !ok {
					r.api.Notify(ctx, "i18n:plugin_sys_set_volume_invalid")
					return
				}
				r.runSystemAction(ctx, "set-volume", func() (*exec.Cmd, error) {
					return runSetVolumeCommand(percent)
				})
			},
		},
		r.fixedVolumeCommand(0),
		r.fixedVolumeCommand(25),
		r.fixedVolumeCommand(50),
		r.fixedVolumeCommand(75),
		r.fixedVolumeCommand(100),
		{
			ID:          "volume-up",
			Title:       "i18n:plugin_sys_volume_up",
			Icon:        sysVolumeUpIcon,
			Aliases:     []string{"turn volume up", "volume up", "increase volume", "音量加", "调高音量"},
			SupportedOS: []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux},
			IsAvailable: isVolumeCommandAvailable,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.runSystemAction(ctx, "volume-up", runVolumeUpCommand)
			},
		},
		{
			ID:          "volume-down",
			Title:       "i18n:plugin_sys_volume_down",
			Icon:        sysVolumeDownIcon,
			Aliases:     []string{"turn volume down", "volume down", "decrease volume", "音量减", "调低音量"},
			SupportedOS: []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux},
			IsAvailable: isVolumeCommandAvailable,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.runSystemAction(ctx, "volume-down", runVolumeDownCommand)
			},
		},
		{
			ID:          "toggle-mute",
			Title:       "i18n:plugin_sys_toggle_mute",
			Icon:        sysMuteIcon,
			Aliases:     []string{"mute", "toggle mute", "unmute", "静音", "切换静音"},
			SupportedOS: []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux},
			IsAvailable: isVolumeCommandAvailable,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.runSystemAction(ctx, "toggle-mute", runToggleMuteCommand)
			},
		},
		{
			ID:                     "sleep",
			Title:                  "i18n:plugin_sys_sleep",
			Icon:                   sysSleepIcon,
			Aliases:                []string{"sleep computer", "suspend", "睡眠", "电脑睡眠"},
			SupportedOS:            []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux},
			IsAvailable:            isSleepCommandAvailable,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.handleConfirmedSystemCommand(ctx, actionContext, "sleep")
			},
		},
		{
			ID:          "sleep-displays",
			Title:       "i18n:plugin_sys_sleep_displays",
			Icon:        sysDisplaySleepIcon,
			Aliases:     []string{"sleep displays", "turn off display", "monitor off", "关闭显示器", "显示器睡眠"},
			SupportedOS: []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux},
			IsAvailable: isSleepDisplaysCommandAvailable,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.runSystemAction(ctx, "sleep-displays", runSleepDisplaysCommand)
			},
		},
		{
			ID:                     "log-out",
			Title:                  "i18n:plugin_sys_log_out",
			Icon:                   sysLogoutIcon,
			Aliases:                []string{"logout", "sign out", "log out", "注销", "登出"},
			SupportedOS:            []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux},
			IsAvailable:            isLogoutCommandAvailable,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.handleConfirmedSystemCommand(ctx, actionContext, "log-out")
			},
		},
		{
			ID:          "eject-all-disks",
			Title:       "i18n:plugin_sys_eject_all_disks",
			Icon:        sysEjectIcon,
			Aliases:     []string{"eject disks", "eject all", "弹出磁盘", "弹出所有磁盘"},
			SupportedOS: []string{util.PlatformWindows, util.PlatformMacOS},
			IsAvailable: isEjectAllDisksCommandAvailable,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.runSystemAction(ctx, "eject-all-disks", runEjectAllDisksCommand)
			},
		},
		{
			ID:          "show-desktop",
			Title:       "i18n:plugin_sys_show_desktop",
			Icon:        sysDesktopIcon,
			Aliases:     []string{"desktop", "show desktop", "显示桌面"},
			SupportedOS: []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux},
			IsAvailable: isShowDesktopCommandAvailable,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.runSystemAction(ctx, "show-desktop", runShowDesktopCommand)
			},
		},
		{
			ID:          "show-task-view",
			Title:       "i18n:plugin_sys_show_task_view",
			Icon:        sysDesktopIcon,
			Aliases:     []string{"task view", "window switcher", "virtual desktop", "virtual desktops", "desktops", "任务视图", "虚拟桌面", "窗口选择", "窗口切换"},
			SupportedOS: []string{util.PlatformWindows},
			IsAvailable: isShowTaskViewCommandAvailable,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.runSystemAction(ctx, "show-task-view", runShowTaskViewCommand)
			},
		},
		{
			ID:          "show-screen-saver",
			Title:       "i18n:plugin_sys_show_screen_saver",
			Icon:        sysScreenSaverIcon,
			Aliases:     []string{"screen saver", "screensaver", "显示屏保", "屏幕保护"},
			SupportedOS: []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux},
			IsAvailable: isShowScreenSaverCommandAvailable,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.runSystemAction(ctx, "show-screen-saver", runShowScreenSaverCommand)
			},
		},
		{
			ID:          "quit-all-applications",
			Title:       "i18n:plugin_sys_quit_all_applications",
			Icon:        sysQuitAppsIcon,
			Aliases:     []string{"quit all apps", "close all apps", "退出所有应用", "关闭所有应用"},
			SupportedOS: []string{util.PlatformWindows, util.PlatformMacOS},
			IsAvailable: isQuitAllApplicationsCommandAvailable,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.runSystemAction(ctx, "quit-all-applications", runQuitAllApplicationsCommand)
			},
		},
		{
			ID:          "hide-all-apps-except-frontmost",
			Title:       "i18n:plugin_sys_hide_all_apps_except_frontmost",
			Icon:        sysHideAppsIcon,
			Aliases:     []string{"hide all apps", "hide others", "隐藏其他应用", "只显示当前应用"},
			SupportedOS: []string{util.PlatformMacOS},
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.runSystemAction(ctx, "hide-all-apps-except-frontmost", runHideAllAppsExceptFrontmostCommand)
			},
		},
		{
			ID:          "unhide-all-hidden-apps",
			Title:       "i18n:plugin_sys_unhide_all_hidden_apps",
			Icon:        sysUnhideAppsIcon,
			Aliases:     []string{"unhide all apps", "show hidden apps", "取消隐藏应用", "显示隐藏应用"},
			SupportedOS: []string{util.PlatformMacOS},
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.runSystemAction(ctx, "unhide-all-hidden-apps", runUnhideAllHiddenAppsCommand)
			},
		},
		{
			ID:          "toggle-system-appearance",
			Title:       "i18n:plugin_sys_toggle_system_appearance",
			Icon:        sysAppearanceIcon,
			Aliases:     []string{"toggle appearance", "dark mode", "light mode", "切换外观", "深色模式", "浅色模式"},
			SupportedOS: []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux},
			IsAvailable: isToggleSystemAppearanceCommandAvailable,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.runSystemAction(ctx, "toggle-system-appearance", runToggleSystemAppearanceCommand)
			},
		},
		{
			ID:          "toggle-hidden-files",
			Title:       "i18n:plugin_sys_toggle_hidden_files",
			Icon:        sysHiddenFilesIcon,
			Aliases:     []string{"hidden files", "show hidden files", "toggle hidden files", "隐藏文件", "显示隐藏文件"},
			SupportedOS: []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux},
			IsAvailable: isToggleHiddenFilesCommandAvailable,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.runSystemAction(ctx, "toggle-hidden-files", runToggleHiddenFilesCommand)
			},
		},
		{
			ID:       "clear_all_cache",
			Title:    "i18n:plugin_sys_clear_all_cache",
			SubTitle: "i18n:plugin_sys_clear_all_cache_subtitle",
			Icon:     common.NewWoxImageEmoji("🗑️"),
			Aliases:  []string{"clear cache", "cache", "清理缓存", "清除缓存"},
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				location := util.GetLocation()
				cacheDirectory := location.GetCacheDirectory()

				// Remove the whole cache tree so stale generated assets cannot survive partial cleanup.
				if _, err := os.Stat(cacheDirectory); err == nil {
					removeErr := os.RemoveAll(cacheDirectory)
					if removeErr != nil {
						r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to remove cache directory: %s", removeErr.Error()))
						r.api.Notify(ctx, "i18n:plugin_sys_clear_cache_failed")
						return
					}

					// Recreate the cache directory structure expected by image conversion.
					os.MkdirAll(location.GetImageCacheDirectory(), 0755)
					common.ClearConvertIconPathExistenceCache()

					r.api.Log(ctx, plugin.LogLevelInfo, "cache directory cleared successfully")
					r.api.Notify(ctx, "i18n:plugin_sys_clear_cache_success")
				}
			},
		},
	}
}

func (r *SysPlugin) buildDevCommands() []SysCommand {
	return []SysCommand{
		{
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
		},

		{
			Title: "test notification short",
			Icon:  common.CPUProfileIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				img, _ := common.WoxIcon.ToImage()
				notifier.Notify(img, `This is a very short notification.`+time.Now().String())
			},
		},

		{
			Title: "test attention",
			Icon:  sysAttentionIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				now := time.Now().Format(time.RFC3339)
				r.api.PushAttention(ctx, plugin.PushAttentionRequest{
					Key:         "sys_test_attention",
					Title:       "Test attention " + now,
					Description: "This is a persistent attention item pushed from the system test command.",
					Icon:        &sysAttentionIcon,
					Action: &plugin.AttentionAction{
						Type:  plugin.AttentionActionTypeChangeQuery,
						Query: "attention ",
					},
				})
			},
		},

		{
			Title:                  "test toolbar msg",
			Icon:                   common.CPUProfileIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				var progress int = 0

				util.Go(ctx, "test toolbar msg", func() {
					toolbarMsgId := uuid.New().String()
					for progress <= 100 {
						time.Sleep(500 * time.Millisecond)
						r.api.ShowToolbarMsg(ctx, plugin.ToolbarMsg{
							Id:       toolbarMsgId,
							Title:    fmt.Sprintf("Progress: %d%%", progress),
							Icon:     sysIcon,
							Progress: &progress,
							Actions: []plugin.ToolbarMsgAction{
								{
									Name:                   "Action1",
									Icon:                   common.ExecuteRunIcon,
									Hotkey:                 util.PrimaryHotkey("1"),
									PreventHideAfterAction: true,
									Action: func(ctx context.Context, actionContext plugin.ToolbarMsgActionContext) {
										r.api.Notify(ctx, "Action 1 executed")
									},
								},
								{
									Name:                   "Stop and Clear",
									Icon:                   common.ExecuteRunIcon,
									Hotkey:                 util.PrimaryHotkey("enter"),
									PreventHideAfterAction: true,
									Action: func(ctx context.Context, actionContext plugin.ToolbarMsgActionContext) {
										progress = 200
										r.api.ClearToolbarMsg(ctx, toolbarMsgId)
									},
								},
							},
						})
						progress += 10
					}

					r.api.ClearToolbarMsg(ctx, toolbarMsgId)
				})
			},
		},

		{
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
		},

		{
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
		},
	}
}

func (r *SysPlugin) fixedVolumeCommand(percent int) SysCommand {
	return SysCommand{
		ID:          fmt.Sprintf("set-volume-%d", percent),
		Title:       fmt.Sprintf("i18n:plugin_sys_set_volume_%d", percent),
		Icon:        sysVolumeIcon,
		Aliases:     []string{fmt.Sprintf("set volume %d", percent), fmt.Sprintf("volume %d", percent), fmt.Sprintf("音量 %d", percent), fmt.Sprintf("设置音量 %d", percent)},
		SupportedOS: []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux},
		IsAvailable: isVolumeCommandAvailable,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			r.runSystemAction(ctx, fmt.Sprintf("set-volume-%d", percent), func() (*exec.Cmd, error) {
				return runSetVolumeCommand(percent)
			})
		},
	}
}

func (r *SysPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if query.Command != "" {
		if command, ok := r.findCommand(query.Command); ok {
			return plugin.NewQueryResponse([]plugin.QueryResult{r.buildCommandResult(ctx, query, command, 1000)})
		}
		return plugin.QueryResponse{}
	}

	var results []plugin.QueryResult
	for _, command := range r.commands {
		if matched, score := r.commandMatches(ctx, command, query.Search); matched {
			results = append(results, r.buildCommandResult(ctx, query, command, score))
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
								Param: instance.Metadata.Id,
							})
						},
						PreventHideAfterAction: true,
					},
				},
			})
		}
	}

	return plugin.NewQueryResponse(results)
}

func (r *SysPlugin) findCommand(commandID string) (SysCommand, bool) {
	for _, command := range r.commands {
		if command.ID == commandID && r.isCommandAvailable(command) {
			return command, true
		}
	}
	return SysCommand{}, false
}

func (r *SysPlugin) commandMatches(ctx context.Context, command SysCommand, search string) (bool, int64) {
	if !r.isCommandAvailable(command) {
		return false, 0
	}

	search = strings.TrimSpace(search)
	if search == "" {
		return true, 100
	}

	candidates := []string{
		command.ID,
		i18n.GetI18nManager().TranslateWox(ctx, command.Title),
		i18n.GetI18nManager().TranslateWoxEnUs(ctx, command.Title),
	}
	candidates = append(candidates, command.Aliases...)

	var bestScore int64
	for _, candidate := range candidates {
		matched, score := plugin.IsStringMatchScore(ctx, candidate, search)
		if matched && score > bestScore {
			bestScore = score
		}
	}

	if bestScore > 0 {
		return true, bestScore
	}
	return false, 0
}

func (r *SysPlugin) isCommandAvailable(command SysCommand) bool {
	if len(command.SupportedOS) > 0 && !slices.Contains(command.SupportedOS, util.GetCurrentPlatform()) {
		return false
	}
	if command.IsAvailable != nil && !command.IsAvailable() {
		return false
	}
	return true
}

func (r *SysPlugin) buildCommandResult(ctx context.Context, query plugin.Query, command SysCommand, score int64) plugin.QueryResult {
	title := command.Title
	if command.BuildTitle != nil {
		title = command.BuildTitle(ctx, query)
	}
	subtitle := command.SubTitle
	if command.BuildSubTitle != nil {
		subtitle = command.BuildSubTitle(ctx, query)
	}

	contextData := common.ContextData{sysCommandIDContextKey: command.ID}
	if command.BuildContextData != nil {
		for key, value := range command.BuildContextData(query) {
			contextData[key] = value
		}
	}

	return plugin.QueryResult{
		Title:    title,
		SubTitle: subtitle,
		Score:    score,
		Icon:     command.Icon,
		Actions:  []plugin.QueryResultAction{r.buildCommandAction(command, contextData)},
	}
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
	if !r.isCommandAvailable(*foundCommand) {
		return nil, fmt.Errorf("system command is not available on this platform: %s", commandID)
	}

	result := &plugin.QueryResult{
		Title:    foundCommand.Title,
		SubTitle: foundCommand.SubTitle,
		Icon:     foundCommand.Icon,
		Actions:  []plugin.QueryResultAction{r.buildCommandAction(*foundCommand, mruData.ContextData)},
	}

	return result, nil
}

func (r *SysPlugin) buildCommandAction(command SysCommand, contextData common.ContextData) plugin.QueryResultAction {
	actionName := command.ActionName
	if actionName == "" {
		actionName = "i18n:plugin_sys_execute"
	}
	actionIcon := command.ActionIcon
	if actionIcon.IsEmpty() {
		actionIcon = common.ExecuteRunIcon
	}

	return plugin.QueryResultAction{
		Name:                   actionName,
		Icon:                   actionIcon,
		Action:                 command.Action,
		PreventHideAfterAction: command.PreventHideAfterAction,
		ContextData:            contextData,
	}
}

func (r *SysPlugin) handleConfirmedSystemCommand(ctx context.Context, actionContext plugin.ActionContext, commandID string) {
	if actionContext.ContextData[sysCommandConfirmedContextKey] == "true" {
		r.executeConfirmedSystemCommand(ctx, commandID)
		return
	}

	updatable := r.api.GetUpdatableResult(ctx, actionContext.ResultId)
	if updatable == nil {
		return
	}

	titleKey, subtitleKey := r.getSystemPowerConfirmationText(commandID)
	updatedTitle := titleKey
	updatedSubtitle := subtitleKey
	updatable.Title = &updatedTitle
	updatable.SubTitle = &updatedSubtitle

	if updatable.Actions != nil {
		actions := *updatable.Actions
		for i := range actions {
			if actions[i].Id != actionContext.ResultActionId {
				continue
			}
			if actions[i].ContextData == nil {
				actions[i].ContextData = common.ContextData{}
			}
			actions[i].ContextData[sysCommandIDContextKey] = commandID
			actions[i].ContextData[sysCommandConfirmedContextKey] = "true"
			actions[i].PreventHideAfterAction = true
		}
		updatable.Actions = &actions
	}

	r.api.UpdateResult(ctx, *updatable)
}

func (r *SysPlugin) getSystemPowerConfirmationText(commandID string) (string, string) {
	switch commandID {
	case "restart_computer":
		return "i18n:plugin_sys_restart_confirm_title", "i18n:plugin_sys_restart_confirm_subtitle"
	case "sleep":
		return "i18n:plugin_sys_sleep_confirm_title", "i18n:plugin_sys_sleep_confirm_subtitle"
	case "log-out":
		return "i18n:plugin_sys_log_out_confirm_title", "i18n:plugin_sys_log_out_confirm_subtitle"
	default:
		return "i18n:plugin_sys_shutdown_confirm_title", "i18n:plugin_sys_shutdown_confirm_subtitle"
	}
}

func (r *SysPlugin) executeConfirmedSystemCommand(ctx context.Context, commandID string) {
	var err error

	switch commandID {
	case "restart_computer":
		_, err = runRestartCommand()
	case "sleep":
		_, err = runSleepCommand()
	case "log-out":
		_, err = runLogoutCommand()
	default:
		_, err = runShutdownCommand()
	}

	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to execute %s: %s", commandID, err.Error()))
		r.api.Notify(ctx, err.Error())
		return
	}
}

func (r *SysPlugin) runSystemAction(ctx context.Context, commandID string, action func() (*exec.Cmd, error)) {
	_, err := action()
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to execute %s: %s", commandID, err.Error()))
		r.api.Notify(ctx, err.Error())
	}
}

func buildSetVolumeContextData(query plugin.Query) common.ContextData {
	if percent, ok := parseVolumePercent(query.Search); ok {
		return common.ContextData{sysCommandVolumeContextKey: strconv.Itoa(percent)}
	}
	return common.ContextData{}
}

func buildSetVolumeTitle(ctx context.Context, query plugin.Query) string {
	if percent, ok := parseVolumePercent(query.Search); ok {
		return fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_sys_set_volume_to_percent"), percent)
	}
	return "i18n:plugin_sys_set_volume"
}

func parseVolumeContext(contextData common.ContextData) (int, bool) {
	return parseVolumePercent(contextData[sysCommandVolumeContextKey])
}

func parseVolumePercent(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, "%")
	if raw == "" {
		return 0, false
	}

	percent, err := strconv.Atoi(raw)
	if err != nil || percent < 0 || percent > 100 {
		return 0, false
	}
	return percent, true
}

func runShutdownCommand() (*exec.Cmd, error) {
	return runPlatformShutdownCommand()
}

func runRestartCommand() (*exec.Cmd, error) {
	return runPlatformRestartCommand()
}
