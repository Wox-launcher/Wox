#define _WIN32_WINNT 0x0600
#include <windows.h>
#include <wchar.h>
#include <ctype.h>
#include <stdio.h>

extern void fileExplorerActivatedCallbackCGO(int pid, int isFileDialog, int x, int y, int w, int h);
extern void fileExplorerDeactivatedCallbackCGO();
extern void fileExplorerKeyDownCallbackCGO(char key);

static HWINEVENTHOOK gForegroundHook = NULL;
static HWINEVENTHOOK gObjectShowHook = NULL;
static HHOOK gKeyboardHook = NULL;
static HANDLE gMonitorThread = NULL;
static DWORD gMonitorThreadId = 0;
static DWORD gLastExplorerPid = 0;
static HWND gLastExplorerHwnd = NULL;


typedef struct {
    BOOL found;
} FindChildClassData;

static BOOL CALLBACK EnumChildClassProc(HWND hwnd, LPARAM lParam) {
    FindChildClassData *data = (FindChildClassData *)lParam;
    WCHAR className[256];
    if (GetClassNameW(hwnd, className, 256) == 0) {
        return TRUE;
    }

    if (_wcsicmp(className, L"DUIViewWndClassName") == 0 || _wcsicmp(className, L"DirectUIHWND") == 0) {
        data->found = TRUE;
        return FALSE;
    }

    return TRUE;
}

static int isOpenSaveDialog(HWND hwnd) {
    if (!hwnd) {
        return 0;
    }

    WCHAR className[256];
    if (GetClassNameW(hwnd, className, 256) == 0) {
        return 0;
    }

    if (_wcsicmp(className, L"#32770") != 0) {
        return 0;
    }

    FindChildClassData data;
    data.found = FALSE;
    EnumChildWindows(hwnd, EnumChildClassProc, (LPARAM)&data);
    return data.found ? 1 : 0;
}

static LRESULT CALLBACK LowLevelKeyboardProc(int nCode, WPARAM wParam, LPARAM lParam) {
    if (nCode == HC_ACTION) {
        if (wParam == WM_KEYDOWN) {
            KBDLLHOOKSTRUCT *p = (KBDLLHOOKSTRUCT *)lParam;
            DWORD vkCode = p->vkCode;

            // Ignore special keys (Ctrl, Alt)
            if (GetAsyncKeyState(VK_CONTROL) & 0x8000) return CallNextHookEx(NULL, nCode, wParam, lParam);
            if (GetAsyncKeyState(VK_MENU) & 0x8000) return CallNextHookEx(NULL, nCode, wParam, lParam);
            
            // Map VK to Char
            // Basic mapping for A-Z, 0-9
            if ((vkCode >= 0x41 && vkCode <= 0x5A) || // A-Z
                (vkCode >= 0x30 && vkCode <= 0x39)) { // 0-9
                
                 char c = (char)vkCode;
                 fprintf(stderr, "[Monitor] Key pressed: %c (vk=%lu)\n", c, vkCode);
                 fflush(stderr);
                 fileExplorerKeyDownCallbackCGO(c);
            }
        }
    }
    return CallNextHookEx(NULL, nCode, wParam, lParam);
}


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
        // Use single quotes L'\\' for character literal
        const WCHAR *base = wcsrchr(path, L'\\');
        base = base ? base + 1 : path;
        if (_wcsicmp(base, L"explorer.exe") == 0) {
            isExplorer = 1;
        }
    }

    CloseHandle(process);
    return isExplorer;
}

static int classifyExplorerWindow(HWND hwnd) {
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

    if (_wcsicmp(className, L"Progman") == 0) {
        return -1;
    }

    if (_wcsicmp(className, L"WorkerW") == 0) {
        return -1;
    }

    if (_wcsicmp(className, L"Shell_TrayWnd") == 0) {
        return -1;
    }

    if (_wcsicmp(className, L"Shell_SecondaryTrayWnd") == 0) {
        return -1;
    }

    return 0;
}

static void updateHooksForExplorer(int isExplorerActive) {
    if (isExplorerActive) {
        if (!gKeyboardHook) {
            fprintf(stderr, "[Monitor] Installing WH_KEYBOARD_LL hook...\n");
            fflush(stderr);
            
            gKeyboardHook = SetWindowsHookEx(WH_KEYBOARD_LL, LowLevelKeyboardProc, GetModuleHandle(NULL), 0);
            
            if (!gKeyboardHook) {
                fprintf(stderr, "[Monitor] Failed to install keyboard hook: %lu\n", GetLastError());
                fflush(stderr);
            } else {
                fprintf(stderr, "[Monitor] Keyboard hook installed.\n");
                fflush(stderr);
            }
        }
    } else {
        if (gKeyboardHook) {
            fprintf(stderr, "[Monitor] Removing WH_KEYBOARD_LL hook\n");  
            fflush(stderr);
            UnhookWindowsHookEx(gKeyboardHook);
            gKeyboardHook = NULL;
        }
    }
}

