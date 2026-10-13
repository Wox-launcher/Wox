package filesearchservice

import (
	"errors"
	"sync/atomic"

	"github.com/Masterminds/semver/v3"
)

const (
	Name            = "WoxFileIndexService"
	DisplayName     = "Wox File Index Service"
	Description     = "Provides privileged NTFS file indexing support for Wox File Search."
	ProtocolVersion = 3
	PipeName        = `\\.\pipe\WoxFileIndexService`
)

var EmbeddedVersion = "2.8.3"
var running atomic.Bool

var ErrProtocolMismatch = errors.New("file index service protocol requires an update")

// IsRunning returns the last SCM or health state observed by Wox.
func IsRunning() bool { return running.Load() }

func markRunning() { running.Store(true) }

func markStopped() { running.Store(false) }

var indexedVolumeRootsCache atomic.Value

// IndexedVolumeRoots returns the NTFS volumes the running service indexes.
func IndexedVolumeRoots() []string {
	return indexedVolumeRoots()
}

func rememberIndexedVolumeRoots(roots []string) {
	if len(roots) == 0 {
		return
	}
	indexedVolumeRootsCache.Store(append([]string(nil), roots...))
}

func cachedIndexedVolumeRoots() []string {
	cached, ok := indexedVolumeRootsCache.Load().([]string)
	if !ok || len(cached) == 0 {
		return nil
	}
	return append([]string(nil), cached...)
}

type State string

const (
	StateUnavailable  State = "unavailable"
	StateNotInstalled State = "not_installed"
	StateRunning      State = "running"
	StateStopped      State = "stopped"
	StateUpdateReady  State = "update_ready"
	StateNeedsUpdate  State = "needs_update"
	StateUnhealthy    State = "unhealthy"
)

type Request struct {
	Protocol      int    `json:"protocol"`
	Method        string `json:"method"`
	CandidatePath string `json:"candidatePath,omitempty"`
	Version       string `json:"version,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
}

type RequestHandler func(Request) (afterResponse func(), err error)

type Response struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	Protocol int    `json:"protocol"`
	Version  string `json:"version"`
}

// Status describes the installed service without starting or modifying it.
type Status struct {
	State            State
	InstalledVersion string
	EmbeddedVersion  string
	Detail           string
}

func (s Status) HasUpdate() bool {
	return s.InstalledVersion != "" && updateAvailable(s.InstalledVersion, s.EmbeddedVersion)
}

func updateAvailable(installed, embedded string) bool {
	installedVersion, installedErr := semver.NewVersion(installed)
	embeddedVersion, embeddedErr := semver.NewVersion(embedded)
	return installedErr == nil && embeddedErr == nil && embeddedVersion.GreaterThan(installedVersion)
}
