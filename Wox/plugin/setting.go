package plugin

type CommonSetting struct {
	// Is this plugin disabled by user
	Disabled bool

	// User defined keywords, will be used to trigger this plugin. User may not set custom trigger keywords, which will cause this property to be null
	// So don't use this property directly, use Instance.TriggerKeywords instead
	TriggerKeywords []string
}
