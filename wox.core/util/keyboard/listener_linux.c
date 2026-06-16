#include <X11/Xlib.h>
#include <X11/keysym.h>
#include <pthread.h>
#include <sys/select.h>
#include <unistd.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>

extern void keyboardHotkeyTriggeredCGO(int id);
extern int keyboardHookEventCGO(int eventKind, unsigned int keyCode, unsigned int modifiers);

typedef struct HotkeyEntry {
    int id;
    unsigned int modifiers;
    unsigned int keysym;
    KeyCode keycode;
    struct HotkeyEntry *next;
} HotkeyEntry;

typedef struct ModifierEntry {
    unsigned int keysym;
    KeyCode keycode;
    int pressed;
} ModifierEntry;

typedef struct {
    int action;
    int id;
    unsigned int modifiers;
    unsigned int keysym;
    int enabled;
    int ok;
    char error[256];
    int completed;
} KeyboardRequest;

enum {
    requestRegisterHotkey = 1,
    requestUnregisterHotkey = 2,
    requestSetRawHook = 3
};

static Display *gDisplay = NULL;
static Window gRoot = 0;
static int gPipeRead = -1;
static int gPipeWrite = -1;
static pthread_t gThread;
static int gThreadStarted = 0;
static int gRawEnabled = 0;
static HotkeyEntry *gHotkeys = NULL;
static ModifierEntry gModifiers[8];
static ModifierEntry gRawKeys[58];
static pthread_mutex_t gRequestMutex = PTHREAD_MUTEX_INITIALIZER;
static pthread_cond_t gRequestCond = PTHREAD_COND_INITIALIZER;
static KeyboardRequest gRequest;
static int gHasPendingRequest = 0;

static char *copy_error(const char *message) {
    if (!message) {
        return NULL;
    }
    size_t len = strlen(message) + 1;
    char *copy = malloc(len);
    if (!copy) {
        return NULL;
    }
    memcpy(copy, message, len);
    return copy;
}

static unsigned int current_modifier_mask(void) {
    unsigned int modifiers = 0;
    if (gModifiers[0].pressed || gModifiers[1].pressed) {
        modifiers |= 1;
    }
    if (gModifiers[2].pressed || gModifiers[3].pressed) {
        modifiers |= 2;
    }
    if (gModifiers[4].pressed || gModifiers[5].pressed) {
        modifiers |= 4;
    }
    if (gModifiers[6].pressed || gModifiers[7].pressed) {
        modifiers |= 8;
    }
    return modifiers;
}

static unsigned int to_x11_modifier_mask(unsigned int modifiers) {
    unsigned int mask = 0;
    if (modifiers & 1) {
        mask |= ControlMask;
    }
    if (modifiers & 2) {
        mask |= ShiftMask;
    }
    if (modifiers & 4) {
        mask |= Mod1Mask;
    }
    if (modifiers & 8) {
        mask |= Mod4Mask;
    }
    return mask;
}

static void grab_key_combination(KeyCode keycode, unsigned int modifiers) {
    static const unsigned int ignored_masks[] = {0, LockMask, Mod2Mask, LockMask | Mod2Mask};
    size_t count = sizeof(ignored_masks) / sizeof(ignored_masks[0]);
    for (size_t i = 0; i < count; i++) {
        XGrabKey(gDisplay, keycode, modifiers | ignored_masks[i], gRoot, True, GrabModeAsync, GrabModeAsync);
    }
}

static void ungrab_key_combination(KeyCode keycode, unsigned int modifiers) {
    static const unsigned int ignored_masks[] = {0, LockMask, Mod2Mask, LockMask | Mod2Mask};
    size_t count = sizeof(ignored_masks) / sizeof(ignored_masks[0]);
    for (size_t i = 0; i < count; i++) {
        XUngrabKey(gDisplay, keycode, modifiers | ignored_masks[i], gRoot);
    }
}

static HotkeyEntry *find_hotkey_by_id(int id) {
    HotkeyEntry *entry = gHotkeys;
    while (entry) {
        if (entry->id == id) {
            return entry;
        }
        entry = entry->next;
    }
    return NULL;
}

static void set_error(KeyboardRequest *request, const char *message) {
    snprintf(request->error, sizeof(request->error), "%s", message);
    request->ok = 0;
}

