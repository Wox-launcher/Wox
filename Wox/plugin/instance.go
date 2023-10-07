package plugin

import "wox/util"

type Instance struct {
	Plugin          Plugin                       // plugin implementation
	API             API                          // APIs exposed to plugin
	Metadata        Metadata                     // metadata parsed from plugin.json
	IsSystemPlugin  bool                         // is system plugin, see `plugin.md` for more detail
	PluginDirectory string                       // absolute path to plugin directory
	Host            Host                         // plugin host to run this plugin
	CommonSetting   CommonSetting                // common setting for this plugin
	SpecificSetting util.HashMap[string, string] // specific setting for this plugin

	// for measure performance
	LoadStartTimestamp    int64
	LoadFinishedTimestamp int64
	InitStartTimestamp    int64
	InitFinishedTimestamp int64
}

// trigger keywords to trigger this plugin. Maybe user defined or pre-defined in plugin.json
func (i *Instance) GetTriggerKeywords() []string {
	if i.CommonSetting.TriggerKeywords != nil {
		return i.CommonSetting.TriggerKeywords
	}
	return i.Metadata.TriggerKeywords
}
