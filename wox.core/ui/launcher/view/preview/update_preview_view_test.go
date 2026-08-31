package preview

import (
	"strings"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestParseUpdateReleaseNotes(t *testing.T) {
	parsed := parseUpdateReleaseNotes("Intro paragraph\n![shot](https://example.com/shot.png)\n\n- Add\n  - [`Dictation`] Add offline dictation\n    Continued detail\n\n- Fix\n  - Fix updater state")
	if parsed.intro != "Intro paragraph\n![shot](https://example.com/shot.png)" {
		t.Fatalf("intro = %q", parsed.intro)
	}
	if len(parsed.sections) != 2 || parsed.sections[0].title != "Add" || len(parsed.sections[0].items) != 1 {
		t.Fatalf("sections = %#v", parsed.sections)
	}
	item := parsed.sections[0].items[0]
	if item.tag != "Dictation" || item.summary != "Add offline dictation" || item.continuation != "Continued detail" {
		t.Fatalf("item = %#v", item)
	}
}

func TestParseUpdateReleaseNotesStoreGroups(t *testing.T) {
	parsed := parseUpdateReleaseNotes("- Store\n  - Plugin\n    - [need](https://github.com/zzedbot/wox-plugin-need) Store notes [@zzedbot](https://github.com/zzedbot)\n    - [Timestamp](https://gist.github.com/qianlifeng/example) Convert timestamps\n  - Theme\n    - [Wox Dracula](https://github.com/wox-launcher/wox) Dracula colors")
	if len(parsed.sections) != 1 || parsed.sections[0].title != "Store" || len(parsed.sections[0].groups) != 2 {
		t.Fatalf("store section = %#v", parsed.sections)
	}
	pluginGroup := parsed.sections[0].groups[0]
	if pluginGroup.title != "Plugin" || len(pluginGroup.items) != 2 || pluginGroup.items[0].tag != "" || pluginGroup.items[0].summary != "[need](https://github.com/zzedbot/wox-plugin-need) Store notes [@zzedbot](https://github.com/zzedbot)" {
		t.Fatalf("plugin group = %#v", pluginGroup)
	}
	themeGroup := parsed.sections[0].groups[1]
	if themeGroup.title != "Theme" || len(themeGroup.items) != 1 || !strings.HasPrefix(themeGroup.items[0].summary, "[Wox Dracula](") {
		t.Fatalf("theme group = %#v", themeGroup)
	}
}

func TestUpdatePreviewShowsReadableErrorCard(t *testing.T) {
	view := UpdatePreviewView(UpdatePreviewProps{
		Width: 800, Height: 600, AutoUpdateEnabled: true, Title: "Update", StatusLabel: "Latest",
		Error: "failed to download version manifest file:\nstatus code 429", Theme: woxcomponent.Theme{},
	}).(woxwidget.Container)
	header := view.Child.(woxwidget.Flex).Children[0].(woxwidget.Flex)
	errorCard := header.Children[1].(woxwidget.Container)
	errorText := errorCard.Child.(woxwidget.TextBlock)
	if errorCard.Width != 760 || errorCard.Height != 52 || errorText.Width != 740 || errorText.Height != 32 || errorText.MaxLines != 4 {
		t.Fatalf("error card = %#v, text = %#v; want height fitted to two visible lines", errorCard, errorText)
	}
	if errorCard.BorderColor.A == 0 || errorCard.Color.A == 0 {
		t.Fatal("error card should have visible semantic error surface and border")
	}
}

func TestUpdatePreviewCentersEmptyReleaseNotes(t *testing.T) {
	view := UpdatePreviewView(UpdatePreviewProps{
		Width: 800, Height: 600, AutoUpdateEnabled: true, Title: "Update", StatusLabel: "Latest",
		NoReleaseNotes: "No release notes available.", Theme: woxcomponent.Theme{ResultSubtitle: woxui.Color{A: 255}},
	}).(woxwidget.Container)
	scroll := view.Child.(woxwidget.Flex).Children[3].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	empty := scroll.Content.(woxwidget.Align)
	message := empty.Child.(woxwidget.Text)
	if empty.Horizontal != 0.5 || empty.Vertical != 0.42 || message.Value != "No release notes available." || message.Style.Weight != woxui.FontWeightSemibold {
		t.Fatalf("empty release notes = %#v / %#v, want centered catalog-style copy", empty, message)
	}
}

// TestUpdatePreviewUnframedSummaryUsesSharedScroll keeps the intro aligned with the scrollable release notes.
func TestUpdatePreviewUnframedSummaryUsesSharedScroll(t *testing.T) {
	for _, test := range []struct{ scale, scrollHeight float32 }{{1, 509}, {1.5, 763}} {
		scale := test.scale
		props := UpdatePreviewProps{
			ID: "release", Width: 800 * scale, Height: 600 * scale, Scale: scale, AutoUpdateEnabled: true,
			ReleaseNotes: "Summary\n\n- Add\n  - New feature", Theme: woxcomponent.Theme{PreviewText: woxui.Color{A: 255}},
		}
		view := UpdatePreviewView(props).(woxwidget.Container)
		scroll := view.Child.(woxwidget.Flex).Children[3].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
		if scroll.Key != "update-preview-scroll-release" || scroll.HideScrollbar || scroll.ThumbColor != props.Theme.PreviewText || scroll.Width != 760*scale || scroll.Height != test.scrollHeight {
			t.Fatalf("scale %v: scroll key %q, hidden %v, color %v, size %vx%v; want themed shared scroll with the existing viewport", scale, scroll.Key, scroll.HideScrollbar, scroll.ThumbColor, scroll.Width, scroll.Height)
		}
		body := scroll.Content.(woxwidget.Container).Child.(woxwidget.Flex)
		intro, ok := body.Children[0].(woxwidget.TextBlock)
		if !ok || intro.Value != "Summary" || intro.Width != 752*scale {
			t.Fatalf("scale %v: intro = %#v, want full-width Markdown without a card", scale, body.Children[0])
		}
	}
}

func TestUpdatePreviewCentersTitleInHeader(t *testing.T) {
	view := UpdatePreviewView(UpdatePreviewProps{
		Width: 800, Height: 600, AutoUpdateEnabled: true, Title: "发现新版本", StatusLabel: "Latest",
	}).(woxwidget.Container)
	header := view.Child.(woxwidget.Flex).Children[0].(woxwidget.Stack)
	title := header.Children[0].Child.(woxwidget.TextBlock)
	if title.Height != header.Height || title.LineHeight != header.Height || title.MaxLines != 1 || title.AlignmentY != 0.5 {
		t.Fatalf("title = %#v, header height = %.0f; want one optically centered line filling the header", title, header.Height)
	}
}
