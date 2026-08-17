package preview

import (
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
	scroll := view.Child.(woxwidget.Flex).Children[3].(woxwidget.ScrollView)
	empty := scroll.Child.(woxwidget.Align)
	message := empty.Child.(woxwidget.Text)
	if empty.Horizontal != 0.5 || empty.Vertical != 0.42 || message.Value != "No release notes available." || message.Style.Weight != woxui.FontWeightSemibold {
		t.Fatalf("empty release notes = %#v / %#v, want centered catalog-style copy", empty, message)
	}
}
