//go:build linux && cgo

package selection

/*
#cgo pkg-config: gtk+-3.0 x11 xtst
#include <gtk/gtk.h>
#include <gdk/gdk.h>
#include <X11/Xlib.h>
#include <X11/keysym.h>
#include <X11/extensions/XTest.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

static int woxEnsureGtkInitialized() {
	static gsize state = 0;

	if (g_once_init_enter(&state)) {
		int ok = gtk_init_check(NULL, NULL) ? 1 : 2;
		g_once_init_leave(&state, ok);
	}

	return state == 1;
}

static GtkClipboard* woxGetClipboard(const char* selectionName) {
	if (!woxEnsureGtkInitialized()) {
		return NULL;
	}

	GdkAtom atom = gdk_atom_intern_static_string(selectionName);
	return gtk_clipboard_get(atom);
}

// woxGetWaylandDisplay opens a GDK Wayland display alongside the existing X11
// display. GTK3 on Ubuntu supports multiple GDK backends simultaneously; a
// display name that matches the Wayland socket (e.g. "wayland-0") is recognised
// by the Wayland GDK backend even when GTK was initialised with X11.
// The returned display is opened once and reused for the lifetime of the process.
static GdkDisplay* woxGetWaylandDisplay() {
	static GdkDisplay* wl_disp = NULL;
	static gsize once = 0;

	if (g_once_init_enter(&once)) {
		if (woxEnsureGtkInitialized()) {
			const char* name = getenv("WAYLAND_DISPLAY");
			if (name == NULL) name = "wayland-0";
			// gdk_display_open recognises the Wayland socket name and uses the
			// Wayland GDK backend, which connects to the compositor directly.
			wl_disp = gdk_display_open(name);
		}
		g_once_init_leave(&once, 1);
	}
	return wl_disp;
}

// woxReadWaylandPrimaryText reads the primary selection from the Wayland
// compositor via a dedicated GDK Wayland display. This reaches the compositor's
// zwp_primary_selection_v1 protocol directly and works for all native Wayland
// applications, even when the main GTK instance uses the X11 GDK backend.
static char* woxReadWaylandPrimaryText() {
	GdkDisplay* disp = woxGetWaylandDisplay();
	if (disp == NULL) {
		return NULL;
	}

	GtkClipboard* clipboard = gtk_clipboard_get_for_display(disp, GDK_SELECTION_PRIMARY);
	if (clipboard == NULL) {
		return NULL;
	}

	gchar* text = gtk_clipboard_wait_for_text(clipboard);
	if (text == NULL) {
		return NULL;
	}

	char* result = strdup(text);
	g_free(text);
	return result;
}

static char* woxReadClipboardText(const char* selectionName) {
	GtkClipboard* clipboard = woxGetClipboard(selectionName);
	if (clipboard == NULL) {
		return NULL;
	}

	gchar* text = gtk_clipboard_wait_for_text(clipboard);
	if (text == NULL) {
		return NULL;
	}

	char* result = strdup(text);
	g_free(text);
	return result;
}

static char* woxReadClipboardPaths(const char* selectionName) {
	GtkClipboard* clipboard = woxGetClipboard(selectionName);
	if (clipboard == NULL) {
		return NULL;
	}

	gchar** uris = gtk_clipboard_wait_for_uris(clipboard);
	if (uris == NULL) {
		return NULL;
	}

	GString* builder = g_string_new(NULL);
	for (int i = 0; uris[i] != NULL; i++) {
		gchar* path = g_filename_from_uri(uris[i], NULL, NULL);
		if (path == NULL) {
			continue;
		}

		if (builder->len > 0) {
			g_string_append_c(builder, '\n');
		}
		g_string_append(builder, path);
		g_free(path);
	}

	g_strfreev(uris);
	if (builder->len == 0) {
		g_string_free(builder, TRUE);
		return NULL;
	}

	return g_string_free(builder, FALSE);
}

static int woxAreModifierKeysReleased(Display* display) {
	char keymap[32];
	KeySym modifierSyms[] = {
		XK_Control_L, XK_Control_R,
		XK_Shift_L, XK_Shift_R,
		XK_Alt_L, XK_Alt_R,
		XK_Super_L, XK_Super_R,
		XK_Meta_L, XK_Meta_R,
	};

	XQueryKeymap(display, keymap);
	for (unsigned long i = 0; i < sizeof(modifierSyms) / sizeof(modifierSyms[0]); i++) {
		KeyCode keycode = XKeysymToKeycode(display, modifierSyms[i]);
		if (keycode == 0) {
			continue;
		}

		if (keymap[keycode / 8] & (1 << (keycode % 8))) {
			return 0;
		}
	}

	return 1;
}

static const char* woxSimulateCtrlCOnX11() {
	Display* display = XOpenDisplay(NULL);
	if (display == NULL) {
		return "failed to open X11 display";
	}

	for (int attempt = 0; attempt < 20; attempt++) {
		if (woxAreModifierKeysReleased(display)) {
			break;
		}
		usleep(50 * 1000);
	}

	KeyCode ctrlCode = XKeysymToKeycode(display, XK_Control_L);
	KeyCode cCode = XKeysymToKeycode(display, XK_c);
	if (ctrlCode == 0 || cCode == 0) {
		XCloseDisplay(display);
		return "failed to resolve X11 keycodes for Ctrl+C";
	}

	if (!XTestFakeKeyEvent(display, ctrlCode, True, CurrentTime) ||
		!XTestFakeKeyEvent(display, cCode, True, CurrentTime) ||
		!XTestFakeKeyEvent(display, cCode, False, CurrentTime) ||
		!XTestFakeKeyEvent(display, ctrlCode, False, CurrentTime)) {
		XCloseDisplay(display);
		return "failed to send Ctrl+C through XTest";
	}

	XFlush(display);
	XSync(display, False);
	XCloseDisplay(display);
	return NULL;
}
*/
import "C"

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unsafe"
	"wox/util"
	"wox/util/keyboard"
)

