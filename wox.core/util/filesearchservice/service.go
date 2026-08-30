package filesearchservice

import (
	"sync/atomic"

	"github.com/Masterminds/semver/v3"
)

const (
	Name            = "WoxFileIndexService"
	DisplayName     = "Wox File Index Service"
	Description     = "Provides privileged NTFS file indexing support for Wox File Search."
	ProtocolVersion = 2
	PipeName        = `\\.\pipe\WoxFileIndexService`
	IndexDirectory  = "file-index-service"
)

var EmbeddedVersion = "2.8.0"
var running atomic.Bool

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
	Protocol      int      `json:"protocol"`
	Method        string   `json:"method"`
	RootPath      string   `json:"rootPath,omitempty"`
	RootPaths     []string `json:"rootPaths,omitempty"`
	CandidatePath string   `json:"candidatePath,omitempty"`
	Version       string   `json:"version,omitempty"`
	SHA256        string   `json:"sha256,omitempty"`
	Query         string   `json:"query,omitempty"`
	Limit         int      `json:"limit,omitempty"`
	UsePinyin     bool     `json:"usePinyin,omitempty"`
	IndexPath     string   `json:"indexPath,omitempty"`
}

type RequestHandler func(Request, func(SnapshotEntry) error) (afterResponse func(), err error)

type Response struct {
	OK         bool        `json:"ok"`
	Error      string      `json:"error,omitempty"`
	Protocol   int         `json:"protocol"`
	Version    string      `json:"version"`
	IndexStats *IndexStats `json:"indexStats,omitempty"`
}

// IndexStats is the service-owned column index state exposed to Wox.
type IndexStats struct {
	IsIndexing         bool     `json:"isIndexing"`
	EntryCount         int64    `json:"entryCount"`
	FileCount          int64    `json:"fileCount"`
	IndexBytes         int64    `json:"indexBytes"`
	IndexCapacityBytes int64    `json:"indexCapacityBytes,omitempty"`
	HeapAllocBytes     int64    `json:"heapAllocBytes,omitempty"`
	HeapInuseBytes     int64    `json:"heapInuseBytes,omitempty"`
	HeapReleasedBytes  int64    `json:"heapReleasedBytes,omitempty"`
	VolumeCount        int      `json:"volumeCount"`
	VolumeIndex        int      `json:"volumeIndex"`
	VolumeTotal        int      `json:"volumeTotal"`
	CurrentVolume      string   `json:"currentVolume,omitempty"`
	Volumes            []string `json:"volumes,omitempty"`
	ElapsedMs          int64    `json:"elapsedMs"`
	LastElapsedMs      int64    `json:"lastElapsedMs"`
	Error              string   `json:"error,omitempty"`
}

// SnapshotEntry is the privileged filesystem fact streamed back to Wox.
type SnapshotEntry struct {
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Score int64  `json:"score,omitempty"`
}

type snapshotFrame struct {
	Entry   *SnapshotEntry  `json:"entry,omitempty"`
	Entries []SnapshotEntry `json:"entries,omitempty"`
	Done    bool            `json:"done,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// Status describes the installed service without starting or modifying it.
type Status struct {
	State            State
	InstalledVersion string
	EmbeddedVersion  string
	Detail           string
}

func updateAvailable(installed, embedded string) bool {
	installedVersion, installedErr := semver.NewVersion(installed)
	embeddedVersion, embeddedErr := semver.NewVersion(embedded)
	return installedErr == nil && embeddedErr == nil && embeddedVersion.GreaterThan(installedVersion)
}
