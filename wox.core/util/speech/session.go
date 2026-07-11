package speech

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"

	"wox/util"
	"wox/util/permission"
)

const maxSessionAudioSamples = audioSampleRate * 60 * 5

// SessionState tracks the lifecycle of a dictation recording session.
type SessionState int

const (
	SessionStateIdle SessionState = iota
	SessionStateRecording
	SessionStateStopped
)

// SessionStopReason records why a recording session ended.
type SessionStopReason string

const (
	SessionStopReasonCompleted        SessionStopReason = "completed"
	SessionStopReasonCancelled        SessionStopReason = "cancelled"
	SessionStopReasonStartupCancelled SessionStopReason = "startup_cancelled"
	SessionStopReasonTimeout          SessionStopReason = "timeout"
	SessionStopReasonPluginUnload     SessionStopReason = "plugin_unload"
)

// SpeechActivity reports whether the current audio input is likely speech.
type SpeechActivity struct {
	Speaking bool
}

// Session manages a dictation recording session. It supports two modes:
//
//   - Streaming (zipformer2/paraformer): audio is fed continuously into the
//     online recognizer, partial results are delivered via onPartial.
//
//   - Offline (qwen3_asr): audio is fed into a VAD that splits it into speech
//     segments, each segment is decoded by the offline recognizer. Results
//     are delivered when each segment completes.
//
// Pools are used to keep the recognizer model, VAD, and audio device alive
// across sessions for fast startup.
type Session struct {
	ctx        context.Context
	config     RecognizerConfig
	vadConfig  VadConfig
	deviceID   string
	pool       *RecognizerPool
	audioPool  *AudioCapturePool
	vadPool    *VadPool
	recognizer Recognizer
	vad        *VoiceActivityDetector
	capture    *AudioCapture
	state      SessionState
	streaming  bool
	mu         sync.Mutex

	onPartial func(text string)
	onFinal   func(text string)
	onSpeech  func(activity SpeechActivity)

	// Streaming mode state
	lastText        string
	accumulatedText string
	lastSpeech      bool
	hasSpeechState  bool

	// Offline mode state
	decodeWG          sync.WaitGroup
	stopped           chan struct{}
	audioProcessor    *AdaptiveAudioProcessor
	offlineAudio      []float32
	devRawAudio       []float32
	devProcessedAudio []float32
	diagnosticID      string
	startedAt         time.Time
	primarySegments   int
	fallbackType      string
}

// NewSessionWithPools creates a session that uses recognizer, VAD, and audio
// capture pools. The VAD is only used for offline (non-streaming) models;
// streaming models ignore the VAD and feed audio directly to the recognizer.
func NewSessionWithPools(ctx context.Context, config RecognizerConfig, vadConfig VadConfig, deviceID string, pool *RecognizerPool, audioPool *AudioCapturePool, vadPool *VadPool, onPartial func(string), onFinal func(string)) *Session {
	return &Session{
		ctx:            ctx,
		config:         config,
		vadConfig:      vadConfig,
		deviceID:       deviceID,
		pool:           pool,
		audioPool:      audioPool,
		vadPool:        vadPool,
		onPartial:      onPartial,
		onFinal:        onFinal,
		stopped:        make(chan struct{}),
		audioProcessor: NewAdaptiveAudioProcessor(),
		diagnosticID:   uuid.NewString(),
		fallbackType:   "none",
	}
}

// SetSpeechActivityCallback registers a callback for speech/silence state changes.
func (s *Session) SetSpeechActivityCallback(onSpeech func(SpeechActivity)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSpeech = onSpeech
}

