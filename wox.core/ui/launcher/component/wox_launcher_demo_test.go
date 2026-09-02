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

	if image := children[0].Child.(woxwidget.Image); image.Source != backdrop || image.Fit != woxwidget.ImageFitCover {
		t.Fatalf("launcher demo backdrop = %#v, want covered wallpaper crop", children[0].Child)
	}
	border := children[len(children)-1].Child.(woxwidget.Container)
	if border.Radius != 12 || border.BorderWidth != 1 {
		t.Fatalf("launcher demo border = %#v, want shared 12px window chrome", border)
	}
	toolbar := children[len(children)-2].Child.(woxwidget.Container).Child.(woxwidget.Clip)
	fill := toolbar.Child.(woxwidget.Stack).Children[0]
	if fill.Top != toolbar.Height-demo.Height {
		t.Fatalf("toolbar fill top = %v, want window-sized rounded rect aligned to the demo", fill.Top)
	}
	if box := fill.Child.(woxwidget.Container); box.Radius != 12 || box.Height != demo.Height {
		t.Fatalf("toolbar fill = %#v, want the 12px window corners clipped to the footer", box)
	}
}

func TestWoxLauncherDemoKeepsThemedChromeWhileFading(t *testing.T) {
	backdrop := &woxui.Image{Width: 10, Height: 10}
	background := woxui.Color{R: 22, G: 22, B: 26, A: 180}
	fading := WoxLauncherDemo(LauncherDemoProps{
		Width: 400, Height: 240, Backdrop: backdrop, Background: background, ShowQuery: true, ShowToolbar: true,
		Theme: Theme{QueryText: woxui.Color{A: 255}, Background: background}, Query: "query", Opacity: .4,
	}).(woxwidget.Clip)
	children := fading.Child.(woxwidget.Stack).Children
	if image := children[0].Child.(woxwidget.Image); image.Source != backdrop {
		t.Fatal("fading demo dropped the blurred wallpaper underlay")
	}
	if mica := children[1].Child.(woxwidget.Container); mica.Color.A != demoMicaColor(background).A {
		t.Fatalf("fading mica alpha = %d, want rest chrome so the window does not punch through the previous scene", mica.Color.A)
	}
}

func TestWoxLauncherDemoKeepsQuietWindowHairline(t *testing.T) {
	glass := launcherDemoBorderColor(t, woxui.Color{R: 255, G: 255, B: 255, A: 41})
	if glass != (woxui.Color{R: 255, G: 255, B: 255, A: 41}) {
		t.Fatalf("glass hairline = %#v, want the theme's 0.16 white split", glass)
	}
	opaque := launcherDemoBorderColor(t, woxui.Color{R: 255, G: 255, B: 255, A: 255})
	if opaque.A != demoWindowBorderMaxAlpha {
		t.Fatalf("opaque split alpha = %d, want capped window chrome %d", opaque.A, demoWindowBorderMaxAlpha)
	}
}

