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

// Session manages a single dictation recording session: audio capture,
// streaming recognition, and result delivery via callbacks.
//
// When pool is non-nil, the recognizer model is acquired from the pool and
// returned on Stop (model stays in memory for fast reuse). When pool is nil,
// the recognizer is created and destroyed per session (legacy behavior).
type Session struct {
	ctx        context.Context
	config     RecognizerConfig
	deviceID   string
	pool       *RecognizerPool
	audioPool  *AudioCapturePool
	recognizer Recognizer
	capture    *AudioCapture
	state      SessionState
	mu         sync.Mutex

	// onPartial is called whenever new interim text is available.
	// It is called from the audio capture goroutine.
	onPartial func(text string)
	// onFinal is called when an endpoint is detected (sentence boundary)
	// or when the session is stopped. It receives the final text of the
	// current segment.
	onFinal func(text string)

	// lastText tracks the most recent partial text for endpoint detection.
	lastText string
	// accumulatedText holds all recognized text across segments.
	accumulatedText string
}

// NewSession creates a new dictation session. Call Start to begin recording.
// The recognizer is created and destroyed per session (no pooling).
func NewSession(ctx context.Context, config RecognizerConfig, deviceID string, onPartial func(string), onFinal func(string)) *Session {
	return &Session{
		ctx:       ctx,
		config:    config,
		deviceID:  deviceID,
		onPartial: onPartial,
		onFinal:   onFinal,
	}
}

// NewSessionWithPool creates a session that acquires the recognizer from the
// given pool on Start and returns it on Stop. This keeps the model in memory
// across sessions, eliminating the model-loading delay.
func NewSessionWithPool(ctx context.Context, config RecognizerConfig, deviceID string, pool *RecognizerPool, onPartial func(string), onFinal func(string)) *Session {
	return &Session{
		ctx:       ctx,
		config:    config,
		deviceID:  deviceID,
		pool:      pool,
		onPartial: onPartial,
		onFinal:   onFinal,
	}
}

// NewSessionWithPools creates a session that uses both the recognizer pool
// and the audio capture pool, keeping both the model and the audio device
// alive across sessions for minimal startup latency.
func NewSessionWithPools(ctx context.Context, config RecognizerConfig, deviceID string, pool *RecognizerPool, audioPool *AudioCapturePool, onPartial func(string), onFinal func(string)) *Session {
	return &Session{
		ctx:       ctx,
		config:    config,
		deviceID:  deviceID,
		pool:      pool,
		audioPool: audioPool,
		onPartial: onPartial,
		onFinal:   onFinal,
	}
}

// Start initializes the recognizer and begins audio capture. It returns an
// error if the model files are missing or the audio device cannot be opened.
func (s *Session) Start() error {
	t0 := time.Now()
	logger := util.GetLogger()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != SessionStateIdle {
		return fmt.Errorf("session already started or stopped")
	}

	// Acquire recognizer: from pool (fast path, model already in memory) or
	// create from scratch (slow path, first load).
	if s.pool != nil {
		rec, err := s.pool.Acquire(s.ctx, s.config)
		if err != nil {
			return fmt.Errorf("failed to acquire recognizer from pool (model=%s, type=%s): %w", s.config.ModelPath, s.config.ModelType, err)
		}
		s.recognizer = rec
	} else {
		recognizer, err := newSherpaRecognizer(s.ctx, s.config)
		if err != nil {
			return fmt.Errorf("failed to create recognizer (model=%s, type=%s): %w", s.config.ModelPath, s.config.ModelType, err)
		}
		s.recognizer = recognizer
	}
	logger.Debug(s.ctx, fmt.Sprintf("dictation timing: session.newRecognizer cost=%dms", time.Since(t0).Milliseconds()))

	// Create or acquire the audio capture device.
	var capture *AudioCapture
	if s.audioPool != nil {
		c, err := s.audioPool.Acquire(s.ctx, s.deviceID, func(samples []float32) {
			s.handleAudioSamples(samples)
		})
		if err != nil {
			s.releaseRecognizer()
			s.recognizer = nil
			return fmt.Errorf("failed to acquire audio capture from pool: %w", err)
		}
		capture = c
	} else {
		c, err := NewAudioCapture(s.ctx, s.deviceID, func(samples []float32) {
			s.handleAudioSamples(samples)
		})
		if err != nil {
			s.releaseRecognizer()
			s.recognizer = nil
			return fmt.Errorf("failed to create audio capture: %w", err)
		}
		capture = c
	}
	s.capture = capture
	logger.Debug(s.ctx, fmt.Sprintf("dictation timing: session.newAudioCapture cost=%dms", time.Since(t0).Milliseconds()))

	if err := capture.Start(); err != nil {
		if s.audioPool != nil {
			s.audioPool.Release(s.ctx, capture)
		} else {
			capture.Close()
		}
		s.capture = nil
		s.releaseRecognizer()
		s.recognizer = nil
		return fmt.Errorf("failed to start audio capture: %w", err)
	}
	logger.Debug(s.ctx, fmt.Sprintf("dictation timing: session.captureStart cost=%dms", time.Since(t0).Milliseconds()))

	s.state = SessionStateRecording
	logger.Debug(s.ctx, fmt.Sprintf("dictation timing: session.total cost=%dms", time.Since(t0).Milliseconds()))
	return nil
}

