package widget

import (
	"fmt"
	"image"
	"runtime"
	"testing"
	"weak"

	woxui "wox/ui/runtime"
)

type identityRetentionProps struct {
	Rows   int
	Images []*woxui.Image
}

func (p identityRetentionProps) Equal(other identityRetentionProps) bool {
	return p.Rows == other.Rows
}

// TestBoundaryRebuildReleasesStaleIdentityNodes guards against a Boundary keeping
// nodes from a previous, larger build alive through the spare capacity of its
// identity bookkeeping slices. Those nodes carry paint closures that hold decoded
// images, so a launcher that shows many rows once and then only a few must not
// pin the old rows' images while the Host stays alive.
func TestBoundaryRebuildReleasesStaleIdentityNodes(t *testing.T) {
	const largeRows = 40
	const smallRows = 4
	images := make([]*woxui.Image, largeRows)
	for index := range images {
		img, err := woxui.NewImage(image.NewRGBA(image.Rect(0, 0, 2, 2)))
		if err != nil {
			t.Fatal(err)
		}
		images[index] = img
	}
	removedImage := weak.Make(images[largeRows-1])
	keptImage := weak.Make(images[0])

	props := identityRetentionProps{Rows: largeRows, Images: images}
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Boundary[identityRetentionProps]{Key: "list", Props: props, Build: func(p identityRetentionProps) Widget {
			children := make([]Widget, 0, p.Rows)
			for index := 0; index < p.Rows; index++ {
				children = append(children, Semantics{
					Key:          Key(fmt.Sprintf("row-%d", index)),
					AutomationID: fmt.Sprintf("row-%d", index),
					Role:         woxui.AccessibilityRoleButton,
					Label:        fmt.Sprintf("row %d", index),
					Child:        Image{Source: p.Images[index], Width: 10, Height: 10},
				})
			}
			return Flex{Axis: Vertical, Children: children}
		}}
	})
	host.AttachServices(&fakeHostServices{})
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 100, Height: 1000}}
	host.Frame(&woxui.DisplayList{}, frame)

	props = identityRetentionProps{Rows: smallRows, Images: append([]*woxui.Image(nil), images[:smallRows]...)}
	images = nil
	for range 3 {
		host.Frame(&woxui.DisplayList{}, frame)
	}

	runtime.GC()
	runtime.GC()
	removedAlive := removedImage.Value() != nil
	keptAlive := keptImage.Value() != nil
	// The Host must stay reachable here; otherwise the GC could free the whole
	// tree and hide the retention this test exists to catch.
	runtime.KeepAlive(host)
	if !keptAlive {
		t.Fatalf("image of a still-visible row was released")
	}
	if removedAlive {
		t.Fatalf("image of a row removed by a Boundary rebuild is still reachable from the host")
	}
}
