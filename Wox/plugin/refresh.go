package plugin

type RefreshCallback struct {
	ResultId       string
	Refresh        func(QueryResult) QueryResult
	PluginInstance *Instance
	Query          Query
}
