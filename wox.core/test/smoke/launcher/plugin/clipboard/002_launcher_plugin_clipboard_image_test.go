//go:build wox_ui_smoke

package clipboard

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	woxclipboard "wox/util/clipboard"
)

// Test002LauncherPluginClipboardImage verifies image capture and restoration through the Clipboard result action.
// Flow: inject an external PNG -> query Clipboard -> select the image result -> activate Copy.
// Evidence: the Clipboard plugin exposes the image dimensions and the system clipboard receives the same image.
func Test002LauncherPluginClipboardImage(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.PreserveClipboard(t)
		smoke.ShowLauncher(t, ctx, client)

		imagePath, width, height, expectedColor := createClipboardSmokeImage(t)
		if err := writeExternalImageClipboard(imagePath); err != nil {
			t.Fatalf("write external image clipboard: %v", err)
		}

		labelPrefix := fmt.Sprintf("Image (%d×%d)", width, height)
		if _, resultID := waitForClipboardImageResult(t, ctx, client, width, height); resultID == "" {
			t.Fatal("Clipboard image result was not found")
		}
		smoke.SelectLauncherResultLabelPrefix(t, ctx, client, labelPrefix)

		snapshot := smoke.OpenResultActionPanel(t, ctx, client)
		copyAction, found := automationdriver.FindByAutomationIDPrefix(snapshot, "action-result-")
		if !found {
			t.Fatal("Clipboard image Copy action was not exposed")
		}
		if err := client.Perform(ctx, copyAction.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("activate Clipboard image Copy action: %v", err)
		}
		waitForClipboardImage(t, ctx, expectedColor, width, height)

		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read Clipboard image smoke semantics: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// waitForClipboardImage polls the OS clipboard because clipboard ownership changes do not publish UI semantics.
func waitForClipboardImage(t *testing.T, ctx context.Context, expectedColor color.RGBA, width, height int) {
	t.Helper()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if clipboardImageMatches(expectedColor, width, height) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for Clipboard image Copy result: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

// createClipboardSmokeImage writes a uniquely sized solid-color PNG for the external clipboard boundary.
func createClipboardSmokeImage(t *testing.T) (string, int, int, color.RGBA) {
	t.Helper()
	stamp := time.Now().UnixNano()
	width := 48 + int(stamp%31)
	height := 53 + int((stamp/31)%31)
	expectedColor := color.RGBA{R: uint8(stamp), G: uint8(stamp >> 8), B: uint8(stamp >> 16), A: 255}
	imageData := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			imageData.SetRGBA(x, y, expectedColor)
		}
	}

	path := filepath.Join(t.TempDir(), "clipboard-smoke.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create Clipboard smoke image: %v", err)
	}
	if err := png.Encode(file, imageData); err != nil {
		_ = file.Close()
		t.Fatalf("encode Clipboard smoke image: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close Clipboard smoke image: %v", err)
	}
	return path, width, height, expectedColor
}

// waitForClipboardImageResult retries fresh query generations until the asynchronous image watcher has persisted the PNG.
func waitForClipboardImageResult(t *testing.T, ctx context.Context, client *automationdriver.Client, width, height int) (woxwidget.AutomationSnapshot, string) {
	t.Helper()
	queries := []string{"cb", "cb "}
	labelPrefix := fmt.Sprintf("Image (%d×%d)", width, height)
	for attempt := 0; ; attempt++ {
		query := queries[attempt%len(queries)]
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
			t.Fatalf("enter Clipboard image query %q: %v", query, err)
		}
		waitCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		snapshot, err := client.WaitFor(waitCtx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			if !found || input.Value != query {
				return false
			}
			results, complete := automationdriver.Find(snapshot, "launcher.results")
			if !complete || results.Value != "complete" {
				return false
			}
			for _, node := range snapshot.Tree.Nodes {
				if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.HasPrefix(node.Label, labelPrefix) {
					return true
				}
			}
			return false
		})
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				t.Fatalf("wait for Clipboard image query %q: %v", query, ctx.Err())
			}
			continue
		}
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.HasPrefix(node.Label, labelPrefix) {
				return snapshot, node.AutomationID
			}
		}
	}
}

// writeExternalImageClipboard changes the OS clipboard outside Wox so its watcher treats the image as user-originated.
func writeExternalImageClipboard(imagePath string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-Command", `$path = [Console]::In.ReadToEnd().Trim(); Add-Type -AssemblyName System.Windows.Forms; $files = New-Object System.Collections.Specialized.StringCollection; [void]$files.Add($path); [System.Windows.Forms.Clipboard]::SetFileDropList($files)`)
	case "darwin":
		command = exec.Command("osascript", "-e", `on run argv
	set the clipboard to (read (POSIX file (item 1 of argv)) as «class PNGf»)
end run`, imagePath)
	case "linux":
		candidates := []struct {
			name string
			args []string
		}{
			{name: "wl-copy", args: []string{"--type", "image/png"}},
			{name: "xclip", args: []string{"-selection", "clipboard", "-t", "image/png", "-i"}},
		}
		for _, candidate := range candidates {
			path, err := exec.LookPath(candidate.name)
			if err == nil {
				command = exec.Command(path, candidate.args...)
				break
			}
		}
		if command == nil {
			return fmt.Errorf("no supported Linux image clipboard command found")
		}
	default:
		return fmt.Errorf("unsupported OS %q", runtime.GOOS)
	}

	if runtime.GOOS == "windows" {
		command.Stdin = strings.NewReader(imagePath)
	} else if runtime.GOOS == "linux" {
		data, err := os.ReadFile(imagePath)
		if err != nil {
			return err
		}
		command.Stdin = bytes.NewReader(data)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func clipboardImageMatches(expectedColor color.RGBA, width, height int) bool {
	data, err := woxclipboard.Read()
	if err != nil {
		return false
	}
	imageData, ok := data.(*woxclipboard.ImageData)
	if !ok || imageData.Image.Bounds().Dx() != width || imageData.Image.Bounds().Dy() != height {
		return false
	}
	got := color.NRGBAModel.Convert(imageData.Image.At(width/2, height/2)).(color.NRGBA)
	return got.R == expectedColor.R && got.G == expectedColor.G && got.B == expectedColor.B
}
