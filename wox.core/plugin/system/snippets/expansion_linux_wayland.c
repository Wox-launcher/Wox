//go:build linux && cgo

#include <errno.h>
#include <fcntl.h>
#include <glob.h>
#include <linux/input.h>
#include <linux/uinput.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/epoll.h>
#include <sys/inotify.h>
#include <sys/ioctl.h>
#include <sys/mman.h>
#include <unistd.h>
#include <wayland-client.h>
#include <xkbcommon/xkbcommon.h>
#include <xkbcommon/xkbcommon-keysyms.h>
#include "compose_linux.h"

extern void woxSnippetWaylandEvent(uintptr_t callback, int kind, int down, unsigned int key, unsigned int character);

#define SNIPPET_MAX_DEVICES 128
#define SNIPPET_DEVICE_NAME "Wox Snippets Virtual Keyboard"
#define SNIPPET_BIT(array, bit) ((array)[(bit) / 8] & (1u << ((bit) % 8)))

typedef struct SnippetInputDevice {
    int fd;
    int keyboard;
    int pointer;
    int desynchronized;
    char path[256];
} SnippetInputDevice;

typedef struct SnippetWayland {
    struct wl_display *display;
    struct wl_registry *registry;
    struct wl_seat *seat;
    struct wl_keyboard *keyboard;
    struct xkb_context *xkb;
    struct xkb_keymap *keymap;
    struct xkb_state *keys;
    SnippetCompose *compose;
    uint32_t group;
    int group_valid;
    int epoll_fd;
    int inotify_fd;
    int output_fd;
    int device_count;
    SnippetInputDevice devices[SNIPPET_MAX_DEVICES];
    uintptr_t callback;
} SnippetWayland;

static void snippetWaylandResetKeyboard(SnippetWayland *state);
int snippetWaylandSetGroup(SnippetWayland *state, unsigned int group);

static void snippetKeyboardKeymap(void *data, struct wl_keyboard *keyboard, uint32_t format, int32_t fd, uint32_t size) {
    SnippetWayland *state = data;
    // Never continue with a stale layout after the compositor announces a change.
    if (state->keys) xkb_state_unref(state->keys);
    if (state->keymap) xkb_keymap_unref(state->keymap);
    state->keys = NULL;
    state->keymap = NULL;
    state->group_valid = 0;
    if (state->callback) woxSnippetWaylandEvent(state->callback, 4, 1, 0, 0);
    if (format != WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1 || size == 0) { close(fd); return; }
    void *mapping = mmap(NULL, size, PROT_READ, MAP_PRIVATE, fd, 0);
    close(fd);
    if (mapping == MAP_FAILED) return;
    struct xkb_keymap *keymap = xkb_keymap_new_from_string(state->xkb, mapping, XKB_KEYMAP_FORMAT_TEXT_V1, XKB_KEYMAP_COMPILE_NO_FLAGS);
    munmap(mapping, size);
    if (!keymap) return;
    state->keymap = keymap;
    state->keys = xkb_state_new(keymap);
    snippetWaylandResetKeyboard(state);
    state->group = 0;
    state->group_valid = state->keys && xkb_keymap_num_layouts(keymap) == 1;
}

static void snippetKeyboardEnter(void *data, struct wl_keyboard *keyboard, uint32_t serial, struct wl_surface *surface, struct wl_array *keys) {}
static void snippetKeyboardLeave(void *data, struct wl_keyboard *keyboard, uint32_t serial, struct wl_surface *surface) {}
static void snippetKeyboardKey(void *data, struct wl_keyboard *keyboard, uint32_t serial, uint32_t time, uint32_t key, uint32_t keyState) {}
static void snippetKeyboardModifiers(void *data, struct wl_keyboard *keyboard, uint32_t serial, uint32_t depressed, uint32_t latched, uint32_t locked, uint32_t group) {}
static void snippetKeyboardRepeat(void *data, struct wl_keyboard *keyboard, int32_t rate, int32_t delay) {}