// releaseRecognizer returns the recognizer to the pool (if pooled) or closes
// it fully (if not pooled). The caller must hold s.mu.
func (s *Session) releaseRecognizer() {
	if s.recognizer == nil {
		return
	}
	if s.pool != nil {
		// Cast to access CloseStream; the pool path always produces sherpaRecognizer.
		if sr, ok := s.recognizer.(*sherpaRecognizer); ok {
			s.pool.Release(s.ctx, sr)
		} else {
			s.recognizer.Close()
		}
	} else {
		s.recognizer.Close()
	}
}

// handleAudioSamples is called from the malgo capture callback goroutine.
// It feeds audio into the recognizer and runs decode passes in the same
// goroutine to avoid contention with the UI thread.
func (s *Session) handleAudioSamples(samples []float32) {
	s.recognizer.AcceptWaveform(16000, samples)

	// Run decode passes until the recognizer has no more buffered audio.
	for s.recognizer.IsReady() {
		s.recognizer.Decode()
	}

	// Check for partial results.
	result := s.recognizer.GetResult()
	if result.Text != s.lastText {
		s.lastText = result.Text
		if s.onPartial != nil {
			// Show the full accumulated text plus the current partial so the
			// overlay reflects everything recognized so far, not just the
			// latest segment.
			s.onPartial(s.accumulatedText + s.lastText)
		}
	}

	// Check for endpoint (sentence boundary).
	if s.recognizer.IsEndpoint() {
		finalText := s.recognizer.GetResult().Text
		s.recognizer.Reset()
		s.lastText = ""
		if finalText != "" {
			s.accumulatedText += finalText
			if s.onFinal != nil {
				// Report the full accumulated text so consumers can display
				// the complete transcription across all segments.
				s.onFinal(s.accumulatedText)
			}
		}
	}
}

// Stop stops the recording session and returns all accumulated text.
// The caller should use this text as the final output. If the recognizer
// has pending partial text that hasn't triggered an endpoint, it is
// appended to the result.
func (s *Session) Stop() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != SessionStateRecording {
		return "", fmt.Errorf("session is not recording")
	}

	// Stop the audio capture first so no more samples arrive.
	if s.capture != nil {
		_ = s.capture.Stop()
	}

	// Flush any remaining partial text.
	partial := s.recognizer.GetResult().Text
	if partial != "" {
		s.accumulatedText += partial
	}

	totalText := s.accumulatedText

	// Return audio capture to pool (keeps device alive) or close it fully.
	if s.capture != nil {
		if s.audioPool != nil {
			s.audioPool.Release(s.ctx, s.capture)
		} else {
			s.capture.Close()
		}
		s.capture = nil
	}

	// Return recognizer to pool (keeps model in memory) or close it fully.
	s.releaseRecognizer()
	s.recognizer = nil

	s.state = SessionStateStopped
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
	partial := s.lastText
	return text + partial
}

// sessionTimeout is the maximum duration a session can run before
// auto-stopping. This prevents runaway sessions if the user forgets to stop.
const sessionTimeout = 5 * time.Minute

// StartWithTimeout starts the session and schedules an auto-stop after
// the timeout duration. The onTimeout callback is called if the session
// is auto-stopped.
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
