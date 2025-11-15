package system

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"
	"wox/util/shell"

	"github.com/Masterminds/semver/v3"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	cp "github.com/otiai10/copy"
	"github.com/samber/lo"
)

var wpmIcon = plugin.PluginWPMIcon
var localPluginDirectoriesKey = "local_plugin_directories"
var pluginTemplates = []pluginTemplate{
	{
		Runtime: plugin.PLUGIN_RUNTIME_NODEJS,
		Name:    "Wox.Plugin.Template.Nodejs",
		Url:     "https://codeload.github.com/Wox-launcher/Wox.Plugin.Template.Nodejs/zip/refs/heads/main",
	},
}

var scriptPluginTemplates = []pluginTemplate{
	{
		Runtime: plugin.PLUGIN_RUNTIME_SCRIPT,
		Name:    "JavaScript Script Plugin",
		Url:     "template.js",
	},
	{
		Runtime: plugin.PLUGIN_RUNTIME_SCRIPT,
		Name:    "Python Script Plugin",
		Url:     "template.py",
	},
	{
		Runtime: plugin.PLUGIN_RUNTIME_SCRIPT,
		Name:    "Bash Script Plugin",
		Url:     "template.sh",
	},
}

type LocalPlugin struct {
	Path string
}

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &WPMPlugin{
		reloadPluginTimers: util.NewHashMap[string, *time.Timer](),
	})
}

type WPMPlugin struct {
	api                    plugin.API
	localPluginDirectories []string
	localPlugins           []localPlugin
	reloadPluginTimers     *util.HashMap[string, *time.Timer]
}

type pluginTemplate struct {
	Runtime plugin.Runtime
	Name    string
	Url     string
}

type localPlugin struct {
	metadata plugin.MetadataWithDirectory
	watcher  *fsnotify.Watcher
}

func (w *WPMPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "e2c5f005-6c73-43c8-bc53-ab04def265b2",
		Name:          "Wox Plugin Manager",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Plugin manager for Wox",
		Icon:          wpmIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"wpm",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
			},
		},
		Commands: []plugin.MetadataCommand{
			{
				Command:     "install",
				Description: "i18n:plugin_wpm_command_install",
			},
			{
				Command:     "uninstall",
				Description: "i18n:plugin_wpm_command_uninstall",
			},
			{
				Command:     "create",
				Description: "i18n:plugin_wpm_command_create",
			},
			{
				Command:     "dev.list",
				Description: "i18n:plugin_wpm_command_dev_list",
			},
			{
				Command:     "dev.add",
				Description: "i18n:plugin_wpm_command_dev_add",
			},
			{
				Command:     "dev.remove",
				Description: "i18n:plugin_wpm_command_dev_remove",
			},
			{
				Command:     "dev.reload",
				Description: "i18n:plugin_wpm_command_dev_reload",
			},
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		SettingDefinitions: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key:     localPluginDirectoriesKey,
					Title:   "i18n:plugin_wpm_local_plugin_directories",
					Tooltip: "i18n:plugin_wpm_local_plugin_directories_tooltip",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:   "path",
							Label: "i18n:plugin_wpm_path",
							Type:  definition.PluginSettingValueTableColumnTypeDirPath,
						},
					},
				},
			},
		},
	}
}

func (w *WPMPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	w.api = initParams.API

	w.reloadAllDevPlugins(ctx)

	util.Go(ctx, "reload dev plugins in dist", func() {
		// must delay reload, because host env is not ready when system plugin init
		time.Sleep(time.Second * 5)

		newCtx := util.NewTraceContext()
		for _, lp := range w.localPlugins {
			w.reloadLocalDistPlugin(newCtx, lp.metadata, "reload after startup")
		}
	})
}

func (w *WPMPlugin) reloadAllDevPlugins(ctx context.Context) {
	var localPluginDirs []LocalPlugin
	unmarshalErr := json.Unmarshal([]byte(w.api.GetSetting(ctx, localPluginDirectoriesKey)), &localPluginDirs)
	if unmarshalErr != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to unmarshal local plugin directories: %s", unmarshalErr.Error()))
		return
	}

	// remove invalid and duplicate directories
	var pluginDirs []string
	for _, pluginDir := range localPluginDirs {
		if _, statErr := os.Stat(pluginDir.Path); statErr != nil {
			w.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("Failed to stat local plugin directory, remove it: %s", statErr.Error()))
			os.RemoveAll(pluginDir.Path)
			continue
		}

		if !lo.Contains(pluginDirs, pluginDir.Path) {
			pluginDirs = append(pluginDirs, pluginDir.Path)
		}
	}

	w.localPluginDirectories = pluginDirs
	for _, directory := range w.localPluginDirectories {
		w.loadDevPlugin(ctx, directory)
	}
}

