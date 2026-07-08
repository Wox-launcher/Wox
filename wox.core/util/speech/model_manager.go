package speech

import (
	"archive/tar"
	"compress/bzip2"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"wox/util"
)

// ModelInfo describes a downloadable ASR model.
type ModelInfo struct {
	ID          string
	DisplayName string
	Description string
	Languages   string
	Recommended bool
	DownloadURL string
	ModelType   string
	Language    string
	SizeMB      int
}

// RecommendedModels is the list of models offered to users for first-time
// download. They are ordered by recommendation (most useful first).
var RecommendedModels = []ModelInfo{
	{
		ID:          "sherpa-onnx-qwen3-asr-0.6B-int8-2026-03-25",
		DisplayName: "Qwen3-ASR 0.6B",
		Description: "i18n:plugin_dictation_model_qwen3_desc",
		Languages:   "i18n:plugin_dictation_model_qwen3_lang",
		Recommended: true,
		DownloadURL: "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-qwen3-asr-0.6B-int8-2026-03-25.tar.bz2",
		ModelType:   "qwen3_asr",
		Language:    "multi",
		SizeMB:      600,
	},
	{
		ID:          "sherpa-onnx-sense-voice-zh-en-ja-ko-yue-int8-2024-07-17",
		DisplayName: "SenseVoice-Small",
		Description: "i18n:plugin_dictation_model_sensevoice_small_desc",
		Languages:   "i18n:plugin_dictation_model_sensevoice_small_lang",
		Recommended: false,
		DownloadURL: "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-int8-2024-07-17.tar.bz2",
		ModelType:   "sense_voice",
		Language:    "auto",
		SizeMB:      155,
	},
}

// ModelManager handles model discovery, download, and verification.
type ModelManager struct {
	modelsDir string

	// downloadStatus tracks ongoing and completed downloads, keyed by model ID.
	downloadStatusMu sync.RWMutex
	downloadStatus   map[string]*DownloadStatus
}

// DownloadStatus tracks the state of a model download.
type DownloadStatus struct {
	ModelID  string
	Progress int // 0-100
	State    DownloadState
	Error    string
}

// DownloadState describes the lifecycle of a model download.
type DownloadState string

const (
	DownloadStateIdle        DownloadState = "idle"
	DownloadStateDownloading DownloadState = "downloading"
	DownloadStateExtracting  DownloadState = "extracting"
	DownloadStateFinalizing  DownloadState = "finalizing"
	DownloadStateDone        DownloadState = "done"
	DownloadStateFailed      DownloadState = "failed"
)

// NewModelManager creates a model manager rooted at the given directory.
// The directory is created if it does not exist.
func NewModelManager(modelsDir string) (*ModelManager, error) {
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create models directory: %w", err)
	}
	return &ModelManager{
		modelsDir:      modelsDir,
		downloadStatus: make(map[string]*DownloadStatus),
	}, nil
}

// ListLocalModels scans the models directory and returns info about each
// model that has the required files for its type.
func (m *ModelManager) ListLocalModels() ([]LocalModel, error) {
	entries, err := os.ReadDir(m.modelsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read models directory: %w", err)
	}

	var models []LocalModel
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		modelDir := filepath.Join(m.modelsDir, entry.Name())
		info, ok := m.inspectModelDir(modelDir)
		if ok {
			models = append(models, info)
		}
	}
	return models, nil
}

// LocalModel describes a model that exists on disk.
type LocalModel struct {
	ID          string
	Path        string
	ModelType   string
	DisplayName string
}

