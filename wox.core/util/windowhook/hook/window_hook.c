#define COBJMACROS
#define _WIN32_WINNT 0x0600

#include <windows.h>
#include <commctrl.h>
#include <shlobj.h>
#include <shobjidl.h>
#include <stdio.h>
#include <stdlib.h>
#include <wchar.h>

#define WOX_WINDOW_HOOK_VERSION 1
#define WOX_WINDOW_PATH_CAPACITY 32768
// Only reached when the target thread never runs the posted command: a healthy dialog
// answers in single-digit milliseconds. Every caller has a slower fallback route, so this
// bound is what the user waits through before that route even starts.
#define WOX_WINDOW_HOOK_TIMEOUT_MS 80
#define WOX_WM_GETISHELLBROWSER (WM_USER + 7)
// Wox publishes the overlay's physical pixel offset from the target's outer frame
// through these window properties. The subclass then translates the overlay inside
// the target's own WM_WINDOWPOSCHANGED instead of waiting for a cross-process round
// trip, which is what makes a dragged overlay track without lag.
#define WOX_STICKY_OFFSET_ACTIVE_PROP L"Wox.WindowHook.StickyOffset.Active.v1"
#define WOX_STICKY_OFFSET_X_PROP L"Wox.WindowHook.StickyOffset.X.v1"
#define WOX_STICKY_OFFSET_Y_PROP L"Wox.WindowHook.StickyOffset.Y.v1"
#ifndef WM_DPICHANGED
#define WM_DPICHANGED 0x02E0
#endif

enum WoxWindowHookCommandType
{
    woxWindowHookCommandNavigateDialog = 1,
    woxWindowHookCommandAttachSticky = 2,
    woxWindowHookCommandDetachSticky = 3,
    woxWindowHookCommandSelectDialogItem = 4
};

enum WoxWindowHookStage
{
    woxWindowHookStageNone = 0,
    woxWindowHookStageValidateInput = 1,
    woxWindowHookStageValidateWindow = 2,
    woxWindowHookStageResolveThread = 3,
    woxWindowHookStageCreateIpc = 4,
    woxWindowHookStageMapIpc = 5,
    woxWindowHookStageInstallHook = 6,
    woxWindowHookStagePostMessage = 7,
    woxWindowHookStageWait = 8,
    woxWindowHookStageCallback = 9,
    woxWindowHookStageCallbackValidate = 10,
    woxWindowHookStageCoInitialize = 11,
    woxWindowHookStageGetShellBrowser = 12,
    woxWindowHookStageParsePath = 13,
    woxWindowHookStageBrowse = 14,
    woxWindowHookStageCompleted = 15,
    woxWindowHookStageQueryActiveView = 16,
    woxWindowHookStageBindParent = 17,
    woxWindowHookStageGetViewFolder = 18,
    woxWindowHookStageCompareParent = 19,
    woxWindowHookStageSelectItem = 20
};

typedef struct
{
    DWORD stage;
    DWORD win32Error;
    LONG hresult;
    DWORD targetPid;
    DWORD targetThread;
    DWORD shellViewFound;
    DWORD hookInstalled;
    DWORD callbackEntered;
    DWORD waitResult;
} WoxWindowHookDiagnostic;

typedef struct
{
    DWORD version;
    DWORD type;
    HWND target;
    HWND overlay;
    volatile LONG result;
    WoxWindowHookDiagnostic diagnostic;
    WCHAR path[WOX_WINDOW_PATH_CAPACITY];
} WoxWindowHookCommand;

typedef struct
{
    HHOOK hook;
    HWND target;
    HWND overlay;
    DWORD targetPid;
    DWORD targetThread;
} WoxStickyHook;

static HMODULE gModule = NULL;
static UINT gCommandMessage = 0;
static UINT gStickyChangedMessage = 0;
static SRWLOCK gCommandLock = SRWLOCK_INIT;

// getCommandMessage registers the cross-process command outside DllMain.
static UINT getCommandMessage(void)
{
    UINT message = gCommandMessage;
    if (message == 0)
    {
        message = RegisterWindowMessageW(L"Wox.WindowHook.Command.v1");
        if (message != 0)
        {
            InterlockedCompareExchange((volatile LONG *)&gCommandMessage, (LONG)message, 0);
            message = gCommandMessage;
        }
    }
    return message;
}

