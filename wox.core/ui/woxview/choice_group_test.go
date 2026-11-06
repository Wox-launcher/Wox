package woxview

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// singleChoiceForm keeps field identity stable across producer echoes, resets, and disabling.
func singleChoiceForm(t *testing.T, kind, value string, revision int, disabled bool) *Tree {
	t.Helper()
	options := "options"
	if kind == "tabs" {
		options = "items"
	}
	return mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"form","props":{"disabled":%t,"onSubmit":{"$handler":"submit"}},"children":[
		{"type":%q,"key":"choice","props":{"name":"mode","label":"Mode","value":%q,"revision":%d,"onChange":{"$handler":"change"},%q:[{"value":"a","label":"Alpha"},{"value":"b","label":"Beta","disabled":true},{"value":"c","label":"Charlie"}]}},
		{"type":"submit","key":"save","props":{"label":"Save"}}
	]}}`, disabled, kind, value, revision, options))
}

// TestSingleChoiceGroupsKeepFormState covers keyboard navigation, stale echoes, and forced resets.
func TestSingleChoiceGroupsKeepFormState(t *testing.T) {
	for _, kind := range []string{"radio-group", "tabs"} {
		for _, scale := range []float32{1, 1.5, 2} {
			t.Run(fmt.Sprintf("%s/%g", kind, scale), func(t *testing.T) {
				v := NewView()
				v.SetTree(singleChoiceForm(t, kind, "a", 0, false))
				var events []recordedEvent
				host := layoutHost(v, &events)
				defer host.Dispose()
				render := func() { renderLayout(host, 300, 180, scale) }
				id := "woxview/form0/k:choice/" + kind
				render()
				a := automationBounds(t, host, id+"/option:YQ")
				c := automationBounds(t, host, id+"/option:Yw")
				if a.Height != 32 || a.Y != c.Y || c.X <= a.X || a.Width != c.Width || c.X+c.Width > 300 {
					t.Fatalf("segment geometry: %+v %+v", a, c)
				}
				host.FocusAutomationID(id + "/option:YQ")
				host.Key(ui.KeyEvent{Key: ui.KeyArrowRight, Down: true})
				if host.FocusedKey() != widget.Key(id+"/option:Yw") || len(events) != 0 {
					t.Fatal("navigation selected a value or focused a disabled option")
				}
				host.Key(ui.KeyEvent{Key: ui.KeyEnter, Down: true})
				render()
				if len(events) != 1 || events[0].args != `["c"]` {
					t.Fatalf("activation: %+v", events)
				}
				tap(host, automationBounds(t, host, id+"/option:YQ"))
				v.SetTree(singleChoiceForm(t, kind, "c", 0, false))
				render()
				if v.fieldValues[id] != "a" {
					t.Fatal("stale echo rewound local selection")
				}
				tap(host, automationBounds(t, host, "woxview/form0/k:save/submit"))
				if events[len(events)-1].args != `[{"mode":"a"}]` {
					t.Fatalf("form snapshot: %+v", events)
				}
				v.SetTree(singleChoiceForm(t, kind, "c", 1, true))
				render()
				total := len(events)
				tap(host, automationBounds(t, host, id+"/option:YQ"))
				if v.fieldValues[id] != "c" || len(events) != total || host.FocusAutomationID(id+"/option:YQ") {
					t.Fatal("reset or form disabling failed")
				}
			})
		}
	}
}

// multiChoiceForm distinguishes ordered array values from scalar choice values on the wire.
func multiChoiceForm(t *testing.T, kind, value string, revision int, options string) *Tree {
	t.Helper()
	return mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"form","props":{"onSubmit":{"$handler":"submit"}},"children":[
		{"type":%q,"key":"choice","props":{"name":"selected","label":"Languages","value":%s,"revision":%d,"onChange":{"$handler":"change"},"options":%s}},
		{"type":"submit","key":"save","props":{"label":"Save"}}
	]}}`, kind, value, revision, options))
}

