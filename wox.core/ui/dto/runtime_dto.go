package dto

// RuntimeStatusDto describes the current state of a plugin runtime host.
type RuntimeStatusDto struct {
	Runtime           string
	IsStarted         bool
	HostVersion       string
	StatusCode        string
	StatusMessage     string
	ExecutablePath    string
	LastStartError    string
	CanRestart        bool
	InstallUrl        string
	LoadedPluginCount int
	LoadedPluginNames []string
}
