package svg

import (
	"encoding/xml"
	"fmt"
	"image/color"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/srwiley/rasterx"
	"golang.org/x/image/colornames"
)

type paint struct {
	color      color.Color
	gradientID string
	disabled   bool
}

type pathStyle struct {
	fill          paint
	stroke        paint
	currentColor  color.Color
	opacity       float64
	fillOpacity   float64
	strokeOpacity float64
	fillNonZero   bool
	lineWidth     float64
	miterLimit    float64
	lineCap       rasterx.CapFunc
	lineJoin      rasterx.JoinMode
	dashes        []float64
	dashOffset    float64
	matrix        rasterx.Matrix2D
}

type svgShape struct {
	path  rasterx.Path
	style pathStyle
}

type viewBox struct {
	x, y, width, height float64
}

// Icon is a parsed SVG document that can be rendered repeatedly at different sizes.
type Icon struct {
	viewBox             viewBox
	preserveAspectRatio string
	shapes              []svgShape
	gradients           map[string]*rasterx.Gradient
}

// Parse reads the SVG subset used by Wox icons into a reusable document.
func Parse(reader io.Reader) (*Icon, error) {
	icon := &Icon{gradients: map[string]*rasterx.Gradient{}, preserveAspectRatio: "xMidYMid meet"}
	styles := []pathStyle{defaultPathStyle()}
	decoder := xml.NewDecoder(reader)
	defsDepth := 0
	var currentGradient *rasterx.Gradient

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse SVG XML: %w", err)
		}
		switch element := token.(type) {
		case xml.StartElement:
			attributes := attributeMap(element.Attr)
			style, err := applyStyle(styles[len(styles)-1], attributes)
			if err != nil {
				return nil, fmt.Errorf("parse <%s> style: %w", element.Name.Local, err)
			}
			styles = append(styles, style)

			switch element.Name.Local {
			case "svg":
				if err := icon.readRoot(attributes); err != nil {
					return nil, err
				}
			case "defs":
				defsDepth++
			case "linearGradient":
				gradient, id, err := readLinearGradient(attributes)
				if err != nil {
					return nil, err
				}
				if id != "" {
					icon.gradients[id] = gradient
				}
				currentGradient = gradient
			case "radialGradient":
				gradient, id, err := readRadialGradient(attributes)
				if err != nil {
					return nil, err
				}
				if id != "" {
					icon.gradients[id] = gradient
				}
				currentGradient = gradient
			case "stop":
				if currentGradient != nil {
					stop, err := readGradientStop(attributes)
					if err != nil {
						return nil, err
					}
					currentGradient.Stops = append(currentGradient.Stops, stop)
				}
			default:
				if defsDepth == 0 {
					path, recognized, err := readShape(element.Name.Local, attributes)
					if err != nil {
						return nil, fmt.Errorf("parse <%s>: %w", element.Name.Local, err)
					}
					if recognized && len(path) > 0 {
						icon.shapes = append(icon.shapes, svgShape{path: path, style: style})
					}
				}
			}
		case xml.EndElement:
			switch element.Name.Local {
			case "defs":
				defsDepth--
			case "linearGradient", "radialGradient":
				currentGradient = nil
			}
			if len(styles) > 1 {
				styles = styles[:len(styles)-1]
			}
		}
	}

	if icon.viewBox.width <= 0 || icon.viewBox.height <= 0 {
		return nil, fmt.Errorf("SVG requires a positive viewBox or width and height")
	}
	return icon, nil
}

func defaultPathStyle() pathStyle {
	return pathStyle{
		fill:          paint{color: color.NRGBA{A: 255}},
		stroke:        paint{disabled: true},
		currentColor:  color.NRGBA{A: 255},
		opacity:       1,
		fillOpacity:   1,
		strokeOpacity: 1,
		fillNonZero:   true,
		lineWidth:     1,
		miterLimit:    4,
		lineCap:       rasterx.ButtCap,
		lineJoin:      rasterx.Miter,
		matrix:        rasterx.Identity,
	}
}

