package plugin

import (
	"context"
	"wox/setting"
)

type Instance struct {
	Plugin             Plugin                 // plugin implementation
	API                API                    // APIs exposed to plugin
	Metadata           Metadata               // metadata parsed from plugin.json
	IsSystemPlugin     bool                   // is system plugin, see `plugin.md` for more detail
	IsDevPlugin        bool                   // plugins loaded from `local plugin directories` which defined in wpm settings
	DevPluginDirectory string                 // absolute path to dev plugin directory defined in wpm settings
	PluginDirectory    string                 // absolute path to plugin directory
	Host               Host                   // plugin host to run this plugin
	Setting            *setting.PluginSetting // setting for this plugin

	DynamicSettingCallbacks []func(key string) string // dynamic setting callbacks
	SettingChangeCallbacks  []func(key string, value string)
	DeepLinkCallbacks       []func(arguments map[string]string)
	UnloadCallbacks         []func()

	// for measure performance
	LoadStartTimestamp    int64
	LoadFinishedTimestamp int64
	InitStartTimestamp    int64
	InitFinishedTimestamp int64
}

// trigger keywords to trigger this plugin. Maybe user defined or pre-defined in plugin.json
func (i *Instance) GetTriggerKeywords() []string {
	if i.Setting.TriggerKeywords != nil {
		return i.Setting.TriggerKeywords
	}
	return i.Metadata.TriggerKeywords
}

// query commands to query this plugin. Maybe plugin author dynamical registered or pre-defined in plugin.json
func (i *Instance) GetQueryCommands() []MetadataCommand {
	commands := i.Metadata.Commands
	for _, command := range i.Setting.QueryCommands {
		commands = append(commands, MetadataCommand{
			Command:     command.Command,
			Description: command.Description,
		})
	}
	return commands
}

func (i *Instance) String() string {
	return i.Metadata.Name
}

func (i *Instance) SaveSetting(ctx context.Context) error {
	return setting.GetSettingManager().SavePluginSetting(ctx, i.Metadata.Id, i.Setting)
}
