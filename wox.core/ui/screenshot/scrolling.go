package screenshot

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"math"
	"time"
)

const (
	screenshotScrollingMinimumFrameLimit = 16
	screenshotScrollingMaximumFrameLimit = 96
	screenshotScrollingPixelBudget       = 24 * 1024 * 1024
	screenshotScrollingTemplateMaxHeight = 240
	screenshotScrollingMatchThreshold    = 0.82
	screenshotScrollingSeamFeatherRows   = 10
)

type screenshotScrollingFrame struct {
	image           *image.RGBA
	cropTop         int
	seamFeatherRows int
}

func (frame screenshotScrollingFrame) visibleHeight() int {
	return max(0, frame.image.Bounds().Dy()-frame.cropTop)
}

type screenshotScrollingMatch struct {
	overlapRows int
	score       float64
	duplicate   bool
}

type screenshotScrollingDirection uint8

const (
	screenshotScrollingAppend screenshotScrollingDirection = iota
	screenshotScrollingPrepend
)

// appendScreenshotScrollingFrame registers a new viewport and keeps frames in document order.
func appendScreenshotScrollingFrame(frames []screenshotScrollingFrame, next *image.RGBA) ([]screenshotScrollingFrame, bool) {
	if len(frames) == 0 {
		return []screenshotScrollingFrame{{image: next}}, true
	}
	if next.Bounds().Size() != frames[0].image.Bounds().Size() || len(frames) >= screenshotScrollingFrameLimit(frames[0].image) {
		return frames, false
	}

	appendMatch := findScreenshotScrollingOverlap(frames[len(frames)-1], next, screenshotScrollingAppend)
	prependMatch := findScreenshotScrollingOverlap(frames[0], next, screenshotScrollingPrepend)
	if appendMatch.score < screenshotScrollingMatchThreshold && prependMatch.score < screenshotScrollingMatchThreshold {
		return frames, false
	}

	if appendMatch.score >= prependMatch.score {
		if appendMatch.duplicate {
			return frames, false
		}
		frame := screenshotScrollingFrame{
			image:           next,
			cropTop:         appendMatch.overlapRows,
			seamFeatherRows: min(screenshotScrollingSeamFeatherRows, appendMatch.overlapRows),
		}
		if frame.visibleHeight() <= 0 {
			return frames, false
		}
		return append(frames, frame), true
	}

	if prependMatch.duplicate {
		return frames, false
	}
	first := &frames[0]
	first.cropTop = min(first.image.Bounds().Dy(), first.cropTop+prependMatch.overlapRows)
	first.seamFeatherRows = min(screenshotScrollingSeamFeatherRows, min(first.cropTop, first.visibleHeight()))
	if first.visibleHeight() <= 0 {
		frames = frames[1:]
	}
	return append([]screenshotScrollingFrame{{image: next}}, frames...), true
}

func screenshotScrollingFrameLimit(first *image.RGBA) int {
	pixels := first.Bounds().Dx() * first.Bounds().Dy()
	if pixels <= 0 {
		return screenshotScrollingMinimumFrameLimit
	}
	return min(screenshotScrollingMaximumFrameLimit, max(screenshotScrollingMinimumFrameLimit, screenshotScrollingPixelBudget/pixels))
}

func findScreenshotScrollingOverlap(previous screenshotScrollingFrame, next *image.RGBA, direction screenshotScrollingDirection) screenshotScrollingMatch {
	width, height := next.Bounds().Dx(), next.Bounds().Dy()
	if width != previous.image.Bounds().Dx() || height != previous.image.Bounds().Dy() {
		return screenshotScrollingMatch{score: math.Inf(-1)}
	}
	templateHeight := max(1, min(screenshotScrollingTemplateMaxHeight, int(math.Round(float64(previous.visibleHeight())*0.28))))
	templateTop := previous.cropTop
	if direction == screenshotScrollingAppend {
		templateTop = height - templateHeight
	}
	searchMaxY := max(0, min(height-templateHeight, int(float64(height)*0.85)))
	sample := buildScreenshotScrollingSample(previous.image, templateTop, templateHeight)
	candidateY, score := findScreenshotScrollingCandidate(sample, next, searchMaxY)
	overlap := height - candidateY
	if direction == screenshotScrollingAppend {
		overlap = candidateY + templateHeight
	}
	reliable := score >= screenshotScrollingMatchThreshold && overlap > 0 && overlap <= height
	if !reliable {
		overlap = 0
	}
	return screenshotScrollingMatch{
		overlapRows: overlap,
		score:       score,
		duplicate:   reliable && float64(overlap) >= float64(height)*0.94,
	}
}

