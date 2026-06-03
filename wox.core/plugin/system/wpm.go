package system

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path"
	"sort"
	"strings"
	texttmpl "text/template"
	"time"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
	"wox/util/shell"
	"wox/util/trash"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	cp "github.com/otiai10/copy"
	"github.com/samber/lo"
)

var wpmIcon = common.PluginWPMIcon
var localPluginDirectoriesKey = "local_plugin_directories"

const (
	wpmInstallStatusRefinementKey          = "wpm_install_status"
	wpmInstallStatusRefinementAll          = "all"
	wpmInstallStatusRefinementInstalled    = "installed"
	wpmInstallStatusRefinementNotInstalled = "not_installed"
	wpmInstallStatusRefinementUpgradable   = "upgradable"
)

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
		Name:    "plugin_wpm_script_template_nodejs",
		Url:     "template.js",
	},
	{
		Runtime: plugin.PLUGIN_RUNTIME_SCRIPT,
		Name:    "plugin_wpm_script_template_python",
		Url:     "template.py",
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
	metadata plugin.Metadata
	watcher  *fsnotify.Watcher
}

func (w *WPMPlugin) getLocalPluginName(ctx context.Context, metadata plugin.Metadata) string {
	return metadata.GetName(ctx)
}

func (w *WPMPlugin) getLocalPluginDescription(ctx context.Context, metadata plugin.Metadata) string {
	return metadata.GetDescription(ctx)
}