// TestMultiSelectSnapshotsAndRemovedOptions protects arrays from mutation by later edits or normalization.
func TestMultiSelectSnapshotsAndRemovedOptions(t *testing.T) {
	const options = `[{"value":"a","label":"Alpha"},{"value":"b","label":"Beta","disabled":true},{"value":"c","label":"Charlie"}]`
	for _, kind := range []string{"multi-select", "tags"} {
		v := NewView()
		v.SetTree(multiChoiceForm(t, kind, `[]`, 0, options))
		var events []Event
		host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
			return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, OnNodeEvent: func(e Event) { events = append(events, e) }})
		})
		host.AttachServices(fakeServices{})
		defer host.Dispose()
		id := "woxview/form0/k:choice/" + kind
		for _, scale := range []float32{1, 1.5, 2} {
			renderLayout(host, 180, 250, scale)
			a := automationBounds(t, host, id+"/option:YQ")
			c := automationBounds(t, host, id+"/option:Yw")
			if a.Height != 32 || c.Height != 32 || c.Y <= a.Y || c.X+c.Width > 180 {
				t.Fatalf("tags did not wrap within logical width: %+v %+v", a, c)
			}
		}
		tap(host, automationBounds(t, host, id+"/option:YQ"))
		renderLayout(host, 180, 250, 2)
		first := v.fieldValues[id].([]string)
		host.Key(ui.KeyEvent{Key: ui.KeyArrowRight, Down: true})
		if host.FocusedKey() != widget.Key(id+"/option:Yw") {
			t.Fatal("tag keyboard navigation did not skip disabled choice")
		}
		host.Key(ui.KeyEvent{Key: ui.KeySpace, Down: true})
		renderLayout(host, 180, 250, 2)
		if !reflect.DeepEqual(first, []string{"a"}) || !reflect.DeepEqual(v.fieldValues[id], []string{"a", "c"}) {
			t.Fatalf("selection mutated retained snapshot: %v / %v", first, v.fieldValues[id])
		}
		if len(events) != 2 || string(events[1].Args) != `[["a","c"]]` || events[1].Key != "choice" || !events[1].Coalescible {
			t.Fatalf("array change event: %+v", events)
		}
		v.SetTree(multiChoiceForm(t, kind, `["a"]`, 0, options))
		renderLayout(host, 180, 250, 1.5)
		if !reflect.DeepEqual(v.fieldValues[id], []string{"a", "c"}) {
			t.Fatal("stale array echo replaced a newer selection")
		}
		tap(host, automationBounds(t, host, "woxview/form0/k:save/submit"))
		if string(events[len(events)-1].Args) != `[{"selected":["a","c"]}]` || events[len(events)-1].Key != "save" || events[len(events)-1].Coalescible {
			t.Fatalf("array form value: %s", events[len(events)-1].Args)
		}
		// Removing c must keep a across repeated builds, even with an unchanged [] prop.
		v.SetTree(multiChoiceForm(t, kind, `[]`, 1, options))
		renderLayout(host, 180, 250, 1)
		if value := v.fieldValues[id].([]string); value == nil || len(value) != 0 {
			t.Fatalf("reset must produce [], got %#v", value)
		}
		tap(host, automationBounds(t, host, id+"/option:YQ"))
		renderLayout(host, 180, 250, 1)
		tap(host, automationBounds(t, host, id+"/option:Yw"))
		v.SetTree(multiChoiceForm(t, kind, `[]`, 1, `[{"value":"a","label":"Alpha"}]`))
		renderLayout(host, 180, 250, 1.5)
		renderLayout(host, 180, 250, 2)
		if !reflect.DeepEqual(v.fieldValues[id], []string{"a"}) {
			t.Fatalf("normalization reset unrelated local selection: %v", v.fieldValues[id])
		}
	}
}

