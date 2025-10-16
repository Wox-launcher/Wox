package dto

// RuntimeStatusDto describes the current state of a plugin runtime host.
type RuntimeStatusDto struct {
	Runtime           string
	IsStarted         bool
	LoadedPluginCount int
	LoadedPluginNames []string
}
