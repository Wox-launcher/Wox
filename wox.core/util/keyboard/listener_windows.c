#define _WIN32_WINNT 0x0600
#include <windows.h>

extern void keyboardHotkeyTriggeredCGO(int id);
extern int keyboardHookEventCGO(int eventKind, unsigned int vkCode, unsigned int modifiers);

#define WM_WOX_KEYBOARD_REQUEST (WM_APP + 71)

enum requestAction
{
    requestRegisterHotkey = 1,
    requestUnregisterHotkey = 2,
    requestSetRawHook = 3
};

typedef struct
{
    int action;
    int id;
    UINT modifiers;
    UINT vkCode;
    int enabled;
    HANDLE done;
    int ok;
    DWORD errorCode;
} KeyboardRequest;

static HANDLE gKeyboardThread = NULL;
static DWORD gKeyboardThreadId = 0;
static HHOOK gRawKeyboardHook = NULL;

static void clearKeyboardThreadHandle(void)
{
    if (gKeyboardThread)
    {
        CloseHandle(gKeyboardThread);
    }
    gKeyboardThread = NULL;
    gKeyboardThreadId = 0;
    gRawKeyboardHook = NULL;
}

static void resetKeyboardThreadIfExited(void)
{
    if (!gKeyboardThread)
    {
        return;
    }

    DWORD waitResult = WaitForSingleObject(gKeyboardThread, 0);
    if (waitResult == WAIT_OBJECT_0 || waitResult == WAIT_FAILED)
    {
        // Dev rebuilds can leave Go-side state alive while the native message
        // thread has exited. Drop the stale handle so the next request can
        // recreate the low-level keyboard hook instead of failing with
        // ERROR_INVALID_THREAD_ID.
        clearKeyboardThreadHandle();
    }
}

static UINT toWindowsModifierMask(UINT modifiers)
{
    UINT nativeMask = 0;
    if (modifiers & 1)
    {
        nativeMask |= MOD_CONTROL;
    }
    if (modifiers & 2)
    {
        nativeMask |= MOD_SHIFT;
    }
    if (modifiers & 4)
    {
        nativeMask |= MOD_ALT;
    }
    if (modifiers & 8)
    {
        nativeMask |= MOD_WIN;
    }
#ifdef MOD_NOREPEAT
    nativeMask |= MOD_NOREPEAT;
#endif
    return nativeMask;
}

static UINT currentModifierMask(void)
{
    UINT modifiers = 0;
    if (GetAsyncKeyState(VK_CONTROL) & 0x8000)
    {
        modifiers |= 1;
    }
    if (GetAsyncKeyState(VK_SHIFT) & 0x8000)
    {
        modifiers |= 2;
    }
    if (GetAsyncKeyState(VK_MENU) & 0x8000)
    {
        modifiers |= 4;
    }
    if ((GetAsyncKeyState(VK_LWIN) & 0x8000) || (GetAsyncKeyState(VK_RWIN) & 0x8000))
    {
        modifiers |= 8;
    }
    return modifiers;
}

static LRESULT CALLBACK lowLevelKeyboardProc(int nCode, WPARAM wParam, LPARAM lParam)
{
    if (nCode == HC_ACTION)
    {
        KBDLLHOOKSTRUCT *event = (KBDLLHOOKSTRUCT *)lParam;
        int eventKind = -1;
        if (wParam == WM_KEYDOWN || wParam == WM_SYSKEYDOWN)
        {
            eventKind = 0;
        }
        else if (wParam == WM_KEYUP || wParam == WM_SYSKEYUP)
        {
            eventKind = 1;
        }

        if (eventKind != -1)
        {
            int consume = keyboardHookEventCGO(eventKind, event->vkCode, currentModifierMask());
            if (consume != 0)
            {
                return 1;
            }
        }
    }

    return CallNextHookEx(NULL, nCode, wParam, lParam);
}

static void handleRequest(KeyboardRequest *request)
{
    request->ok = 0;
    request->errorCode = 0;

    if (request->action == requestRegisterHotkey)
    {
        if (RegisterHotKey(NULL, request->id, toWindowsModifierMask(request->modifiers), request->vkCode))
        {
            request->ok = 1;
        }
        else
        {
            request->errorCode = GetLastError();
        }
    }
    else if (request->action == requestUnregisterHotkey)
    {
        if (UnregisterHotKey(NULL, request->id))
        {
            request->ok = 1;
        }
        else
        {
            request->errorCode = GetLastError();
        }
    }
    else if (request->action == requestSetRawHook)
    {
        if (request->enabled)
        {
            if (!gRawKeyboardHook)
            {
                gRawKeyboardHook = SetWindowsHookEx(WH_KEYBOARD_LL, lowLevelKeyboardProc, GetModuleHandle(NULL), 0);
                if (!gRawKeyboardHook)
                {
                    request->errorCode = GetLastError();
                }
            }
            request->ok = gRawKeyboardHook != NULL;
        }
        else
        {
            if (gRawKeyboardHook)
            {
                if (!UnhookWindowsHookEx(gRawKeyboardHook))
                {
                    request->errorCode = GetLastError();
                    SetEvent(request->done);
                    return;
                }
                gRawKeyboardHook = NULL;
            }
            request->ok = 1;
        }
    }

    SetEvent(request->done);
}