func (icon *Icon) readRoot(attributes map[string]string) error {
	if value := attributes["viewbox"]; value != "" {
		values, err := parseNumberList(value)
		if err != nil || len(values) != 4 {
			return fmt.Errorf("invalid SVG viewBox %q", value)
		}
		icon.viewBox = viewBox{x: values[0], y: values[1], width: values[2], height: values[3]}
	}
	if icon.viewBox.width == 0 {
		icon.viewBox.width, _ = parseLength(attributes["width"])
	}
	if icon.viewBox.height == 0 {
		icon.viewBox.height, _ = parseLength(attributes["height"])
	}
	if value := strings.TrimSpace(attributes["preserveaspectratio"]); value != "" {
		icon.preserveAspectRatio = value
	}
	return nil
}

func attributeMap(attributes []xml.Attr) map[string]string {
	result := make(map[string]string, len(attributes))
	for _, attribute := range attributes {
		result[strings.ToLower(attribute.Name.Local)] = strings.TrimSpace(attribute.Value)
	}
	return result
}

func applyStyle(base pathStyle, attributes map[string]string) (pathStyle, error) {
	properties := make(map[string]string, len(attributes))
	for key, value := range attributes {
		properties[key] = value
	}
	if inline := attributes["style"]; inline != "" {
		for _, pair := range strings.Split(inline, ";") {
			parts := strings.SplitN(pair, ":", 2)
			if len(parts) == 2 {
				properties[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
			}
		}
	}

	style := base
	if value := properties["color"]; value != "" {
		parsed, err := parseColor(value)
		if err != nil {
			return style, err
		}
		style.currentColor = parsed
	}
	if value, ok := properties["fill"]; ok {
		parsed, err := parsePaint(value, style.currentColor)
		if err != nil {
			return style, err
		}
		style.fill = parsed
	}
	if value, ok := properties["stroke"]; ok {
		parsed, err := parsePaint(value, style.currentColor)
		if err != nil {
			return style, err
		}
		style.stroke = parsed
	}
	if value := strings.ToLower(properties["fill-rule"]); value != "" {
		style.fillNonZero = value != "evenodd"
	}
	if value := properties["opacity"]; value != "" {
		opacity, err := parseFraction(value)
		if err != nil {
			return style, err
		}
		style.opacity *= clampUnit(opacity)
	}
	if value := properties["fill-opacity"]; value != "" {
		opacity, err := parseFraction(value)
		if err != nil {
			return style, err
		}
		style.fillOpacity = clampUnit(opacity)
	}
	if value := properties["stroke-opacity"]; value != "" {
		opacity, err := parseFraction(value)
		if err != nil {
			return style, err
		}
		style.strokeOpacity = clampUnit(opacity)
	}
	if value := properties["stroke-width"]; value != "" {
		width, err := parseLength(value)
		if err != nil {
			return style, err
		}
		style.lineWidth = width
	}
	if value := properties["stroke-miterlimit"]; value != "" {
		limit, err := parseLength(value)
		if err != nil {
			return style, err
		}
		style.miterLimit = limit
	}
	if value := strings.ToLower(properties["stroke-linecap"]); value != "" {
		switch value {
		case "round":
			style.lineCap = rasterx.RoundCap
		case "square":
			style.lineCap = rasterx.SquareCap
		default:
			style.lineCap = rasterx.ButtCap
		}
	}
	if value := strings.ToLower(properties["stroke-linejoin"]); value != "" {
		switch value {
		case "round":
			style.lineJoin = rasterx.Round
		case "bevel":
			style.lineJoin = rasterx.Bevel
		default:
			style.lineJoin = rasterx.Miter
		}
	}
	if value := properties["stroke-dasharray"]; value != "" {
		if strings.EqualFold(value, "none") {
			style.dashes = nil
		} else {
			dashes, err := parseNumberList(value)
			if err != nil {
				return style, err
			}
			style.dashes = dashes
		}
	}
	if value := properties["stroke-dashoffset"]; value != "" {
		offset, err := parseLength(value)
		if err != nil {
			return style, err
		}
		style.dashOffset = offset
	}
	if value := properties["transform"]; value != "" {
		matrix, err := parseTransform(style.matrix, value)
		if err != nil {
			return style, err
		}
		style.matrix = matrix
	}
	return style, nil
}

func parsePaint(value string, currentColor color.Color) (paint, error) {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "none") || value == "" {
		return paint{disabled: true}, nil
	}
	if strings.EqualFold(value, "currentColor") {
		return paint{color: currentColor}, nil
	}
	if strings.HasPrefix(strings.ToLower(value), "url(") && strings.HasSuffix(value, ")") {
		reference := strings.TrimSpace(value[4 : len(value)-1])
		if strings.HasPrefix(reference, "#") {
			return paint{gradientID: reference[1:]}, nil
		}
		return paint{}, fmt.Errorf("unsupported paint reference %q", value)
	}
	parsed, err := parseColor(value)
	if err != nil {
		return paint{}, err
	}
	return paint{color: parsed}, nil
}

