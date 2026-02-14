#define NTDDI_VERSION NTDDI_VISTA
#define _WIN32_WINNT _WIN32_WINNT_VISTA
#include <windows.h>
#include <psapi.h>
#include <shellapi.h>
#include <commdlg.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>

char *getIconData(HICON hIcon, unsigned char **iconData, int *iconSize, int *width, int *height)
{
    ICONINFO iconinfo;
    if (!GetIconInfo(hIcon, &iconinfo))
    {
        return "Failed to get icon info";
    }

    BITMAP bm;
    if (!GetObject(iconinfo.hbmColor, sizeof(BITMAP), &bm))
    {
        return "Failed to retrieve bitmap info";
    }

    *width = bm.bmWidth;
    *height = bm.bmHeight;

    HDC hdc = GetDC(NULL);
    if (!hdc)
    {
        return "Failed to get device context";
    }

    HDC hdcMem = CreateCompatibleDC(hdc);
    if (!hdcMem)
    {
        ReleaseDC(NULL, hdc);
        return "Failed to create memory device context";
    }

    HBITMAP hbmp = CreateCompatibleBitmap(hdc, *width, *height);
    if (!hbmp)
    {
        DeleteDC(hdcMem);
        ReleaseDC(NULL, hdc);
        return "Failed to create bitmap";
    }

    SelectObject(hdcMem, hbmp);
    DrawIconEx(hdcMem, 0, 0, hIcon, *width, *height, 0, NULL, DI_NORMAL);

    BITMAPINFOHEADER bi = {sizeof(BITMAPINFOHEADER), *width, -*height, 1, 32, BI_RGB};
    *iconSize = (*width) * (*height) * 4;
    *iconData = (unsigned char *)malloc(*iconSize);
    if (!*iconData)
    {
        DeleteObject(hbmp);
        DeleteDC(hdcMem);
        ReleaseDC(NULL, hdc);
        return "Failed to allocate memory for icon data";
    }

    if (!GetDIBits(hdcMem, hbmp, 0, *height, *iconData, (BITMAPINFO *)&bi, DIB_RGB_COLORS))
    {
        free(*iconData);
        DeleteObject(hbmp);
        DeleteDC(hdcMem);
        ReleaseDC(NULL, hdc);
        return "Failed to retrieve bits from bitmap";
    }

    DeleteObject(iconinfo.hbmColor);
    DeleteObject(iconinfo.hbmMask);
    DeleteObject(hbmp);
    DeleteDC(hdcMem);
    ReleaseDC(NULL, hdc);
    DestroyIcon(hIcon);

    return NULL;
}

char *getActiveWindowIcon(unsigned char **iconData, int *iconSize, int *width, int *height)
{
    HWND hwnd = GetForegroundWindow();
    if (!hwnd)
    {
        return "Unable to get active window handle";
    }

    DWORD processId;
    GetWindowThreadProcessId(hwnd, &processId);

    HANDLE hProcess = OpenProcess(PROCESS_QUERY_INFORMATION | PROCESS_VM_READ, FALSE, processId);
    if (!hProcess)
    {
        return "Unable to open process";
    }

    WCHAR exePath[MAX_PATH];
    DWORD exePathLen = MAX_PATH;
    if (0 == QueryFullProcessImageNameW(hProcess, 0, exePath, &exePathLen))
    {
        CloseHandle(hProcess);
        return "Unable to retrieve executable path";
    }

    char exePathA[MAX_PATH];
    WideCharToMultiByte(CP_ACP, 0, exePath, -1, exePathA, MAX_PATH, NULL, NULL);

    HICON hIcon;
    ExtractIconExA(exePathA, 0, &hIcon, NULL, 1);
    if (!hIcon)
    {
        CloseHandle(hProcess);
        return "Failed to extract icon from executable";
    }

    char *result = getIconData(hIcon, iconData, iconSize, width, height);
    CloseHandle(hProcess);
    return result;
}

