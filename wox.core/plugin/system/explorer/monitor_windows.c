#define _WIN32_WINNT 0x0600
#include <windows.h>
#include <wchar.h>
#include <ctype.h>
#include <stdio.h>
#include <stdarg.h>

extern void fileExplorerActivatedCallbackCGO(int pid, int isFileDialog, int x, int y, int w, int h);
extern void fileExplorerDeactivatedCallbackCGO();
extern void fileExplorerKeyDownCallbackCGO(char key);
extern void fileExplorerLogCallbackCGO(char *msg);

static HWINEVENTHOOK gForegroundHook = NULL;
static HWINEVENTHOOK gObjectShowHook = NULL;
static HHOOK gKeyboardHook = NULL;
static HANDLE gMonitorThread = NULL;
static DWORD gMonitorThreadId = 0;
static DWORD gLastExplorerPid = 0;
static HWND gLastExplorerHwnd = NULL;
static DWORD gLastKeyLogTick = 0;
static DWORD gLastEnsureActivateTick = 0;

static void ensureForegroundActivation();

static void logMessage(const char *fmt, ...)
{
    char buffer[512];
    va_list args;
    va_start(args, fmt);
    _vsnprintf_s(buffer, sizeof(buffer), _TRUNCATE, fmt, args);
    va_end(args);
    fileExplorerLogCallbackCGO(buffer);
}

static void getWindowClassNameUtf8(HWND hwnd, char *out, size_t outLen)
{
    if (!out || outLen == 0)
        return;
    out[0] = '\0';
    if (!hwnd)
        return;
    WCHAR className[256];
    if (GetClassNameW(hwnd, className, 256) == 0)
    {
        return;
    }
    WideCharToMultiByte(CP_UTF8, 0, className, -1, out, (int)outLen, NULL, NULL);
}

static void getProcessImageBaseNameUtf8(DWORD pid, char *out, size_t outLen)
{
    if (!out || outLen == 0)
        return;
    out[0] = '\0';
    if (pid == 0)
        return;
    HANDLE process = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid);
    if (!process)
    {
        return;
    }
    WCHAR path[MAX_PATH];
    DWORD size = (DWORD)(sizeof(path) / sizeof(path[0]));
    if (QueryFullProcessImageNameW(process, 0, path, &size))
    {
        const WCHAR *base = wcsrchr(path, L'\\');
        base = base ? base + 1 : path;
        WideCharToMultiByte(CP_UTF8, 0, base, -1, out, (int)outLen, NULL, NULL);
    }
    CloseHandle(process);
}

typedef struct
{
    BOOL found;
} FindChildClassData;

static BOOL CALLBACK EnumChildClassProc(HWND hwnd, LPARAM lParam)
{
    FindChildClassData *data = (FindChildClassData *)lParam;
    WCHAR className[256];
    if (GetClassNameW(hwnd, className, 256) == 0)
    {
        logMessage("EnumChildClassProc: GetClassNameW failed err=%lu", GetLastError());
        return TRUE;
    }

    if (_wcsicmp(className, L"DUIViewWndClassName") == 0 || _wcsicmp(className, L"DirectUIHWND") == 0)
    {
        data->found = TRUE;
        return FALSE;
    }

    return TRUE;
}

static int isOpenSaveDialog(HWND hwnd)
{
    if (!hwnd)
    {
        return 0;
    }

    WCHAR className[256];
    if (GetClassNameW(hwnd, className, 256) == 0)
    {
        logMessage("isOpenSaveDialog: GetClassNameW failed err=%lu", GetLastError());
        return 0;
    }

    if (_wcsicmp(className, L"#32770") != 0)
    {
        return 0;
    }

    FindChildClassData data;
    data.found = FALSE;
    EnumChildWindows(hwnd, EnumChildClassProc, (LPARAM)&data);
    return data.found ? 1 : 0;
}

