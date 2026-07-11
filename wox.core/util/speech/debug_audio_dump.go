package speech

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"wox/util"
)

const devAudioRetentionGroups = 20

var devAudioWriterMutex sync.Mutex

type sessionAudioDump struct {
	SessionID       string
	StartedAt       time.Time
	EndedAt         time.Time
	StopReason      SessionStopReason
	DeviceID        string
	ModelType       string
	Raw             []float32
	Processed       []float32
	Stats           AudioProcessingStats
	VadConfig       VadConfig
	PrimarySegments int
	FallbackType    string
	ResultEmpty     bool
}

// DevelopmentAudioFiles points to one complete local diagnostic audio pair.
type DevelopmentAudioFiles struct {
	RawPath       string
	ProcessedPath string
}

type sessionAudioMetadata struct {
	Version               int               `json:"version"`
	SampleRate            int               `json:"sample_rate"`
	Channels              int               `json:"channels"`
	SampleFormat          string            `json:"sample_format"`
	SessionID             string            `json:"session_id"`
	StartedAt             time.Time         `json:"started_at"`
	EndedAt               time.Time         `json:"ended_at"`
	StopReason            SessionStopReason `json:"stop_reason"`
	DeviceID              string            `json:"device_id"`
	ModelType             string            `json:"model_type"`
	DurationMilliseconds  int64             `json:"duration_ms"`
	InputRMSDBFS          float64           `json:"input_rms_dbfs"`
	InputPeakDBFS         float64           `json:"input_peak_dbfs"`
	OutputRMSDBFS         float64           `json:"output_rms_dbfs"`
	OutputPeakDBFS        float64           `json:"output_peak_dbfs"`
	NoiseFloorDBFS        float64           `json:"noise_floor_dbfs"`
	AverageGainDB         float64           `json:"average_gain_db"`
	MaximumGainDB         float64           `json:"maximum_gain_db"`
	CandidateMilliseconds int64             `json:"candidate_speech_ms"`
	VadThreshold          float32           `json:"vad_threshold"`
	VadMinSpeechSeconds   float32           `json:"vad_min_speech_seconds"`
	VadMinSilenceSeconds  float32           `json:"vad_min_silence_seconds"`
	VadMaxSpeechSeconds   float32           `json:"vad_max_speech_seconds"`
	PrimarySegments       int               `json:"primary_segments"`
	FallbackType          string            `json:"fallback_type"`
	ResultEmpty           bool              `json:"result_empty"`
}

// scheduleSessionAudioDump writes development diagnostics without delaying result delivery.
func scheduleSessionAudioDump(ctx context.Context, dump sessionAudioDump) {
	if !util.IsDev() {
		return
	}
	util.Go(context.Background(), "write dictation development audio", func() {
		path, err := writeSessionAudioDump(dump)
		if err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("dictation: failed to save development audio: %s", err))
			return
		}
		util.GetLogger().Info(ctx, fmt.Sprintf("dictation: development audio saved, directory=%s", path))
	})
}

// DevelopmentAudioFilesBySession lists complete retained diagnostic groups in one directory scan.
func DevelopmentAudioFilesBySession() map[string]DevelopmentAudioFiles {
	result := make(map[string]DevelopmentAudioFiles)
	if !util.IsDev() {
		return result
	}
	root := filepath.Join(util.GetLocation().GetLogDirectory(), "dictation", "audio")
	entries, err := os.ReadDir(root)
	if err != nil {
		return result
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		separator := strings.LastIndex(entry.Name(), "_")
		if separator < 0 || separator == len(entry.Name())-1 {
			continue
		}
		sessionID := entry.Name()[separator+1:]
		rawPath := filepath.Join(root, entry.Name(), "raw.wav")
		processedPath := filepath.Join(root, entry.Name(), "processed.wav")
		if isRegularFile(rawPath) && isRegularFile(processedPath) {
			result[sessionID] = DevelopmentAudioFiles{RawPath: rawPath, ProcessedPath: processedPath}
		}
	}
	return result
}

