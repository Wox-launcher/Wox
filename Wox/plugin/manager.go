package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"wox/ai"
	"wox/i18n"
	"wox/setting"
	"wox/share"
	"wox/util"
	"wox/util/notifier"

	"github.com/Masterminds/semver/v3"
	"github.com/google/uuid"
	"github.com/jinzhu/copier"
	"github.com/samber/lo"
	"github.com/wissance/stringFormatter"
)

var managerInstance *Manager
var managerOnce sync.Once
var logger *util.Log

type debounceTimer struct {
	timer  *time.Timer
	onStop func()
}

type Manager struct {
	instances          []*Instance
	ui                 share.UI
	resultCache        *util.HashMap[string, *QueryResultCache]
	debounceQueryTimer *util.HashMap[string, *debounceTimer]
	aiProviders        *util.HashMap[ai.ProviderName, ai.Provider]

	activeBrowserUrl string //active browser url before wox is activated
}

func GetPluginManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = &Manager{
			resultCache:        util.NewHashMap[string, *QueryResultCache](),
			debounceQueryTimer: util.NewHashMap[string, *debounceTimer](),
			aiProviders:        util.NewHashMap[ai.ProviderName, ai.Provider](),
		}
		logger = util.GetLogger()
	})
	return managerInstance
}

func (m *Manager) Start(ctx context.Context, ui share.UI) error {
	m.ui = ui

	loadErr := m.loadPlugins(ctx)
	if loadErr != nil {
		return fmt.Errorf("failed to load plugins: %w", loadErr)
	}

	util.Go(ctx, "start store manager", func() {
		GetStoreManager().Start(util.NewTraceContext())
	})

	return nil
}

func (m *Manager) Stop(ctx context.Context) {
	for _, host := range AllHosts {
		host.Stop(ctx)
	}
}

func (m *Manager) SetActiveBrowserUrl(url string) {
	m.activeBrowserUrl = url
}

func (m *Manager) loadPlugins(ctx context.Context) error {
	logger.Info(ctx, "start loading plugins")

	// load system plugin first
	m.loadSystemPlugins(ctx)

	logger.Debug(ctx, "start loading user plugin metadata")
	basePluginDirectory := util.GetLocation().GetPluginDirectory()
	pluginDirectories, readErr := os.ReadDir(basePluginDirectory)
	if readErr != nil {
		return fmt.Errorf("failed to read plugin directory: %w", readErr)
	}

	var metaDataList []MetadataWithDirectory
	for _, entry := range pluginDirectories {
		if entry.Name() == ".DS_Store" {
			continue
		}
		if !entry.IsDir() {
			continue
		}

		pluginDirectory := path.Join(basePluginDirectory, entry.Name())
		metadata, metadataErr := m.ParseMetadata(ctx, pluginDirectory)
		if metadataErr != nil {
			logger.Error(ctx, metadataErr.Error())
			continue
		}

		//check if metadata already exist, only add newer version
		existMetadata, exist := lo.Find(metaDataList, func(item MetadataWithDirectory) bool {
			return item.Metadata.Id == metadata.Id
		})
		if exist {
			existVersion, existVersionErr := semver.NewVersion(existMetadata.Metadata.Version)
			currentVersion, currentVersionErr := semver.NewVersion(metadata.Version)
			if existVersionErr == nil && currentVersionErr == nil {
				if existVersion.GreaterThan(currentVersion) || existVersion.Equal(currentVersion) {
					logger.Info(ctx, fmt.Sprintf("skip parse %s(%s) metadata, because it's already parsed(%s)", metadata.Name, metadata.Version, existMetadata.Metadata.Version))
					continue
				} else {
					// remove older version
					logger.Info(ctx, fmt.Sprintf("remove older metadata version %s(%s)", existMetadata.Metadata.Name, existMetadata.Metadata.Version))
					var newMetaDataList []MetadataWithDirectory
					for _, item := range metaDataList {
						if item.Metadata.Id != existMetadata.Metadata.Id {
							newMetaDataList = append(newMetaDataList, item)
						}
					}
					metaDataList = newMetaDataList
				}
			}
		}
		metaDataList = append(metaDataList, MetadataWithDirectory{Metadata: metadata, Directory: pluginDirectory})
	}
	logger.Info(ctx, fmt.Sprintf("start loading user plugins, found %d user plugins", len(metaDataList)))

	for _, h := range AllHosts {
		host := h
		util.Go(ctx, fmt.Sprintf("[%s] start host", host.GetRuntime(ctx)), func() {
			newCtx := util.NewTraceContext()
			hostErr := host.Start(newCtx)
			if hostErr != nil {
				logger.Error(newCtx, fmt.Errorf("[%s HOST] %w", host.GetRuntime(newCtx), hostErr).Error())
				return
			}

			for _, metadata := range metaDataList {
				if strings.ToUpper(metadata.Metadata.Runtime) != strings.ToUpper(string(host.GetRuntime(newCtx))) {
					continue
				}

				loadErr := m.loadHostPlugin(newCtx, host, metadata)
				if loadErr != nil {
					logger.Error(newCtx, fmt.Errorf("[%s HOST] %w", host.GetRuntime(newCtx), loadErr).Error())
					continue
				}
			}
		})
	}

	return nil
}

