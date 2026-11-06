package woxview

import (
	"errors"
	"fmt"

	"wox/common/icons"
	component "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// ButtonVariant selects a shared Wox button treatment.
type ButtonVariant string

func (ButtonVariant) enumValues() []string {
	return []string{"", string(ButtonVariantSecondary), string(ButtonVariantPrimary), string(ButtonVariantOutline), string(ButtonVariantText), string(ButtonVariantSurface)}
}

const (
	ButtonVariantSecondary ButtonVariant = "secondary"
	ButtonVariantPrimary   ButtonVariant = "primary"
	ButtonVariantOutline   ButtonVariant = "outline"
	ButtonVariantText      ButtonVariant = "text"
	ButtonVariantSurface   ButtonVariant = "surface"
)

// ButtonProps is a labeled button with an optional leading catalog icon.
type ButtonProps struct {
	Styled
	Label         string        `json:"label"`
	Icon          string        `json:"icon,omitempty"`
	IconColorMode IconColorMode `json:"iconColorMode,omitempty"`
	Variant       ButtonVariant `json:"variant,omitempty"`
	Disabled      bool          `json:"disabled,omitempty"`
	OnPress       *Handler      `json:"onPress,omitempty"`
}

func (p *ButtonProps) handlers() []namedHandler {
	return append(p.Styled.handlers(), []namedHandler{{"onPress", p.OnPress}}...)
}

func (p *ButtonProps) prepare() error {
	if err := p.IconColorMode.validate(); err != nil {
		return err
	}
	switch p.Variant {
	case "", ButtonVariantSecondary, ButtonVariantPrimary, ButtonVariantOutline, ButtonVariantText, ButtonVariantSurface:
	default:
		return fmt.Errorf("unknown variant %q", p.Variant)
	}
	if err := validateIcon(p.Icon); err != nil {
		return err
	}
	return p.Style.validate()
}

// IconButtonProps is a compact icon-only button; Label is its accessible name.
type IconButtonProps struct {
	Styled
	Icon          string        `json:"icon"`
	IconColorMode IconColorMode `json:"iconColorMode,omitempty"`
	Label         string        `json:"label,omitempty"`
	Disabled      bool          `json:"disabled,omitempty"`
	OnPress       *Handler      `json:"onPress,omitempty"`
}

func (p *IconButtonProps) handlers() []namedHandler {
	return append(p.Styled.handlers(), []namedHandler{{"onPress", p.OnPress}}...)
}

func (p *IconButtonProps) prepare() error {
	if err := p.IconColorMode.validate(); err != nil {
		return err
	}
	if p.Icon == "" {
		return errors.New("icon-button needs an icon")
	}
	if err := validateIcon(p.Icon); err != nil {
		return err
	}
	return p.Style.validate()
}

func validateIcon(name string) error {
	if name != "" && icons.Get(name).IsEmpty() {
		return fmt.Errorf("unknown icon %q", name)
	}
	return nil
}

func buildButton(b *builder, node *Node, id string) widget.Widget {
	props := node.Props.(*ButtonProps)
	return buildButtonControl(b, props, id, b.tap(props.OnPress))
}

// buildButtonControl accepts the action directly so form submission preserves its full payload.
func buildButtonControl(b *builder, props *ButtonProps, id string, onTap func()) widget.Widget {
	theme := b.theme()
	variant := component.ButtonSecondary
	foreground := theme.ControlText
	switch props.Variant {
	case ButtonVariantPrimary:
		variant, foreground = component.ButtonPrimary, theme.AccentText
	case ButtonVariantOutline:
		variant, foreground = component.ButtonOutline, theme.Text
	case ButtonVariantText:
		variant, foreground = component.ButtonText, theme.Text
	case ButtonVariantSurface:
		variant, foreground = component.ButtonSurface, theme.BodyText
	}
	var icon *woxui.Image
	paint := props.Style.controlPaint(theme)
	if paint != nil && paint.Foreground != nil {
		foreground = *paint.Foreground
	}
	if props.Icon != "" {
		if props.IconColorMode == IconColorTint {
			if props.Disabled && (paint == nil || paint.Foreground == nil) {
				foreground.A = component.DisabledContentAlpha
			}
			icon = component.IconImage(props.Icon, 16, foreground)
		} else {
			icon = b.originalIcon(props.Icon, 16)
		}
	}
	var iconOpacity *float32
	if props.Disabled && props.IconColorMode != IconColorTint {
		disabledOpacity := float32(component.DisabledContentAlpha) / 255
		iconOpacity = &disabledOpacity
	}
	style := props.Style.resolve(b.theme())
	return props.Style.wrapControl(b.theme(), component.WoxButton(component.ButtonProps{
		ID: id, Label: b.translate(props.Label), Icon: icon, IconOpacity: iconOpacity, Variant: variant, Disabled: props.Disabled,
		Width: style.Width, Height: style.Height, Radius: style.BorderRadius, FontSize: style.FontSize,
		OnTap: onTap, Theme: theme, Paint: paint,
	}))
}

func buildIconButton(b *builder, node *Node, id string) widget.Widget {
	props := node.Props.(*IconButtonProps)
	theme := b.theme()
	style := props.Style.resolve(b.theme())
	width, height := style.Width, style.Height
	if width <= 0 {
		width = 28
	}
	if height <= 0 {
		height = width
	}
	radius := style.BorderRadius
	if radius <= 0 {
		radius = 4
	}
	color := style.Color.resolve(theme, theme.Text)
	if props.Disabled && (props.Style.state == nil || props.Style.state.Color == nil) {
		color.A = uint8(float32(color.A) * 0.4)
	}
	hover := theme.Text
	hover.A = 24
	if props.Style.state != nil {
		if props.Style.state.BackgroundColor != nil {
			hover = style.BackgroundColor.resolve(theme, woxui.Color{})
		}
		if props.Style.state.BorderRadius != nil {
			radius = style.BorderRadius
		}
	}
	background, _ := props.Style.controlFill(theme)
	size := min(width, height) * 0.57
	var icon widget.Widget
	if props.IconColorMode != IconColorTint {
		icon = widget.Image{Source: b.originalIcon(props.Icon, size), Width: size, Height: size}
		if props.Disabled {
			icon = widget.Opacity{Value: float32(component.DisabledContentAlpha) / 255, Child: icon}
		}
	} else {
		icon = component.IconGlyph(props.Icon, size, color)
	}
	return props.Style.wrapControl(b.theme(), component.WoxIconButton(component.IconButtonProps{
		ID: id, Label: b.translate(props.Label), Icon: icon,
		Width: width, Height: height, Radius: radius, Background: background, Paint: props.Style.controlPaint(theme),
		HoverBackground: hover, FocusRingColor: theme.Focus, Disabled: props.Disabled, OnTap: b.tap(props.OnPress),
	}))
}

func (p *ButtonProps) isDisabled() bool { return p.Disabled }

func (p *IconButtonProps) isDisabled() bool { return p.Disabled }