char *getActiveWindowName()
{
    HWND hwnd = GetForegroundWindow();
    if (!hwnd)
    {
        char *result = (char *)malloc(1);
        result[0] = '\0';
        return result;
    }

    WCHAR windowTitle[1024];
    if (0 == GetWindowTextW(hwnd, windowTitle, 1024))
    {
        char *result = (char *)malloc(1);
        result[0] = '\0';
        return result;
    }

    int len = WideCharToMultiByte(CP_UTF8, 0, windowTitle, -1, NULL, 0, NULL, NULL);
    char *windowTitleA = (char *)malloc(len);
    WideCharToMultiByte(CP_UTF8, 0, windowTitle, -1, windowTitleA, len, NULL, NULL);

    return windowTitleA;
}

int getActiveWindowPid()
{
    HWND hwnd = GetForegroundWindow();
    if (!hwnd)
    {
        return 0;
    }

    DWORD processId;
    GetWindowThreadProcessId(hwnd, &processId);
    return processId;
}

typedef struct
{
    BOOL found;
} FindChildClassData;

BOOL CALLBACK EnumChildClassProc(HWND hwnd, LPARAM lParam)
{
    FindChildClassData *data = (FindChildClassData *)lParam;
    WCHAR className[256];
    if (GetClassNameW(hwnd, className, 256) == 0)
    {
        return TRUE;
    }

    if (wcscmp(className, L"DUIViewWndClassName") == 0 || wcscmp(className, L"DirectUIHWND") == 0)
    {
        data->found = TRUE;
        return FALSE;
    }

    return TRUE;
}

static int isOpenSaveDialogWindow(HWND hwnd)
{
    if (!hwnd)
    {
        return 0;
    }

    WCHAR className[256];
    if (GetClassNameW(hwnd, className, 256) == 0)
    {
        return 0;
    }

    if (wcscmp(className, L"#32770") != 0)
    {
        return 0;
    }

    FindChildClassData data;
    data.found = FALSE;
    EnumChildWindows(hwnd, EnumChildClassProc, (LPARAM)&data);
    return data.found ? 1 : 0;
}

int isOpenSaveDialog()
{
    HWND hwnd = GetForegroundWindow();
    return isOpenSaveDialogWindow(hwnd);
}

static char *dupEmptyString()
{
    char *result = (char *)malloc(1);
    if (result)
    {
        result[0] = '\0';
    }
    return result;
}

static char *copyUtf8FromWide(const WCHAR *wide)
{
    if (!wide)
    {
        return dupEmptyString();
    }

    int len = WideCharToMultiByte(CP_UTF8, 0, wide, -1, NULL, 0, NULL, NULL);
    if (len <= 0)
    {
        return dupEmptyString();
    }

    char *result = (char *)malloc((size_t)len);
    if (!result)
    {
        return dupEmptyString();
    }

    WideCharToMultiByte(CP_UTF8, 0, wide, -1, result, len, NULL, NULL);
    return result;
}

