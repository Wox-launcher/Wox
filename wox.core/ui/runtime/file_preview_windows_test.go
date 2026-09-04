//go:build windows

package woxui

import "testing"

// TestNativeFilePreviewOcclusionIsRetainedForLaterPreviews covers an overlay reported before the
// delayed handler exists, the clearing path, and a display-scale transition at unchanged bounds.
func TestNativeFilePreviewOcclusionIsRetainedForLaterPreviews(t *testing.T) {
	window := &platformWindow{scale: 1.25, nativeFilePreviewCornerRadius: 7}
	bounds := Rect{X: -10.2, Y: 20.2, Width: 100, Height: 50}
	command := windowCommand{kind: windowCommandSetNativeFilePreviewOcclusion, nativeFileBounds: bounds}

	window.executeCommand(command)
	if window.nativeFilePreviewOcclusion != bounds || window.nativeFilePreviewOcclusionScale != 1.25 {
		t.Fatalf("overlay was not retained before the delayed preview was created: %#v at scale %v", window.nativeFilePreviewOcclusion, window.nativeFilePreviewOcclusionScale)
	}

	window.scale = 1.5
	window.executeCommand(command)
	if window.nativeFilePreviewOcclusionScale != 1.5 {
		t.Fatalf("unchanged logical bounds ignored the display-scale transition: scale %v", window.nativeFilePreviewOcclusionScale)
	}

	command.nativeFileBounds = Rect{}
	window.executeCommand(command)
	if window.nativeFilePreviewOcclusion != (Rect{}) {
		t.Fatalf("closing the overlay did not clear occlusion: %#v", window.nativeFilePreviewOcclusion)
	}
	if window.nativeFilePreviewCornerRadius != 7 {
		t.Fatalf("closing the overlay discarded the preview's rounded corners: radius %v", window.nativeFilePreviewCornerRadius)
	}
}

// TestNativeFilePreviewOcclusionPixelsRoundOutward keeps fractional scaling from leaving a strip of
// preview visible over the overlay, including displays with a negative desktop origin.
func TestNativeFilePreviewOcclusionPixelsRoundOutward(t *testing.T) {
	bounds := Rect{X: -10.2, Y: 20.2, Width: 100, Height: 50}
	for _, tc := range []struct {
		scale float32
		want  [4]int32
	}{
		{1, [4]int32{-11, 20, 90, 71}},
		{1.25, [4]int32{-13, 25, 113, 88}},
		{1.5, [4]int32{-16, 30, 135, 106}},
		{2, [4]int32{-21, 40, 180, 141}},
	} {
		left, top, right, bottom := filePreviewOcclusionPixels(bounds, tc.scale)
		if got := [4]int32{left, top, right, bottom}; got != tc.want {
			t.Fatalf("scale %v: got %v, want %v", tc.scale, got, tc.want)
		}
	}

	left, top, right, bottom := filePreviewOcclusionPixels(Rect{}, 1.5)
	if left != 0 || top != 0 || right != 0 || bottom != 0 {
		t.Fatalf("empty overlay must restore the entire native region: got %v %v %v %v", left, top, right, bottom)
	}
}

func TestNativeFilePreviewCommandsIgnoreOlderGenerations(t *testing.T) {
	window := &platformWindow{nativeFilePreviewGeneration: 2}

	window.executeCommand(windowCommand{
		kind:                        windowCommandShowNativeFilePreview,
		nativeFilePath:              "stale.docx",
		nativeFilePreviewGeneration: 1,
	})
	if window.nativeFilePreviewGeneration != 2 || window.nativeFilePreview != nil {
		t.Fatalf("stale show changed native preview state: generation %d preview %#v", window.nativeFilePreviewGeneration, window.nativeFilePreview)
	}

	window.executeCommand(windowCommand{
		kind:                        windowCommandHideNativeFilePreview,
		nativeFilePreviewGeneration: 3,
	})
	window.executeCommand(windowCommand{
		kind:                        windowCommandShowNativeFilePreview,
		nativeFilePath:              "stale-again.docx",
		nativeFilePreviewGeneration: 2,
	})
	if window.nativeFilePreviewGeneration != 3 || window.nativeFilePreview != nil {
		t.Fatalf("older show resurrected after hide: generation %d preview %#v", window.nativeFilePreviewGeneration, window.nativeFilePreview)
	}
}
