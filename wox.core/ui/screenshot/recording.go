package screenshot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"wox/util"
)

const (
	recordingCountdownDuration = 3 * time.Second
	recordingFrameQueueSize    = 8
	recordingOrphanMaxAge      = 24 * time.Hour
)

type recordingState uint8

const (
	recordingStateReady recordingState = iota
	recordingStatePreparing
	recordingStateCountdown
	recordingStateRecording
	recordingStatePaused
	recordingStateFinalizing
	recordingStateSave
	recordingStateError
	recordingStateClosed
)

func (state recordingState) String() string {
	switch state {
	case recordingStateReady:
		return "ready"
	case recordingStatePreparing:
		return "preparing"
	case recordingStateCountdown:
		return "countdown"
	case recordingStateRecording:
		return "recording"
	case recordingStatePaused:
		return "paused"
	case recordingStateFinalizing:
		return "finalizing"
	case recordingStateSave:
		return "save"
	case recordingStateError:
		return "error"
	default:
		return "closed"
	}
}

type recordingFrame struct {
	image *image.RGBA
	index int64
}

type recordingEncoder interface {
	Start(path string, width, height, fps int) error
	WriteFrame(frame recordingFrame) error
	Finalize() error
	Abort() error
}

type recordingSessionConfig struct {
	FPS          int
	ShowPointer  bool
	ShowKeypress bool
	PixelBounds  image.Rectangle
	Capture      func() (image.Image, error)
	Compose      func(image.Image) (*image.RGBA, error)
	Encoder      recordingEncoder
	Release      func()
	TempRoot     string
	Countdown    time.Duration
	Now          func() time.Time
	Sleep        func(context.Context, time.Duration) error
	OnChanged    func()
	Diagnostics  bool
}

// recordingSession owns one take from countdown through finalization and cleanup.
type recordingSession struct {
	mu              sync.Mutex
	operationMu     sync.Mutex
	config          recordingSessionConfig
	state           recordingState
	tempPath        string
	startedAt       time.Time
	pausedAt        time.Time
	pausedDuration  time.Duration
	countdownEndsAt time.Time
	error           error
	framesWritten   int64
	framesDropped   int64
	frames          chan recordingFrame
	stopCapture     context.CancelFunc
	workerDone      chan struct{}
	captureDone     chan struct{}
	workerErr       error
	stopOnce        sync.Once
	captureEpoch    uint64
	captureNanos    int64
	captureSamples  int64
}