static int copyParentDirectoryPath(const WCHAR *fullPath, WCHAR *out, size_t outLen)
{
    if (!fullPath || !out || outLen == 0)
    {
        return 0;
    }

    WCHAR tmp[32768];
    size_t tmpCap = sizeof(tmp) / sizeof(tmp[0]);
    wcsncpy(tmp, fullPath, tmpCap - 1);
    tmp[tmpCap - 1] = L'\0';
    size_t len = wcslen(tmp);
    if (len == 0)
    {
        return 0;
    }

    // Strip trailing separators while keeping drive roots like C:\.
    while (len > 1 && (tmp[len - 1] == L'\\' || tmp[len - 1] == L'/'))
    {
        if (len == 3 && tmp[1] == L':')
        {
            break;
        }
        tmp[len - 1] = L'\0';
        len--;
    }

    WCHAR *lastBackslash = wcsrchr(tmp, L'\\');
    WCHAR *lastSlash = wcsrchr(tmp, L'/');
    WCHAR *lastSep = lastBackslash;
    if (lastSlash && (!lastSep || lastSlash > lastSep))
    {
        lastSep = lastSlash;
    }
    if (!lastSep)
    {
        return 0;
    }

    // Preserve UNC share roots (\\server\share) and don't strip above share level.
    if (tmp[0] == L'\\' && tmp[1] == L'\\')
    {
        WCHAR *p = tmp + 2;
        while (*p && *p != L'\\' && *p != L'/')
        {
            p++;
        }
        if (*p)
        {
            p++;
            while (*p && *p != L'\\' && *p != L'/')
            {
                p++;
            }
            if (lastSep < p)
            {
                wcsncpy(out, tmp, outLen - 1);
                out[outLen - 1] = L'\0';
                return out[0] != L'\0';
            }
        }
    }

    if (lastSep == tmp + 2 && tmp[1] == L':')
    {
        lastSep++;
    }
    else if (lastSep == tmp)
    {
        lastSep++;
    }

    *lastSep = L'\0';
    if (tmp[0] == L'\0')
    {
        return 0;
    }

    wcsncpy(out, tmp, outLen - 1);
    out[outLen - 1] = L'\0';
    return out[0] != L'\0';
}

static char *getDialogDirectoryPathByWindow(HWND hwnd)
{
    if (!isOpenSaveDialogWindow(hwnd))
    {
        return dupEmptyString();
    }

    WCHAR folderPath[32768];
    ZeroMemory(folderPath, sizeof(folderPath));
    LRESULT folderLen = SendMessageW(hwnd, CDM_GETFOLDERPATH, (WPARAM)(sizeof(folderPath) / sizeof(folderPath[0])), (LPARAM)folderPath);
    if (folderLen > 0 && folderPath[0] != L'\0')
    {
        return copyUtf8FromWide(folderPath);
    }

    WCHAR selectedPath[32768];
    ZeroMemory(selectedPath, sizeof(selectedPath));
    LRESULT selectedLen = SendMessageW(hwnd, CDM_GETFILEPATH, (WPARAM)(sizeof(selectedPath) / sizeof(selectedPath[0])), (LPARAM)selectedPath);
    if (selectedLen > 0 && selectedPath[0] != L'\0')
    {
        WCHAR parentPath[32768];
        ZeroMemory(parentPath, sizeof(parentPath));
        if (copyParentDirectoryPath(selectedPath, parentPath, sizeof(parentPath) / sizeof(parentPath[0])))
        {
            return copyUtf8FromWide(parentPath);
        }
    }

    return dupEmptyString();
}

char *getActiveFileDialogPath()
{
    HWND hwnd = GetForegroundWindow();
    if (!hwnd)
    {
        return dupEmptyString();
    }

    return getDialogDirectoryPathByWindow(hwnd);
}

char *getFileDialogPathByPid(int pid)
{
    if (pid <= 0)
    {
        return dupEmptyString();
    }

    DWORD targetPid = (DWORD)pid;
    HWND foreground = GetForegroundWindow();
    if (foreground)
    {
        DWORD fgPid = 0;
        GetWindowThreadProcessId(foreground, &fgPid);
        if (fgPid == targetPid && isOpenSaveDialogWindow(foreground))
        {
            char *path = getDialogDirectoryPathByWindow(foreground);
            if (path && path[0] != '\0')
            {
                return path;
            }
            if (path)
            {
                free(path);
            }
        }
    }

    for (int pass = 0; pass < 2; pass++)
    {
        for (HWND hwnd = GetWindow(GetDesktopWindow(), GW_CHILD); hwnd != NULL; hwnd = GetWindow(hwnd, GW_HWNDNEXT))
        {
            DWORD wndPid = 0;
            GetWindowThreadProcessId(hwnd, &wndPid);
            if (wndPid != targetPid)
            {
                continue;
            }
            if (!isOpenSaveDialogWindow(hwnd))
            {
                continue;
            }
            if (pass == 0 && (!IsWindowVisible(hwnd) || IsIconic(hwnd)))
            {
                continue;
            }

            char *path = getDialogDirectoryPathByWindow(hwnd);
            if (path && path[0] != '\0')
            {
                return path;
            }
            if (path)
            {
                free(path);
            }
        }
    }

    return dupEmptyString();
}