func (w *WPMPlugin) loadDevPlugin(ctx context.Context, pluginDirectory string) {
	w.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("start to load dev plugin: %s", pluginDirectory))

	metadata, err := w.parseMetadata(ctx, pluginDirectory)
	if err != nil {
		w.api.Log(ctx, plugin.LogLevelError, err.Error())
		return
	}

	lp := localPlugin{
		metadata: metadata,
	}

	// check if plugin is already loaded
	existingLocalPlugin, exist := lo.Find(w.localPlugins, func(lp localPlugin) bool {
		return lp.metadata.Metadata.Id == metadata.Metadata.Id
	})
	if exist {
		w.api.Log(ctx, plugin.LogLevelInfo, "plugin already loaded, unload first")
		if existingLocalPlugin.watcher != nil {
			closeWatcherErr := existingLocalPlugin.watcher.Close()
			if closeWatcherErr != nil {
				w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to close watcher: %s", closeWatcherErr.Error()))
			}
		}

		w.localPlugins = lo.Filter(w.localPlugins, func(lp localPlugin, _ int) bool {
			return lp.metadata.Metadata.Id != metadata.Metadata.Id
		})
	}

	// watch dist directory changes and auto reload plugin
	distDirectory := path.Join(pluginDirectory, "dist")
	if _, statErr := os.Stat(distDirectory); statErr == nil {
		watcher, watchErr := util.WatchDirectoryChanges(ctx, distDirectory, func(e fsnotify.Event) {
			if e.Op != fsnotify.Chmod {
				// debounce reload plugin to avoid reload multiple times in a short time
				if t, ok := w.reloadPluginTimers.Load(metadata.Metadata.Id); ok {
					t.Stop()
				}
				w.reloadPluginTimers.Store(metadata.Metadata.Id, time.AfterFunc(time.Second*2, func() {
					w.reloadLocalDistPlugin(ctx, metadata, "dist directory changed")
				}))
			}
		})
		if watchErr != nil {
			w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to watch dist directory: %s", watchErr.Error()))
		} else {
			w.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Watching dist directory: %s", distDirectory))
			lp.watcher = watcher
		}
	}

	w.localPlugins = append(w.localPlugins, lp)
}

func (w *WPMPlugin) parseMetadata(ctx context.Context, directory string) (plugin.MetadataWithDirectory, error) {
	// parse plugin.json in directory
	metadata, metadataErr := plugin.GetPluginManager().ParseMetadata(ctx, directory)
	if metadataErr != nil {
		return plugin.MetadataWithDirectory{}, fmt.Errorf("failed to parse plugin.json in %s: %s", directory, metadataErr.Error())
	}
	return plugin.MetadataWithDirectory{
		Metadata:  metadata,
		Directory: directory,
	}, nil
}

func (w *WPMPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	if query.Command == "create" {
		return w.createCommand(ctx, query)
	}

	if query.Command == "install" {
		return w.installCommand(ctx, query)
	}

	if query.Command == "uninstall" {
		return w.uninstallCommand(ctx, query)
	}

	if query.Command == "dev.add" {
		return w.addDevCommand(ctx, query)
	}

	if query.Command == "dev.remove" {
		return w.removeDevCommand(ctx, query)
	}

	if query.Command == "dev.reload" {
		return w.reloadDevCommand(ctx)
	}

	if query.Command == "dev.list" {
		return w.listDevCommand(ctx)
	}

	return []plugin.QueryResult{}
}

func (w *WPMPlugin) createCommand(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	// Check if user has entered a plugin name
	pluginName := strings.TrimSpace(query.Search)
	if pluginName == "" {
		return []plugin.QueryResult{
			{
				Id:       uuid.NewString(),
				Title:    "i18n:plugin_wpm_enter_plugin_name",
				SubTitle: "i18n:plugin_wpm_enter_plugin_name_subtitle",
				Icon:     wpmIcon,
			},
		}
	}

	var results []plugin.QueryResult

	// Add regular plugin templates with group
	for _, template := range pluginTemplates {
		templateCopy := template // Create a copy for the closure
		results = append(results, plugin.QueryResult{
			Id:       uuid.NewString(),
			Title:    fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_create_plugin"), string(template.Runtime)),
			SubTitle: fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_plugin_name"), query.Search),
			Icon:     wpmIcon,
			Group:    i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_group_regular_plugins"),
			Actions: []plugin.QueryResultAction{
				{
					Name:                   "i18n:plugin_wpm_create",
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						pluginName := query.Search
						util.Go(ctx, "create plugin", func() {
							w.createPlugin(ctx, templateCopy, pluginName, query)
						})
						w.api.ChangeQuery(ctx, common.PlainQuery{
							QueryType: plugin.QueryTypeInput,
							QueryText: fmt.Sprintf("%s create ", query.TriggerKeyword),
						})
					},
				},
			}})
	}

	// Add script plugin templates with group
	for _, template := range scriptPluginTemplates {
		templateCopy := template // Create a copy for the closure

		// Check if script plugin already exists
		exists, fileName := w.checkScriptPluginExists(pluginName, template.Url)

		var title, subtitle string
		var actions []plugin.QueryResultAction

		if exists {
			title = fmt.Sprintf("⚠️ %s (Already exists)", template.Name)
			subtitle = fmt.Sprintf("File '%s' already exists. Choose an action below.", fileName)
			// When file exists, provide actions to open or overwrite the existing file
			actions = []plugin.QueryResultAction{
				{
					Name: "Open existing file",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						userScriptPluginDirectory := util.GetLocation().GetUserScriptPluginsDirectory()
						scriptFilePath := path.Join(userScriptPluginDirectory, fileName)
						openErr := shell.Open(scriptFilePath)
						if openErr != nil {
							w.api.Notify(ctx, fmt.Sprintf("Failed to open file: %s", openErr.Error()))
						}
					},
				},
				{
					Name:                   "Overwrite existing file",
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						pluginName := query.Search
						util.Go(ctx, "overwrite script plugin", func() {
							w.createScriptPluginWithTemplate(ctx, templateCopy, pluginName, query)
						})
						w.api.ChangeQuery(ctx, common.PlainQuery{
							QueryType: plugin.QueryTypeInput,
							QueryText: fmt.Sprintf("%s create ", query.TriggerKeyword),
						})
					},
				},
			}
		} else {
			title = fmt.Sprintf("Create %s", template.Name)
			subtitle = fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_plugin_name"), query.Search)
			actions = []plugin.QueryResultAction{
				{
					Name:                   "i18n:plugin_wpm_create",
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						pluginName := query.Search
						util.Go(ctx, "create script plugin", func() {
							w.createScriptPluginWithTemplate(ctx, templateCopy, pluginName, query)
						})
						w.api.ChangeQuery(ctx, common.PlainQuery{
							QueryType: plugin.QueryTypeInput,
							QueryText: fmt.Sprintf("%s create ", query.TriggerKeyword),
						})
					},
				},
			}
		}

		results = append(results, plugin.QueryResult{
			Id:       uuid.NewString(),
			Title:    title,
			SubTitle: subtitle,
			Icon:     wpmIcon,
			Group:    i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_group_script_plugins"),
			Actions:  actions,
		})
	}

	return results
}

