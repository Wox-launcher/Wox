package common

import (
	"fmt"
	"strings"
)

// Theme swatch geometry matches the Settings theme catalog icon.
const (
	ThemeSwatchSize                float32 = 32
	ThemeSwatchRadius              float32 = 8
	ThemeSwatchInset               float32 = 4
	ThemeSwatchQueryHeight         float32 = 10
	ThemeSwatchQueryRadius         float32 = 4
	ThemeSwatchGap                 float32 = 4
	ThemeSwatchResultHeight        float32 = 5
	ThemeSwatchResultRadius        float32 = 2
	ThemeSwatchAutoQueryTop        float32 = 5
	ThemeSwatchAutoLineWidth       float32 = 1.5
	ThemeSwatchOutlineWidthDefault float32 = 2
	ThemeSwatchOutlineWidthMin     float32 = 1.5
	ThemeSwatchOutlineWidthMax     float32 = 2.5
)

const themeAutoSwatchMaskID = "theme-auto-swatch"

type themeSwatchColors struct {
	Background   string
	Query        string
	Selected     string
	Outline      string
	OutlineWidth float32
}

type themeSwatchChrome struct {
	Color string
	Width float32
}

type themeSwatchPoint struct {
	X, Y float32
}

var (
	themeSwatchFallback = themeSwatchColors{
		Background: "#181D26F2",
		Query:      "#384352E6",
		Selected:   "#2BB5A8D2",
	}
	themeSwatchLightFallback = themeSwatchColors{
		Background: "#F5F5F5",
		Query:      "#E8E8E8",
		Selected:   "#D8D8D8",
	}
	themeSwatchDarkFallback = themeSwatchColors{
		Background: "#2B2B2B",
		Query:      "#3D3D3D",
		Selected:   "#4A4A4A",
	}
)

// NewWoxImageTheme builds the same rounded catalog swatch used in Settings.
func NewWoxImageTheme(theme Theme) WoxImage {
	return NewWoxImageSvg(ThemeSwatchSVG(theme))
}

// NewWoxImageThemeAuto builds the diagonal AUTO catalog swatch used in Settings.
func NewWoxImageThemeAuto(light, dark Theme) WoxImage {
	return NewWoxImageSvg(ThemeAutoSwatchSVG(light, dark))
}

// ThemeSwatchSVG renders one theme as the Settings catalog icon.
func ThemeSwatchSVG(theme Theme) string {
	return themeSwatchSVG(themeSwatchColorsFromTheme(theme, themeSwatchFallback))
}

// ThemeAutoSwatchSVG renders light and dark variants as the Settings AUTO icon.
func ThemeAutoSwatchSVG(light, dark Theme) string {
	lightColors := themeSwatchColorsFromTheme(light, themeSwatchLightFallback)
	darkColors := themeSwatchColorsFromTheme(dark, themeSwatchDarkFallback)
	chrome := themeSwatchChrome{Color: darkColors.Outline, Width: darkColors.OutlineWidth}
	if chrome.Width == 0 {
		chrome = themeSwatchChrome{Color: lightColors.Outline, Width: lightColors.OutlineWidth}
	}
	return themeAutoSwatchSVG(lightColors, darkColors, chrome)
}

// ThemeSwatchOutlineWidth scales an authored window border onto the catalog icon.
func ThemeSwatchOutlineWidth(authored int) float32 {
	if authored <= 0 {
		return 0
	}
	width := float32(authored) * 0.75
	if width < ThemeSwatchOutlineWidthMin {
		return ThemeSwatchOutlineWidthMin
	}
	if width > ThemeSwatchOutlineWidthMax {
		return ThemeSwatchOutlineWidthMax
	}
	return width
}

func themeSwatchQueryTop() float32 {
	return (ThemeSwatchSize - ThemeSwatchQueryHeight - ThemeSwatchGap - ThemeSwatchResultHeight) / 2
}

func themeSwatchAutoResultTop() float32 {
	return ThemeSwatchAutoQueryTop + ThemeSwatchQueryHeight + ThemeSwatchGap
}

