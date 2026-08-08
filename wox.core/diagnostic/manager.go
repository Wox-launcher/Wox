package diagnostic

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"wox/updater"
	"wox/util"
)

const (
	ArgSupervisor = "--bug-aware-supervisor"
	ArgChild      = "--bug-aware-child"
	ArgWaitParent = "--bug-aware-wait-parent"
)

type State struct {
	RunId            string `json:"runId"`
	StartedAt        int64  `json:"startedAt"`
	LastHeartbeatAt  int64  `json:"lastHeartbeatAt"`
	LastCleanExit    bool   `json:"lastCleanExit"`
	CorePid          int    `json:"corePid"`
	UIPid            int    `json:"uiPid"`
	ChildPid         int    `json:"childPid"`
	LastUIExitCode   int    `json:"lastUIExitCode"`
	LastUIExitSignal string `json:"lastUIExitSignal"`
	LastCoreExitCode int    `json:"lastCoreExitCode"`
	LastCoreSignal   string `json:"lastCoreSignal"`
	LastExportPath   string `json:"lastExportPath"`
}

type Breadcrumb struct {
	Timestamp int64          `json:"timestamp"`
	Event     string         `json:"event"`
	Data      map[string]any `json:"data,omitempty"`
}

// CrashIncident records one abnormal exit and its locally generated report.
type CrashIncident struct {
	ID         string `json:"id"`
	DetectedAt int64  `json:"detectedAt"`
	PID        int    `json:"pid"`
	ExitCode   int    `json:"exitCode"`
	Signal     string `json:"signal,omitempty"`
	DurationMs int64  `json:"durationMs"`
	ReportPath string `json:"reportPath"`
	DumpPath   string `json:"dumpPath,omitempty"`
	Version    string `json:"version"`
	Prompted   bool   `json:"prompted"`
}

type Manager struct {
	mu sync.Mutex
}

var manager = &Manager{}

func GetManager() *Manager {
	return manager
}

func (m *Manager) IsSupervisorArg(args []string) bool {
	return hasArg(args, ArgSupervisor)
}

func (m *Manager) IsChildArg(args []string) bool {
	return hasArg(args, ArgChild)
}

func (m *Manager) DiagnosticsDirectory() string {
	return filepath.Join(util.GetLocation().GetWoxDataDirectory(), "diagnostics")
}

func (m *Manager) StatePath() string {
	return filepath.Join(m.DiagnosticsDirectory(), "state.json")
}

func (m *Manager) BreadcrumbPath() string {
	return filepath.Join(m.DiagnosticsDirectory(), "breadcrumbs.jsonl")
}

func (m *Manager) SupervisorLogPath() string {
	return filepath.Join(m.DiagnosticsDirectory(), "supervisor.log")
}

func (m *Manager) ExportsDirectory() string {
	return filepath.Join(m.DiagnosticsDirectory(), "exports")
}

func (m *Manager) CrashDirectory() string {
	return filepath.Join(m.DiagnosticsDirectory(), "crashes")
}

func (m *Manager) CrashDumpsDirectory() string {
	return filepath.Join(m.CrashDirectory(), "dumps")
}

func (m *Manager) CrashReportsDirectory() string {
	return filepath.Join(m.CrashDirectory(), "reports")
}

// CrashIncidentsDirectory stores one metadata file for each retained crash event.
func (m *Manager) CrashIncidentsDirectory() string {
	return filepath.Join(m.CrashDirectory(), "incidents")
}

func (m *Manager) CrashIncidentPath() string {
	return filepath.Join(m.CrashDirectory(), "latest.json")
}

func (m *Manager) EnsureDirectories() error {
	for _, dir := range []string{m.DiagnosticsDirectory(), m.ExportsDirectory(), m.CrashDumpsDirectory(), m.CrashReportsDirectory(), m.CrashIncidentsDirectory()} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) LoadState() State {
	state := State{LastCleanExit: true, LastUIExitCode: -1, LastCoreExitCode: -1}
	data, err := os.ReadFile(m.StatePath())
	if err != nil {
		return state
	}
	if unmarshalErr := json.Unmarshal(data, &state); unmarshalErr != nil {
		return State{LastCleanExit: true, LastUIExitCode: -1, LastCoreExitCode: -1}
	}
	return state
}

func (m *Manager) SaveState(state State) error {
	if err := m.EnsureDirectories(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.StatePath(), data, 0644)
}

func (m *Manager) RecordRunStart(ctx context.Context, child bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.LoadState()
	now := util.GetSystemTimestamp()
	state.RunId = fmt.Sprintf("%d-%d", now, os.Getpid())
	state.StartedAt = now
	state.LastHeartbeatAt = now
	state.LastCleanExit = false
	state.CorePid = os.Getpid()
	if child {
		state.ChildPid = os.Getpid()
	}
	_ = m.SaveState(state)
	m.AppendBreadcrumb(ctx, "run_start", map[string]any{"child": child, "pid": os.Getpid()})
}

