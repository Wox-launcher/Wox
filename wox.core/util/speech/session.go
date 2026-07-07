package speech

import (
	"context"
	"fmt"
	"sync"
	"time"

	"wox/util"
)

// SessionState tracks the lifecycle of a dictation recording session.
type SessionState int

const (
	SessionStateIdle SessionState = iota
	SessionStateRecording
	SessionStateStopped
)

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

	// Streaming mode state
	lastText        string
	accumulatedText string

	// Offline mode state
	decodeWG sync.WaitGroup
	stopped  chan struct{}
}

// NewSessionWithPools creates a session that uses recognizer, VAD, and audio
// capture pools. The VAD is only used for offline (non-streaming) models;
// streaming models ignore the VAD and feed audio directly to the recognizer.
func NewSessionWithPools(ctx context.Context, config RecognizerConfig, vadConfig VadConfig, deviceID string, pool *RecognizerPool, audioPool *AudioCapturePool, vadPool *VadPool, onPartial func(string), onFinal func(string)) *Session {
	return &Session{
		ctx:       ctx,
		config:    config,
		vadConfig: vadConfig,
		deviceID:  deviceID,
		pool:      pool,
		audioPool: audioPool,
		vadPool:   vadPool,
		onPartial: onPartial,
		onFinal:   onFinal,
		stopped:   make(chan struct{}),
	}
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

	// Acquire recognizer from pool.
	rec, err := s.pool.Acquire(s.ctx, s.config)
	if err != nil {
		return fmt.Errorf("failed to acquire recognizer: %w", err)
	}
	s.recognizer = rec
	s.streaming = rec.IsStreaming()
	logger.Info(s.ctx, fmt.Sprintf("dictation timing: session.newRecognizer cost=%dms (streaming=%t)", time.Since(t0).Milliseconds(), s.streaming))

	// Acquire VAD only for offline mode.
	if !s.streaming {
		vad, err := s.vadPool.Acquire(s.ctx, s.vadConfig)
		if err != nil {
			s.pool.Release(s.ctx, rec)
			s.recognizer = nil
			return fmt.Errorf("failed to acquire VAD: %w", err)
		}
		s.vad = vad
		logger.Info(s.ctx, fmt.Sprintf("dictation timing: session.newVad cost=%dms", time.Since(t0).Milliseconds()))
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
		s.audioPool.Release(s.ctx, capture)
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
	s.mu.Unlock()

	if rec == nil {
		return
	}

	if s.streaming {
		s.handleStreamingSamples(rec, samples)
	} else {
		s.handleOfflineSamples(rec, vad, samples)
	}
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

		samplesCopy := make([]float32, len(seg.Samples))
		copy(samplesCopy, seg.Samples)

		s.decodeWG.Add(1)
		util.Go(s.ctx, "dictation decode segment", func() {
			defer s.decodeWG.Done()

			text := rec.DecodeSamples(samplesCopy)
			if text == "" {
				return
			}

			s.mu.Lock()
			s.accumulatedText += text
			full := s.accumulatedText
			s.mu.Unlock()

			if s.onPartial != nil {
				s.onPartial(text)
			}
			if s.onFinal != nil {
				s.onFinal(full)
			}
		})
	}
}

// Stop stops the recording session and returns all accumulated text.
func (s *Session) Stop() (string, error) {
	logger := util.GetLogger()
	logger.Info(s.ctx, "dictation: session.Stop enter")

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
			samplesCopy := make([]float32, len(seg.Samples))
			copy(samplesCopy, seg.Samples)

			s.decodeWG.Add(1)
			util.Go(s.ctx, "dictation decode flush segment", func() {
				defer s.decodeWG.Done()
				text := s.recognizer.DecodeSamples(samplesCopy)
				if text == "" {
					return
				}
				s.mu.Lock()
				s.accumulatedText += text
				s.mu.Unlock()
			})
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

	logger.Info(s.ctx, fmt.Sprintf("dictation: session.Stop done, textLen=%d", len(totalText)))
	return totalText, nil
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
				if onTimeout != nil {
					onTimeout()
				}
				_, _ = s.Stop()
			}
		case <-s.ctx.Done():
		}
	})
	return nil
}