func parseColor(value string) (color.Color, error) {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if lower == "transparent" {
		return color.NRGBA{}, nil
	}
	if named, ok := colornames.Map[lower]; ok {
		return named, nil
	}
	if strings.HasPrefix(value, "#") {
		hex := value[1:]
		if len(hex) == 3 || len(hex) == 4 {
			expanded := make([]byte, 0, len(hex)*2)
			for index := range hex {
				expanded = append(expanded, hex[index], hex[index])
			}
			hex = string(expanded)
		}
		if len(hex) != 6 && len(hex) != 8 {
			return nil, fmt.Errorf("invalid SVG color %q", value)
		}
		parsed, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid SVG color %q", value)
		}
		if len(hex) == 6 {
			return color.NRGBA{R: uint8(parsed >> 16), G: uint8(parsed >> 8), B: uint8(parsed), A: 255}, nil
		}
		return color.NRGBA{R: uint8(parsed >> 24), G: uint8(parsed >> 16), B: uint8(parsed >> 8), A: uint8(parsed)}, nil
	}
	if strings.HasPrefix(lower, "rgb(") || strings.HasPrefix(lower, "rgba(") {
		open := strings.IndexByte(value, '(')
		components := strings.FieldsFunc(value[open+1:len(value)-1], func(r rune) bool { return r == ',' || r == ' ' || r == '/' })
		if len(components) != 3 && len(components) != 4 {
			return nil, fmt.Errorf("invalid SVG color %q", value)
		}
		channels := [4]uint8{0, 0, 0, 255}
		for index := 0; index < 3; index++ {
			channel, err := parseColorChannel(components[index])
			if err != nil {
				return nil, err
			}
			channels[index] = channel
		}
		if len(components) == 4 {
			alpha, err := parseFraction(components[3])
			if err != nil {
				return nil, err
			}
			channels[3] = uint8(math.Round(clampUnit(alpha) * 255))
		}
		return color.NRGBA{R: channels[0], G: channels[1], B: channels[2], A: channels[3]}, nil
	}
	return nil, fmt.Errorf("unsupported SVG color %q", value)
}