// getStickyChangedMessage returns the shared notification used by injected targets and overlays.
static UINT getStickyChangedMessage(void)
{
    UINT message = gStickyChangedMessage;
    if (message == 0)
    {
        message = RegisterWindowMessageW(L"Wox.WindowHook.StickyChanged.v1");
        if (message != 0)
        {
            InterlockedCompareExchange((volatile LONG *)&gStickyChangedMessage, (LONG)message, 0);
            message = gStickyChangedMessage;
        }
    }
    return message;
}

// findShellViewProc locates the native Shell view hosted by a common file dialog.
static BOOL CALLBACK findShellViewProc(HWND hwnd, LPARAM lParam)
{
    WCHAR className[64];
    if (GetClassNameW(hwnd, className, (int)(sizeof(className) / sizeof(className[0]))) > 0 && _wcsicmp(className, L"SHELLDLL_DefView") == 0)
    {
        *((HWND *)lParam) = hwnd;
        return FALSE;
    }
    return TRUE;
}

// findShellView returns the dialog's Shell view and rejects unrelated #32770 windows.
static HWND findShellView(HWND dialog)
{
    WCHAR className[64];
    if (!dialog || !IsWindow(dialog) || GetClassNameW(dialog, className, (int)(sizeof(className) / sizeof(className[0]))) <= 0 || wcscmp(className, L"#32770") != 0)
    {
        return NULL;
    }

    HWND shellView = NULL;
    EnumChildWindows(dialog, findShellViewProc, (LPARAM)&shellView);
    return shellView;
}

// navigateShellView records the exact COM stage used by the injected dialog thread.
static HRESULT navigateShellView(HWND dialog, const WCHAR *targetPath, WoxWindowHookDiagnostic *diagnostic)
{
    HWND shellView = findShellView(dialog);
    diagnostic->shellViewFound = shellView != NULL;
    if (!shellView || !targetPath || targetPath[0] == L'\0')
    {
        diagnostic->stage = woxWindowHookStageCallbackValidate;
        diagnostic->hresult = E_INVALIDARG;
        return E_INVALIDARG;
    }

    diagnostic->stage = woxWindowHookStageCoInitialize;
    HRESULT initializeResult = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
    diagnostic->hresult = initializeResult;
    BOOL shouldUninitialize = SUCCEEDED(initializeResult);
    if (FAILED(initializeResult) && initializeResult != RPC_E_CHANGED_MODE)
    {
        return initializeResult;
    }

    IShellBrowser *shellBrowser = NULL;
    PIDLIST_ABSOLUTE pidl = NULL;

    diagnostic->stage = woxWindowHookStageGetShellBrowser;
    // SHELLDLL_DefView asks its parent browser for this interface, so the message must target the dialog host.
    shellBrowser = (IShellBrowser *)SendMessageW(dialog, WOX_WM_GETISHELLBROWSER, 0, 0);
    HRESULT result = shellBrowser ? S_OK : E_NOINTERFACE;
    diagnostic->hresult = result;
    if (shellBrowser)
    {
        IShellBrowser_AddRef(shellBrowser);
    }
    if (SUCCEEDED(result))
    {
        diagnostic->stage = woxWindowHookStageParsePath;
        result = SHParseDisplayName(targetPath, NULL, &pidl, 0, NULL);
        diagnostic->hresult = result;
    }
    if (SUCCEEDED(result))
    {
        diagnostic->stage = woxWindowHookStageBrowse;
        result = IShellBrowser_BrowseObject(shellBrowser, pidl, SBSP_SAMEBROWSER);
        diagnostic->hresult = result;
    }

    if (pidl)
    {
        CoTaskMemFree(pidl);
    }
    if (shellBrowser)
    {
        IShellBrowser_Release(shellBrowser);
    }
    if (shouldUninitialize)
    {
        CoUninitialize();
    }
    return result;
}