func (w *WPMPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "e2c5f005-6c73-43c8-bc53-ab04def265b2",
		Name:          "i18n:plugin_wpm_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_wpm_plugin_description",
		Icon:          wpmIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"wpm",
			"store",
			"pm",
			"*",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
			},
			{
				Name: plugin.MetadataFeatureResultPreviewWidthRatio,
				Params: map[string]any{
					"WidthRatio": 0.35,
				},
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
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
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
		return lp.metadata.Id == metadata.Id
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
			return lp.metadata.Id != metadata.Id
		})
	}

	// watch dist directory changes and auto reload plugin
	distDirectory := path.Join(pluginDirectory, "dist")
	if _, statErr := os.Stat(distDirectory); statErr == nil {
		watcher, watchErr := util.WatchDirectoryChanges(ctx, distDirectory, func(e fsnotify.Event) {
			if e.Op != fsnotify.Chmod {
				// debounce reload plugin to avoid reload multiple times in a short time
				if t, ok := w.reloadPluginTimers.Load(metadata.Id); ok {
					t.Stop()
				}
				w.reloadPluginTimers.Store(metadata.Id, time.AfterFunc(time.Second*2, func() {
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

func (w *WPMPlugin) parseMetadata(ctx context.Context, directory string) (plugin.Metadata, error) {
	// parse plugin.json in directory
	metadata, metadataErr := plugin.GetPluginManager().ParseMetadata(ctx, directory)
	if metadataErr != nil {
		return plugin.Metadata{}, fmt.Errorf("failed to parse plugin.json in %s: %s", directory, metadataErr.Error())
	}
	return metadata, nil
}

func (w *WPMPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if query.IsGlobalQuery() {
		return plugin.NewQueryResponse(w.globalQueryCommand(ctx, query))
	}

	if query.Command == "create" {
		return plugin.NewQueryResponse(w.createCommand(ctx, query))
	}

	if query.Command == "install" {
		response := plugin.NewQueryResponse(w.installCommand(ctx, query))
		response.Refinements = []plugin.QueryRefinement{w.buildInstallStatusRefinement()}
		return response
	}

	if query.Command == "uninstall" {
		return plugin.NewQueryResponse(w.uninstallCommand(ctx, query))
	}

	if query.Command == "dev.add" {
		return plugin.NewQueryResponse(w.addDevCommand(ctx, query))
	}

	if query.Command == "dev.remove" {
		return plugin.NewQueryResponse(w.removeDevCommand(ctx, query))
	}

	if query.Command == "dev.reload" {
		return plugin.NewQueryResponse(w.reloadDevCommand(ctx))
	}

	if query.Command == "dev.list" {
		return plugin.NewQueryResponse(w.listDevCommand(ctx))
	}

	return plugin.QueryResponse{}
}

func (w *WPMPlugin) globalQueryCommand(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	if strings.TrimSpace(query.Search) == "" {
		return []plugin.QueryResult{}
	}

	// Feature addition: global query should be discovery-only. The explicit
	// "wpm install" command can show installed, upgradable, and uninstallable
	// rows, but global search stays focused on plugins the user does not have.
	installedById := w.buildInstalledPluginLookup()
	results := []plugin.QueryResult{}
	for _, pluginManifest := range w.searchStorePlugins(ctx, query.Search) {
		if _, installed := installedById[pluginManifest.Id]; installed {
			continue
		}

		results = append(results, w.buildGlobalStorePluginResult(ctx, pluginManifest))
	}

	return results
}

func (w *WPMPlugin) buildGlobalStorePluginResult(ctx context.Context, pluginManifest plugin.StorePluginManifest) plugin.QueryResult {
	pluginName := pluginManifest.GetName(ctx)

	// Feature addition: Enter from global query changes the launcher into the
	// explicit WPM install command instead of installing immediately. This keeps
	// the existing install preview and confirmation flow in one place.
	return plugin.QueryResult{
		Id:       uuid.NewString(),
		Title:    pluginName,
		SubTitle: pluginManifest.GetDescription(ctx),
		Icon:     w.buildPluginDetailIcon(pluginManifest),
		Tails:    []plugin.QueryResultTail{plugin.NewQueryResultTailText(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_plugin_store"))},
		Preview:  w.buildPluginDetailPreview(ctx, pluginManifest, false, false),
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "i18n:plugin_wpm_view_install",
				Icon:                   wpmIcon,
				IsDefault:              true,
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					w.api.ChangeQuery(ctx, common.PlainQuery{
						QueryType: plugin.QueryTypeInput,
						QueryText: fmt.Sprintf("wpm install %s", pluginName),
					})
				},
			},
		},
	}
}

func (w *WPMPlugin) buildInstallStatusRefinement() plugin.QueryRefinement {
	// Feature addition: WPM install can now narrow store results by install
	// state through the shared QueryRefinement channel. The default "All"
	// value preserves the old install search behavior until the user explicitly
	// chooses a status filter.
	return plugin.QueryRefinement{
		Id:           wpmInstallStatusRefinementKey,
		Title:        "i18n:plugin_wpm_refinement_status",
		Type:         plugin.QueryRefinementTypeSingleSelect,
		DefaultValue: []string{wpmInstallStatusRefinementAll},
		Hotkey:       wpmInstallStatusRefinementHotkey(),
		Persist:      false,
		Options: []plugin.QueryRefinementOption{
			{Value: wpmInstallStatusRefinementAll, Title: "i18n:plugin_wpm_refinement_status_all"},
			{Value: wpmInstallStatusRefinementInstalled, Title: "i18n:plugin_wpm_refinement_status_installed"},
			{Value: wpmInstallStatusRefinementNotInstalled, Title: "i18n:plugin_wpm_refinement_status_not_installed"},
			{Value: wpmInstallStatusRefinementUpgradable, Title: "i18n:plugin_wpm_refinement_status_upgradable"},
		},
	}
}

func wpmInstallStatusRefinementHotkey() string {
	return util.PrimaryHotkey("s")
}

func selectedWPMInstallStatus(query plugin.Query) string {
	selectedStatus := query.Refinements[wpmInstallStatusRefinementKey]
	if selectedStatus == "" {
		return wpmInstallStatusRefinementAll
	}

	switch selectedStatus {
	case wpmInstallStatusRefinementInstalled, wpmInstallStatusRefinementNotInstalled, wpmInstallStatusRefinementUpgradable:
		return selectedStatus
	default:
		return wpmInstallStatusRefinementAll
	}
}

func wpmInstallStatusMatches(selectedStatus string, installed bool, upgradable bool) bool {
	switch selectedStatus {
	case wpmInstallStatusRefinementInstalled:
		return installed
	case wpmInstallStatusRefinementNotInstalled:
		return !installed
	case wpmInstallStatusRefinementUpgradable:
		return upgradable
	default:
		return true
	}
}

func (w *WPMPlugin) searchStorePlugins(ctx context.Context, keyword string) []plugin.StorePluginManifest {
	pluginManifests := plugin.GetStoreManager().Search(ctx, keyword)

	// Refactor support: global query and explicit install query both need the
	// same stable ordering. Keeping the sort in one helper prevents discovery
	// results from drifting away from the WPM install command.
	sort.SliceStable(pluginManifests, func(i, j int) bool {
		return pluginManifests[i].GetName(ctx) < pluginManifests[j].GetName(ctx)
	})

	return pluginManifests
}

func (w *WPMPlugin) buildInstalledPluginLookup() map[string]*plugin.Instance {
	// Refactor support: install filtering and global discovery both need an
	// exact installed-state lookup. A map keeps the behavior consistent without
	// repeatedly scanning every installed plugin for each store row.
	installedById := map[string]*plugin.Instance{}
	for _, installedPlugin := range plugin.GetPluginManager().GetPluginInstances() {
		installedById[installedPlugin.Metadata.Id] = installedPlugin
	}

	return installedById
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
			SubTitle: fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_create_plugin_name"), query.Search),
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
		templateDisplayName := i18n.GetI18nManager().TranslateWox(ctx, template.Name)

		// Check if script plugin already exists
		exists, fileName := w.checkScriptPluginExists(pluginName, template.Url)

		var title, subtitle string
		var actions []plugin.QueryResultAction

		if exists {
			title = fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_script_plugin_exists_title"), templateDisplayName)
			subtitle = fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_script_plugin_exists_subtitle"), fileName)
			// When file exists, provide actions to open or overwrite the existing file
			actions = []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_wpm_script_plugin_open_existing_file",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						userScriptPluginDirectory := util.GetLocation().GetUserScriptPluginsDirectory()
						scriptFilePath := path.Join(userScriptPluginDirectory, fileName)
						openErr := shell.Open(scriptFilePath)
						if openErr != nil {
							w.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_open_file_failed"), openErr.Error()))
						}
					},
				},
				{
					Name:                   "i18n:plugin_wpm_script_plugin_overwrite_existing_file",
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
			title = fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_script_plugin_create_title"), templateDisplayName)
			subtitle = fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_create_plugin_name"), query.Search)
			actions = []plugin.QueryResultAction{
				{
					Name:                   "i18n:plugin_wpm_create",
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						pluginName := query.Search
						util.Go(ctx, "create script plugin", func() {
							w.createScriptPluginWithTemplate(ctx, templateCopy, pluginName, query)
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
		return !pluginInstance.IsSystemPlugin
	})
	if query.Search != "" {
		plugins = lo.Filter(plugins, func(pluginInstance *plugin.Instance, _ int) bool {
			isNameMatch := plugin.IsStringMatch(ctx, pluginInstance.GetName(ctx), query.Search)
			isDescriptionMatch := plugin.IsStringMatch(ctx, pluginInstance.GetDescription(ctx), query.Search)
			isTriggerKeywordMatch := lo.SomeBy(pluginInstance.Metadata.TriggerKeywords, func(kw string) bool {
				return plugin.IsStringMatchNoPinYin(ctx, kw, query.Search)
			})
			return isNameMatch || isDescriptionMatch || isTriggerKeywordMatch
		})
	}

	results = lo.Map(plugins, func(pluginInstanceShadow *plugin.Instance, _ int) plugin.QueryResult {
		// action will be executed in another go routine, so we need to copy the variable
		pluginInstance := pluginInstanceShadow

		icon := common.ParseWoxImageOrDefault(pluginInstance.Metadata.Icon, wpmIcon)
		icon = common.ConvertRelativePathToAbsolutePath(ctx, icon, pluginInstance.PluginDirectory)

		return plugin.QueryResult{
			Id:       uuid.NewString(),
			Title:    pluginInstance.GetName(ctx),
			SubTitle: pluginInstance.GetDescription(ctx),
			Icon:     icon,
			Actions: []plugin.QueryResultAction{
				{
					Name:                   "i18n:plugin_wpm_uninstall",
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						pluginName := pluginInstance.GetName(ctx)
						w.api.Notify(ctx, fmt.Sprintf(
							w.api.GetTranslation(ctx, "i18n:plugin_installer_action_start"),
							w.api.GetTranslation(ctx, "i18n:plugin_installer_uninstall"),
							pluginName,
						))
						if err := plugin.GetStoreManager().UninstallWithProgress(ctx, pluginInstance, false, func(message string) {
							w.api.Notify(ctx, fmt.Sprintf("%s: %s", pluginName, message))
						}); err != nil {
							w.api.Notify(ctx, fmt.Sprintf(
								w.api.GetTranslation(ctx, "i18n:plugin_installer_action_failed"),
								w.api.GetTranslation(ctx, "i18n:plugin_installer_uninstall"),
								fmt.Sprintf("%s: %s", pluginName, err.Error()),
							))
						} else {
							w.api.Notify(ctx, fmt.Sprintf(
								w.api.GetTranslation(ctx, "i18n:plugin_installer_action_success"),
								pluginName,
								w.api.GetTranslation(ctx, "i18n:plugin_installer_verb_uninstall_past"),
							))
						}
					},
				},
			},
		}
	})
	return results
}

// buildPluginDetailIcon returns the display icon for a store plugin manifest.
func (w *WPMPlugin) buildPluginDetailIcon(manifest plugin.StorePluginManifest) common.WoxImage {
	if manifest.IconEmoji != "" {
		return common.NewWoxImageEmoji(manifest.IconEmoji)
	}
	if manifest.IconUrl != "" {
		return common.NewWoxImageUrl(manifest.IconUrl)
	}
	return wpmIcon
}

// buildPluginDetailPreview constructs the WoxPreview shown in the preview panel
// for a plugin listed in installCommand results.
// isInstalled reflects whether the plugin is currently installed.
// isInstalling reflects whether an install/upgrade is currently in progress;
// the preview intentionally no longer serializes either state. The result tail
// and toolbar message already carry install progress, and repeating the same
// state inside plugin detail preview made the UI feel noisy.
func (w *WPMPlugin) buildPluginDetailPreview(ctx context.Context, manifest plugin.StorePluginManifest, isInstalled bool, isInstalling bool) plugin.WoxPreview {
	icon := w.buildPluginDetailIcon(manifest)
	pluginDetailData := map[string]interface{}{
		"Id":             manifest.Id,
		"Name":           manifest.GetName(ctx),
		"Description":    manifest.GetDescription(ctx),
		"Author":         manifest.Author,
		"Version":        manifest.Version,
		"Icon":           icon,
		"Website":        manifest.Website,
		"Runtime":        manifest.Runtime,
		"ScreenshotUrls": manifest.ScreenshotUrls,
	}
	pluginDetailJSON, _ := json.Marshal(pluginDetailData)
	return plugin.WoxPreview{
		PreviewType: plugin.WoxPreviewTypePluginDetail,
		PreviewData: string(pluginDetailJSON),
	}
}

// buildPostInstallActions returns the actions to show after a successful
// install or upgrade: always includes Uninstall, and adds "Start Using" when
// the plugin has a non-wildcard trigger keyword.
func (w *WPMPlugin) buildPostInstallActions(ctx context.Context, pluginManifest plugin.StorePluginManifest) []plugin.QueryResultAction {
	newActions := []plugin.QueryResultAction{w.createUninstallAction(pluginManifest)}

	// Brief wait for plugin manager to register the newly-loaded plugin so its
	// trigger keywords are already available when we build the action list.
	time.Sleep(500 * time.Millisecond)
	instances := plugin.GetPluginManager().GetPluginInstances()
	if len(instances) > 0 {
		if inst, ok := lo.Find(instances, func(it *plugin.Instance) bool { return it.Metadata.Id == pluginManifest.Id }); ok {
			if len(inst.Metadata.TriggerKeywords) > 0 {
				kw := inst.Metadata.TriggerKeywords[0]
				if kw != "*" && strings.TrimSpace(kw) != "" {
					// Capture kw in a local variable for the closure so that
					// all "Start Using" closures do not share the loop variable.
					kwCopy := kw
					newActions = append(newActions, plugin.QueryResultAction{
						Name:                   "i18n:plugin_wpm_start_using",
						Icon:                   common.NewWoxImageEmoji("▶️"),
						PreventHideAfterAction: true,
						IsDefault:              true,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							w.api.ChangeQuery(ctx, common.PlainQuery{QueryType: plugin.QueryTypeInput, QueryText: kwCopy + " "})
						},
					})
				}
			}
		}
	}
	return newActions
}

// createInstallAction creates an install action with immediate UI lock:
// as soon as the user presses Enter the install action is removed and the
// preview shows "Installing..." so they cannot trigger a second install while
// one is already in flight. On failure the install action is restored.
func (w *WPMPlugin) createInstallAction(pluginManifest plugin.StorePluginManifest) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Name:                   "i18n:plugin_wpm_install",
		Icon:                   common.InstallIcon,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			// Lock the UI immediately (before spawning the goroutine) so the user
			// cannot press Enter again while the install is running.
			if updatable := w.api.GetUpdatableResult(ctx, actionContext.ResultId); updatable != nil {
				emptyActions := []plugin.QueryResultAction{}
				updatable.Actions = &emptyActions
				installingPreview := w.buildPluginDetailPreview(ctx, pluginManifest, false, true)
				updatable.Preview = &installingPreview
				w.api.UpdateResult(ctx, *updatable)
			}

			util.Go(ctx, "install plugin", func() {
				pluginName := pluginManifest.GetName(ctx)
				w.api.Notify(ctx, fmt.Sprintf(
					w.api.GetTranslation(ctx, "i18n:plugin_installer_action_start"),
					w.api.GetTranslation(ctx, "i18n:plugin_installer_install"),
					pluginName,
				))

				installErr := plugin.GetStoreManager().InstallWithProgress(ctx, pluginManifest, func(message string) {
					w.api.Notify(ctx, fmt.Sprintf("%s: %s", pluginName, message))
				})

				if installErr != nil {
					// Restore the install action so the user can retry after a failure.
					if updatable := w.api.GetUpdatableResult(ctx, actionContext.ResultId); updatable != nil {
						restoredActions := []plugin.QueryResultAction{w.createInstallAction(pluginManifest)}
						updatable.Actions = &restoredActions
						restoredPreview := w.buildPluginDetailPreview(ctx, pluginManifest, false, false)
						updatable.Preview = &restoredPreview
						w.api.UpdateResult(ctx, *updatable)
					}
					w.api.Notify(ctx, fmt.Sprintf(
						w.api.GetTranslation(ctx, "i18n:plugin_installer_action_failed"),
						w.api.GetTranslation(ctx, "i18n:plugin_installer_install"),
						formatPluginInstallError(ctx, w.api, pluginManifest.Runtime, pluginName, pluginManifest.Version, installErr),
					))
					return
				}

				// Update tails, preview, and actions to the installed state.
				if updatable := w.api.GetUpdatableResult(ctx, actionContext.ResultId); updatable != nil {
					newTails := []plugin.QueryResultTail{{Type: plugin.QueryResultTailTypeImage, Image: common.PluginInstalledIcon}}
					updatable.Tails = &newTails
					successPreview := w.buildPluginDetailPreview(ctx, pluginManifest, true, false)
					updatable.Preview = &successPreview
					newActions := w.buildPostInstallActions(ctx, pluginManifest)
					updatable.Actions = &newActions
					w.api.UpdateResult(ctx, *updatable)
				}

				w.api.Notify(ctx, fmt.Sprintf(
					w.api.GetTranslation(ctx, "i18n:plugin_installer_action_success"),
					pluginName,
					w.api.GetTranslation(ctx, "i18n:plugin_installer_verb_install_past"),
				))
			})
		},
	}
}

// createUpgradeAction creates an upgrade action with the same UI-lock pattern
// as createInstallAction: the action is immediately removed when triggered to
// prevent double-upgrades, and restored if the upgrade fails.
func (w *WPMPlugin) createUpgradeAction(pluginManifest plugin.StorePluginManifest) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Name:                   "i18n:plugin_wpm_upgrade",
		Icon:                   common.UpdateIcon,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			// Lock the UI immediately so the user cannot press Enter again
			// while the upgrade is running.
			if updatable := w.api.GetUpdatableResult(ctx, actionContext.ResultId); updatable != nil {
				emptyActions := []plugin.QueryResultAction{}
				updatable.Actions = &emptyActions
				installingPreview := w.buildPluginDetailPreview(ctx, pluginManifest, true, true)
				updatable.Preview = &installingPreview
				w.api.UpdateResult(ctx, *updatable)
			}

			util.Go(ctx, "upgrade plugin", func() {
				pluginName := pluginManifest.GetName(ctx)
				w.api.Notify(ctx, fmt.Sprintf(
					w.api.GetTranslation(ctx, "i18n:plugin_installer_action_start"),
					w.api.GetTranslation(ctx, "i18n:plugin_installer_upgrade"),
					pluginName,
				))

				installErr := plugin.GetStoreManager().InstallWithProgress(ctx, pluginManifest, func(message string) {
					w.api.Notify(ctx, fmt.Sprintf("%s: %s", pluginName, message))
				})

				if installErr != nil {
					// Restore upgrade + uninstall actions so the user can retry.
					if updatable := w.api.GetUpdatableResult(ctx, actionContext.ResultId); updatable != nil {
						restoredActions := []plugin.QueryResultAction{w.createUpgradeAction(pluginManifest), w.createUninstallAction(pluginManifest)}
						updatable.Actions = &restoredActions
						restoredPreview := w.buildPluginDetailPreview(ctx, pluginManifest, true, false)
						updatable.Preview = &restoredPreview
						w.api.UpdateResult(ctx, *updatable)
					}
					w.api.Notify(ctx, fmt.Sprintf(
						w.api.GetTranslation(ctx, "i18n:plugin_installer_action_failed"),
						w.api.GetTranslation(ctx, "i18n:plugin_installer_upgrade"),
						formatPluginInstallError(ctx, w.api, pluginManifest.Runtime, pluginName, pluginManifest.Version, installErr),
					))
					return
				}

				// Update tails, preview, and actions to the installed state.
				if updatable := w.api.GetUpdatableResult(ctx, actionContext.ResultId); updatable != nil {
					newTails := []plugin.QueryResultTail{{Type: plugin.QueryResultTailTypeImage, Image: common.PluginInstalledIcon}}
					updatable.Tails = &newTails
					successPreview := w.buildPluginDetailPreview(ctx, pluginManifest, true, false)
					updatable.Preview = &successPreview
					newActions := w.buildPostInstallActions(ctx, pluginManifest)
					updatable.Actions = &newActions
					w.api.UpdateResult(ctx, *updatable)
				}

				w.api.Notify(ctx, fmt.Sprintf(
					w.api.GetTranslation(ctx, "i18n:plugin_installer_action_success"),
					pluginName,
					w.api.GetTranslation(ctx, "i18n:plugin_installer_verb_upgrade_past"),
				))
			})
		},
	}
}