func (m *Manager) ReloadPlugin(ctx context.Context, metadata MetadataWithDirectory) error {
	logger.Info(ctx, fmt.Sprintf("start reloading dev plugin: %s", metadata.Metadata.Name))

	pluginHost, exist := lo.Find(AllHosts, func(item Host) bool {
		return strings.ToLower(string(item.GetRuntime(ctx))) == strings.ToLower(metadata.Metadata.Runtime)
	})
	if !exist {
		return fmt.Errorf("unsupported runtime: %s", metadata.Metadata.Runtime)
	}

	pluginInstance, pluginInstanceExist := lo.Find(m.instances, func(item *Instance) bool {
		return item.Metadata.Id == metadata.Metadata.Id
	})
	if pluginInstanceExist {
		logger.Info(ctx, fmt.Sprintf("plugin(%s) is loaded, unload first", metadata.Metadata.Name))
		m.UnloadPlugin(ctx, pluginInstance)
	} else {
		logger.Info(ctx, fmt.Sprintf("plugin(%s) is not loaded, skip unload", metadata.Metadata.Name))
	}

	loadErr := m.loadHostPlugin(ctx, pluginHost, metadata)
	if loadErr != nil {
		return loadErr
	}

	return nil
}

func (m *Manager) loadHostPlugin(ctx context.Context, host Host, metadata MetadataWithDirectory) error {
	loadStartTimestamp := util.GetSystemTimestamp()
	plugin, loadErr := host.LoadPlugin(ctx, metadata.Metadata, metadata.Directory)
	if loadErr != nil {
		logger.Error(ctx, fmt.Errorf("[%s HOST] failed to load plugin: %w", host.GetRuntime(ctx), loadErr).Error())
		return loadErr
	}
	loadFinishTimestamp := util.GetSystemTimestamp()

	instance := &Instance{
		Metadata:              metadata.Metadata,
		PluginDirectory:       metadata.Directory,
		Plugin:                plugin,
		Host:                  host,
		LoadStartTimestamp:    loadStartTimestamp,
		LoadFinishedTimestamp: loadFinishTimestamp,
		IsDevPlugin:           metadata.IsDev,
		DevPluginDirectory:    metadata.DevPluginDirectory,
	}
	instance.API = NewAPI(instance)
	pluginSetting, settingErr := setting.GetSettingManager().LoadPluginSetting(ctx, metadata.Metadata.Id, metadata.Metadata.Name, metadata.Metadata.SettingDefinitions)
	if settingErr != nil {
		instance.API.Log(ctx, LogLevelError, fmt.Errorf("[SYS] failed to load plugin[%s] setting: %w", metadata.Metadata.Name, settingErr).Error())
		return settingErr
	}
	instance.Setting = pluginSetting

	m.instances = append(m.instances, instance)

	if pluginSetting.Disabled {
		logger.Info(ctx, fmt.Errorf("[%s HOST] plugin is disabled by user, skip init: %s", host.GetRuntime(ctx), metadata.Metadata.Name).Error())
		instance.API.Log(ctx, LogLevelWarning, fmt.Sprintf("[SYS] plugin is disabled by user, skip init: %s", metadata.Metadata.Name))
		return nil
	}

	util.Go(ctx, fmt.Sprintf("[%s] init plugin", metadata.Metadata.Name), func() {
		m.initPlugin(ctx, instance)
	})

	return nil
}

func (m *Manager) LoadPlugin(ctx context.Context, pluginDirectory string) error {
	metadata, parseErr := m.ParseMetadata(ctx, pluginDirectory)
	if parseErr != nil {
		return parseErr
	}

	pluginHost, exist := lo.Find(AllHosts, func(item Host) bool {
		return strings.ToLower(string(item.GetRuntime(ctx))) == strings.ToLower(metadata.Runtime)
	})
	if !exist {
		return fmt.Errorf("unsupported runtime: %s", metadata.Runtime)
	}

	loadErr := m.loadHostPlugin(ctx, pluginHost, MetadataWithDirectory{Metadata: metadata, Directory: pluginDirectory})
	if loadErr != nil {
		return loadErr
	}

	return nil
}

func (m *Manager) UnloadPlugin(ctx context.Context, pluginInstance *Instance) {
	for _, callback := range pluginInstance.UnloadCallbacks {
		callback()
	}
	pluginInstance.Host.UnloadPlugin(ctx, pluginInstance.Metadata)

	var newInstances []*Instance
	for _, instance := range m.instances {
		if instance.Metadata.Id != pluginInstance.Metadata.Id {
			newInstances = append(newInstances, instance)
		}
	}
	m.instances = newInstances
}

func (m *Manager) loadSystemPlugins(ctx context.Context) {
	start := util.GetSystemTimestamp()
	logger.Info(ctx, fmt.Sprintf("start loading system plugins, found %d system plugins", len(AllSystemPlugin)))

	for _, plugin := range AllSystemPlugin {
		util.Go(ctx, fmt.Sprintf("load system plugin <%s>", plugin.GetMetadata().Name), func() {
			metadata := plugin.GetMetadata()
			instance := &Instance{
				Metadata:              metadata,
				Plugin:                plugin,
				Host:                  nil,
				IsSystemPlugin:        true,
				LoadStartTimestamp:    util.GetSystemTimestamp(),
				LoadFinishedTimestamp: util.GetSystemTimestamp(),
			}
			instance.API = NewAPI(instance)

			startTimestamp := util.GetSystemTimestamp()
			pluginSetting, settingErr := setting.GetSettingManager().LoadPluginSetting(ctx, metadata.Id, metadata.Name, metadata.SettingDefinitions)
			if settingErr != nil {
				errMsg := fmt.Sprintf("failed to load system plugin[%s] setting, use default plugin setting. err: %s", metadata.Name, settingErr.Error())
				logger.Error(ctx, errMsg)
				instance.API.Log(ctx, LogLevelError, fmt.Sprintf("[SYS] %s", errMsg))
				pluginSetting = &setting.PluginSetting{
					Settings: util.NewHashMap[string, string](),
				}
			}
			instance.Setting = pluginSetting
			if util.GetSystemTimestamp()-startTimestamp > 100 {
				logger.Warn(ctx, fmt.Sprintf("load system plugin[%s] setting too slow, cost %d ms", metadata.Name, util.GetSystemTimestamp()-startTimestamp))
			}

			m.instances = append(m.instances, instance)

			m.initPlugin(util.NewTraceContext(), instance)
		})
	}

	logger.Debug(ctx, fmt.Sprintf("finish loading system plugins, cost %d ms", util.GetSystemTimestamp()-start))
}

