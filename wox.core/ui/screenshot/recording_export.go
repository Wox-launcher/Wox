package screenshot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type recordingExportFormat uint8

const (
	recordingExportMP4 recordingExportFormat = iota
	recordingExportGIF
	recordingExportWebP
)

func (format recordingExportFormat) valid() bool {
	return format <= recordingExportWebP
}

// extension keeps the zero-value selection on the original MP4 export path.
func (format recordingExportFormat) extension() string {
	switch format {
	case recordingExportGIF:
		return "gif"
	case recordingExportWebP:
		return "webp"
	default:
		return "mp4"
	}
}

// recordingExportPath supplies the selected extension when the native dialog leaves it off.
func recordingExportPath(path string, format recordingExportFormat) string {
	if filepath.Ext(path) == "" {
		return path + "." + format.extension()
	}
	return path
}

// convertRecording encodes looping animations from the finalized preview without touching its MP4.
func convertRecording(ctx context.Context, source, target string, format recordingExportFormat) error {
	ffmpeg, err := recordingFFmpegPath()
	if err != nil {
		return err
	}
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-i", source, "-an"}
	switch format {
	case recordingExportGIF:
		// Per-frame palettes avoid buffering an entire long recording before palette generation completes.
		args = append(args, "-filter_complex", "fps=20,split[a][b];[a]palettegen=stats_mode=single[p];[b][p]paletteuse=new=1:dither=sierra2_4a", "-loop", "0", "-f", "gif")
	case recordingExportWebP:
		args = append(args, "-vf", "fps=30", "-c:v", "libwebp_anim", "-quality", "80", "-compression_level", "4", "-loop", "0", "-f", "webp")
	default:
		return fmt.Errorf("unsupported recording conversion format: %d", format)
	}
	output, err := exec.CommandContext(ctx, ffmpeg, append(args, target)...).CombinedOutput()
	if err != nil && format == recordingExportWebP && strings.Contains(string(output), "Unknown encoder") && ctx.Err() == nil {
		return convertRecordingWebPFallback(ctx, ffmpeg, source, target)
	}
	if err != nil {
		return fmt.Errorf("export %s recording: %w: %s", format.extension(), err, strings.TrimSpace(string(output)))
	}
	return nil
}

// recordingWebPEncoderPath accepts a packaged companion encoder before searching the development PATH.
func recordingWebPEncoderPath(ffmpeg string) (string, error) {
	executable := "img2webp"
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	encoder := filepath.Join(filepath.Dir(ffmpeg), executable)
	if _, err := os.Stat(encoder); err != nil {
		encoder, err = exec.LookPath(executable)
		if err != nil {
			return "", fmt.Errorf("WebP export requires an FFmpeg build with libwebp or img2webp: %w", err)
		}
	}
	return encoder, nil
}

// convertRecordingWebPFallback supports minimal FFmpeg builds using libwebp's standalone encoder.
// Frames are spooled to disk so decoded images do not accumulate in Wox's memory.
func convertRecordingWebPFallback(ctx context.Context, ffmpeg, source, target string) error {
	encoder, err := recordingWebPEncoderPath(ffmpeg)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp(filepath.Dir(target), ".wox-webp-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	output, err := exec.CommandContext(ctx, ffmpeg, "-nostdin", "-hide_banner", "-loglevel", "error", "-i", source,
		"-an", "-vf", "fps=30", filepath.Join(dir, "frame-%08d.png")).CombinedOutput()
	if err != nil {
		return fmt.Errorf("decode WebP export frames: %w: %s", err, strings.TrimSpace(string(output)))
	}
	frames, err := filepath.Glob(filepath.Join(dir, "frame-*.png"))
	if err != nil || len(frames) == 0 {
		return fmt.Errorf("WebP export has no decoded frames: %v", err)
	}
	// An argument file avoids command-line length limits; relative generated names also avoid path quoting.
	var arguments strings.Builder
	arguments.WriteString("-loop 0 -lossy -q 80 -m 4\n")
	for index, frame := range frames {
		duration := (index+1)*1000/30 - index*1000/30
		fmt.Fprintf(&arguments, "-d %d %s\n", duration, filepath.Base(frame))
	}
	arguments.WriteString("-o animation.webp\n")
	argumentPath := filepath.Join(dir, "arguments.txt")
	if err := os.WriteFile(argumentPath, []byte(arguments.String()), 0o600); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, encoder, argumentPath)
	cmd.Dir = dir
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("encode WebP recording: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return replaceRecordingFile(filepath.Join(dir, "animation.webp"), target)
}
