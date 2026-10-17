package brand

import (
	"strings"
	"testing"

	"wox/util/svg"
)

func TestImageUsesTheRaycastMark(t *testing.T) {
	data := Image().ImageData
	if !strings.Contains(data, `fill="#FF6363"`) || strings.Contains(data, "M8.5 23.2") {
		t.Fatalf("icon = %s", data)
	}
	rendered, err := svg.Render(data, 64, 64)
	if err != nil {
		t.Fatal(err)
	}
	var coral int
	bounds := rendered.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := rendered.RGBAAt(x, y)
			if pixel.A > 200 && pixel.R > 200 && pixel.G > 60 && pixel.G < 140 && pixel.B < 140 {
				coral++
			}
		}
	}
	if coral < 200 {
		t.Fatalf("coral pixels = %d", coral)
	}
}
