package plugin

import (
	"context"
	"encoding/json"
	"fmt"
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
}

func GetPluginManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = &Manager{}
		logger = util.GetLogger()
	})
	return managerInstance
}

func (m *Manager) Start(ctx context.Context, ui share.UI) error {
	loadErr := m.loadPlugins(ctx)
	if loadErr != nil {
		return fmt.Errorf("failed to load plugins: %w", loadErr)
	}

	util.Go(ctx, "start store manager", func() {
		GetStoreManager().Start(util.NewTraceContext())
	})

	return nil
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

				loadStartTimestamp := util.GetSystemTimestamp()
				plugin, loadErr := host.LoadPlugin(newCtx, metadata.Metadata, metadata.Directory)
				if loadErr != nil {
					logger.Error(newCtx, fmt.Errorf("[%s HOST] failed to load plugin: %w", host.GetRuntime(newCtx), loadErr).Error())
					continue
				}
				loadFinishTimestamp := util.GetSystemTimestamp()

				instance := &Instance{
					Metadata:              metadata.Metadata,
					Plugin:                plugin,
					Host:                  host,
					API:                   NewAPI(metadata.Metadata),
					LoadStartTimestamp:    loadStartTimestamp,
					LoadFinishedTimestamp: loadFinishTimestamp,
				}
				m.instances = append(m.instances, instance)

				util.Go(newCtx, fmt.Sprintf("[%s] init plugin", metadata.Metadata.Name), func() {
					m.initPlugin(util.NewTraceContext(), instance)
				})
			}
		})
	}

	return nil
}

func (m *Manager) loadSystemPlugins(ctx context.Context) {
	logger.Info(ctx, fmt.Sprintf("start loading system plugins, found %d system plugins", len(AllSystemPlugin)))

	for _, plugin := range AllSystemPlugin {
		loadStartTimestamp := util.GetSystemTimestamp()
		plugin.Init(ctx, InitParams{
			API: NewAPI(plugin.GetMetadata()),
		})
		loadFinishTimestamp := util.GetSystemTimestamp()

		instance := &Instance{
			Metadata:              plugin.GetMetadata(),
			Plugin:                plugin,
			Host:                  nil,
			IsSystemPlugin:        true,
			API:                   NewAPI(plugin.GetMetadata()),
			LoadStartTimestamp:    loadStartTimestamp,
			LoadFinishedTimestamp: loadFinishTimestamp,
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

	return metadata, nil
}

func (m *Manager) GetPluginInstances() []*Instance {
	return m.instances
}

func (m *Manager) isQueryMatchPlugin(ctx context.Context, pluginInstance *Instance, query Query) bool {
	var validGlobalQuery = lo.Contains(pluginInstance.GetTriggerKeywords(), "*") && query.TriggerKeyword == ""
	var validNonGlobalQuery = lo.Contains(pluginInstance.GetTriggerKeywords(), query.TriggerKeyword)
	if !validGlobalQuery && !validNonGlobalQuery {
		return false
	}

	return true
}

func (m *Manager) QueryForPlugin(ctx context.Context, pluginInstance *Instance, query Query) []QueryResultEx {
	logger.Info(ctx, fmt.Sprintf("[%s] start query: %s", pluginInstance.Metadata.Name, query.RawQuery))
	results := pluginInstance.Plugin.Query(ctx, query)
	return lo.Map(results, func(result QueryResult, _ int) QueryResultEx {
		return QueryResultEx{
			QueryResult:     result,
			PluginInstance:  pluginInstance,
			AssociatedQuery: query,
		}
	})
}

func (m *Manager) Query(ctx context.Context, query Query) (results chan []QueryResultEx, done chan bool) {
	results = make(chan []QueryResultEx, 100)
	done = make(chan bool)

	counter := atomic.Int32{}
	counter.Store(int32(len(m.instances)))

	for _, instance := range m.instances {
		pluginInstance := instance

		if !m.isQueryMatchPlugin(ctx, pluginInstance, query) {
			counter.Add(-1)
			continue
		}

		util.Go(ctx, fmt.Sprintf("[%s] parallel query", instance.Metadata.Name), func() {
			queryResults := m.QueryForPlugin(ctx, pluginInstance, query)
			if len(queryResults) == 0 {
				return
			}

			results <- queryResults
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
