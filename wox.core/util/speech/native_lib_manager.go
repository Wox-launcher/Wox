package speech

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"wox/util"
)

// NativeLibVersion is the sherpa-onnx release version used for native library
// downloads. The version must match the onnxruntime ABI that the embedded
// sherpa-onnx C header expects.
const NativeLibVersion = "v1.13.4"

// nativeLibDownloadURL returns the GitHub release URL for the platform's
// shared library archive. Each archive contains a lib/ directory with the
// three files we need: onnxruntime, sherpa-onnx-c-api, sherpa-onnx-cxx-api.
func nativeLibDownloadURL(goos, goarch string) (string, error) {
	switch goos {
	case "windows":
		if goarch == "amd64" {
			return fmt.Sprintf("https://github.com/k2-fsa/sherpa-onnx/releases/download/%s/sherpa-onnx-%s-win-x64-shared-MD-Release-lib.tar.bz2", NativeLibVersion, NativeLibVersion), nil
		}
	case "linux":
		if goarch == "amd64" {
			return fmt.Sprintf("https://github.com/k2-fsa/sherpa-onnx/releases/download/%s/sherpa-onnx-%s-linux-x64-shared-lib.tar.bz2", NativeLibVersion, NativeLibVersion), nil
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return fmt.Sprintf("https://github.com/k2-fsa/sherpa-onnx/releases/download/%s/sherpa-onnx-%s-osx-x64-shared-lib.tar.bz2", NativeLibVersion, NativeLibVersion), nil
		case "arm64":
			return fmt.Sprintf("https://github.com/k2-fsa/sherpa-onnx/releases/download/%s/sherpa-onnx-%s-osx-arm64-shared-lib.tar.bz2", NativeLibVersion, NativeLibVersion), nil
		}
	}
	return "", fmt.Errorf("unsupported platform: %s/%s", goos, goarch)
}

// sileroVadDownloadURL is the GitHub release URL for silero_vad.onnx.
const sileroVadDownloadURL = "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/silero_vad.onnx"

// NativeLibState describes the lifecycle of a native library download.
type NativeLibState string

const (
	NativeLibStateIdle        NativeLibState = "idle"
	NativeLibStateDownloading NativeLibState = "downloading"
	NativeLibStateExtracting  NativeLibState = "extracting"
	NativeLibStateDone        NativeLibState = "done"
	NativeLibStateFailed      NativeLibState = "failed"
)

// NativeLibStatus tracks the state of native library download + extraction.
type NativeLibStatus struct {
	Progress int // 0-100
	State    NativeLibState
	Error    string
}

// NativeLibManager handles on-demand download and extraction of platform-
// specific sherpa-onnx native libraries (onnxruntime, sherpa-onnx-c-api,
// sherpa-onnx-cxx-api) and the silero VAD model. Libraries are downloaded
// from GitHub releases the first time dictation is used, then cached on disk
// so subsequent starts skip the download.
type NativeLibManager struct {
	libDir string

	mu     sync.Mutex
	status NativeLibStatus

	// downloading is true while a download is in progress. Callers that
	// arrive during a download wait on downloadWait; the downloader signals
	// it when done. After the signal, downloading is cleared so a failed
	// download can be retried on the next EnsureLibraries call.
	downloading  bool
	downloadWait *sync.Cond
}

// NewNativeLibManager creates a native library manager rooted at libDir.
// The directory is created if it does not exist.
func NewNativeLibManager(libDir string) (*NativeLibManager, error) {
	if err := os.MkdirAll(libDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create native lib directory: %w", err)
	}
	m := &NativeLibManager{libDir: libDir}
	m.downloadWait = sync.NewCond(&m.mu)
	return m, nil
}

// LibDir returns the directory where native libraries are stored.
func (m *NativeLibManager) LibDir() string {
	return m.libDir
}

// IsReady checks whether all required native libraries are present on disk.
func (m *NativeLibManager) IsReady() bool {
	for _, name := range sherpaLibraryNames() {
		if !util.IsFileExists(filepath.Join(m.libDir, name)) {
			return false
		}
	}
	return true
}

// VadModelPath returns the on-disk path for silero_vad.onnx. The file may
// not exist yet; callers should ensure it has been downloaded first.
func (m *NativeLibManager) VadModelPath() string {
	return filepath.Join(m.libDir, "silero_vad.onnx")
}

// IsVadModelReady checks whether the silero VAD model exists on disk.
func (m *NativeLibManager) IsVadModelReady() bool {
	return util.IsFileExists(m.VadModelPath())
}

// GetStatus returns the current download status.
func (m *NativeLibManager) GetStatus() NativeLibStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

