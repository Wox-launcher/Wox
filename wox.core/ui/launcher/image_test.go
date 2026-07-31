package launcher

import "testing"

func TestEmbeddedAppIconUsesHighResolutionPNG(t *testing.T) {
	image, err := decodeWoxImageWithTint(appIconImageSource, nil, 256)
	if err != nil {
		t.Fatalf("decode embedded app icon: %v", err)
	}
	if image.Width < 200 || image.Height < 200 {
		t.Fatalf("embedded app icon size = %dx%d, want at least 200x200", image.Width, image.Height)
	}
}

func TestPhysicalImageSizeUsesBackingScale(t *testing.T) {
	tests := []struct {
		name    string
		logical int
		scale   float32
		want    int
	}{
		{name: "one x", logical: 15, scale: 1, want: 15},
		{name: "retina", logical: 15, scale: 2, want: 30},
		{name: "fractional scale", logical: 15, scale: 1.5, want: 23},
		{name: "missing scale", logical: 15, scale: 0, want: 15},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := physicalImageSize(test.logical, test.scale); got != test.want {
				t.Fatalf("physicalImageSize(%d, %v) = %d, want %d", test.logical, test.scale, got, test.want)
			}
		})
	}
}
