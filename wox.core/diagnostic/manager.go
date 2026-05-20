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
	Enabled          bool   `json:"enabled"`
	RunId            string `json:"runId"`
	StartedAt        int64  `json:"startedAt"`
	LastHeartbeatAt  int64  `json:"lastHeartbeatAt"`
	LastCleanExit    bool   `json:"lastCleanExit"`
	PreviousLogLevel string `json:"previousLogLevel"`
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

func (m *Manager) EnsureDirectories() error {
	for _, dir := range []string{m.DiagnosticsDirectory(), m.ExportsDirectory()} {
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

func (m *Manager) IsEnabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.LoadState().Enabled
}

func (m *Manager) Enable(ctx context.Context, previousLogLevel string) (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Feature update: each bug aware session should start with an empty
	// diagnostics directory, otherwise old state, breadcrumbs, and exports can
	// be mistaken for the reproduction that the user is about to capture.
	if err := os.RemoveAll(m.DiagnosticsDirectory()); err != nil {
		return State{}, err
	}
	if err := m.EnsureDirectories(); err != nil {
		return State{}, err
	}
	if err := m.clearSessionLogs(); err != nil {
		return State{}, err
	}

	state := m.LoadState()
	state.Enabled = true
	state.PreviousLogLevel = previousLogLevel
	state.LastCleanExit = true
	state.LastUIExitCode = -1
	state.LastCoreExitCode = -1
	state.LastUIExitSignal = ""
	state.LastCoreSignal = ""
	state.LastExportPath = ""
	state.StartedAt = util.GetSystemTimestamp()
	state.LastHeartbeatAt = state.StartedAt
	state.CorePid = os.Getpid()
	state.UIPid = 0
	state.ChildPid = 0
	state.RunId = fmt.Sprintf("%d-%d", state.StartedAt, os.Getpid())
	if err := m.SaveState(state); err != nil {
		return State{}, err
	}
	// New feature: enabling bug aware mode starts from a clean log boundary so
	// exported reports are focused on the reproduction session instead of old noise.
	m.AppendBreadcrumb(ctx, "bug_aware_enabled", map[string]any{"previousLogLevel": previousLogLevel})
	return state, nil
}

func (m *Manager) Disable(ctx context.Context) (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.LoadState()
	state.Enabled = false
	if err := m.SaveState(state); err != nil {
		return State{}, err
	}
	m.AppendBreadcrumb(ctx, "bug_aware_disabled", nil)
	return state, nil
}

func (m *Manager) RecordRunStart(ctx context.Context, child bool) {
	if !m.IsEnabled() {
		return
	}
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
	if !m.IsEnabled() {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.LoadState()
	state.LastCleanExit = true
	state.LastHeartbeatAt = util.GetSystemTimestamp()
	_ = m.SaveState(state)
	m.AppendBreadcrumb(ctx, "clean_exit", map[string]any{"pid": os.Getpid()})
}

func (m *Manager) RecordUIExit(ctx context.Context, pid int, waitErr error, expected bool) {
	if !m.IsEnabled() {
		return
	}
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
	file, err := os.Create(exportPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	addExistingFile(zipWriter, filepath.Join(util.GetLocation().GetLogDirectory(), "log"), "log/log")
	addExistingFile(zipWriter, filepath.Join(util.GetLocation().GetLogDirectory(), "ui.log"), "log/ui.log")
	addExistingFile(zipWriter, filepath.Join(util.GetLocation().GetLogDirectory(), "crash.log"), "log/crash.log")
	addExistingFile(zipWriter, m.SupervisorLogPath(), "diagnostics/supervisor.log")
	addExistingFile(zipWriter, m.StatePath(), "diagnostics/state.json")
	addExistingFile(zipWriter, m.BreadcrumbPath(), "diagnostics/breadcrumbs.jsonl")
	m.addMetadata(zipWriter)
	m.addMacOSCrashReports(zipWriter)

	state := m.LoadState()
	state.LastExportPath = exportPath
	_ = m.SaveState(state)
	m.AppendBreadcrumb(ctx, "diagnostics_exported", map[string]any{"path": exportPath})
	return exportPath, nil
}

func (m *Manager) clearSessionLogs() error {
	if err := util.GetLogger().ClearHistory(); err != nil {
		return err
	}
	for _, filePath := range []string{
		filepath.Join(util.GetLocation().GetLogDirectory(), "ui.log"),
		m.SupervisorLogPath(),
		m.BreadcrumbPath(),
	} {
		if err := os.WriteFile(filePath, []byte{}, 0644); err != nil {
			return err
		}
	}
	return nil
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
		if !(strings.HasPrefix(lower, "wox") || strings.HasPrefix(lower, "wox-ui")) {
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