// EnsureLibraries downloads and extracts native libraries and the silero VAD
// model if they are not already on disk. If a download is already in progress,
// the caller waits for the existing download to finish and then returns its result.
func (m *NativeLibManager) EnsureLibraries(ctx context.Context) error {
	if m.IsReady() && m.IsVadModelReady() {
		m.mu.Lock()
		m.status = NativeLibStatus{State: NativeLibStateDone, Progress: 100}
		m.mu.Unlock()
		return nil
	}

	// Wait for an in-progress download or start a new one.
	m.mu.Lock()
	if m.downloading {
		// Wait for the current download to finish. The cond releases the
		// lock while waiting and re-acquires it before returning.
		for m.downloading {
			m.downloadWait.Wait()
		}
		m.mu.Unlock()
		// Check if the wait succeeded.
		if m.IsReady() && m.IsVadModelReady() {
			return nil
		}
		// Prior download failed; caller can retry.
		return fmt.Errorf("native libraries download failed, please retry")
	}

	m.downloading = true
	m.status = NativeLibStatus{State: NativeLibStateDownloading, Progress: 0}
	m.mu.Unlock()

	err := m.downloadAndExtract(ctx)

	m.mu.Lock()
	m.downloading = false
	if err != nil {
		m.status = NativeLibStatus{State: NativeLibStateFailed, Error: err.Error()}
	} else {
		m.status = NativeLibStatus{State: NativeLibStateDone, Progress: 100}
	}
	m.downloadWait.Broadcast()
	m.mu.Unlock()

	return err
}

// downloadAndExtract downloads the platform-specific archive, extracts the
// required libraries into libDir, and downloads the silero VAD model. Steps
// for resources that are already on disk are skipped.
func (m *NativeLibManager) downloadAndExtract(ctx context.Context) error {
	// Download and extract native libraries (onnxruntime + sherpa-onnx).
	if !m.IsReady() {
		if err := m.downloadLibraries(ctx); err != nil {
			return err
		}
	}

	// Download silero VAD model separately (small file, ~629KB).
	if !m.IsVadModelReady() {
		vadStart := time.Now()
		if err := downloadFile(ctx, sileroVadDownloadURL, m.VadModelPath(), func(percent int) {
			// VAD model is small; no separate progress phase needed.
		}); err != nil {
			return fmt.Errorf("download silero VAD model: %w", err)
		}
		util.GetLogger().Info(ctx, fmt.Sprintf("dictation native lib: downloaded silero_vad.onnx cost=%dms", time.Since(vadStart).Milliseconds()))
	}

	return nil
}

// downloadLibraries downloads and extracts the platform-specific archive
// containing onnxruntime + sherpa-onnx native libraries.
func (m *NativeLibManager) downloadLibraries(ctx context.Context) error {
	url, err := nativeLibDownloadURL(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp(m.libDir, ".downloading-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download archive.
	archivePath := filepath.Join(tmpDir, "native.tar.bz2")
	downloadStart := time.Now()
	if err := downloadFile(ctx, url, archivePath, func(percent int) {
		m.mu.Lock()
		m.status = NativeLibStatus{State: NativeLibStateDownloading, Progress: percent}
		m.mu.Unlock()
	}); err != nil {
		return fmt.Errorf("download native libraries: %w", err)
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("dictation native lib: downloaded %s cost=%dms", url, time.Since(downloadStart).Milliseconds()))

	// Extract archive.
	m.mu.Lock()
	m.status = NativeLibStatus{State: NativeLibStateExtracting, Progress: 100}
	m.mu.Unlock()

	extractStart := time.Now()
	if err := extractTarBz2(archivePath, tmpDir); err != nil {
		return fmt.Errorf("extract native libraries: %w", err)
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("dictation native lib: extracted cost=%dms", time.Since(extractStart).Milliseconds()))

	// Locate the lib/ directory inside the extracted archive.
	libDir := m.findExtractedLibDir(tmpDir)
	if libDir == "" {
		return fmt.Errorf("no lib/ directory found in native library archive")
	}

	// Copy only the three libraries we need from the extracted archive.
	for _, name := range sherpaLibraryNames() {
		src := filepath.Join(libDir, name)
		dst := filepath.Join(m.libDir, name)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy %s: %w", name, err)
		}
	}

	return nil
}

// findExtractedLibDir locates the lib/ subdirectory inside the extracted
// archive directory. The sherpa-onnx release archives use a top-level
// directory like sherpa-onnx-v1.13.4-win-x64-shared-MD-Release-lib/lib/.
func (m *NativeLibManager) findExtractedLibDir(tmpDir string) string {
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		libPath := filepath.Join(tmpDir, entry.Name(), "lib")
		if util.IsDirExists(libPath) {
			return libPath
		}
	}
	return ""
}

// copyFile copies a file from src to dst, creating parent dirs as needed.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}