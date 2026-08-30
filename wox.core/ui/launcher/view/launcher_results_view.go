package view

import (
	"fmt"
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const launcherResultBoundaryKeyPrefix = "launcher-result-boundary-"

// LauncherResultBackgroundBoundaryKey returns the retained background Boundary key for one result.
func LauncherResultBackgroundBoundaryKey(id string) woxwidget.Key {
	return woxwidget.Key(launcherResultBoundaryKeyPrefix + id + "-background")
}

// LauncherResultIconBoundaryKey returns the retained icon Boundary key for one result.
func LauncherResultIconBoundaryKey(id string) woxwidget.Key {
	return woxwidget.Key(launcherResultBoundaryKeyPrefix + id + "-icon")
}

// LauncherResultTitleBoundaryKey returns the retained title Boundary key for one result.
func LauncherResultTitleBoundaryKey(id string) woxwidget.Key {
	return woxwidget.Key(launcherResultBoundaryKeyPrefix + id + "-title")
}

// LauncherResultSubtitleBoundaryKey returns the retained subtitle Boundary key for one result.
func LauncherResultSubtitleBoundaryKey(id string) woxwidget.Key {
	return woxwidget.Key(launcherResultBoundaryKeyPrefix + id + "-subtitle")
}

// LauncherResultTailsBoundaryKey returns the retained tails Boundary key for one result.
func LauncherResultTailsBoundaryKey(id string) woxwidget.Key {
	return woxwidget.Key(launcherResultBoundaryKeyPrefix + id + "-tails")
}

// LauncherResultTail contains one resolved result-tail visual and its measured width.
type LauncherResultTail struct {
	Text           string
	TextCategory   string
	Tooltip        string
	Image          *woxui.Image
	ImageText      string
	ImageTextColor woxui.Color
	ImageTextSize  float32
	Width          float32
	Height         float32
}

// LauncherResultItem contains one visible list result and its controller callbacks.
type LauncherResultItem struct {
	ID                 string
	Title              string
	Subtitle           string
	Group              bool
	Selected           bool
	Hovered            bool
	Icon               *woxui.Image
	Tails              []LauncherResultTail
	TailWidth          float32
	TailHeight         float32
	QuickSelectNumber  string
	OnHover            func(bool)                     `boundary:"stable"`
	OnSelect           func()                         `boundary:"stable"`
	OnSecondaryTapDown func()                         `boundary:"stable"`
	OnActivate         func()                         `boundary:"stable"`
	OnDragStart        func()                         `boundary:"stable"`
	OnTooltip          func(bool, string, woxui.Rect) `boundary:"stable"`
}

// LauncherResultsProps contains the prepared viewport slice and result-list geometry.
type LauncherResultsProps struct {
	Width             float32
	Height            float32
	ContentHeight     float32
	Offset            float32
	StartIndex        int
	StartOffset       float32
	RowHeight         float32
	GroupRowHeight    float32
	RowGap            float32
	ContainerPadding  woxwidget.Insets
	ItemPadding       woxwidget.Insets
	ItemRadius        float32
	TailColor         woxui.Color
	SelectedTailColor woxui.Color
	Theme             woxcomponent.Theme
	DensityScale      float32
	Complete          bool
	ScrollDetached    bool
	Items             []LauncherResultItem
	OnScroll          func(float32) `boundary:"stable"`
}

type launcherResultRowProps struct {
	Item              LauncherResultItem
	RowWidth          float32
	RowHeight         float32
	GroupRowHeight    float32
	InnerRowWidth     float32
	BaseHeight        float32
	IconSize          float32
	IconGap           float32
	ItemPadding       woxwidget.Insets
	ItemRadius        float32
	TailColor         woxui.Color
	SelectedTailColor woxui.Color
	Theme             woxcomponent.Theme
	DensityScale      float32
	TitleStyle        woxui.TextStyle
	SubtitleStyle     woxui.TextStyle
}

type launcherResultBackgroundProps struct {
	Width  float32
	Height float32
	Radius float32
	Color  woxui.Color
}

func (p launcherResultBackgroundProps) Equal(other launcherResultBackgroundProps) bool {
	return p == other
}

type launcherResultIconProps struct {
	Image *woxui.Image
	Size  float32
}

func (p launcherResultIconProps) Equal(other launcherResultIconProps) bool {
	return p == other
}

type launcherResultTextProps struct {
	Value string
	Style woxui.TextStyle
	Color woxui.Color
}

func (p launcherResultTextProps) Equal(other launcherResultTextProps) bool {
	return p == other
}

// launcherResultSingleLineText keeps result titles and subtitles on one line.
// List rows only have a single-line slot, so hard breaks would wrap inside the
// row and look like a broken title instead of a compact label.
func launcherResultSingleLineText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.Join(strings.Fields(value), " ")
}