func (m *Manager) initPlugin(ctx context.Context, instance *Instance) {
	logger.Info(ctx, fmt.Sprintf("start init plugin: %s", instance.Metadata.Name))
	instance.InitStartTimestamp = util.GetSystemTimestamp()
	instance.Plugin.Init(ctx, InitParams{
		API:             instance.API,
		PluginDirectory: instance.PluginDirectory,
	})
	instance.InitFinishedTimestamp = util.GetSystemTimestamp()
	logger.Info(ctx, fmt.Sprintf("init plugin %s finished, cost %d ms", instance.Metadata.Name, instance.InitFinishedTimestamp-instance.InitStartTimestamp))
}

func (m *Manager) ParseMetadata(ctx context.Context, pluginDirectory string) (Metadata, error) {
	configPath := path.Join(pluginDirectory, "plugin.json")
	if _, statErr := os.Stat(configPath); statErr != nil {
		return Metadata{}, fmt.Errorf("missing plugin.json file in %s", configPath)
	}

	metadataJson, err := os.ReadFile(configPath)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to read plugin.json file: %w", err)
	}

	var metadata Metadata
	unmarshalErr := json.Unmarshal(metadataJson, &metadata)
	if unmarshalErr != nil {
		return Metadata{}, fmt.Errorf("failed to unmarshal plugin.json file (%s): %w", pluginDirectory, unmarshalErr)
	}

	if len(metadata.TriggerKeywords) == 0 {
		return Metadata{}, fmt.Errorf("missing trigger keywords in plugin.json file (%s)", pluginDirectory)
	}
	if !IsSupportedRuntime(metadata.Runtime) {
		return Metadata{}, fmt.Errorf("unsupported runtime in plugin.json file (%s), runtime=%s", pluginDirectory, metadata.Runtime)
	}
	if !IsSupportedOSAny(metadata.SupportedOS) {
		return Metadata{}, fmt.Errorf("unsupported os in plugin.json file (%s), os=%s", pluginDirectory, metadata.SupportedOS)
	}

	return metadata, nil
}

func (m *Manager) GetPluginInstances() []*Instance {
	return m.instances
}

func (m *Manager) canOperateQuery(ctx context.Context, pluginInstance *Instance, query Query) bool {
	if pluginInstance.Setting.Disabled {
		return false
	}

	if query.Type == QueryTypeSelection {
		isPluginSupportSelection := pluginInstance.Metadata.IsSupportFeature(MetadataFeatureQuerySelection)
		return isPluginSupportSelection
	}

	var validGlobalQuery = lo.Contains(pluginInstance.GetTriggerKeywords(), "*") && query.TriggerKeyword == ""
	var validNonGlobalQuery = lo.Contains(pluginInstance.GetTriggerKeywords(), query.TriggerKeyword)
	if !validGlobalQuery && !validNonGlobalQuery {
		return false
	}

	return true
}

func (m *Manager) queryForPlugin(ctx context.Context, pluginInstance *Instance, query Query) (results []QueryResult) {
	defer util.GoRecover(ctx, fmt.Sprintf("<%s> query panic", pluginInstance.Metadata.Name), func(err error) {
		// if plugin query panic, return error result
		failedResult := m.GetResultForFailedQuery(ctx, pluginInstance.Metadata, query, err)
		results = []QueryResult{
			m.PolishResult(ctx, pluginInstance, query, failedResult),
		}
	})

	logger.Info(ctx, fmt.Sprintf("<%s> start query: %s", pluginInstance.Metadata.Name, query.RawQuery))
	start := util.GetSystemTimestamp()

	// set query env base on plugin's feature
	currentEnv := query.Env
	newEnv := QueryEnv{}
	if pluginInstance.Metadata.IsSupportFeature(MetadataFeatureQueryEnv) {
		queryEnvParams, err := pluginInstance.Metadata.GetFeatureParamsForQueryEnv()
		if err != nil {
			logger.Error(ctx, fmt.Sprintf("<%s> invalid query env config: %s", pluginInstance.Metadata.Name, err))
		} else {
			if queryEnvParams.RequireActiveWindowName {
				newEnv.ActiveWindowTitle = currentEnv.ActiveWindowTitle
			}
			if queryEnvParams.RequireActiveWindowPid {
				newEnv.ActiveWindowPid = currentEnv.ActiveWindowPid
			}
			if queryEnvParams.RequireActiveBrowserUrl {
				newEnv.ActiveBrowserUrl = currentEnv.ActiveBrowserUrl
			}
		}
	}
	query.Env = newEnv

	results = pluginInstance.Plugin.Query(ctx, query)
	logger.Debug(ctx, fmt.Sprintf("<%s> finish query, result count: %d, cost: %dms", pluginInstance.Metadata.Name, len(results), util.GetSystemTimestamp()-start))

	for i := range results {
		results[i] = m.addDefaultActions(ctx, pluginInstance, query, results[i])
		results[i] = m.PolishResult(ctx, pluginInstance, query, results[i])
	}

	if query.Type == QueryTypeSelection && query.Search != "" {
		woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
		results = lo.Filter(results, func(item QueryResult, _ int) bool {
			match, _ := util.IsStringMatchScore(item.Title, query.Search, woxSetting.UsePinYin)
			return match
		})
	}

	return results
}

