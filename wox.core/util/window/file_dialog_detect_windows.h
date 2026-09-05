#ifndef WOX_FILE_DIALOG_DETECT_WINDOWS_H
#define WOX_FILE_DIALOG_DETECT_WINDOWS_H

#include <windows.h>
#include <wchar.h>

// SHBrowseForFolder hosts its tree inside this unique child class.
#define WOX_BROWSE_FOLDER_NAMESPACE_CLASS L"SHBrowseForFolder ShellNameSpace Control"
#define WOX_BROWSE_FOLDER_TREE_ID 0x0064
#define WOX_BROWSE_FOLDER_EDIT_ID 0x3744

// woxWideContainsInsensitive reports whether haystack contains needle ignoring case.
static int woxWideContainsInsensitive(const WCHAR *haystack, const WCHAR *needle)
{
    if (!haystack || !needle || needle[0] == L'\0')
    {
        return 0;
    }

    size_t hayLen = wcslen(haystack);
    size_t needleLen = wcslen(needle);
    if (needleLen > hayLen)
    {
        return 0;
    }

    for (size_t i = 0; i + needleLen <= hayLen; i++)
    {
        if (_wcsnicmp(haystack + i, needle, needleLen) == 0)
        {
            return 1;
        }
    }
    return 0;
}

static int woxChildHasClass(HWND hwnd, const WCHAR *className)
{
    WCHAR actual[256];
    if (!hwnd || !className || GetClassNameW(hwnd, actual, 256) == 0)
    {
        return 0;
    }
    return _wcsicmp(actual, className) == 0;
}

// woxDialogLooksLikeSelectFolder matches localized folder-picker chrome.
// Uncertain dialogs return false so ordinary Open/Save keep showing files.
static int woxDialogLooksLikeSelectFolder(HWND hwnd)
{
    if (!hwnd)
    {
        return 0;
    }

    WCHAR title[512];
    ZeroMemory(title, sizeof(title));
    GetWindowTextW(hwnd, title, sizeof(title) / sizeof(title[0]));

    // Match localized and English folder-picker chrome without treating "Open File" as folder-only.
    // Require stronger phrases than a bare "Open" so uncertain dialogs stay false.
    // Do not read child button text: SendMessage into SHBrowseForFolder children can crash explorer.exe.
    static const WCHAR *folderHints[] = {
        L"open folder",
        L"open directory",
        L"select folder",
        L"select project root",
        L"choose folder",
        L"browse for folder",
        L"browse folder",
        L"select directory",
        L"choose directory",
        L"move items",
        L"copy items",
        L"\x6253\x5f00\x6587\x4ef6\x5939", // Simplified Chinese: Open Folder
        L"\x6253\x5f00\x76ee\x5f55", // Simplified Chinese: Open Directory
        L"\x9009\x62e9\x6587\x4ef6\x5939", // Simplified Chinese: Select Folder
        L"\x9009\x62e9\x76ee\x5f55", // Simplified Chinese: Select Directory
        L"\x6d4f\x89c8\x6587\x4ef6\x5939", // Simplified Chinese: Browse Folder
        L"\x79fb\x52a8\x9879\x76ee", // Simplified Chinese: Move Items
        L"\x590d\x5236\x9879\x76ee", // Simplified Chinese: Copy Items
        L"\x79fb\x52d5\x9805\x76ee", // Traditional Chinese: Move Items
        L"\x8907\x88fd\x9805\x76ee", // Traditional Chinese: Copy Items
        L"\x700f\x89bd\x8cc7\x6599\x593e", // Traditional Chinese: Browse Folder
        L"\x30d5\x30a9\x30eb\x30c0\x3092\x9078\x629e", // Japanese: Select Folder
        L"\x30d5\x30a9\x30eb\x30c0\x3092\x958b\x304f", // Japanese: Open Folder
        L"\x043f\x0435\x0440\x0435\x043c\x0435\x0449\x0435\x043d\x0438\x0435 \x044d\x043b\x0435\x043c\x0435\x043d\x0442\x043e\x0432", // Russian: Move Items
        L"\x043a\x043e\x043f\x0438\x0440\x043e\x0432\x0430\x043d\x0438\x0435 \x044d\x043b\x0435\x043c\x0435\x043d\x0442\x043e\x0432", // Russian: Copy Items
        L"\x043e\x0431\x0437\x043e\x0440 \x043f\x0430\x043f\x043e\x043a", // Russian: Browse Folders
        L"mover itens",
        L"copiar itens",
        L"procurar pasta",
        L"\xd56d\xbaa9 \xc774\xb3d9", // Korean: Move Items
        L"\xd56d\xbaa9 \xbcf5\xc0ac", // Korean: Copy Items
        L"\xd3f4\xb354 \xcc3e\xc544\xbcf4\xae30", // Korean: Browse Folder
    };

    for (size_t i = 0; i < sizeof(folderHints) / sizeof(folderHints[0]); i++)
    {
        if (woxWideContainsInsensitive(title, folderHints[i]))
        {
            return 1;
        }
    }

    // IFileDialog folder pickers often put the hint on the OK button, not the title.
    // Do not read child text of SHBrowseForFolder: SendMessage into ShellNameSpace
    // crashes explorer.exe.
    if (FindWindowExW(hwnd, NULL, WOX_BROWSE_FOLDER_NAMESPACE_CLASS, NULL))
    {
        return 0;
    }

    WCHAR buttonText[256];
    ZeroMemory(buttonText, sizeof(buttonText));
    HWND hOk = GetDlgItem(hwnd, IDOK);
    if (hOk)
    {
        GetWindowTextW(hOk, buttonText, sizeof(buttonText) / sizeof(buttonText[0]));
    }
    for (size_t i = 0; i < sizeof(folderHints) / sizeof(folderHints[0]); i++)
    {
        if (woxWideContainsInsensitive(buttonText, folderHints[i]))
        {
            return 1;
        }
    }
    return 0;
}

