package view

import (
	"fmt"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// LauncherResultTail contains one resolved result-tail visual and its measured width.
type LauncherResultTail struct {
	Text           string
	TextCategory   string
	Image          *woxui.Image
	ImageText      string
	ImageTextColor woxui.Color
	ImageTextSize  float32
	Width          float32
	Height         float32
}

// LauncherResultItem contains one visible list result and its controller callbacks.
type LauncherResultItem struct {
	ID          string
	Revision    uint64
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
	OnHover     func(bool) `boundary:"stable"`
	OnSelect    func()     `boundary:"stable"`
	OnActivate  func()     `boundary:"stable"`
}

// Equal compares the prepared visual and stable controller callbacks for one result row.
func (i LauncherResultItem) Equal(other LauncherResultItem) bool {
	if i.ID != other.ID || i.Revision != other.Revision || i.Title != other.Title || i.Subtitle != other.Subtitle || i.Group != other.Group || i.Selected != other.Selected || i.Hovered != other.Hovered || i.Icon != other.Icon || i.TitleHeight != other.TitleHeight || i.TailWidth != other.TailWidth || i.TailHeight != other.TailHeight || len(i.Tails) != len(other.Tails) {
		return false
	}
	for index := range i.Tails {
		if i.Tails[index] != other.Tails[index] {
			return false
		}
	}
	return true
}

// LauncherResultsProps contains the prepared viewport slice and result-list geometry.
type LauncherResultsProps struct {
	Revision          uint64
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
	ScrollDetached    bool
	Items             []LauncherResultItem
	OnScroll          func(float32) `boundary:"stable"`
}

// Equal compares every render dependency for the virtualized result section.
func (p LauncherResultsProps) Equal(other LauncherResultsProps) bool {
	if p.Revision != other.Revision || p.Width != other.Width || p.Height != other.Height || p.ContentHeight != other.ContentHeight || p.Offset != other.Offset || p.StartIndex != other.StartIndex || p.RowHeight != other.RowHeight || p.RowGap != other.RowGap || p.ContainerPadding != other.ContainerPadding || p.ItemPadding != other.ItemPadding || p.ItemRadius != other.ItemRadius || p.TailColor != other.TailColor || p.SelectedTailColor != other.SelectedTailColor || p.Theme != other.Theme || p.DensityScale != other.DensityScale || p.Complete != other.Complete || p.ScrollDetached != other.ScrollDetached || len(p.Items) != len(other.Items) {
		return false
	}
	for index := range p.Items {
		if !p.Items[index].Equal(other.Items[index]) {
			return false
		}
	}
	return true
}

type launcherResultRowProps struct {
	Item              LauncherResultItem
	RowWidth          float32
	RowHeight         float32
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

func (p launcherResultRowProps) Equal(other launcherResultRowProps) bool {
	return p.Item.Equal(other.Item) && p.RowWidth == other.RowWidth && p.RowHeight == other.RowHeight && p.InnerRowWidth == other.InnerRowWidth && p.BaseHeight == other.BaseHeight && p.IconSize == other.IconSize && p.IconGap == other.IconGap && p.ItemPadding == other.ItemPadding && p.ItemRadius == other.ItemRadius && p.TailColor == other.TailColor && p.SelectedTailColor == other.SelectedTailColor && p.Theme == other.Theme && p.DensityScale == other.DensityScale && p.TitleStyle == other.TitleStyle && p.SubtitleStyle == other.SubtitleStyle
}

// LauncherSplitContentView places the result list beside a prepared preview.
func LauncherSplitContentView(results, preview woxwidget.Widget) woxwidget.Widget {
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{results, preview}}
}

// LauncherResultsBoundary retains the complete result section when its prepared props are unchanged.
func LauncherResultsBoundary(props LauncherResultsProps) woxwidget.Widget {
	return woxwidget.Boundary[LauncherResultsProps]{
		Key: "launcher-results-boundary", Label: "results", Props: props,
		Build: func(props LauncherResultsProps) woxwidget.Widget { return LauncherResultsView(props) },
	}
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
		rowProps := launcherResultRowProps{
			Item: item, RowWidth: rowWidth, RowHeight: props.RowHeight, InnerRowWidth: innerRowWidth,
			BaseHeight: baseHeight, IconSize: iconSize, IconGap: iconGap, ItemPadding: props.ItemPadding, ItemRadius: props.ItemRadius,
			TailColor: props.TailColor, SelectedTailColor: props.SelectedTailColor, Theme: props.Theme, DensityScale: props.DensityScale,
			TitleStyle: titleStyle, SubtitleStyle: subtitleStyle,
		}
		rows = append(rows, woxwidget.Boundary[launcherResultRowProps]{
			Key: woxwidget.Key("launcher-result-boundary-" + key), Label: "result:" + key, Props: rowProps,
			Build: func(props launcherResultRowProps) woxwidget.Widget { return launcherResultRow(props) },
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
		Child: woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
			Key: "launcher-result-scroll", Content: content, Width: props.Width, Height: props.Height, ContentHeight: props.ContentHeight, Offset: props.Offset,
			ThumbColor: props.Theme.ResultSubtitle, OnScroll: props.OnScroll,
		}),
	}
}