static const struct wl_keyboard_listener snippetKeyboardListener = {
    .keymap = snippetKeyboardKeymap,
    .enter = snippetKeyboardEnter,
    .leave = snippetKeyboardLeave,
    .key = snippetKeyboardKey,
    .modifiers = snippetKeyboardModifiers,
    .repeat_info = snippetKeyboardRepeat,
};

static void snippetSeatCapabilities(void *data, struct wl_seat *seat, uint32_t capabilities) {
    SnippetWayland *state = data;
    if ((capabilities & WL_SEAT_CAPABILITY_KEYBOARD) && !state->keyboard) {
        state->keyboard = wl_seat_get_keyboard(seat);
        wl_keyboard_add_listener(state->keyboard, &snippetKeyboardListener, state);
    }
}

static void snippetSeatName(void *data, struct wl_seat *seat, const char *name) {}
static const struct wl_seat_listener snippetSeatListener = { .capabilities = snippetSeatCapabilities, .name = snippetSeatName };

static void snippetRegistryGlobal(void *data, struct wl_registry *registry, uint32_t name, const char *interface, uint32_t version) {
    SnippetWayland *state = data;
    if (!state->seat && strcmp(interface, wl_seat_interface.name) == 0) {
        state->seat = wl_registry_bind(registry, name, &wl_seat_interface, version < 7 ? version : 7);
        wl_seat_add_listener(state->seat, &snippetSeatListener, state);
    }
}

static void snippetRegistryRemove(void *data, struct wl_registry *registry, uint32_t name) {}
static const struct wl_registry_listener snippetRegistryListener = { .global = snippetRegistryGlobal, .global_remove = snippetRegistryRemove };

static void snippetWaylandResetKeyboard(SnippetWayland *state) {
    if (!state->keys) return;
    for (int i = 0; i < state->device_count; i++) {
        if (!state->devices[i].keyboard) continue;
        unsigned char down[(KEY_MAX + 8) / 8] = {0};
        unsigned char leds[(LED_MAX + 8) / 8] = {0};
        if (ioctl(state->devices[i].fd, EVIOCGLED(sizeof(leds)), leds) >= 0 && SNIPPET_BIT(leds, LED_CAPSL)) {
            xkb_mod_index_t caps = xkb_keymap_mod_get_index(state->keymap, XKB_MOD_NAME_CAPS);
            if (caps != XKB_MOD_INVALID && caps < 32) xkb_state_update_mask(state->keys, 0, 0, 1u << caps, 0, 0, 0);
        }
        if (ioctl(state->devices[i].fd, EVIOCGKEY(sizeof(down)), down) < 0) continue;
        const int modifiers[] = {KEY_LEFTCTRL, KEY_RIGHTCTRL, KEY_LEFTSHIFT, KEY_RIGHTSHIFT,
                                 KEY_LEFTALT, KEY_RIGHTALT, KEY_LEFTMETA, KEY_RIGHTMETA};
        for (unsigned int n = 0; n < sizeof(modifiers) / sizeof(modifiers[0]); n++) {
            if (SNIPPET_BIT(down, modifiers[n])) xkb_state_update_key(state->keys, modifiers[n] + 8, XKB_KEY_DOWN);
        }
    }
    if (state->group_valid) snippetWaylandSetGroup(state, state->group);
}