static void triggerActivation(HWND hwnd, DWORD pid, int isDialog) {
    RECT rect;
    if (GetWindowRect(hwnd, &rect)) {
        int x = rect.left;
        int y = rect.top;
        int w = rect.right - rect.left;
        int h = rect.bottom - rect.top;
        fileExplorerActivatedCallbackCGO((int)pid, isDialog, x, y, w, h);
    } else {
        // Fallback if GetWindowRect fails
        fileExplorerActivatedCallbackCGO((int)pid, isDialog, 0, 0, 0, 0);
    }
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

    int classResult = classifyExplorerWindow(hwnd);
    if (classResult == -1) {
        if (gLastExplorerPid != 0) {
            gLastExplorerPid = 0;
            gLastExplorerHwnd = NULL;
            updateHooksForExplorer(0);
            fileExplorerDeactivatedCallbackCGO();
        }
        return;
    }

    DWORD pid = 0;
    GetWindowThreadProcessId(hwnd, &pid);
    
    int isValid = 0;
    if (pid != 0) {
        if (isExplorerProcess(pid)) {
            if (classResult == 1) {
                isValid = 1;
            }
        } else if (isOpenSaveDialog(hwnd)) {
            isValid = 1;
        }
        
        // If not valid yet, check if it's a dialog inside explorer process
        if (!isValid && pid != 0 && isExplorerProcess(pid) && isOpenSaveDialog(hwnd)) {
            isValid = 1;
        }
    }

    if (!isValid) {
        if (gLastExplorerPid != 0) {
            gLastExplorerPid = 0;
            gLastExplorerHwnd = NULL;
            updateHooksForExplorer(0);
            fileExplorerDeactivatedCallbackCGO();
        }
        return;
    }

    updateHooksForExplorer(1);

    if (hwnd == gLastExplorerHwnd) {
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
    DWORD eventTime) {
    if (event != EVENT_OBJECT_SHOW) {
        return;
    }

    if (!hwnd) {
        return;
    }

    if (idObject != OBJID_WINDOW || idChild != 0) {
        return;
    }

    int classResult = classifyExplorerWindow(hwnd);
    if (classResult == -1) {
        return;
    }

    if (GetForegroundWindow() != hwnd) {
        return;
    }

    DWORD pid = 0;
    GetWindowThreadProcessId(hwnd, &pid);
    
    int isValid = 0;
    if (pid != 0) {
        if (isExplorerProcess(pid)) {
            if (classResult == 1) {
                isValid = 1;
            }
        } else if (isOpenSaveDialog(hwnd)) {
            isValid = 1;
        }

        // If not valid yet, check if it's a dialog inside explorer process
        if (!isValid && pid != 0 && isExplorerProcess(pid) && isOpenSaveDialog(hwnd)) {
            isValid = 1;
        }
    }

    if (!isValid) {
        return;
    }

    updateHooksForExplorer(1);

    if (hwnd == gLastExplorerHwnd) {
        return;
    }

    gLastExplorerPid = pid;
    gLastExplorerHwnd = hwnd;
    triggerActivation(hwnd, pid, isOpenSaveDialog(hwnd) ? 1 : 0);
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

    gObjectShowHook = SetWinEventHook(
        EVENT_OBJECT_SHOW,
        EVENT_OBJECT_SHOW,
        NULL,
        objectShowProc,
        0,
        0,
        WINEVENT_OUTOFCONTEXT | WINEVENT_SKIPOWNPROCESS);

    HWND hwnd = GetForegroundWindow();
    int initialValid = 0;
    if (hwnd && classifyExplorerWindow(hwnd) != -1) {
        DWORD pid = 0;
        GetWindowThreadProcessId(hwnd, &pid);
        
        int isValid = 0;
        if (pid != 0) {
            if (isExplorerProcess(pid)) {
                if (classifyExplorerWindow(hwnd) == 1) { // Re-check class result for initial window
                     isValid = 1;
                }
            } else if (isOpenSaveDialog(hwnd)) {
                isValid = 1;
            }

            // If not valid yet, check if it's a dialog inside explorer process
            if (!isValid && pid != 0 && isExplorerProcess(pid) && isOpenSaveDialog(hwnd)) {
                isValid = 1;
            }
        }

        if (isValid) {
            initialValid = 1;
            updateHooksForExplorer(1);
            gLastExplorerPid = pid;
            gLastExplorerHwnd = hwnd;
            triggerActivation(hwnd, pid, isOpenSaveDialog(hwnd) ? 1 : 0);
        }
    }
    
    if (!initialValid) {
        updateHooksForExplorer(0);
    }

    while (GetMessageW(&msg, NULL, 0, 0) > 0) {
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }

    if (gForegroundHook) {
        UnhookWinEvent(gForegroundHook);
        gForegroundHook = NULL;
    }

    if (gObjectShowHook) {
        UnhookWinEvent(gObjectShowHook);
        gObjectShowHook = NULL;
    }
    
    if (gKeyboardHook) {
        UnhookWindowsHookEx(gKeyboardHook);
        gKeyboardHook = NULL;
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
