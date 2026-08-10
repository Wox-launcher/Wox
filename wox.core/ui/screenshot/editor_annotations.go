package screenshot

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"strconv"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	screenshotEditorAnnotationStroke = float32(3)
	screenshotEditorMosaicRadius     = float32(18)
	screenshotEditorMosaicBlockSize  = float32(12)
	screenshotEditorTextFontSize     = float32(20)
	screenshotEditorTextFramePadding = float32(4)
	screenshotEditorNumberDiameter   = float32(28)
	screenshotEditorNumberFontSize   = float32(14)
)

var (
	screenshotEditorAnnotationColor = Color{R: 255, G: 91, B: 54, A: 255}
	screenshotEditorPalette         = [...]Color{
		{R: 255, G: 91, B: 54, A: 255},
		{R: 249, G: 199, B: 79, A: 255},
		{R: 41, G: 255, B: 114, A: 255},
		{R: 77, G: 163, B: 255, A: 255},
		{R: 199, G: 125, B: 255, A: 255},
		{R: 255, G: 255, B: 255, A: 255},
	}
	screenshotEditorMosaicRadii = [...]float32{10, 18, 28}
)

var (
	screenshotEditorFontOnce sync.Once
	screenshotEditorFont     *opentype.Font
)

type screenshotEditorAnnotation struct {
	tool         screenshotEditorTool
	rect         Rect
	start        Point
	end          Point
	text         string
	points       []Point
	color        Color
	fontSize     float32
	textSize     Size
	measuredSize float32
	mosaicRadius float32
	number       int
}

func drawScreenshotEditorAnnotations(displayList *DisplayList, annotations []screenshotEditorAnnotation, source *Image, frame Size, uiScale float32) {
	strokeWidth := screenshotEditorAnnotationPreviewStroke(uiScale)
	for _, annotation := range annotations {
		annotationColor := screenshotEditorAnnotationDrawColor(annotation)
		switch annotation.tool {
		case screenshotEditorToolRect:
			displayList.StrokeRoundedRect(annotation.rect, 0, strokeWidth, annotationColor)
		case screenshotEditorToolEllipse:
			drawScreenshotEditorEllipse(displayList, annotation.rect, strokeWidth, annotationColor)
		case screenshotEditorToolArrow:
			drawScreenshotEditorArrow(displayList, annotation.start, annotation.end, strokeWidth, annotationColor)
		case screenshotEditorToolText:
			fontSize := screenshotEditorAnnotationRenderedFontSize(annotation, uiScale)
			textSize := screenshotEditorAnnotationTextSize(annotation, fontSize)
			displayList.DrawText(annotation.text, Rect{X: annotation.start.X, Y: annotation.start.Y, Width: max(float32(480), textSize.Width+8*uiScale), Height: max(fontSize+12, textSize.Height)}, TextStyle{Size: fontSize, Weight: FontWeightSemibold}, annotationColor)
		case screenshotEditorToolNumber:
			drawScreenshotEditorNumber(displayList, annotation, uiScale)
		case screenshotEditorToolMosaic:
			drawScreenshotEditorMosaicPreview(displayList, annotation.points, screenshotEditorAnnotationMosaicRadius(annotation), source, frame)
		}
	}
}

func screenshotEditorAnnotationPreviewStroke(uiScale float32) float32 {
	return screenshotEditorAnnotationStroke * max(float32(1), uiScale)
}

func drawScreenshotEditorAnnotationHandles(displayList *DisplayList, annotation screenshotEditorAnnotation, uiScale float32) {
	points := []Point{}
	switch annotation.tool {
	case screenshotEditorToolRect, screenshotEditorToolEllipse:
		points = screenshotEditorRectHandlePoints(annotation.rect)
	case screenshotEditorToolArrow:
		points = []Point{annotation.start, annotation.end}
	case screenshotEditorToolText:
		drawScreenshotEditorTextFrame(displayList, screenshotEditorTextFrame(annotation, uiScale), uiScale)
		return
	case screenshotEditorToolNumber:
		bounds := screenshotEditorNumberBounds(annotation, uiScale)
		padding := 3 * max(float32(1), uiScale)
		outline := Rect{X: bounds.X - padding, Y: bounds.Y - padding, Width: bounds.Width + 2*padding, Height: bounds.Height + 2*padding}
		displayList.StrokeRoundedRect(outline, outline.Width/2, 1.5*max(float32(1), uiScale), Color{R: 255, G: 255, B: 255, A: 255})
		return
	default:
		return
	}
	for _, point := range points {
		handleRect := Rect{X: point.X - 5*uiScale, Y: point.Y - 5*uiScale, Width: 10 * uiScale, Height: 10 * uiScale}
		displayList.StrokeRoundedRect(handleRect, 5*uiScale, 1.5*uiScale, Color{R: 255, G: 255, B: 255, A: 255})
	}
}

