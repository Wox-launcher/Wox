package view

import (
	"fmt"
	"strconv"
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	utilwindow "wox/util/window"
)

// WindowGroupDisplayProps describes one connected display in the editor.
type WindowGroupDisplayProps struct {
	Id        string
	Bounds    utilwindow.WindowRect
	WorkArea  utilwindow.WindowRect
	IsPrimary bool
}

// WindowGroupLayoutOptionProps describes one selectable layout preset.
type WindowGroupLayoutOptionProps struct {
	ID       string
	Label    string
	Selected bool
	Slots    []WindowGroupSlotProps
}

// WindowGroupLayoutGroupProps groups layout presets by slot count.
type WindowGroupLayoutGroupProps struct {
	SlotCountLabel string
	Layouts        []WindowGroupLayoutOptionProps
}

// WindowGroupSlotProps describes one slot tile in the display preview or layout card.
type WindowGroupSlotProps struct {
	ID        string
	Title     string
	Cols      int
	Rows      int
	Col       int
	Row       int
	ColSpan   int
	RowSpan   int
	AppName   string
	AppIcon   *woxui.Image
	UrlCount  int
	IsBrowser bool
}

// WindowGroupDisplayTileProps describes one selectable display preview tile.
type WindowGroupDisplayTileProps struct {
	Index     int
	Selected  bool
	IsPrimary bool
	Bounds    utilwindow.WindowRect
	WorkArea  utilwindow.WindowRect
	LayoutID  string
	Slots     []WindowGroupSlotProps
}

// WindowGroupEditorProps contains the workspace layout editor dialog.
type WindowGroupEditorProps struct {
	Width                float32
	Height               float32
	Title                string
	GroupName            string
	NamePlaceholder      string
	NameError            string
	LoadingDisplays      bool
	DisplaysError        string
	Editing              bool
	SelectedDisplay      int
	SelectedLayoutID     string
	SelectDisplayLabel   string
	NoDisplaysLabel      string
	PrimaryDisplayLabel  string
	LayoutsLabel         string
	LayoutsDescription   string
	ChooseAppLabel       string
	ChangeAppLabel       string
	BrowserUrlsLabel     string
	BrowserUrlEmptyLabel string
	RetryLabel           string
	CancelLabel          string
	SaveLabel            string
	AddSlotIcon          *woxui.Image
	AppsIcon             *woxui.Image
	LinkIcon             *woxui.Image
	Displays             []WindowGroupDisplayProps
	LayoutGroups         []WindowGroupLayoutGroupProps
	DisplayTiles         []WindowGroupDisplayTileProps
	Window               *woxui.Window
	Theme                woxcomponent.Theme
	OnCancel             func()
	OnSave               func()
	OnNameChanged        func(string)
	OnSelectDisplay      func(int)
	OnSelectLayout       func(string)
	OnOpenAppPicker      func(string)
	OnOpenUrlEditor      func(string)
	OnRetryDisplays      func()
}

// WindowGroupUrlEditorProps contains the browser URL list editor.
type WindowGroupUrlEditorProps struct {
	Width                       float32
	Height                      float32
	Title                       string
	Description                 string
	URLs                        []string
	Window                      *woxui.Window
	CancelLabel                 string
	SaveLabel                   string
	AddLabel                    string
	EditLabel                   string
	DeleteLabel                 string
	OperationLabel              string
	EmptyLabel                  string
	DeleteConfirmation          string
	RequiredLabel               string
	ExtensionChecking           bool
	ExtensionConnected          bool
	ExtensionConnectedLabel     string
	ExtensionDisconnectedLabel  string
	ExtensionInstallLabel       string
	AddIcon                     *woxui.Image
	EditIcon                    *woxui.Image
	DeleteIcon                  *woxui.Image
	EmptyIcon                   *woxui.Image
	ExtensionLoadingIcon        *woxui.Image
	ExtensionConnectedIcon      *woxui.Image
	ExtensionDisconnectedIcon   *woxui.Image
	ExtensionExternalIcon       *woxui.Image
	ExtensionConnectedAccent    woxui.Color
	ExtensionDisconnectedAccent woxui.Color
	Theme                       woxcomponent.Theme
	OnCancel                    func()
	OnSave                      func([]string)
	OnOpenExtensionStore        func()
}

const (
	windowGroupDialogContentWidth = float32(1040)
	windowGroupDialogBodyHeight   = float32(520)
	windowGroupLayoutCardWidth    = float32(142)
	windowGroupLayoutCardHeight   = float32(82)
	windowGroupLayoutCardGap      = float32(8)
)

