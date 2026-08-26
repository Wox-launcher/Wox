package preview

import (
	"math"
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestPreviewImageFitScaleContainsTallImage(t *testing.T) {
	scale := previewImageFitScale(40, 800, 400, 300)
	want := (300 - previewImageFitPadding) / 800
	if math.Abs(float64(scale-want)) > 1e-5 {
		t.Fatalf("tall image fit scale = %v, want %v", scale, want)
	}
}

func TestPreviewImageWheelZoomStaysAroundPointer(t *testing.T) {
	props := PreviewImageProps{Width: 400, Height: 300, Image: &woxui.Image{Width: 400, Height: 300}}
	drawWidth, drawHeight := previewImageDrawSize(props, 1)
	left := (props.Width - drawWidth) * 0.5
	top := (props.Height - drawHeight) * 0.5
	cursorX := left + drawWidth*0.35
	cursorY := top + drawHeight*0.4

	zoom, nextLeft, nextTop := applyPreviewImageWheelZoom(1, left, top, 2, cursorX, cursorY, props)
	if zoom != 2 {
		t.Fatalf("zoom = %v, want 2", zoom)
	}
	nextWidth, nextHeight := previewImageDrawSize(props, zoom)
	if math.Abs(float64((cursorX-nextLeft)/(nextWidth)-(cursorX-left)/drawWidth)) > 1e-4 {
		t.Fatalf("horizontal focus drifted: left %v -> %v", left, nextLeft)
	}
	if math.Abs(float64((cursorY-nextTop)/(nextHeight)-(cursorY-top)/drawHeight)) > 1e-4 {
		t.Fatalf("vertical focus drifted: top %v -> %v", top, nextTop)
	}
}

func TestPreviewImageWheelZoomClampsToFitAndMax(t *testing.T) {
	props := PreviewImageProps{Width: 400, Height: 300, Image: &woxui.Image{Width: 100, Height: 80}}
	zoom, _, _ := applyPreviewImageWheelZoom(1, 0, 0, 0.5, 200, 150, props)
	if zoom != previewImageMinZoom {
		t.Fatalf("zoom out below fit = %v, want %v", zoom, previewImageMinZoom)
	}
	zoom, _, _ = applyPreviewImageWheelZoom(previewImageMaxZoom, 0, 0, 2, 200, 150, props)
	if zoom != previewImageMaxZoom {
		t.Fatalf("zoom past max = %v, want %v", zoom, previewImageMaxZoom)
	}
}

func TestPreviewImageOriginRecentersWhenImageFits(t *testing.T) {
	left, top := clampPreviewImageOrigin(-40, -20, 100, 80, 400, 300)
	if left != 150 || top != 110 {
		t.Fatalf("fitted origin = %v,%v, want centered 150,110", left, top)
	}
}

func TestPreviewImageOriginClampsWhenZoomedPastViewport(t *testing.T) {
	left, top := clampPreviewImageOrigin(-20, -30, 500, 400, 400, 300)
	if left != -20 || top != -30 {
		t.Fatalf("in-range zoomed origin = %v,%v, want -20,-30", left, top)
	}
	left, top = clampPreviewImageOrigin(10, 8, 500, 400, 400, 300)
	if left != 0 || top != 0 {
		t.Fatalf("positive overflow origin = %v,%v, want 0,0", left, top)
	}
	left, top = clampPreviewImageOrigin(-200, -180, 500, 400, 400, 300)
	if left != -100 || top != -100 {
		t.Fatalf("negative overflow origin = %v,%v, want -100,-100", left, top)
	}
}

func TestPreviewImageZoomedGestureEnablesPan(t *testing.T) {
	props := PreviewImageProps{Width: 400, Height: 300, Image: &woxui.Image{Width: 40, Height: 800}}
	state := &previewImageState{zoom: 2, hasOffset: true}
	gesture := state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture)
	if gesture.OnPointer == nil || gesture.OnPanStart == nil || gesture.OnPanUpdate == nil {
		t.Fatal("zoomed preview image should accept wheel zoom and drag pan")
	}
	if gesture.Cursor != woxui.PointerCursorMove {
		t.Fatalf("zoomed cursor = %v, want move", gesture.Cursor)
	}
}

func TestPreviewImageShowsCenteredLoadingIndicator(t *testing.T) {
	color := woxui.Color{R: 10, G: 20, B: 30, A: 255}
	loading := PreviewImage(PreviewImageProps{Width: 240, Height: 180, LoadingColor: color}).(woxwidget.Align)
	if loading.Width != 240 || loading.Height != 180 || loading.Horizontal != 0.5 || loading.Vertical != 0.5 {
		t.Fatalf("image loading align = %#v, want a centered preview placeholder", loading)
	}
	if indicator := loading.Child.(woxwidget.LoopAnimation); indicator.Key != "wox-loading-indicator" {
		t.Fatalf("image loading child = %#v, want WoxLoadingIndicator", indicator)
	}
}

func TestPreviewImageShowsErrorTextInsteadOfLoadingIndicator(t *testing.T) {
	view := PreviewImage(PreviewImageProps{
		Width: 240, Height: 180, Message: "Unable to decode image preview:\nbad data", MessageColor: woxui.Color{R: 244, G: 67, B: 54, A: 255},
	}).(woxwidget.Container)
	block := view.Child.(woxwidget.TextBlock)
	if block.Value != "Unable to decode image preview:\nbad data" {
		t.Fatalf("image error = %q, want the decode failure copy", block.Value)
	}
}

func TestPreviewImageFitGestureDoesNotPan(t *testing.T) {
	props := PreviewImageProps{Width: 400, Height: 300, Image: &woxui.Image{Width: 40, Height: 800}}
	gesture := builtPreviewImage(props)
	if gesture.OnPointer == nil {
		t.Fatal("fit preview image should still accept wheel zoom")
	}
	if gesture.OnPanStart != nil || gesture.OnPanUpdate != nil {
		t.Fatal("fit preview image should not capture drags")
	}
}
