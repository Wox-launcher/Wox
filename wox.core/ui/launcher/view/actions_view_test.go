package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestActionsBoundaryEqualCoversAllFields(t *testing.T) {
	woxwidget.AssertEqualCoversAllFields(t, ActionItem{})
	woxwidget.AssertEqualCoversAllFields(t, ActionsProps{})
	woxwidget.AssertEqualCoversAllFields(t, actionSearchProps{})
}

type actionSearchHostServices struct{}

func (actionSearchHostServices) MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error) {
	return woxui.TextMetrics{Size: woxui.Size{Width: float32(len([]rune(text))) * max(style.Size/2, 1), Height: max(style.Size, 1)}}, nil
}
func (actionSearchHostServices) Invalidate() error { return nil }
func (actionSearchHostServices) InvalidateRect(woxui.Rect) error {
	return nil
}
func (actionSearchHostServices) SetTextInputState(woxui.TextInputState) error { return nil }
func (actionSearchHostServices) SetPointerCursor(woxui.PointerCursor) error   { return nil }
func (actionSearchHostServices) UpdateAccessibility(woxui.AccessibilityTree, woxui.AccessibilityActionHandler) error {
	return nil
}

func TestActionSearchBoundaryVerifyPasses(t *testing.T) {
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return actionSearchBoundary(actionSearchProps{
			Width: 300, Height: 40, Window: &woxui.Window{},
			Style: woxui.TextStyle{Size: 14}, Theme: woxcomponent.Theme{},
		})
	})
	host.AttachServices(actionSearchHostServices{})
	if err := host.SetRepaintDebugMode(woxwidget.RepaintDebugVerify); err != nil {
		t.Fatal(err)
	}
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 300, Height: 40}, PixelSize: woxui.PixelSize{Width: 300, Height: 40}, Scale: 1}
	host.Frame(&woxui.DisplayList{}, frame)
	host.Frame(&woxui.DisplayList{}, frame)
	if diagnostics := host.Snapshot().Diagnostics; len(diagnostics) != 0 {
		t.Fatalf("action search verify diagnostics = %v", diagnostics)
	}
}

func TestActionSearchBoundaryKeepsCaretPaintUnretained(t *testing.T) {
	boundary := actionSearchBoundary(actionSearchProps{}).(woxwidget.Boundary[actionSearchProps])
	if !boundary.DisableRetainedPaint {
		t.Fatal("action search must paint directly so its caret blink reaches the native surface")
	}
}

func TestActionsEmptyStateCentersSearchIconAndMessage(t *testing.T) {
	view := buildActionsView(woxwidget.StateContext{}, ActionsProps{
		WindowWidth: 600, WindowHeight: 600, DensityScale: 1, ActionPadding: woxwidget.UniformInsets(10),
		Theme: woxcomponent.Theme{}, ActionHeader: woxui.Color{A: 255}, NoMatchesLabel: "No matches",
	}, woxwidget.NewScrollController(0)).(woxwidget.Gesture)
	panel := view.Child.(woxwidget.Container)
	content := panel.Child.(woxwidget.Flex)
	actionList := content.Children[2].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	empty := actionList.Content.(woxwidget.Flex).Children[0].(woxwidget.Align)
	if empty.Horizontal != 0.5 || empty.Vertical != 0.5 || empty.Width != actionList.Width || empty.Height != ActionRowHeight {
		t.Fatalf("empty state geometry = %+v, want centered %vx%v slot", empty, actionList.Width, ActionRowHeight)
	}
	row := empty.Child.(woxwidget.Flex)
	if row.Axis != woxwidget.Horizontal || row.Gap != 8 || row.CrossAxisAlignment != woxwidget.CrossAxisCenter || len(row.Children) != 2 {
		t.Fatalf("empty state content = %#v, want centered icon and message row", row)
	}
	if _, ok := row.Children[0].(woxwidget.Image); !ok {
		t.Fatalf("empty state icon = %T, want shared search image", row.Children[0])
	}
	if message, ok := row.Children[1].(woxwidget.Text); !ok || message.Value != "No matches" {
		t.Fatalf("empty state message = %#v, want No matches", row.Children[1])
	}
}

func TestActionHeaderCentersLabel(t *testing.T) {
	view := buildActionsView(woxwidget.StateContext{}, ActionsProps{
		WindowWidth: 600, WindowHeight: 600, DensityScale: 1, ActionPadding: woxwidget.UniformInsets(10),
		Theme: woxcomponent.Theme{}, ActionHeader: woxui.Color{A: 255}, HeaderLabel: "操作",
	}, woxwidget.NewScrollController(0)).(woxwidget.Gesture)
	panel := view.Child.(woxwidget.Container)
	header := panel.Child.(woxwidget.Flex).Children[0].(woxwidget.Align)
	if header.Width != ActionPanelContentWidth || header.Height != ActionHeaderHeight || header.Vertical != 0.5 {
		t.Fatalf("action header slot = %#v, want a full-width centered %v-high slot", header, ActionHeaderHeight)
	}
	label := header.Child.(woxwidget.TextBlock)
	if label.Height != ActionHeaderHeight || label.LineHeight != ActionHeaderHeight || label.AlignmentY != 0.5 || label.Value != "操作" {
		t.Fatalf("action header label = %#v, want an 18px optically centered line", label)
	}
}

func TestActionRowCentersIconAndLabel(t *testing.T) {
	view := buildActionsView(woxwidget.StateContext{}, ActionsProps{
		WindowWidth: 600, WindowHeight: 600, DensityScale: 1, ActionPadding: woxwidget.UniformInsets(10),
		Theme: woxcomponent.Theme{}, Items: []ActionItem{{ID: "open", Label: "打开 系统命令 设置", Icon: &woxui.Image{}}},
	}, woxwidget.NewScrollController(0)).(woxwidget.Gesture)
	panel := view.Child.(woxwidget.Container)
	actionList := panel.Child.(woxwidget.Flex).Children[2].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	row := actionList.Content.(woxwidget.Flex).Children[0].(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	content := row.Child.(woxwidget.Flex)
	if content.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("action row alignment = %v, want vertical center", content.CrossAxisAlignment)
	}
	icon := content.Children[0].(woxwidget.Align)
	if icon.Height != ActionRowHeight || icon.Vertical != 0.5 || icon.Child.(woxwidget.Container).Padding.Top != 0 {
		t.Fatalf("action icon slot = %#v, want a full-height centered slot", icon)
	}
	label := content.Children[1].(woxwidget.Align).Child.(woxwidget.TextBlock)
	if label.Height != 18 || label.LineHeight != 18 || label.AlignmentY != 0.5 || label.Value != "打开 系统命令 设置" {
		t.Fatalf("action label slot = %#v, want an 18px optically centered line", label)
	}
}
