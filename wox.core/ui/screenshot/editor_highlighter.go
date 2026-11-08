package screenshot

import (
	"image"
	"image/color"
	"math"
	"reflect"
	"slices"
)

var screenshotEditorHighlighterColor = Color{R: 255, G: 255, A: 255}
var screenshotEditorHighlighterRadii = [...]float32{6, 10, 14}

func screenshotEditorToolPalette(tool screenshotEditorTool) [6]Color {
	palette := screenshotEditorPalette
	if tool == screenshotEditorToolHighlighter {
		copy(palette[1:], palette[:len(palette)-1])
		palette[0] = screenshotEditorHighlighterColor
	}
	return palette
}

func screenshotEditorToolStrokeRadii(tool screenshotEditorTool) [3]float32 {
	if tool == screenshotEditorToolHighlighter {
		return screenshotEditorHighlighterRadii
	}
	return screenshotEditorBrushRadii
}

func (state *screenshotEditorOverlayState) creationColorLocked() Color {
	if state.activeTool == screenshotEditorToolHighlighter {
		if state.highlighterColor.A == 0 {
			return screenshotEditorHighlighterColor
		}
		return state.highlighterColor
	}
	return state.annotationColor
}

// updateHighlighterStrokeLocked lets Shift replace the trailing segment without touching the earlier freehand path.
func (state *screenshotEditorOverlayState) updateHighlighterStrokeLocked(point Point, shift bool) {
	draft := state.draft
	last := len(draft.points) - 1
	if shift && state.highlighterSnapping {
		if point == draft.points[last-1] {
			draft.points = draft.points[:last]
			draft.flatCap, state.highlighterSnapping = false, false
		} else {
			draft.points[last] = point
		}
		return
	}
	state.highlighterSnapping = false
	if draft.points[last] != point {
		draft.points = append(draft.points, point)
		state.highlighterSnapping = shift
		draft.flatCap = shift && last == 0
	}
}

// screenshotEditorHighlighterBlend samples the frozen capture so draft, export, and overlapping strokes use the same backdrop.
func screenshotEditorHighlighterBlend(source, underlay image.Image, ink Color, uiScale, scaleX, scaleY float32) func(int, int) color.RGBA {
	bounds := source.Bounds()
	inkTone := screenshotEditorHighlighterTone(color.RGBA{R: ink.R, G: ink.G, B: ink.B, A: 255})
	uiScale = max(1, uiScale)
	cellWidth := max(1, int(math.Round(float64(4*uiScale*scaleX))))
	cellHeight := max(1, int(math.Round(float64(4*uiScale*scaleY))))
	dx, dy := 2*cellWidth, 2*cellHeight
	read := screenshotHighlighterPixelReader(source)
	readUnderlay := screenshotHighlighterPixelReader(underlay)
	sample := func(x, y int) int {
		return screenshotEditorHighlighterTone(read(min(max(x, bounds.Min.X), bounds.Max.X-1), min(max(y, bounds.Min.Y), bounds.Max.Y-1)))
	}
	backdrops := make(map[image.Point]screenshotHighlighterBackdrop)
	return func(x, y int) color.RGBA {
		pixel := readUnderlay(x, y)
		if pixel.A == 0 {
			return pixel
		}
		cell := image.Pt((x-bounds.Min.X)/cellWidth, (y-bounds.Min.Y)/cellHeight)
		backdrop, found := backdrops[cell]
		if !found {
			center := image.Pt(bounds.Min.X+cell.X*cellWidth+cellWidth/2, bounds.Min.Y+cell.Y*cellHeight+cellHeight/2)
			backdrop = estimateScreenshotHighlighterBackdrop(center, dx, dy, sample)
			backdrops[cell] = backdrop
		}
		light := backdrop.mostlyLight
		if backdrop.hasEdge {
			light = sample(x, y) >= 128
		}
		tone := screenshotEditorHighlighterTone(pixel)
		contrast := tone - backdrop.dark
		if light {
			contrast = backdrop.light - tone
		}

		// Keep the source foreground when it contrasts with the ink. The ramp retains
		// partially covered glyph edges even when the foreground needs more contrast.
		if inkTone >= 128 && tone > inkTone-96 {
			pixel.R, pixel.G, pixel.B = 0, 0, 0
		} else if inkTone < 128 && tone < inkTone+96 {
			pixel.R, pixel.G, pixel.B = pixel.A, pixel.A, pixel.A
		}
		const textRange = 112
		textWeight := min(textRange, max(0, contrast-24))
		mix := func(inkChannel, textChannel uint8) uint8 {
			return uint8((int(inkChannel)*int(pixel.A)*(textRange-textWeight) + 255*int(textChannel)*textWeight) / (255 * textRange))
		}
		pixel.R = mix(ink.R, pixel.R)
		pixel.G = mix(ink.G, pixel.G)
		pixel.B = mix(ink.B, pixel.B)
		return pixel
	}
}

