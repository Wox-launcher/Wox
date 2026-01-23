package setting

type PluginQueryCommand struct {
	Command     string
	Description string
}

type PluginSetting struct {
	// Is this plugin disabled by user
	Disabled *PluginSettingValue[bool]

	// User defined keywords, will be used to trigger this plugin. User may not set custom trigger keywords, which will cause this property to be null
	//
	// So don't use this property directly, use Instance.TriggerKeywords instead
	TriggerKeywords *PluginSettingValue[[]string]

	// plugin author can register query command dynamically
	// the final query command will be the combination of plugin's metadata commands defined in plugin.json and customized query command registered here
	//
	// So don't use this directly, use Instance.GetQueryCommands instead
	QueryCommands *PluginSettingValue[[]PluginQueryCommand]

	store                     *PluginSettingStore
	defaultSettingsInMetadata map[string]string
}

func NewPluginSetting(store *PluginSettingStore, defaultSettingsInMetadata map[string]string) *PluginSetting {
	return &PluginSetting{
		store:                     store,
		defaultSettingsInMetadata: defaultSettingsInMetadata,
		Disabled:                  NewPluginSettingValue(store, "Disabled", false),
		TriggerKeywords:           NewPluginSettingValue(store, "TriggerKeywords", []string{}),
		QueryCommands:             NewPluginSettingValue(store, "QueryCommands", []PluginQueryCommand{}),
	}
}

// Try to get the value of the setting. If the setting is not found, return the default value in metadata if exist, otherwise return empty string
func (p *PluginSetting) Get(key string) (string, bool) {
	var val string
	err := p.store.Get(key, &val)
	if err != nil {
		if val, ok := p.defaultSettingsInMetadata[key]; ok {
			return val, true
		}

		return "", false
	}

	return val, true
}

func (p *PluginSetting) Set(key string, value string) error {
	if syncStore, ok := any(p.store).(SyncableStore); ok {
		return syncStore.SetWithOplog(key, value, true)
	}
	return p.store.Set(key, value)
}

func (p *PluginSetting) Delete(key string) error {
	if syncStore, ok := any(p.store).(SyncableStore); ok {
		return syncStore.DeleteWithOplog(key, true)
	}
	return p.store.Delete(key)
}