static void snippetWaylandScanDevices(SnippetWayland *state) {
    glob_t paths = {0};
    if (glob("/dev/input/event*", 0, NULL, &paths) != 0) { globfree(&paths); return; }
    for (size_t p = 0; p < paths.gl_pathc && state->device_count < SNIPPET_MAX_DEVICES; p++) {
        const char *path = paths.gl_pathv[p];
        int found = 0;
        for (int i = 0; i < state->device_count; i++) {
            if (strcmp(state->devices[i].path, path) == 0) { found = 1; break; }
        }
        if (found) continue;
        int fd = open(path, O_RDONLY | O_NONBLOCK | O_CLOEXEC);
        if (fd < 0) continue;
        char name[256] = {0};
        if (ioctl(fd, EVIOCGNAME(sizeof(name)), name) >= 0 && strcmp(name, SNIPPET_DEVICE_NAME) == 0) { close(fd); continue; }
        unsigned char bits[(KEY_MAX + 8) / 8] = {0};
        if (ioctl(fd, EVIOCGBIT(EV_KEY, sizeof(bits)), bits) < 0) { close(fd); continue; }
        int keyboard = SNIPPET_BIT(bits, KEY_A) && SNIPPET_BIT(bits, KEY_SPACE);
        int pointer = SNIPPET_BIT(bits, BTN_LEFT);
        if (!keyboard && !pointer) { close(fd); continue; }
        struct epoll_event event = { .events = EPOLLIN | EPOLLERR | EPOLLHUP, .data.fd = fd };
        if (epoll_ctl(state->epoll_fd, EPOLL_CTL_ADD, fd, &event) < 0) { close(fd); continue; }
        SnippetInputDevice *device = &state->devices[state->device_count++];
        device->fd = fd;
        device->keyboard = keyboard;
        device->pointer = pointer;
        device->desynchronized = 0;
        snprintf(device->path, sizeof(device->path), "%s", path);
    }
    globfree(&paths);
}

static int snippetWaylandOpenOutput(void) {
    int fd = open("/dev/uinput", O_WRONLY | O_NONBLOCK | O_CLOEXEC);
    if (fd < 0) return -1;
    if (ioctl(fd, UI_SET_EVBIT, EV_KEY) < 0) { close(fd); return -1; }
    // Advertise a normal keyboard so compositors recognize the virtual device.
    for (int key = KEY_ESC; key <= KEY_F12; key++) {
        if (ioctl(fd, UI_SET_KEYBIT, key) < 0) { close(fd); return -1; }
    }
    struct uinput_setup setup = {0};
    snprintf(setup.name, sizeof(setup.name), "%s", SNIPPET_DEVICE_NAME);
    setup.id.bustype = BUS_USB;
    setup.id.vendor = 0x574f;
    setup.id.product = 0x5853;
    setup.id.version = 1;
    if (ioctl(fd, UI_DEV_SETUP, &setup) < 0 || ioctl(fd, UI_DEV_CREATE) < 0) { close(fd); return -1; }
    return fd;
}

void snippetWaylandClose(SnippetWayland *state);

SnippetWayland *snippetWaylandOpen(int *failure) {
    if (failure) *failure = 1;
    SnippetWayland *state = calloc(1, sizeof(*state));
    if (!state) return NULL;
    state->epoll_fd = state->inotify_fd = state->output_fd = -1;
    state->xkb = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
    state->compose = snippetComposeOpen();
    state->display = wl_display_connect(NULL);
    if (!state->xkb || !state->compose || !state->display) { snippetWaylandClose(state); return NULL; }
    state->registry = wl_display_get_registry(state->display);
    wl_registry_add_listener(state->registry, &snippetRegistryListener, state);
    if (wl_display_roundtrip(state->display) < 0 || wl_display_roundtrip(state->display) < 0 ||
        wl_display_roundtrip(state->display) < 0 || !state->keys) {
        if (failure) *failure = 2;
        snippetWaylandClose(state);
        return NULL;
    }
    state->epoll_fd = epoll_create1(EPOLL_CLOEXEC);
    state->inotify_fd = inotify_init1(IN_NONBLOCK | IN_CLOEXEC);
    if (state->epoll_fd < 0 || state->inotify_fd < 0 || inotify_add_watch(state->inotify_fd, "/dev/input", IN_CREATE | IN_DELETE | IN_MOVED_FROM | IN_MOVED_TO | IN_ATTRIB) < 0) {
        if (failure) *failure = 4;
        snippetWaylandClose(state);
        return NULL;
    }
    struct epoll_event displayEvent = { .events = EPOLLIN, .data.fd = wl_display_get_fd(state->display) };
    struct epoll_event inputEvent = { .events = EPOLLIN, .data.fd = state->inotify_fd };
    if (epoll_ctl(state->epoll_fd, EPOLL_CTL_ADD, displayEvent.data.fd, &displayEvent) < 0 ||
        epoll_ctl(state->epoll_fd, EPOLL_CTL_ADD, inputEvent.data.fd, &inputEvent) < 0) {
        snippetWaylandClose(state);
        return NULL;
    }
    snippetWaylandScanDevices(state);
    int keyboards = 0;
    for (int i = 0; i < state->device_count; i++) keyboards += state->devices[i].keyboard;
    if (!keyboards) {
        if (failure) *failure = 4;
        snippetWaylandClose(state);
        return NULL;
    }
    snippetWaylandResetKeyboard(state);
    state->output_fd = snippetWaylandOpenOutput();
    if (state->output_fd < 0) {
        if (failure) *failure = 3;
        snippetWaylandClose(state);
        return NULL;
    }
    return state;
}

