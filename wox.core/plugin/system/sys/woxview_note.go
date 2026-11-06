package sys

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"wox/plugin"
)

type showcaseNote struct {
	title string
	body  string
	tasks [3]bool
}

// showcaseNotes owns temporary example data separately from the Notes repository.
type showcaseNotes struct {
	mu       sync.Mutex
	window   *plugin.PluginWindow
	notes    []showcaseNote
	selected int
	search   string
	newTitle string
	sequence uint64
}

// showWoxViewNote creates an independent WoxView window with sample notes.
func (r *SysPlugin) showWoxViewNote(ctx context.Context) {
	s := &showcaseNotes{
		newTitle: r.api.GetTranslation(ctx, "plugin_sys_woxview_note_new_title"),
		notes: []showcaseNote{
			{title: r.api.GetTranslation(ctx, "plugin_sys_woxview_note_first"), body: r.api.GetTranslation(ctx, "plugin_sys_woxview_note_first_body")},
			{title: r.api.GetTranslation(ctx, "plugin_sys_woxview_note_second"), body: r.api.GetTranslation(ctx, "plugin_sys_woxview_note_second_body"), tasks: [3]bool{true, false, true}},
		},
	}
	width, height, yes := float64(960), float64(680), true
	created := r.api.CreateWindow(ctx, plugin.CreateWindowOption{
		Window:  plugin.WindowOptions{Title: r.api.GetTranslation(ctx, "plugin_sys_woxview_note_window"), Presentation: plugin.WindowPresentationWindow, Width: &width, Height: &height, Focusable: &yes, Resizable: &yes, Movable: &yes, Anchor: plugin.WindowAnchorCenter},
		Content: plugin.WindowContent{Type: plugin.WindowContentWoxView, Data: s.document()},
		OnEvent: func(eventCtx context.Context, event plugin.WindowEvent) { s.onEvent(eventCtx, r.api, event) },
	})
	if !created.Success || created.Window == nil {
		r.api.Log(ctx, plugin.LogLevelError, "WoxView note example: "+created.ErrMsg)
		r.api.ShowToastMsg(ctx, plugin.ShowToastMsgOption{Message: "i18n:plugin_sys_woxview_note_failed"})
		return
	}
	s.mu.Lock()
	s.window = created.Window
	s.mu.Unlock()
	if result := created.Window.Show(ctx, plugin.WindowShowOption{Activate: true}); !result.Success {
		r.api.Log(ctx, plugin.LogLevelError, "WoxView note example: "+result.ErrMsg)
	}
}

// onEvent applies each event to its original keyed note before publishing the
// next tree. This keeps delayed edits from landing in a newly selected note.
func (s *showcaseNotes) onEvent(ctx context.Context, api plugin.API, event plugin.WindowEvent) {
	if event.Type != plugin.WindowEventWoxView {
		return
	}
	var data struct {
		Handler string
		Key     string
		Args    []json.RawMessage
	}
	if json.Unmarshal(event.Data, &data) != nil {
		return
	}
	s.mu.Lock()
	if s.window == nil || !s.apply(data.Handler, data.Key, data.Args) {
		s.mu.Unlock()
		return
	}
	s.sequence++
	sequence, document, window := s.sequence, s.document(), s.window
	s.mu.Unlock()
	if result := window.Content.Update(ctx, plugin.WindowContentUpdateOption{Data: document, DocumentID: event.DocumentID, Sequence: sequence}); !result.Success {
		api.Log(ctx, plugin.LogLevelError, "WoxView note example update: "+result.ErrMsg)
	}
}

