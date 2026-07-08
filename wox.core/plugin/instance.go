package plugin

import (
	"context"
	"wox/common"
	"wox/setting"
	"wox/setting/definition"
)

type Instance struct {
	Plugin               Plugin                 // plugin implementation
	API                  API                    // APIs exposed to plugin
	Metadata             Metadata               // metadata parsed from plugin.json
	IsSystemPlugin       bool                   // is system plugin, see `plugin.md` for more detail
	RuntimeLoaded        bool                   // host runtime has loaded this plugin
	Initialized          bool                   // plugin Init has run and runtime callbacks may be registered
	IsDevPlugin          bool                   // plugins loaded from `local plugin directories` which defined in wpm settings
	DevPluginDirectory   string                 // absolute path to dev plugin directory defined in wpm settings
	PluginDirectory      string                 // absolute path to plugin directory
	Host                 Host                   // plugin host to run this plugin
	Setting              *setting.PluginSetting // setting for this plugin
	RuntimeQueryCommands []MetadataCommand      // query commands registered at runtime

	DynamicSettingCallbacks   []func(ctx context.Context, key string) definition.PluginSettingDefinitionItem // dynamic setting callbacks
	SettingChangeCallbacks    []func(ctx context.Context, key string, value string)
	DeepLinkCallbacks         []func(ctx context.Context, arguments map[string]string)
	UnloadCallbacks           []func(ctx context.Context)
	MRURestoreCallbacks       []func(ctx context.Context, mruData MRUData) (*QueryResult, error) // MRU restore callbacks
	PluginCommandHandlers     []PluginCommandHandler
	EnterPluginQueryCallbacks []func(ctx context.Context)
	LeavePluginQueryCallbacks []func(ctx context.Context)

	// for measure performance
	LoadStartTimestamp    int64
	LoadFinishedTimestamp int64
	InitStartTimestamp    int64
	InitFinishedTimestamp int64
}

func (i *Instance) translateMetadataText(ctx context.Context, text common.I18nString) string {
	return i.Metadata.translate(ctx, text)
}

func (i *Instance) TranslateMetadataText(ctx context.Context, text common.I18nString) string {
	return i.translateMetadataText(ctx, text)
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

// query commands to query this plugin. Commands come from plugin metadata and runtime registration only.
func (i *Instance) GetQueryCommands() []MetadataCommand {
	commands := make([]MetadataCommand, 0, len(i.Metadata.Commands)+len(i.RuntimeQueryCommands))
	seen := make(map[string]struct{}, len(i.Metadata.Commands)+len(i.RuntimeQueryCommands))
	translateCtx := context.Background()

	appendCommand := func(command MetadataCommand) {
		if command.Command == "" {
			return
		}
		if _, exists := seen[command.Command]; exists {
			return
		}
		seen[command.Command] = struct{}{}
		command.Description = common.I18nString(i.translateMetadataText(translateCtx, command.Description))
		commands = append(commands, command)
	}

	for _, command := range i.Metadata.Commands {
		appendCommand(command)
	}

	for _, command := range i.RuntimeQueryCommands {
		appendCommand(command)
	}

	return commands
}

func (i *Instance) String() string {
	return i.GetName(context.Background())
}