// selectShellViewItem uses the dialog's native Shell view so selection and filename state stay synchronized.
static HRESULT selectShellViewItem(HWND dialog, const WCHAR *targetPath, WoxWindowHookDiagnostic *diagnostic)
{
    HWND shellViewWindow = findShellView(dialog);
    diagnostic->shellViewFound = shellViewWindow != NULL;
    if (!shellViewWindow || !targetPath || targetPath[0] == L'\0')
    {
        diagnostic->stage = woxWindowHookStageCallbackValidate;
        diagnostic->hresult = E_INVALIDARG;
        return E_INVALIDARG;
    }

    diagnostic->stage = woxWindowHookStageCoInitialize;
    HRESULT initializeResult = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
    diagnostic->hresult = initializeResult;
    BOOL shouldUninitialize = SUCCEEDED(initializeResult);
    if (FAILED(initializeResult) && initializeResult != RPC_E_CHANGED_MODE)
    {
        return initializeResult;
    }

    IShellBrowser *shellBrowser = NULL;
    IShellView *shellView = NULL;
    IFolderView *folderView = NULL;
    IShellFolder *parentFolder = NULL;
    IShellFolder *viewFolder = NULL;
    PIDLIST_ABSOLUTE pidl = NULL;
    PIDLIST_ABSOLUTE parentPidl = NULL;
    PIDLIST_ABSOLUTE viewPidl = NULL;
    PCUITEMID_CHILD childPidl = NULL;

    diagnostic->stage = woxWindowHookStageGetShellBrowser;
    shellBrowser = (IShellBrowser *)SendMessageW(dialog, WOX_WM_GETISHELLBROWSER, 0, 0);
    HRESULT result = shellBrowser ? S_OK : E_NOINTERFACE;
    diagnostic->hresult = result;
    if (shellBrowser)
    {
        IShellBrowser_AddRef(shellBrowser);
    }
    if (SUCCEEDED(result))
    {
        diagnostic->stage = woxWindowHookStageQueryActiveView;
        result = IShellBrowser_QueryActiveShellView(shellBrowser, &shellView);
        diagnostic->hresult = result;
    }
    if (SUCCEEDED(result))
    {
        diagnostic->stage = woxWindowHookStageParsePath;
        result = SHParseDisplayName(targetPath, NULL, &pidl, 0, NULL);
        diagnostic->hresult = result;
    }
    if (SUCCEEDED(result))
    {
        diagnostic->stage = woxWindowHookStageBindParent;
        result = SHBindToParent(pidl, &IID_IShellFolder, (void **)&parentFolder, &childPidl);
        diagnostic->hresult = result;
    }
    if (SUCCEEDED(result))
    {
        diagnostic->stage = woxWindowHookStageGetViewFolder;
        result = IShellView_QueryInterface(shellView, &IID_IFolderView, (void **)&folderView);
        if (SUCCEEDED(result))
        {
            result = IFolderView_GetFolder(folderView, &IID_IShellFolder, (void **)&viewFolder);
        }
        diagnostic->hresult = result;
    }
    if (SUCCEEDED(result))
    {
        diagnostic->stage = woxWindowHookStageCompareParent;
        result = SHGetIDListFromObject((IUnknown *)parentFolder, &parentPidl);
        if (SUCCEEDED(result))
        {
            result = SHGetIDListFromObject((IUnknown *)viewFolder, &viewPidl);
        }
        if (SUCCEEDED(result) && !ILIsEqual(parentPidl, viewPidl))
        {
            result = HRESULT_FROM_WIN32(ERROR_RETRY);
        }
        diagnostic->hresult = result;
    }
    if (SUCCEEDED(result))
    {
        diagnostic->stage = woxWindowHookStageSelectItem;
        result = IShellView_SelectItem(shellView, childPidl, SVSI_SELECT | SVSI_DESELECTOTHERS | SVSI_ENSUREVISIBLE | SVSI_FOCUSED);
        diagnostic->hresult = result;
    }

    if (viewPidl)
    {
        CoTaskMemFree(viewPidl);
    }
    if (parentPidl)
    {
        CoTaskMemFree(parentPidl);
    }
    if (viewFolder)
    {
        IShellFolder_Release(viewFolder);
    }
    if (parentFolder)
    {
        IShellFolder_Release(parentFolder);
    }
    if (folderView)
    {
        IFolderView_Release(folderView);
    }
    if (pidl)
    {
        CoTaskMemFree(pidl);
    }
    if (shellView)
    {
        IShellView_Release(shellView);
    }
    if (shellBrowser)
    {
        IShellBrowser_Release(shellBrowser);
    }
    if (shouldUninitialize)
    {
        CoUninitialize();
    }
    return result;
}

