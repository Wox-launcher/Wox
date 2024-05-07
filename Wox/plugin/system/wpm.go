package system

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	cp "github.com/otiai10/copy"
	"github.com/samber/lo"
	"os"
	"path"
	"strings"
	"time"
	"wox/plugin"
	"wox/setting/definition"
	"wox/share"
	"wox/util"
)

var wpmIcon = plugin.NewWoxImageSvg(`<svg t="1697178225584" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="16738" width="200" height="200"><path d="M842.99 884.364H181.01c-22.85 0-41.374-18.756-41.374-41.892V181.528c0-23.136 18.524-41.892 41.374-41.892h661.98c22.85 0 41.374 18.756 41.374 41.892v660.944c0 23.136-18.524 41.892-41.374 41.892z" fill="#9C34FE" p-id="16739" data-spm-anchor-id="a313x.search_index.0.i6.1f873a81xqBP8f"></path><path d="M387.88 307.2h-82.748v83.78c0 115.68 92.618 209.456 206.868 209.456s206.868-93.776 206.868-209.454V307.2h-82.746v83.78c0 69.408-55.572 125.674-124.122 125.674s-124.12-56.266-124.12-125.672V307.2z" fill="#FFFFFF" p-id="16740"></path></svg>`)
var localPluginDirectoriesKey = "local_plugin_directories"
var pluginTemplates = []pluginTemplate{
	{
		Runtime: plugin.PLUGIN_RUNTIME_NODEJS,
		Name:    "Wox.Plugin.Template.Nodejs",
		Url:     "https://codeload.github.com/Wox-launcher/Wox.Plugin.Template.Nodejs/zip/refs/heads/main",
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
	creatingProcess        string
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
				Description: "Install Wox plugins",
			},
			{
				Command:     "uninstall",
				Description: "Uninstall Wox plugins",
			},
			{
				Command:     "create",
				Description: "Create Wox plugin",
			},
			{
				Command:     "dev.list",
				Description: "List local Wox plugins",
			},
			{
				Command:     "dev.add",
				Description: "Add existing Wox plugin directory",
			},
			{
				Command:     "dev.remove",
				Description: "Remove local Wox plugin, followed by a directory",
			},
			{
				Command:     "dev.reload",
				Description: "Reload all dev plugins",
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
					Title:   "Local Plugin Directories",
					Tooltip: "The directories to load local plugins, useful for plugin development",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:   "path",
							Label: "Path",
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
		return w.createCommand(query)
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

func (w *WPMPlugin) createCommand(query plugin.Query) []plugin.QueryResult {
	if w.creatingProcess != "" {
		return []plugin.QueryResult{
			{
				Id:              uuid.NewString(),
				Title:           w.creatingProcess,
				SubTitle:        "Please wait...",
				Icon:            wpmIcon,
				RefreshInterval: 300,
				OnRefresh: func(ctx context.Context, current plugin.RefreshableResult) plugin.RefreshableResult {
					current.Title = w.creatingProcess
					return current
				},
			},
		}
	}

	var results []plugin.QueryResult
	for _, template := range pluginTemplates {
		results = append(results, plugin.QueryResult{
			Id:       uuid.NewString(),
			Title:    "Create " + string(template.Runtime) + " plugin",
			SubTitle: fmt.Sprintf("Name: %s", query.Search),
			Icon:     wpmIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name:                   "create",
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						pluginName := query.Search
						util.Go(ctx, "create plugin", func() {
							w.createPlugin(ctx, template, pluginName, query)
						})
						w.api.ChangeQuery(ctx, share.PlainQuery{
							QueryType: plugin.QueryTypeInput,
							QueryText: fmt.Sprintf("%s create ", query.TriggerKeyword),
						})
					},
				},
			}})
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

		icon := plugin.ParseWoxImageOrDefault(pluginInstance.Metadata.Icon, wpmIcon)
		icon = plugin.ConvertRelativePathToAbsolutePath(ctx, icon, pluginInstance.PluginDirectory)

		return plugin.QueryResult{
			Id:       uuid.NewString(),
			Title:    pluginInstance.Metadata.Name,
			SubTitle: pluginInstance.Metadata.Description,
			Icon:     icon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "uninstall",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						plugin.GetStoreManager().Uninstall(ctx, pluginInstance)
					},
				},
			},
		}
	})
	return results
}

