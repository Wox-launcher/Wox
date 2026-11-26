//go:build wox_ui_smoke

package query

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"wox/i18n"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/clipboard"
)

// Test049LauncherQueryPasteImage verifies selection results for bitmap data and copied image files.
func Test049LauncherQueryPasteImage(t *testing.T) {
	t.Run("bitmap", func(t *testing.T) {
		smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
			smoke.PreserveClipboard(t)
			smoke.ShowLauncher(t, ctx, client)

			img := image.NewNRGBA(image.Rect(0, 0, 96, 72))
			pixel := color.NRGBA{R: 62, G: 130, B: 210, A: 255}
			img.SetNRGBA(12, 24, pixel)
			if err := clipboard.Write(&clipboard.ImageData{Image: img}); err != nil {
				t.Fatalf("write bitmap to clipboard: %v", err)
			}

			path := pasteQueryImageAndCopyPath(t, ctx, client)
			file, err := os.Open(path)
			if err != nil {
				t.Fatalf("open pasted image: %v", err)
			}
			defer file.Close()
			pasted, err := png.Decode(file)
			if err != nil {
				t.Fatalf("decode pasted PNG: %v", err)
			}
			if pasted.Bounds() != img.Bounds() || color.NRGBAModel.Convert(pasted.At(12, 24)) != pixel {
				t.Fatalf("pasted PNG has bounds %v and pixel %v", pasted.Bounds(), pasted.At(12, 24))
			}
		})
	})
	t.Run("copied file", func(t *testing.T) {
		smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
			smoke.PreserveClipboard(t)
			smoke.ShowLauncher(t, ctx, client)

			path := filepath.Join(t.TempDir(), "copied image.jpg")
			file, err := os.Create(path)
			if err != nil {
				t.Fatalf("create copied image fixture: %v", err)
			}
			encodeErr := jpeg.Encode(file, image.NewGray(image.Rect(0, 0, 96, 72)), nil)
			closeErr := file.Close()
			if encodeErr != nil || closeErr != nil {
				t.Fatalf("write copied image fixture: encode=%v close=%v", encodeErr, closeErr)
			}
			if err := clipboard.Write(&clipboard.FilePathData{FilePaths: []string{path}}); err != nil {
				t.Fatalf("write image file path to clipboard: %v", err)
			}
			if got := pasteQueryImageAndCopyPath(t, ctx, client); filepath.Clean(got) != filepath.Clean(path) {
				t.Fatalf("copied image selection path = %q, want original file %q", got, path)
			}
		})
	})
}

// pasteQueryImageAndCopyPath verifies the preview result and returns the selection's file path.
func pasteQueryImageAndCopyPath(t *testing.T, ctx context.Context, client *automationdriver.Client) string {
	t.Helper()
	var labels []map[string]string
	for _, language := range []i18n.LangCode{i18n.LangCodeEnUs, i18n.LangCodeZhCn} {
		translations, err := i18n.GetI18nManager().GetLangMap(ctx, language)
		if err != nil {
			t.Fatalf("load selection translations for %s: %v", language, err)
		}
		labels = append(labels, translations)
	}
	modifier := woxui.KeyModifierControl
	if woxui.KeyModifierMeta.HasPrimary() {
		modifier = woxui.KeyModifierMeta
	}
	if handled, err := client.PressKeyHandled(ctx, woxui.Key("v"), modifier); err != nil || !handled {
		t.Fatalf("paste clipboard image: handled=%v err=%v", handled, err)
	}

	var copyID string
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		for _, translations := range labels {
			id, found := smoke.FindLauncherResult(snapshot, translations["selection_copy_path"])
			if found && smoke.HasLauncherResultLabel(snapshot, translations["selection_preview"]) {
				copyID = id
				return true
			}
		}
		return false
	})
	if err != nil {
		t.Fatalf("wait for pasted image selection results: %v", err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
	if err := client.Perform(ctx, copyID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("copy pasted image path: %v", err)
	}

	var path string
	if _, err := client.WaitFor(ctx, func(woxwidget.AutomationSnapshot) bool {
		path, _ = clipboard.ReadText()
		return path != ""
	}); err != nil {
		t.Fatalf("wait for copied image path: %v", err)
	}
	return path
}