// clearStickyOffset drops the published offset so a reattached overlay cannot inherit it.
static void clearStickyOffset(HWND target)
{
    RemovePropW(target, WOX_STICKY_OFFSET_ACTIVE_PROP);
    RemovePropW(target, WOX_STICKY_OFFSET_X_PROP);
    RemovePropW(target, WOX_STICKY_OFFSET_Y_PROP);
}

// translateStickyOverlay moves the overlay by the offset Wox published for the
// current layout. Only a translation is applied: the target keeps its size while it
// is dragged, so the offset stays valid and Wox's DPI and work-area policy does not
// have to be duplicated here. Wox still recomputes authoritatively on the posted
// notification and republishes a corrected offset when clamping or DPI changes it.
static void translateStickyOverlay(HWND target, HWND overlay)
{
    RECT targetRect;
    if (!IsWindowVisible(target) || IsIconic(target) ||
        !GetPropW(target, WOX_STICKY_OFFSET_ACTIVE_PROP) || !GetWindowRect(target, &targetRect))
    {
        return;
    }

    LONG offsetX = (LONG)(LONG_PTR)GetPropW(target, WOX_STICKY_OFFSET_X_PROP);
    LONG offsetY = (LONG)(LONG_PTR)GetPropW(target, WOX_STICKY_OFFSET_Y_PROP);
    SetWindowPos(overlay, NULL, targetRect.left + offsetX, targetRect.top + offsetY, 0, 0,
                 SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE | SWP_ASYNCWINDOWPOS);
}

// stickySubclassProc keeps the overlay glued by translating it from inside the target's own
// move message, and wakes Wox for a full relayout only when a translation cannot be correct.
static LRESULT CALLBACK stickySubclassProc(HWND hwnd, UINT message, WPARAM wParam, LPARAM lParam, UINT_PTR subclassId, DWORD_PTR refData)
{
    HWND overlay = (HWND)refData;
    if (!IsWindow(overlay))
    {
        RemoveWindowSubclass(hwnd, stickySubclassProc, subclassId);
        return DefSubclassProc(hwnd, message, wParam, lParam);
    }

    // Hide at the target's visibility transition, without waiting for Wox's
    // focus-handoff grace period. Async dispatch avoids blocking the target on
    // Wox's UI thread while Wox may be waiting for this injected thread.
    if ((message == WM_SHOWWINDOW && !wParam) ||
        (message == WM_WINDOWPOSCHANGING && lParam && (((const WINDOWPOS *)lParam)->flags & SWP_HIDEWINDOW)) ||
        message == WM_DESTROY || message == WM_NCDESTROY)
    {
        ShowWindowAsync(overlay, SW_HIDE);
    }

    switch (message)
    {
    case WM_WINDOWPOSCHANGED:
    {
        // Translate after the target has committed its new position so the overlay
        // moves against the same geometry the compositor just used.
        LRESULT result = DefSubclassProc(hwnd, message, wParam, lParam);
        translateStickyOverlay(hwnd, overlay);
        // A pure move is already fully handled by that translation. Notifying Wox here
        // too would queue one full relayout per mouse-move message, and Wox cannot drain
        // them at drag rate: the backlog replays the entire drag from stale coordinates
        // after the user lets go, and each stale relayout republishes a corrupted offset.
        // Showing also needs a full layout: the initial overlay may have been
        // positioned before the target became visible to window management.
        const WINDOWPOS *position = (const WINDOWPOS *)lParam;
        if (!position || !(position->flags & SWP_NOSIZE) || (position->flags & SWP_SHOWWINDOW))
        {
            PostMessageW(overlay, getStickyChangedMessage(), (WPARAM)hwnd, 0);
        }
        return result;
    }
    case WM_EXITSIZEMOVE:
        // One authoritative recompute once the drag settles. This is what re-clamps the
        // overlay after the target crosses onto a display with a different work area or
        // scale, which a plain translation cannot account for, and it costs a single
        // relayout instead of one per drag step.
        PostMessageW(overlay, getStickyChangedMessage(), (WPARAM)hwnd, 0);
        break;
    case WM_DPICHANGED:
        PostMessageW(overlay, getStickyChangedMessage(), (WPARAM)hwnd, 0);
        break;
    case WM_NCDESTROY:
        PostMessageW(overlay, getStickyChangedMessage(), (WPARAM)hwnd, 0);
        clearStickyOffset(hwnd);
        RemoveWindowSubclass(hwnd, stickySubclassProc, subclassId);
        break;
    }

    return DefSubclassProc(hwnd, message, wParam, lParam);
}

