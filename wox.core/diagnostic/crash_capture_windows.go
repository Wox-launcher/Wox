//go:build windows

package diagnostic

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

const (
	windowsCrashArtifactWait = 2 * time.Second
	windowsWERTempWindow     = 10 * time.Second
)

// ConfigureCrashCapture asks Windows Error Reporting to retain local minidumps for Wox.
func (m *Manager) ConfigureCrashCapture(ctx context.Context) error {
	if err := m.EnsureDirectories(); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	keyPath := `Software\Microsoft\Windows\Windows Error Reporting\LocalDumps\` + filepath.Base(executable)
	key, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.SetExpandStringValue("DumpFolder", m.CrashDumpsDirectory()); err != nil {
		return err
	}
	if err := key.SetDWordValue("DumpType", 1); err != nil {
		return err
	}
	if err := key.SetDWordValue("DumpCount", retainedCrashArtifacts); err != nil {
		return err
	}
	m.retainNewestCrashFiles(m.CrashDumpsDirectory(), ".dmp", retainedCrashArtifacts)
	m.retainNewestCrashFiles(m.CrashDumpsDirectory(), ".mdmp", retainedCrashArtifacts)
	m.retainNewestCrashFiles(m.CrashReportsDirectory(), ".zip", retainedCrashArtifacts)
	m.retainNewestCrashFiles(m.CrashIncidentsDirectory(), ".json", retainedCrashArtifacts)
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

func (m *Manager) waitForCrashArtifacts(runStartedAt time.Time) {
	// WER can finish writing just after the process handle signals. Give it a
	// short bounded window so the first generated report contains the dump.
	crashDetectedAt := time.Now()
	deadline := crashDetectedAt.Add(windowsCrashArtifactWait)
	for time.Now().Before(deadline) {
		found := false
		for _, dumpPath := range m.findWindowsCrashDumps(runStartedAt, crashDetectedAt) {
			if !strings.EqualFold(filepath.Dir(dumpPath), m.CrashDumpsDirectory()) {
				if err := m.copyWindowsCrashDump(dumpPath); err != nil {
					continue
				}
			}
			found = true
			break
		}
		if found {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// findWindowsCrashDumps returns recent WER dumps from both the Wox directory
// and the standard Windows locations that WER may use independently of Wox.
func (m *Manager) findWindowsCrashDumps(runStartedAt, crashDetectedAt time.Time) []string {
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

	seen := make(map[string]struct{})
	dumps := make([]string, 0)
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
			dumps = append(dumps, path)
		}
	}
	return dumps
}

// copyWindowsCrashDump preserves a dump found outside Wox's diagnostic tree so
// WER can clean up its temporary source before the report is opened.
func (m *Manager) copyWindowsCrashDump(sourcePath string) error {
	if strings.EqualFold(filepath.Dir(sourcePath), m.CrashDumpsDirectory()) {
		return nil
	}
	if err := m.EnsureDirectories(); err != nil {
		return err
	}
	destinationPath := filepath.Join(m.CrashDumpsDirectory(), "wer-"+filepath.Base(sourcePath))
	for suffix := 1; ; suffix++ {
		_, err := os.Stat(destinationPath)
		if os.IsNotExist(err) {
			break
		}
		if err != nil {
			return err
		}
		destinationPath = filepath.Join(m.CrashDumpsDirectory(), "wer-"+filepath.Base(sourcePath)+"-"+strconv.Itoa(suffix))
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(destination, source); err != nil {
		_ = destination.Close()
		_ = os.Remove(destinationPath)
		return err
	}
	return destination.Close()
}

func isWindowsDumpFile(name string) bool {
	extension := strings.ToLower(filepath.Ext(name))
	return extension == ".dmp" || extension == ".mdmp"
}
