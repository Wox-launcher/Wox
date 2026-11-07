#include <windows.h>

// Resolve the key using the foreground thread's layout without consuming dead-key state.
unsigned int snippetCharacter(unsigned int vk, unsigned int scan) {
    BYTE state[256] = {0};
    if (GetAsyncKeyState(VK_SHIFT) & 0x8000) state[VK_SHIFT] = 0x80;
    if (GetAsyncKeyState(VK_CONTROL) & 0x8000) state[VK_CONTROL] = state[VK_LCONTROL] = 0x80;
    if (GetAsyncKeyState(VK_MENU) & 0x8000) state[VK_MENU] = state[VK_RMENU] = 0x80;
    if (GetKeyState(VK_CAPITAL) & 1) state[VK_CAPITAL] = 1;
    HWND foreground = GetForegroundWindow();
    if (!foreground) return 0;
    HKL layout = GetKeyboardLayout(GetWindowThreadProcessId(foreground, NULL));
    WCHAR text[8] = {0};
    int count = ToUnicodeEx(vk, scan, state, text, 8, 4, layout);
    if (count < 0) return 0xffffffffu;
    if (count != 1 || text[0] < 0x20 || text[0] > 0x7e) return 0;
    return text[0];
}
