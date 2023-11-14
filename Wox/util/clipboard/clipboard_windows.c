#include <windows.h>

char* GetClipboardText() {
    if (!OpenClipboard(NULL)) {
        return NULL;
    }

    HANDLE hData = GetClipboardData(CF_TEXT);
    if (hData == NULL) {
        CloseClipboard();
        return NULL;
    }

    char* data = (char*)GlobalLock(hData);
    if (data == NULL) {
        CloseClipboard();
        return NULL;
    }

    char* result = _strdup(data);

    GlobalUnlock(hData);
    CloseClipboard();

    return result;
}