static void process_request(KeyboardRequest *request) {
    request->ok = 1;
    request->error[0] = '\0';

    if (request->action == requestRegisterHotkey) {
        KeyCode keycode = XKeysymToKeycode(gDisplay, (KeySym)request->keysym);
        if (keycode == 0) {
            set_error(request, "failed to resolve X11 keycode");
        } else {
            HotkeyEntry *entry = malloc(sizeof(HotkeyEntry));
            if (!entry) {
                set_error(request, "failed to allocate hotkey entry");
            } else {
                entry->id = request->id;
                entry->modifiers = request->modifiers;
                entry->keysym = request->keysym;
                entry->keycode = keycode;
                entry->next = gHotkeys;
                gHotkeys = entry;
                grab_key_combination(keycode, to_x11_modifier_mask(request->modifiers));
                XFlush(gDisplay);
            }
        }
    } else if (request->action == requestUnregisterHotkey) {
        HotkeyEntry **entry = &gHotkeys;
        while (*entry) {
            if ((*entry)->id == request->id) {
                HotkeyEntry *target = *entry;
                ungrab_key_combination(target->keycode, to_x11_modifier_mask(target->modifiers));
                *entry = target->next;
                free(target);
                XFlush(gDisplay);
                break;
            }
            entry = &((*entry)->next);
        }
    } else if (request->action == requestSetRawHook) {
        gRawEnabled = request->enabled;
    }

    request->completed = 1;
}

static void poll_modifier_keys(void) {
    char keymap[32];
    if (!gDisplay || !gRawEnabled) {
        return;
    }

    XQueryKeymap(gDisplay, keymap);
    for (int i = 0; i < 8; i++) {
        if (gModifiers[i].keycode == 0) {
            continue;
        }

        int byte_index = gModifiers[i].keycode / 8;
        int bit_index = gModifiers[i].keycode % 8;
        int pressed = (keymap[byte_index] & (1 << bit_index)) != 0;
        if (pressed == gModifiers[i].pressed) {
            continue;
        }

        gModifiers[i].pressed = pressed;
        keyboardHookEventCGO(pressed ? 0 : 1, gModifiers[i].keysym, current_modifier_mask());
    }
}

static void poll_raw_keys(void) {
    char keymap[32];
    if (!gDisplay || !gRawEnabled) {
        return;
    }

    XQueryKeymap(gDisplay, keymap);
    for (int i = 0; i < 58; i++) {
        if (gRawKeys[i].keycode == 0) {
            continue;
        }

        int byte_index = gRawKeys[i].keycode / 8;
        int bit_index = gRawKeys[i].keycode % 8;
        int pressed = (keymap[byte_index] & (1 << bit_index)) != 0;
        if (pressed == gRawKeys[i].pressed) {
            continue;
        }

        gRawKeys[i].pressed = pressed;
        keyboardHookEventCGO(pressed ? 0 : 1, gRawKeys[i].keysym, current_modifier_mask());
    }
}

static void *keyboard_thread_main(void *arg) {
    int xfd = ConnectionNumber(gDisplay);
    while (1) {
        fd_set readfds;
        FD_ZERO(&readfds);
        FD_SET(xfd, &readfds);
        FD_SET(gPipeRead, &readfds);
        int maxfd = xfd > gPipeRead ? xfd : gPipeRead;

        struct timeval tv;
        struct timeval *timeout = NULL;
        if (gRawEnabled) {
            tv.tv_sec = 0;
            tv.tv_usec = 20000;
            timeout = &tv;
        }

        int ready = select(maxfd + 1, &readfds, NULL, NULL, timeout);
        if (ready > 0 && FD_ISSET(gPipeRead, &readfds)) {
            char buffer[32];
            read(gPipeRead, buffer, sizeof(buffer));

            pthread_mutex_lock(&gRequestMutex);
            if (gHasPendingRequest) {
                process_request(&gRequest);
                gHasPendingRequest = 0;
                pthread_cond_broadcast(&gRequestCond);
            }
            pthread_mutex_unlock(&gRequestMutex);
        }

        if (ready > 0 && FD_ISSET(xfd, &readfds)) {
            while (XPending(gDisplay)) {
                XEvent event;
                XNextEvent(gDisplay, &event);
                if (event.type != KeyPress) {
                    continue;
                }

                unsigned int state = event.xkey.state & ~(LockMask | Mod2Mask);
                HotkeyEntry *entry = gHotkeys;
                while (entry) {
                    if (entry->keycode == event.xkey.keycode && to_x11_modifier_mask(entry->modifiers) == state) {
                        keyboardHotkeyTriggeredCGO(entry->id);
                        break;
                    }
                    entry = entry->next;
                }
            }
        }

        if (gRawEnabled) {
            poll_modifier_keys();
            poll_raw_keys();
        }
    }

    return NULL;
}