// drawScreenshotEditorNumber renders a compact filled marker with its sequence centered in white.
func drawScreenshotEditorNumber(displayList *DisplayList, annotation screenshotEditorAnnotation, uiScale float32) {
	bounds := screenshotEditorNumberBounds(annotation, uiScale)
	label, fontSize, textRect := screenshotEditorNumberTextLayout(annotation, uiScale)
	displayList.FillRoundedRect(bounds, bounds.Width/2, screenshotEditorAnnotationDrawColor(annotation))
	displayList.DrawText(label, textRect, TextStyle{Size: fontSize, Weight: FontWeightSemibold}, Color{R: 255, G: 255, B: 255, A: 255})
}

// screenshotEditorNumberTextLayout centers the measured platform line box on the marker center.
func screenshotEditorNumberTextLayout(annotation screenshotEditorAnnotation, uiScale float32) (string, float32, Rect) {
	label, fontSize := screenshotEditorNumberLabel(annotation, uiScale)
	textSize := annotation.textSize
	if textSize.Width <= 0 || textSize.Height <= 0 || math.Abs(float64(annotation.measuredSize-fontSize)) >= 0.01 {
		textSize = Size{Width: screenshotEditorEstimatedTextWidth(label, fontSize), Height: fontSize}
	}
	return label, fontSize, Rect{
		X: annotation.start.X - textSize.Width/2, Y: annotation.start.Y - textSize.Height/2,
		Width: textSize.Width, Height: textSize.Height,
	}
}

// screenshotEditorNumberLabel scales down three-digit markers so the sequence remains inside its circle.
func screenshotEditorNumberLabel(annotation screenshotEditorAnnotation, uiScale float32) (string, float32) {
	label := strconv.Itoa(annotation.number)
	fontSize := screenshotEditorNumberFontSize * max(float32(1), uiScale)
	if len(label) >= 3 {
		fontSize = 11 * max(float32(1), uiScale)
	}
	return label, fontSize
}

func screenshotEditorNumberBounds(annotation screenshotEditorAnnotation, uiScale float32) Rect {
	diameter := screenshotEditorNumberDiameter * max(float32(1), uiScale)
	return Rect{X: annotation.start.X - diameter/2, Y: annotation.start.Y - diameter/2, Width: diameter, Height: diameter}
}

// screenshotEditorTextFrame leaves a small gap around the editable text so its border remains a distinct drag target.
func screenshotEditorTextFrame(annotation screenshotEditorAnnotation, uiScale float32) Rect {
	padding := screenshotEditorTextFramePadding * max(float32(1), uiScale)
	bounds := screenshotEditorAnnotationBounds(annotation, uiScale)
	return Rect{X: bounds.X - padding, Y: bounds.Y - padding, Width: bounds.Width + 2*padding, Height: bounds.Height + 2*padding}
}

// screenshotEditorTextFrameBorderContains limits moving a label to the dashed perimeter instead of its editable content.
func screenshotEditorTextFrameBorderContains(annotation screenshotEditorAnnotation, point Point, uiScale float32) bool {
	frame := screenshotEditorTextFrame(annotation, uiScale)
	tolerance := 4 * max(float32(1), uiScale)
	outer := Rect{X: frame.X - tolerance, Y: frame.Y - tolerance, Width: frame.Width + 2*tolerance, Height: frame.Height + 2*tolerance}
	inner := Rect{X: frame.X + tolerance, Y: frame.Y + tolerance, Width: max(float32(0), frame.Width-2*tolerance), Height: max(float32(0), frame.Height-2*tolerance)}
	return screenshotEditorRectContains(outer, point) && !screenshotEditorRectContains(inner, point)
}

