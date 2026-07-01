#define NTDDI_VERSION NTDDI_VISTA
#define _WIN32_WINNT _WIN32_WINNT_VISTA
#include <windows.h>
#include <psapi.h>
#include <shellapi.h>
#include <shlobj.h>
#include <commdlg.h>
#include <ole2.h>
#include <UIAutomation.h>
#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <string.h>
#include <wchar.h>

typedef struct
{
    int x;
    int y;
    int width;
    int height;
} WoxWindowRectC;

typedef struct
{
    char id[64];
    WoxWindowRectC bounds;
    WoxWindowRectC workArea;
    int isPrimary;
} WoxDisplayInfoC;

typedef struct
{
    char id[64];
    int pid;
    char title[1024];
    WoxWindowRectC bounds;
    WoxDisplayInfoC display;
    int isMinimized;
} WoxManagedWindowC;

char *getWindowIconByPid(int pid, unsigned char **iconData, int *iconSize, int *width, int *height);
static char *dupEmptyString();

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

    return getWindowIconByPid((int)processId, iconData, iconSize, width, height);
}

char *getWindowIconByPid(int pid, unsigned char **iconData, int *iconSize, int *width, int *height)
{
    if (pid <= 0)
    {
        return "Invalid pid";
    }

    HANDLE hProcess = OpenProcess(PROCESS_QUERY_INFORMATION | PROCESS_VM_READ, FALSE, (DWORD)pid);
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

typedef struct
{
    DWORD pid;
    WCHAR title[1024];
    BOOL found;
} WindowTitleByPidData;

static BOOL CALLBACK EnumWindowTitleByPidProc(HWND hwnd, LPARAM lParam)
{
    WindowTitleByPidData *data = (WindowTitleByPidData *)lParam;
    if (!IsWindowVisible(hwnd))
    {
        return TRUE;
    }

    DWORD windowPid = 0;
    GetWindowThreadProcessId(hwnd, &windowPid);
    if (windowPid != data->pid)
    {
        return TRUE;
    }

    WCHAR windowTitle[1024];
    if (GetWindowTextW(hwnd, windowTitle, 1024) == 0)
    {
        return TRUE;
    }

    wcsncpy(data->title, windowTitle, 1023);
    data->title[1023] = L'\0';
    data->found = TRUE;
    return FALSE;
}

char *getWindowNameByPid(int pid)
{
    if (pid <= 0)
    {
        char *result = (char *)malloc(1);
        result[0] = '\0';
        return result;
    }

    WindowTitleByPidData data;
    ZeroMemory(&data, sizeof(data));
    data.pid = (DWORD)pid;
    EnumWindows(EnumWindowTitleByPidProc, (LPARAM)&data);
    if (!data.found)
    {
        char *result = (char *)malloc(1);
        result[0] = '\0';
        return result;
    }

    int len = WideCharToMultiByte(CP_UTF8, 0, data.title, -1, NULL, 0, NULL, NULL);
    char *windowTitleA = (char *)malloc(len);
    WideCharToMultiByte(CP_UTF8, 0, data.title, -1, windowTitleA, len, NULL, NULL);

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

static WoxWindowRectC rectFromWinRectForManagement(RECT rect)
{
    WoxWindowRectC result;
    result.x = rect.left;
    result.y = rect.top;
    result.width = rect.right - rect.left;
    result.height = rect.bottom - rect.top;
    return result;
}

static HWND rootWindowForManagement(HWND hwnd)
{
    if (!hwnd)
    {
        return NULL;
    }
    HWND root = GetAncestor(hwnd, GA_ROOT);
    return root ? root : hwnd;
}

static void copyWindowIdForManagement(char *dest, size_t destSize, HWND hwnd)
{
    if (!dest || destSize == 0)
    {
        return;
    }
    snprintf(dest, destSize, "%llu", (unsigned long long)(UINT_PTR)rootWindowForManagement(hwnd));
    dest[destSize - 1] = '\0';
}

static void copyWindowTitleForManagement(char *dest, size_t destSize, HWND hwnd)
{
    if (!dest || destSize == 0)
    {
        return;
    }
    dest[0] = '\0';

    WCHAR windowTitle[1024];
    if (!hwnd || GetWindowTextW(hwnd, windowTitle, 1024) == 0)
    {
        return;
    }

    WideCharToMultiByte(CP_UTF8, 0, windowTitle, -1, dest, (int)destSize, NULL, NULL);
    dest[destSize - 1] = '\0';
}

static char *formatWindowIdForManagement(HWND hwnd)
{
    HWND root = rootWindowForManagement(hwnd);
    if (!root)
    {
        return dupEmptyString();
    }

    char buffer[64];
    copyWindowIdForManagement(buffer, sizeof(buffer), root);
    char *result = (char *)malloc(strlen(buffer) + 1);
    if (!result)
    {
        return dupEmptyString();
    }
    strcpy(result, buffer);
    return result;
}

static HWND parseWindowIdForManagement(const char *windowId)
{
    if (!windowId || windowId[0] == '\0')
    {
        return NULL;
    }

    unsigned long long value = strtoull(windowId, NULL, 10);
    if (value == 0)
    {
        return NULL;
    }
    return (HWND)(UINT_PTR)value;
}

static DWORD getWindowPidForManagement(HWND hwnd)
{
    DWORD pid = 0;
    if (hwnd)
    {
        GetWindowThreadProcessId(hwnd, &pid);
    }
    return pid;
}

static int isManageableWindowForManagement(HWND hwnd)
{
    return hwnd && IsWindow(hwnd) && IsWindowVisible(hwnd);
}

static HWND resolveWindowForManagement(const char *windowId, int pid)
{
    HWND hwnd = parseWindowIdForManagement(windowId);
    if (hwnd && isManageableWindowForManagement(hwnd))
    {
        if (pid <= 0 || getWindowPidForManagement(hwnd) == (DWORD)pid)
        {
            return rootWindowForManagement(hwnd);
        }
    }

    if (pid <= 0)
    {
        return NULL;
    }

    for (HWND candidate = GetWindow(GetDesktopWindow(), GW_CHILD); candidate != NULL; candidate = GetWindow(candidate, GW_HWNDNEXT))
    {
        if (!isManageableWindowForManagement(candidate) || IsIconic(candidate))
        {
            continue;
        }
        if (getWindowPidForManagement(candidate) == (DWORD)pid)
        {
            return rootWindowForManagement(candidate);
        }
    }
    return NULL;
}

static int fillDisplayInfoForManagement(HMONITOR monitor, WoxDisplayInfoC *outDisplay)
{
    if (!monitor || !outDisplay)
    {
        return 0;
    }

    MONITORINFO info;
    ZeroMemory(&info, sizeof(info));
    info.cbSize = sizeof(info);
    if (!GetMonitorInfo(monitor, &info))
    {
        return 0;
    }

    snprintf(outDisplay->id, sizeof(outDisplay->id), "%llu", (unsigned long long)(UINT_PTR)monitor);
    outDisplay->id[sizeof(outDisplay->id) - 1] = '\0';
    outDisplay->bounds = rectFromWinRectForManagement(info.rcMonitor);
    outDisplay->workArea = rectFromWinRectForManagement(info.rcWork);
    outDisplay->isPrimary = (info.dwFlags & MONITORINFOF_PRIMARY) ? 1 : 0;
    return 1;
}

static int fillManagedWindowForManagement(HWND hwnd, int pid, WoxManagedWindowC *outWindow)
{
    if (!hwnd || !outWindow)
    {
        return -1;
    }

    RECT rect;
    if (!GetWindowRect(hwnd, &rect))
    {
        return -1;
    }

    HMONITOR monitor = MonitorFromWindow(hwnd, MONITOR_DEFAULTTONEAREST);
    WoxDisplayInfoC display;
    ZeroMemory(&display, sizeof(display));
    if (!fillDisplayInfoForManagement(monitor, &display))
    {
        return -3;
    }

    ZeroMemory(outWindow, sizeof(*outWindow));
    copyWindowIdForManagement(outWindow->id, sizeof(outWindow->id), hwnd);
    copyWindowTitleForManagement(outWindow->title, sizeof(outWindow->title), hwnd);
    outWindow->pid = pid > 0 ? pid : (int)getWindowPidForManagement(hwnd);
    outWindow->bounds = rectFromWinRectForManagement(rect);
    outWindow->display = display;
    outWindow->isMinimized = IsIconic(hwnd) ? 1 : 0;
    return 1;
}

char *getActiveWindowIdForManagement()
{
    return formatWindowIdForManagement(GetForegroundWindow());
}

int getManagedWindowForManagement(const char *windowId, int pid, WoxManagedWindowC *outWindow)
{
    if (!outWindow)
    {
        return -1;
    }

    HWND hwnd = resolveWindowForManagement(windowId, pid);
    if (!hwnd)
    {
        return 0;
    }

    return fillManagedWindowForManagement(hwnd, pid, outWindow);
}

typedef struct
{
    WoxManagedWindowC *windows;
    int count;
    int capacity;
    int failed;
} ManagedWindowEnumForManagementData;

static int appendManagedWindowForManagement(ManagedWindowEnumForManagementData *data, HWND hwnd)
{
    if (!data || !hwnd)
    {
        return 1;
    }

    if (data->count >= data->capacity)
    {
        int newCapacity = data->capacity == 0 ? 16 : data->capacity * 2;
        WoxManagedWindowC *newWindows = (WoxManagedWindowC *)realloc(data->windows, sizeof(WoxManagedWindowC) * (size_t)newCapacity);
        if (!newWindows)
        {
            data->failed = 1;
            return 0;
        }
        data->windows = newWindows;
        data->capacity = newCapacity;
    }

    int result = fillManagedWindowForManagement(hwnd, 0, &data->windows[data->count]);
    if (result == 1 && data->windows[data->count].title[0] != '\0')
    {
        data->count++;
    }
    return 1;
}

int listManagedWindowsForManagement(WoxManagedWindowC **outWindows, int *outCount)
{
    if (!outWindows || !outCount)
    {
        return -1;
    }

    *outWindows = NULL;
    *outCount = 0;

    ManagedWindowEnumForManagementData data;
    ZeroMemory(&data, sizeof(data));

    for (HWND candidate = GetWindow(GetDesktopWindow(), GW_CHILD); candidate != NULL; candidate = GetWindow(candidate, GW_HWNDNEXT))
    {
        if (!isManageableWindowForManagement(candidate))
        {
            continue;
        }

        HWND hwnd = rootWindowForManagement(candidate);
        if (!hwnd || hwnd != candidate)
        {
            continue;
        }

        if (!appendManagedWindowForManagement(&data, hwnd))
        {
            break;
        }
    }

    if (data.failed)
    {
        if (data.windows)
        {
            free(data.windows);
        }
        return -1;
    }

    *outWindows = data.windows;
    *outCount = data.count;
    return 1;
}

void freeManagedWindowsForManagement(WoxManagedWindowC *windows)
{
    if (windows)
    {
        free(windows);
    }
}

typedef struct
{
    WoxDisplayInfoC *displays;
    int count;
    int capacity;
    int failed;
} DisplayEnumForManagementData;

static BOOL CALLBACK enumDisplaysForManagementProc(HMONITOR monitor, HDC hdc, LPRECT rect, LPARAM lParam)
{
    DisplayEnumForManagementData *data = (DisplayEnumForManagementData *)lParam;
    if (!data || data->failed)
    {
        return FALSE;
    }

    if (data->count >= data->capacity)
    {
        int newCapacity = data->capacity == 0 ? 4 : data->capacity * 2;
        WoxDisplayInfoC *newDisplays = (WoxDisplayInfoC *)realloc(data->displays, sizeof(WoxDisplayInfoC) * (size_t)newCapacity);
        if (!newDisplays)
        {
            data->failed = 1;
            return FALSE;
        }
        data->displays = newDisplays;
        data->capacity = newCapacity;
    }

    WoxDisplayInfoC display;
    ZeroMemory(&display, sizeof(display));
    if (fillDisplayInfoForManagement(monitor, &display))
    {
        data->displays[data->count] = display;
        data->count++;
    }
    return TRUE;
}

int listDisplaysForManagement(WoxDisplayInfoC **outDisplays, int *outCount)
{
    if (!outDisplays || !outCount)
    {
        return -1;
    }

    *outDisplays = NULL;
    *outCount = 0;

    DisplayEnumForManagementData data;
    ZeroMemory(&data, sizeof(data));
    if (!EnumDisplayMonitors(NULL, NULL, enumDisplaysForManagementProc, (LPARAM)&data))
    {
        if (data.displays)
        {
            free(data.displays);
        }
        return -1;
    }

    if (data.failed)
    {
        if (data.displays)
        {
            free(data.displays);
        }
        return -1;
    }

    if (data.count == 0)
    {
        if (data.displays)
        {
            free(data.displays);
        }
        return -3;
    }

    *outDisplays = data.displays;
    *outCount = data.count;
    return 1;
}

void freeDisplaysForManagement(WoxDisplayInfoC *displays)
{
    if (displays)
    {
        free(displays);
    }
}

int moveResizeWindowForManagement(const char *windowId, int pid, int x, int y, int width, int height)
{
    HWND hwnd = resolveWindowForManagement(windowId, pid);
    if (!hwnd)
    {
        return 0;
    }

    if (IsZoomed(hwnd) || IsIconic(hwnd))
    {
        ShowWindow(hwnd, SW_RESTORE);
    }

    if (width < 1)
    {
        width = 1;
    }
    if (height < 1)
    {
        height = 1;
    }

    if (SetWindowPos(hwnd, HWND_TOP, x, y, width, height, SWP_SHOWWINDOW))
    {
        return 1;
    }

    DWORD err = GetLastError();
    if (err > 0 && err < 100000)
    {
        return -1000 - (int)err;
    }
    return -1;
}

int maximizeWindowForManagement(const char *windowId, int pid)
{
    HWND hwnd = resolveWindowForManagement(windowId, pid);
    if (!hwnd)
    {
        return 0;
    }

    ShowWindow(hwnd, SW_MAXIMIZE);
    return 1;
}

int minimizeWindowForManagement(const char *windowId, int pid)
{
    HWND hwnd = resolveWindowForManagement(windowId, pid);
    if (!hwnd)
    {
        return 0;
    }

    ShowWindow(hwnd, SW_MINIMIZE);
    return 1;
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

typedef struct
{
    DWORD pid;
    int found;
} OpenSaveDialogByPidData;

static BOOL CALLBACK EnumOpenSaveDialogByPidProc(HWND hwnd, LPARAM lParam)
{
    OpenSaveDialogByPidData *data = (OpenSaveDialogByPidData *)lParam;
    DWORD windowPid = 0;
    GetWindowThreadProcessId(hwnd, &windowPid);
    if (windowPid != data->pid)
    {
        return TRUE;
    }

    if (isOpenSaveDialogWindow(hwnd))
    {
        data->found = 1;
        return FALSE;
    }

    return TRUE;
}

int isOpenSaveDialog()
{
    HWND hwnd = GetForegroundWindow();
    return isOpenSaveDialogWindow(hwnd);
}

int isOpenSaveDialogByPid(int pid)
{
    if (pid <= 0)
    {
        return 0;
    }

    OpenSaveDialogByPidData data;
    data.pid = (DWORD)pid;
    data.found = 0;
    EnumWindows(EnumOpenSaveDialogByPidProc, (LPARAM)&data);
    return data.found;
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

static void trimWideInPlace(WCHAR *text)
{
    if (!text)
    {
        return;
    }

    WCHAR *start = text;
    while (*start == L' ' || *start == L'\t' || *start == L'\r' || *start == L'\n' || *start == L'"')
    {
        start++;
    }
    if (start != text)
    {
        memmove(text, start, (wcslen(start) + 1) * sizeof(WCHAR));
    }

    size_t len = wcslen(text);
    while (len > 0)
    {
        WCHAR ch = text[len - 1];
        if (ch != L' ' && ch != L'\t' && ch != L'\r' && ch != L'\n' && ch != L'"')
        {
            break;
        }
        text[len - 1] = L'\0';
        len--;
    }
}

static int isExistingDirectoryPathWide(const WCHAR *path)
{
    if (!path || path[0] == L'\0')
    {
        return 0;
    }

    DWORD attrs = GetFileAttributesW(path);
    return attrs != INVALID_FILE_ATTRIBUTES && (attrs & FILE_ATTRIBUTE_DIRECTORY);
}

// copyKnownFolderPath resolves a shell known folder to its filesystem path.
static int copyKnownFolderPath(const KNOWNFOLDERID *folderId, WCHAR *out, size_t outLen)
{
    if (!folderId || !out || outLen == 0)
    {
        return 0;
    }

    PWSTR path = NULL;
    HRESULT hr = SHGetKnownFolderPath(folderId, KF_FLAG_DEFAULT, NULL, &path);
    if (FAILED(hr) || !path || path[0] == L'\0')
    {
        if (path)
        {
            CoTaskMemFree(path);
        }
        return 0;
    }

    wcsncpy(out, path, outLen - 1);
    out[outLen - 1] = L'\0';
    CoTaskMemFree(path);
    return 1;
}

// copyKnownFolderDisplayName resolves the localized shell display name for a known folder path.
static int copyKnownFolderDisplayName(const WCHAR *path, WCHAR *out, size_t outLen)
{
    if (!path || !out || outLen == 0)
    {
        return 0;
    }

    SHFILEINFOW info;
    ZeroMemory(&info, sizeof(info));
    if (!SHGetFileInfoW(path, 0, &info, sizeof(info), SHGFI_DISPLAYNAME))
    {
        return 0;
    }

    wcsncpy(out, info.szDisplayName, outLen - 1);
    out[outLen - 1] = L'\0';
    trimWideInPlace(out);
    return out[0] != L'\0';
}

static const WCHAR *lastPathSegmentWide(const WCHAR *path)
{
    if (!path)
    {
        return L"";
    }

    const WCHAR *lastBackslash = wcsrchr(path, L'\\');
    const WCHAR *lastSlash = wcsrchr(path, L'/');
    const WCHAR *lastSeparator = lastBackslash > lastSlash ? lastBackslash : lastSlash;
    return lastSeparator ? lastSeparator + 1 : path;
}

static void extractLastBreadcrumbSegment(const WCHAR *candidate, WCHAR *out, size_t outLen)
{
    if (!candidate || !out || outLen == 0)
    {
        return;
    }

    wcsncpy(out, candidate, outLen - 1);
    out[outLen - 1] = L'\0';

    WCHAR *last = out;
    for (WCHAR *p = out; *p; ++p)
    {
        if (*p == L'>' || *p == 0x203A || *p == 0x00BB)
        {
            last = p + 1;
        }
    }

    if (last != out)
    {
        memmove(out, last, (wcslen(last) + 1) * sizeof(WCHAR));
    }
    trimWideInPlace(out);
}

// copyKnownFolderPathCandidate maps localized shell folder labels like Desktop back to filesystem paths.
static int copyKnownFolderPathCandidate(const WCHAR *candidate, WCHAR *out, size_t outLen)
{
    if (!candidate || !out || outLen == 0)
    {
        return 0;
    }

    WCHAR normalized[32768];
    size_t normalizedCap = sizeof(normalized) / sizeof(normalized[0]);
    wcsncpy(normalized, candidate, normalizedCap - 1);
    normalized[normalizedCap - 1] = L'\0';
    trimWideInPlace(normalized);
    if (normalized[0] == L'\0')
    {
        return 0;
    }

    WCHAR label[32768];
    extractLastBreadcrumbSegment(normalized, label, sizeof(label) / sizeof(label[0]));

    const KNOWNFOLDERID *folderIds[] = {
        &FOLDERID_Desktop,
        &FOLDERID_Documents,
        &FOLDERID_Downloads,
        &FOLDERID_Pictures,
        &FOLDERID_Music,
        &FOLDERID_Videos,
        &FOLDERID_Profile,
    };

    for (size_t i = 0; i < sizeof(folderIds) / sizeof(folderIds[0]); ++i)
    {
        WCHAR knownPath[32768];
        ZeroMemory(knownPath, sizeof(knownPath));
        if (!copyKnownFolderPath(folderIds[i], knownPath, sizeof(knownPath) / sizeof(knownPath[0])))
        {
            continue;
        }

        WCHAR displayName[512];
        ZeroMemory(displayName, sizeof(displayName));
        copyKnownFolderDisplayName(knownPath, displayName, sizeof(displayName) / sizeof(displayName[0]));

        if (_wcsicmp(normalized, knownPath) == 0 || _wcsicmp(label, knownPath) == 0 || _wcsicmp(label, lastPathSegmentWide(knownPath)) == 0 || (displayName[0] != L'\0' && (_wcsicmp(normalized, displayName) == 0 || _wcsicmp(label, displayName) == 0)))
        {
            wcsncpy(out, knownPath, outLen - 1);
            out[outLen - 1] = L'\0';
            return 1;
        }
    }

    return 0;
}

static int copyExistingDirectoryPathCandidate(const WCHAR *candidate, WCHAR *out, size_t outLen)
{
    if (!candidate || !out || outLen == 0)
    {
        return 0;
    }

    WCHAR tmp[32768];
    size_t tmpCap = sizeof(tmp) / sizeof(tmp[0]);
    wcsncpy(tmp, candidate, tmpCap - 1);
    tmp[tmpCap - 1] = L'\0';
    trimWideInPlace(tmp);
    if (copyKnownFolderPathCandidate(tmp, out, outLen))
    {
        return 1;
    }
    if (isExistingDirectoryPathWide(tmp))
    {
        wcsncpy(out, tmp, outLen - 1);
        out[outLen - 1] = L'\0';
        return 1;
    }

    size_t len = wcslen(tmp);
    for (size_t i = 0; i + 2 < len; i++)
    {
        if (((tmp[i] >= L'A' && tmp[i] <= L'Z') || (tmp[i] >= L'a' && tmp[i] <= L'z')) && tmp[i + 1] == L':' && (tmp[i + 2] == L'\\' || tmp[i + 2] == L'/'))
        {
            WCHAR pathPart[32768];
            wcsncpy(pathPart, tmp + i, tmpCap - 1);
            pathPart[tmpCap - 1] = L'\0';
            trimWideInPlace(pathPart);
            if (isExistingDirectoryPathWide(pathPart))
            {
                wcsncpy(out, pathPart, outLen - 1);
                out[outLen - 1] = L'\0';
                return 1;
            }
        }
    }

    if (tmp[0] == L'\\' && tmp[1] == L'\\' && isExistingDirectoryPathWide(tmp))
    {
        wcsncpy(out, tmp, outLen - 1);
        out[outLen - 1] = L'\0';
        return 1;
    }

    return 0;
}

typedef struct
{
    IUIAutomation *automation;
    IUIAutomationTreeWalker *walker;
    WCHAR path[32768];
    int found;
    int visited;
} UiaDialogPathSearch;

static int tryCopyUiaElementPath(IUIAutomationElement *element, UiaDialogPathSearch *data)
{
    if (!element || !data || data->found)
    {
        return 0;
    }

    BSTR name = NULL;
    if (SUCCEEDED(element->lpVtbl->get_CurrentName(element, &name)) && name)
    {
        if (copyExistingDirectoryPathCandidate(name, data->path, sizeof(data->path) / sizeof(data->path[0])))
        {
            data->found = 1;
            SysFreeString(name);
            return 1;
        }
        SysFreeString(name);
    }

    VARIANT value;
    VariantInit(&value);
    if (SUCCEEDED(element->lpVtbl->GetCurrentPropertyValue(element, UIA_ValueValuePropertyId, &value)))
    {
        if (value.vt == VT_BSTR && value.bstrVal)
        {
            if (copyExistingDirectoryPathCandidate(value.bstrVal, data->path, sizeof(data->path) / sizeof(data->path[0])))
            {
                data->found = 1;
                VariantClear(&value);
                return 1;
            }
        }
        VariantClear(&value);
    }

    return 0;
}

static int shouldSkipUiaDialogSubtree(IUIAutomationElement *element, int depth)
{
    if (!element)
    {
        return 0;
    }

    CONTROLTYPEID controlType = 0;
    if (FAILED(element->lpVtbl->get_CurrentControlType(element, &controlType)))
    {
        return 0;
    }

    if (controlType == UIA_ListControlTypeId || controlType == UIA_DataGridControlTypeId || controlType == UIA_TableControlTypeId || controlType == UIA_TreeControlTypeId)
    {
        return 1;
    }

    if (depth > 2 && (controlType == UIA_ListItemControlTypeId || controlType == UIA_DataItemControlTypeId || controlType == UIA_TreeItemControlTypeId))
    {
        return 1;
    }

    return 0;
}

static void findUiaDialogDirectoryPath(IUIAutomationElement *element, UiaDialogPathSearch *data, int depth)
{
    if (!element || !data || data->found || depth > 14 || data->visited > 400)
    {
        return;
    }

    data->visited++;
    if (tryCopyUiaElementPath(element, data))
    {
        return;
    }

    if (shouldSkipUiaDialogSubtree(element, depth))
    {
        return;
    }

    IUIAutomationElement *child = NULL;
    if (!data->walker || FAILED(data->walker->lpVtbl->GetFirstChildElement(data->walker, element, &child)) || !child)
    {
        return;
    }

    while (child && !data->found && data->visited <= 400)
    {
        findUiaDialogDirectoryPath(child, data, depth + 1);

        IUIAutomationElement *next = NULL;
        if (FAILED(data->walker->lpVtbl->GetNextSiblingElement(data->walker, child, &next)))
        {
            child->lpVtbl->Release(child);
            break;
        }

        child->lpVtbl->Release(child);
        child = next;
    }
}

static int copyUiaDialogDirectoryPath(HWND hwnd, WCHAR *out, size_t outLen)
{
    if (!hwnd || !out || outLen == 0)
    {
        return 0;
    }

    HRESULT initHr = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
    int shouldUninitialize = SUCCEEDED(initHr);
    if (FAILED(initHr) && initHr != RPC_E_CHANGED_MODE)
    {
        return 0;
    }

    IUIAutomation *automation = NULL;
    HRESULT hr = CoCreateInstance(&CLSID_CUIAutomation, NULL, CLSCTX_INPROC_SERVER, &IID_IUIAutomation, (void **)&automation);
    if (FAILED(hr) || !automation)
    {
        if (shouldUninitialize)
        {
            CoUninitialize();
        }
        return 0;
    }

    IUIAutomationElement *root = NULL;
    hr = automation->lpVtbl->ElementFromHandle(automation, hwnd, &root);
    if (FAILED(hr) || !root)
    {
        automation->lpVtbl->Release(automation);
        if (shouldUninitialize)
        {
            CoUninitialize();
        }
        return 0;
    }

    IUIAutomationTreeWalker *walker = NULL;
    hr = automation->lpVtbl->get_ControlViewWalker(automation, &walker);
    if (FAILED(hr) || !walker)
    {
        root->lpVtbl->Release(root);
        automation->lpVtbl->Release(automation);
        if (shouldUninitialize)
        {
            CoUninitialize();
        }
        return 0;
    }

    UiaDialogPathSearch data;
    ZeroMemory(&data, sizeof(data));
    data.automation = automation;
    data.walker = walker;
    findUiaDialogDirectoryPath(root, &data, 0);

    if (data.found)
    {
        wcsncpy(out, data.path, outLen - 1);
        out[outLen - 1] = L'\0';
    }

    walker->lpVtbl->Release(walker);
    root->lpVtbl->Release(root);
    automation->lpVtbl->Release(automation);
    if (shouldUninitialize)
    {
        CoUninitialize();
    }

	return data.found;
}

typedef struct
{
	IUIAutomationTreeWalker *walker;
	const WCHAR *targetName;
	int found;
	int visited;
} UiaDialogItemSelectSearch;

static const WCHAR *fileNameFromPathWide(const WCHAR *path)
{
	if (!path)
	{
		return L"";
	}

	const WCHAR *lastSlash = wcsrchr(path, L'\\');
	const WCHAR *lastForwardSlash = wcsrchr(path, L'/');
	const WCHAR *lastSeparator = lastSlash > lastForwardSlash ? lastSlash : lastForwardSlash;
	return lastSeparator ? lastSeparator + 1 : path;
}

static int trySelectUiaDialogItem(IUIAutomationElement *element, UiaDialogItemSelectSearch *data)
{
	if (!element || !data || data->found || !data->targetName || data->targetName[0] == L'\0')
	{
		return 0;
	}

	BSTR name = NULL;
	if (FAILED(element->lpVtbl->get_CurrentName(element, &name)) || !name)
	{
		return 0;
	}

	int isTarget = _wcsicmp(name, data->targetName) == 0;
	SysFreeString(name);
	if (!isTarget)
	{
		return 0;
	}

	IUIAutomationScrollItemPattern *scrollItem = NULL;
	if (SUCCEEDED(element->lpVtbl->GetCurrentPattern(element, UIA_ScrollItemPatternId, (IUnknown **)&scrollItem)) && scrollItem)
	{
		scrollItem->lpVtbl->ScrollIntoView(scrollItem);
		scrollItem->lpVtbl->Release(scrollItem);
	}

	IUIAutomationSelectionItemPattern *selectionItem = NULL;
	if (SUCCEEDED(element->lpVtbl->GetCurrentPattern(element, UIA_SelectionItemPatternId, (IUnknown **)&selectionItem)) && selectionItem)
	{
		HRESULT selectHr = selectionItem->lpVtbl->Select(selectionItem);
		selectionItem->lpVtbl->Release(selectionItem);
		if (SUCCEEDED(selectHr))
		{
			data->found = 1;
			return 1;
		}
	}

	return 0;
}

static void findAndSelectUiaDialogItem(IUIAutomationElement *element, UiaDialogItemSelectSearch *data, int depth)
{
	if (!element || !data || data->found || depth > 14 || data->visited > 1500)
	{
		return;
	}

	data->visited++;
	if (trySelectUiaDialogItem(element, data))
	{
		return;
	}

	IUIAutomationElement *child = NULL;
	if (!data->walker || FAILED(data->walker->lpVtbl->GetFirstChildElement(data->walker, element, &child)) || !child)
	{
		return;
	}

	while (child && !data->found && data->visited <= 1500)
	{
		findAndSelectUiaDialogItem(child, data, depth + 1);

		IUIAutomationElement *next = NULL;
		if (FAILED(data->walker->lpVtbl->GetNextSiblingElement(data->walker, child, &next)))
		{
			child->lpVtbl->Release(child);
			break;
		}

		child->lpVtbl->Release(child);
		child = next;
	}
}

static int selectUiaDialogItemByName(HWND hwnd, const WCHAR *targetName)
{
	if (!hwnd || !targetName || targetName[0] == L'\0')
	{
		return 0;
	}

	HRESULT initHr = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
	int shouldUninitialize = SUCCEEDED(initHr);
	if (FAILED(initHr) && initHr != RPC_E_CHANGED_MODE)
	{
		return 0;
	}

	IUIAutomation *automation = NULL;
	HRESULT hr = CoCreateInstance(&CLSID_CUIAutomation, NULL, CLSCTX_INPROC_SERVER, &IID_IUIAutomation, (void **)&automation);
	if (FAILED(hr) || !automation)
	{
		if (shouldUninitialize)
		{
			CoUninitialize();
		}
		return 0;
	}

	IUIAutomationElement *root = NULL;
	hr = automation->lpVtbl->ElementFromHandle(automation, hwnd, &root);
	if (FAILED(hr) || !root)
	{
		automation->lpVtbl->Release(automation);
		if (shouldUninitialize)
		{
			CoUninitialize();
		}
		return 0;
	}

	IUIAutomationTreeWalker *walker = NULL;
	hr = automation->lpVtbl->get_ControlViewWalker(automation, &walker);
	if (FAILED(hr) || !walker)
	{
		root->lpVtbl->Release(root);
		automation->lpVtbl->Release(automation);
		if (shouldUninitialize)
		{
			CoUninitialize();
		}
		return 0;
	}

	UiaDialogItemSelectSearch data;
	ZeroMemory(&data, sizeof(data));
	data.walker = walker;
	data.targetName = targetName;
	findAndSelectUiaDialogItem(root, &data, 0);

	walker->lpVtbl->Release(walker);
	root->lpVtbl->Release(root);
	automation->lpVtbl->Release(automation);
	if (shouldUninitialize)
	{
		CoUninitialize();
	}

	return data.found;
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

static int copyDialogDirectoryPathFromCdm(HWND hwnd, WCHAR *out, size_t outLen)
{
    if (!hwnd || !out || outLen == 0)
    {
        return 0;
    }

    WCHAR folderPath[32768];
    ZeroMemory(folderPath, sizeof(folderPath));
    LRESULT folderLen = SendMessageW(hwnd, CDM_GETFOLDERPATH, (WPARAM)(sizeof(folderPath) / sizeof(folderPath[0])), (LPARAM)folderPath);
    if (folderLen > 0 && copyExistingDirectoryPathCandidate(folderPath, out, outLen))
    {
        return 1;
    }

    WCHAR selectedPath[32768];
    ZeroMemory(selectedPath, sizeof(selectedPath));
    LRESULT selectedLen = SendMessageW(hwnd, CDM_GETFILEPATH, (WPARAM)(sizeof(selectedPath) / sizeof(selectedPath[0])), (LPARAM)selectedPath);
    if (selectedLen > 0 && selectedPath[0] != L'\0')
    {
        WCHAR parentPath[32768];
        ZeroMemory(parentPath, sizeof(parentPath));
        if (copyParentDirectoryPath(selectedPath, parentPath, sizeof(parentPath) / sizeof(parentPath[0])) && copyExistingDirectoryPathCandidate(parentPath, out, outLen))
        {
            return 1;
        }
    }

    return 0;
}

typedef struct
{
    WCHAR path[32768];
    char source[128];
    int found;
} DialogPathChildSearchData;

static BOOL CALLBACK EnumDialogPathChildProc(HWND child, LPARAM lParam)
{
    DialogPathChildSearchData *data = (DialogPathChildSearchData *)lParam;
    if (!data || data->found)
    {
        return FALSE;
    }

    WCHAR className[256];
    className[0] = L'\0';
    GetClassNameW(child, className, sizeof(className) / sizeof(className[0]));

    // Some hosted dialogs wrap the real common dialog in a child #32770; CDM
    // messages must be sent to that child instead of the top-level wrapper.
    if (wcscmp(className, L"#32770") == 0 && copyDialogDirectoryPathFromCdm(child, data->path, sizeof(data->path) / sizeof(data->path[0])))
    {
        data->found = 1;
        snprintf(data->source, sizeof(data->source), "child_cdm hwnd=0x%p", child);
        return FALSE;
    }

    WCHAR text[32768];
    ZeroMemory(text, sizeof(text));
    if (GetWindowTextW(child, text, sizeof(text) / sizeof(text[0])) > 0 && copyExistingDirectoryPathCandidate(text, data->path, sizeof(data->path) / sizeof(data->path[0])))
    {
        data->found = 1;
        snprintf(data->source, sizeof(data->source), "child_text hwnd=0x%p class=%ls", child, className);
        return FALSE;
    }

    return TRUE;
}

static int copyDialogDirectoryPathFromChildren(HWND hwnd, WCHAR *out, size_t outLen, char *source, size_t sourceLen)
{
    if (!hwnd || !out || outLen == 0)
    {
        return 0;
    }

    DialogPathChildSearchData data;
    ZeroMemory(&data, sizeof(data));
    EnumChildWindows(hwnd, EnumDialogPathChildProc, (LPARAM)&data);
    if (!data.found)
    {
        return 0;
    }

    wcsncpy(out, data.path, outLen - 1);
    out[outLen - 1] = L'\0';
    if (source && sourceLen > 0)
    {
        snprintf(source, sourceLen, "%s", data.source);
        source[sourceLen - 1] = '\0';
    }
    return 1;
}

static char *getDialogDirectoryPathByWindow(HWND hwnd)
{
    if (!isOpenSaveDialogWindow(hwnd))
    {
        return dupEmptyString();
    }

    WCHAR cdmPath[32768];
    ZeroMemory(cdmPath, sizeof(cdmPath));
    if (copyDialogDirectoryPathFromCdm(hwnd, cdmPath, sizeof(cdmPath) / sizeof(cdmPath[0])))
    {
        return copyUtf8FromWide(cdmPath);
    }

    WCHAR childPath[32768];
    ZeroMemory(childPath, sizeof(childPath));
    if (copyDialogDirectoryPathFromChildren(hwnd, childPath, sizeof(childPath) / sizeof(childPath[0]), NULL, 0))
    {
        return copyUtf8FromWide(childPath);
    }

    // Modern Common Item Dialogs do not always respond to CDM_GETFOLDERPATH.
    // UI Automation can still expose the address edit value without changing focus.
    WCHAR uiaPath[32768];
    ZeroMemory(uiaPath, sizeof(uiaPath));
    if (copyUiaDialogDirectoryPath(hwnd, uiaPath, sizeof(uiaPath) / sizeof(uiaPath[0])))
    {
        return copyUtf8FromWide(uiaPath);
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

char *getFileDialogPathByWindowId(const char *windowId, int pid)
{
	HWND hwnd = parseWindowIdForManagement(windowId);
	if (!hwnd || !IsWindow(hwnd))
	{
		return dupEmptyString();
	}

	DWORD hwndPid = 0;
	GetWindowThreadProcessId(hwnd, &hwndPid);
	if (pid > 0 && hwndPid != (DWORD)pid)
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

int setActiveFileDialogFileName(const char *path)
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
	if (!hEdit)
	{
		free(wpath);
		return 0;
	}

	SetForegroundWindow(hwnd);
	SetFocus(hEdit);
	SendMessageW(hEdit, WM_SETTEXT, 0, (LPARAM)wpath);
	SendMessageW(hEdit, EM_SETSEL, (WPARAM)0, (LPARAM)-1);
	free(wpath);
	return 1;
}

int highlightInActiveFileDialog(const char *path)
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

	int selected = selectUiaDialogItemByName(hwnd, fileNameFromPathWide(wpath));
	free(wpath);
	return selected;
}

int selectInActiveFileDialog(const char *path)
{
	return setActiveFileDialogFileName(path);
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

typedef struct
{
    HWND directUI;
    HWND sysListView;
} FindExplorerContentData;

static BOOL CALLBACK findExplorerContentProc(HWND hwnd, LPARAM lParam)
{
    FindExplorerContentData *data = (FindExplorerContentData *)lParam;
    WCHAR className[256];
    if (GetClassNameW(hwnd, className, 256) == 0)
    {
        return TRUE;
    }

    if (wcscmp(className, L"DirectUIHWND") == 0)
    {
        data->directUI = hwnd;
        return FALSE;
    }

    if (!data->sysListView && wcscmp(className, L"SysListView32") == 0)
    {
        data->sysListView = hwnd;
    }

    return TRUE;
}

static HWND findExplorerContentHwnd(HWND hwnd)
{
    FindExplorerContentData data;
    data.directUI = NULL;
    data.sysListView = NULL;
    EnumChildWindows(hwnd, findExplorerContentProc, (LPARAM)&data);
    return data.directUI ? data.directUI : data.sysListView;
}

// focusFileExplorerContentByHwnd restores keyboard focus to the file list after
// COM navigation, which can leave Explorer active but focused on non-list chrome.
int focusFileExplorerContentByHwnd(uintptr_t hwndValue)
{
    HWND hwnd = (HWND)hwndValue;
    if (!hwnd || !IsWindow(hwnd))
    {
        return 0;
    }

    if (IsIconic(hwnd))
    {
        ShowWindow(hwnd, SW_RESTORE);
    }
    if (!IsWindowVisible(hwnd))
    {
        ShowWindow(hwnd, SW_SHOW);
    }

    DWORD currentThreadId = GetCurrentThreadId();
    DWORD targetThreadId = GetWindowThreadProcessId(hwnd, NULL);
    int focused = 0;
    for (int attempt = 0; attempt < 6; attempt++)
    {
        HWND content = findExplorerContentHwnd(hwnd);
        if (content)
        {
            BOOL attached = FALSE;
            if (targetThreadId != 0 && targetThreadId != currentThreadId)
            {
                attached = AttachThreadInput(targetThreadId, currentThreadId, TRUE);
            }

            SetForegroundWindow(hwnd);
            BringWindowToTop(hwnd);
            SetActiveWindow(hwnd);
            SetFocus(content);

            if (attached)
            {
                AttachThreadInput(targetThreadId, currentThreadId, FALSE);
            }
            focused = 1;
        }

        // Explorer navigation is asynchronous; re-assert focus briefly so the
        // completed folder load does not leave focus on address/search chrome.
        if (attempt < 5)
        {
            Sleep(50);
        }
    }

    return focused;
}
