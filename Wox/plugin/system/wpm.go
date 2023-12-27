package system

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	cp "github.com/otiai10/copy"
	"github.com/samber/lo"
	"os"
	"path"
	"strings"
	"wox/plugin"
	"wox/share"
	"wox/util"
)

var wpmIcon = plugin.NewWoxImageSvg(`<svg t="1697178225584" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="16738" width="200" height="200"><path d="M842.99 884.364H181.01c-22.85 0-41.374-18.756-41.374-41.892V181.528c0-23.136 18.524-41.892 41.374-41.892h661.98c22.85 0 41.374 18.756 41.374 41.892v660.944c0 23.136-18.524 41.892-41.374 41.892z" fill="#9C34FE" p-id="16739" data-spm-anchor-id="a313x.search_index.0.i6.1f873a81xqBP8f"></path><path d="M387.88 307.2h-82.748v83.78c0 115.68 92.618 209.456 206.868 209.456s206.868-93.776 206.868-209.454V307.2h-82.746v83.78c0 69.408-55.572 125.674-124.122 125.674s-124.12-56.266-124.12-125.672V307.2z" fill="#FFFFFF" p-id="16740"></path></svg>`)
var pluginTemplates = []pluginTemplate{
	{
		Runtime: plugin.PLUGIN_RUNTIME_NODEJS,
		Name:    "Wox.Plugin.Template.Nodejs",
		Url:     "https://codeload.github.com/Wox-launcher/Wox.Plugin.Template.Nodejs/zip/refs/heads/main",
	},
}

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &WPMPlugin{})
}

type WPMPlugin struct {
	api                    plugin.API
	creatingProcess        string
	localPluginDirectories []string
	localPlugins           []plugin.MetadataWithDirectory
}

type pluginTemplate struct {
	Runtime plugin.Runtime
	Name    string
	Url     string
}

func (i *WPMPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "e2c5f005-6c73-43c8-bc53-ab04def265b2",
		Name:          "Wox Plugin Manager",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Nodejs",
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
				Command:     "local",
				Description: "List local Wox plugins",
			},
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (i *WPMPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	i.api = initParams.API

	localPluginDirs := i.api.GetSetting(ctx, "localPluginDirectories")
	if localPluginDirs != "" {
		unmarshalErr := json.Unmarshal([]byte(localPluginDirs), &i.localPluginDirectories)
		if unmarshalErr != nil {
			i.api.Log(ctx, fmt.Sprintf("Failed to unmarshal local plugin directories: %s", unmarshalErr.Error()))
		}
	}

	// remove invalid directories
	i.localPluginDirectories = lo.Filter(i.localPluginDirectories, func(directory string, _ int) bool {
		_, statErr := os.Stat(directory)
		if statErr != nil {
			i.api.Log(ctx, fmt.Sprintf("Failed to stat local plugin directory, remove it: %s", statErr.Error()))
			return false
		}

		return true
	})

	i.saveLocalPluginDirectories(ctx)
}

func (i *WPMPlugin) loadLocalPlugins(ctx context.Context) {
	i.localPlugins = nil
	for _, localPluginDirectory := range i.localPluginDirectories {
		p, err := i.loadLocalPluginsFromDirectory(ctx, localPluginDirectory)
		if err != nil {
			i.api.Log(ctx, err.Error())
			continue
		}

		i.api.Log(ctx, fmt.Sprintf("Loaded local plugin: %s", p.Metadata.Name))
		i.localPlugins = append(i.localPlugins, p)
	}
}

