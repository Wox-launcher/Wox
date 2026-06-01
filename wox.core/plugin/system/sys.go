package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
	PreventHideAfterAction bool
	Action                 func(ctx context.Context, actionContext plugin.ActionContext)
}

const (
	sysCommandIDContextKey        = "commandId"
	sysCommandConfirmedContextKey = "confirmed"
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
			ID:                     "shutdown_computer",
			Title:                  "i18n:plugin_sys_shutdown_computer",
			Icon:                   common.ExitIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.handleSystemPowerCommand(ctx, actionContext, "shutdown_computer")
			},
		},
		{
			ID:                     "restart_computer",
			Title:                  "i18n:plugin_sys_restart_computer",
			Icon:                   common.UpdateIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				r.handleSystemPowerCommand(ctx, actionContext, "restart_computer")
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
			Title: "test attention",
			Icon:  attentionIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				now := time.Now().Format(time.RFC3339)
				r.api.PushAttention(ctx, plugin.PushAttentionRequest{
					Key:         "sys_test_attention",
					Title:       "Test attention " + now,
					Description: "This is a persistent attention item pushed from the system test command.",
					Icon:        &attentionIcon,
					Action: &plugin.AttentionAction{
						Type:  plugin.AttentionActionTypeChangeQuery,
						Query: "attention ",
					},
				})
			},
		})

		r.commands = append(r.commands, SysCommand{
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
									Hotkey:                 "Ctrl+1",
									PreventHideAfterAction: true,
									Action: func(ctx context.Context, actionContext plugin.ToolbarMsgActionContext) {
										r.api.Notify(ctx, "Action 1 executed")
									},
								},
								{
									Name:                   "Stop and Clear",
									Icon:                   common.ExecuteRunIcon,
									Hotkey:                 "Ctrl+Enter",
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
				common.ClearConvertIconPathExistenceCache()

				r.api.Log(ctx, plugin.LogLevelInfo, "cache directory cleared successfully")
				r.api.Notify(ctx, "i18n:plugin_sys_clear_cache_success")
			}
		},
	})

	r.api.OnMRURestore(ctx, r.handleMRURestore)
}

func (r *SysPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	var results []plugin.QueryResult
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
				Actions:  []plugin.QueryResultAction{r.buildCommandAction(command, common.ContextData{sysCommandIDContextKey: command.ID})},
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
		Actions:  []plugin.QueryResultAction{r.buildCommandAction(*foundCommand, mruData.ContextData)},
	}

	return result, nil
}

func (r *SysPlugin) buildCommandAction(command SysCommand, contextData common.ContextData) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Name:                   "i18n:plugin_sys_execute",
		Icon:                   common.ExecuteRunIcon,
		Action:                 command.Action,
		PreventHideAfterAction: command.PreventHideAfterAction,
		ContextData:            contextData,
	}
}

func (r *SysPlugin) handleSystemPowerCommand(ctx context.Context, actionContext plugin.ActionContext, commandID string) {
	if actionContext.ContextData[sysCommandConfirmedContextKey] == "true" {
		r.executeSystemPowerCommand(ctx, commandID)
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
	if commandID == "restart_computer" {
		return "i18n:plugin_sys_restart_confirm_title", "i18n:plugin_sys_restart_confirm_subtitle"
	}
	return "i18n:plugin_sys_shutdown_confirm_title", "i18n:plugin_sys_shutdown_confirm_subtitle"
}

func (r *SysPlugin) executeSystemPowerCommand(ctx context.Context, commandID string) {
	var err error

	switch commandID {
	case "restart_computer":
		_, err = runRestartCommand()
	default:
		_, err = runShutdownCommand()
	}

	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to execute %s: %s", commandID, err.Error()))
		r.api.Notify(ctx, err.Error())
		return
	}
}

func runShutdownCommand() (*exec.Cmd, error) {
	if util.IsMacOS() {
		return shell.Run("osascript", "-e", `tell application "System Events" to shut down`)
	}
	if util.IsWindows() {
		return shell.Run("shutdown.exe", "/s", "/t", "0")
	}
	return shell.Run("systemctl", "poweroff")
}

func runRestartCommand() (*exec.Cmd, error) {
	if util.IsMacOS() {
		return shell.Run("osascript", "-e", `tell application "System Events" to restart`)
	}
	if util.IsWindows() {
		return shell.Run("shutdown.exe", "/r", "/t", "0")
	}
	return shell.Run("systemctl", "reboot")
}
