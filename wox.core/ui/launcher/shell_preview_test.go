package launcher

import "testing"

func TestShellGridMetricsFollowDisplayScale(t *testing.T) {
	for _, test := range []struct {
		scale, logicalWidth float32
		pixelWidth, columns int
	}{
		{1, 8, 8, 30},
		{1.5, 8, 12, 30},
		{2, 7.5, 15, 32},
	} {
		size, width, height := shellGridMetrics(240, 180, 7.45, 18, test.scale)
		if size.Columns != test.columns || size.Rows != 10 || size.CellWidthPixels != test.pixelWidth || width != test.logicalWidth ||
			height != 18 || size.CellHeightPixels != int(18*test.scale) {
			t.Errorf("scale %v: grid %+v, logical cell %vx%v", test.scale, size, width, height)
		}
	}
}