// inspectModelDir checks whether a directory contains a valid model and
// returns its info. The model type is inferred from the file names.
func (m *ModelManager) inspectModelDir(dir string) (LocalModel, bool) {
	// Check for Qwen3-ASR (conv_frontend.onnx + encoder.int8.onnx + decoder.int8.onnx + tokenizer dir).
	if fileExists(filepath.Join(dir, "conv_frontend.onnx")) &&
		fileExists(filepath.Join(dir, "encoder.int8.onnx")) &&
		fileExists(filepath.Join(dir, "decoder.int8.onnx")) &&
		isDir(filepath.Join(dir, "tokenizer")) {
		return LocalModel{
			ID:          filepath.Base(dir),
			Path:        dir,
			ModelType:   "qwen3_asr",
			DisplayName: filepath.Base(dir),
		}, true
	}

	// Check for SenseVoice-Small (model.int8.onnx/model.onnx + tokens.txt).
	if (fileExists(filepath.Join(dir, "model.int8.onnx")) || fileExists(filepath.Join(dir, "model.onnx"))) &&
		fileExists(filepath.Join(dir, "tokens.txt")) {
		return LocalModel{
			ID:          filepath.Base(dir),
			Path:        dir,
			ModelType:   "sense_voice",
			DisplayName: filepath.Base(dir),
		}, true
	}

	// Check for streaming zipformer (encoder/decoder/joiner onnx files).
	encoderFiles, _ := filepath.Glob(filepath.Join(dir, "encoder*.onnx"))
	if len(encoderFiles) > 0 {
		if fileExists(filepath.Join(dir, "tokens.txt")) {
			return LocalModel{
				ID:          filepath.Base(dir),
				Path:        dir,
				ModelType:   "zipformer2",
				DisplayName: filepath.Base(dir),
			}, true
		}
	}

	// Check for paraformer (encoder.int8.onnx + decoder.int8.onnx).
	if fileExists(filepath.Join(dir, "encoder.int8.onnx")) && fileExists(filepath.Join(dir, "tokens.txt")) {
		return LocalModel{
			ID:          filepath.Base(dir),
			Path:        dir,
			ModelType:   "paraformer",
			DisplayName: filepath.Base(dir),
		}, true
	}

	return LocalModel{}, false
}

// DownloadModel downloads and extracts a model archive. The onProgress
// callback receives download progress as a percentage (0-100).
// The model is extracted to a subdirectory named after the model ID inside
// the models directory. If the target directory already exists and contains
// a valid model, the download is skipped.
func (m *ModelManager) DownloadModel(ctx context.Context, info ModelInfo, onProgress func(percent int)) error {
	targetDir := filepath.Join(m.modelsDir, info.ID)

	// Skip if already downloaded.
	if _, ok := m.inspectModelDir(targetDir); ok {
		m.setDownloadStatus(info.ID, DownloadStateDone, 100, "")
		return nil
	}

	// Prevent concurrent downloads of the same model.
	if m.IsDownloading(info.ID) {
		return fmt.Errorf("model %s is already downloading", info.ID)
	}

	m.setDownloadStatus(info.ID, DownloadStateDownloading, 0, "")
	defer func() {
		// If the function exits without reaching Done, mark as failed if still downloading.
		if m.IsDownloading(info.ID) {
			m.setDownloadStatus(info.ID, DownloadStateFailed, 0, "download interrupted")
		}
	}()

	// Create a temporary directory for download + extraction.
	tmpDir, err := os.MkdirTemp(m.modelsDir, ".downloading-*")
	if err != nil {
		m.setDownloadStatus(info.ID, DownloadStateFailed, 0, err.Error())
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download the archive.
	archivePath := filepath.Join(tmpDir, "model.tar.bz2")
	downloadStart := time.Now()
	if err := downloadFile(ctx, info.DownloadURL, archivePath, func(percent int) {
		m.setDownloadStatus(info.ID, DownloadStateDownloading, percent, "")
		if onProgress != nil {
			onProgress(percent)
		}
	}); err != nil {
		m.setDownloadStatus(info.ID, DownloadStateFailed, 0, err.Error())
		return fmt.Errorf("failed to download model: %w", err)
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("dictation model download: downloaded %s cost=%dms", info.ID, time.Since(downloadStart).Milliseconds()))

	// Set status to extracting so the UI can show "extracting" instead of
	// being stuck at 100% download progress.
	m.setDownloadStatus(info.ID, DownloadStateExtracting, 100, "")

	// Extract the tar.bz2 archive. The archive typically contains a single
	// top-level directory with the model files inside.
	extractStart := time.Now()
	if err := extractTarBz2(archivePath, tmpDir); err != nil {
		m.setDownloadStatus(info.ID, DownloadStateFailed, 0, err.Error())
		return fmt.Errorf("failed to extract model: %w", err)
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("dictation model download: extracted %s cost=%dms", info.ID, time.Since(extractStart).Milliseconds()))

	m.setDownloadStatus(info.ID, DownloadStateFinalizing, 100, "")
	finalizeStart := time.Now()

	// Find the extracted directory (the archive's top-level folder).
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		m.setDownloadStatus(info.ID, DownloadStateFailed, 0, err.Error())
		return fmt.Errorf("failed to read extracted files: %w", err)
	}
	var extractedDir string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			extractedDir = filepath.Join(tmpDir, entry.Name())
			break
		}
	}
	if extractedDir == "" {
		m.setDownloadStatus(info.ID, DownloadStateFailed, 0, "no model directory found in archive")
		return fmt.Errorf("no model directory found in archive")
	}

	// Move the extracted directory to the final location.
	if err := os.Rename(extractedDir, targetDir); err != nil {
		m.setDownloadStatus(info.ID, DownloadStateFailed, 0, err.Error())
		return fmt.Errorf("failed to move model to final location: %w", err)
	}

	m.setDownloadStatus(info.ID, DownloadStateDone, 100, "")
	util.GetLogger().Info(ctx, fmt.Sprintf("dictation model download: finalized %s cost=%dms", info.ID, time.Since(finalizeStart).Milliseconds()))
	return nil
}

