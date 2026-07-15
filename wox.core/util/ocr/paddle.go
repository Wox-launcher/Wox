package ocr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"wox/util"
	"wox/util/speech"

	paddleocr "github.com/multippt/gopaddleocr/pkg/ocr"
	classifystub "github.com/multippt/gopaddleocr/pkg/ocr/classify/stub"
	"github.com/multippt/gopaddleocr/pkg/ocr/common"
	paddledet "github.com/multippt/gopaddleocr/pkg/ocr/detect/paddleocr"
	paddlerec "github.com/multippt/gopaddleocr/pkg/ocr/recognize/paddleocr"
	ort "github.com/yalue/onnxruntime_go"
	_ "golang.org/x/image/webp"
	"gopkg.in/yaml.v3"
)

const (
	ModelSystem                  = "system"
	ModelPaddlePPOCRv6Small      = "paddle_ppocrv6_small"
	EnginePaddlePPOCRv6Small     = ModelPaddlePPOCRv6Small
	paddleCharacterDictionary    = "ppocr_keys_v5.txt"
	paddleModelDownloadBufferLen = 32 * 1024
)

type ModelDownloadState string

const (
	ModelDownloadStateNotDownloaded ModelDownloadState = "not_downloaded"
	ModelDownloadStateDownloading   ModelDownloadState = "downloading"
	ModelDownloadStateFinalizing    ModelDownloadState = "finalizing"
	ModelDownloadStateDownloaded    ModelDownloadState = "downloaded"
	ModelDownloadStateFailed        ModelDownloadState = "failed"
)

// PaddleModelStatus reports the local PP-OCRv6 small model state.
type PaddleModelStatus struct {
	ID               string             `json:"ID"`
	DisplayName      string             `json:"DisplayName"`
	Description      string             `json:"Description"`
	Languages        string             `json:"Languages"`
	Recommended      bool               `json:"Recommended"`
	Status           ModelDownloadState `json:"Status"`
	DownloadProgress int                `json:"DownloadProgress"`
	SizeMB           int                `json:"SizeMB"`
	Error            string             `json:"Error"`
}

// PaddleEngineStatus reports the shared ONNX Runtime download state required
// by PaddleOCR inference.
type PaddleEngineStatus struct {
	State    string `json:"State"`
	Progress int    `json:"Progress"`
	Error    string `json:"Error"`
	Ready    bool   `json:"Ready"`
}

type paddleModelFile struct {
	path   string
	url    string
	sha256 string
	size   int64
}

var paddlePPOCRv6SmallFiles = []paddleModelFile{
	{
		path:   "det/inference.onnx",
		url:    "https://huggingface.co/PaddlePaddle/PP-OCRv6_small_det_onnx/resolve/main/inference.onnx",
		sha256: "d73e0058b7a8086bbd57f3d10b8bcd4ff95363f67e06e2762b5e814fe9c9410e",
		size:   9880512,
	},
	{
		path:   "rec/inference.onnx",
		url:    "https://huggingface.co/PaddlePaddle/PP-OCRv6_small_rec_onnx/resolve/main/inference.onnx",
		sha256: "5435fd747c9e0efe15a96d0b378d5bd157e9492ed8fd80edf08f30d02fa24634",
		size:   21159378,
	},
	{
		path:   "rec/inference.yml",
		url:    "https://huggingface.co/PaddlePaddle/PP-OCRv6_small_rec_onnx/resolve/main/inference.yml",
		sha256: "ab078671bb49f06228eadccd34f1bb501e157f7a047095ffb943ba81512c77d1",
		size:   150579,
	},
}

// PaddleModelManager owns the optional PP-OCRv6 small model and its ONNX
// Runtime workflow. It shares the Wox-managed native runtime with dictation.
type PaddleModelManager struct {
	modelsDir      string
	modelDir       string
	runtimeManager *speech.NativeLibManager

	downloadMu sync.Mutex
	status     PaddleModelStatus

	engineMu sync.Mutex
	workflow *paddleocr.Workflow
	runMu    sync.Mutex
}

var paddleModelManagerOnce sync.Once
var paddleModelManager *PaddleModelManager
var paddleModelManagerErr error

