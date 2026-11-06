package woxview

import (
	"encoding/json"
	"fmt"
	"testing"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestDualColorRebuildKeepsDocumentState switches appearance on an unchanged
// tree, including transparent colors, without replacing user-edited fields.
func TestDualColorRebuildKeepsDocumentState(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","props":{"style":{"backgroundColor":{"light":"#12345680","dark":"$surface"}}},"children":[{"type":"text-field","key":"input","props":{"value":"","style":{"width":100}}}]}}`))
	theme := component.ControlTheme{Surface: ui.Color{R: 12, G: 90, B: 200, A: 180}}
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, Theme: theme})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	for i, dark := range []bool{false, true, false, true} {
		theme.Dark = dark
		if i == 3 {
			theme.Surface = ui.Color{}
		}
		scale := []float32{1, 1.5, 2, 1}[i]
		d := &ui.DisplayList{}
		host.Frame(d, ui.FrameInfo{Size: ui.Size{Width: 150, Height: 100}, PixelSize: ui.PixelSize{Width: int(150 * scale), Height: int(100 * scale)}, Scale: scale})
		if i == 0 {
			host.FocusAutomationID("woxview/column0/k:input/text-field/field")
			host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "edited"})
		} else if v.fieldValues["woxview/column0/k:input/text-field"] != "edited" {
			t.Fatal("theme change reset the field")
		}
		d.TransformScene(scale, ui.Point{})
		r, _ := ui.NewSoftwareRenderer(int(150*scale), int(100*scale))
		if err := r.Render(d); err != nil {
			t.Fatal(err)
		}
		got := r.RGBA().RGBAAt(int(120*scale), int(80*scale))
		want := ui.Color{R: 0x12, G: 0x34, B: 0x56, A: 0x80}
		if dark {
			want = theme.Surface
		}
		if got.A != want.A || got.R != uint8(uint32(want.R)*uint32(want.A)/255) || got.G != uint8(uint32(want.G)*uint32(want.A)/255) || got.B != uint8(uint32(want.B)*uint32(want.A)/255) {
			t.Fatalf("dark=%v scale=%v: %+v, want %+v", dark, scale, got, want)
		}
	}
}

// TestSemanticColorTokens verifies all new roles through actual protocol rendering.
func TestSemanticColorTokens(t *testing.T) {
	theme := component.ControlTheme{Hover: ui.Color{R: 1, A: 255}, SelectionBackground: ui.Color{R: 2, A: 255}, SelectionText: ui.Color{R: 3, A: 255}, Focus: ui.Color{R: 4, A: 255}, Disabled: ui.Color{R: 5, A: 255}, Link: ui.Color{R: 6, A: 255}, Overlay: ui.Color{R: 7, A: 255}, WarningText: ui.Color{R: 8, A: 255}}
	for i, token := range []string{"$hover", "$selected", "$selectedText", "$focus", "$disabled", "$link", "$overlay", "$warningText"} {
		v := NewView()
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"view","props":{"style":{"width":30,"height":20,"backgroundColor":%q}}}}`, token)))
		host := widget.NewHost(func(ui.FrameInfo) widget.Widget { return v.Build(BuildProps{Width: 30, Height: 20, Theme: theme}) })
		host.AttachServices(fakeServices{})
		d := &ui.DisplayList{}
		host.Frame(d, ui.FrameInfo{Size: ui.Size{Width: 30, Height: 20}, Scale: 1})
		r, _ := ui.NewSoftwareRenderer(30, 20)
		if err := r.Render(d); err != nil {
			t.Fatal(err)
		}
		if got := r.RGBA().RGBAAt(10, 10); got.R != uint8(i+1) || got.A != 255 {
			t.Fatalf("%s: %+v", token, got)
		}
		host.Dispose()
	}
}

// TestColorContracts keeps scalar compatibility and validates hidden branches.
func TestColorContracts(t *testing.T) {
	for _, input := range []string{`""`, `"#abc"`, `"#11223380"`, `"$selected"`, `{"light":"#abc","dark":"$text"}`, `null`} {
		var c Color
		if err := json.Unmarshal([]byte(input), &c); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		var roundtrip Color
		if err := json.Unmarshal(data, &roundtrip); err != nil || c != roundtrip {
			t.Fatalf("roundtrip %s: %+v %v", input, roundtrip, err)
		}
	}
	for _, value := range []string{`{}`, `{"light":"#fff"}`, `{"light":"","dark":"#fff"}`, `{"light":null,"dark":"#fff"}`, `{"light":"#fff","dark":"bad"}`, `{"light":"$unknown","dark":"#000"}`, `{"light":"#fff","dark":{"light":"#000","dark":"#fff"}}`, `{"light":"#fff","dark":"#000","extra":true}`, `true`, `3`, `[]`} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"view","props":{"style":{"color":` + value + `}}}}`)); err == nil {
			t.Fatalf("accepted %s", value)
		}
	}
	for _, c := range []Color{{Value: "bad"}, {Light: "#fff"}, {Value: "#abc", Light: "#fff", Dark: "#000"}} {
		if _, err := NewTree(&Node{Type: NodeView, Props: &BoxProps{Styled: Styled{Style: Style{Color: c}}}}); err == nil {
			t.Fatalf("accepted programmatic %+v", c)
		}
	}
}