func (w *WPMPlugin) installCommand(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult
	pluginManifests := plugin.GetStoreManager().Search(ctx, query.Search)
	for _, pluginManifestShadow := range pluginManifests {
		// action will be executed in another go routine, so we need to copy the variable
		pluginManifest := pluginManifestShadow

		screenShotsMarkdown := lo.Map(pluginManifest.ScreenshotUrls, func(screenshot string, _ int) string {
			return fmt.Sprintf("![screenshot](%s)", screenshot)
		})

		results = append(results, plugin.QueryResult{
			Id:       uuid.NewString(),
			Title:    pluginManifest.Name,
			SubTitle: pluginManifest.Description,
			Icon:     plugin.NewWoxImageUrl(pluginManifest.IconUrl),
			Preview: plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeMarkdown,
				PreviewData: fmt.Sprintf(`
### Description

%s

### Website

%s

### Screenshots

%s
`, pluginManifest.Description, pluginManifest.Website, strings.Join(screenShotsMarkdown, "\n")),
				PreviewProperties: map[string]string{
					"Author":  pluginManifest.Author,
					"Version": pluginManifest.Version,
					"Website": pluginManifest.Website,
				},
			},
			Actions: []plugin.QueryResultAction{
				{
					Name: "install",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						installErr := plugin.GetStoreManager().Install(ctx, pluginManifest)
						if installErr != nil {
							w.api.Notify(ctx, "Failed to install plugin", installErr.Error())
						}
					},
				},
			}})
	}
	return results
}

func (w *WPMPlugin) listDevCommand(ctx context.Context) []plugin.QueryResult {
	//list all local plugins
	return lo.Map(w.localPlugins, func(lp localPlugin, _ int) plugin.QueryResult {
		iconImage := plugin.ParseWoxImageOrDefault(lp.metadata.Metadata.Icon, wpmIcon)
		iconImage = plugin.ConvertIcon(ctx, iconImage, lp.metadata.Directory)

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
					Name:      "Reload",
					IsDefault: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						w.reloadLocalDistPlugin(ctx, lp.metadata, "reload by user")
					},
				},
				{
					Name: "Open plugin directory",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						openErr := util.ShellOpen(lp.metadata.Directory)
						if openErr != nil {
							w.api.Notify(ctx, "Failed to open plugin directory", openErr.Error())
						}
					},
				},
				{
					Name: "Remove",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						w.localPluginDirectories = lo.Filter(w.localPluginDirectories, func(directory string, _ int) bool {
							return directory != lp.metadata.Directory
						})
						w.saveLocalPluginDirectories(ctx)
					},
				},
				{
					Name: "Remove and delete plugin directory",
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
			Title: "Reload all dev plugins",
			Icon:  wpmIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "Reload",
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
	pluginDirectories := plugin.GetPluginManager().GetUI().PickFiles(ctx, share.PickFilesParams{IsDirectory: true})
	if len(pluginDirectories) == 0 {
		w.api.Notify(ctx, "Please choose a directory", "You need to choose a directory to add local plugin")
		return []plugin.QueryResult{}
	}

	pluginDirectory := pluginDirectories[0]

	if lo.Contains(w.localPluginDirectories, pluginDirectory) {
		w.api.Notify(ctx, "Already added", "The plugin directory has already been added")
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
		w.api.Notify(ctx, "Please input a directory", "You need to input a directory to remove local plugin")
		return []plugin.QueryResult{}
	}

	pluginDirectory := query.Search
	if !lo.Contains(w.localPluginDirectories, pluginDirectory) {
		w.api.Notify(ctx, "Not found", "The plugin directory is not found")
		return []plugin.QueryResult{}
	}

	w.localPluginDirectories = lo.Filter(w.localPluginDirectories, func(directory string, _ int) bool {
		return directory != pluginDirectory
	})
	w.saveLocalPluginDirectories(ctx)
	return []plugin.QueryResult{}
}