type screenshotScrollingSample struct {
	xs       []int
	ys       []int
	values   []float64
	sum      float64
	variance float64
}

func buildScreenshotScrollingSample(frame *image.RGBA, templateTop, templateHeight int) screenshotScrollingSample {
	width := frame.Bounds().Dx()
	padding := min(2, max(0, (width-1)/2))
	startX, endX := padding, max(padding+1, width-padding)
	stepX := max(1, int(math.Ceil(float64(endX-startX)/64)))
	stepY := max(1, int(math.Ceil(float64(templateHeight)/48)))
	sample := screenshotScrollingSample{}
	sumSquares := float64(0)
	for relativeY := 0; relativeY < templateHeight; relativeY += stepY {
		for x := startX; x < endX; x += stepX {
			value := screenshotScrollingLuma(frame, x, templateTop+relativeY)
			sample.xs = append(sample.xs, x)
			sample.ys = append(sample.ys, relativeY)
			sample.values = append(sample.values, value)
			sample.sum += value
			sumSquares += value * value
		}
	}
	if len(sample.values) > 0 {
		sample.variance = sumSquares - sample.sum*sample.sum/float64(len(sample.values))
	}
	return sample
}

func findScreenshotScrollingCandidate(sample screenshotScrollingSample, next *image.RGBA, searchMaxY int) (int, float64) {
	bestY, bestScore := 0, math.Inf(-1)
	scoreY := func(candidateY int) {
		for xShift := -2; xShift <= 2; xShift++ {
			score := screenshotScrollingMatchScore(sample, next, candidateY, xShift)
			if score > bestScore {
				bestY, bestScore = candidateY, score
			}
		}
	}
	coarseStep := max(1, min(6, max(1, searchMaxY)))
	for candidateY := 0; candidateY <= searchMaxY; candidateY += coarseStep {
		scoreY(candidateY)
	}
	if searchMaxY%coarseStep != 0 {
		scoreY(searchMaxY)
	}
	coarseBest := bestY
	for candidateY := max(0, coarseBest-max(12, coarseStep*2)); candidateY <= min(searchMaxY, coarseBest+max(12, coarseStep*2)); candidateY++ {
		scoreY(candidateY)
	}
	return bestY, bestScore
}

func screenshotScrollingMatchScore(sample screenshotScrollingSample, frame *image.RGBA, candidateY, xShift int) float64 {
	count := len(sample.values)
	if count == 0 || sample.variance <= 0.0001 {
		return math.Inf(-1)
	}
	candidateSum, candidateSquares, crossSum := float64(0), float64(0), float64(0)
	for index, value := range sample.values {
		candidate := screenshotScrollingLuma(frame, sample.xs[index]+xShift, candidateY+sample.ys[index])
		candidateSum += candidate
		candidateSquares += candidate * candidate
		crossSum += value * candidate
	}
	candidateVariance := candidateSquares - candidateSum*candidateSum/float64(count)
	if candidateVariance <= 0.0001 {
		return math.Inf(-1)
	}
	numerator := crossSum - sample.sum*candidateSum/float64(count)
	return numerator / math.Sqrt(sample.variance*candidateVariance)
}

func screenshotScrollingLuma(frame *image.RGBA, x, y int) float64 {
	x = min(max(0, x), frame.Bounds().Dx()-1)
	y = min(max(0, y), frame.Bounds().Dy()-1)
	pixel := frame.RGBAAt(x, y)
	return float64(pixel.R)*0.299 + float64(pixel.G)*0.587 + float64(pixel.B)*0.114
}

