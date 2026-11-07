#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/keysym.h>
#include <X11/extensions/XInput2.h>
#include <stdint.h>
#include <stdlib.h>

typedef struct SnippetX11 SnippetX11;
SnippetX11 *snippetX11Open(int *failure);
void snippetX11Poll(SnippetX11 *state, uintptr_t callback);
void snippetX11Close(SnippetX11 *state);
int snippetX11DeleteAndPaste(SnippetX11 *state, unsigned int count, int terminal, uint64_t expectedOwner);

static int observed;
void woxSnippetX11Event(uintptr_t callback, int kind, int down, unsigned int key, uint64_t target, unsigned int character) {
    observed++;
}

static int addMaster(Display *display, char *name) {
    XIAddMasterInfo add = { .type = XIAddMaster, .name = name, .send_core = True, .enable = True };
    int result = XIChangeHierarchy(display, (XIAnyHierarchyChangeInfo *)&add, 1);
    XSync(display, False);
    return result == Success;
}

int main(void) {
    Display *display = XOpenDisplay(NULL);
    if (!display) return 1;
    Window root = DefaultRootWindow(display);
    Window window = XCreateSimpleWindow(display, root, 10, 10, 200, 100, 0, 0, 0);
    XSelectInput(display, window, KeyPressMask | KeyReleaseMask);
    XMapWindow(display, window);
    XSync(display, False);
    XSetInputFocus(display, window, RevertToParent, CurrentTime);
    XSync(display, False);
    if (getenv("WOX_TEST_X11_MULTISEAT")) {
        int major = 2, minor = 0;
        if (XIQueryVersion(display, &major, &minor) != Success || !addMaster(display, "Wox test seat before")) return 11;
    }
    int failure = 0;
    SnippetX11 *state = snippetX11Open(&failure);
    if (getenv("WOX_TEST_X11_EXPECT_NO_XTEST")) {
        if (state) snippetX11Close(state);
        XDestroyWindow(display, window);
        XCloseDisplay(display);
        return state || failure != 2 ? 10 : 0;
    }
    if (!state) return 2;
    if (getenv("WOX_TEST_X11_MULTISEAT")) {
        if (!addMaster(display, "Wox test seat after")) return 12;
        snippetX11Poll(state, 1);
        observed = 0;
    }
    Atom clipboard = XInternAtom(display, "CLIPBOARD", False);
    XSetSelectionOwner(display, clipboard, window, CurrentTime);
    XSync(display, False);
    if (!snippetX11DeleteAndPaste(state, 2, 0, window)) return 3;
    XSync(display, False);
    KeySym expected[] = {XK_BackSpace, XK_BackSpace, XK_Control_L, XK_v};
    int types[] = {KeyPress, KeyRelease, KeyPress, KeyRelease, KeyPress, KeyPress, KeyRelease, KeyRelease};
    KeySym symbols[] = {expected[0], expected[0], expected[1], expected[1], expected[2], expected[3], expected[3], expected[2]};
    for (int i = 0; i < 8; i++) {
        if (!XPending(display)) return 4;
        XEvent event;
        XNextEvent(display, &event);
        if (event.type != types[i] || XLookupKeysym(&event.xkey, 0) != symbols[i]) return 5;
    }
    if (!snippetX11DeleteAndPaste(state, 1, 1, window)) return 7;
    XSync(display, False);
    KeySym terminal[] = {XK_BackSpace, XK_BackSpace, XK_Control_L, XK_Shift_L, XK_v, XK_v, XK_Shift_L, XK_Control_L};
    int terminalTypes[] = {KeyPress, KeyRelease, KeyPress, KeyPress, KeyPress, KeyRelease, KeyRelease, KeyRelease};
    for (int i = 0; i < 8; i++) {
        if (!XPending(display)) return 8;
        XEvent event;
        XNextEvent(display, &event);
        if (event.type != terminalTypes[i] || XLookupKeysym(&event.xkey, 0) != terminal[i]) return 9;
    }
    Window replacementOwner = XCreateSimpleWindow(display, root, 0, 0, 1, 1, 0, 0, 0);
    XSetSelectionOwner(display, clipboard, replacementOwner, CurrentTime);
    XSync(display, False);
    if (snippetX11DeleteAndPaste(state, 1, 0, window)) return 13;
    XSync(display, False);
    while (XPending(display)) {
        XEvent event;
        XNextEvent(display, &event);
        if (event.type == KeyPress || event.type == KeyRelease) return 14;
    }
    XDestroyWindow(display, replacementOwner);
    snippetX11Poll(state, 1);
    snippetX11Close(state);
    XDestroyWindow(display, window);
    XCloseDisplay(display);
    return observed == 0 ? 0 : 6;
}
