#include "../expansion_linux_wayland.c"

static int lastKind;
static unsigned int lastCharacter;

void woxSnippetWaylandEvent(uintptr_t callback, int kind, int down, unsigned int key, unsigned int character) {
    if (down) { lastKind = kind; lastCharacter = character; }
}

static void press(SnippetWayland *state, unsigned short key, int value) {
    struct input_event event = { .type = EV_KEY, .code = key, .value = value };
    snippetWaylandKey(state, event);
}

int main(void) {
    SnippetWayland state = {0};
    struct xkb_rule_names names = { .layout = "us", .variant = "intl" };
    state.xkb = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
    if (!state.xkb) return 1;
    state.keymap = xkb_keymap_new_from_names(state.xkb, &names, XKB_KEYMAP_COMPILE_NO_FLAGS);
    if (!state.keymap) return 2;
    state.keys = xkb_state_new(state.keymap);
    state.compose = snippetComposeOpen();
    state.callback = 1;
    state.group_valid = 1;
    if (!state.keys || !state.compose) return 3;
    press(&state, KEY_A, 1);
    if (lastKind != 0 || lastCharacter != 'a') return 4;
    press(&state, KEY_A, 0);
    press(&state, KEY_LEFTSHIFT, 1);
    press(&state, KEY_A, 1);
    if (lastKind != 0 || lastCharacter != 'A') return 5;
    press(&state, KEY_A, 0);
    press(&state, KEY_LEFTSHIFT, 0);
    press(&state, KEY_APOSTROPHE, 1);
    if (lastKind != 3) return 6;
    press(&state, KEY_APOSTROPHE, 0);
    press(&state, KEY_A, 1);
    if (lastKind != 3) return 7;
    press(&state, KEY_A, 0);

    int output[2];
    if (pipe(output) != 0) return 8;
    state.output_fd = output[1];
    if (!snippetWaylandDeleteAndPaste(&state, 1, 1)) return 9;
    unsigned short codes[] = {KEY_BACKSPACE, KEY_BACKSPACE, KEY_LEFTCTRL, KEY_LEFTSHIFT, KEY_V, KEY_V, KEY_LEFTSHIFT, KEY_LEFTCTRL};
    int values[] = {1, 0, 1, 1, 1, 0, 0, 0};
    for (unsigned int i = 0; i < sizeof(codes) / sizeof(codes[0]); i++) {
        struct input_event event, sync;
        if (read(output[0], &event, sizeof(event)) != sizeof(event) ||
            read(output[0], &sync, sizeof(sync)) != sizeof(sync) ||
            event.type != EV_KEY || event.code != codes[i] || event.value != values[i] ||
            sync.type != EV_SYN || sync.code != SYN_REPORT) return 10;
    }
    close(output[0]);
    close(output[1]);
    struct xkb_rule_names multiple = { .layout = "us,ru" };
    struct xkb_keymap *multiKeymap = xkb_keymap_new_from_names(state.xkb, &multiple, XKB_KEYMAP_COMPILE_NO_FLAGS);
    if (!multiKeymap) return 11;
    char *serialized = xkb_keymap_get_as_string(multiKeymap, XKB_KEYMAP_FORMAT_TEXT_V1);
    FILE *keymapFile = tmpfile();
    if (!serialized || !keymapFile) return 12;
    size_t size = strlen(serialized) + 1;
    if (fwrite(serialized, 1, size, keymapFile) != size || fflush(keymapFile) != 0) return 13;
    int keymapFD = dup(fileno(keymapFile));
    if (keymapFD < 0 || lseek(keymapFD, 0, SEEK_SET) < 0) return 14;
    lastKind = 0;
    snippetKeyboardKeymap(&state, NULL, WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1, keymapFD, size);
    free(serialized);
    fclose(keymapFile);
    xkb_keymap_unref(multiKeymap);
    if (!state.keys || !state.keymap || state.group_valid || lastKind != 4) return 15;
    if (!snippetWaylandSetGroup(&state, 1) || snippetWaylandGroup(&state) != 1) return 16;
    press(&state, KEY_A, 1);
    if (lastKind != 3) return 17;
    press(&state, KEY_A, 0);
    if (!snippetWaylandSetGroup(&state, 0) || snippetWaylandGroup(&state) != 0) return 18;
    press(&state, KEY_A, 1);
    if (lastKind != 0 || lastCharacter != 'a') return 19;
    press(&state, KEY_A, 0);

    state.device_count = 2;
    state.devices[0].fd = state.devices[1].fd = -1;
    state.devices[0].keyboard = state.devices[1].keyboard = 1;
    struct input_event dropped = { .type = EV_SYN, .code = SYN_DROPPED };
    struct input_event key = { .type = EV_KEY, .code = KEY_A, .value = 1 };
    struct input_event report = { .type = EV_SYN, .code = SYN_REPORT };
    snippetWaylandDeviceEvent(&state, &state.devices[0], dropped);
    if (lastKind != 3 || !state.devices[0].desynchronized) return 20;
    lastKind = -1;
    snippetWaylandDeviceEvent(&state, &state.devices[0], key);
    snippetWaylandDeviceEvent(&state, &state.devices[1], key);
    if (lastKind != -1) return 21;
    snippetWaylandDeviceEvent(&state, &state.devices[0], report);
    if (lastKind != 3 || state.devices[0].desynchronized) return 22;
    snippetWaylandDeviceEvent(&state, &state.devices[1], key);
    if (lastKind != 0 || lastCharacter != 'a') return 23;
    key.value = 0;
    snippetWaylandDeviceEvent(&state, &state.devices[1], key);
    state.device_count = 0;
    xkb_state_unref(state.keys);
    xkb_keymap_unref(state.keymap);
    snippetComposeClose(state.compose);
    xkb_context_unref(state.xkb);
    return 0;
}