// buildObjectName creates per-Wox IPC names without exposing path data in window messages.
static void buildObjectName(WCHAR *buffer, size_t capacity, const WCHAR *kind, DWORD ownerPid)
{
    _snwprintf_s(buffer, capacity, _TRUNCATE, L"Local\\WoxWindowHook.%s.%lu", kind, ownerPid);
}

// executeCommand runs on the selected target thread and reports completion through a named event.
static void executeCommand(DWORD ownerPid)
{
    WCHAR mappingName[128];
    WCHAR eventName[128];
    buildObjectName(mappingName, sizeof(mappingName) / sizeof(mappingName[0]), L"Mapping", ownerPid);
    buildObjectName(eventName, sizeof(eventName) / sizeof(eventName[0]), L"Event", ownerPid);

    HANDLE mapping = OpenFileMappingW(FILE_MAP_READ | FILE_MAP_WRITE, FALSE, mappingName);
    HANDLE completed = OpenEventW(EVENT_MODIFY_STATE, FALSE, eventName);
    if (!mapping || !completed)
    {
        if (mapping)
            CloseHandle(mapping);
        if (completed)
            CloseHandle(completed);
        return;
    }

    WoxWindowHookCommand *command = (WoxWindowHookCommand *)MapViewOfFile(mapping, FILE_MAP_READ | FILE_MAP_WRITE, 0, 0, sizeof(WoxWindowHookCommand));
    if (command)
    {
        command->diagnostic.stage = woxWindowHookStageCallback;
        command->diagnostic.callbackEntered = 1;
        DWORD targetPid = 0;
        DWORD targetThread = GetWindowThreadProcessId(command->target, &targetPid);
        command->diagnostic.targetPid = targetPid;
        command->diagnostic.targetThread = targetThread;
        command->diagnostic.stage = woxWindowHookStageCallbackValidate;

        BOOL valid = command->version == WOX_WINDOW_HOOK_VERSION && targetPid == GetCurrentProcessId() && targetThread == GetCurrentThreadId();
        BOOL succeeded = FALSE;
        if (valid && command->type == woxWindowHookCommandNavigateDialog && findShellView(command->target))
        {
            succeeded = SUCCEEDED(navigateShellView(command->target, command->path, &command->diagnostic));
        }
        else if (valid && command->type == woxWindowHookCommandSelectDialogItem && findShellView(command->target))
        {
            succeeded = SUCCEEDED(selectShellViewItem(command->target, command->path, &command->diagnostic));
        }
        else if (valid && command->type == woxWindowHookCommandAttachSticky && IsWindow(command->overlay))
        {
            succeeded = SetWindowSubclass(command->target, stickySubclassProc, (UINT_PTR)command->overlay, (DWORD_PTR)command->overlay);
        }
        else if (valid && command->type == woxWindowHookCommandDetachSticky)
        {
            // An already-absent subclass is also detached; avoid GetWindowSubclass because older comctl32 hosts do not export it by name.
            RemoveWindowSubclass(command->target, stickySubclassProc, (UINT_PTR)command->overlay);
            clearStickyOffset(command->target);
            succeeded = TRUE;
        }

        if (succeeded)
        {
            command->diagnostic.stage = woxWindowHookStageCompleted;
            InterlockedExchange(&command->result, 1);
        }
        else
        {
            if (!valid)
                command->diagnostic.win32Error = ERROR_INVALID_WINDOW_HANDLE;
            else if (command->diagnostic.win32Error == 0)
                command->diagnostic.win32Error = GetLastError();
            InterlockedExchange(&command->result, -1);
        }
        UnmapViewOfFile(command);
    }
    SetEvent(completed);
    CloseHandle(completed);
    CloseHandle(mapping);
}

