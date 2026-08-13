package screenshot

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"wox/util/keyboard"
)

type recordingTestEncoder struct {
	mu        sync.Mutex
	started   bool
	finalized bool
	aborted   bool
	frames    int
}

type recordingTestWriteCloser struct {
	bytes.Buffer
}

func (writer *recordingTestWriteCloser) Close() error { return nil }

func (encoder *recordingTestEncoder) Start(path string, width, height, fps int) error {
	encoder.mu.Lock()
	defer encoder.mu.Unlock()
	encoder.started = true
	return nil
}

func (encoder *recordingTestEncoder) WriteFrame(frame recordingFrame) error {
	encoder.mu.Lock()
	defer encoder.mu.Unlock()
	encoder.frames++
	return nil
}

func (encoder *recordingTestEncoder) Finalize() error {
	encoder.mu.Lock()
	defer encoder.mu.Unlock()
	encoder.finalized = true
	return nil
}

func (encoder *recordingTestEncoder) Abort() error {
	encoder.mu.Lock()
	defer encoder.mu.Unlock()
	encoder.aborted = true
	return nil
}

func waitForRecordingState(t *testing.T, session *recordingSession, want recordingState) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if session.currentState() == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("recording state = %s, want %s", session.currentState(), want)
}

func TestRecordingSessionStateTransitionsExcludePausedTime(t *testing.T) {
	now := time.Unix(100, 0)
	encoder := &recordingTestEncoder{}
	session, err := newRecordingSession(recordingSessionConfig{
		FPS: 30, PixelBounds: image.Rect(0, 0, 4, 2), Encoder: encoder, TempRoot: t.TempDir(),
		Capture: func() (image.Image, error) { return image.NewRGBA(image.Rect(0, 0, 4, 2)), nil },
		Now:     func() time.Time { return now },
		Sleep:   func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatalf("create recording session: %v", err)
	}
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("start recording: %v", err)
	}
	waitForRecordingState(t, session, recordingStateRecording)
	now = now.Add(2 * time.Second)
	if got := session.EffectiveDuration(); got != 2*time.Second {
		t.Fatalf("recorded duration = %s, want 2s", got)
	}
	if err := session.Pause(); err != nil {
		t.Fatalf("pause recording: %v", err)
	}
	now = now.Add(5 * time.Second)
	if got := session.EffectiveDuration(); got != 2*time.Second {
		t.Fatalf("paused duration = %s, want 2s", got)
	}
	if err := session.Resume(); err != nil {
		t.Fatalf("resume recording: %v", err)
	}
	now = now.Add(time.Second)
	if got := session.EffectiveDuration(); got != 3*time.Second {
		t.Fatalf("resumed duration = %s, want 3s", got)
	}
	path, err := session.Finish()
	if err != nil {
		t.Fatalf("finish recording: %v", err)
	}
	if path == "" || session.currentState() != recordingStateSave || !encoder.finalized {
		t.Fatalf("finalized path=%q state=%s encoder=%t", path, session.currentState(), encoder.finalized)
	}
}