static LRESULT CALLBACK LowLevelKeyboardProc(int nCode, WPARAM wParam, LPARAM lParam)
{
    if (nCode == HC_ACTION)
    {
        if (wParam == WM_KEYDOWN)
        {
            ensureForegroundActivation();
            KBDLLHOOKSTRUCT *p = (KBDLLHOOKSTRUCT *)lParam;
            DWORD vkCode = p->vkCode;

            // Ignore special keys (Ctrl, Alt)
            if (GetAsyncKeyState(VK_CONTROL) & 0x8000)
            {
                DWORD now = GetTickCount();
                if (now - gLastKeyLogTick > 1000)
                {
                    gLastKeyLogTick = now;
                    logMessage("LowLevelKeyboardProc: ignore key vk=0x%02lX (CTRL down)", vkCode);
                }
                return CallNextHookEx(NULL, nCode, wParam, lParam);
            }
            if (GetAsyncKeyState(VK_MENU) & 0x8000)
            {
                DWORD now = GetTickCount();
                if (now - gLastKeyLogTick > 1000)
                {
                    gLastKeyLogTick = now;
                    logMessage("LowLevelKeyboardProc: ignore key vk=0x%02lX (ALT down)", vkCode);
                }
                return CallNextHookEx(NULL, nCode, wParam, lParam);
            }

            // Map VK to Char
            // Basic mapping for A-Z, 0-9
            if ((vkCode >= 0x41 && vkCode <= 0x5A) || // A-Z
                (vkCode >= 0x30 && vkCode <= 0x39))
            { // 0-9
                char c = (char)vkCode;
                logMessage("LowLevelKeyboardProc: key vk=0x%02lX char=%c", vkCode, c);
                fileExplorerKeyDownCallbackCGO(c);
            }
            else
            {
                DWORD now = GetTickCount();
                if (now - gLastKeyLogTick > 1000)
                {
                    gLastKeyLogTick = now;
                    logMessage("LowLevelKeyboardProc: ignore key vk=0x%02lX (non-alnum)", vkCode);
                }
            }
        }
    }
    return CallNextHookEx(NULL, nCode, wParam, lParam);
}

static int isExplorerProcess(DWORD pid)
{
    if (pid == 0)
    {
        return 0;
    }

    HANDLE process = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid);
    if (!process)
    {
        logMessage("isExplorerProcess: OpenProcess failed pid=%lu err=%lu", pid, GetLastError());
        return 0;
    }

    WCHAR path[MAX_PATH];
    DWORD size = (DWORD)(sizeof(path) / sizeof(path[0]));
    int isExplorer = 0;

    if (QueryFullProcessImageNameW(process, 0, path, &size))
    {
        // Use single quotes L'\\' for character literal
        const WCHAR *base = wcsrchr(path, L'\\');
        base = base ? base + 1 : path;
        if (_wcsicmp(base, L"explorer.exe") == 0)
        {
            isExplorer = 1;
        }
    }
    else
    {
        logMessage("isExplorerProcess: QueryFullProcessImageNameW failed pid=%lu err=%lu", pid, GetLastError());
    }

    CloseHandle(process);
    return isExplorer;
}

static int classifyExplorerWindow(HWND hwnd)
{
    if (!hwnd)
    {
        return 0;
    }

    WCHAR className[128];
    int len = GetClassNameW(hwnd, className, (int)(sizeof(className) / sizeof(className[0])));
    if (len <= 0)
    {
        logMessage("classifyExplorerWindow: GetClassNameW failed err=%lu", GetLastError());
        return 0;
    }

    if (_wcsicmp(className, L"CabinetWClass") == 0)
    {
        return 1;
    }

    if (_wcsicmp(className, L"ExploreWClass") == 0)
    {
        return 1;
    }

    if (_wcsicmp(className, L"Progman") == 0)
    {
        return -1;
    }

    if (_wcsicmp(className, L"WorkerW") == 0)
    {
        return -1;
    }

    if (_wcsicmp(className, L"Shell_TrayWnd") == 0)
    {
        return -1;
    }

    if (_wcsicmp(className, L"Shell_SecondaryTrayWnd") == 0)
    {
        return -1;
    }

    return 0;
}