func (m *Manager) GetResultForFailedQuery(ctx context.Context, pluginMetadata Metadata, query Query, err error) QueryResult {
	overlayIcon := NewWoxImageEmoji("🚫")
	pluginIcon := ParseWoxImageOrDefault(pluginMetadata.Icon, overlayIcon)
	icon := pluginIcon.OverlayFullPercentage(overlayIcon, 0.6)

	return QueryResult{
		Title:    fmt.Sprintf("%s query failed", pluginMetadata.Name),
		SubTitle: util.EllipsisEnd(err.Error(), 20),
		Icon:     icon,
		Preview: WoxPreview{
			PreviewType: WoxPreviewTypeText,
			PreviewData: err.Error(),
		},
	}
}

func (m *Manager) addDefaultActions(ctx context.Context, pluginInstance *Instance, query Query, result QueryResult) QueryResult {
	if query.Type == QueryTypeInput {
		if result.Group == "" {
			if setting.GetSettingManager().IsFavoriteResult(ctx, pluginInstance.Metadata.Id, result.Title, result.SubTitle) {
				// add "Remove from favorite" action
				result.Actions = append(result.Actions, QueryResultAction{
					Name: "Remove from favorite",
					Icon: RemoveFromFavIcon,
					Action: func(ctx context.Context, actionContext ActionContext) {
						setting.GetSettingManager().RemoveFavoriteResult(ctx, pluginInstance.Metadata.Id, result.Title, result.SubTitle)
					},
				})
			} else {
				// add "Add to favorite" action
				result.Actions = append(result.Actions, QueryResultAction{
					Name: "Add to favorite",
					Icon: AddToFavIcon,
					Action: func(ctx context.Context, actionContext ActionContext) {
						setting.GetSettingManager().AddFavoriteResult(ctx, pluginInstance.Metadata.Id, result.Title, result.SubTitle)
					},
				})
			}
		}
	}

	return result
}

func (m *Manager) PolishResult(ctx context.Context, pluginInstance *Instance, query Query, result QueryResult) QueryResult {
	// set default id
	if result.Id == "" {
		result.Id = uuid.NewString()
	}
	for actionIndex := range result.Actions {
		if result.Actions[actionIndex].Id == "" {
			result.Actions[actionIndex].Id = uuid.NewString()
		}
		if result.Actions[actionIndex].Icon.ImageType == "" {
			// set default action icon if not present
			result.Actions[actionIndex].Icon = DefaultActionIcon
		}
	}

	// convert icon
	result.Icon = ConvertIcon(ctx, result.Icon, pluginInstance.PluginDirectory)
	for i := range result.Tails {
		if result.Tails[i].Type == QueryResultTailTypeImage {
			result.Tails[i].Image = ConvertIcon(ctx, result.Tails[i].Image, pluginInstance.PluginDirectory)
		}
	}

	// add default preview for selection query if no preview is set
	if query.Type == QueryTypeSelection && result.Preview.PreviewType == "" {
		if query.Selection.Type == util.SelectionTypeText {
			result.Preview = WoxPreview{
				PreviewType: WoxPreviewTypeText,
				PreviewData: query.Selection.Text,
			}
		}
		if query.Selection.Type == util.SelectionTypeFile {
			result.Preview = WoxPreview{
				PreviewType: WoxPreviewTypeMarkdown,
				PreviewData: m.formatFileListPreview(ctx, query.Selection.FilePaths),
			}
		}
	}

	// translate title
	result.Title = m.translatePlugin(ctx, pluginInstance, result.Title)
	// translate subtitle
	result.SubTitle = m.translatePlugin(ctx, pluginInstance, result.SubTitle)
	// translate preview properties
	var previewProperties = make(map[string]string)
	for key, value := range result.Preview.PreviewProperties {
		translatedKey := m.translatePlugin(ctx, pluginInstance, key)
		previewProperties[translatedKey] = value
	}
	result.Preview.PreviewProperties = previewProperties
	// translate action names
	for actionIndex := range result.Actions {
		result.Actions[actionIndex].Name = m.translatePlugin(ctx, pluginInstance, result.Actions[actionIndex].Name)
	}

	// set first action as default if no default action is set
	defaultActionCount := lo.CountBy(result.Actions, func(item QueryResultAction) bool {
		return item.IsDefault
	})
	if defaultActionCount == 0 && len(result.Actions) > 0 {
		result.Actions[0].IsDefault = true
		result.Actions[0].Hotkey = "Enter"
	}

	var resultCache = &QueryResultCache{
		ResultId:       result.Id,
		ResultTitle:    result.Title,
		ResultSubTitle: result.SubTitle,
		ContextData:    result.ContextData,
		PluginInstance: pluginInstance,
		Query:          query,
		Actions:        util.NewHashMap[string, func(ctx context.Context, actionContext ActionContext)](),
	}

	// store actions for ui invoke later
	for actionIndex := range result.Actions {
		var action = result.Actions[actionIndex]

		// if default action's hotkey is empty, set it as Enter
		if action.IsDefault && action.Hotkey == "" {
			result.Actions[actionIndex].Hotkey = "Enter"
		}

		// replace hotkey modifiers for platform specific, E.g. replace win to cmd on macos, replace cmd to win on windows
		if util.IsMacOS() {
			result.Actions[actionIndex].Hotkey = strings.ReplaceAll(result.Actions[actionIndex].Hotkey, "win", "cmd")
			result.Actions[actionIndex].Hotkey = strings.ReplaceAll(result.Actions[actionIndex].Hotkey, "windows", "cmd")
			result.Actions[actionIndex].Hotkey = strings.ReplaceAll(result.Actions[actionIndex].Hotkey, "alt", "option")
		}
		if util.IsWindows() || util.IsLinux() {
			result.Actions[actionIndex].Hotkey = strings.ReplaceAll(result.Actions[actionIndex].Hotkey, "cmd", "win")
			result.Actions[actionIndex].Hotkey = strings.ReplaceAll(result.Actions[actionIndex].Hotkey, "command", "win")
			result.Actions[actionIndex].Hotkey = strings.ReplaceAll(result.Actions[actionIndex].Hotkey, "option", "alt")
		}

		if action.Action != nil {
			resultCache.Actions.Store(action.Id, action.Action)
		}
	}

	// if query is input and trigger keyword is global, disable preview and group
	if query.IsGlobalQuery() {
		result.Preview = WoxPreview{}
		result.Group = ""
		result.GroupScore = 0
	}

	// store preview for ui invoke later
	// because preview may contain some heavy data (E.g. image or large text), we will store preview in cache and only send preview to ui when user select the result
	if result.Preview.PreviewType != "" && result.Preview.PreviewType != WoxPreviewTypeRemote {
		resultCache.Preview = result.Preview
		result.Preview = WoxPreview{
			PreviewType: WoxPreviewTypeRemote,
			PreviewData: fmt.Sprintf("/preview?id=%s", result.Id),
		}
	}

	if result.RefreshInterval > 0 && result.OnRefresh != nil {
		newInterval := int(math.Floor(float64(result.RefreshInterval)/100) * 100)
		if result.RefreshInterval != newInterval {
			logger.Info(ctx, fmt.Sprintf("[%s] result(%s) refresh interval %d is not divisible by 100, use %d instead", pluginInstance.Metadata.Name, result.Id, result.RefreshInterval, newInterval))
			result.RefreshInterval = newInterval
		}
		resultCache.Refresh = result.OnRefresh
	}

	ignoreAutoScore := pluginInstance.Metadata.IsSupportFeature(MetadataFeatureIgnoreAutoScore)
	if !ignoreAutoScore {
		score := m.calculateResultScore(ctx, pluginInstance.Metadata.Id, result.Title, result.SubTitle)
		if score > 0 {
			logger.Debug(ctx, fmt.Sprintf("<%s> result(%s) add score: %d", pluginInstance.Metadata.Name, result.Title, score))
			result.Score += score
		}
	}

	m.resultCache.Store(result.Id, resultCache)

	return result
}