// stitchScreenshotScrollingFrames composites visible frame rows and feathers proven overlap seams.
func stitchScreenshotScrollingFrames(frames []screenshotScrollingFrame) (*image.RGBA, error) {
	if len(frames) == 0 {
		return nil, errors.New("no scrolling screenshot frames were captured")
	}
	width, height := frames[0].image.Bounds().Dx(), 0
	for _, frame := range frames {
		if frame.image.Bounds().Dx() != width {
			return nil, errors.New("scrolling screenshot frame widths do not match")
		}
		height += frame.visibleHeight()
	}
	if width <= 0 || height <= 0 {
		return nil, errors.New("scrolling screenshot is empty")
	}
	output := image.NewRGBA(image.Rect(0, 0, width, height))
	destinationY := 0
	for _, frame := range frames {
		visibleHeight := frame.visibleHeight()
		if visibleHeight <= 0 {
			continue
		}
		source := image.Rect(0, frame.cropTop, width, frame.cropTop+visibleHeight)
		draw.Draw(output, image.Rect(0, destinationY, width, destinationY+visibleHeight), frame.image, source.Min, draw.Src)
		featherRows := min(frame.seamFeatherRows, min(frame.cropTop, visibleHeight))
		if destinationY > 0 && featherRows > 0 {
			for row := 0; row < featherRows; row++ {
				alpha := float64(row+1) / float64(featherRows+1)
				for x := 0; x < width; x++ {
					oldPixel := output.RGBAAt(x, destinationY-featherRows+row)
					newPixel := frame.image.RGBAAt(x, frame.cropTop-featherRows+row)
					output.SetRGBA(x, destinationY-featherRows+row, blendScreenshotScrollingPixel(oldPixel, newPixel, alpha))
				}
			}
		}
		destinationY += visibleHeight
	}
	return output, nil
}

func blendScreenshotScrollingPixel(oldPixel, newPixel color.RGBA, alpha float64) color.RGBA {
	blend := func(oldValue, newValue uint8) uint8 {
		return uint8(math.Round(float64(oldValue)*(1-alpha) + float64(newValue)*alpha))
	}
	return color.RGBA{
		R: blend(oldPixel.R, newPixel.R),
		G: blend(oldPixel.G, newPixel.G),
		B: blend(oldPixel.B, newPixel.B),
		A: 255,
	}
}

func cropScreenshotScrollingFrame(source image.Image, selection Rect, frameSize Size) (*image.RGBA, error) {
	pixelSelection, err := screenshotEditorPixelSelection(source.Bounds(), selection, frameSize)
	if err != nil {
		return nil, err
	}
	cropped := image.NewRGBA(image.Rect(0, 0, pixelSelection.Dx(), pixelSelection.Dy()))
	draw.Draw(cropped, cropped.Bounds(), source, pixelSelection.Min, draw.Src)
	return cropped, nil
}

func newScreenshotScrollingPreview(source image.Image, uiScale float32) (*Image, error) {
	bounds := source.Bounds()
	scale := min(float64(1), min(float64(320*uiScale)/float64(bounds.Dx()), 4096/float64(bounds.Dy())))
	width := max(1, int(math.Round(float64(bounds.Dx())*scale)))
	height := max(1, int(math.Round(float64(bounds.Dy())*scale)))
	preview := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		sourceY := bounds.Min.Y + min(bounds.Dy()-1, int(float64(y)/scale))
		for x := 0; x < width; x++ {
			sourceX := bounds.Min.X + min(bounds.Dx()-1, int(float64(x)/scale))
			preview.Set(x, y, source.At(sourceX, sourceY))
		}
	}
	return NewImage(preview)
}

func (state *screenshotEditorOverlayState) beginScrollingCapture(source image.Image, platform screenshotEditorPlatform) {
	state.mu.Lock()
	done := state.scrollingDone
	state.mu.Unlock()
	select {
	case <-state.scrollingStop:
		close(done)
		return
	default:
	}
	state.mu.Lock()
	selection, workspace := state.selection, state.frameSize
	state.mu.Unlock()
	uiScale := float32(1)
	if platform.chromeScale != nil {
		uiScale = max(float32(1), platform.chromeScale(selection))
	}

	first, err := cropScreenshotScrollingFrame(source, selection, workspace)
	if err != nil {
		state.failScrollingStart(done)
		return
	}
	frames := []screenshotScrollingFrame{{image: first}}
	stitched, err := stitchScreenshotScrollingFrames(frames)
	if err != nil {
		state.failScrollingStart(done)
		return
	}
	preview, err := newScreenshotScrollingPreview(stitched, uiScale)
	if err != nil {
		state.failScrollingStart(done)
		return
	}
	controls := screenshotScrollingControlsRect(selection, workspace, stitched.Bounds().Dx(), stitched.Bounds().Dy(), uiScale)
	var closeBorder func()
	if platform.showScrollBorder != nil {
		closeBorder, err = platform.showScrollBorder(selection, workspace)
		if err != nil {
			state.failScrollingStart(done)
			return
		}
	}

	state.mu.Lock()
	state.workspaceSize = workspace
	state.scrollingFrames = frames
	state.scrollingPreview = preview
	state.scrolling = true
	state.scrollingStarting = false
	state.scrollingOverlaps = screenshotEditorRectsOverlap(controls, selection)
	state.scrollBorderClose = closeBorder
	state.uiScale = uiScale
	state.mu.Unlock()
	if err := setScreenshotScrollingWindowBounds(state.window, platform, controls, workspace); err != nil {
		if closeBorder != nil {
			closeBorder()
		}
		state.mu.Lock()
		state.scrolling = false
		state.scrollingStarting = false
		state.scrollBorderClose = nil
		state.mu.Unlock()
		close(done)
		state.invalidate()
		return
	}
	state.invalidate()

	go state.pollScrollingCapture(platform, selection, workspace, done)
}