// newRecordingSession validates cadence, even dimensions, and injectable lifecycle dependencies.
func newRecordingSession(config recordingSessionConfig) (*recordingSession, error) {
	if config.FPS != 30 && config.FPS != 60 {
		return nil, fmt.Errorf("unsupported recording FPS: %d", config.FPS)
	}
	if config.PixelBounds.Empty() || config.PixelBounds.Dx()%2 != 0 || config.PixelBounds.Dy()%2 != 0 {
		return nil, errors.New("recording pixel bounds must have positive even dimensions")
	}
	if config.Capture == nil || config.Encoder == nil {
		return nil, errors.New("recording capture and encoder are required")
	}
	if config.Compose == nil {
		config.Compose = normalizeRecordingFrame
	}
	if config.Countdown <= 0 {
		config.Countdown = recordingCountdownDuration
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.Sleep == nil {
		config.Sleep = sleepRecordingContext
	}
	if config.TempRoot == "" {
		config.TempRoot = filepath.Join(os.TempDir(), "wox-recordings")
	}
	return &recordingSession{config: config, state: recordingStateReady}, nil
}

func sleepRecordingContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// normalizeRecordingPixelBounds trims only the right and bottom edges for H.264 4:2:0 output.
func normalizeRecordingPixelBounds(bounds image.Rectangle) image.Rectangle {
	if bounds.Dx()%2 != 0 {
		bounds.Max.X--
	}
	if bounds.Dy()%2 != 0 {
		bounds.Max.Y--
	}
	return bounds
}

func normalizeRecordingFrame(source image.Image) (*image.RGBA, error) {
	if source == nil || source.Bounds().Empty() {
		return nil, errors.New("captured recording frame is empty")
	}
	bounds := source.Bounds()
	output := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(output, output.Bounds(), source, bounds.Min, draw.Src)
	return output, nil
}

// cropRecordingFrame copies only the encoded pixel rectangle, or reuses an already-cropped RGBA buffer.
func cropRecordingFrame(source image.Image, pixelBounds image.Rectangle) (*image.RGBA, error) {
	if source == nil || pixelBounds.Empty() {
		return nil, errors.New("captured recording frame is empty")
	}
	width, height := pixelBounds.Dx(), pixelBounds.Dy()
	if rgba, ok := source.(*image.RGBA); ok && source.Bounds().Dx() == width && source.Bounds().Dy() == height {
		if source.Bounds().Min == (image.Point{}) && rgba.Stride == width*4 {
			return rgba, nil
		}
	}
	output := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(output, output.Bounds(), source, pixelBounds.Min, draw.Src)
	return output, nil
}

// applyRecordingOverlays draws live chrome in crop-local coordinates so compose never copies the full desktop.
func applyRecordingOverlays(frame *image.RGBA, pixelBounds image.Rectangle, pointer *Point, keycaps []recordingKeycap, showPointer, showKeypress bool, now time.Time, scale float32) error {
	if frame == nil {
		return errors.New("captured recording frame is empty")
	}
	scale = max(float32(1), scale)
	crop := Rect{Width: float32(pixelBounds.Dx()), Height: float32(pixelBounds.Dy())}
	cropSize := Size{Width: crop.Width, Height: crop.Height}
	if showKeypress {
		if err := renderRecordingKeycaps(frame, crop, cropSize, keycaps, now, scale); err != nil {
			return err
		}
	}
	if showPointer && pointer != nil {
		cursorPixel := Point{X: pointer.X - float32(pixelBounds.Min.X), Y: pointer.Y - float32(pixelBounds.Min.Y)}
		return overlayRecordingCursor(frame, cursorPixel, scale)
	}
	return nil
}

// swapRecordingFrameRedBlue converts a packed RGBA capture into the BGR0 layout
// the H.264 encoder and recording overlays already assume.
func swapRecordingFrameRedBlue(frame *image.RGBA) {
	if frame == nil {
		return
	}
	pix := frame.Pix
	stride := frame.Stride
	width, height := frame.Rect.Dx(), frame.Rect.Dy()
	for row := 0; row < height; row++ {
		rowOffset := row * stride
		for col := 0; col < width; col++ {
			index := rowOffset + col*4
			pix[index], pix[index+2] = pix[index+2], pix[index]
		}
	}
}

// overlayRecordingCursor draws a DPI-scaled pointer onto a BGR0 capture buffer.
func overlayRecordingCursor(target *image.RGBA, cursorPixel Point, scale float32) error {
	width := max(1, int(math.Round(float64(screenshotEditorCursorWidth*scale))))
	height := max(1, int(math.Round(float64(screenshotEditorCursorHeight*scale))))
	raster, err := screenshotEditorCursorRaster(width, height)
	if err != nil {
		return err
	}
	hotspotX := screenshotEditorCursorHotspotX * scale
	hotspotY := screenshotEditorCursorHotspotY * scale
	left := int(math.Round(float64(cursorPixel.X - hotspotX)))
	top := int(math.Round(float64(cursorPixel.Y - hotspotY)))
	destination := image.Rect(left, top, left+width, top+height).Intersect(target.Bounds())
	if destination.Empty() {
		return nil
	}
	srcBounds := raster.Bounds()
	for y := destination.Min.Y; y < destination.Max.Y; y++ {
		srcY := y - top + srcBounds.Min.Y
		for x := destination.Min.X; x < destination.Max.X; x++ {
			srcX := x - left + srcBounds.Min.X
			src := raster.RGBAAt(srcX, srcY)
			if src.A == 0 {
				continue
			}
			dstIndex := target.PixOffset(x, y)
			inv := uint32(255 - src.A)
			srcA := uint32(src.A)
			target.Pix[dstIndex] = byte((uint32(src.B)*srcA + uint32(target.Pix[dstIndex])*inv) / 255)
			target.Pix[dstIndex+1] = byte((uint32(src.G)*srcA + uint32(target.Pix[dstIndex+1])*inv) / 255)
			target.Pix[dstIndex+2] = byte((uint32(src.R)*srcA + uint32(target.Pix[dstIndex+2])*inv) / 255)
			target.Pix[dstIndex+3] = 255
		}
	}
	return nil
}

func (session *recordingSession) currentState() recordingState {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.state
}

func (session *recordingSession) notifyChanged() {
	if session.config.OnChanged != nil {
		session.config.OnChanged()
	}
}

// transition performs one guarded state update and rejects duplicate UI actions.
func (session *recordingSession) transition(from []recordingState, to recordingState) bool {
	session.mu.Lock()
	defer session.mu.Unlock()
	for _, allowed := range from {
		if session.state == allowed {
			session.state = to
			return true
		}
	}
	return false
}

// Start prepares a fresh temporary MP4 and begins the non-blocking countdown.
func (session *recordingSession) Start(ctx context.Context) error {
	session.operationMu.Lock()
	defer session.operationMu.Unlock()
	if !session.transition([]recordingState{recordingStateReady}, recordingStatePreparing) {
		return fmt.Errorf("cannot start recording from %s", session.currentState())
	}
	session.notifyChanged()
	if err := cleanupRecordingOrphans(session.config.TempRoot, session.config.Now(), recordingOrphanMaxAge); err != nil && session.config.Diagnostics {
		util.GetLogger().Warn(ctx, fmt.Sprintf("recording orphan cleanup failed: %v", err))
	}
	if err := os.MkdirAll(session.config.TempRoot, 0o700); err != nil {
		return session.fail(fmt.Errorf("create recording temp directory: %w", err))
	}
	file, err := os.CreateTemp(session.config.TempRoot, "wox-recording-*.mp4")
	if err != nil {
		return session.fail(fmt.Errorf("create recording temp file: %w", err))
	}
	tempPath := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		return session.fail(fmt.Errorf("close recording temp file: %w", err))
	}
	if err := session.config.Encoder.Start(tempPath, session.config.PixelBounds.Dx(), session.config.PixelBounds.Dy(), session.config.FPS); err != nil {
		_ = os.Remove(tempPath)
		return session.fail(err)
	}
	if session.config.Diagnostics {
		util.GetLogger().Info(ctx, fmt.Sprintf("recording prepared: backend=ffmpeg pixels=%dx%d fps=%d", session.config.PixelBounds.Dx(), session.config.PixelBounds.Dy(), session.config.FPS))
	}
	session.mu.Lock()
	session.tempPath = tempPath
	session.state = recordingStateCountdown
	session.countdownEndsAt = session.config.Now().Add(session.config.Countdown)
	countdownCtx, cancel := context.WithCancel(ctx)
	session.stopCapture = cancel
	session.mu.Unlock()
	session.notifyChanged()
	go session.countdown(countdownCtx)
	return nil
}

