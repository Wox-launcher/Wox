package launcher

import (
	"context"
	"reflect"
	"strings"
	"wox/common"
	woxui "wox/ui/runtime"
	"wox/util"
)

type queryHintResolver interface {
	ResolveQueryHint(context.Context, string) *common.QueryHint
}

// queryHintEditor tracks semantic ranges and document-level undo for one continuous text editor.
type queryHintEditor struct {
	active      int
	prefix      string
	candidate   *common.QueryHint
	template    *common.QueryHint
	suppressed  string
	allSelected bool
	undo        []queryHintSnapshot
	redo        []queryHintSnapshot
}

type queryHintSnapshot struct {
	template *common.QueryHint
	hint     *common.QueryHint
	text     string
	active   int
	prefix   string
}

// rememberQueryHint snapshots content before an element edit or a structural transition.
func (a *App) rememberQueryHint() {
	s := &a.queryHintEditorState
	if len(s.undo) > 0 {
		last := s.undo[len(s.undo)-1]
		if last.text == a.query.QueryText && last.active == s.active && reflect.DeepEqual(last.hint, a.query.QueryHint) {
			s.redo = nil
			return
		}
	}
	s.undo = append(s.undo, queryHintSnapshot{s.template.Clone(), a.query.QueryHint.Clone(), a.query.QueryText, s.active, s.prefix})
	if len(s.undo) > 100 {
		s.undo = s.undo[len(s.undo)-100:]
	}
	s.redo = nil
}

// installQueryHint installs semantic content without splitting the native text editor.
func (a *App) installQueryHint(hint *common.QueryHint) {
	a.queryHintEditorState.template = nil
	a.query.QueryHint = hint.Clone()
	a.query.QueryText = hint.PlainText()
	a.queryHintEditorState.active = 0
	a.queryHintEditorState.prefix = ""
	a.queryHintEditorState.candidate = nil
	if len(hint.Elements) > 0 && hint.Elements[0].Kind == common.QueryElementText {
		a.queryHintEditorState.prefix = hint.Elements[0].Text
	}
	for i, e := range hint.Elements {
		if e.Kind != common.QueryElementText {
			a.queryHintEditorState.active = i
			break
		}
	}
	a.editor.SetText(hint.PlainText(), false)
	// Installing a hint must leave a visible caret, not preselect the block.
	_, end := queryElementRange(hint, a.queryHintEditorState.active)
	a.editor.SetCaret(end)
}

// installQueryHintTemplate defers unused whitespace separators until navigation needs them.
func (a *App) installQueryHintTemplate(template *common.QueryHint) {
	hint := template.Clone()
	lastValue := 0
	for i, element := range hint.Elements {
		if element.Kind != common.QueryElementText && element.Content() != "" {
			lastValue = i
		}
	}
	for i := lastValue + 1; i < len(hint.Elements); i++ {
		element := &hint.Elements[i]
		if element.Kind == common.QueryElementText && strings.Trim(element.Text, " ") == "" {
			element.Text = ""
		}
	}
	a.installQueryHint(hint)
	a.queryHintEditorState.template = template.Clone()
}