// GetPaddleModelManager returns the process-wide manager for the optional
// PP-OCRv6 small model.
func GetPaddleModelManager() (*PaddleModelManager, error) {
	paddleModelManagerOnce.Do(func() {
		location := util.GetLocation()
		modelsDir := location.GetOCRModelsDirectory()
		if err := os.MkdirAll(modelsDir, 0755); err != nil {
			paddleModelManagerErr = fmt.Errorf("create OCR models directory: %w", err)
			return
		}

		runtimeManager, err := speech.NewNativeLibManager(
			location.GetSherpaONNXRuntimeDirectory(speech.NativeLibVersion),
			location.GetONNXRuntimeDirectory(speech.ONNXRuntimeVersion),
			filepath.Join(location.GetDictationModelsDirectory(), "silero-vad"),
		)
		if err != nil {
			paddleModelManagerErr = fmt.Errorf("create ONNX runtime manager: %w", err)
			return
		}

		paddleModelManager = &PaddleModelManager{
			modelsDir:      modelsDir,
			modelDir:       filepath.Join(modelsDir, ModelPaddlePPOCRv6Small),
			runtimeManager: runtimeManager,
			status:         newPaddleModelStatus(ModelDownloadStateNotDownloaded, 0, ""),
		}
	})
	if paddleModelManagerErr != nil {
		return nil, paddleModelManagerErr
	}
	return paddleModelManager, nil
}

// GetPaddleModelStatus returns a snapshot suitable for a settings model picker.
func GetPaddleModelStatus() (PaddleModelStatus, error) {
	manager, err := GetPaddleModelManager()
	if err != nil {
		return PaddleModelStatus{}, err
	}
	return manager.GetStatus(), nil
}

// GetPaddleEngineStatus returns the shared ONNX Runtime status for OCR settings.
func GetPaddleEngineStatus() (PaddleEngineStatus, error) {
	manager, err := GetPaddleModelManager()
	if err != nil {
		return PaddleEngineStatus{}, err
	}
	return manager.GetEngineStatus(), nil
}

// DownloadPaddleEngine installs the shared ONNX Runtime needed by PaddleOCR.
func DownloadPaddleEngine(ctx context.Context) error {
	manager, err := GetPaddleModelManager()
	if err != nil {
		return err
	}
	return manager.DownloadEngine(ctx)
}

// DownloadPaddleModel downloads PP-OCRv6 small and verifies every source file
// before replacing the previously installed model.
func DownloadPaddleModel(ctx context.Context) error {
	manager, err := GetPaddleModelManager()
	if err != nil {
		return err
	}
	return manager.Download(ctx)
}

// newPaddleModelStatus creates the metadata used by every OCR settings picker.
func newPaddleModelStatus(state ModelDownloadState, progress int, errMsg string) PaddleModelStatus {
	return PaddleModelStatus{
		ID:               ModelPaddlePPOCRv6Small,
		DisplayName:      "PaddleOCR v6 Small",
		Description:      "On-device text recognition powered by PP-OCRv6 Small.",
		Languages:        "Chinese, English, Japanese, and 46 Latin-script languages",
		Recommended:      true,
		Status:           state,
		DownloadProgress: progress,
		SizeMB:           30,
		Error:            errMsg,
	}
}

// GetStatus returns the current download status. Installed files are checked
// by size here so polling does not repeatedly hash 30 MB of model data.
func (m *PaddleModelManager) GetStatus() PaddleModelStatus {
	m.downloadMu.Lock()
	defer m.downloadMu.Unlock()
	if m.status.Status == ModelDownloadStateDownloading || m.status.Status == ModelDownloadStateFinalizing {
		return m.status
	}
	if m.hasInstalledModel() {
		return newPaddleModelStatus(ModelDownloadStateDownloaded, 100, "")
	}
	return m.status
}

// GetEngineStatus returns the runtime download state without requiring the
// dictation-only VAD model.
func (m *PaddleModelManager) GetEngineStatus() PaddleEngineStatus {
	status := m.runtimeManager.GetStatus()
	return PaddleEngineStatus{
		State:    string(status.State),
		Progress: status.Progress,
		Error:    status.Error,
		Ready:    m.runtimeManager.IsReady(),
	}
}