// CountdownRemaining reports the visible pre-roll time without changing session state.
func (session *recordingSession) CountdownRemaining() time.Duration {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.state != recordingStateCountdown {
		return 0
	}
	return max(time.Duration(0), session.countdownEndsAt.Sub(session.config.Now()))
}

// countdown opens the frame and encoder workers only after the pre-roll finishes.
func (session *recordingSession) countdown(parent context.Context) {
	if err := session.config.Sleep(parent, session.config.Countdown); err != nil {
		_ = session.Cancel()
		return
	}
	session.mu.Lock()
	if session.state != recordingStateCountdown {
		session.mu.Unlock()
		return
	}
	session.startedAt = session.config.Now()
	session.frames = make(chan recordingFrame, recordingFrameQueueSize)
	session.workerDone = make(chan struct{})
	session.captureDone = make(chan struct{})
	session.state = recordingStateRecording
	session.mu.Unlock()
	session.notifyChanged()
	go session.encodeLoop()
	go session.captureLoop(parent)
}

// captureLoop samples at the chosen cadence and replaces stale queued frames under backpressure.
func (session *recordingSession) captureLoop(ctx context.Context) {
	defer close(session.captureDone)
	ticker := time.NewTicker(time.Second / time.Duration(session.config.FPS))
	defer ticker.Stop()
	var index int64
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			session.mu.Lock()
			if session.state != recordingStateRecording {
				session.mu.Unlock()
				continue
			}
			captureEpoch := session.captureEpoch
			session.mu.Unlock()
			captureStartedAt := time.Now()
			source, err := session.config.Capture()
			if err != nil {
				session.setRuntimeError(fmt.Errorf("capture recording frame: %w", err))
				return
			}
			composed, err := session.config.Compose(source)
			atomic.AddInt64(&session.captureNanos, time.Since(captureStartedAt).Nanoseconds())
			atomic.AddInt64(&session.captureSamples, 1)
			if err != nil {
				session.setRuntimeError(fmt.Errorf("compose recording frame: %w", err))
				return
			}
			session.mu.Lock()
			if session.state != recordingStateRecording || session.captureEpoch != captureEpoch {
				session.mu.Unlock()
				continue
			}
			// Raw-video encoders have a fixed cadence. Index frames from the active monotonic
			// timeline so slow capture produces held frames instead of a time-compressed video.
			timelineIndex := int64(session.effectiveDurationLocked(session.config.Now())*time.Duration(session.config.FPS)/time.Second) - 1
			if timelineIndex < 0 {
				timelineIndex = 0
			}
			if timelineIndex < index {
				session.mu.Unlock()
				continue
			}
			frame := recordingFrame{image: composed, index: timelineIndex}
			select {
			case session.frames <- frame:
			default:
				select {
				case <-session.frames:
					atomic.AddInt64(&session.framesDropped, 1)
				default:
				}
				select {
				case session.frames <- frame:
				default:
					atomic.AddInt64(&session.framesDropped, 1)
				}
			}
			index = timelineIndex + 1
			session.mu.Unlock()
		}
	}
}