func (state *screenshotEditorOverlayState) failScrollingStart(done chan struct{}) {
	state.mu.Lock()
	state.scrollingStarting = false
	state.mu.Unlock()
	close(done)
	state.invalidate()
}

// pollScrollingCapture reuses desktop capture on all platforms; native wheel observers can replace
// the polling only if idle capture cost becomes measurable.
func (state *screenshotEditorOverlayState) pollScrollingCapture(platform screenshotEditorPlatform, selection Rect, workspace Size, done chan struct{}) {
	defer close(done)
	uiScale := float32(1)
	if platform.chromeScale != nil {
		uiScale = max(float32(1), platform.chromeScale(selection))
	}
	ticker := time.NewTicker(220 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-state.scrollingStop:
			return
		case <-ticker.C:
		}

		state.mu.Lock()
		overlaps := state.scrollingOverlaps
		state.mu.Unlock()
		if overlaps {
			_ = state.window.Hide()
			select {
			case <-state.scrollingStop:
				return
			case <-time.After(40 * time.Millisecond):
			}
		}
		captured, err := platform.captureDesktop()
		if overlaps {
			_, _ = state.window.Show()
		}
		if err != nil {
			continue
		}
		next, err := cropScreenshotScrollingFrame(captured.source, selection, workspace)
		captured.close()
		if err != nil {
			continue
		}

		state.mu.Lock()
		frames, accepted := appendScreenshotScrollingFrame(state.scrollingFrames, next)
		state.scrollingFrames = frames
		snapshot := append([]screenshotScrollingFrame(nil), frames...)
		state.mu.Unlock()
		if !accepted {
			continue
		}
		stitched, err := stitchScreenshotScrollingFrames(snapshot)
		if err != nil {
			continue
		}
		preview, err := newScreenshotScrollingPreview(stitched, uiScale)
		if err != nil {
			continue
		}
		controls := screenshotScrollingControlsRect(selection, workspace, stitched.Bounds().Dx(), stitched.Bounds().Dy(), uiScale)
		state.mu.Lock()
		state.scrollingPreview = preview
		state.scrollingOverlaps = screenshotEditorRectsOverlap(controls, selection)
		state.mu.Unlock()
		_ = setScreenshotScrollingWindowBounds(state.window, platform, controls, workspace)
		state.invalidate()
	}
}

func (state *screenshotEditorOverlayState) stopScrollingCapture() {
	state.scrollingStopOnce.Do(func() {
		close(state.scrollingStop)
	})
	state.mu.Lock()
	done := state.scrollingDone
	closeBorder := state.scrollBorderClose
	state.scrollBorderClose = nil
	state.mu.Unlock()
	if done != nil {
		<-done
	}
	if closeBorder != nil {
		closeBorder()
	}
}

// setScreenshotScrollingWindowBounds keeps Windows in physical pixels and other platforms logical.
func setScreenshotScrollingWindowBounds(window *Window, platform screenshotEditorPlatform, controls Rect, workspace Size) error {
	if platform.setScrollBounds != nil {
		return platform.setScrollBounds(window, controls, workspace)
	}
	return window.SetBounds(platform.logicalSelection(controls, workspace))
}

