#include <windows.h>
#include <shellapi.h>
#include <stdint.h>
#include <string.h>

int snippetClipboardStageText(DWORD expected, const uint16_t *text, unsigned int length, const uint8_t *marker);
int snippetClipboardStillStaged(const uint8_t *marker, const uint16_t *text, unsigned int length);
int snippetClipboardIsEmpty(DWORD expected);
int snippetClipboardRestoreEmpty(const uint8_t *marker);
int snippetClipboardRestoreText(const uint8_t *marker, const uint16_t *text, unsigned int length);
int snippetClipboardRestoreFiles(const uint8_t *marker, const uint16_t *paths, unsigned int length);
int snippetClipboardRestoreImage(const uint8_t *marker, const uint8_t *png, unsigned int pngLength, const uint8_t *dib, unsigned int dibLength);

static int copyText(const wchar_t *text) {
    HWND owner = CreateWindowExW(0, L"STATIC", L"", 0, 0, 0, 0, 0, HWND_MESSAGE, NULL, NULL, NULL);
    if (!owner || !OpenClipboard(owner)) return 0;
    size_t bytes = (wcslen(text) + 1) * sizeof(wchar_t);
    HGLOBAL data = GlobalAlloc(GMEM_MOVEABLE, bytes);
    if (!data) return 0;
    void *dst = GlobalLock(data);
    memcpy(dst, text, bytes);
    GlobalUnlock(data);
    int ok = EmptyClipboard() && SetClipboardData(CF_UNICODETEXT, data);
    CloseClipboard();
    DestroyWindow(owner);
    if (!ok) GlobalFree(data);
    return ok;
}

static int textIs(const wchar_t *expected) {
    if (!OpenClipboard(NULL)) return 0;
    HGLOBAL data = GetClipboardData(CF_UNICODETEXT);
    const wchar_t *text = data ? GlobalLock(data) : NULL;
    int matches = text && wcscmp(text, expected) == 0;
    if (text) GlobalUnlock(data);
    CloseClipboard();
    return matches;
}

static int clearClipboard(void) {
    HWND owner = CreateWindowExW(0, L"STATIC", L"", 0, 0, 0, 0, 0, HWND_MESSAGE, NULL, NULL, NULL);
    if (!owner || !OpenClipboard(owner)) return 0;
    int ok = EmptyClipboard();
    CloseClipboard();
    DestroyWindow(owner);
    return ok;
}

