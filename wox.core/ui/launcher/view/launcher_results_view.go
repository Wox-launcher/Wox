package view

import (
	"fmt"
	"time"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// LauncherResultTail contains one resolved result-tail visual and its measured width.
type LauncherResultTail struct {
	Text         string
	TextCategory string
	Image        *woxui.Image
	Width        float32
	Height       float32
}

// LauncherResultItem contains one visible list result and its controller callbacks.
type LauncherResultItem struct {
	ID          string
	Title       string
	Subtitle    string
	Group       bool
	Selected    bool
	Hovered     bool
	Icon        *woxui.Image
	TitleHeight float32
	Tails       []LauncherResultTail
	TailWidth   float32
	TailHeight  float32
	OnHover     func(bool)
	OnSelect    func()
	OnActivate  func()
}

// LauncherResultsProps contains the prepared viewport slice and result-list geometry.
type LauncherResultsProps struct {
	Width             float32
	Height            float32
	ContentHeight     float32
	Offset            float32
	StartIndex        int
	RowHeight         float32
	RowGap            float32
	ContainerPadding  woxwidget.Insets
	ItemPadding       woxwidget.Insets
	ItemRadius        float32
	TailColor         woxui.Color
	SelectedTailColor woxui.Color
	Theme             woxcomponent.Theme
	DensityScale      float32
	Complete          bool
	Items             []LauncherResultItem
	OnScroll          func(float32)
}

// LauncherSplitContentView places the result list beside a prepared preview.
func LauncherSplitContentView(results, preview woxwidget.Widget) woxwidget.Widget {
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{results, preview}}
}