static int snippetWaylandModifier(xkb_keysym_t sym) {
    return sym == XKB_KEY_Shift_L || sym == XKB_KEY_Shift_R || sym == XKB_KEY_Control_L || sym == XKB_KEY_Control_R ||
           sym == XKB_KEY_Alt_L || sym == XKB_KEY_Alt_R || sym == XKB_KEY_Super_L || sym == XKB_KEY_Super_R || sym == XKB_KEY_ISO_Level3_Shift;
}

static void snippetWaylandKey(SnippetWayland *state, struct input_event event) {
    if (!state->keys || event.value < 0 || event.value > 2) return;
    xkb_keycode_t code = event.code + 8;
    if (!state->group_valid) {
        if (event.value == 1) xkb_state_update_key(state->keys, code, XKB_KEY_DOWN);
        else if (event.value == 0) xkb_state_update_key(state->keys, code, XKB_KEY_UP);
        woxSnippetWaylandEvent(state->callback, 3, event.value != 0, event.code, 0);
        return;
    }
    xkb_keysym_t sym = xkb_state_key_get_one_sym(state->keys, code);
    int blocked = xkb_state_mod_name_is_active(state->keys, XKB_MOD_NAME_CTRL, XKB_STATE_MODS_EFFECTIVE) > 0 ||
                  xkb_state_mod_name_is_active(state->keys, XKB_MOD_NAME_ALT, XKB_STATE_MODS_EFFECTIVE) > 0 ||
                  xkb_state_mod_name_is_active(state->keys, XKB_MOD_NAME_LOGO, XKB_STATE_MODS_EFFECTIVE) > 0;
    unsigned int character = 0;
    int kind = 3;
    int composing = event.value != 0 && snippetComposeFeed(state->compose, sym);
    if (snippetWaylandModifier(sym)) kind = 2;
    else if (composing) kind = 3;
    else if (blocked) kind = 3;
    else if (event.code == KEY_BACKSPACE) kind = 1;
    else {
        char text[8] = {0};
        int length = xkb_state_key_get_utf8(state->keys, code, text, sizeof(text));
        if (length == 1 && text[0] >= 0x20 && text[0] <= 0x7e) { kind = 0; character = (unsigned char)text[0]; }
    }
    if (event.value == 1) xkb_state_update_key(state->keys, code, XKB_KEY_DOWN);
    else if (event.value == 0) xkb_state_update_key(state->keys, code, XKB_KEY_UP);
    if (xkb_keymap_num_layouts(state->keymap) > 1 &&
        xkb_state_serialize_layout(state->keys, XKB_STATE_LAYOUT_EFFECTIVE) != state->group) {
        state->group_valid = 0;
        kind = 3;
        character = 0;
    }
    woxSnippetWaylandEvent(state->callback, kind, event.value != 0, event.code, character);
}