func (w *WPMPlugin) uninstallCommand(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult
	plugins := plugin.GetPluginManager().GetPluginInstances()
	plugins = lo.Filter(plugins, func(pluginInstance *plugin.Instance, _ int) bool {
		return pluginInstance.IsSystemPlugin == false
	})
	if query.Search != "" {
		plugins = lo.Filter(plugins, func(pluginInstance *plugin.Instance, _ int) bool {
			return IsStringMatchNoPinYin(ctx, pluginInstance.Metadata.Name, query.Search)
		})
	}

	results = lo.Map(plugins, func(pluginInstanceShadow *plugin.Instance, _ int) plugin.QueryResult {
		// action will be executed in another go routine, so we need to copy the variable
		pluginInstance := pluginInstanceShadow

		icon := common.ParseWoxImageOrDefault(pluginInstance.Metadata.Icon, wpmIcon)
		icon = common.ConvertRelativePathToAbsolutePath(ctx, icon, pluginInstance.PluginDirectory)

		return plugin.QueryResult{
			Id:       uuid.NewString(),
			Title:    pluginInstance.Metadata.Name,
			SubTitle: pluginInstance.Metadata.Description,
			Icon:     icon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_wpm_uninstall",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						plugin.GetStoreManager().Uninstall(ctx, pluginInstance)
					},
				},
			},
		}
	})
	return results
}

// createInstallAction creates an install action that updates to uninstall action after success
func (w *WPMPlugin) createInstallAction(pluginManifest plugin.StorePluginManifest) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Name:                   "i18n:plugin_wpm_install",
		Icon:                   common.NewWoxImageEmoji("⬇️"),
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			util.Go(ctx, "install plugin", func() {
				// notify starting
				w.api.Notify(ctx, fmt.Sprintf(
					w.api.GetTranslation(ctx, "i18n:plugin_installer_action_start"),
					w.api.GetTranslation(ctx, "i18n:plugin_installer_install"),
					pluginManifest.Name,
				))

				// Install with progress callback
				installErr := plugin.GetStoreManager().InstallWithProgress(ctx, pluginManifest, func(message string) {
					// Show progress notification
					w.api.Notify(ctx, fmt.Sprintf("%s: %s", pluginManifest.Name, message))
				})

				if installErr != nil {
					w.api.Notify(ctx, fmt.Sprintf(
						w.api.GetTranslation(ctx, "i18n:plugin_installer_action_failed"),
						w.api.GetTranslation(ctx, "i18n:plugin_installer_install"),
						fmt.Sprintf("%s(%s): %s", pluginManifest.Name, pluginManifest.Version, installErr.Error()),
					))
					return
				}

				// update tails and actions after successful install
				if updatable := w.api.GetUpdatableResult(ctx, actionContext.ResultId); updatable != nil {
					newTails := []plugin.QueryResultTail{{Type: plugin.QueryResultTailTypeImage, Image: common.NewWoxImageEmoji("\u2705")}}
					updatable.Tails = &newTails

					// create actions: uninstall + start using (if not wildcard trigger)
					newActions := []plugin.QueryResultAction{w.createUninstallAction(pluginManifest)}

					// add "Start Using" action if plugin has non-wildcard trigger keyword
					time.Sleep(500 * time.Millisecond)
					instances := plugin.GetPluginManager().GetPluginInstances()
					if len(instances) > 0 {
						if inst, ok := lo.Find(instances, func(it *plugin.Instance) bool { return it.Metadata.Id == pluginManifest.Id }); ok {
							if len(inst.Metadata.TriggerKeywords) > 0 {
								kw := inst.Metadata.TriggerKeywords[0]
								if kw != "*" && strings.TrimSpace(kw) != "" {
									// add "Start Using" action
									newActions = append(newActions, plugin.QueryResultAction{
										Name:                   "i18n:plugin_wpm_start_using",
										Icon:                   common.NewWoxImageEmoji("▶️"),
										PreventHideAfterAction: true,
										IsDefault:              true,
										Action: func(ctx context.Context, actionContext plugin.ActionContext) {
											w.api.ChangeQuery(ctx, common.PlainQuery{QueryType: plugin.QueryTypeInput, QueryText: kw + " "})
										},
									})
								}
							}
						}
					}

					updatable.Actions = &newActions
					w.api.UpdateResult(ctx, *updatable)
				}

				// success
				w.api.Notify(ctx, fmt.Sprintf(
					w.api.GetTranslation(ctx, "i18n:plugin_installer_action_success"),
					pluginManifest.Name,
					w.api.GetTranslation(ctx, "i18n:plugin_installer_verb_install_past"),
				))
			})
		},
	}
}