// DownloadEngine installs the shared runtime without downloading any OCR model.
func (m *PaddleModelManager) DownloadEngine(ctx context.Context) error {
	return m.runtimeManager.EnsureNativeLibraries(ctx)
}

// Download installs the model atomically so a failed or interrupted download
// never replaces the last verified local model.
func (m *PaddleModelManager) Download(ctx context.Context) error {
	m.downloadMu.Lock()
	if m.status.Status == ModelDownloadStateDownloading || m.status.Status == ModelDownloadStateFinalizing {
		m.downloadMu.Unlock()
		return fmt.Errorf("OCR model download is already in progress")
	}
	modelIsVerified := false
	if m.hasInstalledModel() {
		if err := verifyPaddleModelFiles(m.modelDir); err == nil {
			modelIsVerified = true
		}
	}
	if modelIsVerified {
		m.status = newPaddleModelStatus(ModelDownloadStateFinalizing, 100, "")
	} else {
		m.status = newPaddleModelStatus(ModelDownloadStateDownloading, 0, "")
	}
	m.downloadMu.Unlock()

	var err error
	if modelIsVerified {
		err = m.runtimeManager.EnsureNativeLibraries(ctx)
	} else {
		err = m.downloadAndInstall(ctx)
	}

	m.downloadMu.Lock()
	defer m.downloadMu.Unlock()
	if err != nil {
		m.status = newPaddleModelStatus(ModelDownloadStateFailed, 0, err.Error())
		return err
	}
	m.status = newPaddleModelStatus(ModelDownloadStateDownloaded, 100, "")
	return nil
}

// downloadAndInstall verifies the upstream files in a temporary directory,
// publishes the complete model directory, and prepares the shared runtime.
func (m *PaddleModelManager) downloadAndInstall(ctx context.Context) error {
	temporaryDir, err := os.MkdirTemp(m.modelsDir, ".downloading-*")
	if err != nil {
		return fmt.Errorf("create OCR model temporary directory: %w", err)
	}
	defer os.RemoveAll(temporaryDir)

	totalBytes := int64(0)
	for _, file := range paddlePPOCRv6SmallFiles {
		totalBytes += file.size
	}
	downloadedBytes := int64(0)
	for _, file := range paddlePPOCRv6SmallFiles {
		destination := filepath.Join(temporaryDir, file.path)
		if err := downloadPaddleModelFile(ctx, file.url, destination, file.size, file.sha256, func(current int64) {
			m.setDownloadProgress(int((downloadedBytes + current) * 100 / totalBytes))
		}); err != nil {
			return fmt.Errorf("download %s: %w", file.path, err)
		}
		downloadedBytes += file.size
	}

	m.setDownloadState(ModelDownloadStateFinalizing, 100, "")
	if err := writePaddleCharacterDictionary(filepath.Join(temporaryDir, "rec", "inference.yml"), filepath.Join(temporaryDir, "rec", paddleCharacterDictionary)); err != nil {
		return err
	}
	if err := verifyPaddleModelFiles(temporaryDir); err != nil {
		return err
	}

	backupDir := m.modelDir + ".previous"
	if err := os.RemoveAll(backupDir); err != nil {
		return fmt.Errorf("remove previous OCR model backup: %w", err)
	}
	hasPreviousModel := util.IsDirExists(m.modelDir)
	if hasPreviousModel {
		if err := os.Rename(m.modelDir, backupDir); err != nil {
			return fmt.Errorf("back up previous OCR model: %w", err)
		}
	}
	if err := os.Rename(temporaryDir, m.modelDir); err != nil {
		if !hasPreviousModel {
			return fmt.Errorf("install OCR model: %w", err)
		}
		if restoreErr := os.Rename(backupDir, m.modelDir); restoreErr != nil {
			return fmt.Errorf("install OCR model: %w; restore previous model: %v", err, restoreErr)
		}
		return fmt.Errorf("install OCR model: %w", err)
	}
	if err := os.RemoveAll(backupDir); err != nil {
		return fmt.Errorf("remove previous OCR model backup: %w", err)
	}
	return m.runtimeManager.EnsureNativeLibraries(ctx)
}