// writeSessionAudioDump atomically commits one complete diagnostic group and enforces retention.
func writeSessionAudioDump(dump sessionAudioDump) (string, error) {
	devAudioWriterMutex.Lock()
	defer devAudioWriterMutex.Unlock()

	root := filepath.Join(util.GetLocation().GetLogDirectory(), "dictation", "audio")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", fmt.Errorf("create audio directory: %w", err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return "", fmt.Errorf("set audio directory permissions: %w", err)
	}
	if err := cleanupTemporaryAudioDirectories(root); err != nil {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("dictation: failed to clean temporary audio directories: %s", err))
	}

	baseName := dump.EndedAt.Format("20060102-150405.000") + "_" + sanitizeAudioPathPart(dump.SessionID)
	temporaryPath := filepath.Join(root, ".tmp-"+baseName)
	finalPath := filepath.Join(root, baseName)
	if err := os.Mkdir(temporaryPath, 0o700); err != nil {
		return "", fmt.Errorf("create temporary audio directory: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(temporaryPath)
		}
	}()

	if err := writePCM16WAV(filepath.Join(temporaryPath, "raw.wav"), dump.Raw); err != nil {
		return "", fmt.Errorf("write raw audio: %w", err)
	}
	if err := writePCM16WAV(filepath.Join(temporaryPath, "processed.wav"), dump.Processed); err != nil {
		return "", fmt.Errorf("write processed audio: %w", err)
	}
	metadata := sessionAudioMetadata{
		Version:               1,
		SampleRate:            audioSampleRate,
		Channels:              1,
		SampleFormat:          "pcm_s16le",
		SessionID:             dump.SessionID,
		StartedAt:             dump.StartedAt,
		EndedAt:               dump.EndedAt,
		StopReason:            dump.StopReason,
		DeviceID:              dump.DeviceID,
		ModelType:             dump.ModelType,
		DurationMilliseconds:  int64(float64(len(dump.Processed)) / audioSampleRate * 1000),
		InputRMSDBFS:          dump.Stats.InputRMSDBFS,
		InputPeakDBFS:         dump.Stats.InputPeakDBFS,
		OutputRMSDBFS:         dump.Stats.OutputRMSDBFS,
		OutputPeakDBFS:        dump.Stats.OutputPeakDBFS,
		NoiseFloorDBFS:        dump.Stats.NoiseFloorDBFS,
		AverageGainDB:         dump.Stats.AverageGainDB,
		MaximumGainDB:         dump.Stats.MaximumGainDB,
		CandidateMilliseconds: dump.Stats.CandidateDuration.Milliseconds(),
		VadThreshold:          dump.VadConfig.Threshold,
		VadMinSpeechSeconds:   dump.VadConfig.MinSpeechDuration,
		VadMinSilenceSeconds:  dump.VadConfig.MinSilenceDuration,
		VadMaxSpeechSeconds:   dump.VadConfig.MaxSpeechDuration,
		PrimarySegments:       dump.PrimarySegments,
		FallbackType:          dump.FallbackType,
		ResultEmpty:           dump.ResultEmpty,
	}
	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(temporaryPath, "metadata.json"), metadataBytes, 0o600); err != nil {
		return "", fmt.Errorf("write metadata: %w", err)
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return "", fmt.Errorf("commit audio directory: %w", err)
	}
	committed = true
	if err := retainRecentAudioDirectories(root, devAudioRetentionGroups); err != nil {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("dictation: failed to clean old development audio: %s", err))
	}
	return finalPath, nil
}

// writePCM16WAV writes a standard mono 16 kHz RIFF/WAVE file.
func writePCM16WAV(path string, samples []float32) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	dataSize := uint32(len(samples) * 2)
	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], 36+dataSize)
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], 1)
	binary.LittleEndian.PutUint32(header[24:28], audioSampleRate)
	binary.LittleEndian.PutUint32(header[28:32], audioSampleRate*2)
	binary.LittleEndian.PutUint16(header[32:34], 2)
	binary.LittleEndian.PutUint16(header[34:36], 16)
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], dataSize)
	if _, err := file.Write(header); err != nil {
		return err
	}

	const samplesPerWrite = 4096
	buffer := make([]byte, samplesPerWrite*2)
	for start := 0; start < len(samples); start += samplesPerWrite {
		end := min(start+samplesPerWrite, len(samples))
		chunk := samples[start:end]
		for i, sample := range chunk {
			value := math.Max(-1, math.Min(1, float64(sample)))
			pcm := int16(math.Round(value * 32767))
			binary.LittleEndian.PutUint16(buffer[i*2:i*2+2], uint16(pcm))
		}
		written, err := file.Write(buffer[:len(chunk)*2])
		if err != nil {
			return err
		}
		if written != len(chunk)*2 {
			return io.ErrShortWrite
		}
	}
	return nil
}

// cleanupTemporaryAudioDirectories removes incomplete groups left by interrupted writes.
func cleanupTemporaryAudioDirectories(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".tmp-") {
			if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// retainRecentAudioDirectories removes oldest committed groups beyond the retention limit.
func retainRecentAudioDirectories(root string, limit int) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	var directories []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			directories = append(directories, entry.Name())
		}
	}
	sort.Strings(directories)
	for len(directories) > limit {
		if err := os.RemoveAll(filepath.Join(root, directories[0])); err != nil {
			return err
		}
		directories = directories[1:]
	}
	return nil
}

// sanitizeAudioPathPart keeps diagnostic directory names portable and predictable.
func sanitizeAudioPathPart(value string) string {
	var builder strings.Builder
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_' {
			builder.WriteRune(character)
		}
	}
	if builder.Len() == 0 {
		return "session"
	}
	return builder.String()
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