static void updateHooksForExplorer(int isExplorerActive)
{
    if (isExplorerActive)
    {
        if (!gKeyboardHook)
        {
            gKeyboardHook = SetWindowsHookEx(WH_KEYBOARD_LL, LowLevelKeyboardProc, GetModuleHandle(NULL), 0);
            if (!gKeyboardHook)
            {
                logMessage("SetWindowsHookEx(WH_KEYBOARD_LL) failed err=%lu", GetLastError());
            }
            else
            {
                logMessage("Keyboard hook installed");
            }
        }
    }
    else
    {
        if (gKeyboardHook)
        {
            logMessage("Keyboard hook kept (inactive)");
        }
    }
}

static void triggerActivation(HWND hwnd, DWORD pid, int isDialog)
{
    RECT rect;
    if (GetWindowRect(hwnd, &rect))
    {
        int x = rect.left;
        int y = rect.top;
        int w = rect.right - rect.left;
        int h = rect.bottom - rect.top;
        logMessage("Activated hwnd=0x%p pid=%lu dialog=%d rect=(%d,%d,%d,%d)", hwnd, pid, isDialog, x, y, w, h);
        fileExplorerActivatedCallbackCGO((int)pid, isDialog, x, y, w, h);
    }
    else
    {
        logMessage("GetWindowRect failed hwnd=0x%p err=%lu", hwnd, GetLastError());
        // Fallback if GetWindowRect fails
        fileExplorerActivatedCallbackCGO((int)pid, isDialog, 0, 0, 0, 0);
    }
}

static void ensureForegroundActivation()
{
    DWORD now = GetTickCount();
    if (now - gLastEnsureActivateTick < 200)
    {
        return;
    }
    gLastEnsureActivateTick = now;

    HWND hwnd = GetForegroundWindow();
    if (!hwnd)
    {
        return;
    }

    int classResult = classifyExplorerWindow(hwnd);
    if (classResult == -1)
    {
        return;
    }

    DWORD pid = 0;
    GetWindowThreadProcessId(hwnd, &pid);
    if (pid == 0)
    {
        return;
    }

    int isExplorer = isExplorerProcess(pid);
    int isDialog = isOpenSaveDialog(hwnd);
    int isValid = 0;
    if (isExplorer)
    {
        if (classResult != -1)
        {
            isValid = 1;
        }
    }
    else if (isDialog)
    {
        isValid = 1;
    }

    if (!isValid && isExplorer && isDialog)
    {
        isValid = 1;
    }

    if (!isValid)
    {
        return;
    }

    updateHooksForExplorer(1);

    if (hwnd == gLastExplorerHwnd && gLastExplorerPid != 0)
    {
        return;
    }

    gLastExplorerPid = pid;
    gLastExplorerHwnd = hwnd;
    logMessage("ensureForegroundActivation: reactivate hwnd=0x%p pid=%lu dialog=%d", hwnd, pid, isDialog);
    triggerActivation(hwnd, pid, isDialog ? 1 : 0);
}

static void CALLBACK foregroundChangedProc(
    HWINEVENTHOOK hook,
    DWORD event,
    HWND hwnd,
    LONG idObject,
    LONG idChild,
    DWORD eventThread,
    DWORD eventTime)
{
    if (event != EVENT_SYSTEM_FOREGROUND)
    {
        return;
    }

    if (!hwnd)
    {
        logMessage("foregroundChangedProc: hwnd is null");
        return;
    }

    int classResult = classifyExplorerWindow(hwnd);
    if (classResult == -1)
    {
        if (gLastExplorerPid != 0)
        {
            logMessage("foregroundChangedProc: deactivated (shell class)");
            gLastExplorerPid = 0;
            gLastExplorerHwnd = NULL;
            updateHooksForExplorer(0);
            fileExplorerDeactivatedCallbackCGO();
        }
        return;
    }

    DWORD pid = 0;
    GetWindowThreadProcessId(hwnd, &pid);
    if (pid == 0)
    {
        logMessage("foregroundChangedProc: GetWindowThreadProcessId returned pid=0 hwnd=0x%p", hwnd);
    }

    int isValid = 0;
    if (pid != 0)
    {
        int isExplorer = isExplorerProcess(pid);
        int isDialog = isOpenSaveDialog(hwnd);
        if (isExplorer)
        {
            if (classResult != -1)
            {
                isValid = 1;
            }
        }
        else if (isDialog)
        {
            isValid = 1;
        }

        // If not valid yet, check if it's a dialog inside explorer process
        if (!isValid && pid != 0 && isExplorer && isDialog)
        {
            isValid = 1;
        }
    }

    if (!isValid)
    {
        char className[256];
        char processName[256];
        getWindowClassNameUtf8(hwnd, className, sizeof(className));
        getProcessImageBaseNameUtf8(pid, processName, sizeof(processName));
        logMessage("foregroundChangedProc: invalid window hwnd=0x%p pid=%lu classResult=%d class=%s proc=%s", hwnd, pid, classResult, className[0] ? className : "?", processName[0] ? processName : "?");
        if (gLastExplorerPid != 0)
        {
            gLastExplorerPid = 0;
            gLastExplorerHwnd = NULL;
            updateHooksForExplorer(0);
            fileExplorerDeactivatedCallbackCGO();
        }
        return;
    }

    updateHooksForExplorer(1);

    if (hwnd == gLastExplorerHwnd)
    {
        logMessage("foregroundChangedProc: same hwnd, skip activation");
        return;
    }

    gLastExplorerPid = pid;
    gLastExplorerHwnd = hwnd;
    triggerActivation(hwnd, pid, isOpenSaveDialog(hwnd) ? 1 : 0);
}

