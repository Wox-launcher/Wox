package nativecontextmenu

/*
#cgo LDFLAGS: -lshell32 -lole32 -luuid -luser32 -lshlwapi

#include <windows.h>
#include <shlobj.h>
#include <shobjidl.h>
#include <shellapi.h>
#include <shlwapi.h>
#include <string.h>


// Global pointers for IContextMenu2/3 to handle menu messages
static IContextMenu2* g_pcm2 = NULL;
static IContextMenu3* g_pcm3 = NULL;

static LRESULT CALLBACK WoxCtxMenuWndProc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam) {
    if (g_pcm3 && msg == WM_MENUCHAR) {
        LRESULT lres = 0;
        if (SUCCEEDED(g_pcm3->lpVtbl->HandleMenuMsg2(g_pcm3, msg, wParam, lParam, &lres))) {
            return lres;
        }
    }
    if (g_pcm2 && (msg == WM_INITMENUPOPUP || msg == WM_DRAWITEM || msg == WM_MEASUREITEM)) {
        if (SUCCEEDED(g_pcm2->lpVtbl->HandleMenuMsg(g_pcm2, msg, wParam, lParam))) {
            return 0;
        }
    }
    return DefWindowProc(hwnd, msg, wParam, lParam);
}

// ShowContextMenu displays the Windows shell context menu for a file or folder
// Returns 0 on success, non-zero on error
int ShowContextMenu(const wchar_t* path) {
    HRESULT hr;
    IShellFolder *pDesktopFolder = NULL;
    IShellFolder *pParentFolder = NULL;
    IContextMenu *pContextMenu = NULL;
    LPITEMIDLIST pidl = NULL;
    LPITEMIDLIST pidlChild = NULL;
    HWND hwnd = NULL;
    POINT pt;

    // Initialize COM
    hr = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED | COINIT_DISABLE_OLE1DDE);
    if (FAILED(hr) && hr != RPC_E_CHANGED_MODE) {
        return 1;
    }

    // Get desktop folder
    hr = SHGetDesktopFolder(&pDesktopFolder);
    if (FAILED(hr)) {
        CoUninitialize();
        return 2;
    }

    // Parse the path to get PIDL
    hr = pDesktopFolder->lpVtbl->ParseDisplayName(pDesktopFolder, NULL, NULL, (LPWSTR)path, NULL, &pidl, NULL);
    if (FAILED(hr)) {
        pDesktopFolder->lpVtbl->Release(pDesktopFolder);
        CoUninitialize();
        return 3;
    }

    // Get parent folder and child PIDL
    LPITEMIDLIST pidlParent = ILClone(pidl);
    if (!ILRemoveLastID(pidlParent)) {
        CoTaskMemFree(pidl);
        pDesktopFolder->lpVtbl->Release(pDesktopFolder);
        CoUninitialize();
        return 4;
    }

    // Get the parent folder interface
    hr = pDesktopFolder->lpVtbl->BindToObject(pDesktopFolder, pidlParent, NULL, &IID_IShellFolder, (void**)&pParentFolder);
    CoTaskMemFree(pidlParent);

    if (FAILED(hr)) {
        CoTaskMemFree(pidl);
        pDesktopFolder->lpVtbl->Release(pDesktopFolder);
        CoUninitialize();
        return 5;
    }

    // Get the child PIDL (relative to parent)
    pidlChild = ILFindLastID(pidl);
    // Register window class and create a hidden window to receive menu messages
    WNDCLASSW wc = {0};
    wc.lpfnWndProc = WoxCtxMenuWndProc;
    wc.hInstance = GetModuleHandleW(NULL);
    wc.lpszClassName = L"WoxCtxMenuWnd";
    RegisterClassW(&wc);
    hwnd = CreateWindowExW(0, L"WoxCtxMenuWnd", L"", WS_OVERLAPPED, 0, 0, 0, 0, NULL, NULL, wc.hInstance, NULL);
    if (!hwnd) {
        CoTaskMemFree(pidl);
        pParentFolder->lpVtbl->Release(pParentFolder);
        pDesktopFolder->lpVtbl->Release(pDesktopFolder);
        CoUninitialize();
        return 6;
    }


    // Get the context menu interface
    hr = pParentFolder->lpVtbl->GetUIObjectOf(pParentFolder, hwnd, 1, (LPCITEMIDLIST*)&pidlChild, &IID_IContextMenu, NULL, (void**)&pContextMenu);

    if (FAILED(hr)) {
        CoTaskMemFree(pidl);
        pParentFolder->lpVtbl->Release(pParentFolder);
        pDesktopFolder->lpVtbl->Release(pDesktopFolder);
        CoUninitialize();
        return 6;
    }
    // Query IContextMenu2/IContextMenu3 for proper menu message handling
    if (pContextMenu) {
        pContextMenu->lpVtbl->QueryInterface(pContextMenu, &IID_IContextMenu2, (void**)&g_pcm2);
        pContextMenu->lpVtbl->QueryInterface(pContextMenu, &IID_IContextMenu3, (void**)&g_pcm3);
    }


    // Create a popup menu
    HMENU hMenu = CreatePopupMenu();
    if (!hMenu) {
        pContextMenu->lpVtbl->Release(pContextMenu);
        CoTaskMemFree(pidl);
        pParentFolder->lpVtbl->Release(pParentFolder);
        pDesktopFolder->lpVtbl->Release(pDesktopFolder);
        CoUninitialize();
        return 7;
    }

    // Query the context menu (include extended verbs if Shift is down)
    UINT flags = CMF_NORMAL | CMF_EXPLORE;
    if (GetKeyState(VK_SHIFT) & 0x8000) {
        flags |= CMF_EXTENDEDVERBS;
    }
    hr = pContextMenu->lpVtbl->QueryContextMenu(pContextMenu, hMenu, 0, 1, 0x7FFF, flags);
    if (FAILED(hr)) {
        DestroyMenu(hMenu);
        if (g_pcm3) { g_pcm3->lpVtbl->Release(g_pcm3); g_pcm3 = NULL; }
        if (g_pcm2) { g_pcm2->lpVtbl->Release(g_pcm2); g_pcm2 = NULL; }
        pContextMenu->lpVtbl->Release(pContextMenu);
        CoTaskMemFree(pidl);
        pParentFolder->lpVtbl->Release(pParentFolder);
        pDesktopFolder->lpVtbl->Release(pDesktopFolder);
        CoUninitialize();

        return 8;
    }

    // Get cursor position for menu display
    GetCursorPos(&pt);

    // Determine the owner window: prefer Wox UI if available
    HWND ownerHwnd = hwnd;
    HWND woxHwnd = FindWindowW(L"FLUTTER_RUNNER_WIN32_WINDOW", L"wox-ui");
    if (woxHwnd) { ownerHwnd = woxHwnd; }

    // Display the context menu (hidden window receives Shell menu messages)
    // IMPORTANT: Bring the popup owner (our hidden message window) to foreground,
    // otherwise clicking外部不会自动关闭菜单。
    SetForegroundWindow(hwnd);
    int cmd = TrackPopupMenuEx(hMenu, TPM_RETURNCMD | TPM_RIGHTBUTTON, pt.x, pt.y, hwnd, NULL);
    // Post a dummy message to complete the menu loop so it can properly dismiss
    PostMessage(hwnd, WM_NULL, 0, 0);

    // Execute the selected command
    if (cmd > 0) {
        // Get the parent directory path (Unicode)
        WCHAR parentDirW[MAX_PATH];
        wcscpy_s(parentDirW, MAX_PATH, path);
        PathRemoveFileSpecW(parentDirW);

        // Convert to ANSI for lpDirectory
        CHAR parentDirA[MAX_PATH];
        WideCharToMultiByte(CP_ACP, 0, parentDirW, -1, parentDirA, MAX_PATH, NULL, NULL);

        // If the selected command is "properties", call SHObjectProperties directly (more reliable)
        CHAR verbA[256] = {0};
        HRESULT ghr = pContextMenu->lpVtbl->GetCommandString(pContextMenu, cmd - 1, GCS_VERBA, NULL, verbA, 256);
        if (SUCCEEDED(ghr) && _stricmp(verbA, "properties") == 0) {
            SHObjectProperties(ownerHwnd, SHOP_FILEPATH, path, NULL);
        } else {
            CMINVOKECOMMANDINFOEX ici = {0};
            ici.cbSize = sizeof(CMINVOKECOMMANDINFOEX);
            ici.fMask = CMIC_MASK_UNICODE | CMIC_MASK_PTINVOKE;
            if (GetKeyState(VK_CONTROL) & 0x8000) { ici.fMask |= CMIC_MASK_CONTROL_DOWN; }
            if (GetKeyState(VK_SHIFT) & 0x8000) { ici.fMask |= CMIC_MASK_SHIFT_DOWN; }
            ici.hwnd = ownerHwnd;
            ici.lpVerb = MAKEINTRESOURCEA(cmd - 1);
            ici.lpVerbW = MAKEINTRESOURCEW(cmd - 1);
            ici.lpDirectory = parentDirA;      // Set parent directory (ANSI)
            ici.lpDirectoryW = parentDirW;     // Set parent directory (Unicode)
            ici.nShow = SW_SHOWNORMAL;
            ici.ptInvoke = pt;
            hr = pContextMenu->lpVtbl->InvokeCommand(pContextMenu, (LPCMINVOKECOMMANDINFO)&ici);
        }
    }

    // Cleanup
    DestroyMenu(hMenu);
    if (g_pcm3) { g_pcm3->lpVtbl->Release(g_pcm3); g_pcm3 = NULL; }
    if (g_pcm2) { g_pcm2->lpVtbl->Release(g_pcm2); g_pcm2 = NULL; }
    if (hwnd) { DestroyWindow(hwnd); }
    pContextMenu->lpVtbl->Release(pContextMenu);
    CoTaskMemFree(pidl);
    pParentFolder->lpVtbl->Release(pParentFolder);
    pDesktopFolder->lpVtbl->Release(pDesktopFolder);
    CoUninitialize();

    return 0;
}
*/
import "C"
import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

// ShowContextMenu displays the system context menu for a file or folder on Windows
func ShowContextMenu(path string) error {
	// Convert to absolute path first
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Convert path to UTF-16
	pathPtr, err := syscall.UTF16PtrFromString(absPath)
	if err != nil {
		return fmt.Errorf("failed to convert path to UTF-16: %w", err)
	}

	// Call the C function
	result := C.ShowContextMenu((*C.wchar_t)(unsafe.Pointer(pathPtr)))
	if result != 0 {
		return fmt.Errorf("failed to show context menu, error code: %d", result)
	}

	return nil
}
