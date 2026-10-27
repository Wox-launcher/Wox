package icons

import (
	"image/png"
	"os"
	"strings"
	"testing"

	woxsvg "wox/util/svg"
)

func TestRenderFolderPreview(t *testing.T) {
	source := strings.ReplaceAll(Get(ControlFolderOpen).ImageData, "#fff", "#e8eaed")
	for _, size := range []int{16, 24, 64, 128} {
		img, err := woxsvg.Render(source, size, size)
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.Create("D:/dev/Wox/.tmp-folder-" + itoa(size) + ".png")
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