// WindowGroupEditor builds the workspace layout create/edit dialog.
func WindowGroupEditor(props WindowGroupEditorProps) woxwidget.Widget {
	panelWidth := min(windowGroupDialogContentWidth+48, max(float32(768), props.Width-56))
	innerWidth := max(float32(0), panelWidth-48)
	headerHeight := float32(40)
	if props.NameError != "" {
		headerHeight = 58
	}
	fixedHeight := float32(48+20+16+14+16+38) + headerHeight
	bodyHeight := windowGroupDialogBodyHeight
	if props.Height > 0 {
		if maxBody := props.Height - 48 - fixedHeight; maxBody > 0 && maxBody < bodyHeight {
			bodyHeight = max(float32(420), maxBody)
		}
	}
	panelHeight := fixedHeight + bodyHeight
	border := windowGroupFadeColor(props.Theme.PreviewSplit, 230)
	body := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 14, Children: []woxwidget.Widget{
		windowGroupEditorHeader(props, innerWidth),
		windowGroupEditorBody(props, innerWidth, bodyHeight),
	}}
	actions := woxwidget.Align{Width: innerWidth, Height: 38, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "window-group-cancel", Label: props.CancelLabel, Height: 38, Radius: 4, FontSize: 13, Variant: woxcomponent.ButtonOutline, OnTap: props.OnCancel, Theme: props.Theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "window-group-save", Label: props.SaveLabel, Height: 38, Radius: 4, FontSize: 13, Variant: woxcomponent.ButtonPrimary, OnTap: props.OnSave, Theme: props.Theme}),
	}}}
	content := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 16, Children: []woxwidget.Widget{
		woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 16}, Color: props.Theme.ResultTitle},
		body,
		actions,
	}}
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "window-group-editor", Label: props.Title, Width: panelWidth, Height: panelHeight,
		OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "window-group-backdrop", BackdropAlpha: 210,
		Radius: 20, Padding: woxwidget.UniformInsets(24), BorderColor: border, BorderWidth: 1,
		OnEscape: props.OnCancel, Theme: props.Theme, Child: content,
	})
}

func windowGroupEditorHeader(props WindowGroupEditorProps, width float32) woxwidget.Widget {
	nameField := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: "window-group-name", Hint: props.NamePlaceholder, Width: 360, Height: 40, Radius: 4,
		Padding: woxwidget.Insets{Left: 10, Top: 9, Right: 10, Bottom: 9}, Transparent: true,
		BorderColor: windowGroupFadeColor(props.Theme.ResultSubtitle, 0.55), BorderWidth: 1,
		Value: props.GroupName, Window: props.Window, Theme: props.Theme,
		OnChanged: props.OnNameChanged,
	})
	children := []woxwidget.Widget{nameField}
	if props.NameError != "" {
		children = append(children, woxwidget.Text{Value: props.NameError, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ErrorText})
	}
	left := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: children}
	right := woxwidget.Text{Value: props.SelectDisplayLabel, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 16, Children: []woxwidget.Widget{left, woxwidget.Container{Width: max(float32(0), width-376), Child: right}}}
}

func windowGroupEditorBody(props WindowGroupEditorProps, width, height float32) woxwidget.Widget {
	leftWidth := max(float32(0), width-348)
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 18, Children: []woxwidget.Widget{
		woxwidget.Container{Width: leftWidth, Height: height, Child: windowGroupDisplayArrangement(props, leftWidth, height)},
		woxwidget.Container{Width: 330, Height: height, Child: windowGroupLayoutPanel(props, 330, height)},
	}}
}