func TestRecordingSessionRejectsDuplicateControlsAndCleansTempFile(t *testing.T) {
	encoder := &recordingTestEncoder{}
	session, err := newRecordingSession(recordingSessionConfig{
		FPS: 60, PixelBounds: image.Rect(0, 0, 8, 6), Encoder: encoder, TempRoot: t.TempDir(),
		Capture: func() (image.Image, error) { return image.NewRGBA(image.Rect(0, 0, 8, 6)), nil },
		Sleep:   func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatalf("create recording session: %v", err)
	}
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("start recording: %v", err)
	}
	if err := session.Start(context.Background()); err == nil {
		t.Fatal("duplicate start should be rejected")
	}
	waitForRecordingState(t, session, recordingStateRecording)
	session.mu.Lock()
	path := session.tempPath
	session.mu.Unlock()
	if err := session.Cancel(); err != nil {
		t.Fatalf("cancel recording: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("temporary recording still exists: %v", err)
	}
	if !encoder.aborted || session.currentState() != recordingStateClosed {
		t.Fatalf("cancel state=%s encoder aborted=%t", session.currentState(), encoder.aborted)
	}
	if err := session.Cancel(); err != nil {
		t.Fatalf("duplicate cancel should be harmless: %v", err)
	}
}

func TestRecordingSessionCancelStopsCountdownWithoutResurrection(t *testing.T) {
	countdownStarted := make(chan struct{})
	countdownStopped := make(chan struct{})
	encoder := &recordingTestEncoder{}
	session, err := newRecordingSession(recordingSessionConfig{
		FPS: 30, PixelBounds: image.Rect(0, 0, 8, 6), Encoder: encoder, TempRoot: t.TempDir(),
		Capture: func() (image.Image, error) { return image.NewRGBA(image.Rect(0, 0, 8, 6)), nil },
		Sleep: func(ctx context.Context, _ time.Duration) error {
			close(countdownStarted)
			<-ctx.Done()
			close(countdownStopped)
			return ctx.Err()
		},
	})
	if err != nil {
		t.Fatalf("create recording session: %v", err)
	}
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("start recording: %v", err)
	}
	<-countdownStarted
	if err := session.Cancel(); err != nil {
		t.Fatalf("cancel countdown: %v", err)
	}
	select {
	case <-countdownStopped:
	case <-time.After(time.Second):
		t.Fatal("countdown goroutine did not stop after cancellation")
	}
	time.Sleep(10 * time.Millisecond)
	if state := session.currentState(); state != recordingStateClosed {
		t.Fatalf("cancelled countdown state = %s, want closed", state)
	}
}

func TestRecordingSessionSerializesConcurrentFinishAndCancel(t *testing.T) {
	encoder := &recordingTestEncoder{}
	session, err := newRecordingSession(recordingSessionConfig{
		FPS: 30, PixelBounds: image.Rect(0, 0, 8, 6), Encoder: encoder, TempRoot: t.TempDir(),
		Capture: func() (image.Image, error) { return image.NewRGBA(image.Rect(0, 0, 8, 6)), nil },
		Sleep:   func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatalf("create recording session: %v", err)
	}
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("start recording: %v", err)
	}
	waitForRecordingState(t, session, recordingStateRecording)
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		_, _ = session.Finish()
	}()
	go func() {
		defer workers.Done()
		<-start
		_ = session.Cancel()
	}()
	close(start)
	workers.Wait()
	if state := session.currentState(); state != recordingStateClosed {
		t.Fatalf("concurrent stop state = %s, want closed", state)
	}
}

func TestNormalizeRecordingPixelBoundsTrimsRightAndBottom(t *testing.T) {
	got := normalizeRecordingPixelBounds(image.Rect(-100, 25, 701, 626))
	want := image.Rect(-100, 25, 700, 625)
	if got != want {
		t.Fatalf("normalized recording bounds = %v, want %v", got, want)
	}
}

func TestNormalizeRecordingLogicalSelectionSynchronizesEvenPixelCrop(t *testing.T) {
	got, err := normalizeRecordingLogicalSelection(image.Rect(0, 0, 2001, 1001), Rect{X: 100, Y: 50, Width: 801, Height: 501}, Size{Width: 2001, Height: 1001})
	if err != nil {
		t.Fatalf("normalize logical selection: %v", err)
	}
	pixels, err := screenshotEditorPixelSelection(image.Rect(0, 0, 2001, 1001), got, Size{Width: 2001, Height: 1001})
	if err != nil {
		t.Fatalf("map normalized selection: %v", err)
	}
	if pixels.Dx()%2 != 0 || pixels.Dy()%2 != 0 {
		t.Fatalf("normalized pixel selection = %v, want even dimensions", pixels)
	}
}

