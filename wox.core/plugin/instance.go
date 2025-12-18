package plugin

import (
	"context"
	"wox/common"
	"wox/setting"
	"wox/setting/definition"
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

	DynamicSettingCallbacks []func(key string) definition.PluginSettingDefinitionItem // dynamic setting callbacks
	SettingChangeCallbacks  []func(key string, value string)
	DeepLinkCallbacks       []func(arguments map[string]string)
	UnloadCallbacks         []func()
	MRURestoreCallbacks     []func(mruData MRUData) (*QueryResult, error) // MRU restore callbacks

	// for measure performance
	LoadStartTimestamp    int64
	LoadFinishedTimestamp int64
	InitStartTimestamp    int64
	InitFinishedTimestamp int64
}

func (i *Instance) translateMetadataText(ctx context.Context, text common.I18nString) string {
	return i.Metadata.translate(ctx, text)
}

func (i *Instance) GetName(ctx context.Context) string {
	return i.Metadata.GetName(ctx)
}

func (i *Instance) GetDescription(ctx context.Context) string {
	return i.Metadata.GetDescription(ctx)
}

// trigger keywords to trigger this plugin. Maybe user defined or pre-defined in plugin.json
func (i *Instance) GetTriggerKeywords() []string {
	if i.Setting != nil && i.Setting.TriggerKeywords != nil {
		userDefinedKeywords := i.Setting.TriggerKeywords.Get()
		if len(userDefinedKeywords) > 0 {
			return userDefinedKeywords
		}
	}

	return i.Metadata.TriggerKeywords
}

// query commands to query this plugin. Maybe plugin author dynamical registered or pre-defined in plugin.json
func (i *Instance) GetQueryCommands() []MetadataCommand {
	settingCommandsCount := 0
	if i.Setting != nil && i.Setting.QueryCommands != nil {
		settingCommandsCount = len(i.Setting.QueryCommands.Get())
	}

	commands := make([]MetadataCommand, 0, len(i.Metadata.Commands)+settingCommandsCount)
	commands = append(commands, i.Metadata.Commands...)
	if i.Setting != nil && i.Setting.QueryCommands != nil {
		for _, command := range i.Setting.QueryCommands.Get() {
			commands = append(commands, MetadataCommand{
				Command:     command.Command,
				Description: common.I18nString(command.Description),
			})
		}
	}

	ctx := context.Background()
	for commandIndex := range commands {
		commands[commandIndex].Description = common.I18nString(i.translateMetadataText(ctx, commands[commandIndex].Description))
	}
	return commands
}

func (i *Instance) String() string {
	return i.GetName(context.Background())
}
