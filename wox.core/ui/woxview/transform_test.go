package woxview

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestTransformProtocolPixels checks degree rotation, signed translation, pivot,
// group opacity and unchanged layout through live keyed document updates.
func TestTransformProtocolPixels(t *testing.T) {
	v := NewView()
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	for i, scale := range []float32{1, 1.25, 1.5, 2, 1} {
		factor := 1
		if i == 3 {
			factor = 0
		}
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"stack","props":{"style":{"width":150,"height":100,"backgroundColor":"#00f"}},"children":[
			{"type":"row","key":"box","props":{"label":"Box","onPress":{"$handler":"box"},"style":{"left":30,"top":30,"width":40,"height":20,"margin":{"left":10},"translateX":20,"translateY":-10,"rotate":90,"scale":%d,"opacity":0.5}},"children":[
				{"type":"view","props":{"style":{"width":20,"height":20,"backgroundColor":"#f00"}}},
				{"type":"view","props":{"style":{"width":20,"height":20,"backgroundColor":"#0f0"}}}]},
			{"type":"view","props":{"style":{"left":120,"top":70,"width":20,"height":20,"backgroundColor":"#f0f"}}}
		]}}`, factor)))
		before, _ := json.Marshal(v.Tree().Root)
		d := &ui.DisplayList{}
		host.Frame(d, ui.FrameInfo{Size: ui.Size{Width: 150, Height: 100}, Scale: scale})
		bounds := automationBounds(t, host, "woxview/stack0/k:box/row#press")
		if bounds != (ui.Rect{X: 30, Y: 30, Width: 50, Height: 20}) {
			t.Fatalf("transform changed layout: %+v", bounds)
		}
		d.TransformScene(scale, ui.Point{})
		r, _ := ui.NewSoftwareRenderer(int(150*scale), int(100*scale))
		if err := r.Render(d); err != nil {
			t.Fatal(err)
		}
		pixels := r.RGBA()
		red, green := pixels.RGBAAt(int(80*scale), int(20*scale)), pixels.RGBAAt(int(80*scale), int(40*scale))
		if factor == 0 {
			if red.B != 255 || red.R != 0 || green.B != 255 || green.G != 0 {
				t.Fatal("explicit zero scale did not collapse paint")
			}
		} else if red.R < 126 || red.G != 0 || red.B < 126 || green.G < 126 || green.R != 0 || green.B < 126 {
			t.Fatalf("DPI %g: rotated groups red=%+v green=%+v", scale, red, green)
		}
		if got := pixels.RGBAAt(int(130*scale), int(80*scale)); got.R != 255 || got.B != 255 {
			t.Fatal("transform escaped its subtree")
		}
		tap(host, bounds)
		after, _ := json.Marshal(v.Tree().Root)
		if string(before) != string(after) || len(events) != i+1 {
			t.Fatal("paint mutated document or input routing")
		}
	}
}

// TestTransformOffsetTokensAndDraft keeps numeric offsets logical, resolves token
// offsets per density, and preserves a focused editor when transforms are added/removed.
func TestTransformOffsetTokensAndDraft(t *testing.T) {
	v := NewView()
	theme := component.ControlTheme{}
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, Theme: theme})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	for i, density := range []float32{.9, 1, 1.1, 1} {
		theme.DensityScale = density
		transform := `,"translateX":"$space4","translateY":-4`
		if i == 3 {
			transform = ""
		}
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[{"type":"text-field","key":"input","props":{"label":"Draft","style":{"width":100,"height":40`+transform+`}}},{"type":"view","props":{"style":{"width":10,"height":10,"backgroundColor":"#f00","translateX":"$space4","translateY":-4}}}]}}`))
		scale := []float32{1, 1.5, 2, 1}[i]
		d := &ui.DisplayList{}
		host.Frame(d, ui.FrameInfo{Size: ui.Size{Width: 160, Height: 100}, Scale: scale})
		if i == 0 && !host.FocusAutomationID("woxview/column0/k:input/text-field/field") {
			t.Fatal("missing field")
		}
		host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "x"})
		d.TransformScene(scale, ui.Point{})
		r, _ := ui.NewSoftwareRenderer(int(160*scale), int(100*scale))
		if err := r.Render(d); err != nil {
			t.Fatal(err)
		}
		if got := r.RGBA().RGBAAt(int((theme.Scaled(16)+5)*scale), int(43*scale)); got.R != 255 {
			t.Fatalf("offset token density %g DPI %g: %+v", density, scale, got)
		}
	}
	if v.fieldValues["woxview/column0/k:input/text-field"] != "xxxx" {
		t.Fatalf("transform update reset draft/focus: %+v", v.fieldValues)
	}
}

// TestTransformValidation verifies parser/programmatic contracts and token-preserving serialization.
func TestTransformValidation(t *testing.T) {
	for _, field := range []string{"translateX", "translateY", "scale", "rotate"} {
		for _, raw := range []string{`true`, `[]`, `{}`, `"12px"`, `16385`, `-16385`, `1e100`} {
			if _, err := Parse([]byte(fmt.Sprintf(`{"version":1,"root":{"type":"view","props":{"style":{%q:%s}}}}`, field, raw))); err == nil {
				t.Fatalf("accepted %s=%s", field, raw)
			}
		}
		for _, raw := range []string{`0`, `-2`, `3.5`, `null`} {
			mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"view","props":{"style":{%q:%s}}}}`, field, raw))
		}
	}
	for _, field := range []string{"scale", "rotate"} {
		if _, err := Parse([]byte(fmt.Sprintf(`{"version":1,"root":{"type":"view","props":{"style":{%q:"$space2"}}}}`, field))); err == nil {
			t.Fatal("dimensionless transform accepted token")
		}
	}
	for _, value := range []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
		for _, style := range []Style{{Scale: &value}, {Rotate: value}, {TranslateX: Offset{Value: value}}, {TranslateY: Offset{Value: value}}} {
			if _, err := NewTree(&Node{Type: NodeView, Props: &BoxProps{Styled: Styled{Style: style}}}); err == nil {
				t.Fatal("accepted nonfinite transform")
			}
		}
	}
	for _, raw := range []string{`-10.5`, `0`, `16384`, `-16384`, `"$space1"`, `null`} {
		var value Offset
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var roundtrip Offset
		if err := json.Unmarshal(encoded, &roundtrip); err != nil || roundtrip != value {
			t.Fatalf("offset changed: %s -> %s", raw, encoded)
		}
	}
	for _, value := range []Offset{{Value: 1, Token: SizeSpace1}, {Token: "$bad"}, {Value: -16385}} {
		if _, err := json.Marshal(value); err == nil {
			t.Fatal("serialized invalid offset")
		}
	}
	encoded, _ := json.Marshal(Style{})
	if strings.Contains(string(encoded), "translate") || strings.Contains(string(encoded), "scale") {
		t.Fatalf("defaults serialized explicitly: %s", encoded)
	}
	// Negative offsets must not loosen existing positive layout dimensions.
	if _, err := Parse([]byte(`{"version":1,"root":{"type":"view","props":{"style":{"width":-1}}}}`)); err == nil {
		t.Fatal("negative width accepted")
	}
}