func TestRecordingToolbarLayoutUsesEveryOutsideEdgeAndCollapsesForFullscreen(t *testing.T) {
	frame := Rect{X: -200, Y: -100, Width: 1400, Height: 900}
	tests := []struct {
		name      string
		selection Rect
		edge      string
	}{
		{name: "bottom", selection: Rect{X: 0, Y: 0, Width: 900, Height: 300}, edge: "bottom"},
		{name: "top", selection: Rect{X: 0, Y: 500, Width: 900, Height: 250}, edge: "top"},
		{name: "left", selection: Rect{X: 600, Y: -100, Width: 500, Height: 900}, edge: "left"},
		{name: "right", selection: Rect{X: -180, Y: -100, Width: 220, Height: 900}, edge: "right"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			expanded, _, collapsed := recordingToolbarLayout(frame, testCase.selection, recordingToolbarWidth, recordingToolbarHeight)
			if collapsed {
				t.Fatalf("toolbar unexpectedly collapsed: %+v", expanded)
			}
			switch testCase.edge {
			case "bottom":
				if expanded.Y < testCase.selection.Y+testCase.selection.Height {
					t.Fatalf("bottom toolbar = %+v", expanded)
				}
			case "top":
				if expanded.Y+expanded.Height > testCase.selection.Y {
					t.Fatalf("top toolbar = %+v", expanded)
				}
			case "left":
				if expanded.X+expanded.Width > testCase.selection.X {
					t.Fatalf("left toolbar = %+v", expanded)
				}
			case "right":
				if expanded.X < testCase.selection.X+testCase.selection.Width {
					t.Fatalf("right toolbar = %+v", expanded)
				}
			}
		})
	}
	_, hotspot, collapsed := recordingToolbarLayout(frame, frame, recordingToolbarWidth, recordingToolbarHeight)
	if !collapsed || hotspot.Width != 18 || hotspot.Height != 60 {
		t.Fatalf("fullscreen hotspot = %+v collapsed=%t", hotspot, collapsed)
	}
}

func TestRecordingToolbarLayoutScalesOutsideTheSelection(t *testing.T) {
	frame := Rect{Width: 1920, Height: 1080}
	selection := Rect{X: 200, Y: 200, Width: 800, Height: 400}
	expanded, _, collapsed := recordingToolbarLayout(frame, selection, recordingToolbarWidth*2.5, recordingToolbarHeight*2.5)
	if collapsed {
		t.Fatalf("scaled toolbar collapsed: %+v", expanded)
	}
	if expanded.Width != recordingToolbarWidth*2.5 || expanded.Height != recordingToolbarHeight*2.5 {
		t.Fatalf("scaled toolbar size = %+v", expanded)
	}
	if screenshotEditorRectsOverlap(expanded, selection) {
		t.Fatalf("scaled toolbar overlaps selection: toolbar=%+v selection=%+v", expanded, selection)
	}
}

func TestRecordingBorderPointerMovesAndResizesReadySelection(t *testing.T) {
	newState := func() *recordingToolbarState {
		selection := Rect{X: 100, Y: 100, Width: 400, Height: 300}
		return &recordingToolbarState{
			selection: selection, frameSize: Size{Width: 1000, Height: 800},
			borderOrigin: Point{X: selection.X - recordingBorderMargin, Y: selection.Y - recordingBorderMargin},
			editor: &screenshotEditorOverlayState{
				selection: selection, frameSize: Size{Width: 1000, Height: 800}, hasSelection: true,
				activeTool: screenshotEditorToolSelect, uiScale: 1,
			},
		}
	}
	moving := newState()
	moving.borderPointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: recordingBorderMargin + 50, Y: recordingBorderMargin + 50}})
	moving.borderPointer(PointerEvent{Kind: PointerMove, Button: PointerButtonPrimary, Position: Point{X: recordingBorderMargin + 70, Y: recordingBorderMargin + 65}})
	if moving.selection.X != 120 || moving.selection.Y != 115 {
		t.Fatalf("moved selection = %+v", moving.selection)
	}

	resizing := newState()
	resizing.borderPointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: recordingBorderMargin, Y: recordingBorderMargin}})
	resizing.borderPointer(PointerEvent{Kind: PointerMove, Button: PointerButtonPrimary, Position: Point{X: recordingBorderMargin + 10, Y: recordingBorderMargin + 10}})
	if resizing.selection.X != 110 || resizing.selection.Y != 110 || resizing.selection.Width != 390 || resizing.selection.Height != 290 {
		t.Fatalf("resized selection = %+v", resizing.selection)
	}
}