// woxDialogLooksLikeBrowseForFolder is true for Explorer Move/Copy Items only.
// IFileDialog folder pickers such as Open Folder share select-folder chrome but
// are not SHBrowseForFolder and must keep using the common-item path readers.
static int woxDialogLooksLikeBrowseForFolder(HWND hwnd)
{
    if (!hwnd)
    {
        return 0;
    }

    WCHAR title[512];
    ZeroMemory(title, sizeof(title));
    GetWindowTextW(hwnd, title, sizeof(title) / sizeof(title[0]));

    static const WCHAR *browseHints[] = {
        L"move items",
        L"copy items",
        L"\x79fb\x52a8\x9879\x76ee", // Simplified Chinese: Move Items
        L"\x590d\x5236\x9879\x76ee", // Simplified Chinese: Copy Items
        L"\x79fb\x52d5\x9805\x76ee", // Traditional Chinese: Move Items
        L"\x8907\x88fd\x9805\x76ee", // Traditional Chinese: Copy Items
        L"\x043f\x0435\x0440\x0435\x043c\x0435\x0449\x0435\x043d\x0438\x0435 \x044d\x043b\x0435\x043c\x0435\x043d\x0442\x043e\x0432", // Russian: Move Items
        L"\x043a\x043e\x043f\x0438\x0440\x043e\x0432\x0430\x043d\x0438\x0435 \x044d\x043b\x0435\x043c\x0435\x043d\x0442\x043e\x0432", // Russian: Copy Items
        L"mover itens",
        L"copiar itens",
        L"\xd56d\xbaa9 \xc774\xb3d9", // Korean: Move Items
        L"\xd56d\xbaa9 \xbcf5\xc0ac", // Korean: Copy Items
    };

    for (size_t i = 0; i < sizeof(browseHints) / sizeof(browseHints[0]); i++)
    {
        if (woxWideContainsInsensitive(title, browseHints[i]))
        {
            return 1;
        }
    }
    return 0;
}

// woxIsBrowseForFolderWindow detects SHBrowseForFolder dialogs such as Explorer's
// Move Items / Copy Items chooser. Do not EnumChildWindows or FindWindowEx into
// SHBrowseForFolder ShellNameSpace Control; that crashes explorer.exe.
// Finding the namespace class on the dialog itself is safe: it does not walk
// into the host. IFileDialog Open Folder must not take this path.
static int woxIsBrowseForFolderWindow(HWND hwnd)
{
    if (!woxChildHasClass(hwnd, L"#32770"))
    {
        return 0;
    }
    if (FindWindowExW(hwnd, NULL, WOX_BROWSE_FOLDER_NAMESPACE_CLASS, NULL))
    {
        return 1;
    }
    return woxDialogLooksLikeBrowseForFolder(hwnd);
}

// Explorer's own dialogs put SHELLDLL_DefView two or three levels down, but Office
// wraps it in its own DirectUI host and pushes it to four:
//   #32770 > DUIViewWndClassName > DirectUIHWND > CtrlNotifySink > SHELLDLL_DefView
// The limit stays bounded so detection cannot turn into an unbounded walk of a
// deeply nested tree.
#define WOX_DEF_VIEW_MAX_DEPTH 5

// woxFindDefViewWithin searches for SHELLDLL_DefView without ever entering
// SHBrowseForFolder ShellNameSpace Control. Walking into that host crashes explorer.exe.
static int woxFindDefViewWithin(HWND parent, int depth)
{
    if (!parent || depth > WOX_DEF_VIEW_MAX_DEPTH)
    {
        return 0;
    }

    HWND child = NULL;
    while ((child = FindWindowExW(parent, child, NULL, NULL)) != NULL)
    {
        WCHAR childClass[256];
        if (GetClassNameW(child, childClass, 256) == 0)
        {
            continue;
        }
        if (_wcsicmp(childClass, WOX_BROWSE_FOLDER_NAMESPACE_CLASS) == 0)
        {
            continue;
        }
        if (_wcsicmp(childClass, L"SHELLDLL_DefView") == 0)
        {
            return 1;
        }
        if (woxFindDefViewWithin(child, depth + 1))
        {
            return 1;
        }
    }
    return 0;
}

static int woxHasDefViewShallow(HWND hwnd)
{
    return woxFindDefViewWithin(hwnd, 1);
}

// woxIsOpenSaveDialogWindow is true for common item Open/Save dialogs and for
// SHBrowseForFolder choosers that Quick Switch should also navigate.
static int woxIsOpenSaveDialogWindow(HWND hwnd)
{
    if (!woxChildHasClass(hwnd, L"#32770"))
    {
        return 0;
    }
    // Title/button chrome is enough for Move Items and must run before any
    // child walk. Walking ShellNameSpace crashes explorer.exe.
    if (woxDialogLooksLikeSelectFolder(hwnd))
    {
        return 1;
    }
    if (FindWindowExW(hwnd, NULL, WOX_BROWSE_FOLDER_NAMESPACE_CLASS, NULL))
    {
        return 1;
    }
    return woxHasDefViewShallow(hwnd);
}

#endif