// TestChoiceGroupsTranslateAndDisable exercises labels without changing protocol values.
func TestChoiceGroupsTranslateAndDisable(t *testing.T) {
	for _, kind := range []string{"radio-group", "tabs", "multi-select", "tags"} {
		v := NewView()
		disabled := false
		update := func() {
			options := "options"
			if kind == "tabs" {
				options = "items"
			}
			v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"form","props":{"disabled":%t},"children":[{"type":%q,"props":{"label":"i18n:group","required":true,"error":"i18n:error","onChange":{"$handler":"change"},%q:[{"value":"a","label":"i18n:alpha"}]}}]}}`, disabled, kind, options)))
		}
		update()
		var events []Event
		host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
			return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Translate: func(s string) string { return "translated:" + s }, OnNodeEvent: func(e Event) { events = append(events, e) }})
		})
		host.AttachServices(fakeServices{})
		defer host.Dispose()
		id := "woxview/form0/" + kind + "0"
		renderLayout(host, 220, 200, 1.5)
		for _, node := range host.Snapshot().Tree.Nodes {
			if node.AutomationID == id+"/option:YQ" && node.Label != "translated:i18n:alpha" {
				t.Fatalf("untranslated option: %s", node.Label)
			}
			if node.AutomationID == id+"/group" && node.Label != "translated:i18n:group *" {
				t.Fatalf("required group label: %s", node.Label)
			}
			if node.AutomationID == id+"/error" && node.Label != "translated:i18n:error" {
				t.Fatalf("error label: %s", node.Label)
			}
		}
		tap(host, automationBounds(t, host, id+"/option:YQ"))
		if len(events) != 1 || (string(events[0].Args) != `["a"]` && string(events[0].Args) != `[["a"]]`) {
			t.Fatalf("translated protocol value: %+v", events)
		}
		disabled = true
		update()
		renderLayout(host, 220, 200, 2)
		tap(host, automationBounds(t, host, id+"/option:YQ"))
		if len(events) != 1 || host.FocusAutomationID(id+"/option:YQ") {
			t.Fatal("disabled group accepted input")
		}
	}
}

// TestMultiSelectRetainsOffscreenArrays covers disposal/remount and an offscreen reset.
func TestMultiSelectRetainsOffscreenArrays(t *testing.T) {
	tree := func(revision int) *Tree {
		items := make([]string, 50)
		for i := range items {
			items[i] = fmt.Sprintf(`{"type":"multi-select","key":"%d","props":{"name":"f%d","revision":%d,"options":[{"value":"a","label":"Alpha"}]}}`, i, i, revision)
		}
		return mustParse(t, `{"version":1,"root":{"type":"form","children":[{"type":"lazy-list","props":{"itemHeight":40,"style":{"height":100}},"children":[`+strings.Join(items, ",")+`]}]}}`)
	}
	v := NewView()
	v.SetTree(tree(0))
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	render := func() { renderLayout(host, 200, 140, 1.5) }
	wheel := func(delta float32) {
		host.Pointer(ui.PointerEvent{Kind: ui.PointerScroll, Position: ui.Point{X: 10, Y: 10}, Scroll: ui.Point{Y: delta}})
		render()
	}
	id := "woxview/form0/lazy-list0/k:0/multi-select"
	render()
	tap(host, automationBounds(t, host, id+"/option:YQ"))
	render()
	wheel(-1000)
	if hasAutomation(host, id+"/option:YQ") {
		t.Fatal("edited field was not unmounted")
	}
	v.SetTree(tree(0))
	if !reflect.DeepEqual(v.formValues(v.Tree().Root, "woxview/form0")["f0"], []string{"a"}) {
		t.Fatal("offscreen form lost the array value")
	}
	wheel(10000)
	if !reflect.DeepEqual(v.fieldValues[id], []string{"a"}) {
		t.Fatal("remount lost the array value")
	}
	wheel(-1000)
	v.SetTree(tree(1))
	wheel(10000)
	if !reflect.DeepEqual(v.fieldValues[id], []string{}) {
		t.Fatalf("offscreen reset lost on remount: %v", v.fieldValues[id])
	}
}

// TestTagAndBadgeTranslateAndScale checks shared tag tiers through real layout at mixed DPI.
func TestTagAndBadgeTranslateAndScale(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[{"type":"tag","props":{"text":"i18n:tag","style":{"color":"$success"}}},{"type":"badge","props":{"text":"i18n:badge","style":{"color":"$warning"}}}]}}`))
	density := float32(1)
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: component.ControlTheme{DensityScale: density}, Translate: func(s string) string { return "translated:" + s }})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	for _, scale := range []float32{1, 1.5, 2} {
		for _, d := range []float32{0.9, 1, 1.1} {
			density = d
			renderLayout(host, 200, 200, scale)
			theme := component.ControlTheme{DensityScale: density}
			if automationBounds(t, host, "woxview/column0/tag0").Height != theme.Scaled(22) || automationBounds(t, host, "woxview/column0/badge1").Height != theme.Scaled(18) {
				t.Fatal("tag or badge ignored density")
			}
			for _, node := range host.Snapshot().Tree.Nodes {
				if node.AutomationID == "woxview/column0/tag0" && node.Label != "translated:i18n:tag" {
					t.Fatalf("tag not translated: %s", node.Label)
				}
			}
		}
	}
}

// TestChoiceGroupsRejectInvalidInput verifies the catalog's scalar/array distinction and item limits.
func TestChoiceGroupsRejectInvalidInput(t *testing.T) {
	for _, raw := range []string{
		`{"type":"radio-group","props":{"value":"missing"}}`,
		`{"type":"radio-group","props":{"value":["a"]}}`,
		`{"type":"tabs","props":{"options":[]}}`,
		`{"type":"tabs","props":{"items":[{"value":"a"},{"value":"a"}]}}`,
		`{"type":"tabs","props":{"items":[{"value":""}]}}`,
		`{"type":"tabs","props":{"onChange":{"$handler":""}}}`,
		`{"type":"multi-select","props":{"value":"a"}}`,
		`{"type":"multi-select","props":{"value":["a","a"],"options":[{"value":"a"}]}}`,
		`{"type":"tags","props":{"value":["missing"],"options":[{"value":"a"}]}}`,
		`{"type":"tags","props":{"value":[1]}}`,
		`{"type":"tag","props":{"text":1}}`,
		`{"type":"badge","props":{"style":{"color":"$missing"}}}`,
		`{"type":"tabs","children":[{"type":"text"}]}`,
	} {
		if _, err := Parse([]byte(`{"version":1,"root":` + raw + `}`)); err == nil {
			t.Fatalf("accepted invalid contract %s", raw)
		}
	}
	options := make([]ChoiceOption, 501)
	for i := range options {
		options[i] = ChoiceOption{Value: fmt.Sprint(i)}
	}
	data, _ := json.Marshal(options)
	for _, kind := range []string{"radio-group", "multi-select", "tags", "tabs"} {
		field := "options"
		if kind == "tabs" {
			field = "items"
		}
		if _, err := Parse([]byte(fmt.Sprintf(`{"version":1,"root":{"type":%q,"props":{%q:%s}}}`, kind, field, data))); err == nil {
			t.Fatalf("%s accepted too many choices", kind)
		}
	}
}