func windowGroupDisplayArrangement(props WindowGroupEditorProps, width, height float32) woxwidget.Widget {
	if props.LoadingDisplays {
		return windowGroupMessageBox(width, height, props.Theme, "…")
	}
	if props.DisplaysError != "" {
		retry := woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: "window-group-retry-displays", Label: props.RetryLabel, Variant: woxcomponent.ButtonOutline, OnTap: props.OnRetryDisplays, Theme: props.Theme,
		})
		return woxwidget.Container{
			Width: width, Height: height, Radius: 6, BorderColor: windowGroupFadeColor(props.Theme.ResultSubtitle, 0.35), BorderWidth: 1,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				woxwidget.Text{Value: props.DisplaysError, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ErrorText},
				retry,
			}},
		}
	}
	if len(props.DisplayTiles) == 0 {
		return windowGroupMessageBox(width, height, props.Theme, props.NoDisplaysLabel)
	}
	minX, minY, maxX, maxY := displayDesktopBounds(props.DisplayTiles)
	desktopWidth := max(1.0, float64(maxX-minX))
	desktopHeight := max(1.0, float64(maxY-minY))
	const padding = float32(24)
	scale := min((width-padding*2)/float32(desktopWidth), (height-padding*2)/float32(desktopHeight))
	contentWidth := float32(desktopWidth) * scale
	contentHeight := float32(desktopHeight) * scale
	offsetX := (width - contentWidth) / 2
	offsetY := (height - contentHeight) / 2
	children := make([]woxwidget.StackChild, 0, len(props.DisplayTiles))
	for _, tile := range props.DisplayTiles {
		rect := displayTileRect(tile)
		left := offsetX + (rect.X-minX)*scale
		top := offsetY + (rect.Y-minY)*scale
		tileWidth := max(float32(90), rect.Width*scale)
		tileHeight := max(float32(58), rect.Height*scale)
		children = append(children, woxwidget.StackChild{
			Left: left, Top: top, Child: windowGroupDisplayTile(props, tile, tileWidth, tileHeight),
		})
	}
	return woxwidget.Container{
		Width: width, Height: height, Radius: 6, Color: windowGroupFadeColor(props.Theme.ResultSubtitle, 0.06),
		BorderColor: windowGroupFadeColor(props.Theme.ResultSubtitle, 0.35), BorderWidth: 1,
		Child: woxwidget.Stack{Width: width, Height: height, Children: children},
	}
}

func windowGroupDisplayTile(props WindowGroupEditorProps, tile WindowGroupDisplayTileProps, width, height float32) woxwidget.Widget {
	selectedColor := windowGroupSelectionColor()
	border := windowGroupFadeColor(props.Theme.ResultSubtitle, 0.4)
	borderWidth := float32(1)
	background := props.Theme.QueryBackground
	if tile.Selected {
		border = windowGroupFadeColor(selectedColor, 0.9)
		borderWidth = 2.5
		background = windowGroupBlendColor(windowGroupFadeColor(selectedColor, 0.08), props.Theme.QueryBackground)
	}
	slotChildren := make([]woxwidget.StackChild, 0, len(tile.Slots))
	for _, slot := range tile.Slots {
		x, y, w, h := slotFractionRect(slot, width, height)
		slotChildren = append(slotChildren, woxwidget.StackChild{
			Left: x + 3, Top: y + 3, Child: windowGroupSlotTile(props, slot, tile.Selected, tile.Index, w-6, h-6),
		})
	}
	content := woxwidget.Stack{Width: width, Height: height, Children: slotChildren}
	if tile.IsPrimary {
		content.Children = append(content.Children, woxwidget.StackChild{
			Left: width - 56, Top: 6, Child: woxwidget.Text{Value: props.PrimaryDisplayLabel, Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ResultSubtitle},
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
	return woxwidget.Gesture{
		ID: fmt.Sprintf("window-group-display-%d", tile.Index),
		OnTap: func() {
			if props.OnSelectDisplay != nil {
				props.OnSelectDisplay(tile.Index)
			}
		},
		Child: tileSurface,
	}
}

func windowGroupLayoutPanel(props WindowGroupEditorProps, width, height float32) woxwidget.Widget {
	header := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
		woxwidget.Text{Value: props.LayoutsLabel, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle},
		woxwidget.TextBlock{Value: props.LayoutsDescription, Width: width - 24, MaxLines: 3, LineHeight: 15, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle},
	}}
	groups := make([]woxwidget.Widget, 0, len(props.LayoutGroups))
	for _, group := range props.LayoutGroups {
		groups = append(groups, windowGroupLayoutGroup(props, group))
	}
	scrollContent := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: append([]woxwidget.Widget{header}, groups...)}
	contentHeight := windowGroupLayoutPanelContentHeight(props.LayoutGroups)
	innerHeight := max(float32(0), height-2)
	scroll := woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: "window-group-layout-scroll", Width: width - 2, Height: innerHeight, ContentHeight: max(innerHeight, contentHeight),
		Content: woxwidget.Container{Width: width - 2, Padding: woxwidget.UniformInsets(12), Child: scrollContent}, ThumbColor: props.Theme.ResultSubtitle,
	})
	return woxwidget.Container{
		Width: width, Height: height, Radius: 6, BorderColor: windowGroupFadeColor(props.Theme.ResultSubtitle, 0.35), BorderWidth: 1, Child: scroll,
	}
}