// createUninstallAction creates an uninstall action that updates to install action after success
func (w *WPMPlugin) createUninstallAction(pluginManifest plugin.StorePluginManifest) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Name:                   "i18n:plugin_wpm_uninstall",
		Icon:                   common.NewWoxImageEmoji("🗑️"),
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			instances := plugin.GetPluginManager().GetPluginInstances()
			if inst, ok := lo.Find(instances, func(it *plugin.Instance) bool { return it.Metadata.Id == pluginManifest.Id }); ok {
				util.Go(ctx, "uninstall plugin", func() {
					// notify starting
					w.api.Notify(ctx, fmt.Sprintf(
						w.api.GetTranslation(ctx, "i18n:plugin_installer_action_start"),
						w.api.GetTranslation(ctx, "i18n:plugin_installer_uninstall"),
						pluginManifest.Name,
					))

					uninstallErr := plugin.GetStoreManager().Uninstall(ctx, inst)
					if uninstallErr != nil {
						w.api.Notify(ctx, fmt.Sprintf(
							w.api.GetTranslation(ctx, "i18n:plugin_installer_action_failed"),
							w.api.GetTranslation(ctx, "i18n:plugin_installer_uninstall"),
							fmt.Sprintf("%s(%s): %s", pluginManifest.Name, pluginManifest.Version, uninstallErr.Error()),
						))
						return
					}

					// update tails and actions after uninstall
					if updatable := w.api.GetUpdatableResult(ctx, actionContext.ResultId); updatable != nil {
						newTails := []plugin.QueryResultTail{}
						updatable.Tails = &newTails
						newActions := []plugin.QueryResultAction{w.createInstallAction(pluginManifest)}
						updatable.Actions = &newActions
						w.api.UpdateResult(ctx, *updatable)
					}

					// success
					w.api.Notify(ctx, fmt.Sprintf(
						w.api.GetTranslation(ctx, "i18n:plugin_installer_action_success"),
						pluginManifest.Name,
						w.api.GetTranslation(ctx, "i18n:plugin_installer_verb_uninstall_past"),
					))
				})
			}
		},
	}
}

func (w *WPMPlugin) installCommand(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult
	pluginManifests := plugin.GetStoreManager().Search(ctx, query.Search)
	// get installed plugins once for status checks
	installed := plugin.GetPluginManager().GetPluginInstances()

	for _, pluginManifest := range pluginManifests {
		// build tails to indicate installation/upgrade status
		var tails []plugin.QueryResultTail
		if inst, ok := lo.Find(installed, func(it *plugin.Instance) bool { return it.Metadata.Id == pluginManifest.Id }); ok {
			// plugin is installed, check if upgrade is available
			// best-effort semver comparison; fall back to show installed if parse fails
			upgrade := false
			if vInstalled, err1 := semver.NewVersion(inst.Metadata.Version); err1 == nil {
				if vStore, err2 := semver.NewVersion(pluginManifest.Version); err2 == nil {
					upgrade = vStore.GreaterThan(vInstalled)
				}
			}
			if upgrade {
				// show an upgrade icon
				tails = append(tails, plugin.QueryResultTail{Type: plugin.QueryResultTailTypeImage, Image: common.NewWoxImageEmoji("\u2b06\ufe0f")})
			} else {
				// show an installed icon
				tails = append(tails, plugin.QueryResultTail{Type: plugin.QueryResultTailTypeImage, Image: common.NewWoxImageEmoji("\u2705")})
			}
		}

		// decide actions based on install/upgrade status
		var actions []plugin.QueryResultAction
		installedFlag := lo.ContainsBy(installed, func(it *plugin.Instance) bool { return it.Metadata.Id == pluginManifest.Id })
		if installedFlag {
			upgradeFlag := false
			if inst, ok := lo.Find(installed, func(it *plugin.Instance) bool { return it.Metadata.Id == pluginManifest.Id }); ok {
				if vInstalled, err1 := semver.NewVersion(inst.Metadata.Version); err1 == nil {
					if vStore, err2 := semver.NewVersion(pluginManifest.Version); err2 == nil {
						upgradeFlag = vStore.GreaterThan(vInstalled)
					}
				}
			}
			if upgradeFlag {
				// show Upgrade action
				actions = make([]plugin.QueryResultAction, 0, 2)

				actions = append(actions, plugin.QueryResultAction{
					Name:                   "i18n:plugin_wpm_upgrade",
					Icon:                   plugin.UpdateIcon,
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						// notify starting
						w.api.Notify(ctx, fmt.Sprintf(
							w.api.GetTranslation(ctx, "i18n:plugin_installer_action_start"),
							w.api.GetTranslation(ctx, "i18n:plugin_installer_upgrade"),
							pluginManifest.Name,
						))

						// Start installation in background, show progress via notifications
						util.Go(ctx, "upgrade plugin", func() {
							// Install with progress callback
							installErr := plugin.GetStoreManager().InstallWithProgress(ctx, pluginManifest, func(message string) {
								// Show progress notification
								w.api.Notify(ctx, fmt.Sprintf("%s: %s", pluginManifest.Name, message))
							})

							if installErr != nil {
								w.api.Notify(ctx, fmt.Sprintf(
									w.api.GetTranslation(ctx, "i18n:plugin_installer_action_failed"),
									w.api.GetTranslation(ctx, "i18n:plugin_installer_upgrade"),
									fmt.Sprintf("%s(%s): %s", pluginManifest.Name, pluginManifest.Version, installErr.Error()),
								))
								return
							}

							// update tails and actions after successful upgrade
							if updatable := w.api.GetUpdatableResult(ctx, actionContext.ResultId); updatable != nil {
								newTails := []plugin.QueryResultTail{{Type: plugin.QueryResultTailTypeImage, Image: common.NewWoxImageEmoji("\u2705")}}
								updatable.Tails = &newTails

								// create actions: uninstall + start using (if not wildcard trigger)
								newActions := []plugin.QueryResultAction{w.createUninstallAction(pluginManifest)}

								// add "Start Using" action if plugin has non-wildcard trigger keyword
								time.Sleep(500 * time.Millisecond)
								instances := plugin.GetPluginManager().GetPluginInstances()
								if len(instances) > 0 {
									if inst, ok := lo.Find(instances, func(it *plugin.Instance) bool { return it.Metadata.Id == pluginManifest.Id }); ok {
										if len(inst.Metadata.TriggerKeywords) > 0 {
											kw := inst.Metadata.TriggerKeywords[0]
											if kw != "*" && strings.TrimSpace(kw) != "" {
												// add "Start Using" action
												newActions = append(newActions, plugin.QueryResultAction{
													Name:                   "i18n:plugin_wpm_start_using",
													Icon:                   common.NewWoxImageEmoji("▶️"),
													PreventHideAfterAction: true,
													IsDefault:              true,
													Action: func(ctx context.Context, actionContext plugin.ActionContext) {
														w.api.ChangeQuery(ctx, common.PlainQuery{QueryType: plugin.QueryTypeInput, QueryText: kw + " "})
													},
												})
											}
										}
									}
								}

								updatable.Actions = &newActions
								w.api.UpdateResult(ctx, *updatable)
							}

							// success
							w.api.Notify(ctx, fmt.Sprintf(
								w.api.GetTranslation(ctx, "i18n:plugin_installer_action_success"),
								pluginManifest.Name,
								w.api.GetTranslation(ctx, "i18n:plugin_installer_verb_upgrade_past"),
							))
						})
					},
				})

				actions = append(actions, w.createUninstallAction(pluginManifest))
			} else {
				// installed and up-to-date: provide uninstall
				actions = []plugin.QueryResultAction{w.createUninstallAction(pluginManifest)}
			}
		} else {
			// not installed: show Install
			actions = []plugin.QueryResultAction{w.createInstallAction(pluginManifest)}
		}

		// Create plugin detail JSON for preview
		pluginDetailData := map[string]interface{}{
			"Id":             pluginManifest.Id,
			"Name":           pluginManifest.Name,
			"Description":    pluginManifest.Description,
			"Author":         pluginManifest.Author,
			"Version":        pluginManifest.Version,
			"Website":        pluginManifest.Website,
			"Runtime":        pluginManifest.Runtime,
			"ScreenshotUrls": pluginManifest.ScreenshotUrls,
		}
		pluginDetailJSON, _ := json.Marshal(pluginDetailData)

		// Support both IconUrl and IconEmoji, prefer IconEmoji if both are present
		var icon common.WoxImage
		if pluginManifest.IconEmoji != "" {
			icon = common.NewWoxImageEmoji(pluginManifest.IconEmoji)
		} else if pluginManifest.IconUrl != "" {
			icon = common.NewWoxImageUrl(pluginManifest.IconUrl)
		} else {
			icon = wpmIcon
		}

		results = append(results, plugin.QueryResult{
			Id:       uuid.NewString(),
			Title:    pluginManifest.Name,
			SubTitle: pluginManifest.Description,
			Icon:     icon,
			Tails:    tails,
			Preview: plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypePluginDetail,
				PreviewData: string(pluginDetailJSON),
			},
			Actions: actions,
		})
	}
	return results
}