static void CALLBACK objectShowProc(
    HWINEVENTHOOK hook,
    DWORD event,
    HWND hwnd,
    LONG idObject,
    LONG idChild,
    DWORD eventThread,
    DWORD eventTime)
{
    if (event != EVENT_OBJECT_SHOW)
    {
        return;
    }

    if (!hwnd)
    {
        logMessage("objectShowProc: hwnd is null");
        return;
    }

    if (idObject != OBJID_WINDOW || idChild != 0)
    {
        return;
    }

    int classResult = classifyExplorerWindow(hwnd);
    if (classResult == -1)
    {
        return;
    }

    if (GetForegroundWindow() != hwnd)
    {
        return;
    }

    DWORD pid = 0;
    GetWindowThreadProcessId(hwnd, &pid);
    if (pid == 0)
    {
        logMessage("objectShowProc: pid=0 hwnd=0x%p", hwnd);
    }

    int isValid = 0;
    if (pid != 0)
    {
        int isExplorer = isExplorerProcess(pid);
        int isDialog = isOpenSaveDialog(hwnd);
        if (isExplorer)
        {
            if (classResult != -1)
            {
                isValid = 1;
            }
        }
        else if (isDialog)
        {
            isValid = 1;
        }

        // If not valid yet, check if it's a dialog inside explorer process
        if (!isValid && pid != 0 && isExplorer && isDialog)
        {
            isValid = 1;
        }
    }

    if (!isValid)
    {
        char className[256];
        char processName[256];
        getWindowClassNameUtf8(hwnd, className, sizeof(className));
        getProcessImageBaseNameUtf8(pid, processName, sizeof(processName));
        logMessage("objectShowProc: invalid window hwnd=0x%p pid=%lu classResult=%d class=%s proc=%s", hwnd, pid, classResult, className[0] ? className : "?", processName[0] ? processName : "?");
        return;
    }

    updateHooksForExplorer(1);

    if (hwnd == gLastExplorerHwnd)
    {
        logMessage("objectShowProc: same hwnd, skip activation");
        return;
    }

    gLastExplorerPid = pid;
    gLastExplorerHwnd = hwnd;
    triggerActivation(hwnd, pid, isOpenSaveDialog(hwnd) ? 1 : 0);
}

