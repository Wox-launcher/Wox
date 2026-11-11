#include <X11/Xlib.h>
#include <X11/XKBlib.h>
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
    unsigned int grabStates[4];
    size_t grabCount;
    struct HotkeyEntry *next;
} HotkeyEntry;

typedef struct ModifierEntry {
    unsigned int keysym;
    KeyCode keycode;
    int pressed;
    int emitted;
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
static unsigned int gNumLockMask = 0;
static ModifierEntry gModifiers[8];
static const KeySym rawSyms[] = {
    XK_Caps_Lock,
    XK_a, XK_b, XK_c, XK_d, XK_e, XK_f, XK_g, XK_h, XK_i, XK_j, XK_k, XK_l, XK_m,
    XK_n, XK_o, XK_p, XK_q, XK_r, XK_s, XK_t, XK_u, XK_v, XK_w, XK_x, XK_y, XK_z,
    XK_0, XK_1, XK_2, XK_3, XK_4, XK_5, XK_6, XK_7, XK_8, XK_9,
    XK_space, XK_Return, XK_Escape, XK_Tab, XK_Delete, XK_Insert, XK_Left, XK_Right, XK_Up, XK_Down,
    XK_F1, XK_F2, XK_F3, XK_F4, XK_F5, XK_F6, XK_F7, XK_F8, XK_F9, XK_F10, XK_F11, XK_F12,
    XK_KP_0, XK_KP_1, XK_KP_2, XK_KP_3, XK_KP_4, XK_KP_5, XK_KP_6, XK_KP_7, XK_KP_8, XK_KP_9
};
#define RAW_KEY_COUNT (sizeof(rawSyms) / sizeof(rawSyms[0]))
static ModifierEntry gRawKeys[RAW_KEY_COUNT];
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

static int is_keypad_digit(KeySym keysym) {
    return keysym >= XK_KP_0 && keysym <= XK_KP_9;
}

static int matches_keypad_digit(KeyCode keycode, unsigned int state, KeySym expected) {
    unsigned int consumed = 0;
    KeySym keysym = NoSymbol;
    return XkbLookupKeySym(gDisplay, keycode, state, &consumed, &keysym) && keysym == expected;
}

static void grab_key_combination(HotkeyEntry *entry) {
    int keypad = is_keypad_digit(entry->keysym);
    unsigned int numlock = keypad ? gNumLockMask : Mod2Mask;
    unsigned int ignored_masks[] = {0, LockMask, numlock, LockMask | numlock};
    for (size_t i = 0; i < sizeof(ignored_masks) / sizeof(ignored_masks[0]); i++) {
        unsigned int state = to_x11_modifier_mask(entry->modifiers) | ignored_masks[i];
        // A keypad keycode can also produce Insert or an arrow. Leave those chords to the focused app.
        if (keypad && !matches_keypad_digit(entry->keycode, state, entry->keysym)) {
            continue;
        }
        XGrabKey(gDisplay, entry->keycode, state, gRoot, True, GrabModeAsync, GrabModeAsync);
        entry->grabStates[entry->grabCount++] = state;
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
                entry->grabCount = 0;
                grab_key_combination(entry);
                if (entry->grabCount == 0) {
                    free(entry);
                    set_error(request, "failed to resolve X11 keypad modifier state");
                } else {
                    entry->next = gHotkeys;
                    gHotkeys = entry;
                    XFlush(gDisplay);
                }
            }
        }
    } else if (request->action == requestUnregisterHotkey) {
        HotkeyEntry **entry = &gHotkeys;
        while (*entry) {
            if ((*entry)->id == request->id) {
                HotkeyEntry *target = *entry;
                for (size_t i = 0; i < target->grabCount; i++) {
                    XUngrabKey(gDisplay, target->keycode, target->grabStates[i], gRoot);
                }
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
    for (size_t i = 0; i < RAW_KEY_COUNT; i++) {
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
        if (pressed) {
            gRawKeys[i].emitted = 1;
            if (is_keypad_digit(gRawKeys[i].keysym)) {
                XkbStateRec state;
                gRawKeys[i].emitted = XkbGetState(gDisplay, XkbUseCoreKbd, &state) == Success &&
                    matches_keypad_digit(gRawKeys[i].keycode, XkbBuildCoreState(state.mods, state.group), gRawKeys[i].keysym);
            }
        }
        if (!gRawKeys[i].emitted) {
            continue;
        }
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

                HotkeyEntry *entry = gHotkeys;
                while (entry) {
                    int keypad = is_keypad_digit(entry->keysym);
                    unsigned int state = event.xkey.state & ~(LockMask | (keypad ? gNumLockMask : Mod2Mask));
                    if (entry->keycode == event.xkey.keycode && to_x11_modifier_mask(entry->modifiers) == state &&
                        (!keypad || matches_keypad_digit(entry->keycode, event.xkey.state, entry->keysym))) {
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

    gDisplay = XOpenDisplay(NULL);
    if (!gDisplay) {
        if (errorOut) {
            *errorOut = copy_error("failed to open X11 display");
        }
        return 0;
    }

    gRoot = DefaultRootWindow(gDisplay);
    gNumLockMask = XkbKeysymToModifiers(gDisplay, XK_Num_Lock);
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

    for (size_t i = 0; i < RAW_KEY_COUNT; i++) {
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