// IsDownloading reports whether a model download is currently in progress.
func (m *ModelManager) IsDownloading(modelID string) bool {
	m.downloadStatusMu.RLock()
	defer m.downloadStatusMu.RUnlock()
	s, ok := m.downloadStatus[modelID]
	return ok && (s.State == DownloadStateDownloading || s.State == DownloadStateExtracting || s.State == DownloadStateFinalizing)
}

// GetDownloadStatus returns the current download status for a model.
func (m *ModelManager) GetDownloadStatus(modelID string) *DownloadStatus {
	m.downloadStatusMu.RLock()
	defer m.downloadStatusMu.RUnlock()
	if s, ok := m.downloadStatus[modelID]; ok {
		copy := *s
		return &copy
	}
	return nil
}

// DeleteModel removes a model from disk by its ID. The model directory is
// deleted entirely. If the model does not exist on disk, the call is a no-op.
// The download status entry (if any) is also cleared.
func (m *ModelManager) DeleteModel(modelID string) error {
	targetDir := filepath.Join(m.modelsDir, modelID)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		m.setDownloadStatus(modelID, DownloadStateIdle, 0, "")
		return nil
	}

	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("failed to delete model directory: %w", err)
	}

	m.setDownloadStatus(modelID, DownloadStateIdle, 0, "")
	return nil
}

// setDownloadStatus updates the download status for a model.
func (m *ModelManager) setDownloadStatus(modelID string, state DownloadState, progress int, errMsg string) {
	m.downloadStatusMu.Lock()
	defer m.downloadStatusMu.Unlock()
	m.downloadStatus[modelID] = &DownloadStatus{
		ModelID:  modelID,
		Progress: progress,
		State:    state,
		Error:    errMsg,
	}
}

// downloadFile downloads a file with progress reporting. The onProgress
// callback is called with the percentage of bytes downloaded (0-100).
// If the server does not provide Content-Length, progress is reported as 0.
func downloadFile(ctx context.Context, url, destPath string, onProgress func(int)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	totalSize := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 32*1024)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			downloaded += int64(n)
			if totalSize > 0 && onProgress != nil {
				percent := int(downloaded * 100 / totalSize)
				onProgress(percent)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// extractTarBz2 decompresses and extracts a .tar.bz2 archive to destDir.
func extractTarBz2(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	bz2Reader := bzip2.NewReader(f)
	tarReader := tar.NewReader(bz2Reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)

		// Prevent path traversal.
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(filepath.Separator)) {
			return fmt.Errorf("archive contains path traversal: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tarReader); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// ModelsDir returns the models directory path.
func (m *ModelManager) ModelsDir() string {
	return m.modelsDir
}

// InspectModelDir checks whether a directory contains a valid model and
// returns its info. Returns (info, false) if not valid.
func (m *ModelManager) InspectModelDir(dir string) (LocalModel, bool) {
	return m.inspectModelDir(dir)
}