// updateQueryHintText maps continuous edits back to semantic values, or offers a command template.
func (a *App) updateQueryHintText(text string) string {
	s := &a.queryHintEditorState
	if hint := a.query.QueryHint; hint != nil {
		a.rememberQueryHint()
		// A local edit retains its argument identity. Edits across semantic boundaries
		// keep the user's text but discard metadata that can no longer be trusted.
		old, updated := []rune(hint.PlainText()), []rune(text)
		start := 0
		for start < len(old) && start < len(updated) && old[start] == updated[start] {
			start++
		}
		end, newEnd := len(old), len(updated)
		for end > start && newEnd > start && old[end-1] == updated[newEnd-1] {
			end--
			newEnd--
		}
		if string(old) == text {
			return text
		}
		owner := -1
		for n := 0; n < len(hint.Elements); n++ {
			i := (s.active + n) % len(hint.Elements)
			left, right := queryElementRange(hint, i)
			if hint.Elements[i].Kind == common.QueryElementArgument && start >= left && end <= right {
				owner = i
				break
			}
		}
		if owner >= 0 {
			next := hint.Clone()
			left, right := queryElementRange(hint, owner)
			next.Elements[owner].Value = string(updated[left : right+len(updated)-len(old)])
			a.query.QueryHint = next
			s.active = owner
		} else {
			a.query.QueryHint = nil
			// Word deletion can cross argument separators and return exactly to the
			// context prefix. Resolve its empty template again instead of suppressing it.
			if text != s.prefix {
				s.suppressed = text
			}
		}
		s.allSelected = false
		if a.query.QueryHint != nil || text != s.prefix {
			return text
		}
	}
	s.candidate = nil
	if len(s.undo)+len(s.redo) > 0 && text != a.query.QueryText {
		a.rememberQueryHint()
	}
	if a.query.QueryType != "input" || text == s.suppressed {
		return text
	}
	s.suppressed = ""
	resolver, ok := a.services.(queryHintResolver)
	if !ok {
		return text
	}
	hint := resolver.ResolveQueryHint(a.lifecycleCtx, text)
	if hint == nil {
		return text
	}
	if !strings.HasSuffix(text, " ") {
		s.candidate = hint
		return text
	}
	a.rememberQueryHint()
	a.installQueryHintTemplate(hint)
	return a.query.QueryText
}

// queryHintChanged shares normal query invalidation without treating full text as a slot edit.
func (a *App) queryHintChanged() {
	a.beginQueryGenerationLocked()
	a.beginQueryTransitionLocked(false)
	a.completionHint = nil
	a.canRecallHistory = false
	if a.window != nil {
		_ = a.window.Invalidate()
	}
	a.reconcileSelectedPreview()
	if a.services != nil {
		if err := a.sendCurrentQuery(); err != nil {
			util.GetLogger().Error(a.lifecycleCtx, err.Error())
		}
	}
}

// focusQueryElement changes only local editing state; navigation never starts a query.
func (a *App) focusQueryElement(index int) bool {
	hint := a.query.QueryHint
	if hint == nil || a.editor.State().Composition != "" {
		return false
	}
	direction := 1
	if index < a.queryHintEditorState.active {
		direction = -1
	}
	// Tab traverses semantic values, never command text, and stops at either end.
	next := index
	for next >= 0 && next < len(hint.Elements) && hint.Elements[next].Kind == common.QueryElementText {
		next += direction
	}
	if next < 0 || next >= len(hint.Elements) {
		return false
	}
	if template := a.queryHintEditorState.template; direction > 0 && template != nil {
		updated := hint.Clone()
		for i := 1; i < next; i++ {
			if updated.Elements[i].Kind != common.QueryElementText || updated.Elements[i].Text != "" {
				continue
			}
			for _, original := range template.Elements {
				if original.Id == updated.Elements[i].Id {
					updated.Elements[i].Text = original.Text
					break
				}
			}
		}
		if updated.PlainText() != hint.PlainText() {
			a.rememberQueryHint()
			hint = updated
			a.query.QueryHint = hint
			a.query.QueryText = hint.PlainText()
			a.editor.SetText(a.query.QueryText, false)
			a.queryHintChanged()
		}
	}
	a.queryHintEditorState.active = next
	a.queryHintEditorState.allSelected = false
	start, end := queryElementRange(hint, next)
	a.editor.SetSelection(start, end)
	if a.window != nil {
		_ = a.window.Invalidate()
	}
	return true
}

