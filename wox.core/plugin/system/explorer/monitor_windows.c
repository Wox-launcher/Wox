#define _WIN32_WINNT 0x0600
#include <windows.h>
#include <wchar.h>

extern void fileExplorerActivatedCallbackCGO(int pid);

static HWINEVENTHOOK gForegroundHook = NULL;
static HANDLE gMonitorThread = NULL;
static DWORD gMonitorThreadId = 0;
static DWORD gLastExplorerPid = 0;
static HWND gLastExplorerHwnd = NULL;

static int isExplorerProcess(DWORD pid) {
    if (pid == 0) {
        return 0;
    }

    HANDLE process = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid);
    if (!process) {
        return 0;
    }

    WCHAR path[MAX_PATH];
    DWORD size = (DWORD)(sizeof(path) / sizeof(path[0]));
    int isExplorer = 0;

    if (QueryFullProcessImageNameW(process, 0, path, &size)) {
        const WCHAR *base = wcsrchr(path, L'\\');
        base = base ? base + 1 : path;
        if (_wcsicmp(base, L"explorer.exe") == 0) {
            isExplorer = 1;
        }
    }

    CloseHandle(process);
    return isExplorer;
}

static int isExplorerWindow(HWND hwnd) {
    if (!hwnd) {
        return 0;
    }

    WCHAR className[128];
    int len = GetClassNameW(hwnd, className, (int)(sizeof(className) / sizeof(className[0])));
    if (len <= 0) {
        return 0;
    }

    if (_wcsicmp(className, L"CabinetWClass") == 0) {
        return 1;
    }

    if (_wcsicmp(className, L"ExploreWClass") == 0) {
        return 1;
    }

    return 0;
}

static void CALLBACK foregroundChangedProc(
    HWINEVENTHOOK hook,
    DWORD event,
    HWND hwnd,
    LONG idObject,
    LONG idChild,
    DWORD eventThread,
    DWORD eventTime) {
    if (event != EVENT_SYSTEM_FOREGROUND) {
        return;
    }

    if (!hwnd) {
        return;
    }

    if (!isExplorerWindow(hwnd)) {
        gLastExplorerPid = 0;
        gLastExplorerHwnd = NULL;
        return;
    }

    DWORD pid = 0;
    GetWindowThreadProcessId(hwnd, &pid);
    if (pid == 0 || !isExplorerProcess(pid)) {
        gLastExplorerPid = 0;
        gLastExplorerHwnd = NULL;
        return;
    }

    if (hwnd == gLastExplorerHwnd) {
        return;
    }

    if (pid == gLastExplorerPid) {
        gLastExplorerHwnd = hwnd;
        return;
    }

    gLastExplorerPid = pid;
    gLastExplorerHwnd = hwnd;
    fileExplorerActivatedCallbackCGO((int)pid);
}

static DWORD WINAPI monitorThreadProc(LPVOID param) {
    MSG msg;
    PeekMessageW(&msg, NULL, WM_USER, WM_USER, PM_NOREMOVE);

    gForegroundHook = SetWinEventHook(
        EVENT_SYSTEM_FOREGROUND,
        EVENT_SYSTEM_FOREGROUND,
        NULL,
        foregroundChangedProc,
        0,
        0,
        WINEVENT_OUTOFCONTEXT | WINEVENT_SKIPOWNPROCESS);

    HWND hwnd = GetForegroundWindow();
    if (hwnd && isExplorerWindow(hwnd)) {
        DWORD pid = 0;
        GetWindowThreadProcessId(hwnd, &pid);
        if (isExplorerProcess(pid)) {
            gLastExplorerPid = pid;
            gLastExplorerHwnd = hwnd;
            fileExplorerActivatedCallbackCGO((int)pid);
        }
    }

    while (GetMessageW(&msg, NULL, 0, 0) > 0) {
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }

    if (gForegroundHook) {
        UnhookWinEvent(gForegroundHook);
        gForegroundHook = NULL;
    }

    gLastExplorerPid = 0;
    gLastExplorerHwnd = NULL;
    return 0;
}

void startFileExplorerMonitor() {
    if (gMonitorThread) {
        return;
    }

    gMonitorThread = CreateThread(
        NULL,
        0,
        monitorThreadProc,
        NULL,
        0,
        &gMonitorThreadId);
}

void stopFileExplorerMonitor() {
    if (gMonitorThreadId != 0) {
        PostThreadMessageW(gMonitorThreadId, WM_QUIT, 0, 0);
    }
    if (gMonitorThread) {
        WaitForSingleObject(gMonitorThread, 1000);
        CloseHandle(gMonitorThread);
        gMonitorThread = NULL;
    }
    gMonitorThreadId = 0;
    gLastExplorerPid = 0;
    gLastExplorerHwnd = NULL;
}
