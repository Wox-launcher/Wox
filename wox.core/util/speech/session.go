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
type Session struct {
	ctx        context.Context
	config     RecognizerConfig
	deviceID   string
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
func NewSession(ctx context.Context, config RecognizerConfig, deviceID string, onPartial func(string), onFinal func(string)) *Session {
	return &Session{
		ctx:       ctx,
		config:    config,
		deviceID:  deviceID,
		onPartial: onPartial,
		onFinal:   onFinal,
	}
}

// Start initializes the recognizer and begins audio capture. It returns an
// error if the model files are missing or the audio device cannot be opened.
func (s *Session) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != SessionStateIdle {
		return fmt.Errorf("session already started or stopped")
	}

	// Create the recognizer first so we fail fast if the model is invalid.
	recognizer, err := newSherpaRecognizer(s.ctx, s.config)
	if err != nil {
		return fmt.Errorf("failed to create recognizer (model=%s, type=%s): %w", s.config.ModelPath, s.config.ModelType, err)
	}
	s.recognizer = recognizer

	// Create the audio capture device.
	capture, err := NewAudioCapture(s.ctx, s.deviceID, func(samples []float32) {
		s.handleAudioSamples(samples)
	})
	if err != nil {
		recognizer.Close()
		s.recognizer = nil
		return fmt.Errorf("failed to create audio capture: %w", err)
	}
	s.capture = capture

	if err := capture.Start(); err != nil {
		capture.Close()
		s.capture = nil
		s.recognizer.Close()
		s.recognizer = nil
		return fmt.Errorf("failed to start audio capture: %w", err)
	}

	s.state = SessionStateRecording
	return nil
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

	// Clean up resources.
	if s.capture != nil {
		s.capture.Close()
		s.capture = nil
	}
	if s.recognizer != nil {
		s.recognizer.Close()
		s.recognizer = nil
	}

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
