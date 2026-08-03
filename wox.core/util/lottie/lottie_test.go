package lottie

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestRenderEmbeddedPNGAnimation(t *testing.T) {
	asset := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for index := 0; index < len(asset.Pix); index += 4 {
		asset.Pix[index] = 220
		asset.Pix[index+3] = 255
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, asset); err != nil {
		t.Fatal(err)
	}
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes())
	data := fmt.Sprintf(`{"v":"5.7.4","fr":30,"ip":0,"op":30,"w":32,"h":32,"assets":[{"id":"asset","w":4,"h":4,"u":"","p":%q,"e":1}],"layers":[{"ty":2,"refId":"asset","ks":{"a":{"a":0,"k":[2,2,0]},"p":{"a":1,"k":[{"t":0,"s":[8,16,0]},{"t":29,"s":[24,16,0]}]},"s":{"a":0,"k":[200,200,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}},"ip":0,"op":30,"st":0}]}`, dataURI)

	animation, err := New(data, 32, 32)
	if err != nil {
		t.Fatal(err)
	}
	defer animation.Close()
	first, err := animation.Render(0)
	if err != nil {
		t.Fatal(err)
	}
	last, err := animation.Render(0.95)
	if err != nil {
		t.Fatal(err)
	}
	if alphaAt(first, 8, 16) == 0 || alphaAt(last, 24, 16) == 0 {
		t.Fatal("embedded PNG did not render at the animated positions")
	}
	if bytes.Equal(first.Pix, last.Pix) {
		t.Fatal("animation frames are identical")
	}
}

func alphaAt(img *image.RGBA, x, y int) uint32 {
	_, _, _, alpha := color.RGBAModel.Convert(img.At(x, y)).RGBA()
	return alpha
}