type launcherResultTailsProps struct {
	ID                 string
	Items              []LauncherResultTail
	Width              float32
	Height             float32
	Foreground         woxui.Color
	Selected           bool
	DensityScale       float32
	OnHover            func(bool)                     `boundary:"stable"`
	OnSelect           func()                         `boundary:"stable"`
	OnSecondaryTapDown func()                         `boundary:"stable"`
	OnActivate         func()                         `boundary:"stable"`
	OnDragStart        func()                         `boundary:"stable"`
	OnTooltip          func(bool, string, woxui.Rect) `boundary:"stable"`
}

func (p launcherResultTailsProps) Equal(other launcherResultTailsProps) bool {
	if p.ID != other.ID || p.Width != other.Width || p.Height != other.Height || p.Foreground != other.Foreground || p.Selected != other.Selected || p.DensityScale != other.DensityScale || len(p.Items) != len(other.Items) {
		return false
	}
	for index := range p.Items {
		if p.Items[index] != other.Items[index] {
			return false
		}
	}
	return true
}

// launcherResultTextBoundary retains one independently updatable result label.
func launcherResultTextBoundary(key woxwidget.Key, label string, props launcherResultTextProps) woxwidget.Widget {
	return woxwidget.Boundary[launcherResultTextProps]{
		Key: key, Label: label, Props: props,
		Build: func(props launcherResultTextProps) woxwidget.Widget {
			return woxwidget.Text{Value: props.Value, Style: props.Style, Color: props.Color}
		},
	}
}

// LauncherSplitContentView places the result list beside a prepared preview.
// Each pane is capped to its allocated width so a loose horizontal flex cannot
// measure both children at the full content width and report a 1:1 overflow.
// Height is not filled: the launcher column is a sequential vertical flex, so
// available height is the window, not the remaining content slot.
func LauncherSplitContentView(resultsWidth float32, results woxwidget.Widget, previewWidth float32, preview woxwidget.Widget) woxwidget.Widget {
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Constrained{MaxWidth: resultsWidth, FillWidth: true, Child: results},
		woxwidget.Constrained{MaxWidth: previewWidth, FillWidth: true, Child: preview},
	}}
}

