package ocr

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
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

	"wox/util"
)

// NativeLibVersion identifies the Wox-owned, signed native dependency release.
const NativeLibVersion = "v1.13.4-wox.1"

// ONNXRuntimeVersion identifies the ONNX Runtime ABI bundled with the signed
// sherpa-onnx release.
const ONNXRuntimeVersion = "1.27.0"

const (
	nativeLibReleaseBaseURL           = "https://github.com/Wox-launcher/Wox.Dictation.Native.Dependecies/releases/download/" + NativeLibVersion
	nativeLibReleaseManifestURL       = nativeLibReleaseBaseURL + "/manifest.json"
	nativeLibReleaseManifestSignature = nativeLibReleaseBaseURL + "/manifest.sig"
	nativeLibManifestPublicKeyDER     = "MCowBQYDK2VwAyEAEGuDVKMKBl3z0BFqbZJpOLraySwug7wCdH0FqCKgEp8="
	nativeLibCachedManifestName       = ".wox-native-manifest.json"
	nativeLibCachedSignatureName      = ".wox-native-manifest.sig"
)

type nativeLibReleaseManifest struct {
	SchemaVersion  int                         `json:"schema_version"`
	ReleaseVersion string                      `json:"release_version"`
	Packages       map[string]nativeLibPackage `json:"packages"`
}

type nativeLibPackage struct {
	URL    string                   `json:"url"`
	SHA256 string                   `json:"sha256"`
	Size   int64                    `json:"size"`
	Files  map[string]nativeLibFile `json:"files"`
}

type nativeLibFile struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

func nativeLibraryNames() []string {
	return append([]string{onnxRuntimeLibraryName()}, sherpaLibraryNames()...)
}

// nativeLibraryPath places ONNX Runtime separately from sherpa-onnx files.
func nativeLibraryPath(sherpaLibDir string, onnxRuntimeLibDir string, name string) string {
	directory := sherpaLibDir
	if name == onnxRuntimeLibraryName() {
		directory = onnxRuntimeLibDir
	}
	return filepath.Join(directory, name)
}

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
// specific sherpa-onnx libraries and their shared ONNX Runtime dependency.
type NativeLibManager struct {
	sherpaLibDir      string
	onnxRuntimeLibDir string

	mu     sync.Mutex
	status NativeLibStatus

	// downloading is true while a download is in progress. Callers that
	// arrive during a download wait on downloadWait; the downloader signals
	// it when done. After the signal, downloading is cleared so a failed
	// download can be retried on the next EnsureNativeLibraries call.
	downloading  bool
	downloadWait *sync.Cond
}

var nativeLibManagers = util.NewHashMap[string, *NativeLibManager]()

// NewNativeLibManager creates a native library manager with separate
// directories for sherpa-onnx and ONNX Runtime.
func NewNativeLibManager(sherpaLibDir string, onnxRuntimeLibDir string) (*NativeLibManager, error) {
	key := strings.Join([]string{sherpaLibDir, onnxRuntimeLibDir}, "\x00")
	if manager, ok := nativeLibManagers.Load(key); ok {
		return manager, nil
	}

	for _, dir := range []string{sherpaLibDir, onnxRuntimeLibDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create native runtime directory: %w", err)
		}
	}
	m := &NativeLibManager{
		sherpaLibDir:      sherpaLibDir,
		onnxRuntimeLibDir: onnxRuntimeLibDir,
	}
	m.downloadWait = sync.NewCond(&m.mu)
	manager, _ := nativeLibManagers.LoadOrStore(key, m)
	return manager, nil
}

// OnnxRuntimeLibraryPath returns the absolute path of the shared ONNX Runtime library.
func (m *NativeLibManager) OnnxRuntimeLibraryPath() string {
	return filepath.Join(m.onnxRuntimeLibDir, onnxRuntimeLibraryName())
}

