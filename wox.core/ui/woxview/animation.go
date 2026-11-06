package woxview

import (
	"fmt"
	"image"
	"math"
	"slices"
	"sync"
	"time"

	ui "wox/ui/runtime"
	"wox/ui/widget"
)

type AnimationType string

const (
	AnimationPulse   AnimationType = "pulse"
	AnimationSpin    AnimationType = "spin"
	AnimationShimmer AnimationType = "shimmer"
)

func (AnimationType) enumValues() []string { return []string{"pulse", "spin", "shimmer"} }

// Animation declares one repeating Core-owned preset, never per-frame RPC work.
type Animation struct {
	Type     AnimationType `json:"type"`
	Duration *int          `json:"duration,omitempty"`
}

// validate rejects unknown presets and nonpositive/oversized periods.
func (a *Animation) validate() error {
	if a == nil {
		return nil
	}
	if !slices.Contains(AnimationType("").enumValues(), string(a.Type)) {
		return fmt.Errorf("unknown animation type %q", a.Type)
	}
	if a.Duration != nil && (*a.Duration < 1 || *a.Duration > 60000) {
		return fmt.Errorf("animation.duration must be between 1 and 60000 milliseconds")
	}
	return nil
}

// loopAnimation isolates timeline identity by owner, node and preset. The shared
// LoopAnimation restarts on period changes and drops timelines on unmount.
func (b *builder) loopAnimation(a *Animation, id string, build func(float32) widget.Widget) widget.Widget {
	if a == nil {
		return build(0)
	}
	duration := 1200
	if a.Duration != nil {
		duration = *a.Duration
	}
	return widget.LoopAnimation{Key: widget.Key(fmt.Sprintf("%p/%s/animation/%s", b.view, id, a.Type)), Duration: time.Duration(duration) * time.Millisecond, Builder: build}
}

// animationStyle composes presets after implicit interpolation, preserving the
// authored opacity/rotation instead of making the transition chase loop targets.
func animationStyle(style Style, a *Animation, progress float32) Style {
	if a == nil {
		return style
	}
	switch a.Type {
	case AnimationPulse:
		opacity := float32(1)
		if style.Opacity != nil {
			opacity = *style.Opacity
		}
		opacity *= .825 + .175*float32(math.Cos(float64(progress)*2*math.Pi))
		style.Opacity = &opacity
	case AnimationSpin:
		style.Rotate += 360 * progress
	}
	return style
}

// buildAnimatedNode keeps shimmer inside the node's opacity/transform wrappers.
func (b *builder) buildAnimatedNode(node *Node, id string, style Style, a *Animation) widget.Widget {
	return b.loopAnimation(a, id, func(progress float32) widget.Widget {
		current := animationStyle(style, a, progress)
		paintNode := node
		if current != styleOf(node) {
			paintNode = nodeWithStyle(node, current)
		}
		content := catalog[node.Type].build(b, paintNode, id)
		if a != nil && a.Type == AnimationShimmer {
			margin, dark := current.Margin.resolve(b.theme()), b.theme().Dark
			content = widget.PaintOverlay{Child: content, Paint: func(list *ui.DisplayList, bounds ui.Rect) {
				bounds.X += margin.Left
				bounds.Y += margin.Top
				bounds.Width -= margin.Left + margin.Right
				bounds.Height -= margin.Top + margin.Bottom
				paintShimmer(list, bounds, progress, dark)
			}}
		}
		return b.decorateNode(paintNode, id, content)
	})
}

// Two immutable 256x1 textures cover light/dark appearance with bounded storage.
// Destination geometry stays logical; the renderer samples actual image pixels.
var shimmerTextures = sync.OnceValue(func() [2]*ui.Image {
	var textures [2]*ui.Image
	for variant := range textures {
		pixels := image.NewRGBA(image.Rect(0, 0, 256, 1))
		for x := range 256 {
			alpha := uint8(math.Round(36 * math.Pow(math.Sin(math.Pi*float64(x)/255), 2)))
			offset := x * 4
			pixels.Pix[offset+3] = alpha
			if variant == 1 {
				pixels.Pix[offset], pixels.Pix[offset+1], pixels.Pix[offset+2] = alpha, alpha, alpha
			}
		}
		textures[variant], _ = ui.NewImageFromPackedRGBA(pixels)
	}
	return textures
})

// paintShimmer sweeps a soft neutral band across the local box. The band begins
// and ends entirely outside it, so wrapping the phase produces no visible jump.
func paintShimmer(list *ui.DisplayList, bounds ui.Rect, progress float32, dark bool) {
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	variant := 0
	if dark {
		variant = 1
	}
	band := bounds.Width * .35
	left := bounds.X - band + (bounds.Width+band)*progress
	list.PushClipRect(bounds)
	defer list.PopClipRect()
	list.DrawImage(shimmerTextures()[variant], ui.Rect{X: left, Y: bounds.Y, Width: band, Height: bounds.Height})
}

// inlineAnimationPaint applies one shared span phase to every measured fragment.
// Selection retains its native highlight; shimmer does not tint selected text.
func inlineAnimationPaint(base func(*ui.DisplayList, ui.Rect, bool, func()), a *Animation, progress float32, dark bool, matrix ui.AffineTransform) func(*ui.DisplayList, ui.Rect, bool, func()) {
	if (a == nil || a.Type == AnimationPulse) && matrix == ui.IdentityTransform() {
		return base
	}
	return func(list *ui.DisplayList, bounds ui.Rect, selected bool, draw func()) {
		if matrix != ui.IdentityTransform() {
			x, y := bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2
			list.PushTransform(ui.TranslationTransform(-x, -y).Then(matrix).Then(ui.TranslationTransform(x, y)))
			defer list.PopTransform()
		}
		base(list, bounds, selected, func() {
			draw()
			if a != nil && a.Type == AnimationShimmer && !selected {
				paintShimmer(list, bounds, progress, dark)
			}
		})
	}
}
