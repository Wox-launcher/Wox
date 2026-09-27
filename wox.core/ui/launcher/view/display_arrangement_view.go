package view

import (
	"fmt"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	utilwindow "wox/util/window"
)

// DisplayArrangementTile is one monitor in the shared screen map.
type DisplayArrangementTile struct {
	Index     int
	Selected  bool
	IsPrimary bool
	Bounds    utilwindow.WindowRect
	WorkArea  utilwindow.WindowRect
	Slots     []WindowGroupSlotProps
}

// DisplayArrangementProps draws connected monitors in their real relative positions.
type DisplayArrangementProps struct {
	Width        float32
	Height       float32
	Loading      bool
	Error        string
	RetryLabel   string
	EmptyLabel   string
	PrimaryLabel string
	TileIDPrefix string
	Tiles        []DisplayArrangementTile
	Theme        woxcomponent.ControlTheme
	OnSelect     func(int)
	OnRetry      func()
	// SlotTile draws a layout slot inside a monitor. Nil leaves the monitor empty.
	SlotTile func(DisplayArrangementTile, WindowGroupSlotProps, float32, float32) woxwidget.Widget
}

// DisplayArrangement builds the screen map used by window layouts and the display picker.
func DisplayArrangement(props DisplayArrangementProps) woxwidget.Widget {
	if props.Loading {
		return windowGroupMessageBox(props.Width, props.Height, props.Theme, "…")
	}
	if props.Error != "" {
		retry := woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: props.tileIDPrefix() + "-retry", Label: props.RetryLabel, Variant: woxcomponent.ButtonSecondary, OnTap: props.OnRetry, Theme: props.Theme,
		})
		return woxwidget.Container{
			Width: props.Width, Height: props.Height, Radius: 6, BorderColor: windowGroupFadeColor(props.Theme.TextSecondary, 0.35), BorderWidth: 1,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				woxwidget.Text{Value: props.Error, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.Error},
				retry,
			}},
		}
	}
	if len(props.Tiles) == 0 {
		return windowGroupMessageBox(props.Width, props.Height, props.Theme, props.EmptyLabel)
	}
	minX, minY, maxX, maxY := displayArrangementBounds(props.Tiles)
	desktopWidth := max(float32(1), maxX-minX)
	desktopHeight := max(float32(1), maxY-minY)
	const padding = float32(24)
	scale := min((props.Width-padding*2)/desktopWidth, (props.Height-padding*2)/desktopHeight)
	contentWidth := desktopWidth * scale
	contentHeight := desktopHeight * scale
	offsetX := (props.Width - contentWidth) / 2
	offsetY := (props.Height - contentHeight) / 2
	children := make([]woxwidget.StackChild, 0, len(props.Tiles))
	for _, tile := range props.Tiles {
		rect := displayArrangementRect(tile)
		left := offsetX + (rect.X-minX)*scale
		top := offsetY + (rect.Y-minY)*scale
		tileWidth := max(float32(90), rect.Width*scale)
		tileHeight := max(float32(58), rect.Height*scale)
		children = append(children, woxwidget.StackChild{
			Left: left, Top: top, Child: displayArrangementTile(props, tile, tileWidth, tileHeight),
		})
	}
	return woxwidget.Container{
		Width: props.Width, Height: props.Height, Radius: 6, Color: windowGroupFadeColor(props.Theme.TextSecondary, 0.06),
		BorderColor: windowGroupFadeColor(props.Theme.TextSecondary, 0.35), BorderWidth: 1,
		Child: woxwidget.Stack{Width: props.Width, Height: props.Height, Children: children},
	}
}

func (props DisplayArrangementProps) tileIDPrefix() string {
	if props.TileIDPrefix != "" {
		return props.TileIDPrefix
	}
	return "display"
}