func TestRecordingSelectionEdgeContainsLeavesInteriorClickable(t *testing.T) {
	selection := Rect{X: 100, Y: 100, Width: 400, Height: 300}
	for _, point := range []Point{{X: 100, Y: 250}, {X: 500, Y: 250}, {X: 300, Y: 100}, {X: 300, Y: 400}, {X: 94, Y: 94}} {
		if !recordingSelectionEdgeContains(selection, point, 14) {
			t.Fatalf("border point %+v should be interactive", point)
		}
	}
	for _, point := range []Point{{X: 300, Y: 250}, {X: 50, Y: 50}, {X: 520, Y: 420}, {X: 94, Y: 180}} {
		if recordingSelectionEdgeContains(selection, point, 14) {
			t.Fatalf("non-border point %+v should pass through", point)
		}
	}
}

func TestRecordingSessionDiscardPendingFrames(t *testing.T) {
	session := &recordingSession{frames: make(chan recordingFrame, 2)}
	session.frames <- recordingFrame{index: 1}
	session.frames <- recordingFrame{index: 2}
	session.DiscardPendingFrames()
	if got := len(session.frames); got != 0 {
		t.Fatalf("pending frame count = %d, want 0", got)
	}
	if got := session.framesDropped; got != 2 {
		t.Fatalf("dropped frame count = %d, want 2", got)
	}
}

func TestRecordingKeyLabels(t *testing.T) {
	label := recordingKeyLabel(keyboard.RawKeyEvent{
		Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyK, Character: "k",
		Modifiers: keyboard.ModifierCtrl | keyboard.ModifierShift,
	})
	if label != "Ctrl+Shift+K" {
		t.Fatalf("key label = %q", label)
	}
	if modifier := recordingKeyLabel(keyboard.RawKeyEvent{Key: keyboard.KeyLeftCtrl}); modifier != "Ctrl" {
		t.Fatalf("standalone modifier label = %q", modifier)
	}
	if functional := recordingKeyLabel(keyboard.RawKeyEvent{Key: keyboard.KeyReturn}); functional != "Enter" {
		t.Fatalf("functional key label = %q", functional)
	}
}

func TestRecordingRawKeyKeepsNewestSixKeycaps(t *testing.T) {
	state := &recordingToolbarState{editor: &screenshotEditorOverlayState{}, showKeypress: true}
	for index, key := range []keyboard.Key{keyboard.KeyA, keyboard.KeyB, keyboard.KeyC, keyboard.KeyD, keyboard.KeyE, keyboard.KeyF, keyboard.KeyG} {
		state.rawKey(keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: key, Character: string(rune('a' + index))})
	}
	if len(state.keycaps) != 6 || state.keycaps[0].label != "B" || state.keycaps[5].label != "G" {
		t.Fatalf("keycaps = %+v, want newest B-G", state.keycaps)
	}
}

func TestRecordingKeycapsExpireWithoutPersistence(t *testing.T) {
	state := &recordingToolbarState{
		editor:  &screenshotEditorOverlayState{image: testScreenshotImage(t, 20, 20), selection: Rect{Width: 20, Height: 20}},
		keycaps: []recordingKeycap{{label: "A", expiresAt: time.Now().Add(-time.Millisecond)}},
	}
	state.drawOverlay(&DisplayList{}, FrameInfo{Size: Size{Width: 20, Height: 20}})
	if len(state.keycaps) != 0 {
		t.Fatalf("expired keycaps = %+v, want none", state.keycaps)
	}
}

func TestRenderRecordingKeycapsCompositesOnlyInsideSelection(t *testing.T) {
	target := image.NewRGBA(image.Rect(0, 0, 200, 120))
	for index := 0; index < len(target.Pix); index += 4 {
		target.Pix[index], target.Pix[index+1], target.Pix[index+2], target.Pix[index+3] = 255, 255, 255, 255
	}
	selection := Rect{X: 30, Y: 20, Width: 140, Height: 90}
	if err := renderRecordingKeycaps(target, selection, Size{Width: 200, Height: 120}, []recordingKeycap{{label: "Ctrl+K", expiresAt: time.Now().Add(time.Second)}}, time.Now(), 1); err != nil {
		t.Fatalf("render keycaps: %v", err)
	}
	if outside := target.RGBAAt(5, 5); outside != (color.RGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatalf("outside selection pixel changed: %+v", outside)
	}
	changed := false
	for y := 58; y < 110 && !changed; y++ {
		for x := 30; x < 170; x++ {
			if pixel := target.RGBAAt(x, y); pixel.R < 200 {
				changed = true
				break
			}
		}
	}
	if !changed {
		t.Fatal("keycap was not composited into the selected recording pixels")
	}
}