// LauncherResultsView builds the virtualized result list.
func LauncherResultsView(props LauncherResultsProps) woxwidget.Widget {
	baseHeight := scaledLauncherSize(50, props.DensityScale)
	iconSize := scaledLauncherSize(28, props.DensityScale)
	iconGap := scaledLauncherSize(10, props.DensityScale)
	titleStyle := woxui.TextStyle{Size: scaledLauncherSize(15, props.DensityScale)}
	subtitleStyle := woxui.TextStyle{Size: scaledLauncherSize(12, props.DensityScale)}
	rowWidth := max(float32(0), props.Width-props.ContainerPadding.Left-props.ContainerPadding.Right)
	innerRowWidth := max(float32(0), rowWidth-props.ItemPadding.Left-props.ItemPadding.Right)
	rows := make([]woxwidget.Widget, 0, len(props.Items))
	for _, item := range props.Items {
		item := item
		background := woxui.Color{}
		title := props.Theme.ResultTitle
		subtitle := props.Theme.ResultSubtitle
		tailColor := props.TailColor
		if item.Selected {
			background = props.Theme.SelectedBackground
			title = props.Theme.SelectedTitle
			subtitle = props.Theme.SelectedSubtitle
			tailColor = props.SelectedTailColor
		} else if item.Hovered {
			background = props.Theme.SelectedBackground
			background.A = uint8(float32(background.A)*0.25 + 0.5)
		}
		if item.Group {
			rows = append(rows, woxwidget.Container{
				Width: rowWidth, Height: props.RowHeight, Padding: woxwidget.Insets{Left: scaledLauncherSize(8, props.DensityScale), Top: scaledLauncherSize(18, props.DensityScale)},
				Child: woxwidget.Text{Value: item.Title, Style: titleStyle, Color: title},
			})
			continue
		}
		var icon woxwidget.Widget = woxwidget.Painter{Width: iconSize, Height: iconSize}
		if item.Icon != nil {
			icon = woxwidget.Image{Source: item.Icon, Width: iconSize, Height: iconSize}
		}
		var tail woxwidget.Widget
		if len(item.Tails) > 0 {
			tail = launcherResultTailsWithDensity(item.Tails, item.TailWidth, item.TailHeight, tailColor, item.Selected, props.DensityScale, woxwidget.Key("launcher-result-tails-"+item.ID))
		}
		labelWidth := max(baseHeight, innerRowWidth-iconSize-scaledLauncherSize(20, props.DensityScale)-item.TailWidth)
		labelChildren := []woxwidget.Widget{woxwidget.Text{Value: item.Title, Style: titleStyle, Color: title}}
		labelTop := scaledLauncherSize(7, props.DensityScale)
		labelGap := float32(0)
		if item.Subtitle != "" {
			labelChildren = append(labelChildren, woxwidget.Text{Value: item.Subtitle, Style: subtitleStyle, Color: subtitle})
			labelGap = scaledLauncherSize(2, props.DensityScale)
		} else {
			labelTop = max(float32(0), (baseHeight-item.TitleHeight)/2)
		}
		resultKey := woxwidget.Key(fmt.Sprintf("launcher-result-key-%s", item.ID))
		resultControl := woxwidget.Gesture{
			ID: fmt.Sprintf("result-gesture-%s", item.ID),
			OnHover: func(inside bool) {
				if item.OnHover != nil {
					item.OnHover(inside)
				}
			},
			OnTap: item.OnSelect,
			OnDoubleTap: func() {
				if item.OnSelect != nil {
					item.OnSelect()
				}
				if item.OnActivate != nil {
					item.OnActivate()
				}
			},
			Child: woxwidget.Container{
				Width: rowWidth, Height: props.RowHeight, Radius: props.ItemRadius, Color: background, Padding: props.ItemPadding,
				Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: iconGap, Children: []woxwidget.Widget{
					woxwidget.Container{Width: iconSize, Height: baseHeight, Padding: woxwidget.Insets{Top: max(float32(0), (baseHeight-iconSize)/2)}, Child: icon},
					woxwidget.Container{Width: labelWidth, Height: baseHeight, Padding: woxwidget.Insets{Top: labelTop}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: labelGap, Children: labelChildren}},
					woxwidget.Container{Width: item.TailWidth, Height: baseHeight, Padding: woxwidget.Insets{Top: max(float32(0), (baseHeight-item.TailHeight)/2)}, Child: tail},
				}},
			},
		}
		rows = append(rows, woxwidget.Semantics{
			Key: resultKey, AutomationID: "launcher.result." + item.ID, Role: woxui.AccessibilityRoleListItem,
			Label: item.Title, Description: item.Subtitle, Selected: item.Selected,
			Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
			OnAction: func(action woxui.AccessibilityAction, _ string) error {
				if action == woxui.AccessibilityActionActivate {
					if item.OnSelect != nil {
						item.OnSelect()
					}
					if item.OnActivate != nil {
						item.OnActivate()
					}
				}
				return nil
			},
			Child: resultControl,
		})
	}
	visiblePadding := props.ContainerPadding
	visiblePadding.Top += float32(props.StartIndex) * (props.RowHeight + props.RowGap)
	content := woxwidget.Container{
		Width: props.Width, Height: props.ContentHeight, Padding: visiblePadding,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: props.RowGap, Children: rows},
	}
	state := "loading"
	if props.Complete {
		state = "complete"
	}
	return woxwidget.Semantics{
		Key: "launcher-results-key", AutomationID: "launcher.results", Role: woxui.AccessibilityRoleList, Label: "Search results",
		Value: state, ReadOnly: true,
		Child: launcherResultScrollView(launcherResultScrollProps{
			Content: content, Width: props.Width, Height: props.Height, ContentHeight: props.ContentHeight, Offset: props.Offset,
			ThumbColor: props.Theme.ResultSubtitle, OnScroll: props.OnScroll,
		}),
	}
}

// launcherResultTails restores Flutter's text-tag and image-tail presentation.
func launcherResultTails(tails []LauncherResultTail, width, height float32, foreground woxui.Color, selected bool) woxwidget.Widget {
	return launcherResultTailsWithDensity(tails, width, height, foreground, selected, 1, "")
}