func (m *Manager) formatFileListPreview(ctx context.Context, filePaths []string) string {
	totalFiles := len(filePaths)
	if totalFiles == 0 {
		return i18n.GetI18nManager().TranslateWox(ctx, "i18n:selection_no_files_selected")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "i18n:selection_selected_files_count"), totalFiles))
	sb.WriteString("\n\n")

	maxDisplayFiles := 10
	for i, filePath := range filePaths {
		if i < maxDisplayFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", filePath))
		} else {
			remainingFiles := totalFiles - maxDisplayFiles
			sb.WriteString("\n")
			sb.WriteString(fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "i18n:selection_remaining_files_not_shown"), remainingFiles))
			break
		}
	}

	return sb.String()
}

func (m *Manager) calculateResultScore(ctx context.Context, pluginId, title, subTitle string) int64 {
	var score int64 = 0

	// check if result is favorite result
	if setting.GetSettingManager().IsFavoriteResult(ctx, pluginId, title, subTitle) {
		score += 100000
	}

	resultHash := setting.NewResultHash(pluginId, title, subTitle)
	woxAppData := setting.GetSettingManager().GetWoxAppData(ctx)
	actionResults, ok := woxAppData.ActionedResults.Load(resultHash)
	if !ok {
		return score
	}

	// actioned score are based on actioned counts, the more actioned, the more score
	// also, action timestamp will be considered, the more recent actioned, the more score weight. If action is in recent 7 days, it will be considered as recent actioned and add score weight
	// we will use fibonacci sequence to calculate score, the more recent actioned, the more score: 5, 8, 13, 21, 34, 55, 89
	// that means, actions in day one, we will add weight 89, day two, we will add weight 55, day three, we will add weight 34, and so on
	// E.g. if actioned 3 times in day one, 2 times in day two, 1 time in day three, the score will be: 89*3 + 55*2 + 34*1 = 450

	for _, actionResult := range actionResults {
		var weight int64 = 2

		actionedTime := util.ParseTimeStamp(actionResult.Timestamp)
		hours := util.GetSystemTime().Sub(actionedTime).Hours()
		if hours < 24*7 {
			fibonacciIndex := int(math.Ceil(hours / 24))
			if fibonacciIndex > 7 {
				fibonacciIndex = 7
			}
			if fibonacciIndex < 1 {
				fibonacciIndex = 1
			}
			fibonacci := []int64{5, 8, 13, 21, 34, 55, 89}
			score += fibonacci[7-fibonacciIndex]
		}

		score += weight
	}

	return score
}

