// Command ghosttybuild stages the signed upstream libghostty-vt for native Wox builds.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/crypto/blake2b"
)

const (
	version    = "1.0.0"
	zigVersion = "0.17.0"
	releaseURL = "https://release.files.ghostty.org/libghostty-vt/" + version + "/"
	ghosttyKey = "RWQlAjJC23149WL2sEpT/l0QKy7hMIFhYdQOFy0Z7z7PbneUgvlsnYcV"
)

var zigArchives = map[string]string{
	"x86_64-windows": "b5663f69581dcf391293fbf16c06cb80d81d806545ce618b4d0bab7f0eb8c428",
	"x86_64-linux":   "1cbe9df9f27e6b78d14ccbca43b6703a404ef79ef1c463de901d7f088d4e2026",
	"x86_64-macos":   "4f9a1c5269aa17ebda5e6d3c2b89d6cbf36f7d2b22a0306e9ab98f25f95529c6",
	"aarch64-macos":  "b607e9b9234790a008116ae5bdb71c6243b84b9fb42a53a9e70fde41c06c536a",
}

func main() {
	noticesPath := flag.Bool("notices-path", false, "Print the native stage's verified notices path")
	flag.Parse()
	if *noticesPath {
		fmt.Println(filepath.Join(stagePath(), "ghostty-notices.txt"))
		return
	}
	if err := build(); err != nil {
		fmt.Fprintln(os.Stderr, "ghosttybuild:", err)
		os.Exit(1)
	}
}

func stagePath() string {
	return filepath.Join("plugin/system/shell/interactive/.build", "ghostty-"+version+"-"+runtime.GOOS+"-"+runtime.GOARCH)
}

// build verifies archives before extracting them and publishes only a complete native stage.
func build() error {
	arch := "x86_64"
	if runtime.GOARCH == "arm64" {
		arch = "aarch64"
	}
	osName := runtime.GOOS
	if osName == "darwin" {
		osName = "macos"
	}
	platform := arch + "-" + osName
	digest, supported := zigArchives[platform]
	if !supported || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		return fmt.Errorf("unsupported native build target %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	target, err := filepath.Abs(stagePath())
	if err != nil {
		return err
	}
	parent := filepath.Dir(target)
	if staged(target) {
		return nil
	}
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	// Zig's local cache needs atomic directory renames. Use the host's temp filesystem
	// even when a checkout is on a shared drive or a WSL Windows mount.
	work, err := os.MkdirTemp("", "wox-ghostty-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	zigFolder := "zig-" + platform + "-" + zigVersion
	zigName, zigSuffix := "zig", ".tar.xz"
	archiveName, zigTarget := "libghostty-vt.a", platform+"-gnu"
	switch osName {
	case "macos":
		zigTarget = platform + ".13.0"
	case "windows":
		zigName, zigSuffix = "zig.exe", ".zip"
		archiveName = "ghostty-vt-static.lib"
	}
	client := &http.Client{Timeout: 4 * time.Minute}
	zigURL := "https://ziglang.org/download/" + zigVersion + "/" + zigFolder + zigSuffix
	if err := fetchArchive(client, work, zigURL, digest); err != nil {
		return err
	}
	sums, err := downloadSmall(client, releaseURL+"SHA256SUMS")
	if err != nil {
		return err
	}
	signature, err := downloadSmall(client, releaseURL+"SHA256SUMS.minisig")
	if err != nil {
		return err
	}
	if err := verifyChecksums(sums, signature); err != nil {
		return err
	}
	source := "libghostty-vt-" + version
	for _, name := range []string{source + ".tar.gz", source + "-deps.tar.gz"} {
		want := ""
		for _, line := range strings.Split(string(sums), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[1] == name {
				want = fields[0]
				break
			}
		}
		if err := fetchArchive(client, work, releaseURL+name, want); err != nil {
			return err
		}
	}
	stage, err := os.MkdirTemp(parent, "stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	cmd := exec.Command(filepath.Join(work, zigFolder, zigName), "build", "--prefix", stage,
		"--system", filepath.Join(work, source+"-deps"), "-Dtarget="+zigTarget, "-Dcpu=baseline", "-Doptimize=ReleaseFast")
	cmd.Dir = filepath.Join(work, source)
	cmd.Env = append(os.Environ(), "ZIG_GLOBAL_CACHE_DIR="+filepath.Join(work, "zig-cache"))
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build libghostty-vt: %w", err)
	}
	// A common archive name keeps the cgo flags independent of the build tools' output names.
	if err := os.Rename(filepath.Join(stage, "lib", archiveName), filepath.Join(stage, "woxghostty.a")); err != nil {
		return err
	}
	if err := writeNotices(work, source, stage); err != nil {
		return err
	}
	if err := os.Rename(stage, target); err != nil && !staged(target) {
		return err
	}
	return nil
}

func staged(path string) bool {
	_, err := os.Stat(filepath.Join(path, "woxghostty.a"))
	return err == nil
}

func downloadSmall(client *http.Client, url string) ([]byte, error) {
	response, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, response.Status)
	}
	return io.ReadAll(io.LimitReader(response.Body, 64*1024))
}