func windowGroupLayoutGroup(props WindowGroupEditorProps, group WindowGroupLayoutGroupProps) woxwidget.Widget {
	cards := make([]woxwidget.Widget, 0, len(group.Layouts))
	for _, layout := range group.Layouts {
		cards = append(cards, windowGroupLayoutCard(props, layout))
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Text{Value: group.SlotCountLabel, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
		woxwidget.Wrap{Gap: windowGroupLayoutCardGap, RunGap: windowGroupLayoutCardGap, Children: cards},
	}}
}

func windowGroupLayoutCard(props WindowGroupEditorProps, layout WindowGroupLayoutOptionProps) woxwidget.Widget {
	selected := layout.Selected
	background := woxui.Color{}
	border := windowGroupFadeColor(props.Theme.ResultSubtitle, 0.35)
	borderWidth := float32(1)
	selectedColor := windowGroupSelectionColor()
	if selected {
		background = windowGroupBlendColor(windowGroupFadeColor(selectedColor, 0.14), props.Theme.ActionBackground)
		border = selectedColor
		borderWidth = 2.5
	}
	layoutID := layout.ID
	miniHeight := windowGroupLayoutCardHeight - 14 - 5 - 12
	card := woxwidget.Widget(woxwidget.Container{
		Width: windowGroupLayoutCardWidth, Height: windowGroupLayoutCardHeight, Radius: 4, Color: background, BorderColor: border, BorderWidth: borderWidth,
		Padding: woxwidget.UniformInsets(7),
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
			windowGroupMiniLayout(props, layout.Slots, windowGroupLayoutCardWidth-14, miniHeight),
			woxwidget.Align{Width: windowGroupLayoutCardWidth - 14, Height: 12, Horizontal: 0.5, Child: woxwidget.Text{Value: layout.Label, Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ResultTitle}},
		}},
	})
	if selected {
		card = woxwidget.Stack{Width: windowGroupLayoutCardWidth, Height: windowGroupLayoutCardHeight, Children: []woxwidget.StackChild{
			{Left: -3, Top: -3, Child: woxwidget.Container{
				Width: windowGroupLayoutCardWidth + 6, Height: windowGroupLayoutCardHeight + 6, Radius: 7,
				BorderColor: windowGroupFadeColor(selectedColor, 0.28), BorderWidth: 3,
			}},
			{Child: card},
		}}
	}
	return woxwidget.Gesture{
		ID: "window-group-layout-" + layoutID,
		OnTap: func() {
			if props.OnSelectLayout != nil {
				props.OnSelectLayout(layoutID)
			}
		},
		Child: card,
	}
}

func windowGroupSelectionColor() woxui.Color {
	return woxui.Color{R: 74, G: 222, B: 128, A: 255}
}

func windowGroupMiniLayout(props WindowGroupEditorProps, slots []WindowGroupSlotProps, width, height float32) woxwidget.Widget {
	slotChildren := make([]woxwidget.StackChild, 0, len(slots))
	for _, slot := range slots {
		x, y, w, h := slotFractionRect(slot, width, height)
		slotChildren = append(slotChildren, woxwidget.StackChild{
			Left: x + 1.5, Top: y + 1.5, Child: woxwidget.Container{
				Width: max(float32(1), w-3), Height: max(float32(1), h-3), Radius: 2, Color: windowGroupFadeColor(props.Theme.ActionSelected, 0.7),
			},
		})
	}
	return woxwidget.Container{
		Width: width, Height: height, Radius: 3, BorderColor: windowGroupFadeColor(props.Theme.ResultSubtitle, 0.45), BorderWidth: 1,
		Child: woxwidget.Stack{Width: width, Height: height, Children: slotChildren},
	}
}

func windowGroupLayoutPanelContentHeight(groups []WindowGroupLayoutGroupProps) float32 {
	height := float32(24 + 50)
	for _, group := range groups {
		height += 20
		rows := (len(group.Layouts) + 1) / 2
		height += float32(rows)*windowGroupLayoutCardHeight + float32(max(0, rows-1))*windowGroupLayoutCardGap
		height += 14
	}
	return height
}