// Start initializes the recognizer (and VAD for offline mode), audio capture,
// and begins recording.
func (s *Session) Start() error {
	t0 := time.Now()
	logger := util.GetLogger()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != SessionStateIdle {
		return fmt.Errorf("session already started or stopped")
	}
	if !permission.RequestMicrophonePermission(s.ctx) {
		return fmt.Errorf("microphone permission was denied")
	}
	s.startedAt = time.Now()

	// Acquire recognizer from pool.
	rec, err := s.pool.Acquire(s.ctx, s.config)
	if err != nil {
		return fmt.Errorf("failed to acquire recognizer: %w", err)
	}
	s.recognizer = rec
	s.streaming = rec.IsStreaming()
	logger.Info(s.ctx, fmt.Sprintf("dictation timing: session.newRecognizer cost=%dms (streaming=%t)", time.Since(t0).Milliseconds(), s.streaming))

	// Offline recognition requires VAD for segmenting. Streaming recognition
	// can still use VAD for the overlay activity indicator, but it must not
	// block startup if the optional detector is unavailable.
	if s.vadPool != nil && s.vadConfig.ModelPath != "" {
		vad, err := s.vadPool.Acquire(s.ctx, s.vadConfig)
		if err != nil {
			if !s.streaming {
				s.pool.Release(s.ctx, rec)
				s.recognizer = nil
				return fmt.Errorf("failed to acquire VAD: %w", err)
			}
			logger.Warn(s.ctx, fmt.Sprintf("dictation: optional VAD unavailable for speech activity overlay: %s", err.Error()))
		} else {
			s.vad = vad
			logger.Info(s.ctx, fmt.Sprintf("dictation timing: session.newVad cost=%dms", time.Since(t0).Milliseconds()))
		}
	} else if !s.streaming {
		s.pool.Release(s.ctx, rec)
		s.recognizer = nil
		return fmt.Errorf("failed to acquire VAD: VAD pool or model path is unavailable")
	}

	// Acquire audio capture from pool.
	capture, err := s.audioPool.Acquire(s.ctx, s.deviceID, func(samples []float32) {
		s.handleAudioSamples(samples)
	})
	if err != nil {
		if s.vad != nil {
			s.vadPool.Release(s.ctx, s.vad)
			s.vad = nil
		}
		s.pool.Release(s.ctx, rec)
		s.recognizer = nil
		return fmt.Errorf("failed to create audio capture: %w", err)
	}
	s.capture = capture
	logger.Info(s.ctx, fmt.Sprintf("dictation timing: session.newAudioCapture cost=%dms", time.Since(t0).Milliseconds()))

	if err := capture.Start(); err != nil {
		s.audioPool.Discard(s.ctx, capture)
		if s.vad != nil {
			s.vadPool.Release(s.ctx, s.vad)
			s.vad = nil
		}
		s.pool.Release(s.ctx, rec)
		s.capture = nil
		s.recognizer = nil
		s.recognizer = nil
		return fmt.Errorf("failed to start audio capture: %w", err)
	}
	logger.Info(s.ctx, fmt.Sprintf("dictation timing: session.captureStart cost=%dms", time.Since(t0).Milliseconds()))

	s.state = SessionStateRecording
	logger.Info(s.ctx, fmt.Sprintf("dictation timing: session.total cost=%dms", time.Since(t0).Milliseconds()))
	return nil
}

// handleAudioSamples is called from the malgo capture callback goroutine.
// In streaming mode it feeds audio directly to the recognizer and delivers
// partial results. In offline mode it feeds audio to the VAD and dispatches
// completed speech segments for async decoding.
func (s *Session) handleAudioSamples(samples []float32) {
	select {
	case <-s.stopped:
		return
	default:
	}

	s.mu.Lock()
	rec := s.recognizer
	vad := s.vad
	streaming := s.streaming
	s.mu.Unlock()

	if rec == nil {
		return
	}

	if util.IsDev() {
		s.devRawAudio = appendCappedAudio(s.devRawAudio, samples, maxSessionAudioSamples)
	}
	s.audioProcessor.Process(samples)
	if util.IsDev() && streaming {
		s.devProcessedAudio = appendCappedAudio(s.devProcessedAudio, samples, maxSessionAudioSamples)
	}
	if !streaming {
		s.offlineAudio = appendCappedAudio(s.offlineAudio, samples, maxSessionAudioSamples)
	}

	if streaming {
		if vad != nil {
			s.reportStreamingVadActivity(vad, samples)
		} else {
			s.reportSpeechActivity(isLikelySpeechLevel(samples))
		}
		s.handleStreamingSamples(rec, samples)
	} else {
		s.handleOfflineSamples(rec, vad, samples)
	}
}