func (m *Manager) PolishRefreshableResult(ctx context.Context, pluginInstance *Instance, resultId string, result RefreshableResult) RefreshableResult {
	// convert icon
	result.Icon = ConvertIcon(ctx, result.Icon, pluginInstance.PluginDirectory)
	for i := range result.Tails {
		if result.Tails[i].Type == QueryResultTailTypeImage {
			result.Tails[i].Image = ConvertIcon(ctx, result.Tails[i].Image, pluginInstance.PluginDirectory)
		}
	}

	// translate title
	result.Title = m.translatePlugin(ctx, pluginInstance, result.Title)
	// translate subtitle
	result.SubTitle = m.translatePlugin(ctx, pluginInstance, result.SubTitle)
	// translate tail text
	for i := range result.Tails {
		if result.Tails[i].Type == QueryResultTailTypeText {
			result.Tails[i].Text = m.translatePlugin(ctx, pluginInstance, result.Tails[i].Text)
		}
	}
	// translate preview properties
	var previewProperties = make(map[string]string)
	for key, value := range result.Preview.PreviewProperties {
		translatedKey := m.translatePlugin(ctx, pluginInstance, key)
		previewProperties[translatedKey] = value
	}
	result.Preview.PreviewProperties = previewProperties

	// store preview for ui invoke later
	// because preview may contain some heavy data (E.g. image or large text), we will store preview in cache and only send preview to ui when user select the result
	if result.Preview.PreviewType != "" && result.Preview.PreviewType != WoxPreviewTypeRemote {
		// update preview in cache
		if v, ok := m.resultCache.Load(resultId); ok {
			v.Preview = result.Preview
			m.resultCache.Store(resultId, v)
		}

		result.Preview = WoxPreview{
			PreviewType: WoxPreviewTypeRemote,
			PreviewData: fmt.Sprintf("/preview?id=%s", resultId),
		}
	}

	return result
}

func (m *Manager) Query(ctx context.Context, query Query) (results chan []QueryResultUI, done chan bool) {
	results = make(chan []QueryResultUI, 10)
	done = make(chan bool)

	// clear old result cache
	m.resultCache.Clear()

	counter := &atomic.Int32{}
	counter.Store(int32(len(m.instances)))

	for _, pluginInstance := range m.instances {
		if !m.canOperateQuery(ctx, pluginInstance, query) {
			counter.Add(-1)
			if counter.Load() == 0 {
				done <- true
			}
			continue
		}

		if pluginInstance.Metadata.IsSupportFeature(MetadataFeatureDebounce) {
			debounceParams, err := pluginInstance.Metadata.GetFeatureParamsForDebounce()
			if err == nil {
				logger.Debug(ctx, fmt.Sprintf("[%s] debounce query, will execute in %d ms", pluginInstance.Metadata.Name, debounceParams.intervalMs))
				if v, ok := m.debounceQueryTimer.Load(pluginInstance.Metadata.Id); ok {
					if v.timer.Stop() {
						v.onStop()
					}
				}

				timer := time.AfterFunc(time.Duration(debounceParams.intervalMs)*time.Millisecond, func() {
					m.queryParallel(ctx, pluginInstance, query, results, done, counter)
				})
				onStop := func() {
					logger.Debug(ctx, fmt.Sprintf("[%s] previous debounced query cancelled", pluginInstance.Metadata.Name))
					counter.Add(-1)
					if counter.Load() == 0 {
						done <- true
					}
				}
				m.debounceQueryTimer.Store(pluginInstance.Metadata.Id, &debounceTimer{
					timer:  timer,
					onStop: onStop,
				})
				continue
			} else {
				logger.Error(ctx, fmt.Sprintf("[%s] %s, query directlly", pluginInstance.Metadata.Name, err))
			}
		}

		m.queryParallel(ctx, pluginInstance, query, results, done, counter)
	}

	return
}

func (m *Manager) QuerySilent(ctx context.Context, query Query) bool {
	var startTimestamp = util.GetSystemTimestamp()
	var results []QueryResultUI
	resultChan, doneChan := m.Query(ctx, query)
	for {
		select {
		case r := <-resultChan:
			results = append(results, r...)
		case <-doneChan:
			logger.Info(ctx, fmt.Sprintf("silent query done, total results: %d, cost %d ms", len(results), util.GetSystemTimestamp()-startTimestamp))

			// execute default action if only one result
			if len(results) == 1 {
				result := results[0]
				for _, action := range result.Actions {
					if action.IsDefault {
						m.ExecuteAction(ctx, result.Id, action.Id)
						return true
					}
				}
			} else {
				notifier.Notify(fmt.Sprintf("Silent query failed, there shouldbe only one result, but got %d", len(results)))
			}

			return false
		case <-time.After(time.Minute):
			logger.Error(ctx, "silent query timeout")
			return false
		}
	}
}

func (m *Manager) QueryFallback(ctx context.Context, query Query, queryPlugin *Instance) (results []QueryResultUI) {
	var queryResults []QueryResult
	if query.IsGlobalQuery() {
		for _, instance := range m.instances {
			pluginInstance := instance
			if v, ok := pluginInstance.Plugin.(FallbackSearcher); ok {
				queryResults = v.QueryFallback(ctx, query)
				for i := range queryResults {
					queryResults[i] = m.PolishResult(ctx, pluginInstance, query, queryResults[i])
				}
			}
		}
	} else {
		if query.Command != "" {
			return results
		}

		// search query commands
		commands := lo.Filter(queryPlugin.GetQueryCommands(), func(item MetadataCommand, index int) bool {
			return strings.Contains(item.Command, query.Search) || query.Search == ""
		})
		queryResults = lo.Map(commands, func(item MetadataCommand, index int) QueryResult {
			return QueryResult{
				Title:    item.Command,
				SubTitle: item.Description,
				Icon:     ParseWoxImageOrDefault(queryPlugin.Metadata.Icon, NewWoxImageEmoji("🔍")),
				Actions: []QueryResultAction{
					{
						Name:                   "Execute",
						PreventHideAfterAction: true,
						Action: func(ctx context.Context, actionContext ActionContext) {
							m.ui.ChangeQuery(ctx, share.PlainQuery{
								QueryType: QueryTypeInput,
								QueryText: fmt.Sprintf("%s %s ", query.TriggerKeyword, item.Command),
							})
						},
					},
				},
			}
		})
		for i := range queryResults {
			queryResults[i] = m.PolishResult(ctx, queryPlugin, query, queryResults[i])
		}
	}

	queryResultsUI := lo.Map(queryResults, func(item QueryResult, index int) QueryResultUI {
		return item.ToUI()
	})
	results = append(results, queryResultsUI...)
	return results
}

