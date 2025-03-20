package system

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime/pprof"
	"time"
	"wox/i18n"
	"wox/plugin"
	"wox/share"
	"wox/ui"
	"wox/util"
	"wox/util/shell"
)

var sysIcon = plugin.PluginSysIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &SysPlugin{})
}

type SysPlugin struct {
	api      plugin.API
	commands []SysCommand
}

type SysCommand struct {
	Title                  string
	SubTitle               string
	Icon                   plugin.WoxImage
	PreventHideAfterAction bool
	Action                 func(ctx context.Context, actionContext plugin.ActionContext)
}

func (r *SysPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "227f7d64-df08-4e35-ad05-98a26d540d06",
		Name:          "System Commands",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Provide System related commands. e.g. shutdown,lock,setting etc.",
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
	}
}

func (r *SysPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.api = initParams.API
	r.commands = []SysCommand{
		{
			Title: "i18n:plugin_sys_lock_computer",
			Icon:  plugin.LockIcon,
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
			Title: "i18n:plugin_sys_empty_trash",
			Icon:  plugin.TrashIcon,
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
			Title: "i18n:plugin_sys_quit_wox",
			Icon:  plugin.ExitIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				ui.GetUIManager().ExitApp(ctx)
			},
		},
		{
			Title:                  "i18n:plugin_sys_open_wox_settings",
			PreventHideAfterAction: true,
			Icon:                   plugin.WoxIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				plugin.GetPluginManager().GetUI().OpenSettingWindow(ctx, share.DefaultSettingWindowContext)
			},
		},
		{
			Title: "i18n:plugin_sys_open_system_settings",
			Icon:  plugin.SettingIcon,
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
			Title: "i18n:plugin_sys_performance_cpu_profiling",
			Icon:  plugin.CPUProfileIcon,
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
			Title: "i18n:plugin_sys_delete_image_cache",
			Icon:  plugin.NewWoxImageEmoji("🗑️"),
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				location := util.GetLocation()
				imageCacheDirectory := location.GetImageCacheDirectory()
				if _, err := os.Stat(imageCacheDirectory); err == nil {
					os.RemoveAll(imageCacheDirectory)
				}
			},
		})
	}
}

func (r *SysPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	for _, command := range r.commands {
		translatedTitle := i18n.GetI18nManager().TranslateWox(ctx, command.Title)
		isTitleMatch, titleScore := IsStringMatchScore(ctx, translatedTitle, query.Search)
		if !isTitleMatch {
			translatedTitleEnUs := i18n.GetI18nManager().TranslateWoxEnUs(ctx, command.Title)
			isTitleMatch, titleScore = IsStringMatchScore(ctx, translatedTitleEnUs, query.Search)
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
						Action:                 command.Action,
						PreventHideAfterAction: command.PreventHideAfterAction,
					},
				},
			})
		}
	}

	for _, instance := range plugin.GetPluginManager().GetPluginInstances() {
		//check if plugin has setting
		if len(instance.Metadata.SettingDefinitions) > 0 {
			if match, score := IsStringMatchScore(ctx, instance.Metadata.Name, query.Search); match {
				// load icon
				pluginIcon := plugin.SettingIcon
				iconImg, parseErr := plugin.ParseWoxImage(instance.Metadata.Icon)
				if parseErr == nil {
					pluginIcon = plugin.ConvertRelativePathToAbsolutePath(ctx, iconImg, instance.PluginDirectory)
				}

				results = append(results, plugin.QueryResult{
					Title: fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_sys_open_plugin_settings"), instance.Metadata.Name),
					Score: score,
					Icon:  pluginIcon,
					Actions: []plugin.QueryResultAction{
						{
							Name: "i18n:plugin_sys_execute",
							Action: func(ctx context.Context, actionContext plugin.ActionContext) {
								plugin.GetPluginManager().GetUI().OpenSettingWindow(ctx, share.SettingWindowContext{
									Path:  "/plugin/setting",
									Param: instance.Metadata.Name,
								})
							},
							PreventHideAfterAction: true,
						},
					},
				})
			}
		}
	}

	return
}
