package woxview

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"
)

// TestSpritePlaybackRetainsAtlas checks native advancement and ordinary tree echoes.
func TestSpritePlaybackRetainsAtlas(t *testing.T) {
	pixels := image.NewRGBA(image.Rect(0, 0, 32, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 32; x++ {
			c := color.RGBA{R: 255, A: 255}
			if x >= 16 {
				c = color.RGBA{G: 255, A: 255}
			}
			pixels.SetRGBA(x, y, c)
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, pixels); err != nil {
		t.Fatal(err)
	}
	v := NewView()
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	now := time.Unix(1, 0)
	host.SetAnimationClock(func() time.Time { return now })
	set := func(playback string, paused bool) {
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"stack","children":[{"type":"sprite","key":"mole","props":{"source":{"imageType":"base64","imageData":"data:image/png;base64,%s"},"columns":2,"rows":1,"clip":"walk","clips":[{"name":"walk","frames":[0,1],"fps":4,"loop":true}],"playbackId":%q,"paused":%t,"style":{"width":32,"height":32}}}]}}`, base64.StdEncoding.EncodeToString(data.Bytes()), playback, paused)))
	}
	set("one", false)
	if c := statePixel(t, host, 1, 10, 10); c.R != 255 {
		t.Fatalf("initial frame %v", c)
	}
	if len(v.images.entries) != 1 {
		t.Fatal("atlas not cached")
	}
	var identity uint64
	for _, entry := range v.images.entries {
		identity = entry.image.ID()
	}
	now = now.Add(300 * time.Millisecond)
	if c := statePixel(t, host, 2.5, 10, 10); c.G != 255 {
		t.Fatalf("no native frame advancement %v", c)
	}
	set("one", false)
	if c := statePixel(t, host, 1.25, 10, 10); c.G != 255 {
		t.Fatal("producer echo restarted sprite")
	}
	set("one", true)
	statePixel(t, host, 1, 10, 10)
	now = now.Add(800 * time.Millisecond)
	if c := statePixel(t, host, 1, 10, 10); c.G != 255 {
		t.Fatal("paused sprite advanced")
	}
	set("two", false)
	if c := statePixel(t, host, 1, 10, 10); c.R != 255 {
		t.Fatal("new playback did not restart")
	}
	for _, entry := range v.images.entries {
		if entry.image.ID() != identity {
			t.Fatal("frame update recreated atlas")
		}
	}
}

// TestNativeTimelinePath verifies motion without producer updates and exact terminal pose.
func TestNativeTimelinePath(t *testing.T) {
	v := NewView()
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	now := time.Unix(1, 0)
	host.SetAnimationClock(func() time.Time { return now })
	tree := `{"version":1,"root":{"type":"stack","children":[{"type":"view","key":"walker","props":{"style":{"width":10,"height":10,"backgroundColor":"#f00"},"timeline":{"duration":1000,"keyframes":[{"at":0,"translateX":0},{"at":500,"translateX":50},{"at":1000,"translateX":50,"translateY":50}]}}}]}}`
	v.SetTree(mustParse(t, tree))
	statePixel(t, host, 1, 5, 5)
	now = now.Add(250 * time.Millisecond)
	if c := statePixel(t, host, 2.5, 30, 5); c.R != 255 {
		t.Fatal("first path segment did not advance")
	}
	v.SetTree(mustParse(t, tree))
	if c := statePixel(t, host, 1, 30, 5); c.R != 255 {
		t.Fatal("tree echo restarted path")
	}
	now = now.Add(500 * time.Millisecond)
	if c := statePixel(t, host, 1.5, 55, 30); c.R != 255 {
		t.Fatal("second path segment did not advance")
	}
	now = now.Add(time.Hour)
	if c := statePixel(t, host, 1, 55, 55); c.R != 255 {
		t.Fatal("finite path did not hold final pose")
	}
}

// TestPlaybackValidation bounds untrusted frame indices, clock offsets and keyframe channels.
func TestPlaybackValidation(t *testing.T) {
	for _, props := range []string{
		`{"duration":0,"keyframes":[{"at":0},{"at":100}]}`,
		`{"duration":100,"keyframes":[{"at":0},{"at":0},{"at":100}]}`,
		`{"duration":100,"keyframes":[{"at":0},{"at":99}]}`,
		`{"duration":100,"keyframes":[{"at":0},{"at":100,"opacity":2}]}`,
		`{"duration":100,"keyframes":[{"at":0},{"at":100,"translateX":20000}]}`,
		`{"duration":100,"elapsed":-1,"keyframes":[{"at":0},{"at":100}]}`,
	} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"view","props":{"timeline":` + props + `}}}`)); err == nil {
			t.Errorf("accepted invalid timeline: %s", props)
		}
	}
	for _, fields := range []string{
		`"columns":0,"rows":1`,
		`"columns":2,"rows":1,"frame":2`,
		`"columns":2,"rows":1,"clip":"missing"`,
		`"columns":2,"rows":1,"clips":[{"name":"a","frames":[2],"fps":5}]`,
		`"columns":2,"rows":1,"clips":[{"name":"a","frames":[0],"fps":0}]`,
	} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"sprite","props":{"source":{"imageType":"absolute","imageData":"/atlas.png"},` + fields + `}}}`)); err == nil {
			t.Errorf("accepted invalid sprite: %s", fields)
		}
	}
}