func (w *WPMPlugin) listDevCommand(ctx context.Context) []plugin.QueryResult {
	//list all local plugins
	return lo.Map(w.localPlugins, func(lp localPlugin, _ int) plugin.QueryResult {
		iconImage := common.ParseWoxImageOrDefault(lp.metadata.Metadata.Icon, wpmIcon)
		iconImage = common.ConvertIcon(ctx, iconImage, lp.metadata.Directory)

		return plugin.QueryResult{
			Id:       uuid.NewString(),
			Title:    lp.metadata.Metadata.Name,
			SubTitle: lp.metadata.Metadata.Description,
			Icon:     iconImage,
			Preview: plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeMarkdown,
				PreviewData: fmt.Sprintf(`
- **Directory**: %s
- **Name**: %s
- **Description**: %s
- **Author**: %s
- **Website**: %s
- **Version**: %s
- **MinWoxVersion**: %s
- **Runtime**: %s
- **Entry**: %s
- **TriggerKeywords**: %s
- **Commands**: %s
- **SupportedOS**: %s
- **Features**: %s
`, lp.metadata.Directory, lp.metadata.Metadata.Name, lp.metadata.Metadata.Description, lp.metadata.Metadata.Author,
					lp.metadata.Metadata.Website, lp.metadata.Metadata.Version, lp.metadata.Metadata.MinWoxVersion,
					lp.metadata.Metadata.Runtime, lp.metadata.Metadata.Entry, lp.metadata.Metadata.TriggerKeywords,
					lp.metadata.Metadata.Commands, lp.metadata.Metadata.SupportedOS, lp.metadata.Metadata.Features),
			},
			Actions: []plugin.QueryResultAction{
				{
					Name:      "i18n:plugin_wpm_reload",
					IsDefault: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						w.reloadLocalDistPlugin(ctx, lp.metadata, "reload by user")
					},
				},
				{
					Name: "i18n:plugin_wpm_open_directory",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						openErr := shell.Open(lp.metadata.Directory)
						if openErr != nil {
							w.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_open_directory_failed"), openErr.Error()))
						}
					},
				},
				{
					Name: "i18n:plugin_wpm_remove",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						w.localPluginDirectories = lo.Filter(w.localPluginDirectories, func(directory string, _ int) bool {
							return directory != lp.metadata.Directory
						})
						w.saveLocalPluginDirectories(ctx)
					},
				},
				{
					Name: "i18n:plugin_wpm_remove_and_delete",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						deleteErr := os.RemoveAll(lp.metadata.Directory)
						if deleteErr != nil {
							w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to delete plugin directory: %s", deleteErr.Error()))
							return
						}

						w.localPluginDirectories = lo.Filter(w.localPluginDirectories, func(directory string, _ int) bool {
							return directory != lp.metadata.Directory
						})
						w.saveLocalPluginDirectories(ctx)
					},
				},
			},
		}
	})
}

