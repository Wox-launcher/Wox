package woxview

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	component "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// Align is the cross-axis alignment of a box's children.
type Align string

const (
	AlignStart   Align = "start"
	AlignCenter  Align = "center"
	AlignEnd     Align = "end"
	AlignStretch Align = "stretch"
)

// Justify is the main-axis distribution of a box's children.
type Justify string

const (
	JustifyStart        Justify = "start"
	JustifyCenter       Justify = "center"
	JustifyEnd          Justify = "end"
	JustifySpaceBetween Justify = "space-between"
)

// FontWeight is the portable text weight; bold maps to the heaviest weight the renderer supports.
type FontWeight string

const (
	FontWeightRegular  FontWeight = "regular"
	FontWeightSemibold FontWeight = "semibold"
	FontWeightBold     FontWeight = "bold"
)

// FontFamily selects the platform UI or monospace family.
type FontFamily string

const (
	FontFamilyUI        FontFamily = "ui"
	FontFamilyMonospace FontFamily = "monospace"
)

// TextAlign positions wrapped text lines.
type TextAlign string

// Overflow controls clipping without changing the node's layout dimensions.
type Overflow string

// Shadow selects a fixed elevation treatment rather than arbitrary blur parameters.
type Shadow string

const (
	ShadowSmall  Shadow = "sm"
	ShadowMedium Shadow = "md"
	ShadowLarge  Shadow = "lg"
)

func (Shadow) enumValues() []string {
	return []string{"", string(ShadowSmall), string(ShadowMedium), string(ShadowLarge)}
}

const (
	OverflowVisible Overflow = "visible"
	OverflowHidden  Overflow = "hidden"
)

func (Overflow) enumValues() []string {
	return []string{"", string(OverflowVisible), string(OverflowHidden)}
}

const (
	TextAlignStart  TextAlign = "start"
	TextAlignCenter TextAlign = "center"
)

func (Align) enumValues() []string {
	return []string{"", string(AlignStart), string(AlignCenter), string(AlignEnd), string(AlignStretch)}
}
func (Justify) enumValues() []string {
	return []string{"", string(JustifyStart), string(JustifyCenter), string(JustifyEnd), string(JustifySpaceBetween)}
}
func (FontWeight) enumValues() []string {
	return []string{"", string(FontWeightRegular), string(FontWeightSemibold), string(FontWeightBold)}
}
func (FontFamily) enumValues() []string {
	return []string{"", string(FontFamilyUI), string(FontFamilyMonospace)}
}
func (TextAlign) enumValues() []string {
	return []string{"", string(TextAlignStart), string(TextAlignCenter)}
}

// Edges accepts a size for all sides or an object with sides and horizontal/vertical shorthands.
type Edges struct {
	Top    Size `json:"top,omitempty,omitzero"`
	Right  Size `json:"right,omitempty,omitzero"`
	Bottom Size `json:"bottom,omitempty,omitzero"`
	Left   Size `json:"left,omitempty,omitzero"`
}

// UnmarshalJSON supports `8` and `{"horizontal":12,"top":4}`; explicit sides override shorthands.
func (e *Edges) UnmarshalJSON(data []byte) error {
	var all Size
	if json.Unmarshal(data, &all) == nil {
		*e = Edges{Top: all, Right: all, Bottom: all, Left: all}
		return nil
	}
	var sides struct {
		Top, Right, Bottom, Left *Size
		Horizontal, Vertical     Size
	}
	if err := decodeStrict(data, &sides); err != nil {
		return errors.New("edges must be a size or an object of top/right/bottom/left/horizontal/vertical")
	}
	pick := func(side *Size, fallback Size) Size {
		if side != nil {
			return *side
		}
		return fallback
	}
	*e = Edges{Top: pick(sides.Top, sides.Vertical), Right: pick(sides.Right, sides.Horizontal), Bottom: pick(sides.Bottom, sides.Vertical), Left: pick(sides.Left, sides.Horizontal)}
	return nil
}

func (e Edges) resolve(theme component.ControlTheme) widget.Insets {
	return widget.Insets{Left: e.Left.resolve(theme), Top: e.Top.resolve(theme), Right: e.Right.resolve(theme), Bottom: e.Bottom.resolve(theme)}
}