// verifyChecksums verifies the prehashed Minisign packet and its signed comment using the upstream public key.
func verifyChecksums(message, signature []byte) error {
	lines := strings.Split(strings.TrimRight(string(signature), "\r\n"), "\n")
	if len(lines) != 4 {
		return fmt.Errorf("invalid Ghostty checksum signature")
	}
	key, _ := base64.StdEncoding.DecodeString(ghosttyKey)
	packet, _ := base64.StdEncoding.DecodeString(strings.TrimSuffix(lines[1], "\r"))
	comment, ok := strings.CutPrefix(strings.TrimSuffix(lines[2], "\r"), "trusted comment: ")
	global, _ := base64.StdEncoding.DecodeString(strings.TrimSuffix(lines[3], "\r"))
	if !ok || len(packet) != 74 || string(packet[:2]) != "ED" || !bytes.Equal(packet[2:10], key[2:10]) {
		return fmt.Errorf("invalid Ghostty checksum signature packet")
	}
	digest := blake2b.Sum512(message)
	public := ed25519.PublicKey(key[10:])
	if !ed25519.Verify(public, digest[:], packet[10:]) || !ed25519.Verify(public, append(packet[10:], comment...), global) {
		return fmt.Errorf("Ghostty checksum signature verification failed")
	}
	return nil
}

// fetchArchive checks the exact expected digest before invoking the host's archive tool.
func fetchArchive(client *http.Client, work, url, want string) error {
	if len(want) != 64 {
		return fmt.Errorf("missing SHA-256 for %s", url)
	}
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, response.Status)
	}
	path := filepath.Join(work, filepath.Base(url))
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(file, hash), response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != want {
		return fmt.Errorf("SHA-256 mismatch for %s", url)
	}
	cmd := exec.Command("tar", "-xf", path, "-C", work)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// writeNotices collects license files from the verified linked dependency manifest.
func writeNotices(work, source, stage string) error {
	sourcePath := filepath.Join(work, source)
	manifest, err := os.ReadFile(filepath.Join(sourcePath, "deps", "libghostty-vt-third-party.json"))
	if err != nil {
		return err
	}
	var dependencies struct {
		ThirdParty []struct{ Name, Hash, Path, Use string } `json:"third_party"`
	}
	if err := json.Unmarshal(manifest, &dependencies); err != nil {
		return err
	}
	var notices bytes.Buffer
	appendNotices := func(name, path string) error {
		files, err := filepath.Glob(filepath.Join(path, "LICENSE*"))
		if err != nil || len(files) == 0 {
			return fmt.Errorf("missing %s license files at %s", name, path)
		}
		for _, file := range files {
			content, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			fmt.Fprintf(&notices, "%s — %s\n\n%s\n\n", name, filepath.Base(file), content)
		}
		return nil
	}
	if err := appendNotices("libghostty-vt "+version, sourcePath); err != nil {
		return err
	}
	for _, dep := range dependencies.ThirdParty {
		if dep.Use != "linked" {
			continue
		}
		path := filepath.Join(work, source+"-deps", dep.Hash)
		if dep.Path != "" {
			path = filepath.Join(sourcePath, dep.Path)
		}
		if err := appendNotices(dep.Name, path); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(stage, "ghostty-notices.txt"), notices.Bytes(), 0644)
}