// queryHintResolveActive mirrors Tab's live slot: keep the last target until the
// caret leaves it, including the shared caret point after a command prefix.
func queryHintResolveActive(hint *common.QueryHint, active int, selection woxui.TextSelection) int {
	if hint == nil || len(hint.Elements) == 0 {
		return active
	}
	if active < 0 || active >= len(hint.Elements) {
		active = queryHintFirstValueIndex(hint)
	}
	left, right := queryElementRange(hint, active)
	if selection.Start() < left || selection.End() > right {
		for i := range hint.Elements {
			start, end := queryElementRange(hint, i)
			if selection.Start() >= start && selection.End() <= end {
				return i
			}
		}
	}
	return active
}

func queryHintFirstValueIndex(hint *common.QueryHint) int {
	if hint == nil {
		return 0
	}
	for i, element := range hint.Elements {
		if element.Kind != common.QueryElementText {
			return i
		}
	}
	return 0
}

// queryHintForwardTabTarget is the next semantic value forward Tab would focus.
func queryHintForwardTabTarget(hint *common.QueryHint, active int) (int, bool) {
	if hint == nil || active < 0 || active >= len(hint.Elements) {
		return -1, false
	}
	element := hint.Elements[active]
	if element.Required && strings.TrimSpace(element.Value) == "" {
		return -1, false
	}
	next := active + 1
	for next < len(hint.Elements) && hint.Elements[next].Kind == common.QueryElementText {
		next++
	}
	if next >= len(hint.Elements) {
		return -1, false
	}
	return next, true
}

// rejectQueryTab signals an unavailable action without changing the editor or IME anchor.
func (a *App) rejectQueryTab() {
	if a.editor.State().Composition != "" {
		return
	}
	a.queryTabFeedback++
	if a.window != nil {
		_ = a.window.Invalidate()
	}
}

// onQueryHintKey handles document operations before ordinary completion and text shortcuts.
func (a *App) onQueryHintKey(event woxui.KeyEvent) bool {
	s := &a.queryHintEditorState
	if event.Modifiers.HasPrimary() && (event.Key == woxui.Key("z") || event.Key == woxui.Key("y")) && len(s.undo)+len(s.redo) > 0 {
		from, to := &s.undo, &s.redo
		if event.Key == woxui.Key("y") || event.Modifiers&woxui.KeyModifierShift != 0 {
			from, to = to, from
		}
		if len(*from) == 0 {
			return true
		}
		*to = append(*to, queryHintSnapshot{s.template.Clone(), a.query.QueryHint.Clone(), a.query.QueryText, s.active, s.prefix})
		snapshot := (*from)[len(*from)-1]
		*from = (*from)[:len(*from)-1]
		s.template = snapshot.template.Clone()
		a.query.QueryHint = snapshot.hint.Clone()
		a.query.QueryText = snapshot.text
		s.active, s.prefix, s.allSelected, s.candidate = snapshot.active, snapshot.prefix, false, nil
		text := snapshot.text
		a.editor.SetText(text, false)
		s.suppressed = snapshot.text
		a.queryHintChanged()
		return true
	}
	hint := a.query.QueryHint
	if hint == nil {
		if s.candidate != nil && event.Key == woxui.KeyTab && event.Modifiers == 0 {
			a.rememberQueryHint()
			a.installQueryHintTemplate(s.candidate)
			a.queryHintChanged()
			return true
		}
		return false
	}
	if event.Modifiers.HasPrimary() && (event.Key == woxui.Key("c") || event.Key == woxui.Key("x") || event.Key == woxui.Key("v")) {
		a.selectTouchedQueryBlocks()
	}
	if event.Key == woxui.KeyTab && (event.Modifiers == 0 || event.Modifiers == woxui.KeyModifierShift) {
		s.active = queryHintResolveActive(hint, s.active, a.editor.State().Selection)
		if event.Modifiers == 0 {
			next, ok := queryHintForwardTabTarget(hint, s.active)
			if !ok || !a.focusQueryElement(next) {
				a.rejectQueryTab()
			}
			return true
		}
		a.focusQueryElement(s.active - 1)
		return true
	}
	state := a.editor.State()
	if event.Key == woxui.KeyArrowLeft || event.Key == woxui.KeyArrowRight || event.Key == woxui.KeyHome || event.Key == woxui.KeyEnd {
		s.allSelected = false
	}
	if event.Key == woxui.Key("a") && event.Modifiers.HasPrimary() {
		a.selectEntireQuery()
		if a.window != nil {
			_ = a.window.Invalidate()
		}
		return true
	}
	if s.allSelected && (event.Key == woxui.KeyBackspace || event.Key == woxui.KeyDelete) {
		a.replaceWholeQueryHint("")
		return true
	}
	// Blocks retain atomic deletion; arguments use the ordinary editor's word and
	// character deletion, including selections crossing the command prefix.
	if event.Key == woxui.KeyBackspace || event.Key == woxui.KeyDelete {
		for i, element := range hint.Elements {
			if element.Kind != common.QueryElementBlock {
				continue
			}
			start, end := queryElementRange(hint, i)
			touches := state.Selection.Start() < end && state.Selection.End() > start
			if state.Selection.Collapsed() {
				touches = event.Key == woxui.KeyBackspace && state.Selection.Focus > start && state.Selection.Focus <= end ||
					event.Key == woxui.KeyDelete && state.Selection.Focus >= start && state.Selection.Focus < end
			}
			if touches && state.Selection.Start() >= start && state.Selection.End() <= end {
				a.rememberQueryHint()
				next := hint.Clone()
				next.Elements = append(next.Elements[:i], next.Elements[i+1:]...)
				a.query.QueryHint = next
				a.query.QueryText = next.PlainText()
				a.editor.SetText(a.query.QueryText, false)
				a.editor.SetCaret(start)
				s.active = max(0, min(i, len(next.Elements)-1))
				if len(next.Elements) == 0 {
					a.query.QueryHint = nil
				}
				a.queryHintChanged()
				return true
			}
		}
		a.selectTouchedQueryBlocks()
	}
	return false
}