func themeSwatchColorsFromTheme(theme Theme, fallback themeSwatchColors) themeSwatchColors {
	outline, outlineWidth := themeSwatchOutline(theme)
	return themeSwatchColors{
		Background:   firstNonEmpty(theme.AppBackgroundColor, fallback.Background),
		Query:        firstNonEmpty(theme.QueryBoxBackgroundColor, fallback.Query),
		Selected:     firstNonEmpty(theme.ResultItemActiveBackgroundColor, fallback.Selected),
		Outline:      outline,
		OutlineWidth: outlineWidth,
	}
}

// themeSwatchOutline keeps v2 default divider chrome off the icon; only authored
// visible window outlines such as Jade are represented.
func themeSwatchOutline(theme Theme) (string, float32) {
	if !theme.UsesCustomWindowChrome() {
		return "", 0
	}
	if theme.AppBorderWidth != nil && *theme.AppBorderWidth <= 0 {
		return "", 0
	}
	color := ""
	if colors := theme.ResolvedColors(); colors != nil {
		color = strings.TrimSpace(colors["AppBorderColor"])
	}
	parsed, ok := ParseThemeColor(color)
	if !ok || parsed.A == 0 {
		return "", 0
	}
	if theme.AppBorderWidth == nil && !themeSwatchAuthoredBorderColor(theme) {
		return "", 0
	}
	width := ThemeSwatchOutlineWidthDefault
	if theme.AppBorderWidth != nil {
		width = ThemeSwatchOutlineWidth(*theme.AppBorderWidth)
	}
	return color, width
}

func themeSwatchAuthoredBorderColor(theme Theme) bool {
	source, ok := theme.source.(*themeV2Source)
	if !ok {
		return false
	}
	return source.definition.AppBorderColor != nil
}

func themeSwatchSVG(colors themeSwatchColors) string {
	innerWidth := ThemeSwatchSize - ThemeSwatchInset*2
	queryTop := themeSwatchQueryTop()
	resultTop := queryTop + ThemeSwatchQueryHeight + ThemeSwatchGap
	var builder strings.Builder
	fmt.Fprintf(&builder,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %g %g"><rect width="%g" height="%g" rx="%g" fill="%s"/><rect x="%g" y="%g" width="%g" height="%g" rx="%g" fill="%s"/><rect x="%g" y="%g" width="%g" height="%g" rx="%g" fill="%s"/>`,
		ThemeSwatchSize, ThemeSwatchSize,
		ThemeSwatchSize, ThemeSwatchSize, ThemeSwatchRadius, svgPaint(colors.Background),
		ThemeSwatchInset, queryTop, innerWidth, ThemeSwatchQueryHeight, ThemeSwatchQueryRadius, svgPaint(colors.Query),
		ThemeSwatchInset, resultTop, innerWidth, ThemeSwatchResultHeight, ThemeSwatchResultRadius, svgPaint(colors.Selected),
	)
	builder.WriteString(themeSwatchSVGOutline(themeSwatchChrome{Color: colors.Outline, Width: colors.OutlineWidth}))
	builder.WriteString(`</svg>`)
	return builder.String()
}

