#include <windows.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>

#include "../replacement_windows.c"

int snippetWindowsFocusedControlSafe(void);
int snippetWindowsUsesCodepointBackspace(uint64_t target);
unsigned int snippetCharacter(unsigned int vk, unsigned int scan);

static DWORD WINAPI checkFocusedControl(void *result) {
    *(int *)result = snippetWindowsFocusedControlSafe();
    return 0;
}

// UI Automation may call back into the edit control, so keep its window thread pumping.
static int checkFocusedControlWithPump(void) {
    int *result = malloc(sizeof(*result));
    if (!result) return -1;
    *result = -1;
    HANDLE thread = CreateThread(NULL, 0, checkFocusedControl, result, 0, NULL);
    if (!thread) { free(result); return -1; }
    ULONGLONG deadline = GetTickCount64() + 3000;
    while (GetTickCount64() < deadline) {
        DWORD status = MsgWaitForMultipleObjects(1, &thread, FALSE, 50, QS_ALLINPUT);
        if (status == WAIT_OBJECT_0) { int value = *result; free(result); CloseHandle(thread); return value; }
        MSG message;
        while (PeekMessageW(&message, NULL, 0, 0, PM_REMOVE)) {
            TranslateMessage(&message);
            DispatchMessageW(&message);
        }
    }
    // The process exits on timeout; the worker may still hold its result buffer.
    CloseHandle(thread);
    return -1;
}

// A foreground app can block SetForegroundWindow; attach only for this explicit UI test.
static int focusTestWindow(HWND window) {
    HWND foreground = GetForegroundWindow();
    DWORD targetThread = foreground ? GetWindowThreadProcessId(foreground, NULL) : 0;
    DWORD ownThread = GetCurrentThreadId();
    BOOL attached = targetThread && targetThread != ownThread && AttachThreadInput(ownThread, targetThread, TRUE);
    SetForegroundWindow(window);
    SetFocus(window);
    if (attached) AttachThreadInput(ownThread, targetThread, FALSE);
    return GetForegroundWindow() == window && GetFocus() == window;
}

static int testEditUndo(const WCHAR *expanded, unsigned int remaining) {
    WCHAR original[128] = L"prefix ";
    lstrcatW(original, expanded);
    HWND edit = CreateWindowExW(0, L"EDIT", original,
                                WS_OVERLAPPEDWINDOW | WS_VISIBLE | ES_MULTILINE | ES_AUTOVSCROLL,
                                CW_USEDEFAULT, CW_USEDEFAULT, 420, 140, NULL, NULL, GetModuleHandleW(NULL), NULL);
    if (!edit || !focusTestWindow(edit) || !snippetWindowsUsesCodepointBackspace((uint64_t)(uintptr_t)edit)) return 0;
    SendMessageW(edit, EM_SETSEL, (WPARAM)GetWindowTextLengthW(edit), (LPARAM)GetWindowTextLengthW(edit));
    INPUT backspace[2] = {0};
    backspace[0].type = backspace[1].type = INPUT_KEYBOARD;
    backspace[0].ki.wVk = backspace[1].ki.wVk = VK_BACK;
    backspace[1].ki.dwFlags = KEYEVENTF_KEYUP;
    const uint16_t keyword[] = {';', 'w', 'x'};
    if (SendInput(2, backspace, sizeof(INPUT)) != 2 ||
        !snippetWindowsReplace(remaining, keyword, 3, 0)) { DestroyWindow(edit); return 0; }
    ULONGLONG deadline = GetTickCount64() + 3000;
    while (GetTickCount64() < deadline) {
        MSG message;
        while (PeekMessageW(&message, NULL, 0, 0, PM_REMOVE)) { TranslateMessage(&message); DispatchMessageW(&message); }
        WCHAR text[128] = {0};
        GetWindowTextW(edit, text, 128);
        if (lstrcmpW(text, L"prefix ;wx") == 0) { DestroyWindow(edit); return 1; }
        Sleep(10);
    }
    DestroyWindow(edit);
    return 0;
}

