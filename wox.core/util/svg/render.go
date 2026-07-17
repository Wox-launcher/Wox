package svg

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/srwiley/rasterx"
	"golang.org/x/image/math/fixed"
)

// Render parses SVG data and rasterizes it into a transparent RGBA image.
func Render(data string, width, height int) (*image.RGBA, error) {
	icon, err := Parse(strings.NewReader(data))
	if err != nil {
		return nil, err
	}
	return icon.Render(width, height)
}

// Render rasterizes a parsed icon at the requested pixel size.
func (icon *Icon) Render(width, height int) (*image.RGBA, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("SVG render size must be positive")
	}
	output := image.NewRGBA(image.Rect(0, 0, width, height))
	scanner := newWindingScanner(width, height, output)
	dasher := rasterx.NewDasher(width, height, scanner)
	target := icon.targetMatrix(width, height)

	for _, shape := range icon.shapes {
		matrix := target.Mult(shape.style.matrix)
		if !shape.style.fill.disabled {
			if err := icon.drawFill(dasher, scanner, shape, matrix, width, height); err != nil {
				return nil, err
			}
		}
		if !shape.style.stroke.disabled && shape.style.lineWidth > 0 {
			if err := icon.drawStroke(dasher, scanner, shape, matrix, width, height); err != nil {
				return nil, err
			}
		}
	}
	return output, nil
}

func (icon *Icon) drawFill(dasher *rasterx.Dasher, scanner *windingScanner, shape svgShape, matrix rasterx.Matrix2D, width, height int) error {
	scanner.Clear()
	filler := &dasher.Filler
	filler.SetWinding(shape.style.fillNonZero)
	adder := rasterx.MatrixAdder{Adder: filler, M: matrix}
	shape.path.AddTo(&adder)
	paintValue, err := icon.resolvePaint(shape.style.fill, scanner.GetPathExtent(), matrix, shape.style.opacity*shape.style.fillOpacity, width, height)
	if err != nil {
		return err
	}
	filler.SetColor(paintValue)
	filler.Draw()
	return nil
}

func (icon *Icon) drawStroke(dasher *rasterx.Dasher, scanner *windingScanner, shape svgShape, matrix rasterx.Matrix2D, width, height int) error {
	scanner.Clear()
	scaleX := math.Hypot(matrix.A, matrix.B)
	scaleY := math.Hypot(matrix.C, matrix.D)
	strokeScale := (scaleX + scaleY) / 2
	lineWidth := fixed.Int26_6(math.Round(shape.style.lineWidth * strokeScale * 64))
	if lineWidth < 1 {
		lineWidth = 1
	}
	dashes := make([]float64, len(shape.style.dashes))
	for index, dash := range shape.style.dashes {
		dashes[index] = dash * strokeScale
	}
	dasher.SetStroke(
		lineWidth,
		fixed.Int26_6(math.Round(shape.style.miterLimit*64)),
		shape.style.lineCap,
		shape.style.lineCap,
		nil,
		shape.style.lineJoin,
		dashes,
		shape.style.dashOffset*strokeScale,
	)
	adder := rasterx.MatrixAdder{Adder: dasher, M: matrix}
	shape.path.AddTo(&adder)
	paintValue, err := icon.resolvePaint(shape.style.stroke, scanner.GetPathExtent(), matrix, shape.style.opacity*shape.style.strokeOpacity, width, height)
	if err != nil {
		return err
	}
	dasher.SetColor(paintValue)
	dasher.Draw()
	return nil
}

func (icon *Icon) resolvePaint(source paint, extent fixed.Rectangle26_6, matrix rasterx.Matrix2D, opacity float64, width, height int) (interface{}, error) {
	if source.gradientID == "" {
		return applyOpacity(source.color, opacity), nil
	}
	definition := icon.gradients[source.gradientID]
	if definition == nil {
		return nil, fmt.Errorf("SVG gradient %q was not defined", source.gradientID)
	}
	gradient := *definition
	gradient.Stops = append([]rasterx.GradStop(nil), definition.Stops...)
	if gradient.Units == rasterx.ObjectBoundingBox {
		gradient.Bounds.X = float64(extent.Min.X) / 64
		gradient.Bounds.Y = float64(extent.Min.Y) / 64
		gradient.Bounds.W = float64(extent.Max.X-extent.Min.X) / 64
		gradient.Bounds.H = float64(extent.Max.Y-extent.Min.Y) / 64
	} else {
		gradient.Bounds.X = 0
		gradient.Bounds.Y = 0
		gradient.Bounds.W = float64(width)
		gradient.Bounds.H = float64(height)
	}
	if gradient.Bounds.W == 0 {
		gradient.Bounds.W = 1
	}
	if gradient.Bounds.H == 0 {
		gradient.Bounds.H = 1
	}
	return gradient.GetColorFunctionUS(opacity, matrix), nil
}

func (icon *Icon) targetMatrix(width, height int) rasterx.Matrix2D {
	scaleX := float64(width) / icon.viewBox.width
	scaleY := float64(height) / icon.viewBox.height
	parts := strings.Fields(icon.preserveAspectRatio)
	if len(parts) > 0 && strings.EqualFold(parts[0], "none") {
		return rasterx.Identity.Scale(scaleX, scaleY).Translate(-icon.viewBox.x, -icon.viewBox.y)
	}

	meet := true
	if len(parts) > 1 && strings.EqualFold(parts[1], "slice") {
		meet = false
	}
	scale := math.Max(scaleX, scaleY)
	if meet {
		scale = math.Min(scaleX, scaleY)
	}
	contentWidth := icon.viewBox.width * scale
	contentHeight := icon.viewBox.height * scale
	offsetX := 0.0
	offsetY := 0.0
	align := "xMidYMid"
	if len(parts) > 0 {
		align = parts[0]
	}
	if strings.Contains(align, "xMid") {
		offsetX = (float64(width) - contentWidth) / 2
	} else if strings.Contains(align, "xMax") {
		offsetX = float64(width) - contentWidth
	}
	if strings.Contains(align, "YMid") {
		offsetY = (float64(height) - contentHeight) / 2
	} else if strings.Contains(align, "YMax") {
		offsetY = float64(height) - contentHeight
	}
	return rasterx.Identity.Translate(offsetX, offsetY).Scale(scale, scale).Translate(-icon.viewBox.x, -icon.viewBox.y)
}

func applyOpacity(source color.Color, opacity float64) color.NRGBA {
	if source == nil {
		return color.NRGBA{}
	}
	converted := color.NRGBAModel.Convert(source).(color.NRGBA)
	converted.A = uint8(math.Round(float64(converted.A) * clampUnit(opacity)))
	return converted
}