func windowGroupSlotTile(props WindowGroupEditorProps, slot WindowGroupSlotProps, selectedDisplay bool, displayIndex int, width, height float32) woxwidget.Widget {
	hasApp := slot.AppName != ""
	background := windowGroupFadeColor(props.Theme.ResultTitle, 0.055)
	border := windowGroupFadeColor(props.Theme.ResultSubtitle, 0.38)
	if hasApp {
		background = windowGroupFadeColor(props.Theme.ActionSelected, 0.28)
		border = windowGroupFadeColor(props.Theme.ActionSelected, 0.55)
	}
	var content woxwidget.Widget
	if hasApp {
		icon := woxwidget.Widget(woxwidget.Container{Width: 18, Height: 18})
		if slot.AppIcon != nil {
			icon = woxwidget.Image{Source: slot.AppIcon, Width: 18, Height: 18}
		} else if props.AppsIcon != nil {
			icon = woxwidget.Image{Source: props.AppsIcon, Width: 18, Height: 18}
		}
		content = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			icon,
			woxwidget.Text{Value: slot.AppName, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle},
		}}
	} else {
		addIcon := woxwidget.Widget(woxwidget.Container{Width: 15, Height: 15})
		if props.AddSlotIcon != nil {
			addIcon = woxwidget.Image{Source: props.AddSlotIcon, Width: 15, Height: 15}
		}
		content = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 5, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			addIcon,
			woxwidget.Text{Value: props.ChooseAppLabel, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle},
		}}
	}
	tile := woxwidget.Container{
		Width: width, Height: height, Radius: 4, Color: background, BorderColor: border, BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 8, Right: 8}, Child: woxwidget.Align{Width: width - 16, Height: height, Horizontal: 0.5, Vertical: 0.5, Child: content},
	}
	slotID := slot.ID
	onTap := func() {
		if !selectedDisplay {
			if props.OnSelectDisplay != nil {
				props.OnSelectDisplay(displayIndex)
			}
			return
		}
		if props.OnOpenAppPicker != nil {
			props.OnOpenAppPicker(slotID)
		}
	}
	wrapped := woxwidget.Gesture{ID: "window-group-slot-" + slotID, OnTap: onTap, Child: tile}
	if hasApp && slot.IsBrowser && props.OnOpenUrlEditor != nil {
		urlPillColor := woxui.Color{R: 110, G: 231, B: 183, A: 255}
		urlLabel := props.BrowserUrlEmptyLabel
		if slot.UrlCount > 0 {
			urlLabel = strconv.Itoa(slot.UrlCount)
		}
		linkIcon := woxwidget.Widget(woxwidget.Container{Width: 13, Height: 13})
		if props.LinkIcon != nil {
			linkIcon = woxwidget.Image{Source: props.LinkIcon, Width: 13, Height: 13}
		}
		pill := woxwidget.Container{
			Padding: woxwidget.Insets{Left: 6, Right: 6, Top: 3, Bottom: 3}, Radius: 4,
			Color:       windowGroupBlendColor(windowGroupFadeColor(urlPillColor, 0.18), props.Theme.Background),
			BorderColor: windowGroupFadeColor(urlPillColor, 0.75), BorderWidth: 1,
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 3, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				linkIcon,
				woxwidget.Text{Value: urlLabel, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: urlPillColor},
			}},
		}
		return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
			{Child: wrapped},
			{Left: width - 56, Top: height - 24, Child: woxwidget.Gesture{ID: "window-group-url-" + slotID, OnTap: func() {
				if !selectedDisplay {
					if props.OnSelectDisplay != nil {
						props.OnSelectDisplay(displayIndex)
					}
					return
				}
				props.OnOpenUrlEditor(slotID)
			}, Child: pill}},
		}}
	}
	return wrapped
}

// WindowGroupUrlEditor builds the Flutter-aligned browser URL table dialog.
func WindowGroupUrlEditor(props WindowGroupUrlEditorProps) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: "window-group-url-editor-state", Type: (*windowGroupURLState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &windowGroupURLState{} },
	}
}

// windowGroupURLState owns dialog-local rows so cancel never mutates the workspace assignment.
type windowGroupURLState struct {
	urls          []string
	rowEditor     int
	draft         string
	draftError    string
	deletePending int
}

func (s *windowGroupURLState) InitState(_ woxwidget.StateContext, widget any) {
	props := widget.(WindowGroupUrlEditorProps)
	s.urls = make([]string, 0, len(props.URLs))
	for _, value := range props.URLs {
		if value = strings.TrimSpace(value); value != "" {
			s.urls = append(s.urls, value)
		}
	}
	s.rowEditor = -2
	s.deletePending = -1
}

