package plugin

import (
	"context"
)

var AllHosts []Host

type RuntimeHostStatusCode string

const (
	RuntimeHostStatusRunning            RuntimeHostStatusCode = "running"
	RuntimeHostStatusStopped            RuntimeHostStatusCode = "stopped"
	RuntimeHostStatusExecutableMissing  RuntimeHostStatusCode = "executable_missing"
	RuntimeHostStatusUnsupportedVersion RuntimeHostStatusCode = "unsupported_version"
	RuntimeHostStatusStartFailed        RuntimeHostStatusCode = "start_failed"
)

// RuntimeHostStatus carries actionable host health details to the UI.
// The old IsStarted-only contract could not tell users whether the interpreter
// was missing or the host process failed after launch, so settings and install
// flows had no reliable next action to show.
type RuntimeHostStatus struct {
	StatusCode     RuntimeHostStatusCode
	StatusMessage  string
	ExecutablePath string
	LastStartError string
	CanRestart     bool
	InstallUrl     string
}

type Host interface {
	GetRuntime(ctx context.Context) Runtime
	Start(ctx context.Context) error
	Stop(ctx context.Context)
	IsStarted(ctx context.Context) bool
	RuntimeStatus(ctx context.Context) RuntimeHostStatus
	LoadPlugin(ctx context.Context, metadata Metadata, pluginDirectory string) (Plugin, error)
	UnloadPlugin(ctx context.Context, metadata Metadata)
}