// replaceWholeQueryHint intentionally leaves pasted text unparsed.
func (a *App) replaceWholeQueryHint(text string) {
	a.rememberQueryHint()
	a.query.QueryHint = nil
	a.queryHintEditorState.allSelected = false
	a.queryHintEditorState.candidate = nil
	a.queryHintEditorState.suppressed = text
	a.query.QueryText = text
	a.editor.SetText(text, false)
	a.applyQueryTextChangeLocked(text)
	a.queryHintChanged()
}

func queryPrimaryModifier() woxui.KeyModifiers {
	if woxui.KeyModifierMeta.HasPrimary() {
		return woxui.KeyModifierMeta
	}
	return woxui.KeyModifierControl
}

// selectEntireQuery selects the same continuous document used for painting and native editing.
func (a *App) selectEntireQuery() {
	a.queryHintEditorState.allSelected = a.query.QueryHint != nil
	a.editor.SelectAll()
}

// queryElementRange derives rune offsets from the public hint; offsets never
// become part of the plugin protocol or include placeholder text.
func queryElementRange(hint *common.QueryHint, index int) (start, end int) {
	for i, element := range hint.Elements {
		end = start + len([]rune(element.Content()))
		if i == index {
			return start, end
		}
		start = end
	}
	return start, start
}

// selectTouchedQueryBlocks expands edits touching atomic content without affecting
// freely editable argument ranges. The same rule applies to keyboard, IME and clipboard.
func (a *App) selectTouchedQueryBlocks() {
	if a == nil {
		return
	}
	hint := a.query.QueryHint
	if hint == nil {
		return
	}
	state := a.editor.State()
	start, end := state.Selection.Start(), state.Selection.End()
	for i, element := range hint.Elements {
		if element.Kind != common.QueryElementBlock {
			continue
		}
		left, right := queryElementRange(hint, i)
		if start < right && end > left {
			start, end = min(start, left), max(end, right)
		}
	}
	if start != state.Selection.Start() || end != state.Selection.End() {
		a.editor.SetSelection(start, end)
	}
}