func (s *windowGroupURLState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

func (s *windowGroupURLState) Dispose() {}

func (s *windowGroupURLState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(WindowGroupUrlEditorProps)
	main := s.buildDialog(context, props)
	layers := []woxwidget.StackChild{{Child: main}}
	if s.rowEditor >= -1 {
		layers = append(layers, woxwidget.StackChild{Child: s.buildRowEditor(context, props)})
	}
	if s.deletePending >= 0 {
		layers = append(layers, woxwidget.StackChild{Child: FormTableDeleteDialog(FormTableDeleteDialogProps{
			Width: props.Width, Height: props.Height, Message: props.DeleteConfirmation, CancelLabel: props.CancelLabel, DeleteLabel: props.DeleteLabel, Theme: props.Theme,
			OnCancel: func() { context.SetState(func() { s.deletePending = -1 }) },
			OnDelete: func() {
				context.SetState(func() {
					if s.deletePending >= 0 && s.deletePending < len(s.urls) {
						s.urls = append(s.urls[:s.deletePending], s.urls[s.deletePending+1:]...)
					}
					s.deletePending = -1
				})
			},
		})})
	}
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: layers}
}

// buildDialog composes the URL table and extension status while row edits remain local until save.
func (s *windowGroupURLState) buildDialog(context woxwidget.StateContext, props WindowGroupUrlEditorProps) woxwidget.Widget {
	const contentWidth = float32(480)
	const tableHeight = float32(164)
	panelWidth := min(contentWidth+48, max(float32(360), props.Width-56))
	innerWidth := max(float32(0), panelWidth-48)
	panelHeight := min(props.Height-48, tableHeight+176)
	rows := make([]FormTableRow, len(s.urls))
	for index, value := range s.urls {
		rows[index] = FormTableRow{Index: index, Cells: []FormTableCell{{Text: value}}}
	}
	table := FormTableField(FormTableFieldProps{
		ID: "window-group-url-table", Width: innerWidth, Height: tableHeight, MaxHeight: 120, InlineTitle: true, HideCloneAction: true,
		Columns: []FormTableColumn{{Label: "URL", Width: 360}}, Rows: rows,
		AddLabel: props.AddLabel, EditLabel: props.EditLabel, DeleteLabel: props.DeleteLabel, OperationLabel: props.OperationLabel, EmptyLabel: props.EmptyLabel,
		AddIcon: props.AddIcon, EditIcon: props.EditIcon, DeleteIcon: props.DeleteIcon, EmptyIcon: props.EmptyIcon, Theme: props.Theme,
		OnAdd: func() {
			context.SetState(func() {
				s.rowEditor = -1
				s.draft = ""
				s.draftError = ""
			})
		},
		OnOpenRow: func(index int) {
			if index < 0 || index >= len(s.urls) {
				return
			}
			context.SetState(func() {
				s.rowEditor = index
				s.draft = s.urls[index]
				s.draftError = ""
			})
		},
		OnDeleteRow: func(index int) {
			if index >= 0 && index < len(s.urls) {
				context.SetState(func() { s.deletePending = index })
			}
		},
	})
	actions := woxwidget.Align{Width: innerWidth, Height: 38, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
			woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "window-group-url-cancel", Label: props.CancelLabel, Height: 38, Variant: woxcomponent.ButtonOutline, OnTap: props.OnCancel, Theme: props.Theme}),
			woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "window-group-url-save", Label: props.SaveLabel, Height: 38, Variant: woxcomponent.ButtonPrimary, OnTap: func() {
				if props.OnSave == nil {
					return
				}
				urls := make([]string, 0, len(s.urls))
				for _, value := range s.urls {
					if normalized := normalizeWindowGroupURL(value); normalized != "" {
						urls = append(urls, normalized)
					}
				}
				props.OnSave(urls)
			}, Theme: props.Theme}),
		},
	}}
	content := woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Container{Width: innerWidth, Height: 28, Child: woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 16}, Color: props.Theme.ResultTitle}},
		woxwidget.Container{Width: innerWidth, Height: 22, Child: woxwidget.Text{Value: props.Description, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle}},
		table,
		woxwidget.Container{Width: innerWidth, Height: 50, Padding: woxwidget.Insets{Top: 6}, Child: windowGroupExtensionStatus(props, innerWidth)},
		woxwidget.Container{Width: innerWidth, Height: 44, Padding: woxwidget.Insets{Top: 6}, Child: actions},
	}}
	border := windowGroupFadeColor(props.Theme.PreviewSplit, 0.9)
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "window-group-url-editor", Label: props.Title, Width: panelWidth, Height: panelHeight,
		OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "window-group-url-backdrop", BackdropAlpha: 210,
		Radius: 20, Padding: woxwidget.Insets{Left: 24, Top: 16, Right: 24, Bottom: 16}, BorderColor: border, BorderWidth: 1, OnEscape: props.OnCancel, Theme: props.Theme, Child: content,
	})
}