// launcherResultTailsWithDensity scales launcher-only tail geometry without changing fixed theme previews.
func launcherResultTailsWithDensity(tails []LauncherResultTail, width, height float32, foreground woxui.Color, selected bool, densityScale float32, scrollKey woxwidget.Key) woxwidget.Widget {
	itemLeftPadding := scaledLauncherSize(10, densityScale)
	children := make([]woxwidget.Widget, 0, len(tails))
	contentWidth := float32(0)
	for _, item := range tails {
		var content woxwidget.Widget
		if item.Image != nil {
			content = woxwidget.Image{Source: item.Image, Width: item.Width, Height: item.Height}
		} else {
			textColor, background, border := launcherResultTextTailStyle(item.TextCategory, foreground, selected)
			horizontalPadding := scaledLauncherSize(8, densityScale)
			textWidth := max(float32(0), item.Width-horizontalPadding*2)
			content = woxwidget.Container{
				Width: item.Width, Height: item.Height, Radius: item.Height / 2, Color: background, BorderColor: border, BorderWidth: 1,
				Padding: woxwidget.Insets{Left: horizontalPadding, Right: horizontalPadding},
				Child: woxwidget.Align{Width: textWidth, Height: item.Height, Vertical: 0.5, Child: woxwidget.Text{
					Value: item.Text, Style: woxui.TextStyle{Size: scaledLauncherSize(11, densityScale)}, Color: textColor,
				}},
			}
		}
		itemWidth := itemLeftPadding + item.Width
		children = append(children, woxwidget.Container{
			Width: itemWidth, Height: height,
			Padding: woxwidget.Insets{Left: itemLeftPadding},
			Child:   woxwidget.Align{Width: item.Width, Height: height, Vertical: 0.5, Child: content},
		})
		contentWidth += itemWidth
	}
	content := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: children}
	if scrollKey == "" {
		return woxwidget.Clip{Width: width, Height: height, Child: content}
	}
	return woxwidget.ScrollView{Key: scrollKey, Width: width, Height: height, ContentWidth: max(width, contentWidth), Horizontal: true, Child: content}
}

// launcherResultTextTailStyle maps semantic tail categories to Flutter's stable status colors.
func launcherResultTextTailStyle(category string, foreground woxui.Color, selected bool) (woxui.Color, woxui.Color, woxui.Color) {
	semantic := woxui.Color{}
	switch category {
	case "danger":
		semantic = woxui.Color{R: 180, G: 35, B: 24, A: 255}
	case "warning":
		semantic = woxui.Color{R: 181, G: 71, B: 8, A: 255}
	case "success":
		semantic = woxui.Color{R: 2, G: 122, B: 72, A: 255}
	}
	if semantic.A != 0 {
		border := semantic
		border.A = 184
		return woxui.Color{R: 255, G: 255, B: 255, A: 255}, semantic, border
	}
	border := foreground
	border.A = 51
	if selected {
		border.A = 87
	}
	return foreground, woxui.Color{}, border
}

type launcherResultScrollProps struct {
	Content       woxwidget.Widget
	Width         float32
	Height        float32
	ContentHeight float32
	Offset        float32
	ThumbColor    woxui.Color
	OnScroll      func(float32)
}

// launcherResultScrollState owns transient scrollbar visibility, hover, and drag interaction.
type launcherResultScrollState struct {
	visible   bool
	hovered   bool
	dragging  bool
	dragY     float32
	hideAt    time.Time
	hideTimer *time.Timer
}

// launcherResultScrollView keeps list and grid scrolling visually consistent.
func launcherResultScrollView(props launcherResultScrollProps) woxwidget.Widget {
	if props.ContentHeight <= props.Height || props.Height <= 0 {
		return buildLauncherResultScrollView(woxwidget.StateContext{}, props, nil)
	}
	return woxwidget.Stateful{
		Key: "launcher-result-scroll", Type: (*launcherResultScrollState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &launcherResultScrollState{} },
	}
}

func (s *launcherResultScrollState) InitState(_ woxwidget.StateContext, _ any) {}

// DidUpdateWidget reveals the scrollbar when keyboard navigation moves the controlled offset.
func (s *launcherResultScrollState) DidUpdateWidget(context woxwidget.StateContext, oldWidget, newWidget any) {
	oldProps := oldWidget.(launcherResultScrollProps)
	newProps := newWidget.(launcherResultScrollProps)
	if newProps.Offset != oldProps.Offset && newProps.Height == oldProps.Height && newProps.ContentHeight == oldProps.ContentHeight {
		s.show(context)
	}
}

// Build expires inactivity and composes the scroll surface from local interaction state.
func (s *launcherResultScrollState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	if s.visible && !s.hovered && !s.dragging && !s.hideAt.IsZero() && !time.Now().Before(s.hideAt) {
		s.visible = false
		s.hideAt = time.Time{}
		s.hideTimer = nil
	}
	return buildLauncherResultScrollView(context, widget.(launcherResultScrollProps), s)
}

// Dispose cancels the pending inactivity frame when the result surface leaves the tree.
func (s *launcherResultScrollState) Dispose() {
	if s.hideTimer != nil {
		s.hideTimer.Stop()
		s.hideTimer = nil
	}
}

func (s *launcherResultScrollState) show(context woxwidget.StateContext) {
	s.visible = true
	s.scheduleHide(context)
	context.Invalidate()
}

