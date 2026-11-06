package woxview

import (
	"encoding/json"
	"strings"
	"testing"

	woxui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// TestTranslationUpdatesDisplayWithoutChangingFieldData exercises locale changes on a live form with retained edits.
func TestTranslationUpdatesDisplayWithoutChangingFieldData(t *testing.T) {
	tree := mustParse(t, `{"version":1,"root":{"type":"form","props":{"onSubmit":{"$handler":"i18n:save"}},"children":[
		{"type":"text","props":{"text":"i18n:title"}},
		{"type":"text","props":{"text":"i18n:selectable","selectable":true}},
		{"type":"text-field","key":"text","props":{"name":"i18n:name","label":"i18n:label","placeholder":"i18n:hint","error":"i18n:error","value":"i18n:raw"}},
		{"type":"switch","props":{"name":"enabled","label":"i18n:switch"}},
		{"type":"checkbox","props":{"label":"i18n:checkbox"}},
		{"type":"slider","props":{"label":"i18n:slider"}},
		{"type":"select","props":{"label":"i18n:select","options":[{"value":"i18n:value","label":"i18n:option"}]}},
		{"type":"list","props":{"name":"choice","label":"i18n:list","value":"i18n:value","options":[{"value":"i18n:value","label":"i18n:option"}]}},
		{"type":"button","props":{"label":"i18n:button"}},
		{"type":"icon-button","props":{"label":"i18n:icon","icon":"control.close"}},
		{"type":"view","props":{"label":"i18n:box","onPress":{"$handler":"box"},"style":{"width":100,"height":20}}},
		{"type":"submit","key":"submit","props":{"label":"i18n:submit"}},
		{"type":"button","props":{"label":"  Literal text  "}}
	]}}`)
	v := NewView()
	v.SetTree(tree)
	locale := "en"
	lookups := map[string]bool{}
	var events []recordedEvent
	host := widget.NewHost(func(info woxui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, Translate: func(text string) string {
			if !strings.HasPrefix(text, "i18n:") {
				t.Fatalf("literal sent to translator: %q", text)
			}
			lookups[text] = true
			return locale + ":" + strings.TrimPrefix(text, "i18n:")
		}, OnEvent: func(id HandlerID, args json.RawMessage) { events = append(events, recordedEvent{id, string(args)}) }})
	})
	host.AttachServices(fakeServices{})
	render := func() {
		host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 600, Height: 1200}, PixelSize: woxui.PixelSize{Width: 900, Height: 1800}, Scale: 1.5})
	}
	render()
	host.FocusAutomationID("woxview/form0/k:text/text-field/field")
	host.TextInput(woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: " edited"})
	for _, language := range []string{"en", "zh"} {
		locale = language
		render()
		labels := map[string]bool{}
		for _, n := range host.Snapshot().Tree.Nodes {
			labels[n.Label] = true
		}
		for _, key := range []string{"selectable", "label", "error", "switch", "checkbox", "slider", "select", "option", "button", "icon", "box", "submit"} {
			if !labels[language+":"+key] {
				t.Errorf("missing translated label %s:%s", language, key)
			}
		}
		if !labels["  Literal text  "] {
			t.Error("literal label was modified")
		}
		tap(host, automationBounds(t, host, "woxview/form0/k:submit/submit"))
		got := events[len(events)-1]
		if got.handler != "i18n:save" || got.args != `[{"choice":"i18n:value","enabled":false,"i18n:name":"i18n:raw edited"}]` {
			t.Fatalf("translated protocol data: %+v", got)
		}
	}
	for _, text := range []string{"i18n:title", "i18n:hint", "i18n:error"} {
		if !lookups[text] {
			t.Errorf("did not translate %s", text)
		}
	}
	for _, text := range []string{"i18n:name", "i18n:raw", "i18n:value", "i18n:save"} {
		if lookups[text] {
			t.Errorf("translated data %s", text)
		}
	}
	if p := tree.Root.Children[2].Props.(*TextFieldProps); p.Label != "i18n:label" || p.Error != "i18n:error" {
		t.Fatal("translation mutated field props")
	}
	if p := tree.Root.Children[6].Props.(*ChoiceProps); p.Options[0].Label != "i18n:option" {
		t.Fatal("translation mutated shared option slice")
	}
}

// TestTranslationHookIsOptional preserves existing embedders that have no translation service.
func TestTranslationHookIsOptional(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"button","props":{"label":"i18n:unresolved"}}}`))
	var events []recordedEvent
	host := renderView(t, v, &events)
	for _, n := range host.Snapshot().Tree.Nodes {
		if n.AutomationID == "woxview/button0" && n.Label == "i18n:unresolved" {
			return
		}
	}
	t.Fatal("missing hook changed the original label")
}
