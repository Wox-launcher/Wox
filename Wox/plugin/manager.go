package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"wox/util"
)

var managerInstance *Manager
var managerOnce sync.Once
var logger = util.GetLogger()

type Manager struct {
	instances []Instance
}

func GetPluginManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = &Manager{}
	})
	return managerInstance
}

func (m *Manager) Start(ctx context.Context) error {
	return m.loadPlugins(ctx)
}

func (m *Manager) loadPlugins(ctx context.Context) error {
	logger.Info(ctx, "Loading plugins")

	basePluginDirectory := util.GetLocation().GetPluginDirectory()
	pluginDirectories, readErr := os.ReadDir(basePluginDirectory)
	if readErr != nil {
		return fmt.Errorf("failed to read plugin directory: %w", readErr)
	}

	var metaDataList []Metadata
	for _, entry := range pluginDirectories {
		pluginDirectory := path.Join(basePluginDirectory, entry.Name())
		metadata, metadataErr := m.parseMetadata(ctx, pluginDirectory)
		if metadataErr != nil {
			logger.Error(ctx, metadataErr.Error())
			continue
		}
		metaDataList = append(metaDataList, metadata)
	}

	for _, host := range AllHosts {
		logger.Info(ctx, fmt.Sprintf("Starting host and load host plugins, host=%s", host.GetRuntime(ctx)))
		hostErr := host.Start(ctx)
		if hostErr != nil {
			logger.Error(ctx, fmt.Errorf("failed to start host: %w", hostErr).Error())
			continue
		}

		for _, metadata := range metaDataList {
			if strings.ToUpper(metadata.Runtime) != strings.ToUpper(string(host.GetRuntime(ctx))) {
				continue
			}

			loadStartTimestamp := util.GetSystemTimestamp()
			plugin, loadErr := host.LoadPlugin(ctx, metadata, path.Join(basePluginDirectory, metadata.Name))
			if loadErr != nil {
				logger.Error(ctx, fmt.Errorf("failed to load plugin: %w", loadErr).Error())
				continue
			}
			loadFinishTimestamp := util.GetSystemTimestamp()

			m.instances = append(m.instances, Instance{
				Metadata:              metadata,
				Plugin:                plugin,
				Host:                  host,
				API:                   NewAPI(metadata),
				LoadStartTimestamp:    loadStartTimestamp,
				LoadFinishedTimestamp: loadFinishTimestamp,
			})
		}
	}

	return nil
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

	return Metadata{}, nil
}