// buildRowEditor reuses the retained text field for one add or edit operation.
func (s *windowGroupURLState) buildRowEditor(context woxwidget.StateContext, props WindowGroupUrlEditorProps) woxwidget.Widget {
	panelWidth := min(float32(560), max(float32(360), props.Width-80))
	innerWidth := max(float32(0), panelWidth-48)
	title := props.AddLabel + " URL"
	if s.rowEditor >= 0 {
		title = props.EditLabel + " URL"
	}
	field := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: "window-group-url-row-value", Label: "URL", Hint: "URL", Width: innerWidth, Height: 40, Radius: 4,
		Padding: woxwidget.Insets{Left: 10, Top: 9, Right: 10, Bottom: 9}, Transparent: true,
		BorderColor: windowGroupFadeColor(props.Theme.ResultSubtitle, 0.55), BorderWidth: 1,
		Value: s.draft, Window: props.Window, Theme: props.Theme,
		OnChanged: func(value string) { context.SetState(func() { s.draft = value; s.draftError = "" }) },
	})
	actions := woxwidget.Align{Width: innerWidth, Height: 38, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
			woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "window-group-url-row-cancel", Label: props.CancelLabel, Height: 38, Variant: woxcomponent.ButtonOutline, OnTap: func() { context.SetState(func() { s.rowEditor = -2; s.draftError = "" }) }, Theme: props.Theme}),
			woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "window-group-url-row-save", Label: props.SaveLabel, Height: 38, Variant: woxcomponent.ButtonPrimary, OnTap: func() {
				value := strings.TrimSpace(s.draft)
				if value == "" {
					context.SetState(func() { s.draftError = props.RequiredLabel })
					return
				}
				context.SetState(func() {
					if s.rowEditor >= 0 && s.rowEditor < len(s.urls) {
						s.urls[s.rowEditor] = value
					} else {
						s.urls = append(s.urls, value)
					}
					s.rowEditor = -2
					s.draftError = ""
				})
			}, Theme: props.Theme}),
		},
	}}
	children := []woxwidget.Widget{
		woxwidget.Container{Width: innerWidth, Height: 30, Child: woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText}},
		field,
	}
	if s.draftError != "" {
		children = append(children, woxwidget.Container{Width: innerWidth, Height: 22, Padding: woxwidget.Insets{Top: 4}, Child: woxwidget.Text{Value: s.draftError, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ErrorText}})
	}
	children = append(children, woxwidget.Container{Width: innerWidth, Height: 54, Padding: woxwidget.Insets{Top: 16}, Child: actions})
	panelHeight := float32(176)
	if s.draftError != "" {
		panelHeight += 22
	}
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "window-group-url-row-editor", Label: title, Width: panelWidth, Height: panelHeight,
		OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "window-group-url-row-backdrop", BackdropAlpha: 210,
		Radius: 20, Padding: woxwidget.UniformInsets(24), InitialFocus: "window-group-url-row-value", Theme: props.Theme,
		OnEscape: func() { context.SetState(func() { s.rowEditor = -2; s.draftError = "" }) },
		Child:    woxwidget.Flex{Axis: woxwidget.Vertical, Children: children},
	})
}

