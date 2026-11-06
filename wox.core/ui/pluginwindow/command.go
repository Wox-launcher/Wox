package pluginwindow

import (
	"encoding/json"
	"errors"

	"wox/plugin"
	"wox/ui/woxview"
)

// ContentCommand validates and queues on the UI thread so document replacement
// cannot race the guard. Hidden windows execute when they next receive a frame.
func (s *session) ContentCommand(o plugin.WindowContentCommandOption) error {
	return s.call(func() error {
		if err := s.enqueueContentCommand(o); err != nil {
			return err
		}
		return s.window.Invalidate()
	})
}

// enqueueContentCommand adapts public protocol options to the UI-only operation.
// Deferred failures use the existing document-scoped error event; replacing or
// closing a document drops its pending operations with the old View.
func (s *session) enqueueContentCommand(o plugin.WindowContentCommandOption) error {
	if s.closed {
		return errors.New("window closed")
	}
	if o.DocumentID != "" && o.DocumentID != s.document {
		return errors.New("document replaced")
	}
	if s.content.Type != plugin.WindowContentWoxView || s.view == nil {
		return errors.New("content commands require woxview content")
	}
	if (o.Command == plugin.WindowContentCommandFocus || o.Command == plugin.WindowContentCommandSelectAll) && !boolValue(s.options.Focusable, false) {
		return errors.New("content focus commands require a focusable window")
	}
	document, view := s.document, s.view
	return view.EnqueueCommand(s.host, woxview.Command(o.Command), o.Key, func(err error) {
		if err == nil || s.closed || s.document != document || s.view != view {
			return
		}
		data, _ := json.Marshal(struct {
			Command plugin.WindowContentCommand
			Key     string
		}{o.Command, o.Key})
		s.emit(plugin.WindowEvent{Type: plugin.WindowEventError, DocumentID: document, Data: data, ErrMsg: err.Error()})
	})
}