func (w *WPMPlugin) reloadDevCommand(ctx context.Context) []plugin.QueryResult {
	return []plugin.QueryResult{
		{
			Title: "i18n:plugin_wpm_reload_all_plugins",
			Icon:  wpmIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_wpm_reload",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						w.reloadAllDevPlugins(ctx)
						util.Go(ctx, "reload dev plugins in dist", func() {
							newCtx := util.NewTraceContext()
							for _, lp := range w.localPlugins {
								w.reloadLocalDistPlugin(newCtx, lp.metadata, "reload after user action")
							}
						})
					},
				},
			},
		},
	}
}

func (w *WPMPlugin) addDevCommand(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	w.api.Log(ctx, plugin.LogLevelInfo, "Please choose a directory to add local plugin")
	pluginDirectories := plugin.GetPluginManager().GetUI().PickFiles(ctx, common.PickFilesParams{IsDirectory: true})
	if len(pluginDirectories) == 0 {
		w.api.Notify(ctx, "i18n:plugin_wpm_choose_directory")
		return []plugin.QueryResult{}
	}

	pluginDirectory := pluginDirectories[0]

	if lo.Contains(w.localPluginDirectories, pluginDirectory) {
		w.api.Notify(ctx, "i18n:plugin_wpm_directory_already_added")
		return []plugin.QueryResult{}
	}

	w.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Add local plugin: %s", pluginDirectory))
	w.localPluginDirectories = append(w.localPluginDirectories, pluginDirectory)
	w.saveLocalPluginDirectories(ctx)
	w.loadDevPlugin(ctx, pluginDirectory)
	return []plugin.QueryResult{}
}

func (w *WPMPlugin) removeDevCommand(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	if len(query.Search) == 0 {
		w.api.Notify(ctx, "i18n:plugin_wpm_input_directory")
		return []plugin.QueryResult{}
	}

	pluginDirectory := query.Search
	if !lo.Contains(w.localPluginDirectories, pluginDirectory) {
		w.api.Notify(ctx, "i18n:plugin_wpm_directory_not_found")
		return []plugin.QueryResult{}
	}

	w.localPluginDirectories = lo.Filter(w.localPluginDirectories, func(directory string, _ int) bool {
		return directory != pluginDirectory
	})
	w.saveLocalPluginDirectories(ctx)
	return []plugin.QueryResult{}
}

func (w *WPMPlugin) createPlugin(ctx context.Context, template pluginTemplate, pluginName string, query plugin.Query) {
	w.api.Notify(ctx, "i18n:plugin_wpm_downloading_template")

	tempPluginDirectory := path.Join(os.TempDir(), uuid.NewString())
	if err := util.GetLocation().EnsureDirectoryExist(tempPluginDirectory); err != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to create temp plugin directory: %s", err.Error()))
		w.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_create_temp_dir_failed"), err.Error()))
		return
	}

	w.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_downloading_template_to"), template.Runtime, tempPluginDirectory))
	tempZipPath := path.Join(tempPluginDirectory, "template.zip")
	err := util.HttpDownload(ctx, template.Url, tempZipPath)
	if err != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to download template: %s", err.Error()))
		w.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_download_template_failed"), err.Error()))
		return
	}

	w.api.Notify(ctx, "i18n:plugin_wpm_extracting_template")
	err = util.Unzip(tempZipPath, tempPluginDirectory)
	if err != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to extract template: %s", err.Error()))
		w.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_extract_template_failed"), err.Error()))
		return
	}

	w.api.Notify(ctx, "i18n:plugin_wpm_choose_directory_prompt")
	pluginDirectories := plugin.GetPluginManager().GetUI().PickFiles(ctx, common.PickFilesParams{IsDirectory: true})
	if len(pluginDirectories) == 0 {
		w.api.Notify(ctx, "You need to choose a directory to create the plugin")
		return
	}
	pluginDirectory := path.Join(pluginDirectories[0], pluginName)
	w.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Creating plugin in directory: %s", pluginDirectory))

	cpErr := cp.Copy(path.Join(tempPluginDirectory, template.Name+"-main"), pluginDirectory)
	if cpErr != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to copy template: %s", cpErr.Error()))
		w.api.Notify(ctx, fmt.Sprintf("Failed to copy template: %s", cpErr.Error()))
		return
	}

	// replace variables in plugin.json
	pluginJsonPath := path.Join(pluginDirectory, "plugin.json")
	pluginJson, readErr := os.ReadFile(pluginJsonPath)
	if readErr != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to read plugin.json: %s", readErr.Error()))
		w.api.Notify(ctx, fmt.Sprintf("Failed to read plugin.json: %s", readErr.Error()))
		return
	}

	pluginJsonString := string(pluginJson)
	pluginJsonString = strings.ReplaceAll(pluginJsonString, "[Id]", uuid.NewString())
	pluginJsonString = strings.ReplaceAll(pluginJsonString, "[Name]", pluginName)
	pluginJsonString = strings.ReplaceAll(pluginJsonString, "[Runtime]", strings.ToLower(string(template.Runtime)))
	pluginJsonString = strings.ReplaceAll(pluginJsonString, "[Trigger Keyword]", "np")

	writeErr := os.WriteFile(pluginJsonPath, []byte(pluginJsonString), 0644)
	if writeErr != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to write plugin.json: %s", writeErr.Error()))
		w.api.Notify(ctx, fmt.Sprintf("Failed to write plugin.json: %s", writeErr.Error()))
		return
	}

	// replace variables in package.json
	if template.Runtime == plugin.PLUGIN_RUNTIME_NODEJS {
		packageJsonPath := path.Join(pluginDirectory, "package.json")
		packageJson, readPackageErr := os.ReadFile(packageJsonPath)
		if readPackageErr != nil {
			w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to read package.json: %s", readPackageErr.Error()))
			w.api.Notify(ctx, fmt.Sprintf("Failed to read package.json: %s", readPackageErr.Error()))
			return
		}

		packageJsonString := string(packageJson)
		packageName := strings.ReplaceAll(strings.ToLower(pluginName), ".", "_")
		packageJsonString = strings.ReplaceAll(packageJsonString, "replace_me_with_name", packageName)

		writePackageErr := os.WriteFile(packageJsonPath, []byte(packageJsonString), 0644)
		if writePackageErr != nil {
			w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to write package.json: %s", writePackageErr.Error()))
			w.api.Notify(ctx, fmt.Sprintf("Failed to write package.json: %s", writePackageErr.Error()))
			return
		}
	}

	w.localPluginDirectories = append(w.localPluginDirectories, pluginDirectory)
	w.saveLocalPluginDirectories(ctx)
	w.loadDevPlugin(ctx, pluginDirectory)
	w.api.Notify(ctx, fmt.Sprintf("Plugin created successfully: %s", pluginName))
	w.api.ChangeQuery(ctx, common.PlainQuery{
		QueryType: plugin.QueryTypeInput,
		QueryText: fmt.Sprintf("%s dev ", query.TriggerKeyword),
	})
}

