package preview

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// MarkdownPreviewProps contains one parsed Markdown document and its native actions.
type MarkdownPreviewProps struct {
	ID            string
	Document      woxcomponent.MarkdownDocument
	Width         float32
	Height        float32
	InitialOffset float32
	Theme         woxcomponent.Theme
	Window        *woxui.Window
	ResolveImage  func(source string) (*woxui.Image, string)
	OnOpenImage   func(source string)
	OnOpenLink    func(target string)
}

// MarkdownPreviewView places the shared Markdown component in the generic preview scroller.
func MarkdownPreviewView(props MarkdownPreviewProps) woxwidget.Widget {
	innerWidth := max(float32(0), props.Width-40)
	innerHeight := max(float32(0), props.Height-40)
	content := woxcomponent.WoxMarkdown(woxcomponent.MarkdownProps{
		ID: props.ID, Document: props.Document, Width: innerWidth, Theme: props.Theme, Window: props.Window,
		ResolveImage: props.ResolveImage, OnOpenImage: props.OnOpenImage, OnOpenLink: props.OnOpenLink,
	})
	return woxwidget.Container{
		Width: props.Width, Height: props.Height, Padding: woxwidget.UniformInsets(20),
		Child: woxwidget.ScrollView{
			Key: woxwidget.Key("markdown-scroll-" + props.ID), ID: "markdown-scroll-" + props.ID, InitialOffset: props.InitialOffset,
			Width: innerWidth, Height: innerHeight, ContentHeight: innerHeight, Child: content,
		},
	}
}
