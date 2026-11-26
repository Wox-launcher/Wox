//go:build linux

package window

/*
#cgo pkg-config: xcb
#include <stdlib.h>
#include <string.h>
#include <xcb/xcb.h>

typedef struct {
	xcb_connection_t *connection;
	xcb_window_t root;
	xcb_atom_t active_window;
	xcb_atom_t wm_pid;
	xcb_atom_t stacking;
} WoxX11Focus;

static xcb_atom_t wox_x11_focus_atom(xcb_connection_t *connection, const char *name) {
	xcb_intern_atom_reply_t *reply = xcb_intern_atom_reply(connection,
		xcb_intern_atom(connection, 1, strlen(name), name), NULL);
	xcb_atom_t atom = reply ? reply->atom : XCB_ATOM_NONE;
	free(reply);
	return atom;
}

static int wox_x11_open_focus(WoxX11Focus *focus) {
	int screen = 0;
	focus->connection = xcb_connect(NULL, &screen);
	if (xcb_connection_has_error(focus->connection)) {
		return 0;
	}
	xcb_screen_iterator_t roots = xcb_setup_roots_iterator(xcb_get_setup(focus->connection));
	while (screen-- > 0 && roots.rem > 0) {
		xcb_screen_next(&roots);
	}
	if (roots.rem == 0) {
		return 0;
	}
	focus->root = roots.data->root;
	focus->active_window = wox_x11_focus_atom(focus->connection, "_NET_ACTIVE_WINDOW");
	focus->wm_pid = wox_x11_focus_atom(focus->connection, "_NET_WM_PID");
	focus->stacking = wox_x11_focus_atom(focus->connection, "_NET_CLIENT_LIST_STACKING");
	return focus->active_window != XCB_ATOM_NONE;
}

static uint32_t wox_x11_focus_value(WoxX11Focus *focus, xcb_get_property_cookie_t cookie, xcb_atom_t type) {
	// XCB keeps BadWindow (a client that closed after enumeration) on this request.
	xcb_get_property_reply_t *reply = xcb_get_property_reply(focus->connection, cookie, NULL);
	uint32_t value = 0;
	if (reply && reply->type == type && reply->format == 32 && reply->value_len == 1) {
		value = *(uint32_t *)xcb_get_property_value(reply);
	}
	free(reply);
	return value;
}

static xcb_window_t wox_x11_active_window(WoxX11Focus *focus) {
	return wox_x11_focus_value(focus,
		xcb_get_property(focus->connection, 0, focus->root, focus->active_window, XCB_ATOM_WINDOW, 0, 1),
		XCB_ATOM_WINDOW);
}

static xcb_get_property_cookie_t wox_x11_query_window_pid(WoxX11Focus *focus, xcb_window_t window) {
	return xcb_get_property(focus->connection, 0, window, focus->wm_pid, XCB_ATOM_CARDINAL, 0, 1);
}

static uint32_t wox_x11_window_pid(WoxX11Focus *focus, xcb_window_t window) {
	if (window == XCB_NONE || focus->wm_pid == XCB_ATOM_NONE) {
		return 0;
	}
	return wox_x11_focus_value(focus, wox_x11_query_window_pid(focus, window), XCB_ATOM_CARDINAL);
}

static xcb_window_t wox_x11_focus_window_for_pid(WoxX11Focus *focus, uint32_t pid) {
	if (focus->stacking == XCB_ATOM_NONE || focus->wm_pid == XCB_ATOM_NONE) {
		return XCB_NONE;
	}
	xcb_get_property_reply_t *reply = xcb_get_property_reply(focus->connection,
		xcb_get_property(focus->connection, 0, focus->root, focus->stacking, XCB_ATOM_WINDOW, 0, UINT32_MAX),
		NULL);
	if (!reply || reply->type != XCB_ATOM_WINDOW || reply->format != 32 || reply->value_len == 0) {
		free(reply);
		return XCB_NONE;
	}
	const xcb_window_t *windows = xcb_get_property_value(reply);
	size_t count = reply->value_len;
	xcb_get_property_cookie_t *requests = calloc(count, sizeof(*requests));
	xcb_window_t target = XCB_NONE;
	if (requests) {
		// Queue all PID reads together so a large desktop needs one round trip.
		for (size_t i = 0; i < count; i++) {
			requests[i] = wox_x11_query_window_pid(focus, windows[count - i - 1]);
		}
		for (size_t i = 0; i < count; i++) {
			if (target != XCB_NONE) {
				xcb_discard_reply(focus->connection, requests[i].sequence);
			} else if (wox_x11_focus_value(focus, requests[i], XCB_ATOM_CARDINAL) == pid) {
				target = windows[count - i - 1];
			}
		}
		free(requests);
	}
	free(reply);
	return target;
}

static int wox_x11_request_window_focus(WoxX11Focus *focus, xcb_window_t id) {
	xcb_client_message_event_t event = {0};
	event.response_type = XCB_CLIENT_MESSAGE;
	event.window = id;
	event.type = focus->active_window;
	event.format = 32;
	event.data.data32[0] = 2; // Pager or task switcher request.
	xcb_generic_error_t *error = xcb_request_check(focus->connection,
		xcb_send_event_checked(focus->connection, 0, focus->root,
			XCB_EVENT_MASK_SUBSTRUCTURE_REDIRECT | XCB_EVENT_MASK_SUBSTRUCTURE_NOTIFY,
			(const char *)&event));
	int accepted = !error && !xcb_connection_has_error(focus->connection);
	free(error);
	return accepted;
}
*/
import "C"

import (
	"context"
	"strconv"
	"strings"
)

// focusCapturedX11Window confirms activation on one X connection without launching xprop.
func focusCapturedX11Window(ctx context.Context, id string, pid int) error {
	var target C.xcb_window_t
	if id != "" {
		xid, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(id), "0x"), 16, 32)
		if err != nil || xid == 0 {
			return ErrWindowManagementWindowNotFound
		}
		target = C.xcb_window_t(xid)
	}
	var focus C.WoxX11Focus
	opened := C.wox_x11_open_focus(&focus)
	defer C.xcb_disconnect(focus.connection)
	if opened == 0 {
		return ErrWindowManagementUnsupported
	}

	matches := func() bool {
		active := C.wox_x11_active_window(&focus)
		return active != 0 && (target == 0 || active == target) &&
			(pid <= 0 || C.wox_x11_window_pid(&focus, active) == C.uint32_t(pid))
	}
	if matches() {
		return ctx.Err()
	}
	if target == 0 {
		target = C.wox_x11_focus_window_for_pid(&focus, C.uint32_t(pid))
	} else if pid > 0 && C.wox_x11_window_pid(&focus, target) != C.uint32_t(pid) {
		return ErrWindowManagementWindowNotFound
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if target == 0 {
		return ErrWindowManagementWindowNotFound
	}
	if C.wox_x11_request_window_focus(&focus, target) == 0 {
		return ErrWindowManagementUnsupported
	}
	return waitForForeground(ctx, matches)
}
