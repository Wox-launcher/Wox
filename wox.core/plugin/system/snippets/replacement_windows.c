#include <windows.h>
#include <stdint.h>
#include <stdlib.h>

// Submit deletion and insertion together so other input cannot be interleaved by SendInput.
int snippetWindowsReplace(unsigned int count, const uint16_t *text, unsigned int length, int paste) {
    if (count > 32768 || length > 1048576) return 0;
    unsigned int total = count * 2 + length * 2 + (paste ? 4 : 0);
    if (!total) return 1;
    INPUT *inputs = calloc(total, sizeof(INPUT));
    if (!inputs) return 0;
    unsigned int offset = 0;
    for (unsigned int i = 0; i < count; i++) {
        inputs[offset].type = INPUT_KEYBOARD;
        inputs[offset].ki.wVk = VK_BACK;
        inputs[offset + 1] = inputs[offset];
        inputs[offset + 1].ki.dwFlags = KEYEVENTF_KEYUP;
        offset += 2;
    }
    for (unsigned int i = 0; i < length; i++) {
        inputs[offset].type = INPUT_KEYBOARD;
        inputs[offset].ki.wScan = text[i];
        inputs[offset].ki.dwFlags = KEYEVENTF_UNICODE;
        inputs[offset + 1] = inputs[offset];
        inputs[offset + 1].ki.dwFlags |= KEYEVENTF_KEYUP;
        offset += 2;
    }
    if (paste) {
        inputs[offset].type = inputs[offset + 1].type = inputs[offset + 2].type = inputs[offset + 3].type = INPUT_KEYBOARD;
        inputs[offset].ki.wVk = inputs[offset + 3].ki.wVk = VK_CONTROL;
        inputs[offset + 1].ki.wVk = inputs[offset + 2].ki.wVk = 'V';
        inputs[offset + 2].ki.dwFlags = inputs[offset + 3].ki.dwFlags = KEYEVENTF_KEYUP;
    }
    for (unsigned int i = 0; i < total; i++) inputs[i].ki.dwExtraInfo = (ULONG_PTR)0x574f5854;
    UINT sent = SendInput(total, inputs, sizeof(INPUT));
    free(inputs);
    return sent == total;
}
