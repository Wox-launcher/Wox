package theme

import (
	"math"
	"strconv"
	"strings"
)

type RGBAColor struct {
	R uint8
	G uint8
	B uint8
	A uint8
}

func (c RGBAColor) WithAlphaScale(scale float64) RGBAColor {
	if scale <= 0 {
		c.A = 0
		return c
	}
	if scale >= 1 {
		return c
	}

	c.A = uint8(math.Round(float64(c.A) * scale))
	return c
}

func (c RGBAColor) CompositeOver(background RGBAColor) RGBAColor {
	if c.A == 0 {
		return RGBAColor{R: background.R, G: background.G, B: background.B, A: 255}
	}
	if c.A == 255 {
		return RGBAColor{R: c.R, G: c.G, B: c.B, A: 255}
	}

	alpha := float64(c.A) / 255.0
	inverse := 1.0 - alpha

	return RGBAColor{
		R: uint8(math.Round(float64(c.R)*alpha + float64(background.R)*inverse)),
		G: uint8(math.Round(float64(c.G)*alpha + float64(background.G)*inverse)),
		B: uint8(math.Round(float64(c.B)*alpha + float64(background.B)*inverse)),
		A: 255,
	}
}

func ParseColor(value string) (RGBAColor, bool) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return RGBAColor{}, false
	}

	switch {
	case strings.HasPrefix(trimmed, "#"):
		return parseHexColor(trimmed)
	case strings.HasPrefix(trimmed, "rgba(") && strings.HasSuffix(trimmed, ")"):
		return parseRGBAColor(trimmed)
	case strings.HasPrefix(trimmed, "rgb(") && strings.HasSuffix(trimmed, ")"):
		return parseRGBColor(trimmed)
	default:
		return RGBAColor{}, false
	}
}

func parseHexColor(value string) (RGBAColor, bool) {
	hex := strings.TrimPrefix(value, "#")
	switch len(hex) {
	case 6:
		parsed, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return RGBAColor{}, false
		}
		return RGBAColor{
			R: uint8((parsed >> 16) & 0xff),
			G: uint8((parsed >> 8) & 0xff),
			B: uint8(parsed & 0xff),
			A: 255,
		}, true
	case 8:
		parsed, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return RGBAColor{}, false
		}
		return RGBAColor{
			R: uint8((parsed >> 24) & 0xff),
			G: uint8((parsed >> 16) & 0xff),
			B: uint8((parsed >> 8) & 0xff),
			A: uint8(parsed & 0xff),
		}, true
	default:
		return RGBAColor{}, false
	}
}

func parseRGBColor(value string) (RGBAColor, bool) {
	inner := strings.TrimSuffix(strings.TrimPrefix(value, "rgb("), ")")
	parts := strings.Split(inner, ",")
	if len(parts) != 3 {
		return RGBAColor{}, false
	}

	r, ok := parseUint8Channel(parts[0])
	if !ok {
		return RGBAColor{}, false
	}
	g, ok := parseUint8Channel(parts[1])
	if !ok {
		return RGBAColor{}, false
	}
	b, ok := parseUint8Channel(parts[2])
	if !ok {
		return RGBAColor{}, false
	}

	return RGBAColor{R: r, G: g, B: b, A: 255}, true
}

func parseRGBAColor(value string) (RGBAColor, bool) {
	inner := strings.TrimSuffix(strings.TrimPrefix(value, "rgba("), ")")
	parts := strings.Split(inner, ",")
	if len(parts) != 4 {
		return RGBAColor{}, false
	}

	r, ok := parseUint8Channel(parts[0])
	if !ok {
		return RGBAColor{}, false
	}
	g, ok := parseUint8Channel(parts[1])
	if !ok {
		return RGBAColor{}, false
	}
	b, ok := parseUint8Channel(parts[2])
	if !ok {
		return RGBAColor{}, false
	}
	a, ok := parseAlphaChannel(parts[3])
	if !ok {
		return RGBAColor{}, false
	}

	return RGBAColor{R: r, G: g, B: b, A: a}, true
}

func parseUint8Channel(value string) (uint8, bool) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 || parsed > 255 {
		return 0, false
	}
	return uint8(parsed), true
}

func parseAlphaChannel(value string) (uint8, bool) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed < 0 {
		return 0, false
	}

	if parsed <= 1 {
		return uint8(math.Round(parsed * 255.0)), true
	}

	if parsed > 255 {
		return 0, false
	}

	return uint8(math.Round(parsed)), true
}
