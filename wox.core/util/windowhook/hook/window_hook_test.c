// Build with gcc -o window_hook_test.exe window_hook_test.c -lole32 -lshell32 -luuid -lcomctl32.
#define COBJMACROS
#define _WIN32_WINNT 0x0600
#include <windows.h>
#include <assert.h>

static HWND hiddenOverlay;
static int hideCount;

// Record the asynchronous hide request without creating any visible test windows.
static BOOL WINAPI recordShowWindowAsync(HWND hwnd, int command)
{
    assert(command == SW_HIDE);
    hiddenOverlay = hwnd;
    hideCount++;
    return TRUE;
}

#define ShowWindowAsync recordShowWindowAsync
#include "window_hook.c"
#undef ShowWindowAsync

// Exercise the real subclass using message-only windows, including destruction.
int main(void)
{
    HWND target = CreateWindowExW(0, L"STATIC", L"", 0, 0, 0, 0, 0, HWND_MESSAGE, NULL, NULL, NULL);
    HWND overlay = CreateWindowExW(0, L"STATIC", L"", 0, 0, 0, 0, 0, HWND_MESSAGE, NULL, NULL, NULL);
    assert(target && overlay);
    assert(SetWindowSubclass(target, stickySubclassProc, (UINT_PTR)overlay, (DWORD_PTR)overlay));

    SendMessageW(target, WM_KILLFOCUS, 0, 0);
    SendMessageW(target, WM_SHOWWINDOW, TRUE, 0);
    WINDOWPOS position = {0};
    position.hwnd = target;
    position.flags = SWP_NOMOVE | SWP_NOSIZE | SWP_NOZORDER;
    SendMessageW(target, WM_WINDOWPOSCHANGING, 0, (LPARAM)&position);
    assert(hideCount == 0);

    // A show without resizing must still request authoritative positioning;
    // ordinary moves must not queue repeated Go relayouts while dragging.
    MSG notification;
    while (PeekMessageW(&notification, overlay, getStickyChangedMessage(), getStickyChangedMessage(), PM_REMOVE)) {}
    SendMessageW(target, WM_WINDOWPOSCHANGED, 0, (LPARAM)&position);
    assert(!PeekMessageW(&notification, overlay, getStickyChangedMessage(), getStickyChangedMessage(), PM_REMOVE));
    position.flags |= SWP_SHOWWINDOW;
    SendMessageW(target, WM_WINDOWPOSCHANGED, 0, (LPARAM)&position);
    assert(PeekMessageW(&notification, overlay, getStickyChangedMessage(), getStickyChangedMessage(), PM_REMOVE));
    position.flags &= ~SWP_SHOWWINDOW;

    position.flags |= SWP_HIDEWINDOW;
    SendMessageW(target, WM_WINDOWPOSCHANGING, 0, (LPARAM)&position);
    assert(hideCount == 1 && hiddenOverlay == overlay);
    SendMessageW(target, WM_SHOWWINDOW, FALSE, 0);
    assert(hideCount == 2);

    assert(DestroyWindow(target));
    assert(hideCount >= 4);
    assert(DestroyWindow(overlay));
    return 0;
}
