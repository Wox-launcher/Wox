package woxview

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestSizeJSONContract preserves numeric compatibility and token identity through serialization.
func TestSizeJSONContract(t *testing.T) {
	inputs := []string{`0`, `12.5`, `16384`, `null`}
	for _, token := range (SizeToken("")).enumValues() {
		inputs = append(inputs, fmt.Sprintf("%q", token))
	}
	for _, input := range inputs {
		var size Size
		if err := json.Unmarshal([]byte(input), &size); err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		encoded, err := json.Marshal(size)
		if err != nil {
			t.Fatal(err)
		}
		var roundtrip Size
		if err := json.Unmarshal(encoded, &roundtrip); err != nil || roundtrip != size {
			t.Fatalf("roundtrip %s: %s, %v", input, encoded, err)
		}
		if size.Token != "" && string(encoded) != input {
			t.Fatalf("lost token identity: %s -> %s", input, encoded)
		}
	}
	for _, input := range []string{`-1`, `16385`, `1e100`, `""`, `"12"`, `"12px"`, `"$unknown"`, `true`, `[]`, `{}`} {
		var size Size
		if err := json.Unmarshal([]byte(input), &size); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
	for _, size := range []Size{{Value: -1}, {Value: 16385}, {Value: float32(math.NaN())}, {Value: float32(math.Inf(1))}, {Token: "$missing"}, {Value: 1, Token: SizeSpace1}} {
		if _, err := json.Marshal(size); err == nil {
			t.Fatalf("serialized invalid size %+v", size)
		}
	}
	var edges Edges
	if err := json.Unmarshal([]byte(`{"horizontal":"$space2","vertical":"$space3","left":0,"top":"$space1"}`), &edges); err != nil {
		t.Fatal(err)
	}
	want := Edges{Top: Size{Token: SizeSpace1}, Right: Size{Token: SizeSpace2}, Bottom: Size{Token: SizeSpace3}}
	if edges != want {
		t.Fatalf("explicit zero did not override shorthand: %+v", edges)
	}
	encoded, err := json.Marshal(edges)
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip Edges
	if err := json.Unmarshal(encoded, &roundtrip); err != nil || roundtrip != want {
		t.Fatalf("edge roundtrip: %s %v", encoded, err)
	}
	encoded, err = json.Marshal(Style{})
	if err != nil || strings.Contains(string(encoded), "width") || strings.Contains(string(encoded), "padding") {
		t.Fatalf("zero dimensions no longer omitted: %s %v", encoded, err)
	}
}

// TestSizeValidationEveryDimension exercises public parsing and the programmatic validation path.
func TestSizeValidationEveryDimension(t *testing.T) {
	styleType := reflect.TypeFor[Style]()
	for i := 0; i < styleType.NumField(); i++ {
		field := styleType.Field(i)
		if field.Type != reflect.TypeFor[Size]() && field.Type != reflect.TypeFor[*Size]() {
			continue
		}
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		for _, input := range []string{`"$space2"`, `8`, `null`} {
			mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"view","props":{"style":{%q:%s}}}}`, name, input))
		}
		for _, input := range []string{`"$bad"`, `-1`, `true`} {
			if _, err := Parse([]byte(fmt.Sprintf(`{"version":1,"root":{"type":"view","props":{"style":{%q:%s}}}}`, name, input))); err == nil {
				t.Fatalf("accepted %s=%s", name, input)
			}
		}
	}
	for _, field := range []string{"flex", "opacity", "aspectRatio", "maxLines"} {
		if _, err := Parse([]byte(fmt.Sprintf(`{"version":1,"root":{"type":"view","props":{"style":{%q:"$space2"}}}}`, field))); err == nil {
			t.Fatalf("accepted dimension token for %s", field)
		}
	}
	for _, size := range []Size{{Value: -1}, {Value: float32(math.NaN())}, {Token: "$bad"}} {
		// A valid later anchor must not mask an invalid earlier anchor.
		style := Style{Top: &size, Bottom: &Size{Value: 10}}
		if _, err := NewTree(&Node{Type: NodeView, Props: &BoxProps{Styled: Styled{Style: style}}}); err == nil || !strings.Contains(err.Error(), "top") {
			t.Fatalf("invalid top accepted or misidentified: %v", err)
		}
	}
	for _, node := range []string{
		`{"type":"grid","props":{"columns":2,"gap":"$bad"}}`,
		`{"type":"spacer","props":{"size":-1}}`,
		`{"type":"scroll","props":{"endThreshold":"8px"}}`,
		`{"type":"lazy-list","props":{"itemHeight":0}}`,
		`{"type":"lazy-list","props":{"itemHeight":null}}`,
		`{"type":"table","props":{"columns":[{"key":"c","label":"C","width":true}],"rows":[]}}`,
		`{"type":"view","props":{"style":{"padding":{"left":"$bad"}}}}`,
	} {
		if _, err := Parse([]byte(`{"version":1,"root":` + node + `}`)); err == nil {
			t.Fatalf("accepted %s", node)
		}
	}
}

// TestSizeTokensLiveLayout checks actual boxes and input across density and display transitions.
func TestSizeTokensLiveLayout(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"row","props":{"style":{"padding":{"horizontal":"$space2","top":"$space1"},"gap":"$space1"}},"children":[
		{"type":"view","key":"token","props":{"label":"Target","onPress":{"$handler":"hit"},"style":{"width":"$space4","height":"$space3","margin":{"left":"$space1"}}}},
		{"type":"spacer","props":{"size":"$space2"}},
		{"type":"view","key":"literal","props":{"label":"Target","onPress":{"$handler":"hit"},"style":{"width":16,"height":12}}}
	]}}`))
	before, _ := json.Marshal(v.Tree().Root)
	theme := component.ControlTheme{}
	hits := 0
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, Theme: theme, OnEvent: func(HandlerID, json.RawMessage) { hits++ }})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	for i, density := range []float32{0.9, 1, 1.1, 0.9} {
		theme.DensityScale = density
		scale := []float32{1, 1.5, 2, 1}[i]
		renderLayout(host, 250, 150, scale)
		space1, space2, space3, space4 := theme.Scaled(4), theme.Scaled(8), theme.Scaled(12), theme.Scaled(16)
		token := automationBounds(t, host, "woxview/row0/k:token/view#press")
		literal := automationBounds(t, host, "woxview/row0/k:literal/view#press")
		// Pressable bounds include the authored outer margin.
		if token.X != space2 || token.Y != space1 || token.Width != space4+space1 || token.Height != space3 || literal.X != space2+space1+space4+2*space1+space2 || literal.Width != 16 || literal.Height != 12 {
			t.Fatalf("density %v DPI %v: token %+v literal %+v", density, scale, token, literal)
		}
		tap(host, literal)
	}
	after, _ := json.Marshal(v.Tree().Root)
	if string(before) != string(after) || hits != 4 {
		t.Fatalf("density change mutated tree or lost input: hits=%d", hits)
	}
}