int main(void) {
    HWND edit = CreateWindowExW(0, L"EDIT", L"before ;abc", WS_OVERLAPPEDWINDOW | WS_VISIBLE | ES_AUTOHSCROLL,
                                CW_USEDEFAULT, CW_USEDEFAULT, 420, 100, NULL, NULL, GetModuleHandleW(NULL), NULL);
    if (!edit) return 1;
    int focused = focusTestWindow(edit);
    SendMessageW(edit, EM_SETSEL, (WPARAM)GetWindowTextLengthW(edit), (LPARAM)GetWindowTextLengthW(edit));
    if (!focused) {
        fprintf(stderr, "could not focus the isolated edit control (foreground present=%d, local focus=%d)\n",
                GetForegroundWindow() != NULL, GetFocus() == edit);
        DestroyWindow(edit);
        return 2;
    }
    if (checkFocusedControlWithPump() != 1) {
        fprintf(stderr, "normal edit control was rejected by UI Automation\n");
        DestroyWindow(edit);
        return 5;
    }

    HKL original = GetKeyboardLayout(0);
    HKL us = LoadKeyboardLayoutW(L"00000409", KLF_NOTELLSHELL);
    if (!us || !ActivateKeyboardLayout(us, 0)) { DestroyWindow(edit); return 8; }
    unsigned int scan = MapVirtualKeyExW(VK_OEM_1, MAPVK_VK_TO_VSC, us);
    unsigned int unshifted = snippetCharacter(VK_OEM_1, scan);
    INPUT shift = {0};
    shift.type = INPUT_KEYBOARD;
    shift.ki.wVk = VK_SHIFT;
    if (SendInput(1, &shift, sizeof(shift)) != 1) { DestroyWindow(edit); return 9; }
    unsigned int shifted = snippetCharacter(VK_OEM_1, scan);
    shift.ki.dwFlags = KEYEVENTF_KEYUP;
    int released = SendInput(1, &shift, sizeof(shift)) == 1;
    ActivateKeyboardLayout(original, 0);
    if (!released || unshifted != ';' || shifted != ':') {
        fprintf(stderr, "US layout punctuation mismatch: unshifted=%u shifted=%u\n", unshifted, shifted);
        DestroyWindow(edit);
        return 10;
    }

    HKL german = LoadKeyboardLayoutW(L"00000407", KLF_NOTELLSHELL);
    if (german && ActivateKeyboardLayout(german, 0)) {
        // The physical key at the US Y position is VK_Z under the German layout.
        unsigned int germanScan = 0x15;
        unsigned int germanKey = MapVirtualKeyExW(germanScan, MAPVK_VSC_TO_VK_EX, german);
        unsigned int germanCharacter = snippetCharacter(germanKey, germanScan);
        ActivateKeyboardLayout(original, 0);
        if (germanCharacter != 'z') {
            fprintf(stderr, "German layout key mismatch: character=%u\n", germanCharacter);
            DestroyWindow(edit);
            return 17;
        }
        printf("German keyboard layout verified\n");
    } else {
        printf("German keyboard layout unavailable\n");
    }

    // Input injection needs a non-IME layout, independently of the desktop's current input method.
    if (!ActivateKeyboardLayout(us, 0)) { DestroyWindow(edit); return 8; }
    const uint16_t replacement[] = {0x4e2d, 0xd83d, 0xde00};
    if (!snippetWindowsReplace(4, replacement, 3, 0)) {
        fprintf(stderr, "SendInput failed\n");
        DestroyWindow(edit);
        return 3;
    }
    ULONGLONG deadline = GetTickCount64() + 3000;
    WCHAR text[64] = {0};
    int undoStarted = 0;
    while (GetTickCount64() < deadline) {
        MSG message;
        while (PeekMessageW(&message, NULL, 0, 0, PM_REMOVE)) {
            TranslateMessage(&message);
            DispatchMessageW(&message);
        }
        GetWindowTextW(edit, text, 64);
        if (!undoStarted && lstrcmpW(text, L"before \x4e2d\xd83d\xde00") == 0) {
            const uint16_t keyword[] = {';', 'a', 'b', 'c'};
            INPUT backspace[2] = {0};
            backspace[0].type = backspace[1].type = INPUT_KEYBOARD;
            backspace[0].ki.wVk = backspace[1].ki.wVk = VK_BACK;
            backspace[1].ki.dwFlags = KEYEVENTF_KEYUP;
            if (SendInput(2, backspace, sizeof(INPUT)) != 2 || !snippetWindowsReplace(1, keyword, 4, 0)) {
                fprintf(stderr, "undo injection failed\n");
                DestroyWindow(edit);
                return 3;
            }
            undoStarted = 1;
        } else if (undoStarted && lstrcmpW(text, L"before ;abc") == 0) {
            HWND password = CreateWindowExW(0, L"EDIT", L"secret", WS_OVERLAPPEDWINDOW | WS_VISIBLE | ES_PASSWORD,
                                            CW_USEDEFAULT, CW_USEDEFAULT, 420, 100, NULL, NULL, GetModuleHandleW(NULL), NULL);
            if (!password) { DestroyWindow(edit); return 6; }
            if (!focusTestWindow(password) || checkFocusedControlWithPump() != 0) {
                fprintf(stderr, "password control was not safely rejected\n");
                DestroyWindow(password);
                DestroyWindow(edit);
                return 7;
            }
            DestroyWindow(password);
            DestroyWindow(edit);
            if (!testEditUndo(L"Wox line one\r\nWox line two", 24)) { fprintf(stderr, "multiline undo failed\n"); return 11; }
            if (!testEditUndo(L"a\nb", 2)) { fprintf(stderr, "LF undo failed\n"); return 15; }
            if (!testEditUndo(L"a\rb", 2)) { fprintf(stderr, "CR undo failed\n"); return 16; }
            if (!testEditUndo(L"A\xd83d\xdc69\x200d\xd83d\xdcbb", 3)) { fprintf(stderr, "joined emoji undo failed\n"); return 12; }
            if (!testEditUndo(L"e\x0301", 1)) { fprintf(stderr, "combining accent undo failed\n"); return 13; }
            if (!testEditUndo(L"\xd83c\xddfa\xd83c\xddf8", 1)) { fprintf(stderr, "flag undo failed\n"); return 14; }
            return 0;
        }
        Sleep(10);
    }
    fprintf(stderr, "edit control did not reach the expected text\n");
    DestroyWindow(edit);
    return 4;
}
