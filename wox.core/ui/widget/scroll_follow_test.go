package widget

import (
	"testing"

	woxui "wox/ui/runtime"
)

// TestScrollViewFollowsEndWithoutPullingReadersAway covers both scroll axes and changed geometry.
func TestScrollViewFollowsEndWithoutPullingReadersAway(t *testing.T) {
	for _, horizontal := range []bool{false, true} {
		controller := NewScrollController(0)
		content, viewport := float32(200), float32(40)
		stick := true
		host := NewHost(func(woxui.FrameInfo) Widget {
			width, height := float32(100), viewport
			childWidth, childHeight := float32(100), content
			if horizontal {
				width, height, childWidth, childHeight = viewport, 100, content, 100
			}
			return ScrollView{Key: "follow", Width: width, Height: height, Horizontal: horizontal, StickToEnd: stick, Controller: controller, Child: Container{Width: childWidth, Height: childHeight}}
		})
		host.AttachServices(&fakeHostServices{})
		t.Cleanup(host.Dispose)
		check := func(want float32) {
			t.Helper()
			renderTestFrame(host)
			renderTestFrame(host)
			if got := controller.Offset(); got != want {
				t.Fatalf("horizontal=%v: offset=%v, want %v", horizontal, got, want)
			}
		}
		check(160)
		content = 260
		check(220)
		controller.JumpTo(100)
		check(100)
		content = 300
		check(100)
		controller.JumpTo(controller.MaxOffset())
		check(260)
		content = 340
		check(300)
		viewport = 60
		check(280)
		stick = false
		content = 400
		check(280)
		stick = true
		check(340)
		content = 20
		check(0)
		content = 120
		check(60)
	}
}

// TestScrollViewWithoutFollowKeepsInitialPosition guards the default behavior of existing callers.
func TestScrollViewWithoutFollowKeepsInitialPosition(t *testing.T) {
	controller := NewScrollController(0)
	content := float32(200)
	host := NewHost(func(woxui.FrameInfo) Widget {
		return ScrollView{Key: "ordinary", Width: 100, Height: 40, Controller: controller, Child: Container{Width: 100, Height: content}}
	})
	host.AttachServices(&fakeHostServices{})
	t.Cleanup(host.Dispose)
	renderTestFrame(host)
	content = 300
	renderTestFrame(host)
	if controller.Offset() != 0 {
		t.Fatal("ordinary scroll moved to the end")
	}
}