// encodeLoop is the sole encoder writer, preserving monotonic frame ordering.
func (session *recordingSession) encodeLoop() {
	defer close(session.workerDone)
	for frame := range session.frames {
		if err := session.config.Encoder.WriteFrame(frame); err != nil {
			session.mu.Lock()
			session.workerErr = err
			session.mu.Unlock()
			session.setRuntimeError(fmt.Errorf("encode recording frame: %w", err))
			return
		}
		atomic.AddInt64(&session.framesWritten, 1)
	}
}

// setRuntimeError stops capture while preserving an already encoded temporary artifact.
func (session *recordingSession) setRuntimeError(err error) {
	session.mu.Lock()
	if session.error == nil {
		session.error = err
	}
	session.state = recordingStateError
	cancel := session.stopCapture
	session.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if session.config.Diagnostics {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("recording runtime failed: backend=ffmpeg pixels=%dx%d fps=%d written=%d dropped=%d reason=%v", session.config.PixelBounds.Dx(), session.config.PixelBounds.Dy(), session.config.FPS, atomic.LoadInt64(&session.framesWritten), atomic.LoadInt64(&session.framesDropped), err))
	}
	session.notifyChanged()
}

func (session *recordingSession) fail(err error) error {
	session.mu.Lock()
	session.error = err
	session.state = recordingStateError
	session.mu.Unlock()
	session.notifyChanged()
	return err
}

// Pause excludes subsequent wall time and frame capture from the encoded timeline.
func (session *recordingSession) Pause() error {
	session.mu.Lock()
	if session.state != recordingStateRecording {
		state := session.state
		session.mu.Unlock()
		return fmt.Errorf("cannot pause recording from %s", state)
	}
	session.state = recordingStatePaused
	session.pausedAt = session.config.Now()
	session.captureEpoch++
	session.mu.Unlock()
	session.DiscardPendingFrames()
	session.notifyChanged()
	return nil
}

// Resume continues the current take without including paused time.
func (session *recordingSession) Resume() error {
	session.mu.Lock()
	if session.state != recordingStatePaused {
		state := session.state
		session.mu.Unlock()
		return fmt.Errorf("cannot resume recording from %s", state)
	}
	session.pausedDuration += session.config.Now().Sub(session.pausedAt)
	session.pausedAt = time.Time{}
	session.state = recordingStateRecording
	session.mu.Unlock()
	session.notifyChanged()
	return nil
}

// EffectiveDuration returns the monotonic take duration with pauses removed.
func (session *recordingSession) EffectiveDuration() time.Duration {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.effectiveDurationLocked(session.config.Now())
}

