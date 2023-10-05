package plugin

type Instance struct {
	Plugin          Plugin   // plugin implementation
	API             API      // APIs exposed to plugin
	Metadata        Metadata // metadata parsed from plugin.json
	IsSystemPlugin  bool     // is system plugin, see `plugin.md` for more detail
	TriggerKeywords []string // trigger keywords to trigger this plugin. Maybe user defined or pre-defined in plugin.json
	PluginDirectory string   // absolute path to plugin directory
	Host            Host     // plugin host to run this plugin

	// for measure performance
	LoadStartTimestamp    int64
	LoadFinishedTimestamp int64
	InitStartTimestamp    int64
	InitFinishedTimestamp int64
}
