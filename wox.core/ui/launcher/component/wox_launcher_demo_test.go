package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxLauncherDemoOwnsSharedWindowChrome(t *testing.T) {
	backdrop := &woxui.Image{}
	demo := WoxLauncherDemo(LauncherDemoProps{
		Width: 400, Height: 240, Backdrop: backdrop, Background: woxui.Color{A: 180}, Opacity: 1, ShowQuery: true, ShowToolbar: true,
		Theme: Theme{QueryText: woxui.Color{A: 255}, PreviewSplit: woxui.Color{A: 90}}, Query: "query",
		Results: []LauncherDemoResult{{Title: "result"}},
	}).(woxwidget.Clip)
	children := demo.Child.(woxwidget.Stack).Children

	if children[0].Child.(woxwidget.Image).Source != backdrop {
		t.Fatal("launcher demo did not preserve its backdrop")
	}
	border := children[len(children)-1].Child.(woxwidget.Container)
	if border.Radius != 12 || border.BorderWidth != 1 {
		t.Fatalf("launcher demo border = %#v, want shared 12px window chrome", border)
	}
}

func TestWoxLauncherDemoCentersResultContent(t *testing.T) {
	row := demoResultRow(LauncherDemoProps{Theme: Theme{}}, LauncherDemoResult{Title: "Result", Subtitle: "Subtitle", Tail: "P1"}, 400, 56, 255).(woxwidget.Container)
	content := row.Child.(woxwidget.Flex)
	text := content.Children[1].(woxwidget.Clip).Child.(woxwidget.Align)
	icon := content.Children[0].(woxwidget.Align)
	tail := content.Children[2].(woxwidget.Align)

	if row.Width != 400 || row.Height != 56 || row.Padding.Top != 3 || icon.Height != 50 || text.Height != 50 || tail.Height != 50 || text.Vertical != .5 {
		t.Fatalf("result alignment = %#v, want vertically centered shared row", content)
	}
}

func TestWoxLauncherDemoKeepsQueryPartsAdjacent(t *testing.T) {
	query := demoQuery(LauncherDemoProps{Opacity: 1, QueryParts: []LauncherDemoQueryPart{
		{Text: "wpm", Color: woxui.Color{A: 255}}, {Text: " install", Color: woxui.Color{A: 255}},
	}}, 55, 255).(woxwidget.Container)
	querySlot := query.Child.(woxwidget.Flex).Children[0].(woxwidget.Expanded).Child.(woxwidget.Align)
	parts := querySlot.Child.(woxwidget.Flex)

	if _, firstIsText := parts.Children[0].(woxwidget.Text); !firstIsText {
		t.Fatalf("query part = %T, want directly measured text", parts.Children[0])
	}
	if _, secondIsText := parts.Children[1].(woxwidget.Text); !secondIsText {
		t.Fatalf("query part = %T, want adjacent directly measured text", parts.Children[1])
	}
	if parts.CrossAxisAlignment != woxwidget.CrossAxisCenter || parts.Children[0].(woxwidget.Text).Style.Size != QueryFontSize {
		t.Fatalf("query alignment = %#v, want production query typography and centerline", parts)
	}
	if querySlot.Height != 55 || querySlot.Vertical != .5 {
		t.Fatalf("query slot alignment = %#v, want full-height vertical center", querySlot)
	}
}

func TestWoxLauncherDemoUsesProductionLauncherGeometry(t *testing.T) {
	demo := WoxLauncherDemo(LauncherDemoProps{
		Width: 600, Height: 320, Opacity: 1, ShowQuery: true, ShowToolbar: true,
		Results: []LauncherDemoResult{{Title: "Result"}},
	}).(woxwidget.Clip)
	children := demo.Child.(woxwidget.Stack).Children
	query := children[2]
	result := children[3]

	if query.Left != 10 || query.Top != 10 || result.Left != 10 || result.Top != 73 || result.Child.(woxwidget.Container).Height != 56 {
		t.Fatalf("launcher geometry = query %.0f/%.0f result %.0f/%.0f/%.0f", query.Left, query.Top, result.Left, result.Top, result.Child.(woxwidget.Container).Height)
	}
}

func TestWoxLauncherDemoHighlightsOnlyResultSubtitle(t *testing.T) {
	row := demoResultRow(LauncherDemoProps{
		Theme: Theme{}, HighlightTarget: LauncherDemoHighlightResultSubtitle, HighlightColor: woxui.Color{A: 255},
	}, LauncherDemoResult{Title: "Result", Subtitle: "Subtitle"}, 400, 56, 255).(woxwidget.Container)
	labels := row.Child.(woxwidget.Flex).Children[1].(woxwidget.Clip).Child.(woxwidget.Align).Child.(woxwidget.Flex)

	if _, titleHighlighted := labels.Children[0].(woxwidget.Container); titleHighlighted {
		t.Fatal("result title was highlighted while locating subtitle")
	}
	if subtitle, ok := labels.Children[1].(woxwidget.Container); !ok || subtitle.BorderWidth != 2 {
		t.Fatalf("result subtitle highlight = %#v, want a local 2px highlight", labels.Children[1])
	}
}

func TestWoxLauncherDemoPreservesTransparentThemeColors(t *testing.T) {
	transparent := woxui.Color{R: 255, G: 255, B: 255}
	if demoColorOpacity(transparent, 1).A != 0 {
		t.Fatal("transparent demo color became opaque")
	}
	mica := demoMicaColor(woxui.Color{R: 22, G: 22, B: 26, A: 133})
	if mica.A < 163 || mica.A > 219 {
		t.Fatalf("demo mica alpha = %d, want 0.64-0.86", mica.A)
	}
}

func TestWoxLauncherDemoReservesPreviewAboveToolbar(t *testing.T) {
	demo := WoxLauncherDemo(LauncherDemoProps{
		Width: 600, Height: 320, Opacity: 1, ShowQuery: true, ShowToolbar: true, ResultWidth: 350,
		Preview: woxwidget.Container{Width: 234, Height: 201},
		Results: []LauncherDemoResult{{Title: "One"}, {Title: "Two"}, {Title: "Three"}},
	}).(woxwidget.Clip)
	children := demo.Child.(woxwidget.Stack).Children
	preview := children[len(children)-3]
	toolbar := children[len(children)-2]
	previewClip := preview.Child.(woxwidget.Clip)

	if demo.Height != 320 || preview.Top+previewClip.Height > toolbar.Top {
		t.Fatalf("preview bottom = %.0f, toolbar top = %.0f, demo height = %.0f", preview.Top+previewClip.Height, toolbar.Top, demo.Height)
	}
}