// IsReady checks the signed cached manifest and every native library digest.
func (m *NativeLibManager) IsReady() bool {
	manifestData, err := os.ReadFile(filepath.Join(m.sherpaLibDir, nativeLibCachedManifestName))
	if err != nil {
		return false
	}
	signature, err := os.ReadFile(filepath.Join(m.sherpaLibDir, nativeLibCachedSignatureName))
	if err != nil {
		return false
	}
	manifest, err := verifyNativeLibReleaseManifest(manifestData, signature)
	if err != nil {
		return false
	}
	packageInfo, err := manifest.packageForCurrentPlatform()
	if err != nil {
		return false
	}
	return validateNativeLibraryFiles(m.sherpaLibDir, m.onnxRuntimeLibDir, packageInfo) == nil
}

// GetStatus returns the current download status.
func (m *NativeLibManager) GetStatus() NativeLibStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

// EnsureNativeLibraries downloads and extracts the signed native runtime.
func (m *NativeLibManager) EnsureNativeLibraries(ctx context.Context) error {
	return m.ensure(ctx)
}

func (m *NativeLibManager) ensure(ctx context.Context) error {
	ready := func() bool {
		return m.IsReady()
	}
	if ready() {
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
		if ready() {
			return nil
		}
		// Prior download failed; caller can retry.
		return fmt.Errorf("native libraries download failed, please retry")
	}

	m.downloading = true
	m.status = NativeLibStatus{State: NativeLibStateDownloading, Progress: 0}
	m.mu.Unlock()

	err := m.downloadLibraries(ctx)

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

// downloadLibraries downloads and extracts the platform-specific archive
// containing onnxruntime + sherpa-onnx native libraries.
func (m *NativeLibManager) downloadLibraries(ctx context.Context) error {
	tmpDir, err := os.MkdirTemp(m.sherpaLibDir, ".downloading-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	manifestPath := filepath.Join(tmpDir, "manifest.json")
	signaturePath := filepath.Join(tmpDir, "manifest.sig")
	if err := downloadFile(ctx, nativeLibReleaseManifestURL, manifestPath, nil); err != nil {
		return fmt.Errorf("download native library manifest: %w", err)
	}
	if err := downloadFile(ctx, nativeLibReleaseManifestSignature, signaturePath, nil); err != nil {
		return fmt.Errorf("download native library manifest signature: %w", err)
	}
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read native library manifest: %w", err)
	}
	signature, err := os.ReadFile(signaturePath)
	if err != nil {
		return fmt.Errorf("read native library manifest signature: %w", err)
	}
	manifest, err := verifyNativeLibReleaseManifest(manifestData, signature)
	if err != nil {
		return err
	}
	packageInfo, err := manifest.packageForCurrentPlatform()
	if err != nil {
		return err
	}

	// Download archive.
	archivePath := filepath.Join(tmpDir, "native.tar.bz2")
	downloadStart := time.Now()
	if err := downloadFile(ctx, packageInfo.URL, archivePath, func(percent int) {
		m.mu.Lock()
		m.status = NativeLibStatus{State: NativeLibStateDownloading, Progress: percent}
		m.mu.Unlock()
	}); err != nil {
		return fmt.Errorf("download native libraries: %w", err)
	}
	if err := verifyFileDigest(archivePath, packageInfo.SHA256, packageInfo.Size); err != nil {
		return fmt.Errorf("verify native library archive: %w", err)
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("OCR native runtime: downloaded %s cost=%dms", packageInfo.URL, time.Since(downloadStart).Milliseconds()))

	// Extract archive.
	m.mu.Lock()
	m.status = NativeLibStatus{State: NativeLibStateExtracting, Progress: 100}
	m.mu.Unlock()

	extractStart := time.Now()
	if err := extractTarBz2(archivePath, tmpDir); err != nil {
		return fmt.Errorf("extract native libraries: %w", err)
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("OCR native runtime: extracted cost=%dms", time.Since(extractStart).Milliseconds()))

	// Locate the lib/ directory inside the extracted archive.
	libDir := m.findExtractedLibDir(tmpDir)
	if libDir == "" {
		return fmt.Errorf("no lib/ directory found in native library archive")
	}

	if err := validateExtractedNativeLibraryFiles(libDir, packageInfo); err != nil {
		return fmt.Errorf("verify extracted native libraries: %w", err)
	}
	if err := m.installNativeLibraries(libDir, packageInfo, manifestData, signature); err != nil {
		return fmt.Errorf("install native libraries: %w", err)
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

// packageForCurrentPlatform returns the package covered by the signed manifest.
func (m *nativeLibReleaseManifest) packageForCurrentPlatform() (nativeLibPackage, error) {
	platform := runtime.GOOS + "-" + runtime.GOARCH
	packageInfo, ok := m.Packages[platform]
	if !ok {
		return nativeLibPackage{}, fmt.Errorf("native library release does not support %s", platform)
	}
	return packageInfo, nil
}

// verifyNativeLibReleaseManifest authenticates and parses a release manifest.
func verifyNativeLibReleaseManifest(manifestData, signature []byte) (*nativeLibReleaseManifest, error) {
	publicKeyDER, err := base64.StdEncoding.DecodeString(nativeLibManifestPublicKeyDER)
	if err != nil {
		return nil, fmt.Errorf("decode native library manifest public key: %w", err)
	}
	parsedPublicKey, err := x509.ParsePKIXPublicKey(publicKeyDER)
	if err != nil {
		return nil, fmt.Errorf("parse native library manifest public key: %w", err)
	}
	publicKey, ok := parsedPublicKey.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("native library manifest public key is not Ed25519")
	}
	if !ed25519.Verify(publicKey, manifestData, signature) {
		return nil, fmt.Errorf("native library manifest signature is invalid")
	}

	var manifest nativeLibReleaseManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("parse native library manifest: %w", err)
	}
	if manifest.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported native library manifest schema: %d", manifest.SchemaVersion)
	}
	if manifest.ReleaseVersion != NativeLibVersion {
		return nil, fmt.Errorf("native library manifest version mismatch: expected %s, got %s", NativeLibVersion, manifest.ReleaseVersion)
	}
	return &manifest, nil
}

