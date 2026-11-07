//go:build linux && cgo

#define _GNU_SOURCE
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/XKBlib.h>
#include <X11/keysym.h>
#include <X11/extensions/XInput2.h>
#include <X11/extensions/XTest.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <strings.h>
#include <sys/select.h>
#include "compose_linux.h"

extern void woxSnippetX11Event(uintptr_t callback, int kind, int down, unsigned int key, uint64_t target, unsigned int character);

typedef struct SnippetX11 {
    Display *display;
    int opcode;
    unsigned char test_devices[65536 / 8];
    int test_keyboard_count;
    SnippetCompose *compose;
} SnippetX11;

// Refresh all XTest source IDs after device hierarchy changes, not just the last seat.
static int snippetX11RefreshTestDevices(SnippetX11 *state) {
    int count = 0;
    XIDeviceInfo *devices = XIQueryDevice(state->display, XIAllDevices, &count);
    if (!devices) return 0;
    memset(state->test_devices, 0, sizeof(state->test_devices));
    state->test_keyboard_count = 0;
    for (int i = 0; i < count; i++) {
        int id = devices[i].deviceid;
        if (id < 0 || id >= 65536 || !devices[i].name || !strstr(devices[i].name, "XTEST")) continue;
        if (devices[i].use != XISlaveKeyboard && devices[i].use != XISlavePointer) continue;
        state->test_devices[id / 8] |= (unsigned char)(1u << (id % 8));
        if (devices[i].use == XISlaveKeyboard) state->test_keyboard_count++;
    }
    XIFreeDeviceInfo(devices);
    return state->test_keyboard_count > 0;
}

uint64_t snippetX11Target(SnippetX11 *state) {
    if (!state || !state->display || !state->test_keyboard_count) return 0;
    Window focus = None;
    int revert = 0;
    XGetInputFocus(state->display, &focus, &revert);
    return focus == None || focus == PointerRoot ? 0 : (uint64_t)focus;
}

int snippetX11Ready(SnippetX11 *state) { return state && state->test_keyboard_count > 0; }

SnippetX11 *snippetX11Open(int *failure) {
    if (failure) *failure = 1;
    XInitThreads();
    Display *display = XOpenDisplay(NULL);
    if (!display) return NULL;
    SnippetX11 *state = calloc(1, sizeof(*state));
    if (!state) { XCloseDisplay(display); return NULL; }
    state->display = display;
    state->compose = snippetComposeOpen();
    if (!state->compose) { XCloseDisplay(display); free(state); return NULL; }
    int event, error, xtestEvent, xtestError, xtestMajor, xtestMinor;
    int major = 2, minor = 0;
    if (!XTestQueryExtension(display, &xtestEvent, &xtestError, &xtestMajor, &xtestMinor)) {
        if (failure) *failure = 2;
        snippetComposeClose(state->compose);
        XCloseDisplay(display);
        free(state);
        return NULL;
    }
    if (!XQueryExtension(display, "XInputExtension", &state->opcode, &event, &error) ||
        XIQueryVersion(display, &major, &minor) != Success) {
        snippetComposeClose(state->compose);
        XCloseDisplay(display);
        free(state);
        return NULL;
    }
    unsigned char mask[(XI_LASTEVENT + 7) / 8] = {0};
    unsigned char hierarchyMask[(XI_LASTEVENT + 7) / 8] = {0};
    XISetMask(mask, XI_RawKeyPress);
    XISetMask(mask, XI_RawKeyRelease);
    XISetMask(mask, XI_RawButtonPress);
    XISetMask(hierarchyMask, XI_HierarchyChanged);
    XIEventMask selection[] = {
        { .deviceid = XIAllMasterDevices, .mask_len = sizeof(mask), .mask = mask },
        { .deviceid = XIAllDevices, .mask_len = sizeof(hierarchyMask), .mask = hierarchyMask },
    };
    if (XISelectEvents(display, DefaultRootWindow(display), selection, 2) != Success) {
        snippetComposeClose(state->compose);
        XCloseDisplay(display);
        free(state);
        return NULL;
    }
    // Subscribe before the snapshot so a concurrent hierarchy change is queued.
    // Without an XTest identity, replayed keys could recursively trigger matching.
    if (!snippetX11RefreshTestDevices(state)) {
        if (failure) *failure = 2;
        snippetComposeClose(state->compose);
        XCloseDisplay(display);
        free(state);
        return NULL;
    }
    XSync(display, False);
    return state;
}

static int snippetX11Modifier(KeySym key) {
    return key == XK_Shift_L || key == XK_Shift_R || key == XK_Control_L || key == XK_Control_R ||
           key == XK_Alt_L || key == XK_Alt_R || key == XK_Super_L || key == XK_Super_R || key == XK_ISO_Level3_Shift;
}

