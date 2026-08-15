package textoverlay

import (
	"image"
	"runtime"
	"sync"
	"time"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/overlay"
)

const (
	runtimeTextDefaultWidth = float32(400)
	runtimeTextMinimumWidth = float32(100)
	DefaultFontSize         = float32(14)
	runtimeTextPaddingX     = float32(18)
	runtimeTextPaddingY     = float32(12)
	// runtimeTextSystemCornerRadius is the Linux tooltip outline. Windows and
	// macOS clip the overlay window themselves and must not stroke a second radius.
	runtimeTextSystemCornerRadius = float32(14)
	runtimeTextLeadingGap         = float32(8)
	runtimeTextCloseSize          = float32(20)
	runtimeTextCloseGap           = float32(8)
	runtimeTextCopySize           = float32(28)
	runtimeTextCopyGap            = float32(8)
	runtimeTextBottomPadding      = float32(4)
	runtimeTextTitleBarHeight     = float32(40)
	runtimeTextTitleButtonSize    = float32(32)
	runtimeTextTitleIconSize      = float32(20)
	runtimeTextTooltipHeight      = float32(28)
	runtimeTextTooltipGap         = float32(4)
)

var (
	runtimeTextOverlays  = map[string]*runtimeTextOverlay{}
	runtimeTextOverlaysM sync.Mutex
)

type runtimeTextLayout struct {
	windowSize     woxui.Size
	contentSize    woxui.Size
	titleBarHeight float32
	viewportHeight float32
	textWidth      float32
	textLayout     woxwidget.TextBlockLayout
	scrollable     bool
}

type runtimeTextOverlay struct {
	id        string
	options   Options
	window    *woxui.Window
	scroll    *woxwidget.ScrollController
	layout    runtimeTextLayout
	icon      *woxui.Image
	tipIcon   *woxui.Image
	titleIcon *woxui.Image

	hovered      bool
	followBottom bool
	copied       bool
	copyHovered  bool
	copyAnchor   woxui.Rect
	autoClose    *time.Timer
	copyFeedback *time.Timer
}

// showRuntimeTextOverlay uses the shared Go UI runtime for every text overlay.
func showRuntimeTextOverlay(opts Options) bool {
	if opts.Window.ID == "" {
		return false
	}
	icon, err := runtimeOverlayImage(opts.Icon)
	if err != nil {
		return false
	}
	tipIcon, err := runtimeOverlayImage(opts.TooltipIcon)
	if err != nil {
		return false
	}
	titleIcon, err := runtimeOverlayImage(opts.TitleIcon)
	if err != nil {
		return false
	}

	runtimeTextOverlaysM.Lock()
	instance := runtimeTextOverlays[opts.Window.ID]
	created := instance == nil
	if created {
		instance = &runtimeTextOverlay{id: opts.Window.ID, scroll: woxwidget.NewScrollController(0)}
		runtimeTextOverlays[instance.id] = instance
	}
	runtimeTextOverlaysM.Unlock()
	instance.options = opts
	instance.icon = icon
	instance.tipIcon = tipIcon
	instance.titleIcon = titleIcon
	instance.followBottom = opts.FollowScroll && (created || instance.followBottom || instance.scroll.Offset() >= instance.scroll.MaxOffset()-1)
	shown := overlay.ShowWindow(opts.Window, overlay.View{
		Kind: "text",
		Measure: func(window *woxui.Window, workArea woxui.Rect) woxui.Size {
			instance.window = window
			instance.layout = instance.measure(window, workArea)
			return instance.layout.windowSize
		},
		Build: func(window *woxui.Window, frame woxui.FrameInfo) woxwidget.Widget {
			instance.window = window
			return instance.build(frame)
		},
		OnPointer: func(event woxui.PointerEvent) { instance.hovered = event.Kind != woxui.PointerLeave },
		OnDispose: instance.dispose,
	})
	if shown {
		instance.restartAutoClose()
	}
	return shown
}

func runtimeOverlayImage(source image.Image) (*woxui.Image, error) {
	if source == nil {
		return nil, nil
	}
	return woxui.NewImage(source)
}

