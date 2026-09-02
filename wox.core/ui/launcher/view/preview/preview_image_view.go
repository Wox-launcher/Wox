package preview

import (
	"math"
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	previewImageMinZoom          = float32(1)
	previewImageMaxZoom          = float32(8)
	previewImageWheelSensitivity = 0.005
	previewImageWheelMinFactor   = float32(0.8)
	previewImageWheelMaxFactor   = float32(1.25)
	previewImageFitPadding       = float32(24)
	previewImageOverlayGestureID = "preview-image-overlay"
)

// PreviewImageProps contains a resolved preview image or its loading state.
type PreviewImageProps struct {
	ID           string
	Width        float32
	Height       float32
	Image        *woxui.Image
	Message      string
	MessageColor woxui.Color
	LoadingColor woxui.Color
	OnTap        func()
}

type previewImageState struct {
	zoom      float32
	left      float32
	top       float32
	hasOffset bool
	panX      float32
	panY      float32
}

type previewImageFrame struct {
	DrawWidth  float32
	DrawHeight float32
	Left       float32
	Top        float32
}

// PreviewImage builds a centered image preview that can zoom with the scroll wheel.
func PreviewImage(props PreviewImageProps) woxwidget.Widget {
	if props.Image == nil {
		if message := strings.TrimSpace(props.Message); message != "" {
			return woxwidget.Container{
				Width: props.Width, Height: props.Height, Padding: woxwidget.UniformInsets(24),
				Child: woxwidget.TextBlock{Value: message, Width: max(float32(0), props.Width-48), Height: max(float32(0), props.Height-48), Style: woxui.TextStyle{Size: 13}, Color: props.MessageColor},
			}
		}
		return PreviewLoading(props.Width, props.Height, props.LoadingColor)
	}
	key := props.ID
	if key == "" {
		key = "preview-image"
	}
	return woxwidget.Stateful{
		Key: woxwidget.Key(key), Type: (*previewImageState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &previewImageState{} },
	}
}

func (s *previewImageState) InitState(_ woxwidget.StateContext, _ any) {
	s.zoom = previewImageMinZoom
}

func (*previewImageState) DidUpdateWidget(woxwidget.StateContext, any, any) {}

func (*previewImageState) Dispose() {}

func (s *previewImageState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(PreviewImageProps)
	if s.zoom <= 0 {
		s.zoom = previewImageMinZoom
	}
	if props.Image != nil && props.Image.IsAnimated() {
		key := props.ID
		if key == "" {
			key = "preview-image"
		}
		return woxwidget.FrameAnimation{
			Key:    woxwidget.Key(key + "-gif"),
			Delays: props.Image.FrameDelays(),
			Builder: func(index int) woxwidget.Widget {
				frameProps := props
				frameProps.Image = props.Image.Frame(index)
				return s.previewImageGesture(context, frameProps)
			},
		}
	}
	return s.previewImageGesture(context, props)
}

func (s *previewImageState) previewImageGesture(context woxwidget.StateContext, props PreviewImageProps) woxwidget.Widget {
	frame := s.frame(props)
	gesture := woxwidget.Gesture{
		ID: previewImageOverlayGestureID,
		OnPointer: func(event woxui.PointerEvent) bool {
			if event.Kind != woxui.PointerScroll || event.Scroll.Y == 0 {
				return false
			}
			factor := previewImageZoomFactor(event.Scroll.Y)
			context.SetState(func() {
				s.applyWheelZoom(props, factor, event.Position.X, event.Position.Y)
			})
			return true
		},
		OnTap: props.OnTap,
		Child: woxwidget.Painter{Width: props.Width, Height: props.Height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			if props.Image == nil || frame.DrawWidth <= 0 || frame.DrawHeight <= 0 {
				return
			}
			displayList.PushClipRect(bounds)
			displayList.DrawImage(props.Image, woxui.Rect{
				X: bounds.X + frame.Left, Y: bounds.Y + frame.Top, Width: frame.DrawWidth, Height: frame.DrawHeight,
			})
			displayList.PopClipRect()
		}},
	}
	if s.zoom > previewImageMinZoom {
		gesture.Cursor = woxui.PointerCursorMove
		gesture.OnPanStart = func(position woxui.Point) {
			s.panX = position.X
			s.panY = position.Y
		}
		gesture.OnPanUpdate = func(position woxui.Point) {
			dx := position.X - s.panX
			dy := position.Y - s.panY
			context.SetState(func() {
				s.applyPan(props, dx, dy)
				s.panX = position.X
				s.panY = position.Y
			})
		}
	}
	return gesture
}