func (session *recordingSession) effectiveDurationLocked(now time.Time) time.Duration {
	if session.startedAt.IsZero() {
		return 0
	}
	paused := session.pausedDuration
	if session.state == recordingStatePaused {
		paused += now.Sub(session.pausedAt)
	}
	return max(time.Duration(0), now.Sub(session.startedAt)-paused)
}

// DiscardPendingFrames removes stale captures before an in-selection toolbar is exposed.
func (session *recordingSession) DiscardPendingFrames() {
	session.mu.Lock()
	frames := session.frames
	session.mu.Unlock()
	if frames == nil {
		return
	}
	for {
		select {
		case <-frames:
			atomic.AddInt64(&session.framesDropped, 1)
		default:
			return
		}
	}
}

// stopPipelines waits for capture to exit before closing the encoder queue.
func (session *recordingSession) stopPipelines() error {
	session.mu.Lock()
	cancel := session.stopCapture
	frames := session.frames
	done := session.workerDone
	captureDone := session.captureDone
	session.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if captureDone != nil {
		<-captureDone
	}
	session.stopOnce.Do(func() {
		if frames != nil {
			close(frames)
		}
	})
	if done != nil {
		<-done
	}
	if session.config.Release != nil {
		session.config.Release()
		session.config.Release = nil
	}
	session.mu.Lock()
	err := session.workerErr
	session.mu.Unlock()
	return err
}

// Finish flushes the encoder and leaves the temporary MP4 available for Save As.
func (session *recordingSession) Finish() (string, error) {
	session.operationMu.Lock()
	defer session.operationMu.Unlock()
	if !session.transition([]recordingState{recordingStateRecording, recordingStatePaused, recordingStateError}, recordingStateFinalizing) {
		return "", fmt.Errorf("cannot finish recording from %s", session.currentState())
	}
	session.notifyChanged()
	stopErr := session.stopPipelines()
	if err := session.config.Encoder.Finalize(); err != nil {
		return "", session.fail(err)
	}
	if stopErr != nil && atomic.LoadInt64(&session.framesWritten) == 0 {
		return "", session.fail(stopErr)
	}
	session.mu.Lock()
	path := session.tempPath
	session.state = recordingStateSave
	session.mu.Unlock()
	session.notifyChanged()
	if session.config.Diagnostics {
		samples := atomic.LoadInt64(&session.captureSamples)
		captureAvgMs := 0.0
		if samples > 0 {
			captureAvgMs = float64(atomic.LoadInt64(&session.captureNanos)) / float64(samples) / 1e6
		}
		util.GetLogger().Info(context.Background(), fmt.Sprintf("recording finalized: backend=ffmpeg pixels=%dx%d fps=%d written=%d dropped=%d captureAvgMs=%.1f reason=completed", session.config.PixelBounds.Dx(), session.config.PixelBounds.Dy(), session.config.FPS, atomic.LoadInt64(&session.framesWritten), atomic.LoadInt64(&session.framesDropped), captureAvgMs))
	}
	return path, nil
}

// TempPath returns the session MP4 while it remains available for preview or Save As.
func (session *recordingSession) TempPath() string {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.tempPath
}

// Cancel stops all work and deletes the current temporary MP4.
func (session *recordingSession) Cancel() error {
	session.operationMu.Lock()
	defer session.operationMu.Unlock()
	session.mu.Lock()
	state := session.state
	session.mu.Unlock()
	if state == recordingStateClosed {
		return nil
	}
	_ = session.stopPipelines()
	_ = session.config.Encoder.Abort()
	session.mu.Lock()
	path := session.tempPath
	session.tempPath = ""
	session.state = recordingStateClosed
	session.mu.Unlock()
	session.notifyChanged()
	if session.config.Diagnostics {
		util.GetLogger().Info(context.Background(), fmt.Sprintf("recording ended: backend=ffmpeg pixels=%dx%d fps=%d written=%d dropped=%d reason=cancelled", session.config.PixelBounds.Dx(), session.config.PixelBounds.Dy(), session.config.FPS, atomic.LoadInt64(&session.framesWritten), atomic.LoadInt64(&session.framesDropped)))
	}
	if path != "" {
		return os.Remove(path)
	}
	return nil
}