func screenshotScrollingControlsRect(selection Rect, workspace Size, contentWidth, contentHeight int, uiScale float32) Rect {
	const (
		margin         = float32(24)
		toolbarHeight  = float32(72)
		minimumWidth   = float32(168)
		maximumWidth   = float32(320)
		selectionGap   = float32(20)
		availableInset = float32(44)
	)
	marginPx := margin * uiScale
	toolbarHeightPx := toolbarHeight * uiScale
	minimumWidthPx := minimumWidth * uiScale
	maximumWidthPx := maximumWidth * uiScale
	selectionGapPx := selectionGap * uiScale
	availableInsetPx := availableInset * uiScale
	rightAvailable := max(float32(0), workspace.Width-selection.X-selection.Width-availableInsetPx)
	leftAvailable := max(float32(0), selection.X-availableInsetPx)
	maxPreviewWidth := min(max(rightAvailable, leftAvailable), maximumWidthPx)
	maxPreviewHeight := max(float32(1), workspace.Height-marginPx*2-toolbarHeightPx)
	scale := min(
		max(float32(1), maxPreviewWidth)/max(float32(1), float32(contentWidth)),
		maxPreviewHeight/max(float32(1), float32(contentHeight)),
	)
	previewWidth := max(float32(1), float32(contentWidth)*scale)
	previewHeight := max(float32(1), float32(contentHeight)*scale)
	controlsWidth := max(previewWidth, minimumWidthPx)
	controlsHeight := previewHeight + toolbarHeightPx
	useRight := selection.X+selection.Width+selectionGapPx+controlsWidth <= workspace.Width-marginPx || rightAvailable >= leftAvailable
	left := selection.X + selection.Width + selectionGapPx
	if !useRight {
		left = max(marginPx, selection.X-controlsWidth-selectionGapPx)
	}
	left = min(max(float32(0), left), max(float32(0), workspace.Width-controlsWidth))
	top := min(max(marginPx, selection.Y), max(marginPx, workspace.Height-controlsHeight-marginPx))
	return Rect{X: left, Y: top, Width: controlsWidth, Height: controlsHeight}
}

func screenshotEditorRectsOverlap(left, right Rect) bool {
	return left.X < right.X+right.Width &&
		left.X+left.Width > right.X &&
		left.Y < right.Y+right.Height &&
		left.Y+left.Height > right.Y
}

// screenshotScrollingControlLayout matches Flutter's centered 124x56 action capsule.
func screenshotScrollingControlLayout(frame Size, uiScale float32) (Rect, Rect, Rect) {
	toolbarWidth, toolbarHeight := 124*uiScale, 56*uiScale
	toolbar := Rect{X: (frame.Width - toolbarWidth) / 2, Y: frame.Height - toolbarHeight, Width: toolbarWidth, Height: toolbarHeight}
	cancel := Rect{X: toolbar.X + 18*uiScale, Y: toolbar.Y + 8*uiScale, Width: 40 * uiScale, Height: 40 * uiScale}
	confirm := Rect{X: toolbar.X + 66*uiScale, Y: toolbar.Y + 8*uiScale, Width: 40 * uiScale, Height: 40 * uiScale}
	return toolbar, cancel, confirm
}

func drawScreenshotScrollingControls(displayList *DisplayList, frame Size, preview *Image, uiScale float32) {
	displayList.Clear(Color{})
	if preview != nil {
		maxHeight := max(float32(1), frame.Height-72*uiScale)
		scale := min(frame.Width/float32(preview.Width), maxHeight/float32(preview.Height))
		width, height := float32(preview.Width)*scale, float32(preview.Height)*scale
		previewRect := Rect{X: (frame.Width - width) / 2, Width: width, Height: height}
		// Keep the thumbnail opaque even when image sampling leaves fractional edge pixels.
		displayList.FillRoundedRect(previewRect, 0, Color{R: 30, G: 26, B: 24, A: 255})
		displayList.DrawImage(preview, previewRect)
		displayList.StrokeRoundedRect(previewRect, 0, 2*uiScale, Color{R: 255, G: 255, B: 255, A: 204})
	}
	toolbar, cancel, confirm := screenshotScrollingControlLayout(frame, uiScale)
	displayList.FillRoundedRect(toolbar, 18*uiScale, Color{R: 30, G: 26, B: 24, A: 230})
	drawScreenshotEditorToolbarIcon(displayList, "control.close", cancel, Color{R: 255, G: 107, B: 107, A: 255}, uiScale)
	drawScreenshotEditorToolbarIcon(displayList, "control.check", confirm, Color{R: 48, G: 227, B: 122, A: 255}, uiScale)
}