func (m *Manager) MarkCleanExit(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.LoadState()
	state.LastCleanExit = true
	state.LastHeartbeatAt = util.GetSystemTimestamp()
	_ = m.SaveState(state)
	m.AppendBreadcrumb(ctx, "clean_exit", map[string]any{"pid": os.Getpid()})
}

func (m *Manager) RecordUIExit(ctx context.Context, pid int, waitErr error, expected bool) {
	exitCode, signalName := ResolveProcessExit(waitErr)
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.LoadState()
	state.UIPid = pid
	state.LastUIExitCode = exitCode
	state.LastUIExitSignal = signalName
	_ = m.SaveState(state)
	m.AppendBreadcrumb(ctx, "ui_process_exit", map[string]any{"pid": pid, "exitCode": exitCode, "signal": signalName, "expected": expected})
}

func (m *Manager) RecordSupervisorExit(ctx context.Context, pid int, waitErr error, durationMs int64) {
	exitCode, signalName := ResolveProcessExit(waitErr)
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.LoadState()
	state.ChildPid = pid
	state.LastCoreExitCode = exitCode
	state.LastCoreSignal = signalName
	state.LastCleanExit = waitErr == nil
	state.LastHeartbeatAt = util.GetSystemTimestamp()
	_ = m.SaveState(state)
	m.AppendBreadcrumb(ctx, "core_child_exit", map[string]any{"pid": pid, "exitCode": exitCode, "signal": signalName, "durationMs": durationMs})
}