func drawScreenshotEditorTextFrame(displayList *DisplayList, frame Rect, uiScale float32) {
	scale := max(float32(1), uiScale)
	width, dash, gap := scale, 5*scale, 4*scale
	color := Color{R: 255, G: 255, B: 255, A: 230}
	for offset := float32(0); offset < frame.Width; offset += dash + gap {
		segment := min(dash, frame.Width-offset)
		displayList.FillRect(Rect{X: frame.X + offset, Y: frame.Y, Width: segment, Height: width}, color)
		displayList.FillRect(Rect{X: frame.X + offset, Y: frame.Y + frame.Height - width, Width: segment, Height: width}, color)
	}
	for offset := float32(0); offset < frame.Height; offset += dash + gap {
		segment := min(dash, frame.Height-offset)
		displayList.FillRect(Rect{X: frame.X, Y: frame.Y + offset, Width: width, Height: segment}, color)
		displayList.FillRect(Rect{X: frame.X + frame.Width - width, Y: frame.Y + offset, Width: width, Height: segment}, color)
	}
}

func screenshotEditorAnnotationContains(annotation screenshotEditorAnnotation, point Point, uiScale float32) bool {
	switch annotation.tool {
	case screenshotEditorToolRect:
		return screenshotEditorRectContains(annotation.rect, point)
	case screenshotEditorToolEllipse:
		radiusX, radiusY := annotation.rect.Width/2, annotation.rect.Height/2
		if radiusX <= 0 || radiusY <= 0 {
			return false
		}
		centerX, centerY := annotation.rect.X+annotation.rect.Width/2, annotation.rect.Y+annotation.rect.Height/2
		dx, dy := (point.X-centerX)/radiusX, (point.Y-centerY)/radiusY
		return dx*dx+dy*dy <= 1
	case screenshotEditorToolArrow:
		return screenshotEditorDistanceToSegment(point, annotation.start, annotation.end) <= screenshotEditorAnnotationStroke
	case screenshotEditorToolText:
		fontSize := screenshotEditorAnnotationRenderedFontSize(annotation, uiScale)
		textSize := screenshotEditorAnnotationTextSize(annotation, fontSize)
		return screenshotEditorRectContains(Rect{X: annotation.start.X, Y: annotation.start.Y, Width: textSize.Width, Height: textSize.Height}, point)
	case screenshotEditorToolNumber:
		bounds := screenshotEditorNumberBounds(annotation, uiScale)
		radius := bounds.Width / 2
		return math.Hypot(float64(point.X-annotation.start.X), float64(point.Y-annotation.start.Y)) <= float64(radius)
	case screenshotEditorToolMosaic:
		radius := screenshotEditorAnnotationMosaicRadius(annotation)
		for _, brushPoint := range annotation.points {
			if math.Hypot(float64(point.X-brushPoint.X), float64(point.Y-brushPoint.Y)) <= float64(radius) {
				return true
			}
		}
	}
	return false
}

// screenshotEditorAnnotationAt prefers the smallest matching mark so a large enclosing shape cannot hide nested annotations.
func screenshotEditorAnnotationAt(annotations []screenshotEditorAnnotation, point Point, uiScale float32) (int, bool) {
	return screenshotEditorAnnotationAtMatching(annotations, point, uiScale, false)
}

// screenshotEditorTextAnnotationAt applies the same overlap ordering while the text tool ignores non-text marks.
func screenshotEditorTextAnnotationAt(annotations []screenshotEditorAnnotation, point Point, uiScale float32) (int, bool) {
	return screenshotEditorAnnotationAtMatching(annotations, point, uiScale, true)
}

// screenshotEditorAnnotationAtMatching ranks overlapping hits by area, pointer distance, then visual stacking order.
func screenshotEditorAnnotationAtMatching(annotations []screenshotEditorAnnotation, point Point, uiScale float32, textOnly bool) (int, bool) {
	bestIndex := -1
	bestArea := float32(math.MaxFloat32)
	bestDistance := float32(math.MaxFloat32)
	minimumExtent := 8 * max(float32(1), uiScale)
	for index, annotation := range annotations {
		if textOnly && annotation.tool != screenshotEditorToolText {
			continue
		}
		if !screenshotEditorAnnotationContains(annotation, point, uiScale) &&
			(annotation.tool != screenshotEditorToolText || !screenshotEditorTextFrameBorderContains(annotation, point, uiScale)) {
			continue
		}

		bounds := screenshotEditorAnnotationBounds(annotation, uiScale)
		area := max(bounds.Width, minimumExtent) * max(bounds.Height, minimumExtent)
		center := Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
		distance := float32(math.Hypot(float64(point.X-center.X), float64(point.Y-center.Y)))
		if annotation.tool == screenshotEditorToolArrow {
			distance = screenshotEditorDistanceToSegment(point, annotation.start, annotation.end)
		}
		if area < bestArea || (area == bestArea && (distance < bestDistance || (distance == bestDistance && index > bestIndex))) {
			bestIndex = index
			bestArea = area
			bestDistance = distance
		}
	}
	return bestIndex, bestIndex >= 0
}