// LauncherResultsView builds the virtualized result list.
func LauncherResultsView(props LauncherResultsProps) woxwidget.Widget {
	baseHeight := scaledLauncherSize(50, props.DensityScale)
	iconSize := scaledLauncherSize(28, props.DensityScale)
	iconGap := scaledLauncherSize(10, props.DensityScale)
	titleStyle := woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.ResultTitleFontSize, props.DensityScale)}
	subtitleStyle := woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.ResultSubtitleFontSize, props.DensityScale)}
	rowWidth := max(float32(0), props.Width-props.ContainerPadding.Left-props.ContainerPadding.Right)
	innerRowWidth := max(float32(0), rowWidth-props.ItemPadding.Left-props.ItemPadding.Right)
	rows := make([]woxwidget.Widget, 0, len(props.Items))
	for itemOffset, item := range props.Items {
		key := item.ID
		if key == "" {
			key = fmt.Sprintf("index-%d", props.StartIndex+itemOffset)
		}
		item.ID = key
		rowProps := launcherResultRowProps{
			Item: item, RowWidth: rowWidth, RowHeight: props.RowHeight, GroupRowHeight: props.GroupRowHeight, InnerRowWidth: innerRowWidth,
			BaseHeight: baseHeight, IconSize: iconSize, IconGap: iconGap, ItemPadding: props.ItemPadding, ItemRadius: props.ItemRadius,
			TailColor: props.TailColor, SelectedTailColor: props.SelectedTailColor, Theme: props.Theme, DensityScale: props.DensityScale,
			TitleStyle: titleStyle, SubtitleStyle: subtitleStyle,
		}
		rows = append(rows, launcherResultRow(rowProps))
	}
	visiblePadding := props.ContainerPadding
	if props.StartOffset > 0 {
		visiblePadding.Top += props.StartOffset
	} else {
		visiblePadding.Top += float32(props.StartIndex) * (props.RowHeight + props.RowGap)
	}
	content := woxwidget.Container{
		Width: props.Width, Height: props.ContentHeight, Padding: visiblePadding,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: props.RowGap, Children: rows},
	}
	return WrapLauncherResultsStatus(props.Complete, woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: "launcher-result-scroll", Content: content, Width: props.Width, Height: props.Height, ContentHeight: props.ContentHeight, Offset: props.Offset,
		ThumbColor: props.Theme.ResultSubtitle, OnScroll: props.OnScroll,
	}))
}

// WrapLauncherResultsStatus exposes query completion on every result surface, including grid and preview-only chat.
func WrapLauncherResultsStatus(complete bool, child woxwidget.Widget) woxwidget.Widget {
	state := "loading"
	if complete {
		state = "complete"
	}
	return woxwidget.Semantics{
		Key: "launcher-results-key", AutomationID: "launcher.results", Role: woxui.AccessibilityRoleList, Label: "Search results",
		Value: state, ReadOnly: true, Child: child,
	}
}