func themeAutoSwatchSVG(light, dark themeSwatchColors, chrome themeSwatchChrome) string {
	innerWidth := ThemeSwatchSize - ThemeSwatchInset*2
	queryTop := ThemeSwatchAutoQueryTop
	resultTop := themeSwatchAutoResultTop()
	half := ThemeSwatchAutoLineWidth / 2
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %g %g"><defs><mask id="%s"><rect width="%g" height="%g" rx="%g" fill="#ffffff"/></mask></defs>`,
		ThemeSwatchSize, ThemeSwatchSize, themeAutoSwatchMaskID, ThemeSwatchSize, ThemeSwatchSize, ThemeSwatchRadius,
	))
	builder.WriteString(svgMaskedPolygon(themeSwatchDiagonalPolygon([]themeSwatchPoint{
		{0, 0}, {ThemeSwatchSize, 0}, {ThemeSwatchSize, ThemeSwatchSize}, {0, ThemeSwatchSize},
	}, true), light.Background))
	builder.WriteString(svgMaskedPolygon(themeSwatchDiagonalPolygon([]themeSwatchPoint{
		{0, 0}, {ThemeSwatchSize, 0}, {ThemeSwatchSize, ThemeSwatchSize}, {0, ThemeSwatchSize},
	}, false), dark.Background))
	query := []themeSwatchPoint{
		{ThemeSwatchInset, queryTop},
		{ThemeSwatchInset + innerWidth, queryTop},
		{ThemeSwatchInset + innerWidth, queryTop + ThemeSwatchQueryHeight},
		{ThemeSwatchInset, queryTop + ThemeSwatchQueryHeight},
	}
	result := []themeSwatchPoint{
		{ThemeSwatchInset, resultTop},
		{ThemeSwatchInset + innerWidth, resultTop},
		{ThemeSwatchInset + innerWidth, resultTop + ThemeSwatchResultHeight},
		{ThemeSwatchInset, resultTop + ThemeSwatchResultHeight},
	}
	builder.WriteString(svgMaskedPolygon(themeSwatchDiagonalPolygon(query, true), light.Query))
	builder.WriteString(svgMaskedPolygon(themeSwatchDiagonalPolygon(query, false), dark.Query))
	builder.WriteString(svgMaskedPolygon(themeSwatchDiagonalPolygon(result, true), light.Selected))
	builder.WriteString(svgMaskedPolygon(themeSwatchDiagonalPolygon(result, false), dark.Selected))
	builder.WriteString(svgMaskedPolygon([]themeSwatchPoint{
		{ThemeSwatchSize - half, 0},
		{ThemeSwatchSize + half, 0},
		{half, ThemeSwatchSize},
		{-half, ThemeSwatchSize},
	}, "#00000026"))
	builder.WriteString(themeSwatchSVGOutline(chrome))
	builder.WriteString(`</svg>`)
	return builder.String()
}

func themeSwatchSVGOutline(chrome themeSwatchChrome) string {
	if chrome.Width <= 0 || strings.TrimSpace(chrome.Color) == "" {
		return ""
	}
	half := chrome.Width / 2
	return fmt.Sprintf(
		`<rect x="%g" y="%g" width="%g" height="%g" rx="%g" fill="none" stroke="%s" stroke-width="%g"/>`,
		half, half, ThemeSwatchSize-chrome.Width, ThemeSwatchSize-chrome.Width, max(float32(0), ThemeSwatchRadius-half), svgPaint(chrome.Color), chrome.Width,
	)
}

func svgMaskedPolygon(points []themeSwatchPoint, fill string) string {
	if len(points) < 3 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString(`<polygon points="`)
	for index, point := range points {
		if index > 0 {
			builder.WriteByte(' ')
		}
		fmt.Fprintf(&builder, "%g,%g", point.X, point.Y)
	}
	fmt.Fprintf(&builder, `" fill="%s" mask="url(#%s)"/>`, svgPaint(fill), themeAutoSwatchMaskID)
	return builder.String()
}

// themeSwatchDiagonalPolygon clips a convex surface to one AUTO preview half-plane.
func themeSwatchDiagonalPolygon(points []themeSwatchPoint, light bool) []themeSwatchPoint {
	if len(points) < 3 {
		return nil
	}
	value := func(point themeSwatchPoint) float32 {
		return point.X*ThemeSwatchSize + point.Y*ThemeSwatchSize - ThemeSwatchSize*ThemeSwatchSize
	}
	inside := func(v float32) bool {
		if light {
			return v <= 0
		}
		return v >= 0
	}
	clipped := make([]themeSwatchPoint, 0, 5)
	appendPoint := func(point themeSwatchPoint) {
		if len(clipped) == 0 || clipped[len(clipped)-1] != point {
			clipped = append(clipped, point)
		}
	}
	for index, current := range points {
		next := points[(index+1)%len(points)]
		currentValue, nextValue := value(current), value(next)
		currentInside, nextInside := inside(currentValue), inside(nextValue)
		if currentInside {
			appendPoint(current)
		}
		if currentInside != nextInside {
			t := currentValue / (currentValue - nextValue)
			appendPoint(themeSwatchPoint{X: current.X + (next.X-current.X)*t, Y: current.Y + (next.Y-current.Y)*t})
		}
	}
	if len(clipped) > 1 && clipped[0] == clipped[len(clipped)-1] {
		clipped = clipped[:len(clipped)-1]
	}
	return clipped
}

func svgPaint(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "#000000"
	}
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, `"`, "&quot;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	return value
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
