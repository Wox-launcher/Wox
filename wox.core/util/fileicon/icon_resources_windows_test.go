package fileicon

import (
	"context"
	"errors"
	"image"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// overriddenIconProcedure retains real native allocation while controlling its reported status.
type overriddenIconProcedure struct {
	iconExtractionProcedure
	result    uintptr
	lastError error
	failed    bool
}

// Call overrides status only after native extraction has allocated a valid test icon.
func (p overriddenIconProcedure) Call(args ...uintptr) (uintptr, uintptr, error) {
	ret, second, err := p.iconExtractionProcedure.Call(args...)
	if ret == 0 || ret == 0xFFFFFFFF {
		return ret, second, err
	}
	if p.failed {
		ret = p.result
	}
	return ret, second, p.lastError
}

// TestIconResourceLifetime checks real GDI and USER ownership, including early failure exits.
func TestIconResourceLifetime(t *testing.T) {
	ctx := context.Background()
	shellPath := filepath.Join(os.Getenv("SystemRoot"), "System32", "shell32.dll")
	resources := syscall.NewLazyDLL("user32.dll").NewProc("GetGuiResources")
	if err := resources.Find(); err != nil {
		t.Fatal(err)
	}
	// These subtests must remain sequential because they override the native procedure.
	native := privateExtractIcons
	t.Cleanup(func() { privateExtractIcons = native })
	for _, status := range []struct {
		name      string
		procedure iconExtractionProcedure
		wantError bool
	}{
		{"native", native, false},
		{"localized-success", overriddenIconProcedure{iconExtractionProcedure: native, lastError: errors.New("Operazione completata")}, false},
		{"stale-last-error", overriddenIconProcedure{iconExtractionProcedure: native, lastError: syscall.Errno(5)}, false},
		{"zero-result-with-handle", overriddenIconProcedure{iconExtractionProcedure: native, failed: true, result: 0}, true},
		{"invalid-result-with-handle", overriddenIconProcedure{iconExtractionProcedure: native, failed: true, result: 0xFFFFFFFF}, true},
	} {
		t.Run(status.name, func(t *testing.T) {
			privateExtractIcons = status.procedure
			for _, extraction := range []struct {
				name string
				run  func() (image.Image, error)
			}{
				{"default", func() (image.Image, error) { return getHighResDefaultIcon(ctx) }},
				{"file", func() (image.Image, error) { return getHighResIcon(ctx, shellPath) }},
				{"fallback", func() (image.Image, error) { return getIconUsingExtractIconEx(ctx, shellPath) }},
			} {
				if status.name != "native" && extraction.name == "fallback" {
					continue
				}
				t.Run(extraction.name, func(t *testing.T) {
					// Warm up lazy Windows initialization before measuring retained resources.
					_, _ = extraction.run()
					beforeGDI, _, _ := resources.Call(^uintptr(0), 0)
					beforeUSER, _, _ := resources.Call(^uintptr(0), 1)
					for i := 0; i < 100; i++ {
						img, err := extraction.run()
						if (err != nil) != status.wantError || (!status.wantError && img == nil) {
							t.Fatalf("iteration %d: image present=%t, error=%v, want error=%t", i, img != nil, err, status.wantError)
						}
					}
					afterGDI, _, _ := resources.Call(^uintptr(0), 0)
					afterUSER, _, _ := resources.Call(^uintptr(0), 1)
					t.Logf("100 extractions: GDI %d -> %d; USER %d -> %d", beforeGDI, afterGDI, beforeUSER, afterUSER)
					// Allow minor OS bookkeeping, but catch even one leaked icon per extraction.
					if afterGDI > beforeGDI+8 || afterUSER > beforeUSER+8 {
						t.Fatal("icon extraction leaked native resources")
					}
				})
			}
		})
	}
}