func (m *Manager) AppendBreadcrumb(ctx context.Context, event string, data map[string]any) {
	if err := m.EnsureDirectories(); err != nil {
		return
	}
	entry := Breadcrumb{Timestamp: util.GetSystemTimestamp(), Event: event, Data: data}
	encoded, err := json.Marshal(entry)
	if err != nil {
		return
	}
	file, err := os.OpenFile(m.BreadcrumbPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.Write(append(encoded, '\n'))
}

func (m *Manager) Export(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.EnsureDirectories(); err != nil {
		return "", err
	}
	exportPath := filepath.Join(m.ExportsDirectory(), fmt.Sprintf("wox-diagnostics-%s.zip", time.Now().Format("20060102-150405")))
	return m.exportLocked(ctx, exportPath)
}

// ExportCrash writes an automatic report outside the manually reset diagnostics session.
func (m *Manager) ExportCrash(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.EnsureDirectories(); err != nil {
		return "", err
	}
	exportPath := filepath.Join(m.CrashReportsDirectory(), fmt.Sprintf("wox-crash-%s.zip", time.Now().Format("20060102-150405.000")))
	for suffix := 1; ; suffix++ {
		if _, statErr := os.Stat(exportPath); statErr != nil {
			break
		}
		exportPath = filepath.Join(m.CrashReportsDirectory(), fmt.Sprintf("wox-crash-%s-%d.zip", time.Now().Format("20060102-150405.000"), suffix))
	}
	return m.exportLocked(ctx, exportPath)
}

func (m *Manager) exportLocked(ctx context.Context, exportPath string) (string, error) {
	file, err := os.Create(exportPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	currentLogPath := util.GetLogger().CurrentLogPath()
	addExistingFile(zipWriter, currentLogPath, "log/"+filepath.Base(currentLogPath))
	addExistingFile(zipWriter, filepath.Join(util.GetLocation().GetLogDirectory(), "ui.log"), "log/ui.log")
	addExistingFile(zipWriter, filepath.Join(util.GetLocation().GetLogDirectory(), "crash.log"), "log/crash.log")
	addExistingFile(zipWriter, m.SupervisorLogPath(), "diagnostics/supervisor.log")
	addExistingFile(zipWriter, m.StatePath(), "diagnostics/state.json")
	addExistingFile(zipWriter, m.BreadcrumbPath(), "diagnostics/breadcrumbs.jsonl")
	m.addMetadata(zipWriter)
	m.addMacOSCrashReports(zipWriter)
	m.addWindowsCrashDumps(zipWriter)

	state := m.LoadState()
	state.LastExportPath = exportPath
	_ = m.SaveState(state)
	m.AppendBreadcrumb(ctx, "diagnostics_exported", map[string]any{"path": exportPath})
	return exportPath, nil
}

// SaveCrashIncident persists an abnormal exit in the history and updates the startup pointer.
func (m *Manager) SaveCrashIncident(incident CrashIncident) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if incident.ID == "" || filepath.Base(incident.ID) != incident.ID || strings.ContainsAny(incident.ID, `\/:*?"<>|`) {
		return fmt.Errorf("crash incident ID must be a safe file name")
	}
	if err := m.EnsureDirectories(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(incident, "", "  ")
	if err != nil {
		return err
	}
	incidentPath := filepath.Join(m.CrashIncidentsDirectory(), incident.ID+".json")
	if err := writeCrashIncidentFile(incidentPath, data); err != nil {
		return err
	}
	return writeCrashIncidentFile(m.CrashIncidentPath(), data)
}

// writeCrashIncidentFile updates an incident file without exposing partial JSON.
func writeCrashIncidentFile(path string, data []byte) error {
	temporaryPath := path + ".tmp"
	if err := os.WriteFile(temporaryPath, data, 0644); err != nil {
		return err
	}
	// Windows cannot atomically replace an existing file with os.Rename.
	_ = os.Remove(path)
	return os.Rename(temporaryPath, path)
}

// ListCrashIncidents returns retained crash events from newest to oldest.
func (m *Manager) ListCrashIncidents() []CrashIncident {
	entries, err := os.ReadDir(m.CrashIncidentsDirectory())
	if err != nil {
		if incident, ok := m.LatestCrashIncident(); ok {
			return []CrashIncident{incident}
		}
		return nil
	}

	incidents := make([]CrashIncident, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".json" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(m.CrashIncidentsDirectory(), entry.Name()))
		if readErr != nil {
			continue
		}
		var incident CrashIncident
		if json.Unmarshal(data, &incident) == nil && incident.ID != "" {
			incidents = append(incidents, incident)
		}
	}

	// Include an incident written by an older build until it is superseded by a
	// new event in the per-incident history directory.
	if latest, ok := m.LatestCrashIncident(); ok {
		found := false
		for _, incident := range incidents {
			if incident.ID == latest.ID {
				found = true
				break
			}
		}
		if !found {
			incidents = append(incidents, latest)
		}
	}

	sort.SliceStable(incidents, func(i, j int) bool {
		if incidents[i].DetectedAt == incidents[j].DetectedAt {
			return incidents[i].ID > incidents[j].ID
		}
		return incidents[i].DetectedAt > incidents[j].DetectedAt
	})
	return incidents
}

// LatestCrashIncident returns the most recently captured abnormal exit.
func (m *Manager) LatestCrashIncident() (CrashIncident, bool) {
	data, err := os.ReadFile(m.CrashIncidentPath())
	if err != nil {
		return CrashIncident{}, false
	}
	var incident CrashIncident
	if err := json.Unmarshal(data, &incident); err != nil {
		return CrashIncident{}, false
	}
	return incident, true
}

// TakePendingCrashIncident marks an incident as prompted while retaining its report path.
func (m *Manager) TakePendingCrashIncident() (CrashIncident, bool) {
	incident, ok := m.LatestCrashIncident()
	if !ok || incident.Prompted {
		return CrashIncident{}, false
	}
	incident.Prompted = true
	if err := m.SaveCrashIncident(incident); err != nil {
		return CrashIncident{}, false
	}
	return incident, true
}

func (m *Manager) addMetadata(zipWriter *zip.Writer) {
	state := m.LoadState()
	metadata := map[string]any{
		"version": updater.CURRENT_VERSION,
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
		"state":   state,
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return
	}
	writer, err := zipWriter.Create("diagnostics/metadata.json")
	if err != nil {
		return
	}
	_, _ = writer.Write(data)
}

func (m *Manager) addMacOSCrashReports(zipWriter *zip.Writer) {
	if !util.IsMacOS() {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	reportDir := filepath.Join(home, "Library", "Logs", "DiagnosticReports")
	entries, err := os.ReadDir(reportDir)
	if err != nil {
		return
	}
	state := m.LoadState()
	startedAt := time.UnixMilli(state.StartedAt).Add(-2 * time.Minute)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if !(strings.HasSuffix(lower, ".ips") || strings.HasSuffix(lower, ".crash")) {
			continue
		}
		if !strings.HasPrefix(lower, "wox") {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil || info.ModTime().Before(startedAt) {
			continue
		}
		addExistingFile(zipWriter, filepath.Join(reportDir, name), filepath.Join("macos-diagnostic-reports", name))
	}
}

func addExistingFile(zipWriter *zip.Writer, src string, name string) {
	file, err := os.Open(src)
	if err != nil {
		return
	}
	defer file.Close()
	writer, err := zipWriter.Create(name)
	if err != nil {
		return
	}
	_, _ = io.Copy(writer, file)
}

func hasArg(args []string, arg string) bool {
	for _, item := range args {
		if item == arg {
			return true
		}
	}
	return false
}