func (s *previewImageState) frame(props PreviewImageProps) previewImageFrame {
	drawWidth, drawHeight := previewImageDrawSize(props, s.zoom)
	left, top := s.resolvedOrigin(props, drawWidth, drawHeight)
	return previewImageFrame{DrawWidth: drawWidth, DrawHeight: drawHeight, Left: left, Top: top}
}

func (s *previewImageState) resolvedOrigin(props PreviewImageProps, drawWidth, drawHeight float32) (float32, float32) {
	if !s.hasOffset {
		return (props.Width - drawWidth) * 0.5, (props.Height - drawHeight) * 0.5
	}
	return clampPreviewImageOrigin(s.left, s.top, drawWidth, drawHeight, props.Width, props.Height)
}

func (s *previewImageState) applyWheelZoom(props PreviewImageProps, factor, cursorX, cursorY float32) {
	drawWidth, drawHeight := previewImageDrawSize(props, s.zoom)
	left, top := s.resolvedOrigin(props, drawWidth, drawHeight)
	s.zoom, s.left, s.top = applyPreviewImageWheelZoom(s.zoom, left, top, factor, cursorX, cursorY, props)
	s.hasOffset = true
}

func (s *previewImageState) applyPan(props PreviewImageProps, dx, dy float32) {
	drawWidth, drawHeight := previewImageDrawSize(props, s.zoom)
	left, top := s.resolvedOrigin(props, drawWidth, drawHeight)
	s.left, s.top = clampPreviewImageOrigin(left+dx, top+dy, drawWidth, drawHeight, props.Width, props.Height)
	s.hasOffset = true
}

func previewImageFitScale(imageWidth, imageHeight, viewportWidth, viewportHeight float32) float32 {
	availableWidth := max(float32(0), viewportWidth-previewImageFitPadding)
	availableHeight := max(float32(0), viewportHeight-previewImageFitPadding)
	if imageWidth <= 0 || imageHeight <= 0 || availableWidth <= 0 || availableHeight <= 0 {
		return 0
	}
	return min(availableWidth/imageWidth, availableHeight/imageHeight)
}

func previewImageDrawSize(props PreviewImageProps, zoom float32) (float32, float32) {
	if props.Image == nil {
		return 0, 0
	}
	fit := previewImageFitScale(float32(props.Image.Width), float32(props.Image.Height), props.Width, props.Height)
	return float32(props.Image.Width) * fit * zoom, float32(props.Image.Height) * fit * zoom
}

func clampPreviewImageZoom(zoom float32) float32 {
	return min(previewImageMaxZoom, max(previewImageMinZoom, zoom))
}

func previewImageZoomFactor(scrollY float32) float32 {
	if scrollY == 0 {
		return 1
	}
	return min(previewImageWheelMaxFactor, max(previewImageWheelMinFactor, float32(math.Exp(float64(scrollY)*previewImageWheelSensitivity))))
}

// applyPreviewImageWheelZoom scales around the pointer, then keeps the image in view.
func applyPreviewImageWheelZoom(zoom, left, top, factor, cursorX, cursorY float32, props PreviewImageProps) (float32, float32, float32) {
	next := clampPreviewImageZoom(zoom * factor)
	if next != zoom {
		scale := next / zoom
		left = cursorX - (cursorX-left)*scale
		top = cursorY - (cursorY-top)*scale
	}
	drawWidth, drawHeight := previewImageDrawSize(props, next)
	left, top = clampPreviewImageOrigin(left, top, drawWidth, drawHeight, props.Width, props.Height)
	return next, left, top
}

func clampPreviewImageOrigin(left, top, drawWidth, drawHeight, viewportWidth, viewportHeight float32) (float32, float32) {
	if drawWidth <= viewportWidth {
		left = (viewportWidth - drawWidth) * 0.5
	} else {
		left = min(float32(0), max(viewportWidth-drawWidth, left))
	}
	if drawHeight <= viewportHeight {
		top = (viewportHeight - drawHeight) * 0.5
	} else {
		top = min(float32(0), max(viewportHeight-drawHeight, top))
	}
	return left, top
}
