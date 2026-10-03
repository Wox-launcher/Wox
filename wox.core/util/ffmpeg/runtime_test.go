package ffmpeg

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"wox/util"
)

// TestResolveOrder ensures discovery never installs anything and honors platform-specific paths.
func TestResolveOrder(t *testing.T) {
	for _, goos := range []string{"darwin", "windows", "linux"} {
		root := t.TempDir()
		name := executableName(goos)
		candidates := []string{filepath.Join(root, "runtime", "ffmpeg", Version, goos+"-amd64", name), filepath.Join(root, "others", "recording", goos+"-amd64", name), name}
		for found := range candidates {
			var calls []string
			path, err := resolve(root, goos, "amd64", func(path string) (string, error) {
				calls = append(calls, path)
				if path == candidates[found] {
					return path, nil
				}
				return "", os.ErrNotExist
			})
			if err != nil || path != candidates[found] || len(calls) != found+1 {
				t.Fatalf("discovery: %q %v %v", path, err, calls)
			}
		}
	}
}

// runtimeFixture supplies deterministic compressed bytes and provenance without accessing the network.
func runtimeFixture(t *testing.T) (releaseAsset, downloadFile) {
	t.Helper()
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, _ = writer.Write([]byte("test executable"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	digest := func(data []byte) string { hash := sha256.Sum256(data); return hex.EncodeToString(hash[:]) }
	asset := releaseAsset{name: "test", size: int64(compressed.Len()), sha256: digest(compressed.Bytes()), readmeSHA256: digest([]byte("source")), licenseSHA256: digest([]byte("license"))}
	download := func(ctx context.Context, url, path string, progress func(int64, int64)) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		data := compressed.Bytes()
		if strings.HasSuffix(url, ".README") {
			data = []byte("source")
		}
		if strings.HasSuffix(url, ".LICENSE") {
			data = []byte("license")
		}
		if progress != nil {
			progress(int64(len(data)), -1)
		}
		return os.WriteFile(path, data, 0o600)
	}
	return asset, download
}

// TestInstallAtomicPublication verifies successful publication and all failure paths keep the destination absent.
func TestInstallAtomicPublication(t *testing.T) {
	for _, failure := range []string{"", "checksum", "license", "validation", "cancel", "download"} {
		t.Run(failure, func(t *testing.T) {
			asset, download := runtimeFixture(t)
			destination := filepath.Join(t.TempDir(), "installed")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if failure == "checksum" {
				asset.sha256 = strings.Repeat("0", 64)
			}
			if failure == "license" {
				asset.licenseSHA256 = strings.Repeat("0", 64)
			}
			originalDownload := download
			download = func(ctx context.Context, url, path string, progress func(int64, int64)) error {
				if failure == "download" {
					return errors.New("offline")
				}
				if failure == "cancel" {
					cancel()
				}
				return originalDownload(ctx, url, path, progress)
			}
			validated := false
			validate := func(ctx context.Context, path string) error {
				validated = true
				if _, err := os.Stat(destination); !os.IsNotExist(err) {
					t.Fatal("published before validation")
				}
				data, err := os.ReadFile(path)
				if err != nil || string(data) != "test executable" {
					t.Fatalf("unpacked binary: %q %v", data, err)
				}
				if failure == "validation" {
					return errors.New("missing encoder")
				}
				return nil
			}
			var progress []Progress
			path, err := install(ctx, destination, "ffmpeg", asset, download, validate, func(p Progress) { progress = append(progress, p) })
			staging, _ := filepath.Glob(filepath.Join(filepath.Dir(destination), ".ffmpeg-install-*"))
			if len(staging) != 0 {
				t.Fatal("staging files leaked")
			}
			if failure != "" {
				if err == nil {
					t.Fatal("expected installation failure")
				}
				if _, err := os.Stat(destination); !os.IsNotExist(err) {
					t.Fatal("failed install published files")
				}
				if (failure == "checksum" || failure == "license") && validated {
					t.Fatal("unverified data was executed")
				}
				if failure == "cancel" && !errors.Is(err, context.Canceled) {
					t.Fatalf("lost cancellation: %v", err)
				}
				return
			}
			if err != nil || !validated {
				t.Fatalf("installation: %v", err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
				t.Fatal("binary is not executable")
			}
			for _, name := range []string{"README", "LICENSE"} {
				if _, err := os.Stat(filepath.Join(destination, name)); err != nil {
					t.Fatal(err)
				}
			}
			if len(progress) < 3 || progress[0].Percent != 0 || progress[len(progress)-1].Stage != StageInstalling {
				t.Fatalf("progress: %+v", progress)
			}
		})
	}
}

// TestReleaseManifest checks that every supported target has complete pinned provenance.
func TestReleaseManifest(t *testing.T) {
	for target, asset := range releaseAssets {
		if asset.size <= 0 || asset.name == "" {
			t.Fatalf("incomplete asset: %s", target)
		}
		for _, digest := range []string{asset.sha256, asset.licenseSHA256, asset.readmeSHA256} {
			decoded, err := hex.DecodeString(digest)
			if err != nil || len(decoded) != sha256.Size {
				t.Fatalf("invalid digest: %s", target)
			}
		}
	}
}

// TestPublishedRelease is opt-in because it downloads the real executable and executes its encoder probe.
func TestPublishedRelease(t *testing.T) {
	if os.Getenv("WOX_TEST_FFMPEG_DOWNLOAD") != "1" {
		t.Skip("set WOX_TEST_FFMPEG_DOWNLOAD=1 to verify the published runtime")
	}
	asset, ok := releaseAssets[runtime.GOOS+"-"+runtime.GOARCH]
	if !ok {
		t.Skip("unsupported automatic-install target")
	}
	path, err := install(context.Background(), filepath.Join(t.TempDir(), "runtime"), executableName(runtime.GOOS), asset, util.HttpDownloadWithProgress, validate, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