func (instance *runtimeTextOverlay) dispose() {
	instance.stopTimers()
	runtimeTextOverlaysM.Lock()
	if runtimeTextOverlays[instance.id] == instance {
		delete(runtimeTextOverlays, instance.id)
	}
	runtimeTextOverlaysM.Unlock()
}

func (instance *runtimeTextOverlay) stopTimers() {
	overlay.Close(instance.copyTooltipID())
	if instance.autoClose != nil {
		instance.autoClose.Stop()
		instance.autoClose = nil
	}
	if instance.copyFeedback != nil {
		instance.copyFeedback.Stop()
		instance.copyFeedback = nil
	}
}

func (instance *runtimeTextOverlay) restartAutoClose() {
	if instance.autoClose != nil {
		instance.autoClose.Stop()
		instance.autoClose = nil
	}
	if instance.options.AutoCloseSeconds <= 0 {
		return
	}
	instance.autoClose = time.AfterFunc(time.Duration(instance.options.AutoCloseSeconds)*time.Second, instance.tryAutoClose)
}

func (instance *runtimeTextOverlay) tryAutoClose() {
	_ = woxui.Call(func() {
		runtimeTextOverlaysM.Lock()
		current := runtimeTextOverlays[instance.id]
		runtimeTextOverlaysM.Unlock()
		if current != instance {
			return
		}
		if instance.hovered {
			instance.autoClose = time.AfterFunc(250*time.Millisecond, instance.tryAutoClose)
			return
		}
		overlay.RequestClose(instance.id)
	})
}

func (instance *runtimeTextOverlay) measure(window *woxui.Window, workArea woxui.Rect) runtimeTextLayout {
	style := woxui.TextStyle{Size: textOverlayFontSize(instance.options)}
	leadingWidth := float32(0)
	if instance.options.Loading || instance.icon != nil {
		leadingWidth = float32(instance.options.IconSize)
		if leadingWidth <= 0 {
			leadingWidth = 24
		}
	}
	tipWidth := float32(0)
	if instance.tipIcon != nil {
		tipWidth = float32(instance.options.TooltipIconSize)
		if tipWidth <= 0 {
			tipWidth = 18
		}
	}
	titleBarHeight := float32(0)
	if runtimeTextUsesTitleBar(instance.options) {
		titleBarHeight = runtimeTextTitleBarHeight
	}
	closeReserve := float32(0)
	if instance.options.Closable && titleBarHeight == 0 {
		closeReserve = runtimeTextCloseSize + runtimeTextCloseGap
	}
	leadingReserve := leadingWidth
	if leadingWidth > 0 {
		leadingReserve += runtimeTextLeadingGap
	}
	tipReserve := tipWidth
	if tipWidth > 0 {
		tipReserve += runtimeTextLeadingGap
	}

	natural, _ := window.MeasureText(instance.options.Message, style)
	padding := textOverlayPadding(instance.options)
	windowWidth := natural.Size.Width + leadingReserve + tipReserve + closeReserve + padding.Left + padding.Right
	if titleBarHeight > 0 && (instance.options.Title != "" || instance.titleIcon != nil) {
		titleWidth := float32(0)
		if instance.options.Title != "" {
			title, _ := window.MeasureText(instance.options.Title, woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold})
			titleWidth = title.Size.Width
		}
		titleLeading := float32(12)
		if instance.titleIcon != nil {
			titleLeading = 40
		}
		actionWidth := float32(12)
		if instance.options.Closable {
			actionWidth += runtimeTextTitleButtonSize + 2
		}
		if instance.options.ShowCopyButton {
			actionWidth += runtimeTextTitleButtonSize + 2
		}
		windowWidth = max(windowWidth, titleLeading+titleWidth+actionWidth)
	}
	maxWidth := float32(instance.options.Window.MaxWidth)
	if maxWidth <= 0 {
		maxWidth = runtimeTextDefaultWidth
	}
	maxWidth = min(maxWidth, workArea.Width)
	minWidth := float32(instance.options.Window.MinWidth)
	if minWidth <= 0 {
		minWidth = runtimeTextMinimumWidth
	}
	minWidth = min(minWidth, maxWidth)
	windowWidth = min(max(windowWidth, minWidth), maxWidth)
	if instance.options.Window.Width > 0 {
		windowWidth = min(max(float32(instance.options.Window.Width), minWidth), maxWidth)
	}

	contentWidth := max(float32(1), windowWidth-padding.Left-padding.Right)
	textWidth := max(float32(1), contentWidth-leadingReserve-tipReserve-closeReserve)
	textLayout := woxwidget.LayoutTextBlock(window, instance.options.Message, style, textWidth, 0, 0)
	textHeight := textLayout.Size.Height + runtimeTextBottomPadding
	rowHeight := max(textHeight, leadingWidth)
	if instance.options.Closable {
		rowHeight = max(rowHeight, runtimeTextCloseSize)
	}
	copyReserve := float32(0)
	if instance.options.ShowCopyButton && titleBarHeight == 0 {
		copyReserve = runtimeTextCopySize + runtimeTextCopyGap
	}
	naturalHeight := titleBarHeight + rowHeight + copyReserve + padding.Top + padding.Bottom
	maxHeight := workArea.Height
	if instance.options.Window.MaxHeight > 0 {
		maxHeight = min(maxHeight, float32(instance.options.Window.MaxHeight))
	}
	windowHeight := runtimeTextWindowHeight(naturalHeight, float32(instance.options.Window.Height), maxHeight)
	contentHeight := max(float32(1), windowHeight-titleBarHeight-padding.Top-padding.Bottom)
	viewportHeight := max(float32(1), contentHeight-copyReserve)

	return runtimeTextLayout{
		windowSize:     woxui.Size{Width: windowWidth, Height: windowHeight},
		contentSize:    woxui.Size{Width: contentWidth, Height: contentHeight},
		titleBarHeight: titleBarHeight,
		viewportHeight: viewportHeight,
		textWidth:      textWidth,
		textLayout:     textLayout,
		scrollable:     textHeight > viewportHeight,
	}
}