func TestCleanupRecordingOrphansKeepsRecentAndUnrelatedFiles(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	oldVideo := filepath.Join(root, "old.mp4")
	recentVideo := filepath.Join(root, "recent.mp4")
	unrelated := filepath.Join(root, "old.txt")
	for _, path := range []string{oldVideo, recentVideo, unrelated} {
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatalf("create fixture: %v", err)
		}
	}
	oldTime := now.Add(-25 * time.Hour)
	if err := os.Chtimes(oldVideo, oldTime, oldTime); err != nil {
		t.Fatalf("age old video: %v", err)
	}
	if err := os.Chtimes(unrelated, oldTime, oldTime); err != nil {
		t.Fatalf("age unrelated file: %v", err)
	}
	if err := cleanupRecordingOrphans(root, now, 24*time.Hour); err != nil {
		t.Fatalf("cleanup orphans: %v", err)
	}
	if _, err := os.Stat(oldVideo); !os.IsNotExist(err) {
		t.Fatalf("old video was not removed: %v", err)
	}
	for _, path := range []string{recentVideo, unrelated} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("preserved file %q: %v", path, err)
		}
	}
}

func TestCopyRecordingAtomicallyReplacesTarget(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.mp4")
	target := filepath.Join(root, "target.mp4")
	if err := os.WriteFile(source, []byte("new video"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(target, []byte("old video"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := copyRecordingAtomically(source, target); err != nil {
		t.Fatalf("publish recording: %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(content) != "new video" {
		t.Fatalf("target content = %q", content)
	}
}

func TestCopyRecordingAtomicallyRejectsTemporaryFileAsTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recording.mp4")
	if err := os.WriteFile(path, []byte("video"), 0o600); err != nil {
		t.Fatalf("write recording: %v", err)
	}
	if err := copyRecordingAtomically(path, path); err == nil {
		t.Fatal("publishing over the temporary file should be rejected")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("temporary file should remain retryable: %v", err)
	}
}

func TestFFmpegRecordingEncoderProducesSilentH264MP4(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is unavailable")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is unavailable")
	}
	path := filepath.Join(t.TempDir(), "synthetic.mp4")
	encoder := &ffmpegRecordingEncoder{}
	if err := encoder.Start(path, 64, 32, 30); err != nil {
		t.Fatalf("start encoder: %v", err)
	}
	for index := int64(0); index < 12; index++ {
		frame := image.NewRGBA(image.Rect(0, 0, 64, 32))
		fill := color.RGBA{R: uint8(index * 10), G: 80, B: 160, A: 255}
		for pixel := 0; pixel < len(frame.Pix); pixel += 4 {
			frame.Pix[pixel], frame.Pix[pixel+1], frame.Pix[pixel+2], frame.Pix[pixel+3] = fill.R, fill.G, fill.B, fill.A
		}
		if err := encoder.WriteFrame(recordingFrame{image: frame, index: index}); err != nil {
			t.Fatalf("encode frame: %v", err)
		}
	}
	if err := encoder.Finalize(); err != nil {
		t.Fatalf("finalize encoder: %v", err)
	}
	output, err := exec.Command(ffprobe, "-v", "error", "-show_entries", "stream=codec_type,codec_name", "-of", "json", path).Output()
	if err != nil {
		t.Fatalf("probe encoded MP4: %v", err)
	}
	var probe struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(output, &probe); err != nil {
		t.Fatalf("decode ffprobe output: %v", err)
	}
	if len(probe.Streams) != 1 || probe.Streams[0].CodecType != "video" || probe.Streams[0].CodecName != "h264" {
		t.Fatalf("encoded streams = %+v, want one H.264 video stream", probe.Streams)
	}
}

func TestFFmpegRecordingEncoderHoldsFramesAcrossCaptureGaps(t *testing.T) {
	writer := &recordingTestWriteCloser{}
	encoder := &ffmpegRecordingEncoder{stdin: writer}
	first := image.NewRGBA(image.Rect(0, 0, 2, 2))
	second := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for index := 0; index < len(first.Pix); index += 4 {
		first.Pix[index], first.Pix[index+3] = 255, 255
		second.Pix[index+2], second.Pix[index+3] = 255, 255
	}
	if err := encoder.WriteFrame(recordingFrame{image: first, index: 0}); err != nil {
		t.Fatalf("write first frame: %v", err)
	}
	if err := encoder.WriteFrame(recordingFrame{image: second, index: 3}); err != nil {
		t.Fatalf("write gapped frame: %v", err)
	}
	firstYUV, err := bgraToI420(first)
	if err != nil {
		t.Fatalf("convert first frame: %v", err)
	}
	secondYUV, err := bgraToI420(second)
	if err != nil {
		t.Fatalf("convert second frame: %v", err)
	}
	frameBytes := len(firstYUV)
	if got, want := writer.Len(), frameBytes*4; got != want {
		t.Fatalf("encoded bytes = %d, want %d held frames", got, want)
	}
	if !bytes.Equal(writer.Bytes()[frameBytes*2:frameBytes*3], firstYUV) {
		t.Fatal("capture gap should hold the previous frame")
	}
	if !bytes.Equal(writer.Bytes()[frameBytes*3:], secondYUV) {
		t.Fatal("latest capture should be written at its timeline position")
	}
}

func TestCropRecordingFrameCopiesOnlySelection(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 20, 10))
	source.SetRGBA(12, 4, color.RGBA{R: 255, A: 255})
	cropped, err := cropRecordingFrame(source, image.Rect(10, 2, 16, 8))
	if err != nil {
		t.Fatalf("crop recording frame: %v", err)
	}
	if cropped.Bounds() != image.Rect(0, 0, 6, 6) {
		t.Fatalf("cropped bounds = %v, want 6x6 at origin", cropped.Bounds())
	}
	if got := cropped.RGBAAt(2, 2); got != (color.RGBA{R: 255, A: 255}) {
		t.Fatalf("cropped pixel = %+v, want the selected source pixel", got)
	}
}