// getMessageHookProc handles only Wox's registered command on the selected target thread.
static LRESULT CALLBACK getMessageHookProc(int code, WPARAM wParam, LPARAM lParam)
{
    UINT commandMessage = getCommandMessage();
    if (code >= 0 && wParam == PM_REMOVE && lParam)
    {
        MSG *message = (MSG *)lParam;
        if (commandMessage != 0 && message->message == commandMessage)
        {
            executeCommand((DWORD)message->wParam);
            message->message = WM_NULL;
        }
    }
    return CallNextHookEx(NULL, code, wParam, lParam);
}

// sendCommand serializes the fixed IPC names and dispatches one command through an installed hook.
static BOOL sendCommand(HHOOK hook, DWORD targetThread, WoxWindowHookCommand *input, WoxWindowHookDiagnostic *diagnostic)
{
    DWORD ownerPid = GetCurrentProcessId();
    WCHAR mappingName[128];
    WCHAR eventName[128];
    buildObjectName(mappingName, sizeof(mappingName) / sizeof(mappingName[0]), L"Mapping", ownerPid);
    buildObjectName(eventName, sizeof(eventName) / sizeof(eventName[0]), L"Event", ownerPid);

    diagnostic->stage = woxWindowHookStageCreateIpc;
    HANDLE mapping = CreateFileMappingW(INVALID_HANDLE_VALUE, NULL, PAGE_READWRITE, 0, sizeof(WoxWindowHookCommand), mappingName);
    HANDLE completed = CreateEventW(NULL, TRUE, FALSE, eventName);
    if (!mapping || !completed)
    {
        diagnostic->win32Error = GetLastError();
        if (mapping)
            CloseHandle(mapping);
        if (completed)
            CloseHandle(completed);
        return FALSE;
    }

    diagnostic->stage = woxWindowHookStageMapIpc;
    WoxWindowHookCommand *command = (WoxWindowHookCommand *)MapViewOfFile(mapping, FILE_MAP_READ | FILE_MAP_WRITE, 0, 0, sizeof(WoxWindowHookCommand));
    if (!command)
    {
        diagnostic->win32Error = GetLastError();
        CloseHandle(completed);
        CloseHandle(mapping);
        return FALSE;
    }

    *command = *input;
    command->diagnostic = *diagnostic;
    command->diagnostic.hookInstalled = hook != NULL;
    ResetEvent(completed);

    command->diagnostic.stage = woxWindowHookStagePostMessage;
    BOOL succeeded = FALSE;
    if (PostThreadMessageW(targetThread, getCommandMessage(), ownerPid, 0))
    {
        command->diagnostic.stage = woxWindowHookStageWait;
        DWORD waitResult = WaitForSingleObject(completed, WOX_WINDOW_HOOK_TIMEOUT_MS);
        command->diagnostic.waitResult = waitResult;
        if (waitResult == WAIT_OBJECT_0)
        {
            succeeded = InterlockedCompareExchange(&command->result, 0, 0) == 1;
        }
        else
        {
            command->diagnostic.win32Error = waitResult == WAIT_TIMEOUT ? ERROR_TIMEOUT : GetLastError();
        }
    }
    else
    {
        command->diagnostic.win32Error = GetLastError();
    }

    *diagnostic = command->diagnostic;
    UnmapViewOfFile(command);
    CloseHandle(completed);
    CloseHandle(mapping);
    return succeeded;
}

