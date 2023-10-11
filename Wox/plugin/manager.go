package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/samber/lo"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"wox/share"
	"wox/util"
)

var managerInstance *Manager
var managerOnce sync.Once
var logger *util.Log

type Manager struct {
	instances []*Instance
	ui        share.UI
	actions   util.HashMap[string, func()]
}

func GetPluginManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = &Manager{}
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

func (m *Manager) loadPlugins(ctx context.Context) error {
	logger.Info(ctx, "start loading plugins")

	// load system plugin first
	m.loadSystemPlugins(ctx)

	basePluginDirectory := util.GetLocation().GetPluginDirectory()
	pluginDirectories, readErr := os.ReadDir(basePluginDirectory)
	if readErr != nil {
		return fmt.Errorf("failed to read plugin directory: %w", readErr)
	}

	var metaDataList []MetadataWithDirectory
	for _, entry := range pluginDirectories {
		pluginDirectory := path.Join(basePluginDirectory, entry.Name())
		metadata, metadataErr := m.parseMetadata(ctx, pluginDirectory)
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
		metaDataList = append(metaDataList, MetadataWithDirectory{metadata, pluginDirectory})
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
		API:                   NewAPI(metadata.Metadata),
		LoadStartTimestamp:    loadStartTimestamp,
		LoadFinishedTimestamp: loadFinishTimestamp,
	}
	m.instances = append(m.instances, instance)

	util.Go(ctx, fmt.Sprintf("[%s] init plugin", metadata.Metadata.Name), func() {
		m.initPlugin(ctx, instance)
	})

	return nil
}

func (m *Manager) LoadPlugin(ctx context.Context, pluginDirectory string) error {
	metadata, parseErr := m.parseMetadata(ctx, pluginDirectory)
	if parseErr != nil {
		return parseErr
	}

	pluginHost, exist := lo.Find(AllHosts, func(item Host) bool {
		return strings.ToLower(string(item.GetRuntime(ctx))) == strings.ToLower(metadata.Runtime)
	})
	if !exist {
		return fmt.Errorf("unsupported runtime: %s", metadata.Runtime)
	}

	loadErr := m.loadHostPlugin(ctx, pluginHost, MetadataWithDirectory{metadata, pluginDirectory})
	if loadErr != nil {
		return loadErr
	}

	return nil
}

func (m *Manager) UnloadPlugin(ctx context.Context, pluginInstance *Instance) {
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
	logger.Info(ctx, fmt.Sprintf("start loading system plugins, found %d system plugins", len(AllSystemPlugin)))

	for _, plugin := range AllSystemPlugin {
		instance := &Instance{
			Metadata:              plugin.GetMetadata(),
			Plugin:                plugin,
			Host:                  nil,
			IsSystemPlugin:        true,
			API:                   NewAPI(plugin.GetMetadata()),
			LoadStartTimestamp:    util.GetSystemTimestamp(),
			LoadFinishedTimestamp: util.GetSystemTimestamp(),
		}
		m.instances = append(m.instances, instance)

		util.Go(ctx, fmt.Sprintf("[%s] init system plugin", plugin.GetMetadata().Name), func() {
			m.initPlugin(util.NewTraceContext(), instance)
		})
	}
}

func (m *Manager) initPlugin(ctx context.Context, instance *Instance) {
	logger.Info(ctx, fmt.Sprintf("[%s] init plugin", instance.Metadata.Name))
	instance.InitStartTimestamp = util.GetSystemTimestamp()
	instance.Plugin.Init(ctx, InitParams{
		API: instance.API,
	})
	instance.InitFinishedTimestamp = util.GetSystemTimestamp()
	logger.Info(ctx, fmt.Sprintf("[%s] init plugin finished, cost %d ms", instance.Metadata.Name, instance.InitFinishedTimestamp-instance.InitStartTimestamp))
}

func (m *Manager) parseMetadata(ctx context.Context, pluginDirectory string) (Metadata, error) {
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
		return Metadata{}, fmt.Errorf("failed to unmarshal plugin.json file: %w", unmarshalErr)
	}

	if len(metadata.TriggerKeywords) == 0 {
		return Metadata{}, fmt.Errorf("missing trigger keywords in plugin.json file")
	}
	if !IsSupportedRuntime(metadata.Runtime) {
		return Metadata{}, fmt.Errorf("unsupported runtime in plugin.json file, runtime=%s", metadata.Runtime)
	}
	if !IsSupportedOSAny(metadata.SupportedOS) {
		return Metadata{}, fmt.Errorf("unsupported os in plugin.json file, os=%s", metadata.SupportedOS)
	}

	return metadata, nil
}

func (m *Manager) GetPluginInstances() []*Instance {
	return m.instances
}

func (m *Manager) isQueryMatchPlugin(ctx context.Context, pluginInstance *Instance, query Query) bool {
	// System Plugin Indicator is a special system plugin, it will be triggered even if query enter plugin mode, so that we can prompt plugin commands
	if pluginInstance.Metadata.Id == "39a4a6155f094ef89778188ae4a3ca03" {
		return true
	}

	var validGlobalQuery = lo.Contains(pluginInstance.GetTriggerKeywords(), "*") && query.TriggerKeyword == ""
	var validNonGlobalQuery = lo.Contains(pluginInstance.GetTriggerKeywords(), query.TriggerKeyword)
	if !validGlobalQuery && !validNonGlobalQuery {
		return false
	}

	return true
}

func (m *Manager) QueryForPlugin(ctx context.Context, pluginInstance *Instance, query Query) []QueryResult {
	logger.Info(ctx, fmt.Sprintf("[%s] start query: %s", pluginInstance.Metadata.Name, query.RawQuery))
	results := pluginInstance.Plugin.Query(ctx, query)
	for i := range results {
		results[i].Icon = convertLocalImageToUrl(ctx, results[i].Icon, pluginInstance)
	}
	return results
}

func (m *Manager) Query(ctx context.Context, query Query) (results chan []QueryResultUI, done chan bool) {
	results = make(chan []QueryResultUI, 10)
	done = make(chan bool)

	counter := atomic.Int32{}
	counter.Store(int32(len(m.instances)))

	for _, instance := range m.instances {
		pluginInstance := instance

		if !m.isQueryMatchPlugin(ctx, pluginInstance, query) {
			counter.Add(-1)
			if counter.Load() == 0 {
				done <- true
			}
			continue
		}

		util.Go(ctx, fmt.Sprintf("[%s] parallel query", instance.Metadata.Name), func() {
			queryResults := m.QueryForPlugin(ctx, pluginInstance, query)

			// store actions for ui invoke later
			for _, result := range queryResults {
				m.actions.Store(result.Id, result.Action)
			}

			results <- lo.Map(queryResults, func(item QueryResult, index int) QueryResultUI {
				return QueryResultUI{
					Id:              item.Id,
					Title:           item.Title,
					SubTitle:        item.SubTitle,
					Icon:            item.Icon,
					Preview:         item.Preview,
					Score:           item.Score,
					AssociatedQuery: query.RawQuery,
				}
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

	return
}

func (m *Manager) GetUI() share.UI {
	return m.ui
}

func (m *Manager) GetAction(resultId string) func() {
	action, found := m.actions.Load(resultId)
	if found {
		return action
	}

	return nil
}