func (m *Manager) queryParallel(ctx context.Context, pluginInstance *Instance, query Query, results chan []QueryResultUI, done chan bool, counter *atomic.Int32) {
	util.Go(ctx, fmt.Sprintf("[%s] parallel query", pluginInstance.Metadata.Name), func() {
		queryResults := m.queryForPlugin(ctx, pluginInstance, query)
		results <- lo.Map(queryResults, func(item QueryResult, index int) QueryResultUI {
			return item.ToUI()
		})
		counter.Add(-1)
		if counter.Load() == 0 {
			done <- true
		}
	}, func() {
		counter.Add(-1)
		if counter.Load() == 0 {
			done <- true
		}
	})
}

func (m *Manager) translatePlugin(ctx context.Context, pluginInstance *Instance, key string) string {
	if !strings.HasPrefix(key, "i18n:") {
		return key
	}

	if pluginInstance.IsSystemPlugin {
		return i18n.GetI18nManager().TranslateWox(ctx, key)
	} else {
		return i18n.GetI18nManager().TranslatePlugin(ctx, key, pluginInstance.PluginDirectory)
	}
}

func (m *Manager) GetUI() share.UI {
	return m.ui
}

func (m *Manager) NewQuery(ctx context.Context, plainQuery share.PlainQuery) (Query, *Instance, error) {
	if plainQuery.QueryType == QueryTypeInput {
		newQuery := plainQuery.QueryText
		woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
		if len(woxSetting.QueryShortcuts) > 0 {
			originQuery := plainQuery.QueryText
			expandedQuery := m.expandQueryShortcut(ctx, plainQuery.QueryText, woxSetting.QueryShortcuts)
			if originQuery != expandedQuery {
				logger.Info(ctx, fmt.Sprintf("expand query shortcut: %s -> %s", originQuery, expandedQuery))
				newQuery = expandedQuery
			}
		}
		query, instance := newQueryInputWithPlugins(newQuery, GetPluginManager().GetPluginInstances())
		query.Env.ActiveWindowTitle = m.GetUI().GetActiveWindowName()
		query.Env.ActiveWindowPid = m.GetUI().GetActiveWindowPid()
		query.Env.ActiveBrowserUrl = m.getActiveBrowserUrl(ctx)
		return query, instance, nil
	}

	if plainQuery.QueryType == QueryTypeSelection {
		query := Query{
			Type:      QueryTypeSelection,
			RawQuery:  plainQuery.QueryText,
			Search:    plainQuery.QueryText,
			Selection: plainQuery.QuerySelection,
		}
		query.Env.ActiveWindowTitle = m.GetUI().GetActiveWindowName()
		query.Env.ActiveWindowPid = m.GetUI().GetActiveWindowPid()
		query.Env.ActiveBrowserUrl = m.getActiveBrowserUrl(ctx)
		return query, nil, nil
	}

	return Query{}, nil, errors.New("invalid query type")
}

func (m *Manager) getActiveBrowserUrl(ctx context.Context) string {
	activeWindowName := m.GetUI().GetActiveWindowName()
	isGoogleChrome := strings.ToLower(activeWindowName) == "google chrome"
	if !isGoogleChrome {
		return ""
	}

	return m.activeBrowserUrl
}

func (m *Manager) expandQueryShortcut(ctx context.Context, query string, queryShorts []setting.QueryShortcut) (newQuery string) {
	newQuery = query

	//sort query shorts by shortcut length, we will expand the longest shortcut first
	slices.SortFunc(queryShorts, func(i, j setting.QueryShortcut) int {
		return len(j.Shortcut) - len(i.Shortcut)
	})

	for _, shortcut := range queryShorts {
		if strings.HasPrefix(query, shortcut.Shortcut) {
			if !shortcut.HasPlaceholder() {
				newQuery = strings.Replace(query, shortcut.Shortcut, shortcut.Query, 1)
				break
			} else {
				queryWithoutShortcut := strings.Replace(query, shortcut.Shortcut, "", 1)
				queryWithoutShortcut = strings.TrimLeft(queryWithoutShortcut, " ")
				parameters := strings.Split(queryWithoutShortcut, " ")
				placeholderCount := shortcut.PlaceholderCount()
				var paramsCount = 0

				var params []any
				var nonPrams string
				for _, param := range parameters {
					if paramsCount < placeholderCount {
						paramsCount++
						params = append(params, param)
					} else {
						nonPrams += " " + param
					}
				}
				newQuery = stringFormatter.Format(shortcut.Query, params...) + nonPrams
				break
			}
		}
	}

	return newQuery
}

func (m *Manager) ExecuteAction(ctx context.Context, resultId string, actionId string) error {
	resultCache, found := m.resultCache.Load(resultId)
	if !found {
		return fmt.Errorf("result cache not found for result id (execute action): %s", resultId)
	}
	action, exist := resultCache.Actions.Load(actionId)
	if !exist {
		return fmt.Errorf("action not found for result id: %s, action id: %s", resultId, actionId)
	}

	action(ctx, ActionContext{
		ContextData: resultCache.ContextData,
	})

	setting.GetSettingManager().AddActionedResult(ctx, resultCache.PluginInstance.Metadata.Id, resultCache.ResultTitle, resultCache.ResultSubTitle)
	return nil
}