static void SendKey(WORD vk, BOOL isDown)
{
    INPUT input;
    ZeroMemory(&input, sizeof(INPUT));
    input.type = INPUT_KEYBOARD;
    input.ki.wVk = vk;
    if (!isDown)
    {
        input.ki.dwFlags = KEYEVENTF_KEYUP;
    }
    SendInput(1, &input, sizeof(INPUT));
}

static void SendUnicodeChar(WCHAR ch)
{
    INPUT input;
    ZeroMemory(&input, sizeof(INPUT));
    input.type = INPUT_KEYBOARD;
    input.ki.wVk = 0;
    input.ki.wScan = ch;
    input.ki.dwFlags = KEYEVENTF_UNICODE;
    SendInput(1, &input, sizeof(INPUT));

    ZeroMemory(&input, sizeof(INPUT));
    input.type = INPUT_KEYBOARD;
    input.ki.wVk = 0;
    input.ki.wScan = ch;
    input.ki.dwFlags = KEYEVENTF_UNICODE | KEYEVENTF_KEYUP;
    SendInput(1, &input, sizeof(INPUT));
}

static void SendUnicodeString(const WCHAR *text)
{
    if (!text)
    {
        return;
    }
    for (const WCHAR *p = text; *p; ++p)
    {
        SendUnicodeChar(*p);
    }
}

static HWND findFileNameEdit(HWND hDialog)
{
    // Try Common Item Dialog (Vista+) style: ComboBoxEx32 (0x47c) -> ComboBox -> Edit
    HWND hComboEx = GetDlgItem(hDialog, 0x047c); // cmb13
    if (hComboEx)
    {
        HWND hCombo = FindWindowExW(hComboEx, NULL, L"ComboBox", NULL);
        if (hCombo)
        {
            HWND hEdit = FindWindowExW(hCombo, NULL, L"Edit", NULL);
            if (hEdit)
                return hEdit;
        }
    }

    // Try old style or direct Combo (0x47c) -> Edit (if not Ex) (e.g. in some wrapped dialogs)
    HWND hCombo = GetDlgItem(hDialog, 0x047c);
    if (hCombo)
    {
        HWND hEdit = FindWindowExW(hCombo, NULL, L"Edit", NULL);
        if (hEdit)
            return hEdit;
    }

    return NULL;
}