// sendDialogPathCommand injects one path command into a verified dialog thread.
static BOOL sendDialogPathCommand(HWND dialog, DWORD expectedPid, const WCHAR *targetPath, DWORD commandType, WoxWindowHookDiagnostic *diagnostic)
{
    WoxWindowHookDiagnostic localDiagnostic;
    if (!diagnostic)
        diagnostic = &localDiagnostic;
    ZeroMemory(diagnostic, sizeof(*diagnostic));
    diagnostic->stage = woxWindowHookStageValidateInput;

    UINT commandMessage = getCommandMessage();
    if (!gModule || !commandMessage || expectedPid == 0 || !targetPath || targetPath[0] == L'\0' ||
        (commandType != woxWindowHookCommandNavigateDialog && commandType != woxWindowHookCommandSelectDialogItem))
    {
        diagnostic->win32Error = ERROR_INVALID_PARAMETER;
        return FALSE;
    }

    diagnostic->stage = woxWindowHookStageValidateWindow;
    diagnostic->shellViewFound = findShellView(dialog) != NULL;
    if (!diagnostic->shellViewFound)
    {
        diagnostic->win32Error = ERROR_INVALID_WINDOW_HANDLE;
        return FALSE;
    }

    diagnostic->stage = woxWindowHookStageResolveThread;
    DWORD dialogPid = 0;
    DWORD dialogThread = GetWindowThreadProcessId(dialog, &dialogPid);
    diagnostic->targetPid = dialogPid;
    diagnostic->targetThread = dialogThread;
    if (dialogThread == 0 || dialogPid != expectedPid || dialogPid == GetCurrentProcessId())
    {
        diagnostic->win32Error = ERROR_INVALID_WINDOW_HANDLE;
        return FALSE;
    }

    AcquireSRWLockExclusive(&gCommandLock);
    diagnostic->stage = woxWindowHookStageInstallHook;
    HHOOK hook = SetWindowsHookExW(WH_GETMESSAGE, getMessageHookProc, gModule, dialogThread);
    BOOL succeeded = FALSE;
    if (hook)
    {
        WoxWindowHookCommand command;
        ZeroMemory(&command, sizeof(command));
        command.version = WOX_WINDOW_HOOK_VERSION;
        command.type = commandType;
        command.target = dialog;
        wcsncpy_s(command.path, WOX_WINDOW_PATH_CAPACITY, targetPath, _TRUNCATE);
        succeeded = sendCommand(hook, dialogThread, &command, diagnostic);
        UnhookWindowsHookEx(hook);
    }
    else
    {
        diagnostic->win32Error = GetLastError();
    }
    ReleaseSRWLockExclusive(&gCommandLock);
    return succeeded;
}

// WoxWindowHookNavigateDialog runs one native Shell browser navigation.
__declspec(dllexport) BOOL WINAPI WoxWindowHookNavigateDialog(HWND dialog, DWORD expectedPid, const WCHAR *targetPath, WoxWindowHookDiagnostic *diagnostic)
{
    return sendDialogPathCommand(dialog, expectedPid, targetPath, woxWindowHookCommandNavigateDialog, diagnostic);
}

// WoxWindowHookSelectDialogItem selects one item in the dialog's active Shell view.
__declspec(dllexport) BOOL WINAPI WoxWindowHookSelectDialogItem(HWND dialog, DWORD expectedPid, const WCHAR *targetPath, WoxWindowHookDiagnostic *diagnostic)
{
    return sendDialogPathCommand(dialog, expectedPid, targetPath, woxWindowHookCommandSelectDialogItem, diagnostic);
}