const (
	linuxPrimarySelection   = "PRIMARY"
	linuxClipboardSelection = "CLIPBOARD"
	linuxSelectionPollCount = 10
	linuxSelectionPollDelay = 50 * time.Millisecond
)

func GetSelected(ctx context.Context) (Selection, error) {
	// Try X11 PRIMARY selection first. This works for XWayland apps and X11 sessions.
	if text, err := readLinuxSelectionText(linuxPrimarySelection); err == nil && text != "" {
		util.GetLogger().Debug(ctx, "selection: Successfully got text via PRIMARY selection")
		return Selection{
			Type: SelectionTypeText,
			Text: text,
		}, nil
	}

	if keyboard.IsWaylandSession() {
		// X11 PRIMARY did not contain a selection. On Wayland, native apps publish
		// their selections via the zwp_primary_selection_v1 Wayland protocol, which
		// is NOT visible to X11 clients when GTK is using the X11 backend.
		// Open a second GDK connection using the Wayland backend (GTK3 supports
		// both backends simultaneously) and read the Wayland primary selection from
		// there. This requires no external tools or additional packages.
		if text, err := readWaylandPrimarySelectionNative(ctx); err == nil && text != "" {
			util.GetLogger().Debug(ctx, "selection: Successfully got text via Wayland primary selection (GDK Wayland)")
			return Selection{
				Type: SelectionTypeText,
				Text: text,
			}, nil
		}

		return Selection{}, fmt.Errorf("%w: linux fallback selection is unavailable on Wayland when PRIMARY selection is empty", ErrSelectionUnsupported)
	}

	util.GetLogger().Debug(ctx, "selection: Falling back to X11 simulated Ctrl+C")
	return getSelectedByX11Clipboard(ctx)
}