// createUninstallAction creates an uninstall action that updates to install action after success
func (w *WPMPlugin) createUninstallAction(pluginManifest plugin.StorePluginManifest) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Name:                   "i18n:plugin_wpm_uninstall",
		Icon:                   common.TrashIcon,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			pluginName := pluginManifest.GetName(ctx)
			instances := plugin.GetPluginManager().GetPluginInstances()
			if inst, ok := lo.Find(instances, func(it *plugin.Instance) bool { return it.Metadata.Id == pluginManifest.Id }); ok {
				util.Go(ctx, "uninstall plugin", func() {
					w.api.Notify(ctx, fmt.Sprintf(
						w.api.GetTranslation(ctx, "i18n:plugin_installer_action_start"),
						w.api.GetTranslation(ctx, "i18n:plugin_installer_uninstall"),
						pluginName,
					))

					uninstallErr := plugin.GetStoreManager().UninstallWithProgress(ctx, inst, false, func(message string) {
						w.api.Notify(ctx, fmt.Sprintf("%s: %s", pluginName, message))
					})
					if uninstallErr != nil {
						w.api.Notify(ctx, fmt.Sprintf(
							w.api.GetTranslation(ctx, "i18n:plugin_installer_action_failed"),
							w.api.GetTranslation(ctx, "i18n:plugin_installer_uninstall"),
							fmt.Sprintf("%s(%s): %s", pluginName, pluginManifest.Version, uninstallErr.Error()),
						))
						return
					}

					// Update tails, preview, and actions to the uninstalled state.
					if updatable := w.api.GetUpdatableResult(ctx, actionContext.ResultId); updatable != nil {
						newTails := []plugin.QueryResultTail{}
						updatable.Tails = &newTails
						uninstalledPreview := w.buildPluginDetailPreview(ctx, pluginManifest, false, false)
						updatable.Preview = &uninstalledPreview
						newActions := []plugin.QueryResultAction{w.createInstallAction(pluginManifest)}
						updatable.Actions = &newActions
						w.api.UpdateResult(ctx, *updatable)
					}

					w.api.Notify(ctx, fmt.Sprintf(
						w.api.GetTranslation(ctx, "i18n:plugin_installer_action_success"),
						pluginName,
						w.api.GetTranslation(ctx, "i18n:plugin_installer_verb_uninstall_past"),
					))
				})
			}
		},
	}
}

