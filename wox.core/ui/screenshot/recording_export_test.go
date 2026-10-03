package screenshot

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/gif"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	woxwidget "wox/ui/widget"
)

// TestRecordingExportFormats verifies real containers, animation frames, and source cleanup after saving.
func TestRecordingExportFormats(t *testing.T) {
	ffmpeg, err := recordingFFmpegPath()
	if err != nil {
		t.Skip(err)
	}
	source := filepath.Join(t.TempDir(), "source.mp4")
	encoder := &ffmpegRecordingEncoder{}
	if err := encoder.Start(source, 64, 32, 30); err != nil {
		t.Fatal(err)
	}
	frame := image.NewRGBA(image.Rect(0, 0, 64, 32))
	for index := range 12 {
		for pixel := 0; pixel < len(frame.Pix); pixel += 4 {
			frame.Pix[pixel], frame.Pix[pixel+1], frame.Pix[pixel+2], frame.Pix[pixel+3] = byte(index*20), 60, 180, 255
		}
		if err := encoder.WriteFrame(recordingFrame{image: frame, index: int64(index)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := encoder.Finalize(); err != nil {
		t.Fatal(err)
	}
	sourceData, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []recordingExportFormat{recordingExportMP4, recordingExportGIF, recordingExportWebP} {
		t.Run(format.extension(), func(t *testing.T) {
			if format == recordingExportWebP {
				encoders, err := exec.Command(ffmpeg, "-hide_banner", "-encoders").Output()
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Contains(encoders, []byte("libwebp_anim")) {
					if _, err := recordingWebPEncoderPath(ffmpeg); err != nil {
						t.Skip("neither FFmpeg's WebP encoder nor img2webp is installed")
					}
				}
			}
			dir := t.TempDir()
			temp := filepath.Join(dir, "take.mp4")
			if err := os.WriteFile(temp, sourceData, 0o600); err != nil {
				t.Fatal(err)
			}
			session := &recordingSession{state: recordingStateSave, tempPath: temp}
			target := filepath.Join(dir, "saved."+format.extension())
			if err := session.Save(context.Background(), target, format); err != nil {
				t.Fatal(err)
			}
			if session.currentState() != recordingStateClosed || session.TempPath() != "" {
				t.Fatal("successful export did not close the take")
			}
			if _, err := os.Stat(temp); !os.IsNotExist(err) {
				t.Fatalf("source was not cleaned up: %v", err)
			}
			data, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			switch format {
			case recordingExportMP4:
				if !bytes.Equal(data, sourceData) {
					t.Fatal("MP4 export should preserve the original encoding")
				}
			case recordingExportGIF:
				animation, err := gif.DecodeAll(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				if len(animation.Image) < 2 || animation.LoopCount != 0 || animation.Config.Width != 64 || animation.Config.Height != 32 {
					t.Fatal("GIF did not preserve looping animation and dimensions")
				}
				duration := 0
				for _, delay := range animation.Delay {
					duration += delay
				}
				if duration < 35 || duration > 45 {
					t.Fatalf("GIF duration = %d centiseconds, want about 40", duration)
				}
			case recordingExportWebP:
				if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
					t.Fatal("export is not a WebP container")
				}
				frames, duration, loop := 0, 0, -1
				for offset := 12; offset+8 <= len(data); {
					size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
					if offset+8+size > len(data) {
						t.Fatal("truncated WebP chunk")
					}
					chunk := data[offset+8 : offset+8+size]
					switch string(data[offset : offset+4]) {
					case "ANIM":
						if len(chunk) >= 6 {
							loop = int(binary.LittleEndian.Uint16(chunk[4:6]))
						}
					case "ANMF":
						if len(chunk) >= 16 {
							frames++
							duration += int(chunk[12]) | int(chunk[13])<<8 | int(chunk[14])<<16
						}
					}
					offset += 8 + size + size%2
				}
				if frames < 2 || loop != 0 || duration < 350 || duration > 450 {
					t.Fatalf("invalid WebP animation: frames=%d loop=%d duration=%dms", frames, loop, duration)
				}
			}
		})
	}
}

// TestRecordingExportFailurePreservesTakeAndDestination covers retry without losing either file.
func TestRecordingExportFailurePreservesTakeAndDestination(t *testing.T) {
	dir := t.TempDir()
	source, target := filepath.Join(dir, "invalid.mp4"), filepath.Join(dir, "existing.gif")
	for _, path := range []string{source, target} {
		if err := os.WriteFile(path, []byte("keep me"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	session := &recordingSession{state: recordingStateSave, tempPath: source}
	if err := session.Save(context.Background(), target, recordingExportGIF); err == nil {
		t.Fatal("invalid recording unexpectedly exported")
	}
	for _, path := range []string{source, target} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "keep me" {
			t.Fatalf("export failure changed %s: %q, %v", path, data, err)
		}
	}
	if session.currentState() != recordingStateSave || session.TempPath() != source {
		t.Fatal("failed export cannot be retried")
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, ".wox-recording-*")); len(leftovers) != 0 {
		t.Fatalf("partial exports left behind: %v", leftovers)
	}
	if err := session.Save(context.Background(), filepath.Join(dir, "retry.mp4"), recordingExportMP4); err != nil {
		t.Fatalf("retry with original format failed: %v", err)
	}
}

// TestRecordingFormatMenuFitsCapture checks fractional scaling, edge placement, and desktop-origin independence.
func TestRecordingFormatMenuFitsCapture(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2, 2.5} {
		frame := Size{Width: 1200 * scale, Height: 800 * scale}
		for _, origin := range []Point{{}, {X: -1920, Y: -1080}, {X: 2560, Y: 240}} {
			for _, toolbar := range []Rect{
				{X: 10 * scale, Y: 10 * scale, Width: recordingToolbarWidth * scale, Height: recordingToolbarHeight * scale},
				{X: 810 * scale, Y: 730 * scale, Width: recordingToolbarWidth * scale, Height: recordingToolbarHeight * scale},
			} {
				bounds := recordingFormatMenuBounds(frame, toolbar, Rect{X: 180 * scale}, scale)
				desktop := Rect{X: bounds.X + origin.X, Y: bounds.Y + origin.Y, Width: bounds.Width, Height: bounds.Height}
				if desktop.X < origin.X || desktop.Y < origin.Y || desktop.X+desktop.Width > origin.X+frame.Width || desktop.Y+desktop.Height > origin.Y+frame.Height {
					t.Fatalf("popup outside capture display: %+v at scale %v", desktop, scale)
				}
				if bounds.Width != 280*scale || bounds.Height != 112*scale {
					t.Fatalf("popup chrome did not scale: %+v", bounds)
				}
			}
		}
	}
}

// TestRecordingFormatMenuCommit checks the default, Escape dismissal, and export-time locking.
func TestRecordingFormatMenuCommit(t *testing.T) {
	state := &recordingToolbarState{session: &recordingSession{state: recordingStateSave}}
	if state.exportFormat.extension() != "mp4" {
		t.Fatal("new recordings must default to MP4")
	}
	menu := &recordingFormatMenu{owner: state, selected: recordingExportGIF}
	menu.host = woxwidget.NewHost(menu.build)
	state.formatMenu = menu
	menu.key(KeyEvent{Down: true, Key: KeyEscape})
	if state.exportFormat != recordingExportMP4 || state.formatMenu != nil {
		t.Fatal("Escape changed the format or left the popup open")
	}
	menu.choose(recordingExportWebP)
	if state.exportFormat != recordingExportWebP {
		t.Fatal("WebP selection was not committed")
	}
	state.finishing = true
	menu.choose(recordingExportGIF)
	if state.exportFormat != recordingExportWebP {
		t.Fatal("format changed during export")
	}
}

// TestRecordingExportCancelledPreservesTake covers Escape while a conversion is pending.
func TestRecordingExportCancelledPreservesTake(t *testing.T) {
	dir := t.TempDir()
	source, target := filepath.Join(dir, "take.mp4"), filepath.Join(dir, "cancelled.webp")
	if err := os.WriteFile(source, []byte("original recording"), 0o600); err != nil {
		t.Fatal(err)
	}
	session := &recordingSession{state: recordingStateSave, tempPath: source}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := session.Save(ctx, target, recordingExportWebP); err != context.Canceled {
		t.Fatalf("cancelled export error = %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("cancelled export published a destination")
	}
	if _, err := os.Stat(source); err != nil || session.currentState() != recordingStateSave {
		t.Fatal("cancelled export lost its retryable source")
	}
}

// TestRecordingFormatTriggerOnlyAppearsInPreview keeps capture options intact before the take finishes.
func TestRecordingFormatTriggerOnlyAppearsInPreview(t *testing.T) {
	for _, status := range []recordingState{recordingStateReady, recordingStateRecording, recordingStatePaused, recordingStateFinalizing, recordingStateSave} {
		state := &recordingToolbarState{session: &recordingSession{state: status, config: recordingSessionConfig{Now: time.Now}}}
		state.drawToolbar(&DisplayList{}, FrameInfo{Size: Size{Width: recordingToolbarWidth, Height: recordingToolbarHeight}})
		if visible := state.formatRect.Width > 0; visible != (status == recordingStateSave) {
			t.Fatalf("format trigger visibility = %t in %s", visible, status)
		}
	}
}

// TestRecordingFormatMenuKeyboardAndLayout exercises the retained menu controls at fractional DPI.
func TestRecordingFormatMenuKeyboardAndLayout(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2.5} {
		state := &recordingToolbarState{
			session: &recordingSession{state: recordingStateSave},
			options: ScreenshotOptions{RecordingTooltips: RecordingTooltips{FormatMP4: "MP4 · 视频（默认）", FormatGIF: "GIF · 动图", FormatWebP: "WebP · 动图"}},
		}
		menu := &recordingFormatMenu{owner: state, scale: scale, size: Size{Width: 280 * scale, Height: 112 * scale}}
		menu.host = woxwidget.NewHost(menu.build)
		menu.host.AttachServices(&screenshotTestSurface{})
		state.formatMenu = menu
		menu.host.Frame(&DisplayList{}, FrameInfo{Size: menu.size, Scale: 2})
		rows := 0
		for _, node := range menu.host.Snapshot().Tree.Nodes {
			if !strings.HasPrefix(node.AutomationID, "recording.format.") {
				continue
			}
			rows++
			if node.Bounds.Height != 32*scale || node.Bounds.Y+node.Bounds.Height > menu.size.Height || node.Bounds.X+node.Bounds.Width > menu.size.Width {
				t.Fatalf("menu row clipped at scale %v: %+v", scale, node.Bounds)
			}
		}
		if rows != 3 {
			t.Fatalf("menu has %d accessible choices, want 3", rows)
		}
		menu.key(KeyEvent{Down: true, Key: KeyArrowDown})
		menu.key(KeyEvent{Down: true, Key: KeyEnter})
		if state.exportFormat != recordingExportGIF || state.formatMenu != nil {
			t.Fatal("Down + Enter did not choose GIF and dismiss the menu")
		}
		menu.host.Dispose()
	}
}