func screenshotEditorAnnotationBounds(annotation screenshotEditorAnnotation, uiScale float32) Rect {
	switch annotation.tool {
	case screenshotEditorToolRect, screenshotEditorToolEllipse:
		return annotation.rect
	case screenshotEditorToolArrow:
		return normalizeScreenshotEditorRect(Rect{X: annotation.start.X, Y: annotation.start.Y, Width: annotation.end.X - annotation.start.X, Height: annotation.end.Y - annotation.start.Y}, Size{Width: math.MaxFloat32, Height: math.MaxFloat32})
	case screenshotEditorToolText:
		fontSize := screenshotEditorAnnotationRenderedFontSize(annotation, uiScale)
		textSize := screenshotEditorAnnotationTextSize(annotation, fontSize)
		return Rect{X: annotation.start.X, Y: annotation.start.Y, Width: textSize.Width, Height: textSize.Height}
	case screenshotEditorToolNumber:
		return screenshotEditorNumberBounds(annotation, uiScale)
	case screenshotEditorToolMosaic:
		if len(annotation.points) == 0 {
			return Rect{}
		}
		left, top, right, bottom := annotation.points[0].X, annotation.points[0].Y, annotation.points[0].X, annotation.points[0].Y
		for _, point := range annotation.points[1:] {
			left, top = min(left, point.X), min(top, point.Y)
			right, bottom = max(right, point.X), max(bottom, point.Y)
		}
		radius := screenshotEditorAnnotationMosaicRadius(annotation)
		return Rect{X: left - radius, Y: top - radius, Width: right - left + 2*radius, Height: bottom - top + 2*radius}
	default:
		return Rect{}
	}
}

func screenshotEditorAnnotationTextSize(annotation screenshotEditorAnnotation, renderedFontSize float32) Size {
	if annotation.textSize.Width > 0 && annotation.textSize.Height > 0 && math.Abs(float64(annotation.measuredSize-renderedFontSize)) < 0.01 {
		return annotation.textSize
	}
	return Size{
		Width:  max(float32(24), screenshotEditorEstimatedTextWidth(annotation.text, renderedFontSize)),
		Height: renderedFontSize + 8,
	}
}

func screenshotEditorDistanceToSegment(point, start, end Point) float32 {
	dx, dy := end.X-start.X, end.Y-start.Y
	if dx == 0 && dy == 0 {
		return float32(math.Hypot(float64(point.X-start.X), float64(point.Y-start.Y)))
	}
	ratio := ((point.X-start.X)*dx + (point.Y-start.Y)*dy) / (dx*dx + dy*dy)
	ratio = min(max(float32(0), ratio), 1)
	closest := Point{X: start.X + ratio*dx, Y: start.Y + ratio*dy}
	return float32(math.Hypot(float64(point.X-closest.X), float64(point.Y-closest.Y)))
}

func drawScreenshotEditorEllipse(displayList *DisplayList, rect Rect, width float32, color Color) {
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	const segments = 32
	center := Point{X: rect.X + rect.Width/2, Y: rect.Y + rect.Height/2}
	radiusX, radiusY := rect.Width/2, rect.Height/2
	previous := Point{X: center.X + radiusX, Y: center.Y}
	for index := 1; index <= segments; index++ {
		angle := float64(index) * 2 * math.Pi / segments
		next := Point{X: center.X + radiusX*float32(math.Cos(angle)), Y: center.Y + radiusY*float32(math.Sin(angle))}
		drawScreenshotEditorLine(displayList, previous, next, width, color)
		previous = next
	}
}