// Style describes box layout and presentation. Pointer fields distinguish unset
// values from explicit zero overrides; other zero values use component defaults.
type Style struct {
	// state identifies explicit paint overrides for shared controls with their own defaults.
	state *StateStyle
	// Shared controls resolve their own default fill before animating this target.
	controlTransition *component.PaintTransition
	controlBackground *woxui.Color
	Width             Size     `json:"width,omitempty,omitzero"`
	Height            Size     `json:"height,omitempty,omitzero"`
	MinWidth          Size     `json:"minWidth,omitempty,omitzero"`
	MaxWidth          Size     `json:"maxWidth,omitempty,omitzero"`
	MinHeight         Size     `json:"minHeight,omitempty,omitzero"`
	MaxHeight         Size     `json:"maxHeight,omitempty,omitzero"`
	AspectRatio       *float32 `json:"aspectRatio,omitempty"`
	Overflow          Overflow `json:"overflow,omitempty"`
	Floating          bool     `json:"floating,omitempty"`
	Opacity           *float32 `json:"opacity,omitempty"`
	Shadow            Shadow   `json:"shadow,omitempty"`
	TranslateX        Offset   `json:"translateX,omitempty,omitzero"`
	TranslateY        Offset   `json:"translateY,omitempty,omitzero"`
	Scale             *float32 `json:"scale,omitempty"`
	Rotate            float32  `json:"rotate,omitempty"`
	// Flex makes a child of row/column/view take a weighted share of the remaining main-axis space.
	Flex           float32 `json:"flex,omitempty"`
	Padding        Edges   `json:"padding,omitempty,omitzero"`
	Margin         Edges   `json:"margin,omitempty,omitzero"`
	Gap            Size    `json:"gap,omitempty,omitzero"`
	AlignItems     Align   `json:"alignItems,omitempty"`
	AlignSelf      Align   `json:"alignSelf,omitempty"`
	JustifyContent Justify `json:"justifyContent,omitempty"`

	BackgroundColor   Color `json:"backgroundColor,omitempty"`
	BorderColor       Color `json:"borderColor,omitempty"`
	BorderWidth       Size  `json:"borderWidth,omitempty,omitzero"`
	BorderRadius      Size  `json:"borderRadius,omitempty,omitzero"`
	BorderTopWidth    *Size `json:"borderTopWidth,omitempty,omitzero"`
	BorderRightWidth  *Size `json:"borderRightWidth,omitempty,omitzero"`
	BorderBottomWidth *Size `json:"borderBottomWidth,omitempty,omitzero"`
	BorderLeftWidth   *Size `json:"borderLeftWidth,omitempty,omitzero"`

	Color      Color      `json:"color,omitempty"`
	FontSize   Size       `json:"fontSize,omitempty,omitzero"`
	FontWeight FontWeight `json:"fontWeight,omitempty"`
	FontFamily FontFamily `json:"fontFamily,omitempty"`
	Italic     bool       `json:"italic,omitempty"`
	LineHeight Size       `json:"lineHeight,omitempty,omitzero"`
	MaxLines   int        `json:"maxLines,omitempty"`
	TextAlign  TextAlign  `json:"textAlign,omitempty"`

	// Top/Left/Right/Bottom position a child inside a stack. Setting both opposite sides stretches it.
	Top    *Size `json:"top,omitempty,omitzero"`
	Left   *Size `json:"left,omitempty,omitzero"`
	Right  *Size `json:"right,omitempty,omitzero"`
	Bottom *Size `json:"bottom,omitempty,omitzero"`
}