// validateExtractedNativeLibraryFiles verifies an archive before files are
// separated into their ONNX Runtime and sherpa-onnx directories.
func validateExtractedNativeLibraryFiles(libDir string, packageInfo nativeLibPackage) error {
	return validateNativeLibraryFilesAtPaths(libDir, libDir, packageInfo)
}

// validateNativeLibraryFiles verifies installed files after they are separated
// into their runtime-specific directories.
func validateNativeLibraryFiles(sherpaLibDir string, onnxRuntimeLibDir string, packageInfo nativeLibPackage) error {
	return validateNativeLibraryFilesAtPaths(sherpaLibDir, onnxRuntimeLibDir, packageInfo)
}

func validateNativeLibraryFilesAtPaths(sherpaLibDir string, onnxRuntimeLibDir string, packageInfo nativeLibPackage) error {
	requiredNames := append([]string(nil), nativeLibraryNames()...)
	manifestNames := make([]string, 0, len(packageInfo.Files))
	for name := range packageInfo.Files {
		manifestNames = append(manifestNames, name)
	}
	sort.Strings(requiredNames)
	sort.Strings(manifestNames)
	if strings.Join(requiredNames, "\x00") != strings.Join(manifestNames, "\x00") {
		return fmt.Errorf("native library manifest allowlist mismatch")
	}

	for _, name := range requiredNames {
		fileInfo := packageInfo.Files[name]
		if err := verifyFileDigest(nativeLibraryPath(sherpaLibDir, onnxRuntimeLibDir, name), fileInfo.SHA256, fileInfo.Size); err != nil {
			return fmt.Errorf("verify %s: %w", name, err)
		}
	}
	if err := validateNativeLibraryPlatformSignaturesForPaths(sherpaLibDir, onnxRuntimeLibDir, requiredNames); err != nil {
		return err
	}
	return nil
}