// EV_SYN/SYN_DROPPED invalidates queued events until that device's next SYN_REPORT.
static void snippetWaylandDeviceEvent(SnippetWayland *state, SnippetInputDevice *device, struct input_event event) {
    if (event.type == EV_SYN && event.code == SYN_DROPPED) {
        device->desynchronized = 1;
        woxSnippetWaylandEvent(state->callback, 3, 1, 0, 0);
        return;
    }
    if (device->desynchronized && event.type == EV_SYN && event.code == SYN_REPORT) {
        device->desynchronized = 0;
        if (state->keys) { xkb_state_unref(state->keys); state->keys = xkb_state_new(state->keymap); }
        snippetWaylandResetKeyboard(state);
        woxSnippetWaylandEvent(state->callback, 3, 1, 0, 0);
        return;
    }
    // A lost modifier from any keyboard makes events from every device ambiguous.
    for (int i = 0; i < state->device_count; i++) {
        if (state->devices[i].desynchronized) return;
    }
    if (device->pointer && ((event.type == EV_KEY && event.code >= BTN_MOUSE && event.value == 1) ||
                            (event.type == EV_REL && (event.code == REL_WHEEL || event.code == REL_HWHEEL)))) {
        woxSnippetWaylandEvent(state->callback, 3, 1, 0, 0);
    } else if (device->keyboard && event.type == EV_KEY && event.code < BTN_MISC) {
        snippetWaylandKey(state, event);
    }
}

int snippetWaylandPoll(SnippetWayland *state, uintptr_t callback) {
    if (!state) return 0;
    state->callback = callback;
    if (wl_display_dispatch_pending(state->display) < 0 ||
        (wl_display_flush(state->display) < 0 && errno != EAGAIN)) return 0;
    struct epoll_event events[32];
    int count = epoll_wait(state->epoll_fd, events, 32, 10);
    if (count < 0 && errno != EINTR) return 0;
    for (int i = 0; i < count; i++) {
        int fd = events[i].data.fd;
        if (fd == wl_display_get_fd(state->display)) {
            if (wl_display_dispatch(state->display) < 0) return 0;
            continue;
        }
        if (fd == state->inotify_fd) {
            char buffer[4096];
            while (read(fd, buffer, sizeof(buffer)) > 0) {}
            snippetWaylandScanDevices(state);
            if (state->keys) { xkb_state_unref(state->keys); state->keys = xkb_state_new(state->keymap); }
            snippetWaylandResetKeyboard(state);
            woxSnippetWaylandEvent(callback, 3, 1, 0, 0);
            continue;
        }
        int index = -1;
        for (int n = 0; n < state->device_count; n++) if (state->devices[n].fd == fd) { index = n; break; }
        if (index < 0) continue;
        struct input_event event;
        ssize_t size;
        while ((size = read(fd, &event, sizeof(event))) == sizeof(event)) {
            snippetWaylandDeviceEvent(state, &state->devices[index], event);
        }
        if ((events[i].events & (EPOLLERR | EPOLLHUP)) || size == 0 || (size < 0 && errno != EAGAIN && errno != EINTR)) {
            epoll_ctl(state->epoll_fd, EPOLL_CTL_DEL, fd, NULL);
            close(fd);
            state->devices[index] = state->devices[--state->device_count];
            if (state->keys) { xkb_state_unref(state->keys); state->keys = xkb_state_new(state->keymap); }
            snippetWaylandResetKeyboard(state);
            woxSnippetWaylandEvent(callback, 3, 1, 0, 0);
        }
    }
    return count > 0 ? 2 : 1;
}

int snippetWaylandModifiersHeld(SnippetWayland *state) {
    if (!state || !state->keys || !state->group_valid) return 1;
    return xkb_state_mod_names_are_active(state->keys, XKB_STATE_MODS_DEPRESSED, XKB_STATE_MATCH_ANY,
                                         XKB_MOD_NAME_SHIFT, XKB_MOD_NAME_CTRL, XKB_MOD_NAME_ALT, XKB_MOD_NAME_LOGO, NULL) > 0;
}