// scheduleHide restarts the inactivity deadline unless hover is holding the scrollbar open.
func (s *launcherResultScrollState) scheduleHide(context woxwidget.StateContext) {
	if s.hideTimer != nil {
		s.hideTimer.Stop()
		s.hideTimer = nil
	}
	s.hideAt = time.Time{}
	if s.hovered || s.dragging {
		return
	}
	s.hideAt = time.Now().Add(2 * time.Second)
	s.hideTimer = time.AfterFunc(2*time.Second, context.Invalidate)
}

// setHovered pauses or resumes the local inactivity deadline.
func (s *launcherResultScrollState) setHovered(context woxwidget.StateContext, hovered bool) {
	if s.hovered == hovered {
		return
	}
	s.hovered = hovered
	s.scheduleHide(context)
	context.Invalidate()
}

// buildLauncherResultScrollView composes the controlled viewport around optional retained scrollbar state.
func buildLauncherResultScrollView(context woxwidget.StateContext, props launcherResultScrollProps, state *launcherResultScrollState) woxwidget.Widget {
	children := []woxwidget.StackChild{{Child: woxwidget.ScrollView{
		Width: props.Width, Height: props.Height, ContentHeight: props.ContentHeight, Offset: props.Offset, Child: props.Content,
	}}}
	if props.ContentHeight > props.Height && props.Height > 0 {
		thumbHeight := max(float32(24), props.Height*props.Height/props.ContentHeight)
		thumbTop := (props.Height - thumbHeight) * props.Offset / (props.ContentHeight - props.Height)
		thumbColor := props.ThumbColor
		thumbColor.A = min(150, thumbColor.A)
		visible := state != nil && state.visible
		targetOpacity := float32(0)
		if visible {
			targetOpacity = 1
		}
		targetWidth := float32(3)
		if state != nil && (state.hovered || state.dragging) {
			targetWidth = 7
		}
		var thumb woxwidget.Widget = woxwidget.AnimatedFloat{Key: "result-scrollbar-opacity", Target: targetOpacity, Duration: 200 * time.Millisecond, Builder: func(opacity float32) woxwidget.Widget {
			return woxwidget.AnimatedFloat{Key: "result-scrollbar-width", Target: targetWidth, Duration: 120 * time.Millisecond, Builder: func(width float32) woxwidget.Widget {
				color := thumbColor
				color.A = uint8(float32(color.A)*opacity + 0.5)
				return woxwidget.Align{Width: 12, Height: thumbHeight, Horizontal: 1, Child: woxwidget.Container{Width: width, Height: thumbHeight, Radius: width / 2, Color: color}}
			}}
		}}
		if visible {
			thumb = woxwidget.Gesture{ID: "result-scrollbar", OnHover: func(hovered bool) { state.setHovered(context, hovered) }, OnPanStart: func(point woxui.Point) {
				state.dragY = thumbTop + point.Y
				state.dragging = true
				state.setHovered(context, true)
			}, OnPanUpdate: func(point woxui.Point) {
				pointerY := thumbTop + point.Y
				delta := (pointerY - state.dragY) * props.ContentHeight / props.Height
				state.dragY = pointerY
				if resultScrollOffset(props, delta) != props.Offset {
					state.show(context)
					if props.OnScroll != nil {
						props.OnScroll(delta)
					}
				}
			}, OnPanEnd: func() {
				state.dragging = false
				state.scheduleHide(context)
				context.Invalidate()
			}, Child: thumb}
		}
		children = append(children, woxwidget.StackChild{
			Left: max(float32(0), props.Width-14), Top: thumbTop, Child: thumb,
		})
	}
	return woxwidget.Gesture{ID: "result-scroll", OnScroll: func(delta woxui.Point) {
		scrollDelta := -delta.Y
		if resultScrollOffset(props, scrollDelta) != props.Offset {
			if state != nil {
				state.show(context)
			}
			if props.OnScroll != nil {
				props.OnScroll(scrollDelta)
			}
		}
	}, Child: woxwidget.Stack{Width: props.Width, Height: props.Height, Children: children}}
}

// resultScrollOffset clamps a requested movement to the controlled viewport geometry.
func resultScrollOffset(props launcherResultScrollProps, delta float32) float32 {
	return min(max(float32(0), props.Offset+delta), max(float32(0), props.ContentHeight-props.Height))
}