func runtimeTextUsesTitleBar(options Options) bool {
	return options.Title != "" || options.TitleIcon != nil || options.ShowCopyButton
}

func runtimeTextWindowHeight(natural, requested, maximum float32) float32 {
	if requested > 0 {
		natural = requested
	}
	if maximum <= 0 {
		maximum = natural
	}
	return max(runtimeTextPaddingY*2+1, min(natural, maximum))
}

func (instance *runtimeTextOverlay) build(frame woxui.FrameInfo) woxwidget.Widget {
	layout := instance.layout
	textColor := woxui.Color{R: 246, G: 246, B: 246, A: 255}
	mutedColor := woxui.Color{R: 230, G: 230, B: 230, A: 235}
	style := woxui.TextStyle{Size: textOverlayFontSize(instance.options)}
	text := woxwidget.TextBlock{
		Value: instance.options.Message, Style: style, Color: textColor, Width: layout.textWidth,
		LineHeight: layout.textLayout.LineHeight, Centered: instance.options.CenterContent, Layout: &layout.textLayout,
	}
	var textContent woxwidget.Widget = text
	if layout.scrollable {
		var keepVisible *woxwidget.ScrollRange
		if instance.followBottom {
			end := layout.textLayout.Size.Height + runtimeTextBottomPadding
			keepVisible = &woxwidget.ScrollRange{Start: end, End: end}
		}
		textContent = woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
			Key: "text-overlay-scroll", Content: text, Width: layout.textWidth, Height: layout.viewportHeight,
			ContentHeight: layout.textLayout.Size.Height + runtimeTextBottomPadding, Controller: instance.scroll,
			KeepVisible: keepVisible, ThumbColor: mutedColor, AlwaysShowScrollbar: true,
			OnOffsetChanged: func(offset float32) {
				instance.followBottom = instance.options.FollowScroll && offset >= instance.scroll.MaxOffset()-1
			},
		})
	}

	row := []woxwidget.Widget{}
	if instance.options.Loading {
		size := float32(instance.options.IconSize)
		if size <= 0 {
			size = 24
		}
		row = append(row, woxcomponent.WoxLoadingIndicator(size, textColor))
	} else if instance.icon != nil {
		size := float32(instance.options.IconSize)
		if size <= 0 {
			size = 24
		}
		row = append(row, woxwidget.Image{Source: instance.icon, Width: size, Height: size, Fit: woxwidget.ImageFitContain})
	}
	row = append(row, textContent)
	if instance.tipIcon != nil {
		size := float32(instance.options.TooltipIconSize)
		if size <= 0 {
			size = 18
		}
		row = append(row, woxwidget.Image{Source: instance.tipIcon, Width: size, Height: size, Fit: woxwidget.ImageFitContain})
	}
	if instance.options.Closable && layout.titleBarHeight == 0 {
		row = append(row, woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "text-overlay-close", Label: "Close", Icon: woxcomponent.CloseGlyph(14, textColor),
			Width: runtimeTextCloseSize, Height: runtimeTextCloseSize, Radius: runtimeTextCloseSize / 2,
			HoverBackground: woxui.Color{R: 255, G: 255, B: 255, A: 35}, OnTap: func() { overlay.RequestClose(instance.id) },
		}))
	}
	crossAxis := woxwidget.CrossAxisCenter
	rowHeight := max(layout.textLayout.Size.Height+runtimeTextBottomPadding, runtimeTextCloseSize)
	if instance.options.Loading || instance.icon != nil {
		iconSize := float32(instance.options.IconSize)
		if iconSize <= 0 {
			iconSize = 24
		}
		rowHeight = max(rowHeight, iconSize)
	}
	rowWidget := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: runtimeTextLeadingGap, CrossAxisAlignment: crossAxis, Children: row}
	children := []woxwidget.StackChild{}
	if layout.scrollable {
		children = append(children, woxwidget.StackChild{Top: 0, Child: rowWidget})
	} else {
		// Center the row vertically: rowHeight carries the bottom padding slack,
		// so a top-aligned Stack child would leave the text visually off-center.
		children = append(children, woxwidget.StackChild{Child: woxwidget.Align{Width: layout.contentSize.Width, Height: layout.viewportHeight, Horizontal: 0, Vertical: 0.5, Child: rowWidget}})
	}
	if instance.options.ShowCopyButton && layout.titleBarHeight == 0 {
		label := instance.options.CopyButtonTooltip
		if instance.copied && instance.options.CopyButtonSuccessTooltip != "" {
			label = instance.options.CopyButtonSuccessTooltip
		}
		icon := woxcomponent.CopyGlyph(14, textColor)
		background := woxui.Color{R: 255, G: 255, B: 255, A: 35}
		if instance.copied {
			background = woxui.Color{R: 45, G: 112, B: 82, A: 220}
		}
		children = append(children, woxwidget.StackChild{Right: 0, Bottom: 0, AnchorRight: true, AnchorBottom: true, Child: woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "text-overlay-copy", Label: label, Icon: icon, Width: runtimeTextCopySize, Height: runtimeTextCopySize, Radius: 6,
			Background: background, HoverBackground: woxui.Color{R: 255, G: 255, B: 255, A: 55}, OnTap: instance.copy,
		})})
	}

	content := woxwidget.Stack{Width: layout.contentSize.Width, Height: layout.contentSize.Height, Children: children}
	radius, borderWidth, borderColor := runtimeTextWindowChrome(runtime.GOOS)
	rootChildren := []woxwidget.StackChild{{Child: woxwidget.Container{
		Width: frame.Size.Width, Height: frame.Size.Height,
		// No panel fill: every platform window supplies its own material
		// (macOS NSVisualEffectView, Windows Desktop Acrylic). Windows applies
		// that HWND backdrop even when the overlay is WS_EX_NOACTIVATE.
		Radius: radius, BorderWidth: borderWidth, BorderColor: borderColor,
	}}}
	if layout.titleBarHeight > 0 {
		rootChildren = append(rootChildren, woxwidget.StackChild{Child: instance.buildTitleBar(frame.Size.Width, textColor)})
	}
	padding := textOverlayPadding(instance.options)
	rootChildren = append(rootChildren, woxwidget.StackChild{Left: padding.Left, Top: layout.titleBarHeight + padding.Top, Child: content})
	root := woxwidget.Stack{Width: frame.Size.Width, Height: frame.Size.Height, Children: rootChildren}
	if instance.options.OnClick != nil {
		return woxwidget.Gesture{ID: "text-overlay-click", OnTap: func() { instance.options.OnClick() }, Child: root}
	}
	return root
}