// launcherResultRow builds one pure row subtree from fully prepared props.
func launcherResultRow(props launcherResultRowProps) woxwidget.Widget {
	item := props.Item
	// UpdateResult changes row visuals independently; keep their Boundaries outside any enclosing row Boundary.
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
	titleValue := launcherResultSingleLineText(item.Title)
	if item.Group {
		titleProps := launcherResultTextProps{Value: titleValue, Style: props.TitleStyle, Color: title}
		groupHeight := props.GroupRowHeight
		if groupHeight <= 0 {
			groupHeight = props.RowHeight
		}
		return woxwidget.Container{
			Width: props.RowWidth, Height: groupHeight, Padding: woxwidget.Insets{Left: scaledLauncherSize(8, props.DensityScale)},
			Child: woxwidget.Align{Height: groupHeight, Vertical: 0.5, Child: launcherResultTextBoundary(LauncherResultTitleBoundaryKey(item.ID), "result-title:"+item.ID, titleProps)},
		}
	}
	iconProps := launcherResultIconProps{Image: item.Icon, Size: props.IconSize}
	icon := woxwidget.Boundary[launcherResultIconProps]{
		Key: LauncherResultIconBoundaryKey(item.ID), Label: "result-icon:" + item.ID, Props: iconProps,
		Build: func(props launcherResultIconProps) woxwidget.Widget {
			if props.Image == nil {
				return woxwidget.Painter{Width: props.Size, Height: props.Size}
			}
			return woxwidget.Image{Source: props.Image, Width: props.Size, Height: props.Size}
		},
	}
	var tail woxwidget.Widget
	if len(item.Tails) > 0 {
		tailProps := launcherResultTailsProps{
			ID: item.ID, Items: item.Tails, Width: item.TailWidth, Height: item.TailHeight, Foreground: tailColor, Selected: item.Selected, DensityScale: props.DensityScale,
			OnHover: item.OnHover, OnSelect: item.OnSelect, OnSecondaryTapDown: item.OnSecondaryTapDown, OnActivate: item.OnActivate, OnDragStart: item.OnDragStart, OnTooltip: item.OnTooltip,
		}
		tail = woxwidget.Boundary[launcherResultTailsProps]{
			Key: LauncherResultTailsBoundaryKey(item.ID), Label: "result-tails:" + item.ID, Props: tailProps,
			Build: func(props launcherResultTailsProps) woxwidget.Widget {
				return launcherResultTailsWithDensity(props, woxwidget.Key("launcher-result-tails-"+props.ID))
			},
		}
	}
	quickSelectWidth := launcherQuickSelectSlotWidth(item.QuickSelectNumber, props.DensityScale)
	trailingWidth := item.TailWidth + quickSelectWidth
	gapCount := 1
	if trailingWidth > 0 {
		gapCount++
	}
	labelContentWidth := max(props.BaseHeight, props.InnerRowWidth-props.IconSize-scaledLauncherSize(20, props.DensityScale))
	labelWidth := max(props.BaseHeight, props.InnerRowWidth-props.IconSize-trailingWidth-props.IconGap*float32(gapCount))
	titleProps := launcherResultTextProps{Value: titleValue, Style: props.TitleStyle, Color: title}
	labelChildren := []woxwidget.Widget{launcherResultTextBoundary(LauncherResultTitleBoundaryKey(item.ID), "result-title:"+item.ID, titleProps)}
	subtitleValue := launcherResultSingleLineText(item.Subtitle)
	labelGap := float32(0)
	if subtitleValue != "" {
		subtitleProps := launcherResultTextProps{Value: subtitleValue, Style: props.SubtitleStyle, Color: subtitle}
		labelChildren = append(labelChildren, launcherResultTextBoundary(LauncherResultSubtitleBoundaryKey(item.ID), "result-subtitle:"+item.ID, subtitleProps))
		labelGap = scaledLauncherSize(2, props.DensityScale)
	}
	labelContent := woxwidget.Container{Width: labelContentWidth, Height: props.BaseHeight, Child: woxwidget.Align{Width: labelContentWidth, Height: props.BaseHeight, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: labelGap, Children: labelChildren}}}
	backgroundProps := launcherResultBackgroundProps{Width: props.RowWidth, Height: props.RowHeight, Radius: props.ItemRadius, Color: background}
	backgroundLayer := woxwidget.Boundary[launcherResultBackgroundProps]{
		Key: LauncherResultBackgroundBoundaryKey(item.ID), Label: "result-background:" + item.ID, Props: backgroundProps,
		Build: func(props launcherResultBackgroundProps) woxwidget.Widget {
			return woxwidget.Container{Width: props.Width, Height: props.Height, Radius: props.Radius, Color: props.Color}
		},
	}
	rowChildren := []woxwidget.Widget{
		woxwidget.Align{Width: props.IconSize, Height: props.BaseHeight, Vertical: 0.5, Child: icon},
		woxwidget.Clip{Width: labelWidth, Height: props.BaseHeight, Child: labelContent},
	}
	// Keep tails and the hold-to-number chip in one trailing cluster so every
	// row shares the same right edge. An extra flex gap here is what pushed
	// numbered rows with tags past the result padding.
	if trailing := launcherResultTrailing(tail, item.TailWidth, item.QuickSelectNumber, props.BaseHeight, quickSelectWidth, props.DensityScale, tailColor, props.Theme.Background); trailing != nil {
		rowChildren = append(rowChildren, trailing)
	}
	contentLayer := woxwidget.Container{
		Width: props.RowWidth, Height: props.RowHeight, Padding: props.ItemPadding,
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: props.IconGap, Children: rowChildren},
	}
	resultControl := woxwidget.Gesture{
		ID: fmt.Sprintf("result-gesture-%s", item.ID),
		OnHover: func(inside bool) {
			if item.OnHover != nil {
				item.OnHover(inside)
			}
		},
		OnTap: item.OnSelect,
		OnSecondaryTapDown: func(woxui.Point) {
			if item.OnSecondaryTapDown != nil {
				item.OnSecondaryTapDown()
			}
		},
		OnDragStart: item.OnDragStart,
		OnDoubleTap: func() {
			if item.OnSelect != nil {
				item.OnSelect()
			}
			if item.OnActivate != nil {
				item.OnActivate()
			}
		},
		Child: woxwidget.Stack{Width: props.RowWidth, Height: props.RowHeight, Children: []woxwidget.StackChild{{Child: backgroundLayer}, {Child: contentLayer}}},
	}
	return woxwidget.Semantics{
		Key: woxwidget.Key(fmt.Sprintf("launcher-result-key-%s", item.ID)), AutomationID: "launcher.result." + item.ID, Role: woxui.AccessibilityRoleListItem,
		Label: titleValue, Description: subtitleValue, Value: item.QuickSelectNumber, Selected: item.Selected,
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
	}
}