func TestCropRecordingFrameReusesAlreadyCroppedRGBA(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 8, 6))
	cropped, err := cropRecordingFrame(source, image.Rect(40, 20, 48, 26))
	if err != nil {
		t.Fatalf("reuse cropped frame: %v", err)
	}
	if cropped != source {
		t.Fatal("already-cropped RGBA should be reused without another desktop-sized copy")
	}
}

func TestApplyRecordingOverlaysDrawsCursorInCropSpace(t *testing.T) {
	frame := image.NewRGBA(image.Rect(0, 0, 80, 80))
	pointer := Point{X: 28, Y: 26}
	if err := applyRecordingOverlays(frame, image.Rect(20, 20, 100, 100), &pointer, nil, true, false, time.Now(), 1); err != nil {
		t.Fatalf("apply recording overlays: %v", err)
	}
	found := false
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			if frame.RGBAAt(x, y).A > 0 {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("cursor should be drawn relative to the cropped origin")
	}
}

func TestOverlayRecordingCursorUsesDPIScale(t *testing.T) {
	small := image.NewRGBA(image.Rect(0, 0, 120, 120))
	large := image.NewRGBA(image.Rect(0, 0, 120, 120))
	if err := overlayRecordingCursor(small, Point{X: 8, Y: 8}, 1); err != nil {
		t.Fatalf("draw 1x cursor: %v", err)
	}
	if err := overlayRecordingCursor(large, Point{X: 8, Y: 8}, 2.5); err != nil {
		t.Fatalf("draw 2.5x cursor: %v", err)
	}
	smallCount, largeCount := 0, 0
	for y := 0; y < 120; y++ {
		for x := 0; x < 120; x++ {
			if small.Pix[small.PixOffset(x, y)+3] > 0 {
				smallCount++
			}
			if large.Pix[large.PixOffset(x, y)+3] > 0 {
				largeCount++
			}
		}
	}
	if largeCount <= smallCount {
		t.Fatalf("scaled cursor coverage = %d, unscaled = %d, want a larger pointer at 250%% DPI", largeCount, smallCount)
	}
}