func (s Style) validate() error {
	numbers := map[string]Size{
		"width": s.Width, "height": s.Height, "minWidth": s.MinWidth, "maxWidth": s.MaxWidth, "minHeight": s.MinHeight, "maxHeight": s.MaxHeight,
		"gap": s.Gap, "borderWidth": s.BorderWidth, "borderRadius": s.BorderRadius, "fontSize": s.FontSize, "lineHeight": s.LineHeight,
		"padding.top": s.Padding.Top, "padding.right": s.Padding.Right, "padding.bottom": s.Padding.Bottom, "padding.left": s.Padding.Left,
		"margin.top": s.Margin.Top, "margin.right": s.Margin.Right, "margin.bottom": s.Margin.Bottom, "margin.left": s.Margin.Left,
	}
	for name, position := range map[string]*Size{"top": s.Top, "left": s.Left, "right": s.Right, "bottom": s.Bottom} {
		if position != nil {
			numbers[name] = *position
		}
	}
	for name, width := range map[string]*Size{"borderTopWidth": s.BorderTopWidth, "borderRightWidth": s.BorderRightWidth, "borderBottomWidth": s.BorderBottomWidth, "borderLeftWidth": s.BorderLeftWidth} {
		if width != nil {
			numbers[name] = *width
		}
	}
	if s.AspectRatio != nil && (*s.AspectRatio <= 0 || *s.AspectRatio > 16384 || math.IsNaN(float64(*s.AspectRatio))) {
		return errors.New("style.aspectRatio must be greater than 0 and at most 16384")
	}
	if s.Opacity != nil && (*s.Opacity < 0 || *s.Opacity > 1 || math.IsNaN(float64(*s.Opacity))) {
		return errors.New("style.opacity must be between 0 and 1")
	}
	for name, offset := range map[string]Offset{"translateX": s.TranslateX, "translateY": s.TranslateY} {
		if err := offset.validate(); err != nil {
			return fmt.Errorf("style.%s: %w", name, err)
		}
	}
	for name, value := range map[string]*float32{"scale": s.Scale, "rotate": &s.Rotate} {
		if value != nil && (*value < -16384 || *value > 16384 || math.IsNaN(float64(*value))) {
			return fmt.Errorf("style.%s must be between -16384 and 16384", name)
		}
	}
	switch s.Overflow {
	case "", OverflowVisible, OverflowHidden:
	default:
		return fmt.Errorf("unknown style.overflow %q", s.Overflow)
	}
	switch s.Shadow {
	case "", ShadowSmall, ShadowMedium, ShadowLarge:
	default:
		return fmt.Errorf("unknown style.shadow %q", s.Shadow)
	}
	for name, value := range numbers {
		if err := value.validate(); err != nil {
			return fmt.Errorf("style.%s: %w", name, err)
		}
	}
	if s.Flex < 0 || s.Flex > 16384 || math.IsNaN(float64(s.Flex)) {
		return errors.New("style.flex must be between 0 and 16384")
	}
	if s.MaxLines < 0 || s.MaxLines > 10000 {
		return errors.New("style.maxLines must be between 0 and 10000")
	}
	switch s.AlignItems {
	case "", AlignStart, AlignCenter, AlignEnd, AlignStretch:
	default:
		return fmt.Errorf("unknown style.alignItems %q", s.AlignItems)
	}
	switch s.AlignSelf {
	case "", AlignStart, AlignCenter, AlignEnd, AlignStretch:
	default:
		return fmt.Errorf("unknown style.alignSelf %q", s.AlignSelf)
	}
	switch s.JustifyContent {
	case "", JustifyStart, JustifyCenter, JustifyEnd, JustifySpaceBetween:
	default:
		return fmt.Errorf("unknown style.justifyContent %q", s.JustifyContent)
	}
	switch s.FontWeight {
	case "", FontWeightRegular, FontWeightSemibold, FontWeightBold:
	default:
		return fmt.Errorf("unknown style.fontWeight %q", s.FontWeight)
	}
	switch s.FontFamily {
	case "", FontFamilyUI, FontFamilyMonospace:
	default:
		return fmt.Errorf("unknown style.fontFamily %q", s.FontFamily)
	}
	switch s.TextAlign {
	case "", TextAlignStart, TextAlignCenter:
	default:
		return fmt.Errorf("unknown style.textAlign %q", s.TextAlign)
	}
	for _, color := range []Color{s.BackgroundColor, s.BorderColor, s.Color} {
		if err := color.validate(); err != nil {
			return fmt.Errorf("style: %w", err)
		}
	}
	return nil
}

func (s resolvedStyle) textStyle(theme component.ControlTheme, defaultSize float32) woxui.TextStyle {
	size := s.FontSize
	if size <= 0 {
		size = defaultSize
	}
	style := woxui.TextStyle{Size: theme.Scaled(size), Italic: s.Italic}
	if s.FontWeight == FontWeightSemibold || s.FontWeight == FontWeightBold {
		style.Weight = woxui.FontWeightSemibold
	}
	if s.FontFamily == FontFamilyMonospace {
		style.Family = woxui.FontFamilyMonospace
	}
	return style
}

func (s Style) crossAxis() widget.CrossAxisAlignment {
	switch s.AlignItems {
	case AlignCenter:
		return widget.CrossAxisCenter
	case AlignEnd:
		return widget.CrossAxisEnd
	case AlignStretch:
		return widget.CrossAxisStretch
	}
	return widget.CrossAxisStart
}

func (s Style) mainAxis() widget.MainAxisAlignment {
	switch s.JustifyContent {
	case JustifyCenter:
		return widget.MainAxisCenter
	case JustifyEnd:
		return widget.MainAxisEnd
	case JustifySpaceBetween:
		return widget.MainAxisSpaceBetween
	}
	return widget.MainAxisStart
}

// decorate applies box chrome before shared sizing, clipping, and outer margin.
func (s resolvedStyle) decorate(theme component.ControlTheme, content widget.Widget) widget.Widget {
	box := s.box(theme, content)
	box.Width, box.Height, box.Padding = s.Width, s.Height, s.Padding
	return s.finishBox(theme, box)
}