func (i *WPMPlugin) loadLocalPluginsFromDirectory(ctx context.Context, directory string) (plugin.MetadataWithDirectory, error) {
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

func (i *WPMPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult

	if query.Command == "create" {
		if i.creatingProcess != "" {
			results = append(results, plugin.QueryResult{
				Id:              uuid.NewString(),
				Title:           i.creatingProcess,
				SubTitle:        "Please wait...",
				Icon:            wpmIcon,
				RefreshInterval: 300,
				OnRefresh: func(current plugin.RefreshableResult) plugin.RefreshableResult {
					current.Title = i.creatingProcess
					return current
				},
			})
			return results
		}

		for _, template := range pluginTemplates {
			// action will be executed in another go routine, so we need to copy the variable
			pluginTemplateDummy := template
			results = append(results, plugin.QueryResult{
				Id:       uuid.NewString(),
				Title:    "Create " + string(pluginTemplateDummy.Runtime) + " plugin",
				SubTitle: fmt.Sprintf("Name: %s", query.Search),
				Icon:     wpmIcon,
				Actions: []plugin.QueryResultAction{
					{
						Name:                   "create",
						PreventHideAfterAction: true,
						Action: func(actionContext plugin.ActionContext) {
							pluginName := query.Search
							util.Go(ctx, "create plugin", func() {
								i.createPlugin(ctx, pluginTemplateDummy, pluginName, query)
							})
							i.api.ChangeQuery(ctx, share.ChangedQuery{
								QueryType: plugin.QueryTypeInput,
								QueryText: fmt.Sprintf("%s create ", query.TriggerKeyword),
							})
						},
					},
				}})
		}
	}

	if query.Command == "local" {
		//list all local plugins
		return lo.Map(i.localPlugins, func(metadataWithDirectory plugin.MetadataWithDirectory, _ int) plugin.QueryResult {
			iconImage := plugin.ParseWoxImageOrDefault(metadataWithDirectory.Metadata.Icon, wpmIcon)
			iconImage = plugin.ConvertIcon(ctx, iconImage, metadataWithDirectory.Directory)

			return plugin.QueryResult{
				Id:       uuid.NewString(),
				Title:    metadataWithDirectory.Metadata.Name,
				SubTitle: metadataWithDirectory.Metadata.Description,
				Icon:     iconImage,
				Preview: plugin.WoxPreview{
					PreviewType: plugin.WoxPreviewTypeMarkdown,
					PreviewData: fmt.Sprintf(`
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
`, metadataWithDirectory.Metadata.Name, metadataWithDirectory.Metadata.Description, metadataWithDirectory.Metadata.Author,
						metadataWithDirectory.Metadata.Website, metadataWithDirectory.Metadata.Version, metadataWithDirectory.Metadata.MinWoxVersion,
						metadataWithDirectory.Metadata.Runtime, metadataWithDirectory.Metadata.Entry, metadataWithDirectory.Metadata.TriggerKeywords,
						metadataWithDirectory.Metadata.Commands, metadataWithDirectory.Metadata.SupportedOS, metadataWithDirectory.Metadata.Features),
				},
				Actions: []plugin.QueryResultAction{
					{
						Name: "open",
						Action: func(actionContext plugin.ActionContext) {
							openErr := util.ShellOpen(metadataWithDirectory.Directory)
							if openErr != nil {
								i.api.ShowMsg(ctx, "Failed to open plugin directory", openErr.Error(), wpmIcon.String())
							}
						},
					},
					{
						Name: "Remove",
						Action: func(actionContext plugin.ActionContext) {
							i.localPluginDirectories = lo.Filter(i.localPluginDirectories, func(directory string, _ int) bool {
								return directory != metadataWithDirectory.Directory
							})
							i.saveLocalPluginDirectories(ctx)
						},
					},
					{
						Name: "Remove and delete plugin directory",
						Action: func(actionContext plugin.ActionContext) {
							deleteErr := os.RemoveAll(metadataWithDirectory.Directory)
							if deleteErr != nil {
								i.api.Log(ctx, fmt.Sprintf("Failed to delete plugin directory: %s", deleteErr.Error()))
								return
							}

							i.localPluginDirectories = lo.Filter(i.localPluginDirectories, func(directory string, _ int) bool {
								return directory != metadataWithDirectory.Directory
							})
							i.saveLocalPluginDirectories(ctx)
						},
					},
				},
			}
		})
	}

	if query.Command == "install" {
		if query.Search == "" {
			//TODO: return featured plugins
			return results
		}

		pluginManifests := plugin.GetStoreManager().Search(ctx, query.Search)
		for _, pluginManifestShadow := range pluginManifests {
			// action will be executed in another go routine, so we need to copy the variable
			pluginManifest := pluginManifestShadow
			results = append(results, plugin.QueryResult{
				Id:       uuid.NewString(),
				Title:    pluginManifest.Name,
				SubTitle: pluginManifest.Description,
				Icon:     plugin.NewWoxImageUrl(pluginManifest.IconUrl),
				Actions: []plugin.QueryResultAction{
					{
						Name: "install",
						Action: func(actionContext plugin.ActionContext) {
							plugin.GetStoreManager().Install(ctx, pluginManifest)
						},
					},
				}})
		}
	}

	if query.Command == "uninstall" {
		plugins := plugin.GetPluginManager().GetPluginInstances()
		if query.Search != "" {
			plugins = lo.Filter(plugins, func(pluginInstance *plugin.Instance, _ int) bool {
				return IsStringMatchNoPinYin(ctx, pluginInstance.Metadata.Name, query.Search)
			})
		}

		results = lo.Map(plugins, func(pluginInstanceShadow *plugin.Instance, _ int) plugin.QueryResult {
			// action will be executed in another go routine, so we need to copy the variable
			pluginInstance := pluginInstanceShadow
			return plugin.QueryResult{
				Id:       uuid.NewString(),
				Title:    pluginInstance.Metadata.Name,
				SubTitle: pluginInstance.Metadata.Description,
				Icon:     plugin.ParseWoxImageOrDefault(pluginInstance.Metadata.Icon, wpmIcon),
				Actions: []plugin.QueryResultAction{
					{
						Name: "uninstall",
						Action: func(actionContext plugin.ActionContext) {
							plugin.GetStoreManager().Uninstall(ctx, pluginInstance)
						},
					},
				},
			}
		})
	}

	return results
}

func (i *WPMPlugin) createPlugin(ctx context.Context, template pluginTemplate, pluginName string, query plugin.Query) {
	i.creatingProcess = "Downloading template..."

	tempPluginDirectory := path.Join(os.TempDir(), uuid.NewString())
	if err := util.GetLocation().EnsureDirectoryExist(tempPluginDirectory); err != nil {
		i.api.Log(ctx, fmt.Sprintf("Failed to create temp plugin directory: %s", err.Error()))
		i.creatingProcess = fmt.Sprintf("Failed to create temp plugin directory: %s", err.Error())
		return
	}

	i.creatingProcess = fmt.Sprintf("Downloading %s template to %s", template.Runtime, tempPluginDirectory)
	tempZipPath := path.Join(tempPluginDirectory, "template.zip")
	err := util.HttpDownload(ctx, template.Url, tempZipPath)
	if err != nil {
		i.api.Log(ctx, fmt.Sprintf("Failed to download template: %s", err.Error()))
		i.creatingProcess = fmt.Sprintf("Failed to download template: %s", err.Error())
		return
	}

	i.creatingProcess = "Extracting template..."
	err = util.Unzip(tempZipPath, tempPluginDirectory)
	if err != nil {
		i.api.Log(ctx, fmt.Sprintf("Failed to extract template: %s", err.Error()))
		i.creatingProcess = fmt.Sprintf("Failed to extract template: %s", err.Error())
		return
	}

	// TODO: let user choose the directory
	pluginDirectory := path.Join(util.GetLocation().GetPluginDirectory(), pluginName)
	cpErr := cp.Copy(path.Join(tempPluginDirectory, template.Name+"-main"), pluginDirectory)
	if cpErr != nil {
		i.api.Log(ctx, fmt.Sprintf("Failed to copy template: %s", cpErr.Error()))
		i.creatingProcess = fmt.Sprintf("Failed to copy template: %s", cpErr.Error())
		return
	}

	// replace variables in plugin.json
	pluginJsonPath := path.Join(pluginDirectory, "plugin.json")
	pluginJson, readErr := os.ReadFile(pluginJsonPath)
	if readErr != nil {
		i.api.Log(ctx, fmt.Sprintf("Failed to read plugin.json: %s", readErr.Error()))
		i.creatingProcess = fmt.Sprintf("Failed to read plugin.json: %s", readErr.Error())
		return
	}

	pluginJsonString := string(pluginJson)
	pluginJsonString = strings.ReplaceAll(pluginJsonString, "[Id]", uuid.NewString())
	pluginJsonString = strings.ReplaceAll(pluginJsonString, "[Name]", pluginName)
	pluginJsonString = strings.ReplaceAll(pluginJsonString, "[Runtime]", strings.ToLower(string(template.Runtime)))
	pluginJsonString = strings.ReplaceAll(pluginJsonString, "[Trigger Keyword]", "np")

	writeErr := os.WriteFile(pluginJsonPath, []byte(pluginJsonString), 0644)
	if writeErr != nil {
		i.api.Log(ctx, fmt.Sprintf("Failed to write plugin.json: %s", writeErr.Error()))
		i.creatingProcess = fmt.Sprintf("Failed to write plugin.json: %s", writeErr.Error())
		return
	}

	// replace variables in package.json
	if template.Runtime == plugin.PLUGIN_RUNTIME_NODEJS {
		packageJsonPath := path.Join(pluginDirectory, "package.json")
		packageJson, readPackageErr := os.ReadFile(packageJsonPath)
		if readPackageErr != nil {
			i.api.Log(ctx, fmt.Sprintf("Failed to read package.json: %s", readPackageErr.Error()))
			i.creatingProcess = fmt.Sprintf("Failed to read package.json: %s", readPackageErr.Error())
			return
		}

		packageJsonString := string(packageJson)
		packageJsonString = strings.ReplaceAll(packageJsonString, "replace_me_with_name", pluginName)

		writePackageErr := os.WriteFile(packageJsonPath, []byte(packageJsonString), 0644)
		if writePackageErr != nil {
			i.api.Log(ctx, fmt.Sprintf("Failed to write package.json: %s", writePackageErr.Error()))
			i.creatingProcess = fmt.Sprintf("Failed to write package.json: %s", writePackageErr.Error())
			return
		}
	}

	i.creatingProcess = ""
	i.localPluginDirectories = append(i.localPluginDirectories, pluginDirectory)
	i.saveLocalPluginDirectories(ctx)
	i.api.ChangeQuery(ctx, share.ChangedQuery{
		QueryType: plugin.QueryTypeInput,
		QueryText: fmt.Sprintf("%s local ", query.TriggerKeyword),
	})
}

func (i *WPMPlugin) saveLocalPluginDirectories(ctx context.Context) {
	data, marshalErr := json.Marshal(i.localPluginDirectories)
	if marshalErr != nil {
		i.api.Log(ctx, fmt.Sprintf("Failed to marshal local plugin directories: %s", marshalErr.Error()))
		return
	}
	i.api.SaveSetting(ctx, "localPluginDirectories", string(data), false)
	i.loadLocalPlugins(ctx)
}
