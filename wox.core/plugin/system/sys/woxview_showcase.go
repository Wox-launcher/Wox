package sys

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"wox/plugin"
)

// woxViewShowcase is a real plugin window, so its events and document updates
// travel through the same API path as third-party plugins.
type woxViewShowcase struct {
	mu       sync.Mutex
	window   *plugin.PluginWindow
	revision uint64
	sequence uint64
	visible  bool
	changed  bool
	tab      string
}

// showWoxViewShowcase opens a fresh, independently owned interactive fixture.
func (r *SysPlugin) showWoxViewShowcase(ctx context.Context) {
	s := &woxViewShowcase{visible: true, tab: "a"}
	width, height, yes := float64(760), float64(720), true
	created := r.api.CreateWindow(ctx, plugin.CreateWindowOption{
		Window:  plugin.WindowOptions{Title: r.api.GetTranslation(ctx, "plugin_sys_woxview_showcase"), Presentation: plugin.WindowPresentationWindow, Width: &width, Height: &height, Focusable: &yes, Resizable: &yes, Movable: &yes, Anchor: plugin.WindowAnchorCenter},
		Content: plugin.WindowContent{Type: plugin.WindowContentWoxView, Data: s.document("")},
		OnEvent: func(eventCtx context.Context, event plugin.WindowEvent) {
			if event.Type == plugin.WindowEventWoxView {
				var data struct{ Handler string }
				if json.Unmarshal(event.Data, &data) == nil && data.Handler == "openNote" {
					r.showWoxViewNote(eventCtx)
					return
				}
			}
			s.onEvent(eventCtx, r.api, event)
		},
	})
	if !created.Success || created.Window == nil {
		r.api.Log(ctx, plugin.LogLevelError, "WoxView showcase: "+created.ErrMsg)
		r.api.ShowToastMsg(ctx, plugin.ShowToastMsgOption{Message: "i18n:plugin_sys_woxview_failed"})
		return
	}
	s.mu.Lock()
	s.window = created.Window
	s.mu.Unlock()
	if result := created.Window.Show(ctx, plugin.WindowShowOption{Activate: true}); !result.Success {
		r.api.Log(ctx, plugin.LogLevelError, "WoxView showcase: "+result.ErrMsg)
	}
}

// onEvent keeps revisions and update sequence scoped to this window, and shows
// the actual handler payload so the fixture can verify the public contract.
func (s *woxViewShowcase) onEvent(ctx context.Context, api plugin.API, event plugin.WindowEvent) {
	if event.Type != plugin.WindowEventWoxView && event.Type != plugin.WindowEventThemeChanged && event.Type != plugin.WindowEventError {
		return
	}
	var data struct {
		Handler string
		Args    []json.RawMessage
	}
	_ = json.Unmarshal(event.Data, &data)
	s.mu.Lock()
	w := s.window
	if w == nil {
		s.mu.Unlock()
		return
	}
	switch data.Handler {
	case "reset":
		s.revision++
	case "toggle":
		s.visible = !s.visible
	case "transition":
		s.changed = !s.changed
	case "tabs":
		if len(data.Args) > 0 {
			_ = json.Unmarshal(data.Args[0], &s.tab)
		}
	}
	s.sequence++
	sequence := s.sequence
	status := string(event.Data)
	if event.Type == plugin.WindowEventError {
		status = event.ErrMsg
	}
	document := s.document(status)
	s.mu.Unlock()
	result := w.Content.Update(ctx, plugin.WindowContentUpdateOption{Data: document, DocumentID: event.DocumentID, Sequence: sequence})
	if !result.Success {
		api.Log(ctx, plugin.LogLevelError, "WoxView showcase update: "+result.ErrMsg)
		return
	}
	if data.Handler == "staleSequence" || data.Handler == "staleDocument" {
		stale := plugin.WindowContentUpdateOption{Data: document, DocumentID: event.DocumentID, Sequence: sequence}
		if data.Handler == "staleDocument" {
			stale.DocumentID = "replaced-document"
			stale.Sequence++
		}
		message := "i18n:plugin_sys_woxview_guard_pass"
		if w.Content.Update(ctx, stale).Success {
			message = "i18n:plugin_sys_woxview_guard_fail"
		}
		api.ShowToastMsg(ctx, plugin.ShowToastMsgOption{Message: message})
	}
	var command plugin.WindowContentCommand
	switch data.Handler {
	case "focus":
		command = plugin.WindowContentCommandFocus
	case "selectAll":
		command = plugin.WindowContentCommandSelectAll
	case "scrollTo":
		command = plugin.WindowContentCommandScrollTo
	}
	if command != "" {
		key := "draft"
		if command == plugin.WindowContentCommandScrollTo {
			key = "bottom"
		}
		if result := w.Content.Command(ctx, plugin.WindowContentCommandOption{DocumentID: event.DocumentID, Command: command, Key: key}); !result.Success {
			api.Log(ctx, plugin.LogLevelError, "WoxView showcase command: "+result.ErrMsg)
		}
	}
}