// setDownloadProgress records download progress for settings polling.
func (m *PaddleModelManager) setDownloadProgress(progress int) {
	m.setDownloadState(ModelDownloadStateDownloading, progress, "")
}

// setDownloadState updates the model lifecycle state atomically.
func (m *PaddleModelManager) setDownloadState(state ModelDownloadState, progress int, errMsg string) {
	m.downloadMu.Lock()
	defer m.downloadMu.Unlock()
	m.status = newPaddleModelStatus(state, progress, errMsg)
}

// hasInstalledModel checks the inexpensive local file shape used for status polling.
func (m *PaddleModelManager) hasInstalledModel() bool {
	for _, file := range paddlePPOCRv6SmallFiles {
		info, err := os.Stat(filepath.Join(m.modelDir, file.path))
		if err != nil || info.Size() != file.size {
			return false
		}
	}
	return util.IsFileExists(filepath.Join(m.modelDir, "rec", paddleCharacterDictionary))
}

// verifyPaddleModelFiles validates every downloaded source file before use.
func verifyPaddleModelFiles(modelDir string) error {
	for _, file := range paddlePPOCRv6SmallFiles {
		if err := verifyPaddleModelFile(filepath.Join(modelDir, file.path), file.size, file.sha256); err != nil {
			return fmt.Errorf("verify %s: %w", file.path, err)
		}
	}
	if !util.IsFileExists(filepath.Join(modelDir, "rec", paddleCharacterDictionary)) {
		return fmt.Errorf("PaddleOCR character dictionary is missing")
	}
	return nil
}

