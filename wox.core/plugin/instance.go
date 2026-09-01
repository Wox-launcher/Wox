package plugin

import (
	"context"
	"fmt"
	"sync"
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

	// InitError is set when Init finishes unsuccessfully. Waiters distinguish
	// "still initializing" from "init completed with an error" through initDone.
	InitError       error
	initDone        chan struct{}
	initStateMu     sync.Mutex
	initLifecycleMu sync.Mutex
}

// beginInitCycle opens a new wait channel so later WaitInit calls block on this
// enable/load attempt instead of observing a previous finished cycle.
func (i *Instance) beginInitCycle() {
	if i == nil {
		return
	}
	i.initStateMu.Lock()
	defer i.initStateMu.Unlock()
	i.Initialized = false
	i.InitError = nil
	i.initDone = make(chan struct{})
}

// finishInit publishes the result before waking every waiter for this cycle.
func (i *Instance) finishInit(initialized bool, err error) {
	if i == nil {
		return
	}
	i.initStateMu.Lock()
	defer i.initStateMu.Unlock()
	i.Initialized = initialized
	i.InitError = err
	if i.initDone == nil {
		return
	}
	select {
	case <-i.initDone:
	default:
		close(i.initDone)
	}
}

func (i *Instance) resetInitState() {
	i.initStateMu.Lock()
	defer i.initStateMu.Unlock()
	i.Initialized = false
	i.InitError = nil
}

func (i *Instance) initStatus() (bool, error) {
	i.initStateMu.Lock()
	defer i.initStateMu.Unlock()
	return i.Initialized, i.InitError
}

// initCycleFinished reports whether the current cycle has notified its waiters.
func (i *Instance) initCycleFinished() bool {
	if i == nil {
		return true
	}
	i.initStateMu.Lock()
	defer i.initStateMu.Unlock()
	if i.initDone == nil {
		return true
	}
	select {
	case <-i.initDone:
		return true
	default:
		return false
	}
}

// WaitInit blocks until this instance's current init cycle finishes or ctx ends.
func (i *Instance) WaitInit(ctx context.Context) error {
	if i == nil {
		return fmt.Errorf("plugin instance is nil")
	}
	i.initStateMu.Lock()
	initDone := i.initDone
	i.initStateMu.Unlock()
	if initDone == nil {
		return fmt.Errorf("plugin init has not started")
	}
	select {
	case <-initDone:
		i.initStateMu.Lock()
		defer i.initStateMu.Unlock()
		return i.InitError
	case <-ctx.Done():
		return fmt.Errorf("timed out waiting for plugin init: %w", ctx.Err())
	}
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

// PrimaryTriggerKeyword returns the first non-global ("*") trigger keyword.
// Scoped queries must not use GetTriggerKeywords()[0] because "*" often comes first.
func (i *Instance) PrimaryTriggerKeyword() string {
	for _, keyword := range i.GetTriggerKeywords() {
		if keyword != "" && keyword != "*" {
			return keyword
		}
	}
	return ""
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