// document rebuilds immutable JSON while retaining keyed widget state on updates.
func (s *woxViewShowcase) document(status string) string {
	var root map[string]any
	if err := json.Unmarshal([]byte(woxViewShowcaseJSON), &root); err != nil {
		panic(err)
	}
	var visit func(map[string]any)
	visit = func(node map[string]any) {
		props, _ := node["props"].(map[string]any)
		switch node["key"] {
		case "status":
			props["text"] = status
		case "draft":
			props["revision"] = s.revision
		case "tab-content":
			if s.tab == "b" {
				props["text"] = "i18n:plugin_sys_woxview_option_b"
			}
		case "animated":
			if s.changed {
				props["style"] = map[string]any{"width": 180, "height": 52, "backgroundColor": "$success", "translateX": 12, "translateY": 3, "scale": 1.05, "opacity": 0.75}
			}
		case "presence":
			if !s.visible {
				node["children"] = []any{}
			}
		case "lazy":
			children := make([]any, 40)
			for i := range children {
				children[i] = map[string]any{"type": "text", "key": fmt.Sprintf("item-%d", i), "props": map[string]any{"text": fmt.Sprintf("lazy-list %02d", i)}}
			}
			children[20] = map[string]any{"type": "text-field", "key": "lazy-field", "props": map[string]any{"label": "i18n:plugin_sys_woxview_text", "onChange": map[string]any{"$handler": "lazyChange"}}}
			node["children"] = children
		}
		if children, ok := node["children"].([]any); ok {
			for _, child := range children {
				visit(child.(map[string]any))
			}
		}
	}
	visit(root["root"].(map[string]any))
	data, err := json.Marshal(root)
	if err != nil {
		panic(err)
	}
	return string(data)
}

