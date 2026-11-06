package widget

import (
	"math"
	"testing"

	ui "wox/ui/runtime"
)

// TestAlignSelfOverridesFlex checks both axes, inherited alignment, and weighted wrappers.
func TestAlignSelfOverridesFlex(t *testing.T) {
	for _, axis := range []Axis{Horizontal, Vertical} {
		for _, parent := range []CrossAxisAlignment{CrossAxisStart, CrossAxisCenter, CrossAxisEnd, CrossAxisStretch} {
			for _, own := range []CrossAxisAlignment{CrossAxisStart, CrossAxisCenter, CrossAxisEnd, CrossAxisStretch} {
				for wrapper := 0; wrapper < 4; wrapper++ {
					var child Widget = Gesture{ID: "target", OnTap: func() {}, Child: Container{Width: 20, Height: 20}}
					child = AlignSelf{Alignment: own, Child: child}
					switch wrapper {
					case 1:
						child = Expanded{Child: child}
					case 2:
						child = AlignSelf{Alignment: own, Child: Expanded{Child: Gesture{ID: "target", OnTap: func() {}, Child: Container{Width: 20, Height: 20}}}}
					case 3:
						child = Flexible{Child: child}
					}
					root := (Flex{Axis: axis, CrossAxisAlignment: parent, Children: []Widget{
						Container{Width: 40, Height: 40}, child,
					}}).layout(context{window: &fakeHostServices{}}, constraints{width: 100, height: 100})
					bounds := root.children[1].bounds
					cross, offset := bounds.Height, bounds.Y
					if axis == Vertical {
						cross, offset = bounds.Width, bounds.X
					}
					wantExtent, wantOffset := float32(20), float32(0)
					extent := float32(40)
					if parent == CrossAxisStretch || own == CrossAxisStretch {
						extent = 100
					}
					switch own {
					case CrossAxisStretch:
						wantExtent = 100
					case CrossAxisCenter:
						wantOffset = (extent - 20) / 2
					case CrossAxisEnd:
						wantOffset = extent - 20
					}
					if cross != wantExtent || offset != wantOffset {
						t.Fatalf("axis %v parent %v own %v wrapper %d: %+v, want cross %v offset %v", axis, parent, own, wrapper, bounds, wantExtent, wantOffset)
					}
					point := ui.Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
					if hit := root.hitTest(point); hit == nil || hit.gesture == nil || hit.gesture.id != "target" {
						t.Fatal("aligned item is outside pointer bounds")
					}
				}
			}
		}
	}
}

// TestAlignSelfUnboundedAndNil keeps intrinsic sizing and ignores empty metadata wrappers.
func TestAlignSelfUnboundedAndNil(t *testing.T) {
	root := (Flex{Axis: Vertical, Children: []Widget{
		AlignSelf{}, AlignSelf{Alignment: CrossAxisStretch, Child: Container{Width: 20, Height: 10}},
	}}).layout(context{window: &fakeHostServices{}}, constraints{width: math.MaxFloat32, height: 100})
	if len(root.children) != 1 || root.bounds.Width != 20 || root.bounds.Height != 10 {
		t.Fatalf("unbounded intrinsic size = %+v with %d children", root.bounds, len(root.children))
	}
}