// Save atomically publishes the finalized MP4 and removes the session temporary file.
func (session *recordingSession) Save(target string) error {
	session.operationMu.Lock()
	defer session.operationMu.Unlock()
	session.mu.Lock()
	if session.state != recordingStateSave {
		state := session.state
		session.mu.Unlock()
		return fmt.Errorf("cannot save recording from %s", state)
	}
	source := session.tempPath
	session.mu.Unlock()
	if err := copyRecordingAtomically(source, target); err != nil {
		return err
	}
	if err := os.Remove(source); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove recording temporary file: %w", err)
	}
	session.mu.Lock()
	session.tempPath = ""
	session.state = recordingStateClosed
	session.mu.Unlock()
	session.notifyChanged()
	return nil
}

// cleanupRecordingOrphans removes only stale MP4 files from the dedicated recording directory.
func cleanupRecordingOrphans(root string, now time.Time, maxAge time.Duration) error {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".mp4" {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || now.Sub(info.ModTime()) <= maxAge {
			continue
		}
		if removeErr := os.Remove(filepath.Join(root, entry.Name())); removeErr != nil && !os.IsNotExist(removeErr) {
			return removeErr
		}
	}
	return nil
}

// copyRecordingAtomically uses a sibling file so failed copies never leave a partial destination.
func copyRecordingAtomically(source, target string) error {
	if source == "" || target == "" {
		return errors.New("recording source and target paths are required")
	}
	sourcePath, sourceErr := filepath.Abs(source)
	targetPath, targetErr := filepath.Abs(target)
	if sourceErr == nil && targetErr == nil {
		samePath := filepath.Clean(sourcePath) == filepath.Clean(targetPath)
		if runtime.GOOS == "windows" {
			samePath = strings.EqualFold(filepath.Clean(sourcePath), filepath.Clean(targetPath))
		}
		if samePath {
			return errors.New("recording destination must differ from the temporary file")
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create recording destination directory: %w", err)
	}
	sibling, err := os.CreateTemp(filepath.Dir(target), ".wox-recording-*.tmp")
	if err != nil {
		return fmt.Errorf("create sibling recording file: %w", err)
	}
	siblingPath := sibling.Name()
	defer os.Remove(siblingPath)
	input, err := os.Open(source)
	if err != nil {
		_ = sibling.Close()
		return fmt.Errorf("open finalized recording: %w", err)
	}
	_, copyErr := io.Copy(sibling, input)
	closeInputErr := input.Close()
	syncErr := sibling.Sync()
	closeOutputErr := sibling.Close()
	if copyErr != nil {
		return fmt.Errorf("copy finalized recording: %w", copyErr)
	}
	if closeInputErr != nil || syncErr != nil || closeOutputErr != nil {
		return errors.New("flush finalized recording failed")
	}
	if err := replaceRecordingFile(siblingPath, target); err != nil {
		return fmt.Errorf("publish finalized recording: %w", err)
	}
	return nil
}

type ffmpegRecordingEncoder struct {
	mu             sync.Mutex
	cmd            *exec.Cmd
	stdin          io.WriteCloser
	stderr         bytes.Buffer
	abort          bool
	nextFrameIndex int64
	lastYUV        []byte
}

func (encoder *ffmpegRecordingEncoder) Start(path string, width, height, fps int) error {
	ffmpegPath, err := recordingFFmpegPath()
	if err != nil {
		return errors.New("H.264 recording runtime is unavailable")
	}
	arguments := []string{
		"-hide_banner", "-loglevel", "error", "-f", "rawvideo", "-pixel_format", "yuv420p",
		"-video_size", strconv.Itoa(width) + "x" + strconv.Itoa(height), "-framerate", strconv.Itoa(fps),
		"-i", "pipe:0", "-an", "-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency",
		"-bf", "0", "-g", strconv.Itoa(fps * 2), "-keyint_min", strconv.Itoa(fps * 2),
		"-pix_fmt", "yuv420p", "-movflags", "+faststart", "-y", path,
	}
	cmd := exec.Command(ffmpegPath, arguments...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("open H.264 encoder input: %w", err)
	}
	cmd.Stderr = &encoder.stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return fmt.Errorf("start H.264 encoder: %w", err)
	}
	encoder.mu.Lock()
	encoder.cmd = cmd
	encoder.stdin = stdin
	encoder.abort = false
	encoder.nextFrameIndex = 0
	encoder.lastYUV = nil
	encoder.mu.Unlock()
	return nil
}