static DWORD WINAPI keyboardThreadProc(LPVOID param)
{
    MSG msg;
    PeekMessageW(&msg, NULL, WM_USER, WM_USER, PM_NOREMOVE);

    while (GetMessageW(&msg, NULL, 0, 0) > 0)
    {
        if (msg.message == WM_WOX_KEYBOARD_REQUEST)
        {
            handleRequest((KeyboardRequest *)msg.lParam);
            continue;
        }

        if (msg.message == WM_HOTKEY)
        {
            keyboardHotkeyTriggeredCGO((int)msg.wParam);
            continue;
        }

        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }

    if (gRawKeyboardHook)
    {
        UnhookWindowsHookEx(gRawKeyboardHook);
        gRawKeyboardHook = NULL;
    }
    return 0;
}

int woxKeyboardEnsureThread(void)
{
    resetKeyboardThreadIfExited();

    if (gKeyboardThread)
    {
        return 1;
    }

    gKeyboardThread = CreateThread(NULL, 0, keyboardThreadProc, NULL, 0, &gKeyboardThreadId);
    if (!gKeyboardThread)
    {
        gKeyboardThreadId = 0;
        return 0;
    }

    int attempts = 0;
    while (PostThreadMessageW(gKeyboardThreadId, WM_NULL, 0, 0) == 0)
    {
        if (WaitForSingleObject(gKeyboardThread, 0) == WAIT_OBJECT_0)
        {
            clearKeyboardThreadHandle();
            return 0;
        }
        if (++attempts > 200)
        {
            clearKeyboardThreadHandle();
            return 0;
        }
        Sleep(5);
    }
    return 1;
}

static int sendRequest(KeyboardRequest *request)
{
    if (!woxKeyboardEnsureThread())
    {
        request->errorCode = GetLastError();
        return 0;
    }

    request->done = CreateEventW(NULL, FALSE, FALSE, NULL);
    if (!request->done)
    {
        request->errorCode = GetLastError();
        return 0;
    }

    if (!PostThreadMessageW(gKeyboardThreadId, WM_WOX_KEYBOARD_REQUEST, 0, (LPARAM)request))
    {
        request->errorCode = GetLastError();
        if (request->errorCode == ERROR_INVALID_THREAD_ID)
        {
            clearKeyboardThreadHandle();
            if (woxKeyboardEnsureThread() && PostThreadMessageW(gKeyboardThreadId, WM_WOX_KEYBOARD_REQUEST, 0, (LPARAM)request))
            {
                WaitForSingleObject(request->done, INFINITE);
                CloseHandle(request->done);
                request->done = NULL;
                return request->ok;
            }
            request->errorCode = GetLastError();
        }
        CloseHandle(request->done);
        request->done = NULL;
        return 0;
    }

    WaitForSingleObject(request->done, INFINITE);
    CloseHandle(request->done);
    request->done = NULL;
    return request->ok;
}

int woxKeyboardRegisterHotkey(int id, unsigned int modifiers, unsigned int vkCode, unsigned long *errorCodeOut)
{
    KeyboardRequest request;
    ZeroMemory(&request, sizeof(request));
    request.action = requestRegisterHotkey;
    request.id = id;
    request.modifiers = modifiers;
    request.vkCode = vkCode;
    request.ok = 0;

    int ok = sendRequest(&request);
    if (errorCodeOut)
    {
        *errorCodeOut = request.errorCode;
    }
    return ok;
}

int woxKeyboardUnregisterHotkey(int id, unsigned long *errorCodeOut)
{
    KeyboardRequest request;
    ZeroMemory(&request, sizeof(request));
    request.action = requestUnregisterHotkey;
    request.id = id;
    request.ok = 0;

    int ok = sendRequest(&request);
    if (errorCodeOut)
    {
        *errorCodeOut = request.errorCode;
    }
    return ok;
}

int woxKeyboardSetRawKeyboardHookEnabled(int enabled, unsigned long *errorCodeOut)
{
    KeyboardRequest request;
    ZeroMemory(&request, sizeof(request));
    request.action = requestSetRawHook;
    request.enabled = enabled;
    request.ok = 0;

    int ok = sendRequest(&request);
    if (errorCodeOut)
    {
        *errorCodeOut = request.errorCode;
    }
    return ok;
}