int snippetWaylandHasLayout(SnippetWayland *state) { return state && state->keys && state->group_valid; }

unsigned int snippetWaylandLayoutCount(SnippetWayland *state) {
    return state && state->keymap ? xkb_keymap_num_layouts(state->keymap) : 0;
}

int snippetWaylandGroup(SnippetWayland *state) {
    return state && state->group_valid ? (int)state->group : -1;
}

void snippetWaylandClearGroup(SnippetWayland *state) {
    if (state && state->keymap && xkb_keymap_num_layouts(state->keymap) > 1) state->group_valid = 0;
}

int snippetWaylandSetGroup(SnippetWayland *state, unsigned int group) {
    if (!state || !state->keys || !state->keymap || group >= xkb_keymap_num_layouts(state->keymap)) return 0;
    xkb_mod_mask_t depressed = xkb_state_serialize_mods(state->keys, XKB_STATE_MODS_DEPRESSED);
    xkb_mod_mask_t latched = xkb_state_serialize_mods(state->keys, XKB_STATE_MODS_LATCHED);
    xkb_mod_mask_t locked = xkb_state_serialize_mods(state->keys, XKB_STATE_MODS_LOCKED);
    xkb_state_update_mask(state->keys, depressed, latched, locked, 0, 0, group);
    state->group = group;
    state->group_valid = 1;
    return 1;
}

static int snippetWaylandSend(int fd, unsigned short type, unsigned short code, int value) {
    struct input_event event = { .type = type, .code = code, .value = value };
    return write(fd, &event, sizeof(event)) == sizeof(event);
}

static int snippetWaylandKeyPress(int fd, unsigned short key, int value) {
    return snippetWaylandSend(fd, EV_KEY, key, value) && snippetWaylandSend(fd, EV_SYN, SYN_REPORT, 0);
}

int snippetWaylandDeleteAndPaste(SnippetWayland *state, unsigned int count, int terminal) {
    if (!state || state->output_fd < 0 || count > 32768) return 0;
    int fd = state->output_fd;
    for (unsigned int i = 0; i < count; i++) {
        if (!snippetWaylandKeyPress(fd, KEY_BACKSPACE, 1) || !snippetWaylandKeyPress(fd, KEY_BACKSPACE, 0)) return 0;
    }
    if (!snippetWaylandKeyPress(fd, KEY_LEFTCTRL, 1)) return 0;
    if (terminal && !snippetWaylandKeyPress(fd, KEY_LEFTSHIFT, 1)) {
        snippetWaylandKeyPress(fd, KEY_LEFTCTRL, 0);
        return 0;
    }
    int ok = snippetWaylandKeyPress(fd, KEY_V, 1);
    ok = snippetWaylandKeyPress(fd, KEY_V, 0) && ok;
    if (terminal) ok = snippetWaylandKeyPress(fd, KEY_LEFTSHIFT, 0) && ok;
    return snippetWaylandKeyPress(fd, KEY_LEFTCTRL, 0) && ok;
}

void snippetWaylandClose(SnippetWayland *state) {
    if (!state) return;
    for (int i = 0; i < state->device_count; i++) close(state->devices[i].fd);
    if (state->output_fd >= 0) { ioctl(state->output_fd, UI_DEV_DESTROY); close(state->output_fd); }
    if (state->inotify_fd >= 0) close(state->inotify_fd);
    if (state->epoll_fd >= 0) close(state->epoll_fd);
    if (state->keys) xkb_state_unref(state->keys);
    snippetComposeClose(state->compose);
    if (state->keymap) xkb_keymap_unref(state->keymap);
    if (state->xkb) xkb_context_unref(state->xkb);
    if (state->keyboard) wl_keyboard_destroy(state->keyboard);
    if (state->seat) wl_seat_destroy(state->seat);
    if (state->registry) wl_registry_destroy(state->registry);
    if (state->display) wl_display_disconnect(state->display);
    free(state);
}