func (w *WPMPlugin) saveLocalPluginDirectories(ctx context.Context) {
	var localPluginDirs []LocalPlugin
	for _, directory := range w.localPluginDirectories {
		localPluginDirs = append(localPluginDirs, LocalPlugin{Path: directory})
	}

	data, marshalErr := json.Marshal(localPluginDirs)
	if marshalErr != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to marshal local plugin directories: %s", marshalErr.Error()))
		return
	}
	w.api.SaveSetting(ctx, localPluginDirectoriesKey, string(data), false)
}

func (w *WPMPlugin) reloadLocalDistPlugin(ctx context.Context, localPlugin plugin.MetadataWithDirectory, reason string) error {
	w.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Reloading plugin: %s, reason: %s", localPlugin.Metadata.Name, reason))

	// find dist directory, if not exist, prompt user to build it
	distDirectory := path.Join(localPlugin.Directory, "dist")
	_, statErr := os.Stat(distDirectory)
	if statErr != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to stat dist directory: %s", statErr.Error()))
		return statErr
	}

	distPluginMetadata, err := w.parseMetadata(ctx, distDirectory)
	if err != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to load local plugin: %s", err.Error()))
		return err
	}
	distPluginMetadata.IsDev = true
	distPluginMetadata.DevPluginDirectory = localPlugin.Directory

	reloadErr := plugin.GetPluginManager().ReloadPlugin(ctx, distPluginMetadata)
	if reloadErr != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to reload plugin: %s", reloadErr.Error()))
		return reloadErr
	} else {
		w.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Reloaded plugin: %s", localPlugin.Metadata.Name))
	}

	w.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_reload_success"), localPlugin.Metadata.Name, reason))
	return nil
}

// checkScriptPluginExists checks if a script plugin with the given name already exists
func (w *WPMPlugin) checkScriptPluginExists(pluginName string, templateFile string) (bool, string) {
	cleanPluginName := strings.TrimSpace(pluginName)
	if cleanPluginName == "" {
		return false, ""
	}

	var fileExtension string
	switch templateFile {
	case "template.js":
		fileExtension = ".js"
	case "template.py":
		fileExtension = ".py"
	case "template.sh":
		fileExtension = ".sh"
	default:
		fileExtension = ".js" // Default fallback
	}

	userScriptPluginDirectory := util.GetLocation().GetUserScriptPluginsDirectory()
	scriptFileName := strings.ReplaceAll(strings.ToLower(cleanPluginName), " ", "-") + fileExtension
	scriptFilePath := path.Join(userScriptPluginDirectory, scriptFileName)

	if _, err := os.Stat(scriptFilePath); err == nil {
		return true, scriptFileName
	}
	return false, scriptFileName
}