// reportStreamingVadActivity feeds the optional VAD used only for UI speech
// activity while draining completed segments that streaming recognition ignores.
func (s *Session) reportStreamingVadActivity(vad *VoiceActivityDetector, samples []float32) {
	vad.AcceptWaveform(samples)
	s.reportSpeechActivity(vad.IsSpeech())
	for !vad.IsEmpty() {
		vad.Pop()
	}
}

// reportSpeechActivity emits activity changes only, keeping UI updates out of
// the steady-state audio callback path.
func (s *Session) reportSpeechActivity(speaking bool) {
	s.mu.Lock()
	if s.hasSpeechState && s.lastSpeech == speaking {
		s.mu.Unlock()
		return
	}
	s.hasSpeechState = true
	s.lastSpeech = speaking
	cb := s.onSpeech
	s.mu.Unlock()

	if cb != nil {
		cb(SpeechActivity{Speaking: speaking})
	}
}

// isLikelySpeechLevel is a fallback for streaming models when optional VAD is
// unavailable. It intentionally uses a conservative mean-square threshold.
func isLikelySpeechLevel(samples []float32) bool {
	if len(samples) == 0 {
		return false
	}

	var sumSquares float64
	for _, sample := range samples {
		sumSquares += float64(sample * sample)
	}
	meanSquare := sumSquares / float64(len(samples))
	return meanSquare > 0.0001
}

// handleStreamingSamples feeds audio to the online recognizer and delivers
// partial results. Runs in the audio callback goroutine.
func (s *Session) handleStreamingSamples(rec Recognizer, samples []float32) {
	rec.AcceptWaveform(16000, samples)

	for rec.IsReady() {
		rec.Decode()
	}

	result := rec.GetResult()
	if result.Text != s.lastText {
		s.lastText = result.Text
		if s.onPartial != nil {
			s.onPartial(s.accumulatedText + s.lastText)
		}
	}

	if rec.IsEndpoint() {
		finalText := rec.GetResult().Text
		rec.Reset()
		s.lastText = ""
		if finalText != "" {
			s.mu.Lock()
			s.accumulatedText += finalText
			full := s.accumulatedText
			s.mu.Unlock()
			if s.onFinal != nil {
				s.onFinal(full)
			}
		}
	}
}

// handleOfflineSamples feeds audio to the VAD and dispatches completed speech
// segments for async offline decoding.
func (s *Session) handleOfflineSamples(rec Recognizer, vad *VoiceActivityDetector, samples []float32) {
	if vad == nil {
		return
	}

	vad.AcceptWaveform(samples)
	s.reportSpeechActivity(vad.IsSpeech())

	// Log VAD state periodically for debugging.
	if vad.IsSpeech() {
		util.GetLogger().Debug(s.ctx, fmt.Sprintf("dictation: VAD speech detected, segments available=%v", !vad.IsEmpty()))
	}

	for !vad.IsEmpty() {
		seg := vad.Front()
		vad.Pop()
		if seg == nil || len(seg.Samples) == 0 {
			continue
		}

		util.GetLogger().Info(s.ctx, fmt.Sprintf("dictation: VAD segment ready, samples=%d", len(seg.Samples)))

		s.queueOfflineSegment(rec, seg.Samples, true, "segment")
	}
}

// queueOfflineSegment copies a VAD segment and decodes it outside the capture callback.
func (s *Session) queueOfflineSegment(rec Recognizer, samples []float32, notify bool, source string) {
	samplesCopy := append([]float32(nil), samples...)
	s.mu.Lock()
	s.primarySegments++
	s.mu.Unlock()

	s.decodeWG.Add(1)
	util.Go(s.ctx, "dictation decode "+source, func() {
		defer s.decodeWG.Done()
		text := rec.DecodeSamples(samplesCopy)
		util.GetLogger().Info(s.ctx, fmt.Sprintf("dictation: decode %s result, textLen=%d text=%q", source, len(text), text))
		if text == "" {
			return
		}

		s.mu.Lock()
		s.accumulatedText += text
		full := s.accumulatedText
		s.mu.Unlock()
		if notify && s.onPartial != nil {
			s.onPartial(text)
		}
		if notify && s.onFinal != nil {
			s.onFinal(full)
		}
	})
}