// readWaylandPrimarySelectionNative reads the current primary selection on a
// Wayland session by opening a dedicated GDK Wayland display (separate from
// the X11 GDK display used by the rest of the process). GTK3 on Ubuntu 24.04
// supports both X11 and Wayland GDK backends simultaneously, so we can open a
// second display connection pointing at the Wayland socket and read the
// compositor's primary selection from it. No external tools are required.
func readWaylandPrimarySelectionNative(ctx context.Context) (string, error) {
	cText := C.woxReadWaylandPrimaryText()
	if cText == nil {
		util.GetLogger().Debug(ctx, "selection: GDK Wayland primary selection returned nil (no selection or backend unavailable)")
		return "", noSelection
	}
	defer C.free(unsafe.Pointer(cText))
	return C.GoString(cText), nil
}

func getSelectedByX11Clipboard(ctx context.Context) (Selection, error) {
	clipboardBefore, beforeErr := readLinuxClipboardSelection(linuxClipboardSelection)

	if err := simulateLinuxCopyOnX11(); err != nil {
		return Selection{}, err
	}

	var lastReadErr error
	for attempt := 0; attempt < linuxSelectionPollCount; attempt++ {
		time.Sleep(linuxSelectionPollDelay)

		clipboardAfter, err := readLinuxClipboardSelection(linuxClipboardSelection)
		if err != nil {
			lastReadErr = err
			continue
		}

		if !selectionEquals(clipboardBefore, clipboardAfter) || beforeErr != nil {
			if clipboardAfter.IsEmpty() {
				continue
			}
			return clipboardAfter, nil
		}
	}

	if lastReadErr != nil && beforeErr != nil {
		return Selection{}, lastReadErr
	}

	return Selection{}, noSelection
}

func readLinuxClipboardSelection(selectionName string) (Selection, error) {
	filePaths, fileErr := readLinuxSelectionPaths(selectionName)
	if fileErr == nil && len(filePaths) > 0 {
		return Selection{
			Type:      SelectionTypeFile,
			FilePaths: filePaths,
		}, nil
	}

	text, textErr := readLinuxSelectionText(selectionName)
	if textErr == nil {
		return Selection{
			Type: SelectionTypeText,
			Text: text,
		}, nil
	}

	if fileErr != nil {
		return Selection{}, fileErr
	}

	return Selection{}, textErr
}

func readLinuxSelectionText(selectionName string) (string, error) {
	cSelectionName := C.CString(selectionName)
	defer C.free(unsafe.Pointer(cSelectionName))

	cText := C.woxReadClipboardText(cSelectionName)
	if cText == nil {
		return "", noSelection
	}
	defer C.free(unsafe.Pointer(cText))

	return C.GoString(cText), nil
}

func readLinuxSelectionPaths(selectionName string) ([]string, error) {
	cSelectionName := C.CString(selectionName)
	defer C.free(unsafe.Pointer(cSelectionName))

	cPaths := C.woxReadClipboardPaths(cSelectionName)
	if cPaths == nil {
		return nil, noSelection
	}
	defer C.free(unsafe.Pointer(cPaths))

	paths := strings.Split(C.GoString(cPaths), "\n")
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		filtered = append(filtered, path)
	}

	if len(filtered) == 0 {
		return nil, noSelection
	}

	return filtered, nil
}

func simulateLinuxCopyOnX11() error {
	errText := C.woxSimulateCtrlCOnX11()
	if errText != nil {
		return fmt.Errorf("failed to send Ctrl+C on X11: %s", C.GoString(errText))
	}

	return nil
}

func selectionEquals(left Selection, right Selection) bool {
	if left.Type != right.Type {
		return false
	}

	switch left.Type {
	case SelectionTypeText:
		return left.Text == right.Text
	case SelectionTypeFile:
		if len(left.FilePaths) != len(right.FilePaths) {
			return false
		}

		for index := range left.FilePaths {
			if left.FilePaths[index] != right.FilePaths[index] {
				return false
			}
		}
		return true
	default:
		return left.IsEmpty() && right.IsEmpty()
	}
}
