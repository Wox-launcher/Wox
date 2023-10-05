package plugin

// Metadata parsed from plugin.json, see `Plugin.json.md` for more detail
// All properties are immutable after initialization
type Metadata struct {
	Id              string
	Name            string
	Author          string
	Version         string
	MinWoxVersion   string
	Runtime         string
	Description     string
	Icon            string
	Website         string
	Entry           string
	TriggerKeywords []string //User can add/update/delete trigger keywords
	Commands        []MetadataCommand
	SupportedOS     []string
}

type MetadataCommand struct {
	Command     string
	Description string
}