func drawScreenshotEditorArrow(displayList *DisplayList, start, end Point, width float32, color Color) {
	angle := math.Atan2(float64(end.Y-start.Y), float64(end.X-start.X))
	headLength := float32(14) * width / screenshotEditorAnnotationStroke
	left := Point{X: end.X - headLength*float32(math.Cos(angle-math.Pi/6)), Y: end.Y - headLength*float32(math.Sin(angle-math.Pi/6))}
	right := Point{X: end.X - headLength*float32(math.Cos(angle+math.Pi/6)), Y: end.Y - headLength*float32(math.Sin(angle+math.Pi/6))}
	base := Point{X: (left.X + right.X) / 2, Y: (left.Y + right.Y) / 2}
	drawScreenshotEditorLine(displayList, start, base, width, color)
	displayList.FillConvexPolygon([]Point{end, left, right}, color)
}

func drawScreenshotEditorLine(displayList *DisplayList, start, end Point, width float32, color Color) {
	dx, dy := end.X-start.X, end.Y-start.Y
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length == 0 {
		displayList.FillRoundedRect(Rect{X: start.X - width/2, Y: start.Y - width/2, Width: width, Height: width}, width/2, color)
		return
	}
	nx, ny := -dy/length*width/2, dx/length*width/2
	displayList.FillConvexPolygon([]Point{
		{X: start.X + nx, Y: start.Y + ny},
		{X: end.X + nx, Y: end.Y + ny},
		{X: end.X - nx, Y: end.Y - ny},
		{X: start.X - nx, Y: start.Y - ny},
	}, color)
}

func drawScreenshotEditorMosaicPreview(displayList *DisplayList, points []Point, radius float32, source *Image, frame Size) {
	if source == nil || frame.Width <= 0 || frame.Height <= 0 {
		return
	}
	scaleX := float32(source.Width) / frame.Width
	scaleY := float32(source.Height) / frame.Height
	steps := max(1, int(math.Ceil(float64(radius/screenshotEditorMosaicBlockSize))))
	for _, point := range points {
		for row := -steps; row <= steps; row++ {
			for column := -steps; column <= steps; column++ {
				offsetX := float32(column) * screenshotEditorMosaicBlockSize
				offsetY := float32(row) * screenshotEditorMosaicBlockSize
				if offsetX*offsetX+offsetY*offsetY > radius*radius {
					continue
				}
				logicalX := point.X + offsetX
				logicalY := point.Y + offsetY
				pixelX := min(max(0, int(logicalX*scaleX)), source.Width-1)
				pixelY := min(max(0, int(logicalY*scaleY)), source.Height-1)
				pixel := source.RGBAAt(pixelX, pixelY)
				color := Color{R: pixel.R, G: pixel.G, B: pixel.B, A: 255}
				displayList.FillRect(Rect{
					X:     logicalX - screenshotEditorMosaicBlockSize/2,
					Y:     logicalY - screenshotEditorMosaicBlockSize/2,
					Width: screenshotEditorMosaicBlockSize, Height: screenshotEditorMosaicBlockSize,
				}, color)
			}
		}
	}
}