// Stop stops the recording session as a normal completed session.
func (s *Session) Stop() (string, error) {
	return s.StopWithReason(SessionStopReasonCompleted)
}

// StopWithReason stops the recording session and records its lifecycle outcome.
func (s *Session) StopWithReason(reason SessionStopReason) (string, error) {
	logger := util.GetLogger()
	logger.Info(s.ctx, fmt.Sprintf("dictation: session.Stop enter, reason=%s", reason))

	s.mu.Lock()
	if s.state != SessionStateRecording {
		s.mu.Unlock()
		logger.Info(s.ctx, "dictation: session.Stop not recording, returning")
		return "", fmt.Errorf("session is not recording")
	}
	s.state = SessionStateStopped
	s.mu.Unlock()

	// Signal the audio callback to stop processing.
	close(s.stopped)
	logger.Info(s.ctx, "dictation: session.Stop signaled stopped")

	// Stop audio capture first so no more samples arrive.
	if s.capture != nil {
		logger.Info(s.ctx, "dictation: session.Stop stopping capture")
		_ = s.capture.Stop()
		logger.Info(s.ctx, "dictation: session.Stop capture stopped")
	}

	// Offline mode: flush VAD and queue remaining segments for decoding.
	if !s.streaming && s.vad != nil {
		logger.Info(s.ctx, "dictation: session.Stop flushing VAD")
		s.vad.Flush()
		for !s.vad.IsEmpty() {
			seg := s.vad.Front()
			s.vad.Pop()
			if seg == nil || len(seg.Samples) == 0 {
				continue
			}
			s.queueOfflineSegment(s.recognizer, seg.Samples, false, "flush segment")
		}
		logger.Info(s.ctx, "dictation: session.Stop VAD flush done")
	}

	// Streaming mode: flush remaining partial text.
	if s.streaming && s.recognizer != nil {
		partial := s.recognizer.GetResult().Text
		if partial != "" {
			s.mu.Lock()
			s.accumulatedText += partial
			s.mu.Unlock()
		}
		// The online stream result is "since last Reset"; reset at the session
		// boundary so a pooled recognizer cannot carry this transcript into the
		// next dictation.
		s.recognizer.Reset()
		s.lastText = ""
	}

	// Wait for all in-flight decode goroutines (offline mode). This must be
	// outside s.mu because decode goroutines acquire s.mu to update
	// accumulatedText.
	logger.Info(s.ctx, "dictation: session.Stop waiting for decode goroutines")
	s.decodeWG.Wait()
	logger.Info(s.ctx, "dictation: session.Stop decode goroutines done")

	s.mu.Lock()
	totalText := s.accumulatedText
	s.mu.Unlock()
	if !s.streaming && totalText == "" && s.audioProcessor.Stats().CandidateDuration >= 250*time.Millisecond {
		fallbackText, fallbackType := s.decodeOfflineFallback()
		if fallbackText != "" {
			s.mu.Lock()
			s.accumulatedText += fallbackText
			totalText = s.accumulatedText
			s.mu.Unlock()
		}
		s.fallbackType = fallbackType
	}

	stats := s.audioProcessor.Stats()
	logger.Info(s.ctx, fmt.Sprintf(
		"dictation: audio summary inputRms=%.1fdBFS inputPeak=%.1fdBFS outputRms=%.1fdBFS outputPeak=%.1fdBFS noiseFloor=%.1fdBFS averageGain=%.1fdB maxGain=%.1fdB candidateMs=%d vadSegments=%d fallback=%s",
		stats.InputRMSDBFS, stats.InputPeakDBFS, stats.OutputRMSDBFS, stats.OutputPeakDBFS, stats.NoiseFloorDBFS, stats.AverageGainDB, stats.MaximumGainDB, stats.CandidateDuration.Milliseconds(), s.primarySegments, s.fallbackType,
	))
	if util.IsDev() {
		diagnosticProcessedAudio := s.devProcessedAudio
		if !s.streaming {
			// The offline fallback buffer is already the exact processed stream,
			// so diagnostics share it instead of retaining a third five-minute copy.
			diagnosticProcessedAudio = s.offlineAudio
		}
		dump := sessionAudioDump{
			SessionID:       s.diagnosticID,
			StartedAt:       s.startedAt,
			EndedAt:         time.Now(),
			StopReason:      reason,
			DeviceID:        s.deviceID,
			ModelType:       s.config.ModelType,
			Raw:             s.devRawAudio,
			Processed:       diagnosticProcessedAudio,
			Stats:           stats,
			VadConfig:       s.vadConfig,
			PrimarySegments: s.primarySegments,
			FallbackType:    s.fallbackType,
			ResultEmpty:     totalText == "",
		}
		s.devRawAudio = nil
		s.devProcessedAudio = nil
		scheduleSessionAudioDump(s.ctx, dump)
	}
	s.offlineAudio = nil

	// Return resources to pools.
	if s.capture != nil {
		s.audioPool.Release(s.ctx, s.capture)
		s.capture = nil
	}
	if s.vad != nil {
		s.vadPool.Release(s.ctx, s.vad)
		s.vad = nil
	}
	if s.recognizer != nil {
		s.pool.Release(s.ctx, s.recognizer)
		s.recognizer = nil
	}

	logger.Info(s.ctx, fmt.Sprintf("dictation: session.Stop done, textLen=%d text=%q", len(totalText), totalText))
	return totalText, nil
}