func parseColorChannel(value string) (uint8, error) {
	if strings.HasSuffix(value, "%") {
		fraction, err := parseFraction(value)
		return uint8(math.Round(clampUnit(fraction) * 255)), err
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	return uint8(math.Round(math.Max(0, math.Min(255, parsed)))), nil
}

func readShape(tag string, attributes map[string]string) (rasterx.Path, bool, error) {
	var path rasterx.Path
	switch tag {
	case "path":
		compiled, err := parsePath(attributes["d"])
		return compiled, true, err
	case "rect":
		x, err := parseLength(attributes["x"])
		if err != nil {
			return nil, true, err
		}
		y, err := parseLength(attributes["y"])
		if err != nil {
			return nil, true, err
		}
		width, err := parseLength(attributes["width"])
		if err != nil {
			return nil, true, err
		}
		height, err := parseLength(attributes["height"])
		if err != nil {
			return nil, true, err
		}
		if width <= 0 || height <= 0 {
			return nil, true, nil
		}
		rx, _ := parseLength(attributes["rx"])
		ry, _ := parseLength(attributes["ry"])
		if rx == 0 {
			rx = ry
		}
		if ry == 0 {
			ry = rx
		}
		rx = math.Min(math.Abs(rx), width/2)
		ry = math.Min(math.Abs(ry), height/2)
		rasterx.AddRoundRect(x, y, x+width, y+height, rx, ry, 0, rasterx.RoundGap, &path)
		return path, true, nil
	case "circle", "ellipse":
		cx, err := parseLength(attributes["cx"])
		if err != nil {
			return nil, true, err
		}
		cy, err := parseLength(attributes["cy"])
		if err != nil {
			return nil, true, err
		}
		rx, _ := parseLength(attributes["rx"])
		ry, _ := parseLength(attributes["ry"])
		if radius := attributes["r"]; radius != "" {
			rx, err = parseLength(radius)
			if err != nil {
				return nil, true, err
			}
			ry = rx
		}
		if rx <= 0 || ry <= 0 {
			return nil, true, nil
		}
		rasterx.AddEllipse(cx, cy, rx, ry, 0, &path)
		return path, true, nil
	case "line":
		x1, _ := parseLength(attributes["x1"])
		y1, _ := parseLength(attributes["y1"])
		x2, _ := parseLength(attributes["x2"])
		y2, _ := parseLength(attributes["y2"])
		path.Start(toFixedPoint(x1, y1))
		path.Line(toFixedPoint(x2, y2))
		return path, true, nil
	case "polyline", "polygon":
		points, err := parseNumberList(attributes["points"])
		if err != nil {
			return nil, true, err
		}
		if len(points)%2 != 0 {
			return nil, true, fmt.Errorf("points requires coordinate pairs")
		}
		if len(points) < 4 {
			return nil, true, nil
		}
		path.Start(toFixedPoint(points[0], points[1]))
		for index := 2; index < len(points); index += 2 {
			path.Line(toFixedPoint(points[index], points[index+1]))
		}
		if tag == "polygon" {
			path.Stop(true)
		}
		return path, true, nil
	default:
		return nil, false, nil
	}
}

func readLinearGradient(attributes map[string]string) (*rasterx.Gradient, string, error) {
	gradient := &rasterx.Gradient{Points: [5]float64{0, 0, 1, 0, 0}, Matrix: rasterx.Identity}
	var err error
	for index, key := range []string{"x1", "y1", "x2", "y2"} {
		if value := attributes[key]; value != "" {
			gradient.Points[index], err = parseFraction(value)
			if err != nil {
				return nil, "", err
			}
		}
	}
	if err := applyGradientAttributes(gradient, attributes); err != nil {
		return nil, "", err
	}
	return gradient, attributes["id"], nil
}

func readRadialGradient(attributes map[string]string) (*rasterx.Gradient, string, error) {
	gradient := &rasterx.Gradient{Points: [5]float64{0.5, 0.5, 0.5, 0.5, 0.5}, Matrix: rasterx.Identity, IsRadial: true}
	var err error
	for index, key := range []string{"cx", "cy", "fx", "fy", "r"} {
		if value := attributes[key]; value != "" {
			gradient.Points[index], err = parseFraction(value)
			if err != nil {
				return nil, "", err
			}
		}
	}
	if attributes["fx"] == "" {
		gradient.Points[2] = gradient.Points[0]
	}
	if attributes["fy"] == "" {
		gradient.Points[3] = gradient.Points[1]
	}
	if err := applyGradientAttributes(gradient, attributes); err != nil {
		return nil, "", err
	}
	return gradient, attributes["id"], nil
}

func applyGradientAttributes(gradient *rasterx.Gradient, attributes map[string]string) error {
	if strings.EqualFold(attributes["gradientunits"], "userSpaceOnUse") {
		gradient.Units = rasterx.UserSpaceOnUse
	}
	switch strings.ToLower(attributes["spreadmethod"]) {
	case "reflect":
		gradient.Spread = rasterx.ReflectSpread
	case "repeat":
		gradient.Spread = rasterx.RepeatSpread
	default:
		gradient.Spread = rasterx.PadSpread
	}
	if value := attributes["gradienttransform"]; value != "" {
		matrix, err := parseTransform(rasterx.Identity, value)
		if err != nil {
			return err
		}
		gradient.Matrix = matrix
	}
	return nil
}

func readGradientStop(attributes map[string]string) (rasterx.GradStop, error) {
	properties := map[string]string{}
	for key, value := range attributes {
		properties[key] = value
	}
	if inline := attributes["style"]; inline != "" {
		for _, pair := range strings.Split(inline, ";") {
			parts := strings.SplitN(pair, ":", 2)
			if len(parts) == 2 {
				properties[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
			}
		}
	}
	stop := rasterx.GradStop{StopColor: color.Black, Opacity: 1}
	if value := properties["offset"]; value != "" {
		offset, err := parseFraction(value)
		if err != nil {
			return stop, err
		}
		stop.Offset = clampUnit(offset)
	}
	if value := properties["stop-color"]; value != "" {
		parsed, err := parseColor(value)
		if err != nil {
			return stop, err
		}
		stop.StopColor = parsed
	}
	if value := properties["stop-opacity"]; value != "" {
		opacity, err := parseFraction(value)
		if err != nil {
			return stop, err
		}
		stop.Opacity = clampUnit(opacity)
	}
	return stop, nil
}

func parseTransform(base rasterx.Matrix2D, value string) (rasterx.Matrix2D, error) {
	matrix := base
	remaining := strings.TrimSpace(value)
	for remaining != "" {
		open := strings.IndexByte(remaining, '(')
		close := strings.IndexByte(remaining, ')')
		if open <= 0 || close <= open {
			return matrix, fmt.Errorf("invalid transform %q", value)
		}
		name := strings.ToLower(strings.TrimSpace(remaining[:open]))
		arguments, err := parseNumberList(remaining[open+1 : close])
		if err != nil {
			return matrix, err
		}
		switch name {
		case "matrix":
			if len(arguments) != 6 {
				return matrix, fmt.Errorf("matrix transform requires 6 values")
			}
			matrix = matrix.Mult(rasterx.Matrix2D{A: arguments[0], B: arguments[1], C: arguments[2], D: arguments[3], E: arguments[4], F: arguments[5]})
		case "translate":
			if len(arguments) < 1 || len(arguments) > 2 {
				return matrix, fmt.Errorf("translate transform requires 1 or 2 values")
			}
			y := 0.0
			if len(arguments) == 2 {
				y = arguments[1]
			}
			matrix = matrix.Translate(arguments[0], y)
		case "scale":
			if len(arguments) < 1 || len(arguments) > 2 {
				return matrix, fmt.Errorf("scale transform requires 1 or 2 values")
			}
			y := arguments[0]
			if len(arguments) == 2 {
				y = arguments[1]
			}
			matrix = matrix.Scale(arguments[0], y)
		case "rotate":
			if len(arguments) != 1 && len(arguments) != 3 {
				return matrix, fmt.Errorf("rotate transform requires 1 or 3 values")
			}
			angle := arguments[0] * math.Pi / 180
			if len(arguments) == 1 {
				matrix = matrix.Rotate(angle)
			} else {
				matrix = matrix.Translate(arguments[1], arguments[2]).Rotate(angle).Translate(-arguments[1], -arguments[2])
			}
		case "skewx":
			if len(arguments) != 1 {
				return matrix, fmt.Errorf("skewX transform requires 1 value")
			}
			matrix = matrix.SkewX(arguments[0] * math.Pi / 180)
		case "skewy":
			if len(arguments) != 1 {
				return matrix, fmt.Errorf("skewY transform requires 1 value")
			}
			matrix = matrix.SkewY(arguments[0] * math.Pi / 180)
		default:
			return matrix, fmt.Errorf("unsupported transform %q", name)
		}
		remaining = strings.TrimSpace(remaining[close+1:])
	}
	return matrix, nil
}

func parseNumberList(value string) ([]float64, error) {
	parser := &pathParser{data: value}
	values := []float64{}
	for parser.hasNumber() {
		parsed, err := parser.readNumber()
		if err != nil {
			return nil, err
		}
		values = append(values, parsed)
	}
	parser.skipSeparators()
	if parser.position != len(value) {
		return nil, fmt.Errorf("invalid number list %q", value)
	}
	return values, nil
}

func parseLength(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	for _, suffix := range []string{"px", "pt", "pc", "cm", "mm", "in", "em", "ex"} {
		value = strings.TrimSuffix(value, suffix)
	}
	return strconv.ParseFloat(value, 64)
}

func parseFraction(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, "%") {
		parsed, err := strconv.ParseFloat(strings.TrimSuffix(value, "%"), 64)
		return parsed / 100, err
	}
	return parseLength(value)
}

func clampUnit(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}