// renderScreenshotEditorAnnotations composites editor marks into the captured desktop pixels.
func renderScreenshotEditorAnnotations(source image.Image, annotations []screenshotEditorAnnotation, selection Rect, frame Size, uiScale float32) (*image.RGBA, error) {
	bounds := source.Bounds()
	output := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(output, output.Bounds(), source, bounds.Min, draw.Src)
	if len(annotations) == 0 {
		return output, nil
	}
	scaleX := float32(bounds.Dx()) / frame.Width
	scaleY := float32(bounds.Dy()) / frame.Height
	previewScale := max(float32(1), uiScale)
	strokeWidth := screenshotEditorAnnotationStroke * previewScale * scaleX
	clip, err := screenshotEditorPixelSelection(bounds, selection, frame)
	if err != nil {
		return nil, err
	}
	for _, annotation := range annotations {
		drawColor := screenshotEditorAnnotationDrawColor(annotation)
		pixelColor := color.RGBA{R: drawColor.R, G: drawColor.G, B: drawColor.B, A: drawColor.A}
		switch annotation.tool {
		case screenshotEditorToolRect:
			rect := screenshotEditorScaleRect(annotation.rect, scaleX, scaleY)
			drawScreenshotEditorPixelLine(output, clip, rect.Min, image.Pt(rect.Max.X, rect.Min.Y), strokeWidth, pixelColor)
			drawScreenshotEditorPixelLine(output, clip, image.Pt(rect.Max.X, rect.Min.Y), rect.Max, strokeWidth, pixelColor)
			drawScreenshotEditorPixelLine(output, clip, rect.Max, image.Pt(rect.Min.X, rect.Max.Y), strokeWidth, pixelColor)
			drawScreenshotEditorPixelLine(output, clip, image.Pt(rect.Min.X, rect.Max.Y), rect.Min, strokeWidth, pixelColor)
		case screenshotEditorToolEllipse:
			drawScreenshotEditorPixelEllipse(output, clip, screenshotEditorScaleRect(annotation.rect, scaleX, scaleY), strokeWidth, pixelColor)
		case screenshotEditorToolArrow:
			drawScreenshotEditorPixelArrow(output, clip, screenshotEditorScalePoint(annotation.start, scaleX, scaleY), screenshotEditorScalePoint(annotation.end, scaleX, scaleY), strokeWidth, pixelColor)
		case screenshotEditorToolText:
			drawScreenshotEditorPixelText(output, clip, annotation.text, screenshotEditorScalePoint(annotation.start, scaleX, scaleY), screenshotEditorAnnotationRenderedFontSize(annotation, previewScale)*scaleY, pixelColor)
		case screenshotEditorToolNumber:
			drawScreenshotEditorPixelNumber(output, clip, annotation, previewScale, scaleX, scaleY, pixelColor)
		case screenshotEditorToolMosaic:
			drawScreenshotEditorPixelMosaic(output, clip, annotation.points, screenshotEditorAnnotationMosaicRadius(annotation), scaleX, scaleY)
		}
	}
	return output, nil
}

// drawScreenshotEditorPixelNumber preserves the marker's logical size and centered label in the exported image.
func drawScreenshotEditorPixelNumber(target *image.RGBA, clip image.Rectangle, annotation screenshotEditorAnnotation, previewScale, scaleX, scaleY float32, fill color.RGBA) {
	center := screenshotEditorScalePoint(annotation.start, scaleX, scaleY)
	diameter := screenshotEditorNumberDiameter * previewScale * min(scaleX, scaleY)
	drawScreenshotEditorPixelLine(target, clip, center, center, diameter, fill)
	label := strconv.Itoa(annotation.number)
	fontSize := screenshotEditorNumberFontSize * previewScale * scaleY
	if len(label) >= 3 {
		fontSize = 11 * previewScale * scaleY
	}
	drawScreenshotEditorPixelCenteredText(target, clip, label, center, fontSize, color.RGBA{R: 255, G: 255, B: 255, A: 255})
}

// drawScreenshotEditorPixelCenteredText centers the actual glyph ink rather than its nominal font box.
func drawScreenshotEditorPixelCenteredText(target *image.RGBA, clip image.Rectangle, text string, center image.Point, size float32, textColor color.RGBA) {
	parsed := screenshotEditorExportFont()
	if parsed == nil {
		return
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return
	}
	defer face.Close()
	bounds, _ := font.BoundString(face, text)
	dot := fixed.Point26_6{
		X: fixed.I(center.X) - (bounds.Min.X+bounds.Max.X)/2,
		Y: fixed.I(center.Y) - (bounds.Min.Y+bounds.Max.Y)/2,
	}
	clipped := target.SubImage(clip.Intersect(target.Bounds())).(*image.RGBA)
	drawer := font.Drawer{Dst: clipped, Src: image.NewUniform(textColor), Face: face, Dot: dot}
	drawer.DrawString(text)
}