int main(void) {
    HWINSTA station = CreateWindowStationW(NULL, 0, WINSTA_ALL_ACCESS, NULL);
    if (!station || !SetProcessWindowStation(station)) return 1;
    HDESK desktop = CreateDesktopW(L"wox-clipboard-test", NULL, NULL, 0, GENERIC_ALL, NULL);
    if (!desktop || !SetThreadDesktop(desktop)) return 2;
    const uint8_t marker[16] = {1, 2, 3, 4};
    if (!copyText(L"original")) return 3;
    DWORD initial = GetClipboardSequenceNumber();
    if (snippetClipboardStageText(initial, (const uint16_t *)L"staged", 6, marker) != 1 || !textIs(L"staged")) return 4;
    if (!snippetClipboardStillStaged(marker, (const uint16_t *)L"staged", 6) || snippetClipboardStillStaged(marker, (const uint16_t *)L"other", 5)) return 29;
    if (!copyText(L"new user copy")) return 5;
    if (snippetClipboardStillStaged(marker, (const uint16_t *)L"staged", 6)) return 30;
    if (snippetClipboardRestoreText(marker, (const uint16_t *)L"original", 8) != 0 || !textIs(L"new user copy")) return 6;
    if (snippetClipboardStageText(initial, (const uint16_t *)L"staged", 6, marker) != 0 || !textIs(L"new user copy")) return 7;
    DWORD current = GetClipboardSequenceNumber();
    if (snippetClipboardStageText(current, (const uint16_t *)L"staged", 6, marker) != 1) return 8;
    if (snippetClipboardRestoreText(marker, (const uint16_t *)L"new user copy", 13) != 1 || !textIs(L"new user copy")) return 9;
    const wchar_t paths[] = L"C:\\first.txt\0C:\\second.txt\0";
    current = GetClipboardSequenceNumber();
    if (snippetClipboardStageText(current, (const uint16_t *)L"staged", 6, marker) != 1) return 10;
    if (snippetClipboardRestoreFiles(marker, (const uint16_t *)paths, sizeof(paths) / sizeof(paths[0])) != 1) return 11;
    if (!OpenClipboard(NULL)) return 12;
    HDROP files = (HDROP)GetClipboardData(CF_HDROP);
    wchar_t path[64] = {0};
    int ok = files && DragQueryFileW(files, 0xffffffff, NULL, 0) == 2 &&
             DragQueryFileW(files, 1, path, 64) && wcscmp(path, L"C:\\second.txt") == 0;
    CloseClipboard();
    if (!ok) return 13;
    current = GetClipboardSequenceNumber();
    if (snippetClipboardStageText(current, (const uint16_t *)L"staged", 6, marker) != 1 || !copyText(L"newest")) return 14;
    if (snippetClipboardRestoreFiles(marker, (const uint16_t *)paths, sizeof(paths) / sizeof(paths[0])) != 0 || !textIs(L"newest")) return 15;
    const uint8_t png[] = {0x89, 'P', 'N', 'G'};
    struct { BITMAPINFOHEADER header; uint8_t pixel[4]; } dib = {0};
    dib.header.biSize = sizeof(BITMAPINFOHEADER);
    dib.header.biWidth = dib.header.biHeight = 1;
    dib.header.biPlanes = 1;
    dib.header.biBitCount = 32;
    dib.pixel[0] = 123;
    current = GetClipboardSequenceNumber();
    if (snippetClipboardStageText(current, (const uint16_t *)L"staged", 6, marker) != 1) return 16;
    if (snippetClipboardRestoreImage(marker, png, sizeof(png), (const uint8_t *)&dib, sizeof(dib)) != 1) return 17;
    if (!OpenClipboard(NULL)) return 18;
    HGLOBAL pngHandle = GetClipboardData(RegisterClipboardFormatW(L"PNG"));
    HGLOBAL dibHandle = GetClipboardData(CF_DIB);
    const uint8_t *pngValue = pngHandle ? GlobalLock(pngHandle) : NULL;
    const uint8_t *dibValue = dibHandle ? GlobalLock(dibHandle) : NULL;
    ok = pngValue && dibValue && memcmp(pngValue, png, sizeof(png)) == 0 &&
         dibValue[sizeof(BITMAPINFOHEADER)] == 123;
    if (pngValue) GlobalUnlock(pngHandle);
    if (dibValue) GlobalUnlock(dibHandle);
    CloseClipboard();
    if (!ok) return 19;
    current = GetClipboardSequenceNumber();
    if (snippetClipboardStageText(current, (const uint16_t *)L"staged", 6, marker) != 1 || !copyText(L"new image copy")) return 20;
    if (snippetClipboardRestoreImage(marker, png, sizeof(png), (const uint8_t *)&dib, sizeof(dib)) != 0 || !textIs(L"new image copy")) return 21;
    if (!clearClipboard()) return 22;
    current = GetClipboardSequenceNumber();
    if (snippetClipboardIsEmpty(current) != 1) return 23;
    if (snippetClipboardStageText(current, (const uint16_t *)L"staged", 6, marker) != 1) return 24;
    if (snippetClipboardRestoreEmpty(marker) != 1 || snippetClipboardIsEmpty(GetClipboardSequenceNumber()) != 1) return 25;
    current = GetClipboardSequenceNumber();
    if (snippetClipboardStageText(current, (const uint16_t *)L"staged", 6, marker) != 1 || !copyText(L"new copy")) return 26;
    if (snippetClipboardRestoreEmpty(marker) != 0 || !textIs(L"new copy")) return 27;
    if (snippetClipboardIsEmpty(GetClipboardSequenceNumber()) != 0) return 28;
    return 0;
}
