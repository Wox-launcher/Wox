package woxview

import (
	"errors"
	"net/url"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// LinkProps invokes a producer handler or delegates opening a web/mail URL to the embedder.
type LinkProps struct {
	Styled
	Label    string   `json:"label,omitempty"`
	URL      string   `json:"url"`
	Disabled bool     `json:"disabled,omitempty"`
	OnPress  *Handler `json:"onPress,omitempty"`
}

func (p *LinkProps) handlers() []namedHandler {
	return append(p.Styled.handlers(), namedHandler{"onPress", p.OnPress})
}

func (p *LinkProps) prepare() error {
	u, err := url.ParseRequestURI(p.URL)
	if err != nil || !((u.Scheme == "http" || u.Scheme == "https") && u.Host != "" || u.Scheme == "mailto" && u.Opaque != "") {
		return errors.New("url must be an absolute http, https, or mailto URL")
	}
	return p.Style.validate()
}

// buildLink preserves the original URL bytes and never both opens and invokes a handler.
func buildLink(b *builder, node *Node, id string) widget.Widget {
	p := node.Props.(*LinkProps)
	label := b.translate(p.Label)
	if label == "" {
		label = p.URL
	}
	onTap := b.tap(p.OnPress)
	if p.OnPress == nil && b.props.OnOpenLink != nil {
		onTap = func() { b.props.OnOpenLink(p.URL) }
	}
	theme := b.theme()
	theme.Text = p.Style.Color.resolve(theme, theme.Accent)
	return p.Style.wrapControl(b.theme(), component.WoxButton(component.ButtonProps{
		ID: id, Label: label, Role: ui.AccessibilityRoleLink, Variant: component.ButtonText,
		Width: p.Style.Width.resolve(theme), Height: p.Style.Height.resolve(theme), FontSize: p.Style.FontSize.base(), Theme: theme,
		Disabled: p.Disabled || onTap == nil, OnTap: onTap, Paint: p.Style.controlPaint(b.theme()),
	}))
}

func (p *LinkProps) isDisabled() bool { return p.Disabled }
