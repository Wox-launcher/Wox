package preview

import (
	"testing"
	"time"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestMediaPreviewMatchesFlutterResponsiveLayout(t *testing.T) {
	props := MediaPreviewProps{
		Width: 900, Height: 500, Title: "Track", Artist: "Artist", Album: "Album", AppName: "Player", Playing: true,
		Theme: woxcomponent.Theme{PreviewText: woxui.Color{R: 240, G: 240, B: 240, A: 255}, PreviewPropertyContent: woxui.Color{R: 180, G: 180, B: 180, A: 255}},
	}
	wide := MediaPreviewView(props).(woxwidget.Container)
	if wide.Color.A != 0 || wide.Padding != (woxwidget.Insets{Left: 18, Top: 16, Right: 16, Bottom: 14}) {
		t.Fatalf("wide outer surface = fill %#v padding %#v; want transparent Flutter spacing", wide.Color, wide.Padding)
	}
	wideContent := wide.Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if wideContent.Axis != woxwidget.Horizontal || wideContent.Gap != 38 || len(wideContent.Children) != 2 {
		t.Fatalf("wide layout = axis %v gap %.0f children %d; want Flutter record/details row", wideContent.Axis, wideContent.Gap, len(wideContent.Children))
	}

	props.Width = 620
	props.Height = 520
	compact := MediaPreviewView(props).(woxwidget.Container).Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if compact.Axis != woxwidget.Vertical || compact.Gap != 14 || len(compact.Children) != 2 {
		t.Fatalf("compact layout = axis %v gap %.0f children %d; want Flutter record/details column", compact.Axis, compact.Gap, len(compact.Children))
	}
}

func TestMediaPreviewUsesFlutterControlSurface(t *testing.T) {
	theme := woxcomponent.Theme{PreviewText: woxui.Color{R: 220, G: 225, B: 230, A: 255}, Cursor: woxui.Color{R: 255, G: 100, B: 50, A: 255}}
	controls := mediaControls(MediaPreviewProps{Playing: true, Theme: theme}).(woxwidget.Container)
	if controls.Width != 176 || controls.Height != 48 || controls.Radius != 24 || controls.Color.A != 9 || controls.BorderColor.A != 26 {
		t.Fatalf("controls = %vx%v radius %.0f fill %d border %d; want Flutter pill", controls.Width, controls.Height, controls.Radius, controls.Color.A, controls.BorderColor.A)
	}
	row := controls.Child.(woxwidget.Flex)
	if row.Gap != 28 {
		t.Fatalf("control gap = %.0f, want 28 to center all three controls", row.Gap)
	}
	buttons := row.Children
	if len(buttons) != 3 {
		t.Fatalf("control count = %d, want previous/toggle/next", len(buttons))
	}
	toggle := buttons[1].(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if toggle.ID != "media-preview-media-control-pause" || toggle.Width != 36 || toggle.Background.A != 33 || toggle.Icon == nil {
		t.Fatalf("toggle = %+v; want emphasized Flutter pause control", toggle)
	}
}

func TestMediaRecordAnimationFollowsPlayback(t *testing.T) {
	playing := mediaRecordStage(300, MediaPreviewProps{Playing: true}).(woxwidget.Stack)
	record := playing.Children[0].Child.(woxwidget.LoopAnimation)
	tonearm := playing.Children[1].Child.(woxwidget.AnimatedFloat)
	if record.Paused || record.Duration != 12*time.Second || tonearm.Target != 1 || tonearm.Duration != 480*time.Millisecond {
		t.Fatalf("playing animation = paused %v duration %v tonearm %.0f/%v; want rotating record and lowered tonearm", record.Paused, record.Duration, tonearm.Target, tonearm.Duration)
	}
	paused := mediaRecordStage(300, MediaPreviewProps{}).(woxwidget.Stack)
	if !paused.Children[0].Child.(woxwidget.LoopAnimation).Paused || paused.Children[1].Child.(woxwidget.AnimatedFloat).Target != 0 {
		t.Fatal("paused media did not freeze the record and park the tonearm")
	}
}

func TestFormatMediaDurationIncludesHours(t *testing.T) {
	if got := formatMediaDuration(3723); got != "1:02:03" {
		t.Fatalf("duration = %q, want 1:02:03", got)
	}
}