// TestSizeGridTableAndAnchors covers dimensional props outside the common Style path.
func TestSizeGridTableAndAnchors(t *testing.T) {
	for _, density := range []float32{0.9, 1, 1.1} {
		theme := component.ControlTheme{DensityScale: density}
		for _, tc := range []struct {
			document, target string
			want             ui.Rect
		}{
			{`{"type":"grid","props":{"columns":2,"gap":"$space3","style":{"width":100}},"children":[{"type":"view","props":{"style":{"height":10}}},{"type":"view","key":"cell","props":{"label":"Target","onPress":{"$handler":"hit"},"style":{"height":10,"width":100}}}]}`, "woxview/column0/grid0/k:cell/view#press", ui.Rect{X: (100 + theme.Scaled(12)) / 2, Width: (100 - theme.Scaled(12)) / 2, Height: 10}},
			{`{"type":"table","props":{"columns":[{"key":"a","label":"","width":"$fontTitle"},{"key":"b","label":"","width":40}],"rows":[]}}`, "woxview/column0/table0/header/Yg", ui.Rect{X: theme.Scaled(22), Width: 40, Height: theme.Scaled(40)}},
			{`{"type":"stack","props":{"style":{"width":100,"height":80}},"children":[{"type":"view","key":"box","props":{"label":"Target","onPress":{"$handler":"hit"},"style":{"left":"$space2","right":"$space3","top":"$space1","bottom":"$space4"}}}]}`, "woxview/column0/stack0/k:box/view#press", ui.Rect{X: theme.Scaled(8), Y: theme.Scaled(4), Width: 100 - theme.Scaled(8) - theme.Scaled(12), Height: 80 - theme.Scaled(4) - theme.Scaled(16)}},
		} {
			v := NewView()
			v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[`+tc.document+`]}}`))
			host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
				return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, Theme: theme, OnEvent: func(HandlerID, json.RawMessage) {}})
			})
			host.AttachServices(fakeServices{})
			renderLayout(host, 250, 150, 1.5)
			got := automationBounds(t, host, tc.target)
			host.Dispose()
			if got != tc.want {
				t.Fatalf("%s density %v: %+v want %+v", tc.target, density, got, tc.want)
			}
		}
	}
}

// TestSizeLazyCommandAfterDensityChange must resolve the virtual row at execution, not enqueue time.
func TestSizeLazyCommandAfterDensityChange(t *testing.T) {
	var rows []string
	for i := 0; i < 100; i++ {
		rows = append(rows, fmt.Sprintf(`{"type":"text-field","key":"%d","props":{"value":"","style":{"height":"$fontTitle"}}}`, i))
	}
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"lazy-list","key":"rows","props":{"itemHeight":"$fontTitle","style":{"width":200,"height":100}},"children":[`+strings.Join(rows, ",")+`]}}`))
	theme := component.ControlTheme{DensityScale: 0.9}
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, Theme: theme})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	renderLayout(host, 250, 150, 1)
	first := "woxview/k:rows/lazy-list/k:0/text-field"
	if !host.FocusAutomationID(first + "/field") {
		t.Fatal("missing first row")
	}
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "kept"})
	if err := v.EnqueueCommand(host, CommandFocus, "99", nil); err != nil {
		t.Fatal(err)
	}
	theme.DensityScale = 1.1
	drainCommands(t, v, host, 2)
	last := "woxview/k:rows/lazy-list/k:99/text-field/field"
	if host.FocusedKey() != widget.Key(last) {
		t.Fatalf("wrong focus: %s", host.FocusedKey())
	}
	bounds := automationBounds(t, host, last)
	if bounds.Height != 24 || bounds.Y < 0 || bounds.Y+bounds.Height > 100 || v.fieldValues[first] != "kept" {
		t.Fatalf("row geometry or offscreen value lost: %+v %v", bounds, v.fieldValues[first])
	}
	if got := v.scrolls["woxview/k:rows/lazy-list"].MaxOffset(); got != 2300 {
		t.Fatalf("stale virtual extent: %v", got)
	}
}

// TestSizeEndThresholdChangesWithDensity exercises the notification policy without replacing the tree.
func TestSizeEndThresholdChangesWithDensity(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"scroll","props":{"endThreshold":"$space4","onEndReached":{"$handler":"end"},"style":{"width":100,"height":100}},"children":[{"type":"view","props":{"style":{"height":117}}}]}}`))
	theme := component.ControlTheme{DensityScale: 0.9}
	calls := 0
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, Theme: theme, OnEvent: func(HandlerID, json.RawMessage) { calls++ }})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	renderLayout(host, 250, 150, 1)
	if calls != 0 {
		t.Fatal("fired outside compact threshold")
	}
	theme.DensityScale = 1.1
	renderLayout(host, 250, 150, 2)
	renderLayout(host, 250, 150, 2)
	if calls != 1 {
		t.Fatalf("threshold notification count = %d", calls)
	}
}
