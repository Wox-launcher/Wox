package preview

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// PreviewImageProps contains a resolved preview image or its loading state.
type PreviewImageProps struct {
	Width        float32
	Height       float32
	Image        *woxui.Image
	Message      string
	MessageColor woxui.Color
	OnTap        func()
}

// PreviewImage builds a centered image preview that can open an overlay.
func PreviewImage(props PreviewImageProps) woxwidget.Widget {
	if props.Image == nil {
		return woxwidget.Container{
			Width: props.Width, Height: props.Height, Padding: woxwidget.UniformInsets(24),
			Child: woxwidget.TextBlock{Value: props.Message, Width: max(float32(0), props.Width-48), Height: max(float32(0), props.Height-48), Style: woxui.TextStyle{Size: 13}, Color: props.MessageColor},
		}
	}
	availableWidth := max(float32(0), props.Width-24)
	availableHeight := max(float32(0), props.Height-24)
	scale := min(availableWidth/float32(props.Image.Width), availableHeight/float32(props.Image.Height))
	drawWidth := float32(props.Image.Width) * scale
	drawHeight := float32(props.Image.Height) * scale
	left := (props.Width - drawWidth) * 0.5
	top := (props.Height - drawHeight) * 0.5
	return woxwidget.Gesture{
		ID: "preview-image-overlay", OnTap: props.OnTap,
		Child: woxwidget.Container{Width: props.Width, Height: props.Height, Child: woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
			{Left: left, Top: top, Child: woxwidget.Image{Source: props.Image, Width: drawWidth, Height: drawHeight}},
		}}},
	}
}