// renderScreenshotEditorCursor composites the capture-time pointer at its source-pixel hotspot.
func renderScreenshotEditorCursor(target *image.RGBA, cursorPixel Point, selection Rect, frame Size, captured *screenshotEditorCapturedCursor) error {
	if target == nil || frame.Width <= 0 || frame.Height <= 0 {
		return nil
	}
	scaleX := float32(target.Bounds().Dx()) / frame.Width
	scaleY := float32(target.Bounds().Dy()) / frame.Height
	clip, err := screenshotEditorPixelSelection(target.Bounds(), selection, frame)
	if err != nil {
		return err
	}
	width := max(1, int(math.Round(float64(screenshotEditorCursorWidth*scaleX))))
	height := max(1, int(math.Round(float64(screenshotEditorCursorHeight*scaleY))))
	hotspotX := screenshotEditorCursorHotspotX * scaleX
	hotspotY := screenshotEditorCursorHotspotY * scaleY
	var cursor image.Image
	if captured != nil && captured.raster != nil {
		cursor = captured.raster
		width = captured.raster.Bounds().Dx()
		height = captured.raster.Bounds().Dy()
		hotspotX = captured.hotspot.X
		hotspotY = captured.hotspot.Y
	} else {
		fallback, renderErr := renderScreenshotEditorCursorImage(width, height)
		if renderErr != nil {
			return renderErr
		}
		cursor = fallback
	}
	left := int(math.Round(float64(cursorPixel.X - hotspotX)))
	top := int(math.Round(float64(cursorPixel.Y - hotspotY)))
	destination := image.Rect(left, top, left+width, top+height).Intersect(clip).Intersect(target.Bounds())
	if destination.Empty() {
		return nil
	}
	draw.Draw(target, destination, cursor, image.Pt(destination.Min.X-left, destination.Min.Y-top), draw.Over)
	return nil
}

func screenshotEditorScalePoint(point Point, scaleX, scaleY float32) image.Point {
	return image.Pt(int(math.Round(float64(point.X*scaleX))), int(math.Round(float64(point.Y*scaleY))))
}

func screenshotEditorScaleRect(rect Rect, scaleX, scaleY float32) image.Rectangle {
	return image.Rect(
		int(math.Round(float64(rect.X*scaleX))),
		int(math.Round(float64(rect.Y*scaleY))),
		int(math.Round(float64((rect.X+rect.Width)*scaleX))),
		int(math.Round(float64((rect.Y+rect.Height)*scaleY))),
	)
}

func drawScreenshotEditorPixelLine(target *image.RGBA, clip image.Rectangle, start, end image.Point, width float32, color color.RGBA) {
	dx, dy := float64(end.X-start.X), float64(end.Y-start.Y)
	steps := int(math.Max(math.Abs(dx), math.Abs(dy)))
	if steps < 1 {
		steps = 1
	}
	radius := max(1, int(math.Ceil(float64(width/2))))
	for step := 0; step <= steps; step++ {
		ratio := float64(step) / float64(steps)
		x := int(math.Round(float64(start.X) + dx*ratio))
		y := int(math.Round(float64(start.Y) + dy*ratio))
		for offsetY := -radius; offsetY <= radius; offsetY++ {
			for offsetX := -radius; offsetX <= radius; offsetX++ {
				point := image.Pt(x+offsetX, y+offsetY)
				if offsetX*offsetX+offsetY*offsetY <= radius*radius && point.In(clip) && point.In(target.Bounds()) {
					target.SetRGBA(point.X, point.Y, color)
				}
			}
		}
	}
}

func drawScreenshotEditorPixelEllipse(target *image.RGBA, clip, rect image.Rectangle, width float32, color color.RGBA) {
	centerX, centerY := float64(rect.Min.X+rect.Max.X)/2, float64(rect.Min.Y+rect.Max.Y)/2
	radiusX, radiusY := float64(rect.Dx())/2, float64(rect.Dy())/2
	previous := image.Pt(int(math.Round(centerX+radiusX)), int(math.Round(centerY)))
	for index := 1; index <= 96; index++ {
		angle := float64(index) * 2 * math.Pi / 96
		next := image.Pt(int(math.Round(centerX+radiusX*math.Cos(angle))), int(math.Round(centerY+radiusY*math.Sin(angle))))
		drawScreenshotEditorPixelLine(target, clip, previous, next, width, color)
		previous = next
	}
}

func drawScreenshotEditorPixelArrow(target *image.RGBA, clip image.Rectangle, start, end image.Point, width float32, color color.RGBA) {
	angle := math.Atan2(float64(end.Y-start.Y), float64(end.X-start.X))
	headLength := float64(width) * 14 / float64(screenshotEditorAnnotationStroke)
	left := image.Pt(int(math.Round(float64(end.X)-headLength*math.Cos(angle-math.Pi/6))), int(math.Round(float64(end.Y)-headLength*math.Sin(angle-math.Pi/6))))
	right := image.Pt(int(math.Round(float64(end.X)-headLength*math.Cos(angle+math.Pi/6))), int(math.Round(float64(end.Y)-headLength*math.Sin(angle+math.Pi/6))))
	base := image.Pt((left.X+right.X)/2, (left.Y+right.Y)/2)
	drawScreenshotEditorPixelLine(target, clip, start, base, width, color)
	drawScreenshotEditorPixelTriangle(target, clip, end, left, right, color)
}