// verifyPaddleModelFile compares a model file with its pinned size and digest.
func verifyPaddleModelFile(path string, expectedSize int64, expectedSHA256 string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() != expectedSize {
		return fmt.Errorf("size mismatch: expected %d, got %d", expectedSize, info.Size())
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	if actual := hex.EncodeToString(hasher.Sum(nil)); !strings.EqualFold(actual, expectedSHA256) {
		return fmt.Errorf("SHA-256 mismatch: expected %s, got %s", expectedSHA256, actual)
	}
	return nil
}

// downloadPaddleModelFile streams one upstream file while validating its digest.
func downloadPaddleModelFile(ctx context.Context, url string, destination string, expectedSize int64, expectedSHA256 string, onProgress func(int64)) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, response.Status)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	buffer := make([]byte, paddleModelDownloadBufferLen)
	var downloaded int64
	for {
		read, readErr := response.Body.Read(buffer)
		if read > 0 {
			chunk := buffer[:read]
			if _, err := file.Write(chunk); err != nil {
				return err
			}
			if _, err := hasher.Write(chunk); err != nil {
				return err
			}
			downloaded += int64(read)
			if onProgress != nil {
				onProgress(downloaded)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if downloaded != expectedSize {
		return fmt.Errorf("size mismatch: expected %d, got %d", expectedSize, downloaded)
	}
	if actual := hex.EncodeToString(hasher.Sum(nil)); !strings.EqualFold(actual, expectedSHA256) {
		return fmt.Errorf("SHA-256 mismatch: expected %s, got %s", expectedSHA256, actual)
	}
	return nil
}

type paddleRecConfig struct {
	PostProcess struct {
		CharacterDict []string `yaml:"character_dict"`
	} `yaml:"PostProcess"`
}

// writePaddleCharacterDictionary derives the recognizer dictionary from the
// verified PaddleOCR configuration so it stays aligned with the ONNX model.
func writePaddleCharacterDictionary(configPath string, dictionaryPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read PaddleOCR recognition config: %w", err)
	}
	var config paddleRecConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse PaddleOCR recognition config: %w", err)
	}
	if len(config.PostProcess.CharacterDict) == 0 {
		return fmt.Errorf("PaddleOCR recognition config does not contain a character dictionary")
	}
	if err := os.WriteFile(dictionaryPath, []byte(strings.Join(config.PostProcess.CharacterDict, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("write PaddleOCR character dictionary: %w", err)
	}
	return nil
}

// Recognize runs the selected PP-OCRv6 small model and returns line-level
// text with the quadrilateral bounds supplied by the detector.
func (m *PaddleModelManager) Recognize(ctx context.Context, imagePath string) (Result, error) {
	if err := m.ensureWorkflow(ctx); err != nil {
		return Result{}, err
	}

	file, err := os.Open(imagePath)
	if err != nil {
		return Result{}, fmt.Errorf("open OCR image: %w", err)
	}
	defer file.Close()
	decoded, _, err := image.Decode(file)
	if err != nil {
		return Result{}, fmt.Errorf("decode OCR image: %w", err)
	}

	m.runMu.Lock()
	results, err := m.workflow.RunOCR(convertRGBToBGR(decoded))
	m.runMu.Unlock()
	if err != nil {
		return Result{}, fmt.Errorf("PaddleOCR recognition: %w", err)
	}

	ocrResult := Result{Engine: EnginePaddlePPOCRv6Small}
	lines := make([]string, 0, len(results))
	for _, line := range results {
		text := strings.TrimSpace(line.Text)
		if text == "" {
			continue
		}
		bounds := make([]Point, 0, len(line.Box))
		for _, point := range line.Box {
			bounds = append(bounds, Point{X: float64(point[0]), Y: float64(point[1])})
		}
		ocrResult.Lines = append(ocrResult.Lines, Line{Text: text, Confidence: line.Score, Bounds: bounds})
		lines = append(lines, text)
	}
	ocrResult.Text = strings.Join(lines, "\n")
	return ocrResult, nil
}

// ensureWorkflow initializes the process-wide PaddleOCR workflow on first use.
func (m *PaddleModelManager) ensureWorkflow(ctx context.Context) error {
	m.engineMu.Lock()
	defer m.engineMu.Unlock()
	if m.workflow != nil {
		return nil
	}
	if err := verifyPaddleModelFiles(m.modelDir); err != nil {
		return fmt.Errorf("PaddleOCR model is not ready: %w", err)
	}
	if err := m.runtimeManager.EnsureNativeLibraries(ctx); err != nil {
		return fmt.Errorf("prepare ONNX Runtime: %w", err)
	}

	ort.SetSharedLibraryPath(m.runtimeManager.OnnxRuntimeLibraryPath())
	if !ort.IsInitialized() {
		if err := ort.InitializeEnvironment(); err != nil {
			return fmt.Errorf("initialize ONNX Runtime: %w", err)
		}
	}

	detector := paddledet.NewModel()
	detectorConfig := detector.GetDefaultConfig().(*paddledet.ModelConfig)
	detectorConfig.Thresh = 0.2
	detectorConfig.BoxThresh = 0.45
	detectorConfig.UnclipRatio = 1.4
	detectorConfig.OnnxConfig.ModelPath = filepath.Join(m.modelDir, "det", "inference.onnx")

	recognizer := paddlerec.NewModel()
	recognizerConfig := recognizer.GetDefaultConfig().(*paddlerec.ModelConfig)
	recognizerConfig.OnnxConfig.ModelPath = filepath.Join(m.modelDir, "rec", "inference.onnx")

	configs := paddleConfigSource{configs: map[string]common.ModelConfig{
		detector.GetName():   detectorConfig,
		recognizer.GetName(): recognizerConfig,
	}}
	workflow := paddleocr.NewWorkflow(detector, classifystub.NewModel(), recognizer, configs)
	if err := workflow.Init(); err != nil {
		return fmt.Errorf("initialize PaddleOCR workflow: %w", err)
	}
	m.workflow = workflow
	return nil
}

type paddleConfigSource struct {
	configs map[string]common.ModelConfig
}

// GetConfig returns the caller-owned model configuration for a workflow stage.
func (s paddleConfigSource) GetConfig(modelName string) common.ModelConfig {
	return s.configs[modelName]
}

// convertRGBToBGR matches PaddleOCR's BGR DecodeImage preprocessing contract.
func convertRGBToBGR(source image.Image) *image.RGBA {
	bounds := source.Bounds()
	converted := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := source.At(x, y).RGBA()
			converted.SetRGBA(x, y, color.RGBA{R: uint8(b >> 8), G: uint8(g >> 8), B: uint8(r >> 8), A: uint8(a >> 8)})
		}
	}
	return converted
}