// textOverlayFontSize resolves the message font size, defaulting to the shared
// overlay size when the caller did not request a custom one.
func textOverlayFontSize(options Options) float32 {
	if options.FontSize > 0 {
		return options.FontSize
	}
	return DefaultFontSize
}

// textOverlayPadding resolves the panel padding, defaulting to the shared
// overlay padding when the caller did not request a custom one.
func textOverlayPadding(options Options) woxwidget.Insets {
	if options.Padding != (woxwidget.Insets{}) {
		return options.Padding
	}
	return woxwidget.Insets{Left: runtimeTextPaddingX, Top: runtimeTextPaddingY, Right: runtimeTextPaddingX, Bottom: runtimeTextPaddingY}
}

// runtimeTextWindowChrome returns the widget-drawn window outline. Windows DWM
// and macOS NSVisualEffectView already clip the overlay, so a second 14px stroke
// reads as double corners. Linux utility windows stay square; nonactivating
// tooltips still need this stroke so their alpha corners read as rounded.
func runtimeTextWindowChrome(goos string) (radius, borderWidth float32, borderColor woxui.Color) {
	if goos == "linux" {
		return runtimeTextSystemCornerRadius, 1, woxui.Color{R: 255, G: 255, B: 255, A: 30}
	}
	return 0, 0, woxui.Color{}
}

