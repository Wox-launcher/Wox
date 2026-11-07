#include <windows.h>
#include <stdint.h>
#include <string.h>

static INPUT captured[16];
static UINT capturedCount;
static int partial;

static UINT fakeSendInput(UINT count, LPINPUT inputs, int size) {
    if (size != sizeof(INPUT) || count > 16) return 0;
    memcpy(captured, inputs, count * sizeof(INPUT));
    capturedCount = count;
    return partial ? count - 1 : count;
}

#define SendInput fakeSendInput
#include "../replacement_windows.c"
#undef SendInput

static int key(UINT index, WORD vk, WORD scan, DWORD flags) {
    return captured[index].type == INPUT_KEYBOARD && captured[index].ki.dwExtraInfo == (ULONG_PTR)0x574f5854 && captured[index].ki.wVk == vk &&
           captured[index].ki.wScan == scan && captured[index].ki.dwFlags == flags;
}

int main(void) {
    const uint16_t text[] = {0xd83d, 0xde00};
    if (!snippetWindowsReplace(2, text, 2, 0) || capturedCount != 8 ||
        !key(0, VK_BACK, 0, 0) || !key(1, VK_BACK, 0, KEYEVENTF_KEYUP) ||
        !key(2, VK_BACK, 0, 0) || !key(3, VK_BACK, 0, KEYEVENTF_KEYUP) ||
        !key(4, 0, text[0], KEYEVENTF_UNICODE) || !key(5, 0, text[0], KEYEVENTF_UNICODE | KEYEVENTF_KEYUP) ||
        !key(6, 0, text[1], KEYEVENTF_UNICODE) || !key(7, 0, text[1], KEYEVENTF_UNICODE | KEYEVENTF_KEYUP)) return 1;
    if (!snippetWindowsReplace(1, NULL, 0, 1) || capturedCount != 6 ||
        !key(0, VK_BACK, 0, 0) || !key(1, VK_BACK, 0, KEYEVENTF_KEYUP) ||
        !key(2, VK_CONTROL, 0, 0) || !key(3, 'V', 0, 0) ||
        !key(4, 'V', 0, KEYEVENTF_KEYUP) || !key(5, VK_CONTROL, 0, KEYEVENTF_KEYUP)) return 2;
    partial = 1;
    if (snippetWindowsReplace(1, NULL, 0, 1)) return 3;
    return 0;
}
