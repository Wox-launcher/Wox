package dictationoverlay

import (
	"sync"

	"wox/util/overlay"
)

const (
	dictationOverlayWidth         = 132
	dictationOverlayHeight        = 48
	dictationOverlayContentHeight = 24
)

// Options configures the dictation overlay.
type Options struct {
	Window   overlay.WindowOptions
	Active   bool
	Closable bool
}

type dictationOverlayRenderer struct {
	handle uintptr
}

var renderers = struct {
	sync.Mutex
	byID map[string]*dictationOverlayRenderer
}{
	byID: make(map[string]*dictationOverlayRenderer),
}

// Show attaches the dictation overlay renderer to the base overlay window.
func Show(opts Options) {
	base := opts.Window
	overlay.RegisterClickCallback(base.ID, nil)
	renderer := rendererForID(base.ID, opts.Closable)
	if renderer == nil {
		overlay.ShowWindow(base)
		return
	}

	renderer.setActive(opts.Active)
	attachment := renderer.nativeAttachment()
	attachment.OnRelease = func() {
		Release(base.ID)
	}
	base.NativeAttachment = attachment

	overlay.ShowWindow(base)
}

// UpdateActive updates the voice activity animation state without refreshing the base overlay.
func UpdateActive(id string, active bool) {
	renderers.Lock()
	renderer := renderers.byID[id]
	renderers.Unlock()
	if renderer == nil {
		return
	}
	renderer.setActive(active)
}

// Release destroys the identified dictation overlay renderer while leaving the overlay window alone.
func Release(id string) {
	renderers.Lock()
	renderer := renderers.byID[id]
	if renderer != nil {
		delete(renderers.byID, id)
	}
	renderers.Unlock()

	if renderer != nil {
		renderer.destroy()
	}
}

// Close closes the base overlay. The overlay core releases the attached dictation overlay renderer.
func Close(id string) {
	overlay.Close(id)
}

func rendererForID(id string, closable bool) *dictationOverlayRenderer {
	renderers.Lock()
	defer renderers.Unlock()

	if renderer := renderers.byID[id]; renderer != nil {
		return renderer
	}

	renderer, ok := newDictationOverlayRenderer(id, closable)
	if !ok {
		return nil
	}
	renderers.byID[id] = renderer
	return renderer
}