// runtimeTextCopyTooltip builds the compact copy-feedback overlay.
func runtimeTextCopyTooltip(width float32, label string, style woxui.TextStyle, foreground woxui.Color) woxwidget.Container {
	radius, borderWidth, borderColor := runtimeTextWindowChrome(runtime.GOOS)
	return woxwidget.Container{
		Width: width, Height: runtimeTextTooltipHeight, Radius: radius, BorderWidth: borderWidth, BorderColor: borderColor,
		Child: woxwidget.Align{Width: width, Height: runtimeTextTooltipHeight, Horizontal: .5, Vertical: .5, Child: woxwidget.TextBlock{
			Value: label, Width: width - 16, Height: 18, MaxLines: 1, Centered: true, Style: style, Color: foreground,
		}},
	}
}

func (instance *runtimeTextOverlay) buildTitleBar(width float32, foreground woxui.Color) woxwidget.Widget {
	hoverBackground := woxui.Color{R: 255, G: 255, B: 255, A: 20}
	children := []woxwidget.StackChild{
		{Child: woxwidget.Container{Width: width, Height: runtimeTextTitleBarHeight}},
		{Top: runtimeTextTitleBarHeight - 1, Child: woxwidget.Container{Width: width, Height: 1, Color: woxui.Color{R: 255, G: 255, B: 255, A: 76}}},
	}
	closeWidth := runtimeTextTitleButtonSize
	closeRightMargin := float32(6)
	if runtime.GOOS == "windows" && instance.options.Closable {
		closeWidth = 46
		closeRightMargin = 0
	}
	actionWidth := float32(0)
	if instance.options.Closable {
		actionWidth += closeWidth
	}
	if instance.options.ShowCopyButton {
		actionWidth += runtimeTextTitleButtonSize
	}
	if instance.options.Closable && instance.options.ShowCopyButton {
		actionWidth += 2
	}
	actionsLeft := width - closeRightMargin - actionWidth
	titleLeft := float32(12)
	if instance.titleIcon != nil {
		children = append(children, woxwidget.StackChild{Left: 12, Top: 10, Child: woxwidget.Image{Source: instance.titleIcon, Width: runtimeTextTitleIconSize, Height: runtimeTextTitleIconSize, Fit: woxwidget.ImageFitContain}})
		titleLeft = 40
	}
	if instance.options.Title != "" {
		children = append(children, woxwidget.StackChild{Left: titleLeft, Top: 9, Child: woxwidget.TextBlock{
			Value: instance.options.Title, Width: max(float32(0), actionsLeft-titleLeft-8), Height: 24, MaxLines: 1,
			Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: foreground,
		}})
	}
	right := width - 6
	if instance.options.Closable {
		closeLeft := width - 6 - closeWidth
		closeTop := float32(4)
		if runtime.GOOS == "windows" {
			closeLeft = width - closeWidth
			closeTop = 0
		}
		children = append(children, woxwidget.StackChild{Left: closeLeft, Top: closeTop, Child: overlay.TitleBarCloseButton(runtime.GOOS == "windows", "text-overlay-close", foreground, func() { overlay.RequestClose(instance.id) })})
		right = closeLeft - 2
	}
	if instance.options.ShowCopyButton {
		label := instance.options.CopyButtonTooltip
		if label == "" {
			label = "Copy"
		}
		background := woxui.Color{}
		if instance.copied {
			if instance.options.CopyButtonSuccessTooltip != "" {
				label = instance.options.CopyButtonSuccessTooltip
			}
			background = woxui.Color{R: 45, G: 112, B: 82, A: 220}
		}
		children = append(children, woxwidget.StackChild{Left: right - runtimeTextTitleButtonSize, Top: 4, Child: woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "text-overlay-copy", Label: label, Icon: woxcomponent.CopyGlyph(15, foreground),
			Width: runtimeTextTitleButtonSize, Height: runtimeTextTitleButtonSize, Radius: 5,
			Background: background, HoverBackground: hoverBackground, OnTap: instance.copy, OnHoverAt: func(inside bool, bounds woxui.Rect) {
				instance.copyHovered = inside
				instance.copyAnchor = bounds
				if inside {
					instance.showCopyTooltip()
				} else {
					overlay.Close(instance.copyTooltipID())
				}
			},
		})})
	}
	return woxwidget.Stack{Width: width, Height: runtimeTextTitleBarHeight, Children: children}
}

