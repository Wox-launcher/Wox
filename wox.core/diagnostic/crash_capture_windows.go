//go:build windows

package diagnostic

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unsafe"
	"wox/resource"
	"wox/util"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	windowsCrashArtifactWait = 2 * time.Second
	windowsWERTempWindow     = 10 * time.Second
	windowsCrashHandlerKey   = `Software\Wox\CrashHandler`
	windowsWERModuleKey      = `Software\Microsoft\Windows\Windows Error Reporting\RuntimeExceptionHelperModules`
	windowsCrashHandlerAsset = "others/crash_handler/WoxCrashHandler64.dll"
)

var werRegisterRuntimeExceptionModule = windows.NewLazySystemDLL("kernel32.dll").NewProc("WerRegisterRuntimeExceptionModule")

// ConfigureCrashCapture registers Wox's per-user out-of-process WER dump writer.
func (m *Manager) ConfigureCrashCapture(ctx context.Context) error {
	if err := m.EnsureDirectories(); err != nil {
		return err
	}
	m.retainNewestCrashFiles(m.CrashDumpsDirectory(), ".dmp", retainedCrashArtifacts)
	m.retainNewestCrashFiles(m.CrashDumpsDirectory(), ".mdmp", retainedCrashArtifacts)
	m.retainNewestCrashFiles(m.CrashReportsDirectory(), ".zip", retainedCrashArtifacts)
	m.retainNewestCrashFiles(m.CrashIncidentsDirectory(), ".json", retainedCrashArtifacts)

	// Only the supervised child can produce the incident that the supervisor
	// packages. Avoid consuming one of WER's module slots in bootstrap processes.
	if !m.IsChildArg(os.Args) {
		return nil
	}
	handlerPath, err := m.extractWindowsCrashHandler()
	if err != nil {
		return err
	}
	if err := m.configureWindowsCrashHandlerRegistry(handlerPath); err != nil {
		return err
	}
	if err := registerWindowsCrashHandler(handlerPath); err != nil {
		return err
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("registered Windows crash handler: module=%s dumpDirectory=%s", handlerPath, m.CrashDumpsDirectory()))
	return nil
}

// extractWindowsCrashHandler writes a content-addressed DLL outside the normal
// resource tree, which is replaced later during startup.
func (m *Manager) extractWindowsCrashHandler() (string, error) {
	data, err := resource.OthersFS.ReadFile(windowsCrashHandlerAsset)
	if err != nil {
		return "", fmt.Errorf("read embedded crash handler: %w", err)
	}
	digest := sha256.Sum256(data)
	handlerDirectory := filepath.Join(m.DiagnosticsDirectory(), "crash-handler")
	if err := os.MkdirAll(handlerDirectory, 0755); err != nil {
		return "", err
	}
	handlerPath := filepath.Join(handlerDirectory, fmt.Sprintf("WoxCrashHandler64-%x.dll", digest[:6]))
	if existing, readErr := os.ReadFile(handlerPath); readErr == nil && bytes.Equal(existing, data) {
		return handlerPath, nil
	}
	temporaryPath := fmt.Sprintf("%s.tmp-%d", handlerPath, os.Getpid())
	if err := os.WriteFile(temporaryPath, data, 0644); err != nil {
		return "", err
	}
	if err := os.Remove(handlerPath); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(temporaryPath)
		return "", err
	}
	if err := os.Rename(temporaryPath, handlerPath); err != nil {
		_ = os.Remove(temporaryPath)
		return "", err
	}
	return handlerPath, nil
}

