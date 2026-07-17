package view

import (
	"fmt"

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
	Items             []LauncherResultItem
	OnScroll          func(float32)
}

// LauncherEmptyResultsView builds the launcher content shown before results arrive.
func LauncherEmptyResultsView(width, height float32, pending bool, message string, color woxui.Color) woxwidget.Widget {
	if pending {
		return woxwidget.Container{Width: width, Height: height}
	}
	return woxwidget.Container{
		Width: width, Height: height, Padding: woxwidget.Insets{Left: 28, Top: 18},
		Child: woxwidget.Text{Value: message, Style: woxui.TextStyle{Size: 14}, Color: color},
	}
}

// LauncherSplitContentView places the result list beside a prepared preview.
func LauncherSplitContentView(results, preview woxwidget.Widget) woxwidget.Widget {
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{results, preview}}
}

// LauncherResultsView builds the virtualized result list.
func LauncherResultsView(props LauncherResultsProps) woxwidget.Widget {
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
				Width: rowWidth, Height: props.RowHeight, Padding: woxwidget.Insets{Left: 8, Top: 18},
				Child: woxwidget.Text{Value: item.Title, Style: woxui.TextStyle{Size: 15}, Color: subtitle},
			})
			continue
		}
		var icon woxwidget.Widget = woxwidget.Painter{Width: 28, Height: 28}
		if item.Icon != nil {
			icon = woxwidget.Image{Source: item.Icon, Width: 28, Height: 28}
		}
		var tail woxwidget.Widget
		if len(item.Tails) > 0 {
			tail = launcherResultTails(item.Tails, item.TailWidth, item.TailHeight, tailColor, item.Selected)
		}
		labelWidth := max(float32(50), innerRowWidth-28-20-item.TailWidth)
		labelChildren := []woxwidget.Widget{woxwidget.Text{Value: item.Title, Style: woxui.TextStyle{Size: 15}, Color: title}}
		labelTop := float32(7)
		labelGap := float32(0)
		if item.Subtitle != "" {
			labelChildren = append(labelChildren, woxwidget.Text{Value: item.Subtitle, Style: woxui.TextStyle{Size: 12}, Color: subtitle})
			labelGap = 2
		} else {
			labelTop = max(float32(0), (50-item.TitleHeight)/2)
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
				Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
					woxwidget.Container{Width: 28, Height: 50, Padding: woxwidget.Insets{Top: 11}, Child: icon},
					woxwidget.Container{Width: labelWidth, Height: 50, Padding: woxwidget.Insets{Top: labelTop}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: labelGap, Children: labelChildren}},
					woxwidget.Container{Width: item.TailWidth, Height: 50, Padding: woxwidget.Insets{Top: max(float32(0), (50-item.TailHeight)/2)}, Child: tail},
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
	return woxwidget.Semantics{
		Key: "launcher-results-key", AutomationID: "launcher.results", Role: woxui.AccessibilityRoleList, Label: "Search results",
		Child: launcherResultScrollView(launcherResultScrollProps{
			Content: content, Width: props.Width, Height: props.Height, ContentHeight: props.ContentHeight, Offset: props.Offset,
			ThumbColor: props.Theme.ResultSubtitle, OnScroll: props.OnScroll,
		}),
	}
}

// launcherResultTails restores Flutter's text-tag and image-tail presentation.
func launcherResultTails(tails []LauncherResultTail, width, height float32, foreground woxui.Color, selected bool) woxwidget.Widget {
	const itemLeftPadding = float32(10)
	children := make([]woxwidget.Widget, 0, len(tails))
	for _, item := range tails {
		var content woxwidget.Widget
		if item.Image != nil {
			content = woxwidget.Image{Source: item.Image, Width: item.Width, Height: item.Height}
		} else {
			textColor, background, border := launcherResultTextTailStyle(item.TextCategory, foreground, selected)
			textWidth := max(float32(0), item.Width-16)
			textHeight := max(float32(0), item.Height-6)
			content = woxwidget.Container{
				Width: item.Width, Height: item.Height, Radius: item.Height / 2, Color: background, BorderColor: border, BorderWidth: 1,
				Padding: woxwidget.Insets{Left: 8, Top: 3, Right: 8, Bottom: 3}, Child: woxwidget.TextBlock{
					Value: item.Text, Width: textWidth, Height: textHeight, MaxLines: 1, LineHeight: textHeight, Style: woxui.TextStyle{Size: 11}, Color: textColor,
				},
			}
		}
		children = append(children, woxwidget.Container{
			Width: itemLeftPadding + item.Width, Height: height,
			Padding: woxwidget.Insets{Left: itemLeftPadding, Top: max(float32(0), (height-item.Height)/2)}, Child: content,
		})
	}
	return woxwidget.Clip{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: children}}
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

// launcherResultScrollView keeps list and grid scrolling visually consistent.
func launcherResultScrollView(props launcherResultScrollProps) woxwidget.Widget {
	children := []woxwidget.StackChild{{Child: woxwidget.ScrollView{
		Width: props.Width, Height: props.Height, ContentHeight: props.ContentHeight, Offset: props.Offset, Child: props.Content,
	}}}
	if props.ContentHeight > props.Height && props.Height > 0 {
		thumbHeight := max(float32(24), props.Height*props.Height/props.ContentHeight)
		thumbTop := (props.Height - thumbHeight) * props.Offset / (props.ContentHeight - props.Height)
		thumbColor := props.ThumbColor
		thumbColor.A = min(150, thumbColor.A)
		children = append(children, woxwidget.StackChild{
			Left: max(float32(0), props.Width-5), Top: thumbTop,
			Child: woxwidget.Container{Width: 3, Height: thumbHeight, Radius: 2, Color: thumbColor},
		})
	}
	return woxwidget.Gesture{ID: "result-scroll", OnScroll: func(delta woxui.Point) {
		if props.OnScroll != nil {
			props.OnScroll(-delta.Y)
		}
	}, Child: woxwidget.Stack{Width: props.Width, Height: props.Height, Children: children}}
}