// windowGroupExtensionStatus mirrors Flutter's loading, connected, and install states.
func windowGroupExtensionStatus(props WindowGroupUrlEditorProps, width float32) woxwidget.Widget {
	accent := props.ExtensionDisconnectedAccent
	icon := props.ExtensionDisconnectedIcon
	label := props.ExtensionDisconnectedLabel
	if props.ExtensionChecking {
		accent = props.Theme.ResultSubtitle
		icon = props.ExtensionLoadingIcon
		label = "..."
	} else if props.ExtensionConnected {
		accent = props.ExtensionConnectedAccent
		icon = props.ExtensionConnectedIcon
		label = props.ExtensionConnectedLabel
	}
	background := accent
	background.A = 24
	border := accent
	border.A = 115
	content := []woxwidget.Widget{}
	if icon != nil {
		content = append(content, woxwidget.Image{Source: icon, Width: 14, Height: 14})
	}
	if !props.ExtensionChecking && !props.ExtensionConnected {
		textWidth := max(float32(0), width-52)
		installChildren := []woxwidget.Widget{}
		if props.ExtensionExternalIcon != nil {
			installChildren = append(installChildren, woxwidget.Image{Source: props.ExtensionExternalIcon, Width: 11, Height: 11})
		}
		installChildren = append(installChildren, woxwidget.Text{Value: props.ExtensionInstallLabel, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: accent})
		content = append(content, woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
			woxwidget.TextBlock{Value: label, Width: textWidth, Height: 16, MaxLines: 1, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: accent},
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 3, Children: installChildren},
		}})
	} else {
		content = append(content, woxwidget.TextBlock{Value: label, Width: max(float32(0), width-42), Height: 18, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ActionText})
	}
	statusRow := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: content}
	box := woxwidget.Container{Width: width, Height: 40, Radius: 4, Color: background, BorderColor: border, BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 8, Right: 8}, Child: woxwidget.Align{Width: max(float32(0), width-16), Height: 40, Vertical: 0.5, Child: statusRow}}
	if !props.ExtensionChecking && !props.ExtensionConnected {
		return woxwidget.Gesture{ID: "window-group-url-extension-install", OnTap: props.OnOpenExtensionStore, Child: box}
	}
	return box
}

// normalizeWindowGroupURL applies Flutter's save-time HTTP(S) normalization contract.
func normalizeWindowGroupURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") {
		return "https://" + value
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return value
	}
	return ""
}

func windowGroupMessageBox(width, height float32, theme woxcomponent.Theme, message string) woxwidget.Widget {
	return woxwidget.Container{
		Width: width, Height: height, Radius: 6, BorderColor: windowGroupFadeColor(theme.ResultSubtitle, 0.35), BorderWidth: 1,
		Child: woxwidget.Align{Width: width, Height: height, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: message, Style: woxui.TextStyle{Size: 13}, Color: theme.ResultSubtitle}},
	}
}

func displayDesktopBounds(tiles []WindowGroupDisplayTileProps) (minX, minY, maxX, maxY float32) {
	if len(tiles) == 0 {
		return 0, 0, 1, 1
	}
	first := displayTileRect(tiles[0])
	minX, minY, maxX, maxY = first.X, first.Y, first.X+first.Width, first.Y+first.Height
	for _, tile := range tiles[1:] {
		rect := displayTileRect(tile)
		minX = min(minX, rect.X)
		minY = min(minY, rect.Y)
		maxX = max(maxX, rect.X+rect.Width)
		maxY = max(maxY, rect.Y+rect.Height)
	}
	return minX, minY, maxX, maxY
}

type displayRect struct {
	X, Y, Width, Height float32
}

func displayTileRect(tile WindowGroupDisplayTileProps) displayRect {
	rect := tile.Bounds
	if rect.Width <= 0 || rect.Height <= 0 {
		rect = tile.WorkArea
	}
	return displayRect{X: float32(rect.X), Y: float32(rect.Y), Width: max(1, float32(rect.Width)), Height: max(1, float32(rect.Height))}
}

func slotFractionRect(slot WindowGroupSlotProps, width, height float32) (x, y, w, h float32) {
	cellW := width / float32(max(1, slot.Cols))
	cellH := height / float32(max(1, slot.Rows))
	return cellW * float32(slot.Col), cellH * float32(slot.Row), cellW * float32(slot.ColSpan), cellH * float32(slot.RowSpan)
}

func windowGroupFadeColor(color woxui.Color, alpha float32) woxui.Color {
	color.A = uint8(float32(color.A) * alpha)
	return color
}

func windowGroupBlendColor(overlay, base woxui.Color) woxui.Color {
	if overlay.A == 0 {
		return base
	}
	alpha := float32(overlay.A) / 255
	return woxui.Color{
		R: uint8(float32(overlay.R)*alpha + float32(base.R)*(1-alpha)),
		G: uint8(float32(overlay.G)*alpha + float32(base.G)*(1-alpha)),
		B: uint8(float32(overlay.B)*alpha + float32(base.B)*(1-alpha)),
		A: base.A,
	}
}
