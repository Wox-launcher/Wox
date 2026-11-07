#include <windows.h>
#include <stdio.h>
#include <stdlib.h>

int main(void) {
    int multiline = getenv("WOX_TEST_WINDOWS_MANUAL_MULTILINE") != NULL;
    HWND window = CreateWindowExW(0, L"STATIC", L"Wox test: type ;wx, then Backspace", WS_OVERLAPPEDWINDOW | WS_VISIBLE,
                                  CW_USEDEFAULT, CW_USEDEFAULT, 550, 120, NULL, NULL, GetModuleHandleW(NULL), NULL);
    if (!window) return 1;
    HWND edit = CreateWindowExW(WS_EX_CLIENTEDGE, L"EDIT", L"", WS_CHILD | WS_VISIBLE | (multiline ? ES_MULTILINE | ES_AUTOVSCROLL : ES_AUTOHSCROLL),
                                15, 15, 500, 60, window, NULL, GetModuleHandleW(NULL), NULL);
    if (!edit) { DestroyWindow(window); return 2; }
    HWND foreground = GetForegroundWindow();
    DWORD other = foreground ? GetWindowThreadProcessId(foreground, NULL) : 0;
    DWORD self = GetCurrentThreadId();
    BOOL attached = other && other != self && AttachThreadInput(self, other, TRUE);
    SetForegroundWindow(window);
    SetFocus(edit);
    if (attached) AttachThreadInput(self, other, FALSE);
    if (GetForegroundWindow() != window || GetFocus() != edit) { DestroyWindow(window); return 3; }
    puts("READY"); fflush(stdout);
    ULONGLONG deadline = GetTickCount64() + 90000;
    int expanded = 0;
    while (GetTickCount64() < deadline) {
        MSG message;
        while (PeekMessageW(&message, NULL, 0, 0, PM_REMOVE)) {
            if (message.message == WM_QUIT) { DestroyWindow(window); return 4; }
            TranslateMessage(&message);
            DispatchMessageW(&message);
        }
        WCHAR text[128] = {0};
        GetWindowTextW(edit, text, 128);
        if (!expanded && lstrcmpW(text, multiline ? L"Wox line one\r\nWox line two" : L"Wox test expanded") == 0) {
            expanded = 1;
            puts("EXPANDED"); fflush(stdout);
        } else if (expanded && lstrcmpW(text, L";wx") == 0) {
            puts("UNDONE"); fflush(stdout);
            DestroyWindow(window);
            return 0;
        }
        Sleep(10);
    }
    DestroyWindow(window);
    return 5;
}