func displayArrangementTile(props DisplayArrangementProps, tile DisplayArrangementTile, width, height float32) woxwidget.Widget {
	selectedColor := windowGroupSelectionColor()
	border := windowGroupFadeColor(props.Theme.TextSecondary, 0.4)
	borderWidth := float32(1)
	background := props.Theme.InputBackground
	if tile.Selected {
		border = windowGroupFadeColor(selectedColor, 0.9)
		borderWidth = 2
		background = windowGroupBlendColor(windowGroupFadeColor(selectedColor, 0.08), props.Theme.InputBackground)
	}
	slotChildren := make([]woxwidget.StackChild, 0, len(tile.Slots))
	for _, slot := range tile.Slots {
		if props.SlotTile == nil {
			break
		}
		x, y, w, h := slotFractionRect(slot, width, height)
		slotChildren = append(slotChildren, woxwidget.StackChild{
			Left: x + 3, Top: y + 3, Child: props.SlotTile(tile, slot, w-6, h-6),
		})
	}
	content := woxwidget.Stack{Width: width, Height: height, Children: slotChildren}
	if tile.IsPrimary && props.PrimaryLabel != "" {
		content.Children = append(content.Children, woxwidget.StackChild{
			Top: 6, Right: 6, AnchorRight: true, Child: woxwidget.Text{Value: props.PrimaryLabel, Style: woxui.TextStyle{Size: 10}, Color: props.Theme.TextSecondary},
		})
	}
	tileSurface := woxwidget.Widget(woxwidget.Container{
		Width: width, Height: height, Radius: 6, Color: background, BorderColor: border, BorderWidth: borderWidth, Child: content,
	})
	if tile.Selected {
		tileSurface = woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
			{Left: -3, Top: -3, Child: woxwidget.Container{
				Width: width + 6, Height: height + 6, Radius: 9,
				BorderColor: windowGroupFadeColor(selectedColor, 0.22), BorderWidth: 3,
			}},
			{Child: tileSurface},
		}}
	}
	index := tile.Index
	return woxwidget.Gesture{
		ID: fmt.Sprintf("%s-%d", props.tileIDPrefix(), tile.Index),
		OnTap: func() {
			if props.OnSelect != nil {
				props.OnSelect(index)
			}
		},
		Child: tileSurface,
	}
}

func displayArrangementBounds(tiles []DisplayArrangementTile) (minX, minY, maxX, maxY float32) {
	if len(tiles) == 0 {
		return 0, 0, 1, 1
	}
	first := displayArrangementRect(tiles[0])
	minX, minY, maxX, maxY = first.X, first.Y, first.X+first.Width, first.Y+first.Height
	for _, tile := range tiles[1:] {
		rect := displayArrangementRect(tile)
		minX = min(minX, rect.X)
		minY = min(minY, rect.Y)
		maxX = max(maxX, rect.X+rect.Width)
		maxY = max(maxY, rect.Y+rect.Height)
	}
	return minX, minY, maxX, maxY
}

func displayArrangementRect(tile DisplayArrangementTile) displayRect {
	rect := tile.Bounds
	if rect.Width <= 0 || rect.Height <= 0 {
		rect = tile.WorkArea
	}
	return displayRect{X: float32(rect.X), Y: float32(rect.Y), Width: max(float32(1), float32(rect.Width)), Height: max(float32(1), float32(rect.Height))}
}

// windowGroupDisplayArrangement keeps the workspace editor on the shared screen map.
func windowGroupDisplayArrangement(props WindowGroupEditorProps, width, height float32) woxwidget.Widget {
	tiles := make([]DisplayArrangementTile, 0, len(props.DisplayTiles))
	for _, tile := range props.DisplayTiles {
		tiles = append(tiles, DisplayArrangementTile{
			Index: tile.Index, Selected: tile.Selected, IsPrimary: tile.IsPrimary,
			Bounds: tile.Bounds, WorkArea: tile.WorkArea, Slots: tile.Slots,
		})
	}
	return DisplayArrangement(DisplayArrangementProps{
		Width: width, Height: height, Loading: props.LoadingDisplays, Error: props.DisplaysError,
		RetryLabel: props.RetryLabel, EmptyLabel: props.NoDisplaysLabel, PrimaryLabel: props.PrimaryDisplayLabel,
		TileIDPrefix: "window-group-display", Tiles: tiles, Theme: props.Theme,
		OnSelect: props.OnSelectDisplay, OnRetry: props.OnRetryDisplays,
		SlotTile: func(tile DisplayArrangementTile, slot WindowGroupSlotProps, slotWidth, slotHeight float32) woxwidget.Widget {
			return windowGroupSlotTile(props, slot, tile.Selected, tile.Index, slotWidth, slotHeight)
		},
	})
}