static DWORD WINAPI monitorThreadProc(LPVOID param)
{
    MSG msg;
    PeekMessageW(&msg, NULL, WM_USER, WM_USER, PM_NOREMOVE);

    logMessage("monitorThreadProc: start thread=%lu", GetCurrentThreadId());

    gForegroundHook = SetWinEventHook(
        EVENT_SYSTEM_FOREGROUND,
        EVENT_SYSTEM_FOREGROUND,
        NULL,
        foregroundChangedProc,
        0,
        0,
        WINEVENT_OUTOFCONTEXT | WINEVENT_SKIPOWNPROCESS);

    if (!gForegroundHook)
    {
        logMessage("SetWinEventHook(EVENT_SYSTEM_FOREGROUND) failed err=%lu", GetLastError());
    }
    else
    {
        logMessage("Foreground WinEvent hook installed");
    }

    gObjectShowHook = SetWinEventHook(
        EVENT_OBJECT_SHOW,
        EVENT_OBJECT_SHOW,
        NULL,
        objectShowProc,
        0,
        0,
        WINEVENT_OUTOFCONTEXT | WINEVENT_SKIPOWNPROCESS);

    if (!gObjectShowHook)
    {
        logMessage("SetWinEventHook(EVENT_OBJECT_SHOW) failed err=%lu", GetLastError());
    }
    else
    {
        logMessage("ObjectShow WinEvent hook installed");
    }

    HWND hwnd = GetForegroundWindow();
    int initialValid = 0;
    if (hwnd && classifyExplorerWindow(hwnd) != -1)
    {
        DWORD pid = 0;
        GetWindowThreadProcessId(hwnd, &pid);

        int isValid = 0;
        if (pid != 0)
        {
            int isExplorer = isExplorerProcess(pid);
            int isDialog = isOpenSaveDialog(hwnd);
            if (isExplorer)
            {
                if (classifyExplorerWindow(hwnd) != -1)
                { // Re-check class result for initial window
                    isValid = 1;
                }
            }
            else if (isDialog)
            {
                isValid = 1;
            }

            // If not valid yet, check if it's a dialog inside explorer process
            if (!isValid && pid != 0 && isExplorer && isDialog)
            {
                isValid = 1;
            }
        }

        if (isValid)
        {
            initialValid = 1;
            updateHooksForExplorer(1);
            gLastExplorerPid = pid;
            gLastExplorerHwnd = hwnd;
            triggerActivation(hwnd, pid, isOpenSaveDialog(hwnd) ? 1 : 0);
        }
    }

    if (!initialValid)
    {
        logMessage("monitorThreadProc: initial window not valid");
        updateHooksForExplorer(0);
    }

    while (GetMessageW(&msg, NULL, 0, 0) > 0)
    {
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }

    logMessage("monitorThreadProc: message loop exit");

    if (gForegroundHook)
    {
        UnhookWinEvent(gForegroundHook);
        gForegroundHook = NULL;
        logMessage("Foreground WinEvent hook removed");
    }

    if (gObjectShowHook)
    {
        UnhookWinEvent(gObjectShowHook);
        gObjectShowHook = NULL;
        logMessage("ObjectShow WinEvent hook removed");
    }

    if (gKeyboardHook)
    {
        UnhookWindowsHookEx(gKeyboardHook);
        gKeyboardHook = NULL;
        logMessage("Keyboard hook removed (thread exit)");
    }

    gLastExplorerPid = 0;
    gLastExplorerHwnd = NULL;
    return 0;
}

void startFileExplorerMonitor()
{
    if (gMonitorThread)
    {
        logMessage("startFileExplorerMonitor: already running");
        return;
    }

    gMonitorThread = CreateThread(
        NULL,
        0,
        monitorThreadProc,
        NULL,
        0,
        &gMonitorThreadId);

    if (!gMonitorThread)
    {
        logMessage("CreateThread failed err=%lu", GetLastError());
        gMonitorThreadId = 0;
    }
    else
    {
        logMessage("startFileExplorerMonitor: threadId=%lu", gMonitorThreadId);
    }
}

void stopFileExplorerMonitor()
{
    if (gMonitorThreadId != 0)
    {
        PostThreadMessageW(gMonitorThreadId, WM_QUIT, 0, 0);
    }
    if (gMonitorThread)
    {
        WaitForSingleObject(gMonitorThread, 1000);
        CloseHandle(gMonitorThread);
        gMonitorThread = NULL;
        logMessage("stopFileExplorerMonitor: thread stopped");
    }
    else
    {
        logMessage("stopFileExplorerMonitor: no thread to stop");
    }
    gMonitorThreadId = 0;
    gLastExplorerPid = 0;
    gLastExplorerHwnd = NULL;
}