int woxLinuxEnsureKeyboardReady(char **errorOut) {
    if (gThreadStarted) {
        return 1;
    }

    XInitThreads();
    gDisplay = XOpenDisplay(NULL);
    if (!gDisplay) {
        if (errorOut) {
            *errorOut = copy_error("failed to open X11 display");
        }
        return 0;
    }

    gRoot = DefaultRootWindow(gDisplay);
    int pipefd[2];
    if (pipe(pipefd) != 0) {
        if (errorOut) {
            *errorOut = copy_error("failed to create Linux keyboard pipe");
        }
        return 0;
    }

    gPipeRead = pipefd[0];
    gPipeWrite = pipefd[1];

    KeySym modifierSyms[8] = {XK_Control_L, XK_Control_R, XK_Shift_L, XK_Shift_R, XK_Alt_L, XK_Alt_R, XK_Super_L, XK_Super_R};
    for (int i = 0; i < 8; i++) {
        gModifiers[i].keysym = (unsigned int)modifierSyms[i];
        gModifiers[i].keycode = XKeysymToKeycode(gDisplay, modifierSyms[i]);
        gModifiers[i].pressed = 0;
    }

    KeySym rawSyms[58] = {
        XK_Caps_Lock,
        XK_a, XK_b, XK_c, XK_d, XK_e, XK_f, XK_g, XK_h, XK_i, XK_j, XK_k, XK_l, XK_m,
        XK_n, XK_o, XK_p, XK_q, XK_r, XK_s, XK_t, XK_u, XK_v, XK_w, XK_x, XK_y, XK_z,
        XK_0, XK_1, XK_2, XK_3, XK_4, XK_5, XK_6, XK_7, XK_8, XK_9,
        XK_space, XK_Return, XK_Escape, XK_Tab, XK_Delete, XK_Left, XK_Right, XK_Up, XK_Down,
        XK_F1, XK_F2, XK_F3, XK_F4, XK_F5, XK_F6, XK_F7, XK_F8, XK_F9, XK_F10, XK_F11, XK_F12
    };
    for (int i = 0; i < 58; i++) {
        gRawKeys[i].keysym = (unsigned int)rawSyms[i];
        gRawKeys[i].keycode = XKeysymToKeycode(gDisplay, rawSyms[i]);
        gRawKeys[i].pressed = 0;
    }

    if (pthread_create(&gThread, NULL, keyboard_thread_main, NULL) != 0) {
        if (errorOut) {
            *errorOut = copy_error("failed to start Linux keyboard thread");
        }
        return 0;
    }

    gThreadStarted = 1;
    return 1;
}

int woxLinuxRegisterHotkey(int id, unsigned int modifiers, unsigned int keyCode, char **errorOut) {
    if (!woxLinuxEnsureKeyboardReady(errorOut)) {
        return 0;
    }

    pthread_mutex_lock(&gRequestMutex);
    gRequest.action = requestRegisterHotkey;
    gRequest.id = id;
    gRequest.modifiers = modifiers;
    gRequest.keysym = keyCode;
    gRequest.completed = 0;
    gHasPendingRequest = 1;
    write(gPipeWrite, "r", 1);
    while (!gRequest.completed) {
        pthread_cond_wait(&gRequestCond, &gRequestMutex);
    }
    int ok = gRequest.ok;
    if (!ok && errorOut && gRequest.error[0] != '\0') {
        *errorOut = copy_error(gRequest.error);
    }
    pthread_mutex_unlock(&gRequestMutex);
    return ok;
}

int woxLinuxUnregisterHotkey(int id, char **errorOut) {
    if (!woxLinuxEnsureKeyboardReady(errorOut)) {
        return 0;
    }

    pthread_mutex_lock(&gRequestMutex);
    gRequest.action = requestUnregisterHotkey;
    gRequest.id = id;
    gRequest.completed = 0;
    gHasPendingRequest = 1;
    write(gPipeWrite, "u", 1);
    while (!gRequest.completed) {
        pthread_cond_wait(&gRequestCond, &gRequestMutex);
    }
    int ok = gRequest.ok;
    if (!ok && errorOut && gRequest.error[0] != '\0') {
        *errorOut = copy_error(gRequest.error);
    }
    pthread_mutex_unlock(&gRequestMutex);
    return ok;
}

int woxLinuxSetRawKeyboardHookEnabled(int enabled, char **errorOut) {
    if (!woxLinuxEnsureKeyboardReady(errorOut)) {
        return 0;
    }

    pthread_mutex_lock(&gRequestMutex);
    gRequest.action = requestSetRawHook;
    gRequest.enabled = enabled;
    gRequest.completed = 0;
    gHasPendingRequest = 1;
    write(gPipeWrite, "k", 1);
    while (!gRequest.completed) {
        pthread_cond_wait(&gRequestCond, &gRequestMutex);
    }
    int ok = gRequest.ok;
    if (!ok && errorOut && gRequest.error[0] != '\0') {
        *errorOut = copy_error(gRequest.error);
    }
    pthread_mutex_unlock(&gRequestMutex);
    return ok;
}