// DevelopmentAudioSessionID returns the local diagnostic link for development history previews.
func (s *Session) DevelopmentAudioSessionID() string {
	if !util.IsDev() {
		return ""
	}
	return s.diagnosticID
}

// decodeOfflineFallback retries quiet candidate audio sequentially after the primary path returns no text.
func (s *Session) decodeOfflineFallback() (string, string) {
	if s.recognizer == nil || len(s.offlineAudio) == 0 {
		return "", "none"
	}

	fallbackConfig := s.vadConfig
	fallbackConfig.Threshold = 0.20
	fallbackConfig.MinSpeechDuration = 0.10
	fallbackConfig.MinSilenceDuration = 0.70
	fallbackConfig.MaxSpeechDuration = 15
	fallbackVad, err := NewVoiceActivityDetector(s.ctx, fallbackConfig)
	if err != nil {
		util.GetLogger().Warn(s.ctx, fmt.Sprintf("dictation: low-threshold fallback VAD unavailable: %s", err))
	} else {
		var text string
		segmentCount := 0
		drainFallbackSegments := func() {
			for !fallbackVad.IsEmpty() {
				segment := fallbackVad.Front()
				fallbackVad.Pop()
				if segment != nil && len(segment.Samples) > 0 {
					segmentCount++
					text += s.recognizer.DecodeSamples(segment.Samples)
				}
			}
		}
		for start := 0; start < len(s.offlineAudio); start += audioSampleRate / 10 {
			end := min(start+audioSampleRate/10, len(s.offlineAudio))
			fallbackVad.AcceptWaveform(s.offlineAudio[start:end])
			drainFallbackSegments()
		}
		fallbackVad.Flush()
		drainFallbackSegments()
		fallbackVad.Close()
		if segmentCount > 0 {
			util.GetLogger().Info(s.ctx, fmt.Sprintf("dictation: low-threshold VAD fallback finished, segments=%d textLen=%d", segmentCount, len(text)))
			return text, "low_threshold_vad"
		}
	}

	ranges := prepareCandidateRanges(s.audioProcessor.CandidateRanges(), s.offlineAudio)
	var text string
	for _, candidateRange := range ranges {
		text += s.recognizer.DecodeSamples(s.offlineAudio[candidateRange.StartSample:candidateRange.EndSample])
	}
	if len(ranges) > 0 {
		util.GetLogger().Info(s.ctx, fmt.Sprintf("dictation: candidate-region fallback finished, regions=%d textLen=%d", len(ranges), len(text)))
		return text, "candidate_regions"
	}
	return "", "none"
}