func (w *WPMPlugin) installCommand(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult
	pluginManifests := w.searchStorePlugins(ctx, query.Search)

	// Feature addition: build one installed-plugin lookup and use it for both
	// filtering and row decoration. The previous per-row scans were acceptable
	// for tails/actions only, but refinement filtering needs the same status
	// decision to stay consistent across the whole result.
	installedById := w.buildInstalledPluginLookup()
	selectedStatus := selectedWPMInstallStatus(query)

	for _, pluginManifest := range pluginManifests {
		installedPlugin, installedFlag := installedById[pluginManifest.Id]
		upgradeFlag := installedFlag && plugin.IsVersionUpgradable(installedPlugin.Metadata.Version, pluginManifest.Version)
		if !wpmInstallStatusMatches(selectedStatus, installedFlag, upgradeFlag) {
			continue
		}

		// build tails to indicate installation/upgrade status
		var tails []plugin.QueryResultTail
		if installedFlag {
			// plugin is installed, check if upgrade is available
			if upgradeFlag {
				// show an upgrade icon
				tails = append(tails, plugin.QueryResultTail{Type: plugin.QueryResultTailTypeImage, Image: common.UpgradeIcon})
			} else {
				// show an installed icon
				tails = append(tails, plugin.QueryResultTail{Type: plugin.QueryResultTailTypeImage, Image: common.PluginInstalledIcon})
			}
		}

		// decide actions based on install/upgrade status
		var actions []plugin.QueryResultAction
		if installedFlag {
			if upgradeFlag {
				// Upgrade is available: show Upgrade + Uninstall actions.
				actions = []plugin.QueryResultAction{
					w.createUpgradeAction(pluginManifest),
					w.createUninstallAction(pluginManifest),
				}
			} else {
				// installed and up-to-date: provide uninstall
				actions = []plugin.QueryResultAction{w.createUninstallAction(pluginManifest)}
			}
		} else {
			// not installed: show Install
			actions = []plugin.QueryResultAction{w.createInstallAction(pluginManifest)}
		}

		// Add common actions for all plugins
		if pluginManifest.Website != "" {
			actions = append(actions, plugin.QueryResultAction{
				Name: "i18n:plugin_wpm_visit_website",
				Icon: common.PluginWebsearchIcon,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					shell.Open(pluginManifest.Website)
				},
			})
		}

		if pluginManifest.DownloadUrl != "" {
			actions = append(actions, plugin.QueryResultAction{
				Name: "i18n:plugin_wpm_manual_download",
				Icon: common.PluginAppIcon,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					shell.Open(pluginManifest.DownloadUrl)
				},
			})
		}

		icon := w.buildPluginDetailIcon(pluginManifest)
		pluginName := pluginManifest.GetName(ctx)
		pluginDescription := pluginManifest.GetDescription(ctx)

		results = append(results, plugin.QueryResult{
			Id:       uuid.NewString(),
			Title:    pluginName,
			SubTitle: pluginDescription,
			Icon:     icon,
			Tails:    tails,
			Preview:  w.buildPluginDetailPreview(ctx, pluginManifest, installedFlag, false),
			Actions:  actions,
		})
	}
	return results
}

