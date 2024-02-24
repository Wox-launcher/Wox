package setting

import (
	"wox/util"
)

type PluginQueryCommand struct {
	Command     string
	Description string
}

type PluginSetting struct {
	// plugin name, readonly
	Name string

	// Is this plugin disabled by user
	Disabled bool

	// User defined keywords, will be used to trigger this plugin. User may not set custom trigger keywords, which will cause this property to be null
	//
	// So don't use this property directly, use Instance.TriggerKeywords instead
	TriggerKeywords []string

	// plugin author can register query command dynamically
	// the final query command will be the combination of plugin's metadata commands defined in plugin.json and customized query command registered here
	//
	// So don't use this directly, use Instance.GetQueryCommands instead
	QueryCommands []PluginQueryCommand

	Settings *util.HashMap[string, string]
}

func (p *PluginSetting) GetSetting(key string) (string, bool) {
	if p.Settings == nil {
		return "", false
	}
	return p.Settings.Load(key)
}