// apply handles only events from this example; the caller serializes state access.
func (s *showcaseNotes) apply(handler, key string, args []json.RawMessage) bool {
	var text string
	switch handler {
	case "search":
		if len(args) == 0 || json.Unmarshal(args[0], &text) != nil {
			return false
		}
		s.search = text
		return true
	case "add":
		s.notes = append(s.notes, showcaseNote{title: s.newTitle})
		s.selected = len(s.notes) - 1
		s.search = ""
		return true
	case "select", "title", "body", "task":
	default:
		return false
	}
	parts := strings.Split(key, "-")
	if len(parts) < 2 {
		return false
	}
	index, err := strconv.Atoi(parts[1])
	if err != nil || index < 0 || index >= len(s.notes) {
		return false
	}
	if handler == "select" {
		s.selected = index
		return true
	}
	if len(args) == 0 {
		return false
	}
	if handler == "task" {
		if len(parts) != 3 {
			return false
		}
		task, err := strconv.Atoi(parts[2])
		var checked bool
		if err != nil || task < 0 || task >= len(s.notes[index].tasks) || json.Unmarshal(args[0], &checked) != nil {
			return false
		}
		s.notes[index].tasks[task] = checked
		return true
	}
	if json.Unmarshal(args[0], &text) != nil {
		return false
	}
	if handler == "title" {
		s.notes[index].title = text
	} else {
		s.notes[index].body = text
	}
	return true
}

type showcaseNode map[string]any

// noteNode creates a compact declarative node without retaining UI state.
func noteNode(kind, key string, props map[string]any, children ...showcaseNode) showcaseNode {
	n := showcaseNode{"type": kind}
	if key != "" {
		n["key"] = key
	}
	if len(props) > 0 {
		n["props"] = props
	}
	if len(children) > 0 {
		n["children"] = children
	}
	return n
}

func noteHandler(id string) map[string]string { return map[string]string{"$handler": id} }