func launcherDemoBorderColor(t *testing.T, split woxui.Color) woxui.Color {
	t.Helper()
	demo := WoxLauncherDemo(LauncherDemoProps{
		Width: 400, Height: 180, Opacity: 1, ShowQuery: true, Theme: Theme{PreviewSplit: split}, Query: "query",
	}).(woxwidget.Clip)
	children := demo.Child.(woxwidget.Stack).Children
	return children[len(children)-1].Child.(woxwidget.Container).BorderColor
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

func TestWoxLauncherDemoResultTailPadsHorizontalText(t *testing.T) {
	row := demoResultRow(LauncherDemoProps{Theme: Theme{}}, LauncherDemoResult{Title: "Result", Tail: "已就绪"}, 400, 56, 255).(woxwidget.Container)
	tail := row.Child.(woxwidget.Flex).Children[2].(woxwidget.Align).Child.(woxwidget.Container)
	inner := tail.Child.(woxwidget.Align)
	if tail.Padding.Left != 8 || tail.Padding.Right != 8 || tail.Width != 49 || inner.Width != 33 {
		t.Fatalf("result tail = width %v padding %#v inner %v, want 8px inset around CJK text", tail.Width, tail.Padding, inner.Width)
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

func TestWoxLauncherDemoCollapsesFadedResults(t *testing.T) {
	results := []LauncherDemoResult{{Title: "One"}, {Title: "Two"}, {Title: "Three"}}
	hidden := WoxLauncherDemo(LauncherDemoProps{
		Width: 400, Height: 260, Opacity: 1, ShowQuery: true, ShowToolbar: true,
		FadeResults: true, ResultsOpacity: 0, Results: results,
	}).(woxwidget.Clip)
	shown := WoxLauncherDemo(LauncherDemoProps{
		Width: 400, Height: 260, Opacity: 1, ShowQuery: true, ShowToolbar: true,
		FadeResults: true, ResultsOpacity: 1, Results: results,
	}).(woxwidget.Clip)

	if hidden.Height != 113 {
		t.Fatalf("collapsed faded demo height = %.0f, want query plus toolbar only", hidden.Height)
	}
	if shown.Height <= hidden.Height {
		t.Fatalf("visible demo height = %.0f, collapsed height = %.0f, want results to grow the window", shown.Height, hidden.Height)
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

func TestWoxLauncherDemoResultTailUsesTailColor(t *testing.T) {
	subtitle := woxui.Color{R: 10, G: 20, B: 30, A: 255}
	tail := woxui.Color{R: 200, G: 80, B: 40, A: 255}
	selectedTail := woxui.Color{R: 40, G: 180, B: 90, A: 255}
	theme := Theme{ResultSubtitle: subtitle, ResultTail: tail, SelectedTail: selectedTail}

	normal := demoResultRow(LauncherDemoProps{Theme: theme}, LauncherDemoResult{Title: "Result", Subtitle: "Subtitle", Tail: "P1"}, 400, 56, 255).(woxwidget.Container)
	if got := launcherDemoResultTailText(t, normal); got != tail {
		t.Fatalf("unselected result tail color = %#v, want %#v", got, tail)
	}
	if got := launcherDemoResultSubtitleText(t, normal); got != subtitle {
		t.Fatalf("result subtitle color = %#v, want %#v", got, subtitle)
	}

	selected := demoResultRow(LauncherDemoProps{Theme: theme}, LauncherDemoResult{Title: "Result", Subtitle: "Subtitle", Tail: "P1", Selected: true}, 400, 56, 255).(woxwidget.Container)
	if got := launcherDemoResultTailText(t, selected); got != selectedTail {
		t.Fatalf("selected result tail color = %#v, want %#v", got, selectedTail)
	}
}

func launcherDemoResultTailText(t *testing.T, row woxwidget.Container) woxui.Color {
	t.Helper()
	tail := row.Child.(woxwidget.Flex).Children[2].(woxwidget.Align).Child.(woxwidget.Container)
	return tail.Child.(woxwidget.Align).Child.(woxwidget.Text).Color
}

func launcherDemoResultSubtitleText(t *testing.T, row woxwidget.Container) woxui.Color {
	t.Helper()
	labels := row.Child.(woxwidget.Flex).Children[1].(woxwidget.Clip).Child.(woxwidget.Align).Child.(woxwidget.Flex)
	return labels.Children[1].(woxwidget.Text).Color
}

func TestWoxLauncherDemoHighlightsSelectedResultTail(t *testing.T) {
	row := demoResultRow(LauncherDemoProps{
		Theme: Theme{}, HighlightTarget: LauncherDemoHighlightSelectedTail, HighlightColor: woxui.Color{A: 255},
	}, LauncherDemoResult{Title: "Result", Tail: "Live", Selected: true}, 400, 56, 255).(woxwidget.Container)
	tail := row.Child.(woxwidget.Flex).Children[2].(woxwidget.Align).Child.(woxwidget.Stack)
	if overlay := tail.Children[1].Child.(woxwidget.Container); overlay.BorderWidth != 2 {
		t.Fatalf("selected result tail highlight = %#v, want a local 2px highlight", overlay)
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

func TestWoxLauncherDemoActionRowsLeadWithIcons(t *testing.T) {
	demo := WoxLauncherDemo(LauncherDemoProps{
		Width: 400, Height: 280, Opacity: 1, ShowQuery: true, ShowToolbar: true, ActionProgress: 1,
		Query: "query", ActionCopy: "Copy", ActionMore: "More",
		Results: []LauncherDemoResult{{Title: "result"}},
	}).(woxwidget.Clip)
	children := demo.Child.(woxwidget.Stack).Children
	panel := children[len(children)-2].Child.(woxwidget.Container)
	rows := panel.Child.(woxwidget.Flex).Children
	if len(rows) < 4 {
		t.Fatalf("action panel children = %d, want header, divider, and two action rows", len(rows))
	}
	for index, row := range []woxwidget.Widget{rows[2], rows[3]} {
		content := row.(woxwidget.Container).Child.(woxwidget.Flex)
		if len(content.Children) < 2 {
			t.Fatalf("action row %d children = %d, want leading icon plus label", index, len(content.Children))
		}
		iconSlot := content.Children[0].(woxwidget.Align)
		if iconSlot.Width != 37 || iconSlot.Height != 38 || iconSlot.Vertical != .5 {
			t.Fatalf("action row %d icon align = %#v, want a 37x38 vertically centered slot", index, iconSlot)
		}
		icon, ok := iconSlot.Child.(woxwidget.Container).Child.(woxwidget.Image)
		if !ok || icon.Source == nil || icon.Width != 22 || icon.Height != 22 {
			t.Fatalf("action row %d icon = %#v, want the shared 22px action glyph", index, iconSlot.Child)
		}
		label := content.Children[1].(woxwidget.Align)
		if label.Height != 38 || label.Vertical != .5 {
			t.Fatalf("action row %d label align = %#v, want vertically centered in the row", index, label)
		}
	}
}

func TestWoxLauncherDemoActionPanelFitsActionRowsBelowQuery(t *testing.T) {
	demo := WoxLauncherDemo(LauncherDemoProps{
		Width: 600, Height: 320, Opacity: 1, ShowQuery: true, ShowToolbar: true, ActionProgress: 1,
		Query: "query", Results: []LauncherDemoResult{{Title: "One"}, {Title: "Two"}, {Title: "Three"}},
	}).(woxwidget.Clip)
	children := demo.Child.(woxwidget.Stack).Children
	slot := children[len(children)-2]
	panel := slot.Child.(woxwidget.Container)
	if panel.Height != demoActionPanelHeight() {
		t.Fatalf("action panel height = %v, want content height %v for two actions", panel.Height, demoActionPanelHeight())
	}
	if slot.Top < 65 {
		t.Fatalf("action panel top = %v, want to stay below the query box", slot.Top)
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

	if gap := toolbar.Top - (preview.Top + previewClip.Height); gap != 10 {
		t.Fatalf("preview/toolbar gap = %.0f, want 10px app padding above the toolbar", gap)
	}
}

func TestWoxLauncherDemoHidesPreviewUntilTypedQueryResultsFadeIn(t *testing.T) {
	props := LauncherDemoProps{
		Width: 600, Height: 320, Opacity: 1, ShowQuery: true, ShowToolbar: true, ResultWidth: 350,
		Preview: woxwidget.Container{Width: 234, Height: 201}, FadeResults: true,
		Results: []LauncherDemoResult{{Title: "One"}, {Title: "Two"}, {Title: "Three"}},
	}
	typing := WoxLauncherDemo(props).(woxwidget.Clip)
	if typing.Height != 113 {
		t.Fatalf("typed-query demo height before results = %.0f, want query plus toolbar", typing.Height)
	}
	props.ResultsOpacity = 1
	done := WoxLauncherDemo(props).(woxwidget.Clip)
	children := done.Child.(woxwidget.Stack).Children
	if done.Height != 320 {
		t.Fatalf("typed-query demo height after results = %.0f, want the preview pane", done.Height)
	}
	if _, ok := children[len(children)-3].Child.(woxwidget.Clip); !ok {
		t.Fatal("typed-query demo dropped the preview after results appeared")
	}
}