func (instance *runtimeTextOverlay) copyTooltipID() string {
	return instance.id + ".copy-tooltip"
}

func (instance *runtimeTextOverlay) copyTooltipLabel() string {
	if instance.copied && instance.options.CopyButtonSuccessTooltip != "" {
		return instance.options.CopyButtonSuccessTooltip
	}
	if instance.options.CopyButtonTooltip != "" {
		return instance.options.CopyButtonTooltip
	}
	return "Copy"
}

func runtimeTextCopyTooltipAnchor(windowBounds, buttonBounds woxui.Rect) (float64, float64) {
	return float64(windowBounds.X + buttonBounds.X + buttonBounds.Width/2), float64(windowBounds.Y + buttonBounds.Y - runtimeTextTooltipGap)
}

// showCopyTooltip places feedback above the title bar in a separate window so it cannot cover the body text.
func (instance *runtimeTextOverlay) showCopyTooltip() {
	if instance.window == nil {
		return
	}
	windowBounds, err := instance.window.Bounds()
	if err != nil {
		return
	}
	label := instance.copyTooltipLabel()
	style := woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}
	metrics, _ := instance.window.MeasureText(label, style)
	width := max(float32(48), metrics.Size.Width+16)
	x, y := runtimeTextCopyTooltipAnchor(windowBounds, instance.copyAnchor)
	foreground := woxui.Color{R: 246, G: 246, B: 246, A: 255}
	overlay.ShowWindow(overlay.WindowOptions{
		ID: instance.copyTooltipID(), Topmost: true, AbsolutePosition: true, Anchor: overlay.AnchorBottomCenter,
		OffsetX: x, OffsetY: y, Width: float64(width), Height: float64(runtimeTextTooltipHeight),
	}, overlay.View{Kind: "text-copy-tooltip", Build: func(_ *woxui.Window, _ woxui.FrameInfo) woxwidget.Widget {
		return runtimeTextCopyTooltip(width, label, style, foreground)
	}})
}

// copy runs the caller's clipboard action and keeps feedback state inside the runtime host.
func (instance *runtimeTextOverlay) copy() {
	if instance.options.OnClick == nil || !instance.options.OnClick() {
		return
	}
	instance.copied = true
	_ = instance.window.Invalidate()
	if instance.copyHovered {
		instance.showCopyTooltip()
	}
	if instance.copyFeedback != nil {
		instance.copyFeedback.Stop()
	}
	instance.copyFeedback = time.AfterFunc(1200*time.Millisecond, func() {
		_ = woxui.Call(func() {
			instance.copied = false
			_ = instance.window.Invalidate()
			if instance.copyHovered {
				instance.showCopyTooltip()
			}
		})
	})
}