func screenshotHighlighterPixelReader(source image.Image) func(int, int) color.RGBA {
	if packed, ok := source.(interface{ RGBAAt(int, int) color.RGBA }); ok {
		return packed.RGBAAt
	}
	return func(x, y int) color.RGBA { return color.RGBAModel.Convert(source.At(x, y)).(color.RGBA) }
}

type screenshotHighlighterBackdrop struct {
	dark, light int
	mostlyLight bool
	hasEdge     bool
}

// estimateScreenshotHighlighterBackdrop samples across several glyphs so an adjacent line
// of text cannot change the ink polarity. Strong opposite edges distinguish two backgrounds.
func estimateScreenshotHighlighterBackdrop(center image.Point, dx, dy int, sample func(int, int) int) screenshotHighlighterBackdrop {
	const radius = 3
	const side = 2*radius + 1
	var tones [side * side]int
	var darkCount, left, right, top, bottom int
	for row := -radius; row <= radius; row++ {
		for col := -radius; col <= radius; col++ {
			tone := sample(center.X+col*dx, center.Y+row*dy)
			tones[(row+radius)*side+col+radius] = tone
			if tone < 128 {
				darkCount++
				continue
			}
			if col <= -radius+1 {
				left++
			} else if col >= radius-1 {
				right++
			}
			if row <= -radius+1 {
				top++
			} else if row >= radius-1 {
				bottom++
			}
		}
	}
	slices.Sort(tones[:])
	// Require two rows or columns of agreement so a bright text baseline cannot mimic an edge.
	opposite := func(a, b int) bool { return min(a, b) <= 2 && max(a, b) >= 2*side-2 }
	return screenshotHighlighterBackdrop{
		dark: tones[max(0, darkCount-1)/2], light: tones[(darkCount+len(tones)-1)/2],
		mostlyLight: darkCount < len(tones)-darkCount,
		hasEdge:     opposite(left, right) || opposite(top, bottom),
	}
}

func screenshotEditorHighlighterTone(pixel color.RGBA) int {
	return (54*int(pixel.R) + 183*int(pixel.G) + 19*int(pixel.B)) * 255 / (256 * max(1, int(pixel.A)))
}

// paintScreenshotEditorHighlighter works in capture pixels while its points and radius stay logical.
func paintScreenshotEditorHighlighter(target *image.RGBA, source, underlay image.Image, annotation screenshotEditorAnnotation, origin image.Point, uiScale, scaleX, scaleY float32) {
	blend := screenshotEditorHighlighterBlend(source, underlay, screenshotEditorAnnotationDrawColor(annotation), uiScale, scaleX, scaleY)
	visitScreenshotEditorStrokePixels(target.Bounds().Sub(origin), annotation.points, screenshotEditorStrokeRadius(annotation, uiScale), scaleX, scaleY, annotation.flatCap, func(x, y int) {
		x, y = x+origin.X, y+origin.Y
		target.SetRGBA(x, y, blend(x, y))
	})
}

func screenshotEditorHighlighterTailBounds(mark screenshotEditorAnnotation, uiScale float32) Rect {
	mark.points = mark.points[max(0, len(mark.points)-2):]
	return screenshotEditorAnnotationDirtyRect(mark, uiScale)
}

// The editor owns these rasters on its paint thread. Committed pixels are reused while
// the draft paints sparse tiles, with the moving Shift endpoint kept in a separate layer.
type screenshotEditorHighlighterPreview struct {
	source      image.Image
	wallpaper   *image.RGBA
	windowAlpha bool
	selection   Rect
	frame       Size
	uiScale     float32
	marks       []screenshotEditorAnnotation
	underlay    *image.RGBA
	inkUnderlay image.Image
	composed    screenshotHighlighterTiles
	stable      screenshotHighlighterTiles
	stableCount int
	tail        screenshotHighlighterTiles
	tailPoints  []Point
	tailFlat    bool
}