func drawScreenshotEditorPixelTriangle(target *image.RGBA, clip image.Rectangle, first, second, third image.Point, color color.RGBA) {
	bounds := image.Rect(
		min(first.X, second.X, third.X), min(first.Y, second.Y, third.Y),
		max(first.X, second.X, third.X)+1, max(first.Y, second.Y, third.Y)+1,
	).Intersect(clip).Intersect(target.Bounds())
	edge := func(left, right, point image.Point) int {
		return (point.X-left.X)*(right.Y-left.Y) - (point.Y-left.Y)*(right.X-left.X)
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			point := image.Pt(x, y)
			a, b, c := edge(first, second, point), edge(second, third, point), edge(third, first, point)
			if (a >= 0 && b >= 0 && c >= 0) || (a <= 0 && b <= 0 && c <= 0) {
				target.SetRGBA(x, y, color)
			}
		}
	}
}

func drawScreenshotEditorPixelText(target *image.RGBA, clip image.Rectangle, text string, position image.Point, size float32, color color.RGBA) {
	if text == "" {
		return
	}
	parsed := screenshotEditorExportFont()
	if parsed == nil {
		return
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return
	}
	defer face.Close()
	clipped := target.SubImage(clip.Intersect(target.Bounds())).(*image.RGBA)
	drawer := font.Drawer{Dst: clipped, Src: image.NewUniform(color), Face: face, Dot: fixed.P(position.X, position.Y+int(size))}
	drawer.DrawString(text)
}

func screenshotEditorExportFont() *opentype.Font {
	screenshotEditorFontOnce.Do(func() {
		data, err := os.ReadFile("/System/Library/Fonts/Supplemental/Arial Unicode.ttf")
		if err != nil {
			data = goregular.TTF
		}
		screenshotEditorFont, _ = opentype.Parse(data)
	})
	return screenshotEditorFont
}

func drawScreenshotEditorPixelMosaic(target *image.RGBA, clip image.Rectangle, points []Point, logicalRadius, scaleX, scaleY float32) {
	block := max(2, int(math.Round(float64(screenshotEditorMosaicBlockSize*scaleX))))
	radius := max(block, int(math.Round(float64(logicalRadius*scaleX))))
	for _, logicalPoint := range points {
		point := screenshotEditorScalePoint(logicalPoint, scaleX, scaleY)
		for y := point.Y - radius; y <= point.Y+radius; y += block {
			for x := point.X - radius; x <= point.X+radius; x += block {
				if (x-point.X)*(x-point.X)+(y-point.Y)*(y-point.Y) > radius*radius {
					continue
				}
				sample := image.Pt(x+block/2, y+block/2)
				if !sample.In(clip) || !sample.In(target.Bounds()) {
					continue
				}
				fill := image.Rect(x, y, x+block, y+block).Intersect(clip).Intersect(target.Bounds())
				draw.Draw(target, fill, image.NewUniform(target.RGBAAt(sample.X, sample.Y)), image.Point{}, draw.Src)
			}
		}
	}
}

func screenshotEditorAnnotationDrawColor(annotation screenshotEditorAnnotation) Color {
	if annotation.color.A == 0 {
		return screenshotEditorAnnotationColor
	}
	return annotation.color
}

func screenshotEditorAnnotationFontSize(annotation screenshotEditorAnnotation) float32 {
	if annotation.fontSize <= 0 {
		return screenshotEditorTextFontSize
	}
	return annotation.fontSize
}

func screenshotEditorAnnotationRenderedFontSize(annotation screenshotEditorAnnotation, uiScale float32) float32 {
	return screenshotEditorAnnotationFontSize(annotation) * max(float32(1), uiScale)
}

func screenshotEditorAnnotationMosaicRadius(annotation screenshotEditorAnnotation) float32 {
	if annotation.mosaicRadius <= 0 {
		return screenshotEditorMosaicRadius
	}
	return annotation.mosaicRadius
}