// document composes a themed header, searchable sidebar, checklist and editor.
func (s *showcaseNotes) document() string {
	list := make([]showcaseNode, 0, len(s.notes))
	for i, note := range s.notes {
		if !strings.Contains(strings.ToLower(note.title), strings.ToLower(s.search)) {
			continue
		}
		title := note.title
		if title == "" {
			title = "i18n:plugin_sys_woxview_note_new_title"
		}
		style := map[string]any{"padding": 8, "borderRadius": "$radiusSm"}
		textStyle := map[string]any{"maxLines": 1}
		hover := "$hover"
		if i == s.selected {
			style["backgroundColor"] = "$selected"
			textStyle["color"] = "$selectedText"
			hover = "$selected"
		}
		list = append(list, noteNode("view", fmt.Sprintf("note-%d", i), map[string]any{"label": title, "onPress": noteHandler("select"), "style": style, "hoverStyle": map[string]any{"backgroundColor": hover}}, noteNode("text", "", map[string]any{"text": title, "style": textStyle})))
	}
	if len(list) == 0 {
		list = append(list, noteNode("text", "", map[string]any{"text": "i18n:plugin_sys_woxview_note_no_results", "style": map[string]any{"color": "$textSecondary"}}))
	}
	note := s.notes[s.selected]
	index := strconv.Itoa(s.selected)
	checkboxes := make([]showcaseNode, 0, len(note.tasks))
	for i, checked := range note.tasks {
		label := fmt.Sprintf("i18n:plugin_sys_woxview_note_task_%d", i+1)
		checkboxes = append(checkboxes, noteNode("row", "", map[string]any{"style": map[string]any{"gap": 8, "alignItems": "center"}},
			noteNode("checkbox", fmt.Sprintf("task-%d-%d", s.selected, i), map[string]any{"label": label, "value": checked, "onChange": noteHandler("task")}),
			noteNode("text", "", map[string]any{"text": label, "style": map[string]any{"flex": 1}})))
	}
	var toolbar []showcaseNode
	for _, item := range []struct{ icon, label string }{
		{"control.format-heading", "plugin_sys_woxview_note_format_heading"}, {"control.format-bold", "plugin_sys_woxview_note_format_bold"},
		{"control.format-italic", "plugin_sys_woxview_note_format_italic"}, {"control.format-underline", "plugin_sys_woxview_note_format_underline"},
		{"control.format-strikethrough", "plugin_sys_woxview_note_format_strike"}, {"control.format-ordered", "plugin_sys_woxview_note_format_ordered"},
		{"control.format-quote", "plugin_sys_woxview_note_format_quote"}, {"control.format-table", "plugin_sys_woxview_note_format_table"},
	} {
		// The current WoxView editor is plain text, so rich-format actions cannot be applied.
		toolbar = append(toolbar, noteNode("icon-button", "", map[string]any{"icon": item.icon, "iconColorMode": "tint", "label": "i18n:" + item.label, "disabled": true}))
	}
	root := noteNode("column", "", map[string]any{"style": map[string]any{"padding": 12, "gap": 12, "alignItems": "stretch"}},
		noteNode("row", "", map[string]any{"style": map[string]any{"gap": 12, "alignItems": "center"}},
			noteNode("image", "", map[string]any{"icon": "plugin.notes", "style": map[string]any{"width": 24, "height": 24}}),
			noteNode("text", "", map[string]any{"text": "i18n:plugin_sys_woxview_note_window", "style": map[string]any{"fontSize": "$fontLg", "fontWeight": "semibold"}}),
			noteNode("spacer", "", nil),
			noteNode("button", "", map[string]any{"icon": "action.add", "label": "i18n:plugin_sys_woxview_note_new", "onPress": noteHandler("add")})),
		noteNode("row", "", map[string]any{"style": map[string]any{"gap": 16, "flex": 1, "alignItems": "stretch"}},
			noteNode("column", "", map[string]any{"style": map[string]any{"width": 220, "padding": 12, "gap": 8, "alignItems": "stretch", "backgroundColor": "$surface", "borderColor": "$border", "borderWidth": 1, "borderRadius": "$radiusMd"}},
				noteNode("row", "", map[string]any{"style": map[string]any{"alignItems": "center", "gap": 8}},
					noteNode("image", "", map[string]any{"icon": "control.folder-open", "style": map[string]any{"width": 20, "height": 20}}),
					noteNode("text", "", map[string]any{"text": "Wox", "style": map[string]any{"fontWeight": "semibold"}}),
					noteNode("spacer", "", nil),
					noteNode("icon-button", "", map[string]any{"icon": "action.add", "label": "i18n:plugin_sys_woxview_note_new", "onPress": noteHandler("add")})),
				noteNode("text-field", "search", map[string]any{"label": "i18n:plugin_sys_woxview_note_search", "placeholder": "i18n:plugin_sys_woxview_note_search", "value": s.search, "onChange": noteHandler("search")}),
				noteNode("scroll", "", map[string]any{"style": map[string]any{"flex": 1, "gap": 4}}, list...)),
			noteNode("column", "", map[string]any{"style": map[string]any{"flex": 1, "gap": 12, "padding": 8, "alignItems": "stretch"}},
				noteNode("text-field", "title-"+index, map[string]any{"label": "i18n:plugin_sys_woxview_note_title", "value": note.title, "onChange": noteHandler("title"), "style": map[string]any{"fontSize": "$fontLg"}}),
				noteNode("scroll", "", map[string]any{"style": map[string]any{"flex": 1, "gap": 12}},
					noteNode("text", "", map[string]any{"text": "i18n:plugin_sys_woxview_note_tasks", "style": map[string]any{"fontSize": "$fontTitle", "fontWeight": "semibold"}}),
					noteNode("column", "", map[string]any{"style": map[string]any{"gap": 4}}, checkboxes...),
					noteNode("text", "", map[string]any{"text": "i18n:plugin_sys_woxview_note_body", "style": map[string]any{"fontSize": "$fontLg", "fontWeight": "semibold"}}),
					noteNode("text-field", "body-"+index, map[string]any{"label": "i18n:plugin_sys_woxview_note_body", "value": note.body, "maxLines": 12, "onChange": noteHandler("body"), "style": map[string]any{"height": 260}})),
				noteNode("row", "", map[string]any{"wrap": true, "style": map[string]any{"gap": 4}}, toolbar...),
				noteNode("text", "", map[string]any{"text": "i18n:plugin_sys_woxview_note_temporary", "style": map[string]any{"color": "$textSecondary", "fontSize": "$fontSm"}}))))
	data, err := json.Marshal(map[string]any{"version": 1, "root": root})
	if err != nil {
		panic(err)
	}
	return string(data)
}