// draw replaces only marker-covered pixels in the native preview with the shared export composite.
func (preview *screenshotEditorHighlighterPreview) draw(list *DisplayList, source image.Image, wallpaper *image.RGBA, windowAlpha bool, selection Rect, frame Size, uiScale float32, marks []screenshotEditorAnnotation, draft *screenshotEditorAnnotation) {
	if source == nil || (draft == nil && !slices.ContainsFunc(marks, func(mark screenshotEditorAnnotation) bool {
		return mark.tool == screenshotEditorToolHighlighter
	})) {
		*preview = screenshotEditorHighlighterPreview{}
		return
	}
	bounds := source.Bounds()
	scaleX, scaleY := float32(bounds.Dx())/frame.Width, float32(bounds.Dy())/frame.Height
	if preview.source != source || preview.wallpaper != wallpaper || preview.windowAlpha != windowAlpha ||
		preview.selection != selection || preview.frame != frame || preview.uiScale != uiScale || !reflect.DeepEqual(preview.marks, marks) {
		underlay, inkUnderlay, err := compositeScreenshotEditorAnnotations(source, marks, selection, frame, uiScale)
		if err != nil {
			*preview = screenshotEditorHighlighterPreview{}
			return
		}
		*preview = screenshotEditorHighlighterPreview{
			source: source, wallpaper: wallpaper, windowAlpha: windowAlpha, selection: selection, frame: frame, uiScale: uiScale,
			marks: marks, underlay: underlay, inkUnderlay: inkUnderlay,
		}
		for _, mark := range marks {
			if mark.tool == screenshotEditorToolHighlighter {
				preview.composed.paint(preview, mark, bounds.Min, scaleX, scaleY, underlay.RGBAAt)
			}
		}
	}
	preview.composed.draw(list, bounds.Min, scaleX, scaleY)
	if draft == nil {
		preview.stable, preview.stableCount = screenshotHighlighterTiles{}, 0
		preview.tail, preview.tailPoints = screenshotHighlighterTiles{}, nil
		return
	}
	preview.drawDraft(list, *draft, bounds.Min, scaleX, scaleY)
}

// drawDraft keeps completed segments and only replaces the segment that can still move.
func (preview *screenshotEditorHighlighterPreview) drawDraft(list *DisplayList, draft screenshotEditorAnnotation, origin image.Point, scaleX, scaleY float32) {
	pixel := screenshotEditorHighlighterBlend(preview.source, preview.inkUnderlay, screenshotEditorAnnotationDrawColor(draft), preview.uiScale, scaleX, scaleY)
	stableCount := 0
	if !draft.flatCap {
		stableCount = max(0, len(draft.points)-1)
	}
	if preview.stableCount > stableCount {
		preview.stable, preview.stableCount = screenshotHighlighterTiles{}, 0
	}
	if stableCount > preview.stableCount {
		completed := draft
		completed.points = draft.points[max(0, preview.stableCount-1):stableCount]
		preview.stable.paint(preview, completed, origin, scaleX, scaleY, pixel)
		preview.stableCount = stableCount
	}
	tailPoints := draft.points[max(0, len(draft.points)-2):]
	if preview.tailFlat != draft.flatCap || !slices.Equal(preview.tailPoints, tailPoints) {
		preview.tail, preview.tailFlat = screenshotHighlighterTiles{}, draft.flatCap
		preview.tailPoints = append(preview.tailPoints[:0], tailPoints...)
		draft.points = tailPoints
		preview.tail.paint(preview, draft, origin, scaleX, scaleY, pixel)
	}
	preview.stable.draw(list, origin, scaleX, scaleY)
	preview.tail.draw(list, origin, scaleX, scaleY)
}

const screenshotHighlighterTileSize = 128

type screenshotHighlighterTile struct {
	pixels *image.RGBA
	bitmap *Image
}

type screenshotHighlighterTiles struct {
	byOrigin map[image.Point]*screenshotHighlighterTile
	ordered  []*screenshotHighlighterTile
}