// configureWindowsCrashHandlerRegistry allowlists only Wox-owned handler DLLs
// for the current user and tells the callback where to write completed dumps.
func (m *Manager) configureWindowsCrashHandlerRegistry(handlerPath string) error {
	configKey, _, err := registry.CreateKey(registry.CURRENT_USER, windowsCrashHandlerKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	if err := configKey.SetStringValue("DumpFolder", m.CrashDumpsDirectory()); err != nil {
		configKey.Close()
		return err
	}
	configKey.Close()

	moduleKey, _, err := registry.CreateKey(registry.CURRENT_USER, windowsWERModuleKey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer moduleKey.Close()
	handlerDirectory := filepath.Dir(handlerPath)
	valueNames, _ := moduleKey.ReadValueNames(-1)
	for _, valueName := range valueNames {
		if !strings.EqualFold(valueName, handlerPath) && strings.EqualFold(filepath.Dir(valueName), handlerDirectory) {
			_ = moduleKey.DeleteValue(valueName)
		}
	}
	return moduleKey.SetDWordValue(handlerPath, 0)
}

// registerWindowsCrashHandler opts the current process into the allowlisted WER module.
func registerWindowsCrashHandler(handlerPath string) error {
	handlerPathPointer, err := windows.UTF16PtrFromString(handlerPath)
	if err != nil {
		return err
	}
	hresult, _, _ := werRegisterRuntimeExceptionModule.Call(uintptr(unsafe.Pointer(handlerPathPointer)), 0)
	runtime.KeepAlive(handlerPathPointer)
	if hresult != 0 {
		return fmt.Errorf("WerRegisterRuntimeExceptionModule failed: HRESULT %#x", uint32(hresult))
	}
	return nil
}

func (m *Manager) addWindowsCrashDumps(zipWriter *zip.Writer) {
	// Use the run start boundary to select only dumps that can belong to the
	// current supervised process.
	entries, err := os.ReadDir(m.CrashDumpsDirectory())
	if err != nil {
		return
	}
	state := m.LoadState()
	startedAt := time.UnixMilli(state.StartedAt).Add(-2 * time.Minute)
	for _, entry := range entries {
		if entry.IsDir() || !isWindowsDumpFile(entry.Name()) {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil || info.ModTime().Before(startedAt) {
			continue
		}
		addExistingFile(zipWriter, filepath.Join(m.CrashDumpsDirectory(), entry.Name()), filepath.Join("windows-dumps", entry.Name()))
	}
}

func (m *Manager) waitForCrashArtifacts(pid int, runStartedAt time.Time) string {
	// WER can finish writing just after the process handle signals. Give it a
	// short bounded window so the first generated report contains the dump.
	crashDetectedAt := time.Now()
	deadline := crashDetectedAt.Add(windowsCrashArtifactWait)
	for time.Now().Before(deadline) {
		for _, dumpPath := range m.findWindowsCrashDumps(pid, runStartedAt, crashDetectedAt) {
			if !strings.EqualFold(filepath.Dir(dumpPath), m.CrashDumpsDirectory()) {
				preservedPath, err := m.copyWindowsCrashDump(dumpPath)
				if err != nil {
					continue
				}
				return preservedPath
			}
			return dumpPath
		}
		time.Sleep(100 * time.Millisecond)
	}
	return ""
}

// findWindowsCrashDumps returns recent WER dumps from both the Wox directory
// and the standard Windows locations that WER may use independently of Wox.
func (m *Manager) findWindowsCrashDumps(pid int, runStartedAt, crashDetectedAt time.Time) []string {
	executable, _ := os.Executable()
	executableName := strings.ToLower(filepath.Base(executable))
	executableStem := strings.TrimSuffix(executableName, strings.ToLower(filepath.Ext(executableName)))
	standardDumpDirectory := ""
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		standardDumpDirectory = filepath.Join(localAppData, "CrashDumps")
	}
	werTempDirectory := ""
	if programData := os.Getenv("ProgramData"); programData != "" {
		werTempDirectory = filepath.Join(programData, "Microsoft", "Windows", "WER", "Temp")
	}

	type dumpSearchLocation struct {
		directory string
		minTime   time.Time
		matchName bool
	}
	locations := []dumpSearchLocation{
		{directory: m.CrashDumpsDirectory(), minTime: runStartedAt.Add(-2 * time.Minute)},
		{directory: standardDumpDirectory, minTime: runStartedAt.Add(-2 * time.Minute), matchName: true},
		// WER temporary files are not named after the crashed executable and are
		// removed after WER finishes, so only consider files created at detection.
		{directory: werTempDirectory, minTime: crashDetectedAt.Add(-windowsWERTempWindow)},
	}

	type dumpCandidate struct {
		path       string
		modifiedAt time.Time
		matchesPID bool
	}
	seen := make(map[string]struct{})
	candidates := make([]dumpCandidate, 0)
	for _, location := range locations {
		if location.directory == "" {
			continue
		}
		entries, err := os.ReadDir(location.directory)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !isWindowsDumpFile(entry.Name()) {
				continue
			}
			if location.matchName && executableName != "" {
				lowerName := strings.ToLower(entry.Name())
				if !strings.Contains(lowerName, executableName) && !strings.Contains(lowerName, executableStem) {
					continue
				}
			}
			info, err := entry.Info()
			if err != nil || info.Size() == 0 || info.ModTime().Before(location.minTime) {
				continue
			}
			path := filepath.Join(location.directory, entry.Name())
			key := strings.ToLower(filepath.Clean(path))
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			candidates = append(candidates, dumpCandidate{
				path:       path,
				modifiedAt: info.ModTime(),
				matchesPID: dumpNameMatchesPID(entry.Name(), pid),
			})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].matchesPID != candidates[j].matchesPID {
			return candidates[i].matchesPID
		}
		return candidates[i].modifiedAt.After(candidates[j].modifiedAt)
	})
	dumps := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		dumps = append(dumps, candidate.path)
	}
	return dumps
}