void snippetX11Poll(SnippetX11 *state, uintptr_t callback) {
    if (!state) return;
    Display *display = state->display;
    if (!XPending(display)) {
        int fd = ConnectionNumber(display);
        fd_set fds;
        FD_ZERO(&fds);
        FD_SET(fd, &fds);
        struct timeval timeout = { .tv_sec = 0, .tv_usec = 10000 };
        if (select(fd + 1, &fds, NULL, NULL, &timeout) <= 0) return;
    }
    while (XPending(display)) {
        XEvent event;
        XNextEvent(display, &event);
        if (event.type != GenericEvent || event.xcookie.extension != state->opcode || !XGetEventData(display, &event.xcookie)) continue;
        if (event.xcookie.evtype == XI_HierarchyChanged) {
            if (!snippetX11RefreshTestDevices(state)) state->test_keyboard_count = 0;
            woxSnippetX11Event(callback, 3, 1, 0, 0, 0);
            XFreeEventData(display, &event.xcookie);
            continue;
        }
        XIRawEvent *raw = event.xcookie.data;
        int source = raw->sourceid;
        int isTest = source >= 0 && source < 65536 && (state->test_devices[source / 8] & (1u << (source % 8)));
        if (state->test_keyboard_count && !isTest) {
            uint64_t target = snippetX11Target(state);
            if (event.xcookie.evtype == XI_RawButtonPress) {
                woxSnippetX11Event(callback, 3, 1, 0, target, 0);
            } else if (event.xcookie.evtype == XI_RawKeyPress || event.xcookie.evtype == XI_RawKeyRelease) {
                int down = event.xcookie.evtype == XI_RawKeyPress;
                XkbStateRec keys = {0};
                unsigned int consumed = 0;
                KeySym sym = NoSymbol;
                if (XkbGetState(display, XkbUseCoreKbd, &keys) == Success) {
                    XkbLookupKeySym(display, raw->detail, XkbBuildCoreState(keys.mods, keys.group), &consumed, &sym);
                }
                int kind = 3;
                unsigned int character = 0;
                int composing = down && snippetComposeFeed(state->compose, sym);
                if (snippetX11Modifier(sym)) kind = 2;
                else if (composing) kind = 3;
                else if (keys.mods & (ControlMask | Mod1Mask | Mod4Mask)) kind = 3;
                else if (sym == XK_BackSpace) kind = 1;
                else if (sym >= 0x20 && sym <= 0x7e) { kind = 0; character = (unsigned int)sym; }
                woxSnippetX11Event(callback, kind, down, (unsigned int)raw->detail, target, character);
            }
        }
        XFreeEventData(display, &event.xcookie);
    }
}

void snippetX11Close(SnippetX11 *state) {
    if (!state) return;
    snippetComposeClose(state->compose);
    XCloseDisplay(state->display);
    free(state);
}

uint64_t snippetX11ClipboardOwner(SnippetX11 *state) {
    if (!state) return 0;
    Atom clipboard = XInternAtom(state->display, "CLIPBOARD", False);
    return (uint64_t)XGetSelectionOwner(state->display, clipboard);
}

int snippetX11ModifiersHeld(SnippetX11 *state) {
    XkbStateRec keys;
    if (!state || XkbGetState(state->display, XkbUseCoreKbd, &keys) != Success) return 1;
    return (keys.mods & (ShiftMask | ControlMask | Mod1Mask | Mod4Mask)) != 0;
}

int snippetX11IsTerminal(SnippetX11 *state) {
    if (!state) return 0;
    Window window = (Window)snippetX11Target(state);
    for (int i = 0; window && i < 16; i++) {
        XClassHint hint = {0};
        if (XGetClassHint(state->display, window, &hint)) {
            const char *name = hint.res_class ? hint.res_class : "";
            int found = strcasestr(name, "terminal") || strcasestr(name, "konsole") || strcasestr(name, "xterm") ||
                        strcasestr(name, "alacritty") || strcasestr(name, "kitty") || strcasestr(name, "wezterm");
            if (hint.res_name) XFree(hint.res_name);
            if (hint.res_class) XFree(hint.res_class);
            if (found) return 1;
        }
        Window root, parent, *children = NULL;
        unsigned int count = 0;
        if (!XQueryTree(state->display, window, &root, &parent, &children, &count)) break;
        if (children) XFree(children);
        if (parent == window || parent == root) break;
        window = parent;
    }
    return 0;
}

int snippetX11DeleteAndPaste(SnippetX11 *state, unsigned int count, int terminal, uint64_t expectedOwner) {
    if (!state || !state->test_keyboard_count || count > 32768 || !expectedOwner) return 0;
    Display *display = state->display;
    KeyCode back = XKeysymToKeycode(display, XK_BackSpace);
    KeyCode ctrl = XKeysymToKeycode(display, XK_Control_L);
    KeyCode shift = XKeysymToKeycode(display, XK_Shift_L);
    KeyCode v = XKeysymToKeycode(display, XK_v);
    if (!back || !ctrl || !shift || !v) return 0;
    if (snippetX11ClipboardOwner(state) != expectedOwner) return 0;
    for (unsigned int i = 0; i < count; i++) {
        XTestFakeKeyEvent(display, back, True, CurrentTime);
        XTestFakeKeyEvent(display, back, False, CurrentTime);
    }
    XTestFakeKeyEvent(display, ctrl, True, CurrentTime);
    if (terminal) XTestFakeKeyEvent(display, shift, True, CurrentTime);
    XTestFakeKeyEvent(display, v, True, CurrentTime);
    XTestFakeKeyEvent(display, v, False, CurrentTime);
    if (terminal) XTestFakeKeyEvent(display, shift, False, CurrentTime);
    XTestFakeKeyEvent(display, ctrl, False, CurrentTime);
    XSync(display, False);
    return 1;
}
