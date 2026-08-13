package screenshot

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"os/exec"
	"strconv"
)

const (
	recordingCountdownLogicalSize = float32(160)
	recordingPreviewMaxEdge       = 1280
)

var recordingCountdownFill = Color{R: 255, G: 59, B: 48, A: 255}
var recordingCountdownStroke = Color{R: 255, G: 255, B: 255, A: 255}

// recordingCountdownFontSize scales the pre-roll digit with DPI while keeping it inside the selection.
func recordingCountdownFontSize(selection Rect, uiScale float32) float32 {
	scale := max(float32(1), uiScale)
	size := recordingCountdownLogicalSize * scale
	limit := min(selection.Width*0.55, selection.Height*0.62)
	if limit > 0 && size > limit {
		return max(48*scale, limit)
	}
	return size
}

// drawRecordingCountdown paints a large red digit with a white outline in the selection center.
func drawRecordingCountdown(displayList *DisplayList, selection Rect, seconds int, uiScale float32) {
	if displayList == nil || seconds < 1 {
		return
	}
	label := strconv.Itoa(seconds)
	size := recordingCountdownFontSize(selection, uiScale)
	strokeWidth := max(float32(4), size*0.05)
	style := TextStyle{Size: size, Weight: FontWeightSemibold}
	width := screenshotEditorEstimatedTextWidth(label, size) + strokeWidth*2
	height := size + strokeWidth*2
	rect := Rect{
		X: selection.X + (selection.Width-width)/2, Y: selection.Y + (selection.Height-height)/2,
		Width: width, Height: height,
	}
	drawRecordingOutlinedText(displayList, label, rect, style, recordingCountdownFill, recordingCountdownStroke, strokeWidth)
}

// drawRecordingOutlinedText draws a solid halo first so the fill stays readable on any desktop.
func drawRecordingOutlinedText(displayList *DisplayList, text string, rect Rect, style TextStyle, fill, stroke Color, strokeWidth float32) {
	for _, offset := range recordingOutlineOffsets(strokeWidth) {
		displayList.DrawText(text, Rect{X: rect.X + offset.X, Y: rect.Y + offset.Y, Width: rect.Width, Height: rect.Height}, style, stroke)
	}
	displayList.DrawText(text, rect, style, fill)
}

func recordingOutlineOffsets(strokeWidth float32) []Point {
	steps := max(1, int(math.Round(float64(strokeWidth))))
	offsets := make([]Point, 0, 8*steps)
	for step := 1; step <= steps; step++ {
		delta := float32(step)
		offsets = append(offsets,
			Point{X: -delta, Y: -delta}, Point{X: 0, Y: -delta}, Point{X: delta, Y: -delta},
			Point{X: -delta, Y: 0}, Point{X: delta, Y: 0},
			Point{X: -delta, Y: delta}, Point{X: 0, Y: delta}, Point{X: delta, Y: delta},
		)
	}
	return offsets
}

// recordingPreviewPixelSize keeps in-overlay playback within a decode budget while preserving aspect.
func recordingPreviewPixelSize(width, height float32) (int, int) {
	w := max(2, int(math.Round(float64(width))))
	h := max(2, int(math.Round(float64(height))))
	if w > recordingPreviewMaxEdge || h > recordingPreviewMaxEdge {
		if w >= h {
			h = max(2, h*recordingPreviewMaxEdge/w)
			w = recordingPreviewMaxEdge
		} else {
			w = max(2, w*recordingPreviewMaxEdge/h)
			h = recordingPreviewMaxEdge
		}
	}
	if w%2 != 0 {
		w--
	}
	if h%2 != 0 {
		h--
	}
	return max(2, w), max(2, h)
}

func recordingPreviewFrameBytes(width, height int) int {
	return width * height * 4
}

// extractRecordingPreviewFrame decodes the first MP4 frame into packed RGBA at the preview size.
func extractRecordingPreviewFrame(path string, width, height int) (*image.RGBA, error) {
	if path == "" || width < 2 || height < 2 {
		return nil, errors.New("recording preview frame is unavailable")
	}
	ffmpegPath, err := recordingFFmpegPath()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(ffmpegPath,
		"-hide_banner", "-loglevel", "error", "-i", path, "-an",
		"-vf", "scale="+strconv.Itoa(width)+":"+strconv.Itoa(height),
		"-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "rgba", "pipe:1",
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("decode recording poster: %w", err)
	}
	return recordingRGBAFrame(output, width, height)
}

// startRecordingPreviewDecode opens a real-time RGBA stream for in-overlay playback.
func startRecordingPreviewDecode(ctx context.Context, path string, width, height int) (*exec.Cmd, io.ReadCloser, error) {
	if path == "" || width < 2 || height < 2 {
		return nil, nil, errors.New("recording preview stream is unavailable")
	}
	ffmpegPath, err := recordingFFmpegPath()
	if err != nil {
		return nil, nil, err
	}
	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-hide_banner", "-loglevel", "error", "-re", "-i", path, "-an",
		"-vf", "scale="+strconv.Itoa(width)+":"+strconv.Itoa(height),
		"-f", "rawvideo", "-pix_fmt", "rgba", "pipe:1",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start recording preview: %w", err)
	}
	return cmd, stdout, nil
}

func recordingRGBAFrame(pixels []byte, width, height int) (*image.RGBA, error) {
	needed := recordingPreviewFrameBytes(width, height)
	if len(pixels) < needed {
		return nil, fmt.Errorf("recording preview frame is truncated: %d < %d", len(pixels), needed)
	}
	frame := image.NewRGBA(image.Rect(0, 0, width, height))
	copy(frame.Pix, pixels[:needed])
	return frame, nil
}

// pumpRecordingPreview decodes the MP4 in real time and delivers packed RGBA frames until the context ends.
func pumpRecordingPreview(ctx context.Context, path string, width, height int, onFrame func(*image.RGBA) error) error {
	cmd, stdout, err := startRecordingPreviewDecode(ctx, path, width, height)
	if err != nil {
		return err
	}
	defer func() {
		_ = stdout.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()
	buffer := make([]byte, recordingPreviewFrameBytes(width, height))
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := io.ReadFull(stdout, buffer); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return err
		}
		frame, err := recordingRGBAFrame(buffer, width, height)
		if err != nil {
			return err
		}
		if err := onFrame(frame); err != nil {
			return err
		}
	}
}