int navigateActiveFileDialog(const char *path)
{
    if (!path || path[0] == '\0')
    {
        return 0;
    }

    if (!isOpenSaveDialog())
    {
        return 0;
    }

    HWND hwnd = GetForegroundWindow();
    if (!hwnd)
    {
        return 0;
    }

    int wlen = MultiByteToWideChar(CP_UTF8, 0, path, -1, NULL, 0);
    if (wlen <= 1)
    {
        return 0;
    }
    WCHAR *wpath = (WCHAR *)malloc(sizeof(WCHAR) * (size_t)wlen);
    if (!wpath)
    {
        return 0;
    }
    MultiByteToWideChar(CP_UTF8, 0, path, -1, wpath, wlen);

    HWND hEdit = findFileNameEdit(hwnd);
    if (hEdit)
    {
        SendMessageW(hEdit, WM_SETTEXT, 0, (LPARAM)wpath);

        // Trigger command
        HWND hButton = GetDlgItem(hwnd, IDOK);
        if (hButton)
        {
            SendMessage(hwnd, WM_COMMAND, MAKEWPARAM(IDOK, BN_CLICKED), (LPARAM)hButton);
        }
        else
        {
            PostMessage(hEdit, WM_KEYDOWN, VK_RETURN, 0);
            PostMessage(hEdit, WM_KEYUP, VK_RETURN, 0);
        }

        free(wpath);
        return 1;
    }

    SetForegroundWindow(hwnd);
    Sleep(30);

    // Focus address/location bar (Alt+D then Ctrl+L as fallback)
    SendKey(VK_MENU, TRUE);
    SendKey('D', TRUE);
    SendKey('D', FALSE);
    SendKey(VK_MENU, FALSE);
    Sleep(30);

    SendKey(VK_CONTROL, TRUE);
    SendKey('L', TRUE);
    SendKey('L', FALSE);
    SendKey(VK_CONTROL, FALSE);
    Sleep(30);

    // Select all, replace with target path, then Enter.
    SendKey(VK_CONTROL, TRUE);
    SendKey('A', TRUE);
    SendKey('A', FALSE);
    SendKey(VK_CONTROL, FALSE);
    Sleep(30);

    SendUnicodeString(wpath);
    Sleep(30);

    SendKey(VK_RETURN, TRUE);
    SendKey(VK_RETURN, FALSE);

    free(wpath);
    return 1;
}

typedef struct
{
    DWORD targetPid;
    HWND foundWindow;
} FindWindowData;

BOOL CALLBACK EnumWindowsProc(HWND hwnd, LPARAM lParam)
{
    FindWindowData *data = (FindWindowData *)lParam;

    // Skip invisible windows
    if (!IsWindowVisible(hwnd))
    {
        return TRUE;
    }

    // Skip windows without title
    WCHAR windowTitle[256];
    if (GetWindowTextW(hwnd, windowTitle, 256) == 0)
    {
        return TRUE;
    }

    // Get the process ID of this window
    DWORD pid;
    GetWindowThreadProcessId(hwnd, &pid);

    // Check if this window belongs to our target process
    if (pid == data->targetPid)
    {
        // Check if this is a main window (has WS_OVERLAPPEDWINDOW style)
        LONG style = GetWindowLong(hwnd, GWL_STYLE);
        if ((style & WS_OVERLAPPEDWINDOW) || (style & WS_POPUP))
        {
            data->foundWindow = hwnd;
            return FALSE; // Stop enumeration
        }
    }

    return TRUE; // Continue enumeration
}

int activateWindowByPid(int pid)
{
    if (pid <= 0)
    {
        return 0;
    }

    FindWindowData data;
    data.targetPid = (DWORD)pid;
    data.foundWindow = NULL;

    // Enumerate all top-level windows
    EnumWindows(EnumWindowsProc, (LPARAM)&data);

    if (data.foundWindow == NULL)
    {
        return 0; // Window not found
    }

    HWND hwnd = data.foundWindow;

    // Restore window if minimized
    if (IsIconic(hwnd))
    {
        ShowWindow(hwnd, SW_RESTORE);
    }

    // Show window if hidden
    if (!IsWindowVisible(hwnd))
    {
        ShowWindow(hwnd, SW_SHOW);
    }

    // Bring window to foreground with proper activation
    DWORD curThreadId = GetCurrentThreadId();
    DWORD fgThreadId = GetWindowThreadProcessId(GetForegroundWindow(), NULL);

    if (fgThreadId != 0 && fgThreadId != curThreadId)
    {
        AttachThreadInput(fgThreadId, curThreadId, TRUE);
    }

    SetForegroundWindow(hwnd);
    BringWindowToTop(hwnd);
    SetFocus(hwnd);

    if (fgThreadId != 0 && fgThreadId != curThreadId)
    {
        AttachThreadInput(fgThreadId, curThreadId, FALSE);
    }

    return 1; // Success
}
