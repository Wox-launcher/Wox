package woxui

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"sync"
	"unicode/utf8"

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
	mosaicRadius float32
}

func drawScreenshotEditorAnnotations(displayList *DisplayList, annotations []screenshotEditorAnnotation, source *Image, frame Size) {
	for _, annotation := range annotations {
		annotationColor := screenshotEditorAnnotationDrawColor(annotation)
		switch annotation.tool {
		case screenshotEditorToolRect:
			displayList.StrokeRoundedRect(annotation.rect, 0, screenshotEditorAnnotationStroke, annotationColor)
		case screenshotEditorToolEllipse:
			drawScreenshotEditorEllipse(displayList, annotation.rect, screenshotEditorAnnotationStroke, annotationColor)
		case screenshotEditorToolArrow:
			drawScreenshotEditorArrow(displayList, annotation.start, annotation.end, screenshotEditorAnnotationStroke, annotationColor)
		case screenshotEditorToolText:
			fontSize := screenshotEditorAnnotationFontSize(annotation)
			displayList.DrawText(annotation.text, Rect{X: annotation.start.X, Y: annotation.start.Y, Width: 480, Height: fontSize + 12}, TextStyle{Size: fontSize, Weight: FontWeightSemibold}, annotationColor)
		case screenshotEditorToolMosaic:
			drawScreenshotEditorMosaicPreview(displayList, annotation.points, screenshotEditorAnnotationMosaicRadius(annotation), source, frame)
		}
	}
}

func drawScreenshotEditorAnnotationHandles(displayList *DisplayList, annotation screenshotEditorAnnotation) {
	points := []Point{}
	switch annotation.tool {
	case screenshotEditorToolRect, screenshotEditorToolEllipse:
		points = screenshotEditorRectHandlePoints(annotation.rect)
	case screenshotEditorToolArrow:
		points = []Point{annotation.start, annotation.end}
	default:
		return
	}
	color := screenshotEditorAnnotationDrawColor(annotation)
	for _, point := range points {
		displayList.FillRoundedRect(Rect{X: point.X - 6, Y: point.Y - 6, Width: 12, Height: 12}, 4, color)
	}
}

func screenshotEditorAnnotationContains(annotation screenshotEditorAnnotation, point Point) bool {
	switch annotation.tool {
	case screenshotEditorToolRect:
		return screenshotEditorRectContains(Rect{X: annotation.rect.X - 8, Y: annotation.rect.Y - 8, Width: annotation.rect.Width + 16, Height: annotation.rect.Height + 16}, point)
	case screenshotEditorToolEllipse:
		radiusX, radiusY := annotation.rect.Width/2+8, annotation.rect.Height/2+8
		if radiusX <= 0 || radiusY <= 0 {
			return false
		}
		centerX, centerY := annotation.rect.X+annotation.rect.Width/2, annotation.rect.Y+annotation.rect.Height/2
		dx, dy := (point.X-centerX)/radiusX, (point.Y-centerY)/radiusY
		return dx*dx+dy*dy <= 1
	case screenshotEditorToolArrow:
		return screenshotEditorDistanceToSegment(point, annotation.start, annotation.end) <= 10
	case screenshotEditorToolText:
		fontSize := screenshotEditorAnnotationFontSize(annotation)
		return screenshotEditorRectContains(Rect{X: annotation.start.X, Y: annotation.start.Y, Width: max(float32(24), float32(utf8.RuneCountInString(annotation.text))*fontSize*0.6), Height: fontSize + 8}, point)
	case screenshotEditorToolMosaic:
		radius := screenshotEditorAnnotationMosaicRadius(annotation)
		for _, brushPoint := range annotation.points {
			if math.Hypot(float64(point.X-brushPoint.X), float64(point.Y-brushPoint.Y)) <= float64(radius+4) {
				return true
			}
		}
	}
	return false
}