// prepareCandidateRanges pads, merges, and splits speech-like regions for sequential decoding.
func prepareCandidateRanges(input []audioCandidateRange, audio []float32) []audioCandidateRange {
	const (
		paddingSamples  = audioSampleRate * 300 / 1000
		mergeGapSamples = audioSampleRate * 500 / 1000
		maximumSamples  = audioSampleRate * 15
	)
	var merged []audioCandidateRange
	for _, candidateRange := range input {
		candidateRange.StartSample = max(0, candidateRange.StartSample-paddingSamples)
		candidateRange.EndSample = min(len(audio), candidateRange.EndSample+paddingSamples)
		if candidateRange.StartSample >= candidateRange.EndSample {
			continue
		}
		if len(merged) > 0 && candidateRange.StartSample-merged[len(merged)-1].EndSample < mergeGapSamples {
			merged[len(merged)-1].EndSample = max(merged[len(merged)-1].EndSample, candidateRange.EndSample)
			continue
		}
		merged = append(merged, candidateRange)
	}

	var result []audioCandidateRange
	for _, candidateRange := range merged {
		for candidateRange.EndSample-candidateRange.StartSample > maximumSamples {
			preferredEnd := candidateRange.StartSample + maximumSamples
			end := quietestFrameBoundary(audio, preferredEnd-audioSampleRate, preferredEnd+audioSampleRate)
			if end <= candidateRange.StartSample {
				end = preferredEnd
			}
			result = append(result, audioCandidateRange{StartSample: candidateRange.StartSample, EndSample: end})
			candidateRange.StartSample = end
		}
		if candidateRange.StartSample < candidateRange.EndSample {
			result = append(result, candidateRange)
		}
	}
	return result
}

// quietestFrameBoundary finds the lowest-RMS 100 ms boundary near a forced split.
func quietestFrameBoundary(audio []float32, start int, end int) int {
	const frameSamples = audioSampleRate / 10
	start = max(frameSamples, start/frameSamples*frameSamples)
	end = min(len(audio)-frameSamples, end/frameSamples*frameSamples)
	bestBoundary := start
	bestRMS := math.MaxFloat64
	for boundary := start; boundary <= end; boundary += frameSamples {
		var sumSquares float64
		for _, sample := range audio[boundary-frameSamples : boundary] {
			sumSquares += float64(sample * sample)
		}
		rms := math.Sqrt(sumSquares / frameSamples)
		if rms < bestRMS {
			bestRMS = rms
			bestBoundary = boundary
		}
	}
	return bestBoundary
}

// appendCappedAudio appends only the portion that fits the per-session memory limit.
func appendCappedAudio(destination []float32, samples []float32, limit int) []float32 {
	remaining := limit - len(destination)
	if remaining <= 0 {
		return destination
	}
	if len(samples) > remaining {
		samples = samples[:remaining]
	}
	return append(destination, samples...)
}

// IsRecording reports whether the session is currently capturing audio.
func (s *Session) IsRecording() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == SessionStateRecording
}

// GetAccumulatedText returns all text recognized so far across segments.
func (s *Session) GetAccumulatedText() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	text := s.accumulatedText
	if s.streaming {
		text += s.lastText
	}
	return text
}

// sessionTimeout is the maximum duration a session can run before auto-stopping.
const sessionTimeout = 5 * time.Minute

// StartWithTimeout starts the session and schedules an auto-stop after
// the timeout duration.
func (s *Session) StartWithTimeout(onTimeout func()) error {
	if err := s.Start(); err != nil {
		return err
	}
	util.Go(s.ctx, "dictation session timeout", func() {
		timer := time.NewTimer(sessionTimeout)
		defer timer.Stop()
		select {
		case <-timer.C:
			if s.IsRecording() {
				_, _ = s.StopWithReason(SessionStopReasonTimeout)
				if onTimeout != nil {
					onTimeout()
				}
			}
		case <-s.ctx.Done():
		}
	})
	return nil
}