// createScriptPluginWithTemplate creates a script plugin from a specific template
func (w *WPMPlugin) createScriptPluginWithTemplate(ctx context.Context, template pluginTemplate, pluginName string, query plugin.Query) {
	w.api.Notify(ctx, "i18n:plugin_wpm_creating_script_plugin")

	// Get user script plugins directory
	userScriptPluginDirectory := util.GetLocation().GetUserScriptPluginsDirectory()

	// Ensure the directory exists
	if err := util.GetLocation().EnsureDirectoryExist(userScriptPluginDirectory); err != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to create user script plugin directory: %s", err.Error()))
		w.api.Notify(ctx, fmt.Sprintf("i18n:plugin_wpm_create_script_dir_failed: %s", err.Error()))
		return
	}

	// Use the template file specified in the template
	templateFile := template.Url // We store the template filename in the Url field
	var fileExtension string

	switch templateFile {
	case "template.js":
		fileExtension = ".js"
	case "template.py":
		fileExtension = ".py"
	case "template.sh":
		fileExtension = ".sh"
	default:
		fileExtension = ".js" // Default fallback
	}

	w.api.Notify(ctx, "i18n:plugin_wpm_copying_template")

	// Read template from embedded resources
	scriptTemplateDirectory := util.GetLocation().GetScriptPluginTemplatesDirectory()
	templatePath := path.Join(scriptTemplateDirectory, templateFile)

	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to read template file: %s", err.Error()))
		w.api.Notify(ctx, fmt.Sprintf("i18n:plugin_wpm_read_template_failed: %s", err.Error()))
		return
	}

	// Generate script file name
	cleanPluginName := strings.TrimSpace(pluginName)
	// Plugin name should not be empty at this point due to validation in createCommand
	if cleanPluginName == "" {
		w.api.Log(ctx, plugin.LogLevelError, "Plugin name is empty")
		w.api.Notify(ctx, "i18n:plugin_wpm_plugin_name_empty")
		return
	}

	scriptFileName := strings.ReplaceAll(strings.ToLower(cleanPluginName), " ", "-") + fileExtension
	scriptFilePath := path.Join(userScriptPluginDirectory, scriptFileName)

	// Check if file already exists and notify user
	if _, err := os.Stat(scriptFilePath); err == nil {
		w.api.Notify(ctx, fmt.Sprintf("i18n:plugin_wpm_overwriting_script_plugin: %s", scriptFileName))
	}

	// Replace template variables
	templateString := string(templateContent)
	pluginId := strings.ReplaceAll(strings.ToLower(cleanPluginName), " ", "-")
	triggerKeyword := strings.ToLower(strings.ReplaceAll(cleanPluginName, " ", ""))
	if len(triggerKeyword) > 10 {
		triggerKeyword = triggerKeyword[:10]
	}

	// Replace template placeholders
	templateString = strings.ReplaceAll(templateString, "script-plugin-template", pluginId)
	templateString = strings.ReplaceAll(templateString, "python-script-template", pluginId)
	templateString = strings.ReplaceAll(templateString, "bash-script-template", pluginId)
	templateString = strings.ReplaceAll(templateString, "Script Plugin Template", cleanPluginName)
	templateString = strings.ReplaceAll(templateString, "Python Script Template", cleanPluginName)
	templateString = strings.ReplaceAll(templateString, "Bash Script Template", cleanPluginName)
	templateString = strings.ReplaceAll(templateString, "spt", triggerKeyword)
	templateString = strings.ReplaceAll(templateString, "pst", triggerKeyword)
	templateString = strings.ReplaceAll(templateString, "bst", triggerKeyword)
	templateString = strings.ReplaceAll(templateString, "A template for Wox script plugins", fmt.Sprintf("A script plugin for %s", cleanPluginName))
	templateString = strings.ReplaceAll(templateString, "A Python template for Wox script plugins", fmt.Sprintf("A script plugin for %s", cleanPluginName))
	templateString = strings.ReplaceAll(templateString, "A Bash template for Wox script plugins", fmt.Sprintf("A script plugin for %s", cleanPluginName))

	// Write the script file
	err = os.WriteFile(scriptFilePath, []byte(templateString), 0755)
	if err != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to write script file: %s", err.Error()))
		w.api.Notify(ctx, fmt.Sprintf("i18n:plugin_wpm_create_script_file_failed: %s", err.Error()))
		return
	}

	// Show success notification
	w.api.Notify(ctx, fmt.Sprintf("i18n:plugin_wpm_script_plugin_created_success: %s", scriptFileName))
	w.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Created script plugin: %s", scriptFilePath))

	// Actively trigger script plugin loading instead of waiting
	util.Go(ctx, "load script plugin immediately", func() {
		// Trigger immediate reload of script plugins
		pluginManager := plugin.GetPluginManager()

		// Parse the script metadata to get plugin ID
		metadata, parseErr := pluginManager.ParseScriptMetadata(ctx, scriptFilePath)
		if parseErr != nil {
			w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to parse script metadata: %s", parseErr.Error()))
			w.api.Notify(ctx, fmt.Sprintf("i18n:plugin_wpm_script_plugin_manual_try: %s", triggerKeyword))
			return
		}

		// Create metadata with directory for loading
		virtualDirectory := path.Join(userScriptPluginDirectory, metadata.Id)
		metadataWithDirectory := plugin.MetadataWithDirectory{
			Metadata:  metadata,
			Directory: virtualDirectory,
		}

		// Use ReloadPlugin to load the plugin immediately
		loadErr := pluginManager.ReloadPlugin(ctx, metadataWithDirectory)
		if loadErr != nil {
			w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to load script plugin: %s", loadErr.Error()))
			w.api.Notify(ctx, fmt.Sprintf("i18n:plugin_wpm_script_plugin_manual_try: %s", triggerKeyword))
			return
		}

		w.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Successfully loaded script plugin: %s", metadata.Name))

		// Change query to the new plugin
		w.api.ChangeQuery(ctx, common.PlainQuery{
			QueryType: plugin.QueryTypeInput,
			QueryText: fmt.Sprintf("%s ", triggerKeyword),
		})
	})
}