// box maps all edge overrides together so an explicit zero can suppress one
// side of a uniform border without painting the original stroke underneath it.
func (s resolvedStyle) box(theme component.ControlTheme, content widget.Widget) widget.Container {
	box := widget.Container{
		Radius: s.BorderRadius, Floating: s.Floating,
		Color: s.BackgroundColor.resolve(theme, woxui.Color{}), BorderColor: s.BorderColor.resolve(theme, theme.Border), BorderWidth: s.borderWidth(),
		Child: content,
	}
	if s.hasSideBorders() {
		box.EdgeBordersOnTop = true
		width := func(override *float32) float32 {
			if override != nil {
				return *override
			}
			return s.BorderWidth
		}
		box.TopBorderColor, box.RightBorderColor, box.BottomBorderColor, box.LeftBorderColor = box.BorderColor, box.BorderColor, box.BorderColor, box.BorderColor
		box.TopBorderWidth, box.RightBorderWidth = width(s.BorderTopWidth), width(s.BorderRightWidth)
		box.BottomBorderWidth, box.LeftBorderWidth = width(s.BorderBottomWidth), width(s.BorderLeftWidth)
		box.BorderWidth = 0
	}
	return box
}

func (s resolvedStyle) hasSideBorders() bool {
	return s.BorderTopWidth != nil || s.BorderRightWidth != nil || s.BorderBottomWidth != nil || s.BorderLeftWidth != nil
}

// borderWidth defaults to one unit when only a border color is set.
func (s resolvedStyle) borderWidth() float32 {
	if s.BorderWidth == 0 && s.BorderColor != (Color{}) {
		return 1
	}
	return s.BorderWidth
}

func (s resolvedStyle) constrain(child widget.Widget) widget.Widget {
	if s.MinWidth == 0 && s.MaxWidth == 0 && s.MinHeight == 0 && s.MaxHeight == 0 {
		return child
	}
	return widget.Constrained{MinWidth: s.MinWidth, MaxWidth: s.MaxWidth, MinHeight: s.MinHeight, MaxHeight: s.MaxHeight, Child: child}
}

// wrapOuter preserves a control's own chrome unless new box decorations are requested.
func (s resolvedStyle) wrapOuter(theme component.ControlTheme, child widget.Widget) widget.Widget {
	if s.Floating || s.hasSideBorders() || s.controlTransition != nil || s.state != nil && s.state.BackgroundColor != nil && s.BackgroundColor != (Color{}) {
		child = s.box(theme, child)
	}
	return s.finishBox(theme, child)
}

// finishBox applies the ratio inside min/max bounds, clips before margin, and
// preserves the existing natural-size path when no new property is supplied.
func (s resolvedStyle) finishBox(theme component.ControlTheme, child widget.Widget) widget.Widget {
	if s.AspectRatio != nil {
		content, ratio := child, *s.AspectRatio
		child = widget.LayoutBuilder{Build: func(available woxui.Size) widget.Widget {
			width, height := s.Width, s.Height
			if width > 0 {
				width = max(width, s.MinWidth)
			}
			if height > 0 {
				height = max(height, s.MinHeight)
			}
			if width <= 0 && height <= 0 {
				if available.Width < math.MaxFloat32 {
					width = available.Width
				} else if available.Height < math.MaxFloat32 {
					height = available.Height
				} else {
					return content
				}
			}
			if width > 0 && height > 0 {
				// Both explicit dimensions take precedence over a preferred ratio.
				width, height = min(width, available.Width), min(height, available.Height)
			} else {
				if width > 0 {
					width = min(width, available.Width)
					height = width / ratio
				} else {
					height = min(height, available.Height)
					width = height * ratio
				}
				if width < s.MinWidth {
					width, height = s.MinWidth, s.MinWidth/ratio
				}
				if height < s.MinHeight {
					height, width = s.MinHeight, s.MinHeight*ratio
				}
				if height > available.Height {
					height, width = available.Height, available.Height*ratio
				}
				if width > available.Width {
					width, height = available.Width, available.Width/ratio
				}
			}
			return widget.Constrained{MinWidth: width, MaxWidth: width, MinHeight: height, MaxHeight: height, Child: content}
		}}
	}
	child = s.constrain(child)
	if s.Overflow == OverflowHidden {
		child = widget.Clip{SizeToChild: true, Child: child}
	}
	if s.Shadow != "" {
		var sigma, offset float32
		switch s.Shadow {
		case ShadowSmall:
			sigma, offset = 2, 1
		case ShadowMedium:
			sigma, offset = 4, 2
		case ShadowLarge:
			sigma, offset = 8, 4
		}
		// Elevation surrounds the clipped border box, not its margin. Ancestor
		// clipping still applies and group opacity composites the entire shadow.
		child = widget.BoxShadow{Sigma: theme.Scaled(sigma), Offset: woxui.Point{Y: theme.Scaled(offset)},
			Radius: s.BorderRadius, Color: woxui.Color{A: 64}, Child: child}
	}
	if s.Margin != (widget.Insets{}) {
		child = widget.Container{Padding: s.Margin, Child: child}
	}
	return child
}