func screenshotEditorAnnotationBounds(annotation screenshotEditorAnnotation) Rect {
	switch annotation.tool {
	case screenshotEditorToolRect, screenshotEditorToolEllipse:
		return annotation.rect
	case screenshotEditorToolArrow:
		return normalizeScreenshotEditorRect(Rect{X: annotation.start.X, Y: annotation.start.Y, Width: annotation.end.X - annotation.start.X, Height: annotation.end.Y - annotation.start.Y}, Size{Width: math.MaxFloat32, Height: math.MaxFloat32})
	case screenshotEditorToolText:
		fontSize := screenshotEditorAnnotationFontSize(annotation)
		return Rect{X: annotation.start.X, Y: annotation.start.Y, Width: max(float32(24), float32(utf8.RuneCountInString(annotation.text))*fontSize*0.6), Height: fontSize + 8}
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
	drawScreenshotEditorLine(displayList, start, end, width, color)
	angle := math.Atan2(float64(end.Y-start.Y), float64(end.X-start.X))
	const headLength = 14
	left := Point{X: end.X - headLength*float32(math.Cos(angle-math.Pi/6)), Y: end.Y - headLength*float32(math.Sin(angle-math.Pi/6))}
	right := Point{X: end.X - headLength*float32(math.Cos(angle+math.Pi/6)), Y: end.Y - headLength*float32(math.Sin(angle+math.Pi/6))}
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
				pixelOffset := (pixelY*source.Width + pixelX) * 4
				color := Color{R: source.pixels[pixelOffset], G: source.pixels[pixelOffset+1], B: source.pixels[pixelOffset+2], A: 255}
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
func renderScreenshotEditorAnnotations(source image.Image, annotations []screenshotEditorAnnotation, selection Rect, frame Size) (*image.RGBA, error) {
	bounds := source.Bounds()
	output := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(output, output.Bounds(), source, bounds.Min, draw.Src)
	if len(annotations) == 0 {
		return output, nil
	}
	scaleX := float32(bounds.Dx()) / frame.Width
	scaleY := float32(bounds.Dy()) / frame.Height
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
			drawScreenshotEditorPixelLine(output, clip, rect.Min, image.Pt(rect.Max.X, rect.Min.Y), 3*scaleX, pixelColor)
			drawScreenshotEditorPixelLine(output, clip, image.Pt(rect.Max.X, rect.Min.Y), rect.Max, 3*scaleX, pixelColor)
			drawScreenshotEditorPixelLine(output, clip, rect.Max, image.Pt(rect.Min.X, rect.Max.Y), 3*scaleX, pixelColor)
			drawScreenshotEditorPixelLine(output, clip, image.Pt(rect.Min.X, rect.Max.Y), rect.Min, 3*scaleX, pixelColor)
		case screenshotEditorToolEllipse:
			drawScreenshotEditorPixelEllipse(output, clip, screenshotEditorScaleRect(annotation.rect, scaleX, scaleY), 3*scaleX, pixelColor)
		case screenshotEditorToolArrow:
			drawScreenshotEditorPixelArrow(output, clip, screenshotEditorScalePoint(annotation.start, scaleX, scaleY), screenshotEditorScalePoint(annotation.end, scaleX, scaleY), 3*scaleX, pixelColor)
		case screenshotEditorToolText:
			drawScreenshotEditorPixelText(output, clip, annotation.text, screenshotEditorScalePoint(annotation.start, scaleX, scaleY), screenshotEditorAnnotationFontSize(annotation)*scaleY, pixelColor)
		case screenshotEditorToolMosaic:
			drawScreenshotEditorPixelMosaic(output, clip, annotation.points, screenshotEditorAnnotationMosaicRadius(annotation), scaleX, scaleY)
		}
	}
	return output, nil
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
	drawScreenshotEditorPixelLine(target, clip, start, end, width, color)
	angle := math.Atan2(float64(end.Y-start.Y), float64(end.X-start.X))
	headLength := float64(width * 5)
	left := image.Pt(int(math.Round(float64(end.X)-headLength*math.Cos(angle-math.Pi/6))), int(math.Round(float64(end.Y)-headLength*math.Sin(angle-math.Pi/6))))
	right := image.Pt(int(math.Round(float64(end.X)-headLength*math.Cos(angle+math.Pi/6))), int(math.Round(float64(end.Y)-headLength*math.Sin(angle+math.Pi/6))))
	drawScreenshotEditorPixelLine(target, clip, end, left, width, color)
	drawScreenshotEditorPixelLine(target, clip, end, right, width, color)
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

func screenshotEditorAnnotationMosaicRadius(annotation screenshotEditorAnnotation) float32 {
	if annotation.mosaicRadius <= 0 {
		return screenshotEditorMosaicRadius
	}
	return annotation.mosaicRadius
}