// launcherResultTails restores Flutter's text-tag and image-tail presentation.
func launcherResultTails(tails []LauncherResultTail, width, height float32, foreground woxui.Color, selected bool) woxwidget.Widget {
	return launcherResultTailsWithDensity(launcherResultTailsProps{
		Items: tails, Width: width, Height: height, Foreground: foreground, Selected: selected, DensityScale: 1,
	}, "")
}

// launcherResultTailsWithDensity scales launcher-only tail geometry without changing fixed theme previews.
func launcherResultTailsWithDensity(props launcherResultTailsProps, scrollKey woxwidget.Key) woxwidget.Widget {
	itemLeftPadding := scaledLauncherSize(10, props.DensityScale)
	children := make([]woxwidget.Widget, 0, len(props.Items))
	contentWidth := float32(0)
	for index, item := range props.Items {
		var content woxwidget.Widget
		if item.Image != nil {
			content = woxwidget.Image{Source: item.Image, Width: item.Width, Height: item.Height}
			if item.ImageText != "" && item.ImageTextSize > 0 {
				content = woxwidget.Stack{Width: item.Width, Height: item.Height, Children: []woxwidget.StackChild{
					{Child: content},
					{Child: woxwidget.Align{Width: item.Width, Height: item.Height, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
						Value: item.ImageText, Style: woxui.TextStyle{Size: item.ImageTextSize}, Color: item.ImageTextColor,
					}}},
				}}
			}
		} else {
			textColor, background, border := launcherResultTextTailStyle(item.TextCategory, props.Foreground, props.Selected)
			horizontalPadding := scaledLauncherSize(8, props.DensityScale)
			textWidth := max(float32(0), item.Width-horizontalPadding*2)
			content = woxwidget.Container{
				Width: item.Width, Height: item.Height, Radius: item.Height / 2, Color: background, BorderColor: border, BorderWidth: 1,
				Padding: woxwidget.Insets{Left: horizontalPadding, Right: horizontalPadding},
				Child: woxwidget.Align{Width: textWidth, Height: item.Height, Vertical: 0.5, Child: woxwidget.Text{
					Value: item.Text, Style: woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.TailFontSize, props.DensityScale)}, Color: textColor,
				}},
			}
		}
		if tooltip := strings.TrimSpace(item.Tooltip); tooltip != "" && props.OnTooltip != nil {
			content = launcherResultTailHover(props, index, tooltip, content)
		}
		itemWidth := itemLeftPadding + item.Width
		children = append(children, woxwidget.Container{
			Width: itemWidth, Height: props.Height,
			Padding: woxwidget.Insets{Left: itemLeftPadding},
			Child:   woxwidget.Align{Width: item.Width, Height: props.Height, Vertical: 0.5, Child: content},
		})
		contentWidth += itemWidth
	}
	content := woxwidget.Container{Width: contentWidth, Height: props.Height, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: children}}
	if contentWidth <= props.Width {
		return woxwidget.Align{Width: props.Width, Height: props.Height, Horizontal: 1, Vertical: 0.5, Child: content}
	}
	if scrollKey == "" {
		return woxwidget.Clip{Width: props.Width, Height: props.Height, Child: woxwidget.Align{Width: props.Width, Height: props.Height, Horizontal: 1, Child: content}}
	}
	return woxwidget.ScrollView{
		Key: scrollKey, Width: props.Width, Height: props.Height, ContentWidth: contentWidth, Horizontal: true,
		InitialOffset: contentWidth - props.Width, KeepVisible: &woxwidget.ScrollRange{Start: contentWidth - props.Width, End: contentWidth},
		Child: content,
	}
}