func TestDrawRecordingCountdownUsesRedFillAndWhiteOutline(t *testing.T) {
	displayList := &DisplayList{}
	drawRecordingCountdown(displayList, Rect{Width: 800, Height: 600}, 3, 1)
	if displayList.CommandCount() < 9 {
		t.Fatalf("countdown commands = %d, want a white outline plus red fill", displayList.CommandCount())
	}
	size := recordingCountdownFontSize(Rect{Width: 800, Height: 600}, 1)
	if size < recordingCountdownLogicalSize {
		t.Fatalf("countdown size = %v, want at least %v", size, recordingCountdownLogicalSize)
	}
	scaled := recordingCountdownFontSize(Rect{Width: 3000, Height: 2000}, 2.5)
	if scaled <= size {
		t.Fatalf("countdown size at 250%% DPI = %v, unscaled = %v, want a larger digit", scaled, size)
	}
	if recordingCountdownFill.R < 200 || recordingCountdownFill.G > 80 || recordingCountdownFill.B > 80 {
		t.Fatalf("countdown fill = %+v, want red", recordingCountdownFill)
	}
	if recordingCountdownStroke != (Color{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatalf("countdown stroke = %+v, want white", recordingCountdownStroke)
	}
}

func TestRecordingPreviewPixelSizeCapsLongEdgeAndStaysEven(t *testing.T) {
	width, height := recordingPreviewPixelSize(3024, 2072)
	if width > recordingPreviewMaxEdge || height > recordingPreviewMaxEdge {
		t.Fatalf("preview size = %dx%d, exceeds %d", width, height, recordingPreviewMaxEdge)
	}
	if width%2 != 0 || height%2 != 0 {
		t.Fatalf("preview size = %dx%d, want even dimensions", width, height)
	}
	if width < 2 || height < 2 {
		t.Fatalf("preview size = %dx%d", width, height)
	}
}

func TestRecordingSessionFinishLeavesTempPathForPreview(t *testing.T) {
	encoder := &recordingTestEncoder{}
	session, err := newRecordingSession(recordingSessionConfig{
		FPS: 30, PixelBounds: image.Rect(0, 0, 4, 2), Encoder: encoder, TempRoot: t.TempDir(),
		Capture: func() (image.Image, error) { return image.NewRGBA(image.Rect(0, 0, 4, 2)), nil },
		Sleep:   func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatalf("create recording session: %v", err)
	}
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("start recording: %v", err)
	}
	waitForRecordingState(t, session, recordingStateRecording)
	path, err := session.Finish()
	if err != nil {
		t.Fatalf("finish recording: %v", err)
	}
	if path == "" || session.TempPath() != path {
		t.Fatalf("temp path = %q session = %q", path, session.TempPath())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("finalized recording missing: %v", err)
	}
	if session.currentState() != recordingStateSave {
		t.Fatalf("state = %s, want save", session.currentState())
	}
}

func TestExtractRecordingPreviewFrameFromEncodedMP4(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is unavailable")
	}
	path := filepath.Join(t.TempDir(), "preview-source.mp4")
	encoder := &ffmpegRecordingEncoder{}
	if err := encoder.Start(path, 64, 32, 30); err != nil {
		t.Fatalf("start encoder: %v", err)
	}
	frame := image.NewRGBA(image.Rect(0, 0, 64, 32))
	for pixel := 0; pixel < len(frame.Pix); pixel += 4 {
		frame.Pix[pixel], frame.Pix[pixel+1], frame.Pix[pixel+2], frame.Pix[pixel+3] = 40, 80, 200, 255
	}
	for index := int64(0); index < 6; index++ {
		if err := encoder.WriteFrame(recordingFrame{image: frame, index: index}); err != nil {
			t.Fatalf("encode frame: %v", err)
		}
	}
	if err := encoder.Finalize(); err != nil {
		t.Fatalf("finalize encoder: %v", err)
	}
	preview, err := extractRecordingPreviewFrame(path, 32, 16)
	if err != nil {
		t.Fatalf("extract preview frame: %v", err)
	}
	if preview.Bounds().Dx() != 32 || preview.Bounds().Dy() != 16 {
		t.Fatalf("preview bounds = %v, want 32x16", preview.Bounds())
	}
}