// launcherResultRow builds one pure row subtree from fully prepared props.
func launcherResultRow(props launcherResultRowProps) woxwidget.Widget {
	item := props.Item
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
		return woxwidget.Container{
			Width: props.RowWidth, Height: props.RowHeight, Padding: woxwidget.Insets{Left: scaledLauncherSize(8, props.DensityScale), Top: scaledLauncherSize(18, props.DensityScale)},
			Child: woxwidget.Text{Value: item.Title, Style: props.TitleStyle, Color: title},
		}
	}
	var icon woxwidget.Widget = woxwidget.Painter{Width: props.IconSize, Height: props.IconSize}
	if item.Icon != nil {
		icon = woxwidget.Image{Source: item.Icon, Width: props.IconSize, Height: props.IconSize}
	}
	var tail woxwidget.Widget
	if len(item.Tails) > 0 {
		tail = launcherResultTailsWithDensity(item.Tails, item.TailWidth, item.TailHeight, tailColor, item.Selected, props.DensityScale, woxwidget.Key("launcher-result-tails-"+item.ID))
	}
	labelWidth := max(props.BaseHeight, props.InnerRowWidth-props.IconSize-scaledLauncherSize(20, props.DensityScale)-item.TailWidth)
	labelChildren := []woxwidget.Widget{woxwidget.Text{Value: item.Title, Style: props.TitleStyle, Color: title}}
	labelTop := scaledLauncherSize(7, props.DensityScale)
	labelGap := float32(0)
	if item.Subtitle != "" {
		labelChildren = append(labelChildren, woxwidget.Text{Value: item.Subtitle, Style: props.SubtitleStyle, Color: subtitle})
		labelGap = scaledLauncherSize(2, props.DensityScale)
	} else {
		labelTop = max(float32(0), (props.BaseHeight-item.TitleHeight)/2)
	}
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
			Width: props.RowWidth, Height: props.RowHeight, Radius: props.ItemRadius, Color: background, Padding: props.ItemPadding,
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: props.IconGap, Children: []woxwidget.Widget{
				woxwidget.Container{Width: props.IconSize, Height: props.BaseHeight, Padding: woxwidget.Insets{Top: max(float32(0), (props.BaseHeight-props.IconSize)/2)}, Child: icon},
				woxwidget.Container{Width: labelWidth, Height: props.BaseHeight, Padding: woxwidget.Insets{Top: labelTop}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: labelGap, Children: labelChildren}},
				woxwidget.Container{Width: item.TailWidth, Height: props.BaseHeight, Padding: woxwidget.Insets{Top: max(float32(0), (props.BaseHeight-item.TailHeight)/2)}, Child: tail},
			}},
		},
	}
	return woxwidget.Semantics{
		Key: woxwidget.Key(fmt.Sprintf("launcher-result-key-%s", item.ID)), AutomationID: "launcher.result." + item.ID, Role: woxui.AccessibilityRoleListItem,
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
			if item.ImageText != "" && item.ImageTextSize > 0 {
				content = woxwidget.Stack{Width: item.Width, Height: item.Height, Children: []woxwidget.StackChild{
					{Child: content},
					{Child: woxwidget.Align{Width: item.Width, Height: item.Height, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
						Value: item.ImageText, Style: woxui.TextStyle{Size: item.ImageTextSize}, Color: item.ImageTextColor,
					}}},
				}}
			}
		} else {
			textColor, background, border := launcherResultTextTailStyle(item.TextCategory, foreground, selected)
			horizontalPadding := scaledLauncherSize(8, densityScale)
			textWidth := max(float32(0), item.Width-horizontalPadding*2)
			content = woxwidget.Container{
				Width: item.Width, Height: item.Height, Radius: item.Height / 2, Color: background, BorderColor: border, BorderWidth: 1,
				Padding: woxwidget.Insets{Left: horizontalPadding, Right: horizontalPadding},
				Child: woxwidget.Align{Width: textWidth, Height: item.Height, Vertical: 0.5, Child: woxwidget.Text{
					Value: item.Text, Style: woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.TailFontSize, densityScale)}, Color: textColor,
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