const woxViewShowcaseJSON = `{
 "version":1,"root":{"type":"column","props":{"style":{"padding":12,"gap":8},"shortcuts":[{"key":"mod+enter","onPress":{"$handler":"shortcut"}}]},"children":[
  {"type":"text","props":{"text":"i18n:plugin_sys_woxview_showcase","style":{"fontSize":"$fontTitle","fontWeight":"bold"}}},
  {"type":"text","props":{"text":"i18n:plugin_sys_woxview_instructions","style":{"color":"$textSecondary"}}},
  {"type":"button","props":{"label":"i18n:plugin_sys_woxview_note_open","icon":"plugin.notes","onPress":{"$handler":"openNote"}}},
  {"type":"text","key":"status","props":{"text":"","style":{"color":"$accent","maxLines":2}}},
  {"type":"scroll","props":{"style":{"flex":1,"gap":12},"onEndReached":{"$handler":"end"}},"children":[
   {"type":"text","props":{"text":"i18n:plugin_sys_woxview_form","style":{"fontSize":"$fontLg","fontWeight":"semibold"}}},
   {"type":"form","props":{"style":{"gap":8},"onSubmit":{"$handler":"submit"}},"children":[
    {"type":"text-field","key":"draft","props":{"name":"draft","label":"i18n:plugin_sys_woxview_text","value":"","required":true,"placeholder":"i18n:plugin_sys_woxview_text","changeDebounce":150,"onChange":{"$handler":"change"},"onFocusChange":{"$handler":"focusChange"}}},
    {"type":"number-field","key":"number","props":{"name":"number","label":"i18n:plugin_sys_woxview_number","value":3,"min":0,"max":10,"step":1,"onChange":{"$handler":"change"}}},
    {"type":"hotkey","key":"hotkey","props":{"name":"hotkey","label":"i18n:plugin_sys_woxview_hotkey","onChange":{"$handler":"change"}}},
    {"type":"checkbox","key":"checkbox","props":{"name":"checkbox","label":"i18n:plugin_sys_woxview_checkbox","value":true,"onChange":{"$handler":"change"}}},
    {"type":"switch","key":"switch","props":{"name":"switch","label":"i18n:plugin_sys_woxview_switch","onChange":{"$handler":"change"}}},
    {"type":"slider","key":"slider","props":{"name":"slider","label":"i18n:plugin_sys_woxview_slider","value":0.5,"onChange":{"$handler":"change"}}},
    {"type":"select","key":"select","props":{"name":"select","label":"i18n:plugin_sys_woxview_select","value":"a","options":[{"value":"a","label":"i18n:plugin_sys_woxview_option_a"},{"value":"b","label":"i18n:plugin_sys_woxview_option_b"}],"onChange":{"$handler":"change"}}},
    {"type":"radio-group","props":{"name":"radio","label":"i18n:plugin_sys_woxview_radio","value":"a","options":[{"value":"a","label":"i18n:plugin_sys_woxview_option_a"},{"value":"b","label":"i18n:plugin_sys_woxview_option_b"}],"onChange":{"$handler":"change"}}},
    {"type":"multi-select","props":{"name":"multi","label":"i18n:plugin_sys_woxview_multi","options":[{"value":"a","label":"i18n:plugin_sys_woxview_option_a"},{"value":"b","label":"i18n:plugin_sys_woxview_option_b"}],"onChange":{"$handler":"change"}}},
    {"type":"row","props":{"wrap":true,"style":{"gap":8}},"children":[
     {"type":"submit","props":{"label":"i18n:plugin_sys_woxview_submit","variant":"primary"}},
     {"type":"button","props":{"label":"i18n:plugin_sys_woxview_reset","onPress":{"$handler":"reset"}}},
     {"type":"button","props":{"label":"i18n:plugin_sys_woxview_focus","onPress":{"$handler":"focus"}}},
     {"type":"button","props":{"label":"i18n:plugin_sys_woxview_select_all","onPress":{"$handler":"selectAll"}}},
     {"type":"button","props":{"label":"i18n:plugin_sys_woxview_scroll","onPress":{"$handler":"scrollTo"}}},
     {"type":"button","props":{"label":"i18n:plugin_sys_woxview_sequence","onPress":{"$handler":"staleSequence"}}},
     {"type":"button","props":{"label":"i18n:plugin_sys_woxview_document","onPress":{"$handler":"staleDocument"}}}
    ]}
   ]},
   {"type":"text-field","props":{"label":"i18n:plugin_sys_woxview_error","error":"i18n:plugin_sys_woxview_error_text","required":true}},
   {"type":"text-field","props":{"label":"i18n:plugin_sys_woxview_disabled","disabled":true,"disabledStyle":{"color":"$disabled"}}},
   {"type":"form","props":{"disabled":true,"onSubmit":{"$handler":"disabledSubmit"}},"children":[{"type":"text-field","props":{"name":"disabled","label":"i18n:plugin_sys_woxview_disabled"}},{"type":"submit","props":{"label":"i18n:plugin_sys_woxview_submit"}}]},
   {"type":"form","props":{"submitting":true,"onSubmit":{"$handler":"submitting"}},"children":[{"type":"submit","props":{"label":"i18n:plugin_sys_woxview_submit"}}]},
   {"type":"divider"},
   {"type":"text","props":{"text":"i18n:plugin_sys_woxview_components","style":{"fontSize":"$fontLg","fontWeight":"semibold"}}},
   {"type":"tabs","props":{"label":"i18n:plugin_sys_woxview_tabs","value":"a","items":[{"value":"a","label":"i18n:plugin_sys_woxview_option_a"},{"value":"b","label":"i18n:plugin_sys_woxview_option_b"}],"onChange":{"$handler":"tabs"}}},
   {"type":"text","key":"tab-content","props":{"text":"i18n:plugin_sys_woxview_option_a"}},
   {"type":"tags","props":{"label":"i18n:plugin_sys_woxview_tags","options":[{"value":"a","label":"i18n:plugin_sys_woxview_option_a"},{"value":"b","label":"i18n:plugin_sys_woxview_option_b"}],"onChange":{"$handler":"change"}}},
   {"type":"list","props":{"label":"i18n:plugin_sys_woxview_list","style":{"height":80},"options":[{"value":"a","label":"i18n:plugin_sys_woxview_option_a"},{"value":"b","label":"i18n:plugin_sys_woxview_option_b"}],"onChange":{"$handler":"change"}}},
   {"type":"row","props":{"wrap":true,"style":{"gap":8}},"children":[{"type":"tag","props":{"text":"i18n:plugin_sys_woxview_tag"}},{"type":"badge","props":{"text":"i18n:plugin_sys_woxview_badge"}},{"type":"image","props":{"icon":"action.execute","style":{"width":24,"height":24}}},{"type":"icon-button","props":{"icon":"control.refresh","iconColorMode":"tint","label":"i18n:plugin_sys_woxview_reset","onPress":{"$handler":"reset"}}},{"type":"loading"},{"type":"progress","props":{"value":0.6,"style":{"width":120}}}]},
   {"type":"text","props":{"selectable":true},"children":[{"type":"span","props":{"text":"i18n:plugin_sys_woxview_rich","style":{"fontWeight":"semibold","color":"$link","underline":true},"onPress":{"$handler":"span"}}}]},
   {"type":"markdown","props":{"content":"[Wox](https://woxlauncher.com)","onLinkPress":{"$handler":"markdown"}}},
   {"type":"link","props":{"label":"i18n:plugin_sys_woxview_link","url":"https://woxlauncher.com","onPress":{"$handler":"link"}}},
   {"type":"code","props":{"code":"{\"version\":1}","language":"json"}},
   {"type":"table","props":{"label":"i18n:plugin_sys_woxview_table","style":{"height":120},"columns":[{"key":"name","label":"i18n:plugin_sys_woxview_text","width":180}],"rows":[{"key":"row-1","cells":["row-1"],"actions":[{"key":"open","label":"i18n:plugin_sys_woxview_button","onPress":{"$handler":"row"}}]}]}},
   {"type":"grid","props":{"columns":2,"gap":8},"children":[{"type":"tag","props":{"text":"i18n:plugin_sys_woxview_option_a","style":{"alignSelf":"center"}}},{"type":"tag","props":{"text":"i18n:plugin_sys_woxview_option_b"}}]},
   {"type":"row","props":{"style":{"gap":8}},"children":[{"type":"button","props":{"label":"i18n:plugin_sys_woxview_button","variant":"outline","onPress":{"$handler":"button"}}},{"type":"spacer"},{"type":"button","props":{"label":"i18n:plugin_sys_woxview_disabled","disabled":true}}]},
   {"type":"stack","props":{"style":{"height":72,"overflow":"hidden","floating":true,"borderRadius":"$radiusMd","borderLeftWidth":2,"borderColor":"$border","backgroundColor":{"light":"#eeeeee","dark":"#333333"}}},"children":[{"type":"view","props":{"style":{"left":8,"top":8,"width":55,"aspectRatio":1,"backgroundColor":"$accent","opacity":0.7,"shadow":"sm","rotate":12}}},{"type":"text","props":{"text":"stack / overflow / aspectRatio / shadow","style":{"left":80,"top":20}}}]},
   {"type":"scroll","props":{"horizontal":true,"stickToEnd":true,"onEndReached":{"$handler":"horizontalEnd"},"style":{"height":45}},"children":[{"type":"tag","props":{"text":"i18n:plugin_sys_woxview_option_a","style":{"width":300}}},{"type":"tag","props":{"text":"i18n:plugin_sys_woxview_option_b","style":{"width":300}}}]},
   {"type":"text","props":{"text":"i18n:plugin_sys_woxview_animation","style":{"fontSize":"$fontLg","fontWeight":"semibold"}}},
   {"type":"row","props":{"wrap":true,"style":{"gap":8}},"children":[{"type":"button","props":{"label":"i18n:plugin_sys_woxview_toggle","onPress":{"$handler":"toggle"}}},{"type":"button","props":{"label":"i18n:plugin_sys_woxview_transition","onPress":{"$handler":"transition"}}}]},
   {"type":"view","key":"animated","props":{"label":"i18n:plugin_sys_woxview_transition","onPress":{"$handler":"card"},"tooltip":"i18n:plugin_sys_woxview_tooltip","onHoverChange":{"$handler":"hover"},"onDoubleTap":{"$handler":"double"},"onContextMenu":{"$handler":"context"},"contextMenu":[{"label":"i18n:plugin_sys_woxview_button","onPress":{"$handler":"menu"}}],"hoverStyle":{"opacity":0.8},"pressedStyle":{"opacity":0.6},"focusStyle":{"borderColor":"$focus"},"transition":{"properties":["width","height","opacity","backgroundColor","translateX","translateY","scale"],"duration":400},"style":{"width":120,"height":42,"backgroundColor":"$accent","borderRadius":"$radiusMd"}}},
   {"type":"row","key":"presence","props":{"style":{"gap":8}},"children":[{"type":"tag","key":"fade","props":{"text":"fade","enter":"fade","exit":"fade"}},{"type":"tag","key":"slide","props":{"text":"slideUp","enter":"slideUp","exit":"slideUp"}},{"type":"tag","key":"scale","props":{"text":"scale","enter":"scale","exit":"scale"}}]},
   {"type":"row","props":{"style":{"gap":8}},"children":[{"type":"tag","props":{"text":"pulse","animation":{"type":"pulse","duration":1200}}},{"type":"tag","props":{"text":"spin","animation":{"type":"spin","duration":1200}}},{"type":"tag","props":{"text":"shimmer","animation":{"type":"shimmer","duration":1200}}}]},
   {"type":"lazy-list","key":"lazy","props":{"itemHeight":32,"onEndReached":{"$handler":"lazyEnd"},"style":{"height":120}}},
   {"type":"text","key":"bottom","props":{"text":"i18n:plugin_sys_woxview_bottom"}}
  ]}
 ]}}
`