// verifyFileDigest compares a file against its signed size and SHA-256 digest.
func verifyFileDigest(path, expectedDigest string, expectedSize int64) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}
	if expectedSize <= 0 || stat.Size() != expectedSize {
		return fmt.Errorf("size mismatch: expected %d, got %d", expectedSize, stat.Size())
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	actualDigest := hex.EncodeToString(hasher.Sum(nil))
	if len(expectedDigest) != sha256.Size*2 || !strings.EqualFold(actualDigest, expectedDigest) {
		return fmt.Errorf("SHA-256 mismatch: expected %s, got %s", expectedDigest, actualDigest)
	}
	return nil
}

// installNativeLibraries atomically publishes verified files and commits the signed manifest last.
func (m *NativeLibManager) installNativeLibraries(sourceDir string, packageInfo nativeLibPackage, manifestData, signature []byte) error {
	_ = os.Remove(filepath.Join(m.sherpaLibDir, nativeLibCachedManifestName))
	_ = os.Remove(filepath.Join(m.sherpaLibDir, nativeLibCachedSignatureName))

	for _, name := range nativeLibraryNames() {
		source := filepath.Join(sourceDir, name)
		destination := nativeLibraryPath(m.sherpaLibDir, m.onnxRuntimeLibDir, name)
		temporaryDestination := destination + ".new"
		_ = os.Remove(temporaryDestination)
		if err := copyFile(source, temporaryDestination); err != nil {
			return fmt.Errorf("copy %s: %w", name, err)
		}
		if err := verifyFileDigest(temporaryDestination, packageInfo.Files[name].SHA256, packageInfo.Files[name].Size); err != nil {
			_ = os.Remove(temporaryDestination)
			return fmt.Errorf("verify copied %s: %w", name, err)
		}
		_ = os.Remove(destination)
		if err := os.Rename(temporaryDestination, destination); err != nil {
			_ = os.Remove(temporaryDestination)
			return fmt.Errorf("replace %s: %w", name, err)
		}
	}
	if err := writeFileAtomically(filepath.Join(m.sherpaLibDir, nativeLibCachedManifestName), manifestData, 0644); err != nil {
		return err
	}
	if err := writeFileAtomically(filepath.Join(m.sherpaLibDir, nativeLibCachedSignatureName), signature, 0644); err != nil {
		_ = os.Remove(filepath.Join(m.sherpaLibDir, nativeLibCachedManifestName))
		return err
	}
	return nil
}

// writeFileAtomically replaces a cache metadata file without exposing partial contents.
func writeFileAtomically(path string, data []byte, permission os.FileMode) error {
	temporaryPath := path + ".new"
	_ = os.Remove(temporaryPath)
	if err := os.WriteFile(temporaryPath, data, permission); err != nil {
		return err
	}
	_ = os.Remove(path)
	if err := os.Rename(temporaryPath, path); err != nil {
		_ = os.Remove(temporaryPath)
		return err
	}
	return nil
}

// copyFile copies a file from src to dst, creating parent dirs as needed.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(destination, source); err != nil {
		destination.Close()
		return err
	}
	return destination.Close()
}

func sherpaLibraryNames() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"sherpa-onnx-cxx-api.dll", "sherpa-onnx-c-api.dll"}
	case "darwin":
		return []string{"libsherpa-onnx-cxx-api.dylib", "libsherpa-onnx-c-api.dylib"}
	default:
		return []string{"libsherpa-onnx-cxx-api.so", "libsherpa-onnx-c-api.so"}
	}
}

func onnxRuntimeLibraryName() string {
	switch runtime.GOOS {
	case "windows":
		return "onnxruntime.dll"
	case "darwin":
		return "libonnxruntime." + ONNXRuntimeVersion + ".dylib"
	default:
		return "libonnxruntime.so"
	}
}