// launcherResultTailHover keeps row activation on the tail because a nested
// hover target wins hit-testing and would otherwise swallow select, actions, and drag.
func launcherResultTailHover(props launcherResultTailsProps, index int, tooltip string, content woxwidget.Widget) woxwidget.Widget {
	id := fmt.Sprintf("result-tail-%s-%d", props.ID, index)
	label := strings.TrimSpace(props.Items[index].Text)
	if label == "" {
		label = strings.TrimSpace(props.Items[index].ImageText)
	}
	return woxwidget.Semantics{
		Key: woxwidget.Key(id), AutomationID: id, Role: woxui.AccessibilityRoleText, Label: label, Description: tooltip,
		Child: woxwidget.Gesture{
			ID: id, OnHover: props.OnHover, OnHoverAt: func(inside bool, bounds woxui.Rect) {
				props.OnTooltip(inside, tooltip, bounds)
			}, OnTap: props.OnSelect, OnSecondaryTapDown: func(woxui.Point) {
				if props.OnSecondaryTapDown != nil {
					props.OnSecondaryTapDown()
				}
			}, OnDragStart: props.OnDragStart, OnDoubleTap: func() {
				if props.OnSelect != nil {
					props.OnSelect()
				}
				if props.OnActivate != nil {
					props.OnActivate()
				}
			}, Child: content,
		},
	}
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

const (
	launcherQuickSelectSize          = float32(20)
	launcherQuickSelectRadius        = float32(4)
	launcherQuickSelectPaddingLeft   = float32(10)
	launcherQuickSelectPaddingRight  = float32(5)
	launcherQuickSelectBorderOpacity = float32(0.3)
)

func launcherQuickSelectSlotWidth(number string, densityScale float32) float32 {
	if number == "" {
		return 0
	}
	return scaledLauncherSize(launcherQuickSelectPaddingLeft+launcherQuickSelectSize+launcherQuickSelectPaddingRight, densityScale)
}

// launcherResultTrailing pins tags and the hold-to-number chip to the row's right edge.
func launcherResultTrailing(tail woxwidget.Widget, tailWidth float32, number string, height, badgeWidth, densityScale float32, fill, text woxui.Color) woxwidget.Widget {
	var children []woxwidget.Widget
	if tailWidth > 0 && tail != nil {
		children = append(children, woxwidget.Align{Width: tailWidth, Height: height, Vertical: 0.5, Child: tail})
	}
	if badgeWidth > 0 && number != "" {
		children = append(children, woxwidget.Align{
			Width: badgeWidth, Height: height, Vertical: 0.5,
			Child: launcherQuickSelectBadge(number, densityScale, fill, text),
		})
	}
	if len(children) == 0 {
		return nil
	}
	if len(children) == 1 {
		return children[0]
	}
	return woxwidget.Container{
		Width: tailWidth + badgeWidth, Height: height,
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: children},
	}
}

// launcherQuickSelectBadge draws the hold-to-number chip with text that uses the
// window background so a light selected tail color cannot wash out the digit.
func launcherQuickSelectBadge(number string, densityScale float32, fill, text woxui.Color) woxwidget.Widget {
	size := scaledLauncherSize(launcherQuickSelectSize, densityScale)
	radius := scaledLauncherSize(launcherQuickSelectRadius, densityScale)
	left := scaledLauncherSize(launcherQuickSelectPaddingLeft, densityScale)
	right := scaledLauncherSize(launcherQuickSelectPaddingRight, densityScale)
	border := fill
	border.A = uint8(float32(fill.A)*launcherQuickSelectBorderOpacity + 0.5)
	return woxwidget.Container{
		Width: left + size + right, Height: size, Padding: woxwidget.Insets{Left: left, Right: right},
		Child: woxwidget.Container{
			Width: size, Height: size, Radius: radius, Color: fill, BorderColor: border, BorderWidth: 1,
			Child: woxwidget.Align{Width: size, Height: size, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
				Value: number, Style: woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.TailFontSize, densityScale), Weight: woxui.FontWeightSemibold}, Color: text,
			}},
		},
	}
}
