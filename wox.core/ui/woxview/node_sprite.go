package woxview

import (
	"fmt"
	"math"
	"time"

	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// SpriteClip addresses row-major cells in one immutable atlas.
type SpriteClip struct {
	Name   string  `json:"name"`
	Frames []int   `json:"frames"`
	FPS    float32 `json:"fps"`
	Loop   bool    `json:"loop,omitempty"`
}

// SpriteProps changes source rectangles without decoding, copying, or uploading frames.
type SpriteProps struct {
	Styled
	Source     *ImageSource `json:"source"`
	Columns    int          `json:"columns"`
	Rows       int          `json:"rows"`
	Frame      int          `json:"frame,omitempty"`
	Clips      []SpriteClip `json:"clips,omitempty"`
	Clip       string       `json:"clip,omitempty"`
	Paused     bool         `json:"paused,omitempty"`
	FlipX      bool         `json:"flipX,omitempty"`
	PlaybackID string       `json:"playbackId,omitempty"`
	Elapsed    int          `json:"elapsed,omitempty"`
	image      ImageProps
	selected   *SpriteClip
}

// prepare bounds all producer-controlled frame data before layout.
func (p *SpriteProps) prepare() error {
	if p.Source == nil {
		return fmt.Errorf("sprite.source is required")
	}
	p.image = ImageProps{Source: p.Source}
	if err := p.image.prepare(); err != nil {
		return err
	}
	if p.Columns < 1 || p.Columns > 64 || p.Rows < 1 || p.Rows > 64 {
		return fmt.Errorf("sprite columns and rows must be 1..64")
	}
	if p.Frame < 0 || p.Frame >= p.Columns*p.Rows {
		return fmt.Errorf("sprite.frame is outside atlas")
	}
	if len(p.Clips) > 64 || len(p.PlaybackID) > 256 || p.Elapsed < 0 || p.Elapsed > 86400000 {
		return fmt.Errorf("sprite playback exceeds limits")
	}
	seen := map[string]bool{}
	p.selected = nil
	for i := range p.Clips {
		clip := &p.Clips[i]
		if clip.Name == "" || len(clip.Name) > 128 || seen[clip.Name] || len(clip.Frames) == 0 || len(clip.Frames) > 256 || math.IsNaN(float64(clip.FPS)) || clip.FPS < 1 || clip.FPS > 60 {
			return fmt.Errorf("invalid sprite clip %q", clip.Name)
		}
		seen[clip.Name] = true
		for _, frame := range clip.Frames {
			if frame < 0 || frame >= p.Columns*p.Rows {
				return fmt.Errorf("sprite clip frame is outside atlas")
			}
		}
		if clip.Name == p.Clip {
			p.selected = clip
		}
	}
	if p.Clip != "" && p.selected == nil {
		return fmt.Errorf("unknown sprite clip %q", p.Clip)
	}
	return p.Style.validate()
}

// buildSprite keeps one image identity across every named clip and source rectangle.
func buildSprite(b *builder, node *Node, id string) widget.Widget {
	p := node.Props.(*SpriteProps)
	style := p.Style.resolve(b.theme())
	width, height := style.Width, style.Height
	if width <= 0 {
		width = 32
	}
	if height <= 0 {
		height = width
	}
	image := b.view.images.get(&p.image, !b.theme().Dark, width, height)
	build := func(index int) widget.Widget {
		var content widget.Widget = widget.Container{Width: width, Height: height}
		if image != nil && !image.IsAnimated() {
			cellWidth, cellHeight := float32(image.Width)/float32(p.Columns), float32(image.Height)/float32(p.Rows)
			content = widget.Image{Source: image, Width: width, Height: height, SourceRect: ui.Rect{X: float32(index%p.Columns) * cellWidth, Y: float32(index/p.Columns) * cellHeight, Width: cellWidth, Height: cellHeight}}
			if p.FlipX {
				content = widget.Transform{Matrix: ui.ScaleTransform(-1, 1), OriginX: .5, OriginY: .5, Child: content}
			}
		}
		return style.wrapOuter(b.theme(), content)
	}
	if p.selected == nil || len(p.selected.Frames) == 1 {
		frame := p.Frame
		if p.selected != nil {
			frame = p.selected.Frames[0]
		}
		return build(frame)
	}
	clip := p.selected
	step := time.Duration(float64(time.Second) / float64(clip.FPS))
	return widget.TimelineAnimation{Key: widget.Key(fmt.Sprintf("%p/%s/sprite", b.view, id)), PlaybackID: fmt.Sprintf("%d:%s%s", len(p.PlaybackID), p.PlaybackID, p.Clip), Duration: step * time.Duration(len(clip.Frames)), Elapsed: time.Duration(p.Elapsed) * time.Millisecond, Step: step, Loop: clip.Loop, Paused: p.Paused,
		Builder: func(elapsed time.Duration) widget.Widget {
			return build(clip.Frames[min(int(elapsed/step), len(clip.Frames)-1)])
		},
	}
}