func (w *WPMPlugin) listDevCommand(ctx context.Context) []plugin.QueryResult {
	//list all local plugins
	return lo.Map(w.localPlugins, func(lp localPlugin, _ int) plugin.QueryResult {
		pluginName := w.getLocalPluginName(ctx, lp.metadata)
		pluginDescription := w.getLocalPluginDescription(ctx, lp.metadata)
		iconImage := common.ParseWoxImageOrDefault(lp.metadata.Icon, wpmIcon)
		iconImage = common.ConvertIcon(ctx, iconImage, lp.metadata.Directory)

		return plugin.QueryResult{
			Id:       uuid.NewString(),
			Title:    pluginName,
			SubTitle: pluginDescription,
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
`, lp.metadata.Directory, pluginName, pluginDescription, lp.metadata.Author,
					lp.metadata.Website, lp.metadata.Version, lp.metadata.MinWoxVersion,
					lp.metadata.Runtime, lp.metadata.Entry, lp.metadata.TriggerKeywords,
					lp.metadata.Commands, lp.metadata.SupportedOS, lp.metadata.Features),
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
						deleteErr := trash.MoveToTrash(lp.metadata.Directory)
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
					Name:                   "i18n:plugin_wpm_reload",
					PreventHideAfterAction: true,
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

func (w *WPMPlugin) reloadLocalDistPlugin(ctx context.Context, localPlugin plugin.Metadata, reason string) error {
	localPluginName := w.getLocalPluginName(ctx, localPlugin)
	w.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Reloading plugin: %s, reason: %s", localPluginName, reason))

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
		w.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Reloaded plugin: %s", localPluginName))
	}

	w.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_reload_success"), localPluginName, reason))
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

	triggerKeyword := strings.ToLower(strings.ReplaceAll(cleanPluginName, " ", ""))
	if len(triggerKeyword) > 10 {
		triggerKeyword = triggerKeyword[:10]
	}

	triggerKeywordsJSON, err := json.Marshal([]string{triggerKeyword})
	if err != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to render template: %s", err.Error()))
		w.api.Notify(ctx, fmt.Sprintf("i18n:plugin_wpm_render_template_failed: %s", err.Error()))
		return
	}

	author := "Wox User"
	if currentUser, userErr := user.Current(); userErr == nil {
		if currentUser.Name != "" {
			author = currentUser.Name
		} else if currentUser.Username != "" {
			author = currentUser.Username
		}
	}

	templateData := struct {
		PluginID            string
		Name                string
		Author              string
		Description         string
		TriggerKeywordsJSON string
	}{
		PluginID:            uuid.NewString(),
		Name:                cleanPluginName,
		Author:              author,
		Description:         fmt.Sprintf("A script plugin for %s", cleanPluginName),
		TriggerKeywordsJSON: string(triggerKeywordsJSON),
	}

	scriptTemplate, err := texttmpl.New("script-plugin").Option("missingkey=error").Parse(string(templateContent))
	if err != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to render template: %s", err.Error()))
		w.api.Notify(ctx, fmt.Sprintf("i18n:plugin_wpm_render_template_failed: %s", err.Error()))
		return
	}

	var renderedTemplate bytes.Buffer
	if err := scriptTemplate.Execute(&renderedTemplate, templateData); err != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to render template: %s", err.Error()))
		w.api.Notify(ctx, fmt.Sprintf("i18n:plugin_wpm_render_template_failed: %s", err.Error()))
		return
	}

	// Write the script file
	err = os.WriteFile(scriptFilePath, renderedTemplate.Bytes(), 0755)
	if err != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to write script file: %s", err.Error()))
		w.api.Notify(ctx, fmt.Sprintf("i18n:plugin_wpm_create_script_file_failed: %s", err.Error()))
		return
	}

	// Show success notification
	w.api.Notify(ctx, fmt.Sprintf("i18n:plugin_wpm_script_plugin_created_success: %s", scriptFileName))
	w.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Created script plugin: %s", scriptFilePath))
	openErr := shell.Open(path.Dir(scriptFilePath))
	if openErr != nil {
		w.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_wpm_open_directory_failed"), openErr.Error()))
	} else {
		w.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_wpm_script_plugin_opened_directory"), scriptFileName))
	}

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

		// Use ReloadPlugin to load the plugin immediately
		loadErr := pluginManager.ReloadPlugin(ctx, metadata)
		if loadErr != nil {
			w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to load script plugin: %s", loadErr.Error()))
			w.api.Notify(ctx, fmt.Sprintf("i18n:plugin_wpm_script_plugin_manual_try: %s", triggerKeyword))
			return
		}

		w.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Successfully loaded script plugin: %s", metadata.GetName(ctx)))

		// wait a moment to ensure plugin is fully loaded
		time.Sleep(300 * time.Millisecond)

		// Change query to the new plugin
		w.api.ChangeQuery(ctx, common.PlainQuery{
			QueryType: plugin.QueryTypeInput,
			QueryText: fmt.Sprintf("%s ", triggerKeyword),
		})
	})
}
