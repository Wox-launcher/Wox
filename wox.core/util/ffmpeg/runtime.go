package ffmpeg

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"wox/util"
)

// Progress distinguishes network transfer from verification and installation work.
type Progress struct {
	Percent int
	Stage   string
}

const (
	StageDownloading = "downloading"
	StageInstalling  = "installing"
)

var installGate = make(chan struct{}, 1)

func executableName(goos string) string {
	if goos == "windows" {
		return "ffmpeg.exe"
	}
	return "ffmpeg"
}

// Resolve performs no downloads: consent belongs to the caller before Install is invoked.
func Resolve() (string, error) {
	return resolve(util.GetLocation().GetWoxDataDirectory(), runtime.GOOS, runtime.GOARCH, exec.LookPath)
}

// resolve prefers the installed runtime, then bundled legacy files, then the user's PATH.
func resolve(dataRoot, goos, goarch string, lookPath func(string) (string, error)) (string, error) {
	name := executableName(goos)
	if dataRoot != "" {
		for _, candidate := range []string{
			filepath.Join(dataRoot, "runtime", "ffmpeg", Version, goos+"-"+goarch, name),
			filepath.Join(dataRoot, "others", "recording", goos+"-"+goarch, name),
		} {
			if path, err := lookPath(candidate); err == nil {
				return path, nil
			}
		}
	}
	return lookPath(name)
}

// DownloadSize reports the compressed executable size without contacting the server.
func DownloadSize() (int64, error) {
	asset, ok := releaseAssets[runtime.GOOS+"-"+runtime.GOARCH]
	if !ok {
		return 0, fmt.Errorf("automatic FFmpeg installation is unavailable for %s-%s", runtime.GOOS, runtime.GOARCH)
	}
	return asset.size, nil
}

// Install downloads a verified runtime into Wox's data directory after caller-owned user consent.
func Install(ctx context.Context, progress func(Progress)) (string, error) {
	select {
	case installGate <- struct{}{}:
		defer func() { <-installGate }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if path, err := Resolve(); err == nil {
		return path, nil
	}
	asset, ok := releaseAssets[runtime.GOOS+"-"+runtime.GOARCH]
	if !ok {
		return "", fmt.Errorf("unsupported FFmpeg platform: %s-%s", runtime.GOOS, runtime.GOARCH)
	}
	dataRoot := util.GetLocation().GetWoxDataDirectory()
	if dataRoot == "" {
		return "", errors.New("Wox data directory is unavailable")
	}
	destination := filepath.Join(dataRoot, "runtime", "ffmpeg", Version, runtime.GOOS+"-"+runtime.GOARCH)
	return install(ctx, destination, executableName(runtime.GOOS), asset, util.HttpDownloadWithProgress, validate, progress)
}

type downloadFile func(context.Context, string, string, func(int64, int64)) error

// install stages all files beside the destination so failures never publish a partial runtime.
func install(ctx context.Context, destination, name string, asset releaseAsset, download downloadFile, validateBinary func(context.Context, string) error, progress func(Progress)) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(filepath.Dir(destination), ".ffmpeg-install-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	report := func(stage string, percent int) {
		if progress != nil {
			progress(Progress{Stage: stage, Percent: percent})
		}
	}
	report(StageDownloading, 0)
	archive := filepath.Join(stage, "ffmpeg.gz")
	lastPercent := -1
	err = download(ctx, releaseURL+"/ffmpeg-"+asset.name+".gz", archive, func(current, total int64) {
		if total <= 0 {
			total = asset.size
		}
		percent := int(min(int64(100), current*100/max(int64(1), total)))
		if percent != lastPercent {
			lastPercent = percent
			report(StageDownloading, percent)
		}
	})
	if err != nil {
		return "", fmt.Errorf("download FFmpeg: %w", err)
	}
	report(StageInstalling, 100)
	if err := verifyFile(archive, asset.sha256); err != nil {
		return "", err
	}
	binaryPath := filepath.Join(stage, name)
	if err := unpackExecutable(archive, binaryPath); err != nil {
		return "", err
	}
	if err := os.Remove(archive); err != nil {
		return "", err
	}
	// Keep the build provenance and licenses next to the downloaded executable.
	for _, document := range []struct{ suffix, digest string }{{"LICENSE", asset.licenseSHA256}, {"README", asset.readmeSHA256}} {
		path := filepath.Join(stage, document.suffix)
		if err := download(ctx, releaseURL+"/"+asset.name+"."+document.suffix, path, nil); err != nil {
			return "", err
		}
		if err := verifyFile(path, document.digest); err != nil {
			return "", err
		}
	}
	if err := validateBinary(ctx, binaryPath); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.Rename(stage, destination); err != nil {
		return "", fmt.Errorf("install FFmpeg: %w", err)
	}
	util.GetLogger().Info(ctx, "installed recording runtime: FFmpeg "+Version)
	return filepath.Join(destination, name), nil
}

// verifyFile checks publisher digests before decompression or execution.
func verifyFile(path, want string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != want {
		return fmt.Errorf("FFmpeg checksum mismatch: %s", filepath.Base(path))
	}
	return nil
}

// unpackExecutable extracts one bounded gzip stream rather than trusting archive paths.
func unpackExecutable(archive, destination string) error {
	input, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer input.Close()
	reader, err := gzip.NewReader(input)
	if err != nil {
		return err
	}
	defer reader.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	const maxBinarySize = 512 << 20
	size, copyErr := io.Copy(output, io.LimitReader(reader, maxBinarySize+1))
	syncErr := output.Sync()
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if size == 0 || size > maxBinarySize {
		return errors.New("invalid FFmpeg executable size")
	}
	return errors.Join(syncErr, closeErr)
}

// validate rejects incompatible binaries before they become the preferred recording runtime.
func validate(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, "-hide_banner", "-encoders").CombinedOutput()
	if err != nil {
		return fmt.Errorf("validate FFmpeg runtime: %w: %s", err, strings.TrimSpace(string(output)))
	}
	for _, encoder := range []string{"libx264", "gif", "libwebp_anim"} {
		found := false
		for line := range strings.SplitSeq(string(output), "\n") {
			fields := strings.Fields(line)
			if len(fields) > 1 && fields[1] == encoder {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("FFmpeg runtime is missing encoder %s", encoder)
		}
	}
	return nil
}