// WoxWindowHookAttachSticky keeps the injected hook alive until the overlay explicitly detaches it.
__declspec(dllexport) void *WINAPI WoxWindowHookAttachSticky(HWND target, DWORD expectedPid, HWND overlay, WoxWindowHookDiagnostic *diagnostic)
{
    WoxWindowHookDiagnostic localDiagnostic;
    if (!diagnostic)
        diagnostic = &localDiagnostic;
    ZeroMemory(diagnostic, sizeof(*diagnostic));

    DWORD targetPid = 0;
    DWORD targetThread = GetWindowThreadProcessId(target, &targetPid);
    diagnostic->targetPid = targetPid;
    diagnostic->targetThread = targetThread;
    diagnostic->stage = woxWindowHookStageValidateInput;
    if (!gModule || !getCommandMessage() || !targetThread || targetPid != expectedPid || !IsWindow(overlay))
    {
        diagnostic->win32Error = ERROR_INVALID_PARAMETER;
        return NULL;
    }

    // Wox owns both HWNDs on its UI thread. Posting an injection command and
    // waiting here would block that very thread; install the same subclass directly.
    if (targetPid == GetCurrentProcessId())
    {
        if (targetThread != GetCurrentThreadId())
        {
            diagnostic->win32Error = ERROR_INVALID_THREAD_ID;
            return NULL;
        }
        WoxStickyHook *sticky = (WoxStickyHook *)calloc(1, sizeof(WoxStickyHook));
        if (!sticky)
            return NULL;
        if (!SetWindowSubclass(target, stickySubclassProc, (UINT_PTR)overlay, (DWORD_PTR)overlay))
        {
            diagnostic->win32Error = GetLastError();
            free(sticky);
            return NULL;
        }
        sticky->target = target;
        sticky->overlay = overlay;
        sticky->targetPid = targetPid;
        sticky->targetThread = targetThread;
        return sticky;
    }

    AcquireSRWLockExclusive(&gCommandLock);
    diagnostic->stage = woxWindowHookStageInstallHook;
    HHOOK hook = SetWindowsHookExW(WH_GETMESSAGE, getMessageHookProc, gModule, targetThread);
    diagnostic->hookInstalled = hook != NULL;
    WoxStickyHook *sticky = NULL;
    if (!hook)
    {
        diagnostic->win32Error = GetLastError();
    }
    if (hook)
    {
        WoxWindowHookCommand command;
        ZeroMemory(&command, sizeof(command));
        command.version = WOX_WINDOW_HOOK_VERSION;
        command.type = woxWindowHookCommandAttachSticky;
        command.target = target;
        command.overlay = overlay;
        if (sendCommand(hook, targetThread, &command, diagnostic))
        {
            sticky = (WoxStickyHook *)calloc(1, sizeof(WoxStickyHook));
            if (sticky)
            {
                sticky->hook = hook;
                sticky->target = target;
                sticky->overlay = overlay;
                sticky->targetPid = targetPid;
                sticky->targetThread = targetThread;
                hook = NULL;
            }
            else
            {
                command.type = woxWindowHookCommandDetachSticky;
                sendCommand(hook, targetThread, &command, diagnostic);
            }
        }
        if (hook)
            UnhookWindowsHookEx(hook);
    }
    ReleaseSRWLockExclusive(&gCommandLock);
    return sticky;
}

// WoxWindowHookDetachSticky releases the injection hook only after the target subclass is gone.
__declspec(dllexport) BOOL WINAPI WoxWindowHookDetachSticky(void *handle)
{
    WoxStickyHook *sticky = (WoxStickyHook *)handle;
    if (!sticky)
        return TRUE;

    // A null hook marks a directly installed same-thread subclass. The DLL
    // stays loaded until removal succeeds, just as for an injected attachment.
    if (!sticky->hook)
    {
        if (GetCurrentThreadId() != sticky->targetThread)
            return FALSE;
        if (IsWindow(sticky->target))
        {
            RemoveWindowSubclass(sticky->target, stickySubclassProc, (UINT_PTR)sticky->overlay);
            clearStickyOffset(sticky->target);
        }
        free(sticky);
        return TRUE;
    }

    AcquireSRWLockExclusive(&gCommandLock);
    DWORD targetPid = 0;
    DWORD targetThread = GetWindowThreadProcessId(sticky->target, &targetPid);
    BOOL detached = targetThread != sticky->targetThread || targetPid != sticky->targetPid;
    if (targetThread == sticky->targetThread && targetPid == sticky->targetPid)
    {
        WoxWindowHookCommand command;
        WoxWindowHookDiagnostic diagnostic;
        ZeroMemory(&command, sizeof(command));
        ZeroMemory(&diagnostic, sizeof(diagnostic));
        command.version = WOX_WINDOW_HOOK_VERSION;
        command.type = woxWindowHookCommandDetachSticky;
        command.target = sticky->target;
        command.overlay = sticky->overlay;
        detached = sendCommand(sticky->hook, sticky->targetThread, &command, &diagnostic);
    }
    if (detached)
    {
        UnhookWindowsHookEx(sticky->hook);
        free(sticky);
    }
    // A timed-out target must retain its hook; unloading the DLL while its subclass remains would leave a dangling window procedure.
    ReleaseSRWLockExclusive(&gCommandLock);
    return detached;
}

BOOL WINAPI DllMain(HINSTANCE instance, DWORD reason, LPVOID reserved)
{
    (void)reserved;
    if (reason == DLL_PROCESS_ATTACH)
    {
        gModule = instance;
        DisableThreadLibraryCalls(instance);
    }
    return TRUE;
}
