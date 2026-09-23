package widget

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestViewportSlotLoadsOnlyTheVisibleImage(t *testing.T) {
	loads := map[string]int{}
	slot := func(id string) ViewportSlot {
		return ViewportSlot{
			Key: Key(id), Overscan: -1,
			Build: func(visible bool, _ float32) (Widget, float32) {
				if visible {
					loads[id]++
				}
				return Painter{Width: 80, Height: 100}, 0
			},
		}
	}
	host := NewHost(func(woxui.FrameInfo) Widget {
		return ScrollView{
			Width: 80, Height: 100,
			Child: Flex{Axis: Vertical, Children: []Widget{slot("top"), slot("bottom")}},
		}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)
	renderTestFrame(host)
	if loads["top"] == 0 || loads["bottom"] != 0 {
		t.Fatalf("loads = %#v, want only the visible top image", loads)
	}
}

func TestViewportSlotWithoutScrollLoadsImmediately(t *testing.T) {
	loads := 0
	host := NewHost(func(woxui.FrameInfo) Widget {
		return ViewportSlot{
			Key: "only",
			Build: func(visible bool, _ float32) (Widget, float32) {
				if visible {
					loads++
				}
				return Painter{Width: 10, Height: 10}, 0
			},
		}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)
	if loads == 0 {
		t.Fatal("image outside a scroll view was not loaded")
	}
}