// recordingFFmpegPath prefers the packaged, version-pinned runtime and retains a development fallback.
func recordingFFmpegPath() (string, error) {
	executable := "ffmpeg"
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	dataRoot := util.GetLocation().GetWoxDataDirectory()
	if dataRoot != "" {
		packaged := filepath.Join(util.GetLocation().GetOthersDirectory(), "recording", runtime.GOOS+"-"+runtime.GOARCH, executable)
		if util.IsFileExists(packaged) {
			if runtime.GOOS != "windows" {
				if err := os.Chmod(packaged, 0o755); err != nil {
					return "", fmt.Errorf("make packaged recording runtime executable: %w", err)
				}
			}
			return packaged, nil
		}
	}
	return exec.LookPath("ffmpeg")
}

func (encoder *ffmpegRecordingEncoder) WriteFrame(frame recordingFrame) error {
	encoder.mu.Lock()
	stdin := encoder.stdin
	aborted := encoder.abort
	encoder.mu.Unlock()
	if stdin == nil || aborted {
		return errors.New("H.264 encoder is not running")
	}
	if frame.image == nil {
		return errors.New("H.264 frame is empty")
	}
	if frame.index < encoder.nextFrameIndex {
		return nil
	}
	yuv, err := bgraToI420(frame.image)
	if err != nil {
		return err
	}
	filler := encoder.lastYUV
	if filler == nil {
		filler = yuv
	}
	for encoder.nextFrameIndex < frame.index {
		if _, err := stdin.Write(filler); err != nil {
			return err
		}
		encoder.nextFrameIndex++
	}
	if _, err := stdin.Write(yuv); err != nil {
		return err
	}
	encoder.nextFrameIndex++
	encoder.lastYUV = yuv
	return nil
}

// bgraToI420 converts a BGR0/BGRA capture into planar YUV 4:2:0 for the H.264 pipe.
func bgraToI420(frame *image.RGBA) ([]byte, error) {
	width, height := frame.Rect.Dx(), frame.Rect.Dy()
	if width <= 0 || height <= 0 || width%2 != 0 || height%2 != 0 {
		return nil, errors.New("I420 frame must have positive even dimensions")
	}
	ySize := width * height
	uvSize := ySize / 4
	output := make([]byte, ySize+uvSize*2)
	yPlane := output[:ySize]
	uPlane := output[ySize : ySize+uvSize]
	vPlane := output[ySize+uvSize:]
	stride := frame.Stride
	pix := frame.Pix
	for row := 0; row < height; row++ {
		rowOffset := row * stride
		yRow := row * width
		for col := 0; col < width; col++ {
			index := rowOffset + col*4
			blue, green, red := int(pix[index]), int(pix[index+1]), int(pix[index+2])
			y := (66*red + 129*green + 25*blue + 128) >> 8
			yPlane[yRow+col] = byte(y + 16)
			if row%2 == 0 && col%2 == 0 {
				u := (-38*red - 74*green + 112*blue + 128) >> 8
				v := (112*red - 94*green - 18*blue + 128) >> 8
				uvIndex := (row/2)*(width/2) + col/2
				uPlane[uvIndex] = byte(u + 128)
				vPlane[uvIndex] = byte(v + 128)
			}
		}
	}
	return output, nil
}

func (encoder *ffmpegRecordingEncoder) Finalize() error {
	encoder.mu.Lock()
	stdin, cmd := encoder.stdin, encoder.cmd
	encoder.stdin = nil
	encoder.mu.Unlock()
	if stdin == nil || cmd == nil {
		return errors.New("H.264 encoder is not running")
	}
	if err := stdin.Close(); err != nil {
		return fmt.Errorf("close H.264 encoder input: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("finalize H.264 encoder: %w: %s", err, encoder.stderr.String())
	}
	return nil
}

func (encoder *ffmpegRecordingEncoder) Abort() error {
	encoder.mu.Lock()
	encoder.abort = true
	stdin, cmd := encoder.stdin, encoder.cmd
	encoder.stdin = nil
	encoder.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	return nil
}