// paint allocates tiles only where a stroke covers pixels. Tiles can overlap across the
// two draft layers because both derive the same opaque result from the committed scene.
func (tiles *screenshotHighlighterTiles) paint(preview *screenshotEditorHighlighterPreview, mark screenshotEditorAnnotation, origin image.Point, scaleX, scaleY float32, sample func(int, int) color.RGBA) {
	if tiles.byOrigin == nil {
		tiles.byOrigin = make(map[image.Point]*screenshotHighlighterTile)
	}
	clip := preview.underlay.Bounds()
	var windowPixel func(int, int) color.RGBA
	if preview.windowAlpha {
		windowPixel = screenshotHighlighterPixelReader(preview.source)
	}
	wallpaperOrigin := image.Point{}
	if preview.wallpaper != nil {
		wallpaperOrigin = preview.wallpaper.Bounds().Min.Add(preview.wallpaper.Bounds().Size().Sub(clip.Size()).Div(2))
	}
	visitScreenshotEditorStrokePixels(clip.Sub(origin), mark.points, screenshotEditorStrokeRadius(mark, preview.uiScale), scaleX, scaleY, mark.flatCap, func(x, y int) {
		x, y = x+origin.X, y+origin.Y
		left := clip.Min.X + (x-clip.Min.X)/screenshotHighlighterTileSize*screenshotHighlighterTileSize
		top := clip.Min.Y + (y-clip.Min.Y)/screenshotHighlighterTileSize*screenshotHighlighterTileSize
		key := image.Pt(left, top)
		tile := tiles.byOrigin[key]
		if tile == nil {
			tile = &screenshotHighlighterTile{pixels: image.NewRGBA(image.Rectangle{Min: key, Max: key.Add(image.Pt(screenshotHighlighterTileSize, screenshotHighlighterTileSize))}.Intersect(clip))}
			tiles.byOrigin[key] = tile
			tiles.ordered = append(tiles.ordered, tile)
		}
		pixel := sample(x, y)
		if windowPixel != nil {
			if alpha := windowPixel(x, y).A; alpha < pixel.A {
				pixel.R = uint8(uint16(pixel.R) * uint16(alpha) / uint16(pixel.A))
				pixel.G = uint8(uint16(pixel.G) * uint16(alpha) / uint16(pixel.A))
				pixel.B = uint8(uint16(pixel.B) * uint16(alpha) / uint16(pixel.A))
				pixel.A = alpha
			}
		}
		if preview.wallpaper != nil && pixel.A != 255 {
			back := preview.wallpaper.RGBAAt(x-clip.Min.X+wallpaperOrigin.X, y-clip.Min.Y+wallpaperOrigin.Y)
			remaining := uint16(255 - pixel.A)
			pixel.R += uint8(uint16(back.R) * remaining / 255)
			pixel.G += uint8(uint16(back.G) * remaining / 255)
			pixel.B += uint8(uint16(back.B) * remaining / 255)
		}
		pixel.A = 255
		if tile.pixels.RGBAAt(x, y) == pixel {
			return
		}
		if tile.bitmap != nil {
			// A previous display list can still retain the published buffer.
			updated := image.NewRGBA(tile.pixels.Bounds())
			copy(updated.Pix, tile.pixels.Pix)
			tile.pixels, tile.bitmap = updated, nil
		}
		tile.pixels.SetRGBA(x, y, pixel)
	})
}

// draw publishes changed tiles without modifying the pixel buffers held by previous display lists.
func (tiles *screenshotHighlighterTiles) draw(list *DisplayList, origin image.Point, scaleX, scaleY float32) {
	for _, tile := range tiles.ordered {
		if tile.bitmap == nil {
			tile.bitmap, _ = NewImageFromPackedRGBA(tile.pixels)
		}
		list.DrawImage(tile.bitmap, screenshotHighlighterLogicalRect(tile.pixels.Bounds(), origin, scaleX, scaleY))
	}
}

func screenshotHighlighterLogicalRect(bounds image.Rectangle, origin image.Point, scaleX, scaleY float32) Rect {
	return Rect{X: float32(bounds.Min.X-origin.X) / scaleX, Y: float32(bounds.Min.Y-origin.Y) / scaleY,
		Width: float32(bounds.Dx()) / scaleX, Height: float32(bounds.Dy()) / scaleY}
}
