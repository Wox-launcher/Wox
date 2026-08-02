package component

import (
	"math"
	"testing"

	woxwidget "wox/ui/widget"
)

func TestWoxSwitchMatchesFlutterFittedGeometry(t *testing.T) {
	animation := WoxSwitch(SwitchProps{}).(woxwidget.AnimatedFloat)
	off := animation.Builder(0).(woxwidget.Stack)
	on := animation.Builder(1).(woxwidget.Stack)
	offThumb := off.Children[1].Child.(woxwidget.Container)
	onThumb := on.Children[1].Child.(woxwidget.Container)

	if off.Width != 36 || off.Height != 24 {
		t.Fatalf("switch size = %vx%v, want 36x24", off.Width, off.Height)
	}
	if track := off.Children[0].Child.(woxwidget.Container); track.Width != 31.2 || track.Height != 19.2 {
		t.Fatalf("switch track = %vx%v, want 31.2x19.2", track.Width, track.Height)
	}
	if math.Abs(float64(offThumb.Width-9.6)) > 0.001 || math.Abs(float64(onThumb.Width-14.4)) > 0.001 {
		t.Fatalf("switch thumb sizes = %v/%v, want 9.6/14.4", offThumb.Width, onThumb.Width)
	}
}
