#include <windows.h>
#include <stdint.h>
#include <stdlib.h>

extern void woxSnippetPointerEvent(uintptr_t callback);

typedef struct SnippetPointer {
    HANDLE thread;
    HANDLE ready;
    DWORD thread_id;
    HHOOK hook;
    uintptr_t callback;
} SnippetPointer;

static SnippetPointer *activePointer = NULL;

static LRESULT CALLBACK snippetPointerProc(int code, WPARAM kind, LPARAM data) {
    if (code == HC_ACTION && activePointer &&
        (kind == WM_LBUTTONDOWN || kind == WM_RBUTTONDOWN || kind == WM_MBUTTONDOWN ||
         kind == WM_XBUTTONDOWN || kind == WM_MOUSEWHEEL || kind == WM_MOUSEHWHEEL)) {
        woxSnippetPointerEvent(activePointer->callback);
    }
    return CallNextHookEx(NULL, code, kind, data);
}

static DWORD WINAPI snippetPointerThread(LPVOID arg) {
    SnippetPointer *pointer = (SnippetPointer *)arg;
    MSG message;
    PeekMessageW(&message, NULL, WM_USER, WM_USER, PM_NOREMOVE);
    activePointer = pointer;
    pointer->hook = SetWindowsHookExW(WH_MOUSE_LL, snippetPointerProc, GetModuleHandleW(NULL), 0);
    SetEvent(pointer->ready);
    if (!pointer->hook) { activePointer = NULL; return 0; }
    while (GetMessageW(&message, NULL, 0, 0) > 0) {
        TranslateMessage(&message);
        DispatchMessageW(&message);
    }
    UnhookWindowsHookEx(pointer->hook);
    activePointer = NULL;
    return 0;
}

SnippetPointer *snippetPointerStart(uintptr_t callback) {
    SnippetPointer *pointer = calloc(1, sizeof(*pointer));
    if (!pointer) return NULL;
    pointer->callback = callback;
    pointer->ready = CreateEventW(NULL, TRUE, FALSE, NULL);
    if (!pointer->ready) { free(pointer); return NULL; }
    pointer->thread = CreateThread(NULL, 0, snippetPointerThread, pointer, 0, &pointer->thread_id);
    if (!pointer->thread) { CloseHandle(pointer->ready); free(pointer); return NULL; }
    WaitForSingleObject(pointer->ready, INFINITE);
    CloseHandle(pointer->ready);
    if (!pointer->hook) { WaitForSingleObject(pointer->thread, INFINITE); CloseHandle(pointer->thread); free(pointer); return NULL; }
    return pointer;
}

void snippetPointerStop(SnippetPointer *pointer) {
    if (!pointer) return;
    PostThreadMessageW(pointer->thread_id, WM_QUIT, 0, 0);
    WaitForSingleObject(pointer->thread, INFINITE);
    CloseHandle(pointer->thread);
    free(pointer);
}