// copyWindowsCrashDump preserves a dump found outside Wox's diagnostic tree so
// WER can clean up its temporary source before the report is opened.
func (m *Manager) copyWindowsCrashDump(sourcePath string) (string, error) {
	if strings.EqualFold(filepath.Dir(sourcePath), m.CrashDumpsDirectory()) {
		return sourcePath, nil
	}
	if err := m.EnsureDirectories(); err != nil {
		return "", err
	}
	destinationPath := filepath.Join(m.CrashDumpsDirectory(), "wer-"+filepath.Base(sourcePath))
	for suffix := 1; ; suffix++ {
		_, err := os.Stat(destinationPath)
		if os.IsNotExist(err) {
			break
		}
		if err != nil {
			return "", err
		}
		destinationPath = filepath.Join(m.CrashDumpsDirectory(), "wer-"+filepath.Base(sourcePath)+"-"+strconv.Itoa(suffix))
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer source.Close()
	destination, err := os.Create(destinationPath)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(destination, source); err != nil {
		_ = destination.Close()
		_ = os.Remove(destinationPath)
		return "", err
	}
	if err := destination.Close(); err != nil {
		_ = os.Remove(destinationPath)
		return "", err
	}
	return destinationPath, nil
}

// dumpNameMatchesPID requires numeric boundaries so reused PID fragments do not
// associate an unrelated dump with the current crash incident.
func dumpNameMatchesPID(name string, pid int) bool {
	if pid <= 0 {
		return false
	}
	pidText := strconv.Itoa(pid)
	for offset := 0; ; {
		index := strings.Index(name[offset:], pidText)
		if index < 0 {
			return false
		}
		index += offset
		leftBoundary := index == 0 || name[index-1] < '0' || name[index-1] > '9'
		rightIndex := index + len(pidText)
		rightBoundary := rightIndex == len(name) || name[rightIndex] < '0' || name[rightIndex] > '9'
		if leftBoundary && rightBoundary {
			return true
		}
		offset = index + 1
	}
}

func isWindowsDumpFile(name string) bool {
	extension := strings.ToLower(filepath.Ext(name))
	return extension == ".dmp" || extension == ".mdmp"
}