func (w *WPMPlugin) createPlugin(ctx context.Context, template pluginTemplate, pluginName string, query plugin.Query) {
	w.creatingProcess = "Downloading template..."

	tempPluginDirectory := path.Join(os.TempDir(), uuid.NewString())
	if err := util.GetLocation().EnsureDirectoryExist(tempPluginDirectory); err != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to create temp plugin directory: %s", err.Error()))
		w.creatingProcess = fmt.Sprintf("Failed to create temp plugin directory: %s", err.Error())
		return
	}

	w.creatingProcess = fmt.Sprintf("Downloading %s template to %s", template.Runtime, tempPluginDirectory)
	tempZipPath := path.Join(tempPluginDirectory, "template.zip")
	err := util.HttpDownload(ctx, template.Url, tempZipPath)
	if err != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to download template: %s", err.Error()))
		w.creatingProcess = fmt.Sprintf("Failed to download template: %s", err.Error())
		return
	}

	w.creatingProcess = "Extracting template..."
	err = util.Unzip(tempZipPath, tempPluginDirectory)
	if err != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to extract template: %s", err.Error()))
		w.creatingProcess = fmt.Sprintf("Failed to extract template: %s", err.Error())
		return
	}

	w.creatingProcess = "Please choose a directory..."
	pluginDirectories := plugin.GetPluginManager().GetUI().PickFiles(ctx, share.PickFilesParams{IsDirectory: true})
	if len(pluginDirectories) == 0 {
		w.api.Notify(ctx, "Please choose a directory", "You need to choose a directory to create the plugin")
		return
	}
	pluginDirectory := path.Join(pluginDirectories[0], pluginName)
	w.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Creating plugin in directory: %s", pluginDirectory))

	cpErr := cp.Copy(path.Join(tempPluginDirectory, template.Name+"-main"), pluginDirectory)
	if cpErr != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to copy template: %s", cpErr.Error()))
		w.creatingProcess = fmt.Sprintf("Failed to copy template: %s", cpErr.Error())
		return
	}

	// replace variables in plugin.json
	pluginJsonPath := path.Join(pluginDirectory, "plugin.json")
	pluginJson, readErr := os.ReadFile(pluginJsonPath)
	if readErr != nil {
		w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to read plugin.json: %s", readErr.Error()))
		w.creatingProcess = fmt.Sprintf("Failed to read plugin.json: %s", readErr.Error())
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
		w.creatingProcess = fmt.Sprintf("Failed to write plugin.json: %s", writeErr.Error())
		return
	}

	// replace variables in package.json
	if template.Runtime == plugin.PLUGIN_RUNTIME_NODEJS {
		packageJsonPath := path.Join(pluginDirectory, "package.json")
		packageJson, readPackageErr := os.ReadFile(packageJsonPath)
		if readPackageErr != nil {
			w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to read package.json: %s", readPackageErr.Error()))
			w.creatingProcess = fmt.Sprintf("Failed to read package.json: %s", readPackageErr.Error())
			return
		}

		packageJsonString := string(packageJson)
		packageName := strings.ReplaceAll(strings.ToLower(pluginName), ".", "_")
		packageJsonString = strings.ReplaceAll(packageJsonString, "replace_me_with_name", packageName)

		writePackageErr := os.WriteFile(packageJsonPath, []byte(packageJsonString), 0644)
		if writePackageErr != nil {
			w.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to write package.json: %s", writePackageErr.Error()))
			w.creatingProcess = fmt.Sprintf("Failed to write package.json: %s", writePackageErr.Error())
			return
		}
	}

	w.creatingProcess = ""
	w.localPluginDirectories = append(w.localPluginDirectories, pluginDirectory)
	w.saveLocalPluginDirectories(ctx)
	w.loadDevPlugin(ctx, pluginDirectory)
	w.api.ChangeQuery(ctx, share.PlainQuery{
		QueryType: plugin.QueryTypeInput,
		QueryText: fmt.Sprintf("%s dev ", query.TriggerKeyword),
	})
}

func (w *WPMPlugin) saveLocalPluginDirectories(ctx context.Context) {
	data, marshalErr := json.Marshal(w.localPluginDirectories)
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

	w.api.Notify(ctx, "Reloaded dev plugin", fmt.Sprintf("%s(%s)", localPlugin.Metadata.Name, reason))
	return nil
}
