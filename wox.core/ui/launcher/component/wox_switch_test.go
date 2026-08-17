package component

import (
	"testing"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxSwitchUsesIntegerGeometry(t *testing.T) {
	animation := WoxSwitch(SwitchProps{}).(woxwidget.AnimatedFloat)
	off := animation.Builder(0).(woxwidget.Stack)
	on := animation.Builder(1).(woxwidget.Stack)
	offThumb := off.Children[1].Child.(woxwidget.Container)
	onThumb := on.Children[1].Child.(woxwidget.Container)

	if off.Width != 36 || off.Height != 24 {
		t.Fatalf("switch size = %vx%v, want 36x24", off.Width, off.Height)
	}
	if track := off.Children[0].Child.(woxwidget.Container); track.Width != 32 || track.Height != 20 || track.Radius != 10 {
		t.Fatalf("switch track = %vx%v r%v, want 32x20 r10", track.Width, track.Height, track.Radius)
	}
	if offThumb.Width != 10 || onThumb.Width != 14 {
		t.Fatalf("switch thumb sizes = %v/%v, want 10/14", offThumb.Width, onThumb.Width)
	}
}

func TestWoxSwitchAnimatesThumbSizeOnHover(t *testing.T) {
	theme := Theme{
		ResultTitle:        woxui.Color{R: 80, G: 90, B: 100, A: 255},
		ActionSelected:     woxui.Color{R: 20, G: 80, B: 160, A: 255},
		ActionSelectedText: woxui.Color{R: 255, G: 255, B: 255, A: 255},
	}
	switchControl := WoxSwitch(SwitchProps{ID: "enabled", Label: "Enabled", Value: true, OnChange: func(bool) {}, Theme: theme}).(woxwidget.Semantics)
	stateful := switchControl.Child.(woxwidget.Focusable).Child
	normalAnimation := buildHoverable(stateful, false).(woxwidget.Gesture).Child.(woxwidget.AnimatedFloat)
	hoverAnimation := buildHoverable(stateful, true).(woxwidget.Gesture).Child.(woxwidget.AnimatedFloat)
	normal := normalAnimation.Builder(0).(woxwidget.AnimatedFloat).Builder(1).(woxwidget.Stack)
	hovered := hoverAnimation.Builder(1).(woxwidget.AnimatedFloat).Builder(1).(woxwidget.Stack)

	if hoverAnimation.Target != 1 || hoverAnimation.Duration != 120*time.Millisecond {
		t.Fatalf("switch hover animation = target %v duration %v", hoverAnimation.Target, hoverAnimation.Duration)
	}
	if normal.Children[0].Child.(woxwidget.Container).Color == hovered.Children[0].Child.(woxwidget.Container).Color {
		t.Fatal("switch track did not change color on hover")
	}
	normalThumb := normal.Children[1].Child.(woxwidget.Container)
	hoveredThumb := hovered.Children[1].Child.(woxwidget.Container)
	if hoveredThumb.Width-normalThumb.Width != 2 {
		t.Fatalf("switch hover thumb growth = %v, want 2", hoveredThumb.Width-normalThumb.Width)
	}
}