func (m *Manager) ExecuteRefresh(ctx context.Context, refreshableResultWithId RefreshableResultWithResultId) (RefreshableResultWithResultId, error) {
	var refreshableResult RefreshableResult
	copyErr := copier.Copy(&refreshableResult, &refreshableResultWithId)
	if copyErr != nil {
		return RefreshableResultWithResultId{}, fmt.Errorf("failed to copy refreshable result: %w", copyErr)
	}

	resultCache, found := m.resultCache.Load(refreshableResultWithId.ResultId)
	if !found {
		return refreshableResultWithId, fmt.Errorf("result cache not found for result id (execute refresh): %s", refreshableResultWithId.ResultId)
	}

	newResult := resultCache.Refresh(ctx, refreshableResult)
	newResult = m.PolishRefreshableResult(ctx, resultCache.PluginInstance, refreshableResultWithId.ResultId, newResult)

	// update result cache
	resultCache.ResultTitle = newResult.Title
	resultCache.ResultSubTitle = newResult.SubTitle
	resultCache.ContextData = newResult.ContextData

	return RefreshableResultWithResultId{
		ResultId:        refreshableResultWithId.ResultId,
		Title:           newResult.Title,
		SubTitle:        newResult.SubTitle,
		Icon:            newResult.Icon,
		Tails:           newResult.Tails,
		Preview:         newResult.Preview,
		ContextData:     newResult.ContextData,
		RefreshInterval: newResult.RefreshInterval,
	}, nil
}

func (m *Manager) GetResultPreview(ctx context.Context, resultId string) (WoxPreview, error) {
	resultCache, found := m.resultCache.Load(resultId)
	if !found {
		return WoxPreview{}, fmt.Errorf("result cache not found for result id (get preview): %s", resultId)
	}

	preview := m.polishPreview(ctx, resultCache.Preview)

	// if preview text is too long, ellipsis it, otherwise UI maybe freeze when render
	if preview.PreviewType == WoxPreviewTypeText {
		preview.PreviewData = util.EllipsisMiddle(preview.PreviewData, 2000)
	}

	return preview, nil
}

func (m *Manager) polishPreview(ctx context.Context, preview WoxPreview) WoxPreview {
	if preview.PreviewType == WoxPreviewTypeImage {
		woxImage, err := ParseWoxImage(preview.PreviewData)
		if err != nil {
			logger.Error(ctx, fmt.Sprintf("failed to parse wox image for preview: %s", err.Error()))
			return preview
		}

		if woxImage.ImageType == WoxImageTypeAbsolutePath {
			newWoxImage := convertLocalImageToUrl(ctx, woxImage)
			return WoxPreview{
				PreviewType:       WoxPreviewTypeImage,
				PreviewData:       newWoxImage.String(),
				PreviewProperties: preview.PreviewProperties,
			}
		}
	}

	return preview
}

func (m *Manager) ReplaceQueryVariable(ctx context.Context, query string) string {
	if strings.Contains(query, QueryVariableSelectedText) {
		selection, selectedErr := util.GetSelected()
		if selectedErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to get selected text: %s", selectedErr.Error()))
		} else {
			if selection.Type == util.SelectionTypeText {
				query = strings.ReplaceAll(query, QueryVariableSelectedText, selection.Text)
			} else {
				logger.Error(ctx, fmt.Sprintf("selected data is not text, type: %s", selection.Type))
			}
		}
	}

	if strings.Contains(query, QueryVariableActiveBrowserUrl) {
		activeBrowserUrl := m.activeBrowserUrl
		query = strings.ReplaceAll(query, QueryVariableActiveBrowserUrl, activeBrowserUrl)
	}

	return query
}

func (m *Manager) IsHostStarted(ctx context.Context, runtime Runtime) bool {
	if runtime == PLUGIN_RUNTIME_GO {
		return true
	}

	for _, host := range AllHosts {
		if host.GetRuntime(ctx) == runtime {
			return host.IsStarted(ctx)
		}
	}

	return false
}

func (m *Manager) GetAIProvider(ctx context.Context, provider ai.ProviderName) (ai.Provider, error) {
	if v, exist := m.aiProviders.Load(provider); exist {
		return v, nil
	}

	//check if provider has setting
	aiProviderSettings := setting.GetSettingManager().GetWoxSetting(ctx).AIProviders
	providerSetting, providerSettingExist := lo.Find(aiProviderSettings, func(item setting.AIProvider) bool {
		return item.Name == string(provider)
	})
	if !providerSettingExist {
		return nil, fmt.Errorf("ai provider setting not found: %s", provider)
	}

	newProvider, newProviderErr := ai.NewProvider(ctx, providerSetting)
	if newProviderErr != nil {
		return nil, newProviderErr
	}
	m.aiProviders.Store(provider, newProvider)
	return newProvider, nil
}

func (m *Manager) ExecutePluginDeeplink(ctx context.Context, pluginId string, arguments map[string]string) {
	pluginInstance, exist := lo.Find(m.instances, func(item *Instance) bool {
		return item.Metadata.Id == pluginId
	})
	if !exist {
		logger.Error(ctx, fmt.Sprintf("plugin not found: %s", pluginId))
		return
	}

	for _, callback := range pluginInstance.DeepLinkCallbacks {
		util.Go(ctx, fmt.Sprintf("[%s] execute deeplink callback", pluginInstance.Metadata.Name), func() {
			callback(arguments)
		})
	}
}
