#include <windows.h>
#include <shlobj.h>
#include <stdint.h>
#include <string.h>

// A private format identifies the staged value even if another copy contains the same text.
static UINT snippetClipboardMarkerFormat(void) {
    return RegisterClipboardFormatW(L"Wox.Snippets.StagedClipboard.v1");
}

static HWND snippetClipboardOpen(void) {
    HWND owner = CreateWindowExW(0, L"STATIC", L"", 0, 0, 0, 0, 0, HWND_MESSAGE, NULL, NULL, NULL);
    if (!owner) return NULL;
    for (int i = 0; i < 10; i++) {
        if (OpenClipboard(owner)) return owner;
        Sleep(10);
    }
    DestroyWindow(owner);
    return NULL;
}

static void snippetClipboardClose(HWND owner) {
    CloseClipboard();
    DestroyWindow(owner);
}

static HGLOBAL snippetClipboardText(const uint16_t *text, unsigned int length) {
    HGLOBAL data = GlobalAlloc(GMEM_MOVEABLE, ((SIZE_T)length + 1) * sizeof(uint16_t));
    if (!data) return NULL;
    uint16_t *dst = GlobalLock(data);
    if (!dst) { GlobalFree(data); return NULL; }
    if (length) memcpy(dst, text, (SIZE_T)length * sizeof(uint16_t));
    dst[length] = 0;
    GlobalUnlock(data);
    return data;
}

// The source sequence is checked while the clipboard is open before it is replaced.
int snippetClipboardStageText(DWORD expected, const uint16_t *text, unsigned int length, const uint8_t *marker) {
    UINT format = snippetClipboardMarkerFormat();
    HGLOBAL payload = snippetClipboardText(text, length);
    HGLOBAL tag = GlobalAlloc(GMEM_MOVEABLE, 16);
    if (!format || !payload || !tag) { if (payload) GlobalFree(payload); if (tag) GlobalFree(tag); return -1; }
    void *tagData = GlobalLock(tag);
    if (!tagData) { GlobalFree(payload); GlobalFree(tag); return -1; }
    memcpy(tagData, marker, 16);
    GlobalUnlock(tag);
    HWND owner = snippetClipboardOpen();
    if (!owner) { GlobalFree(payload); GlobalFree(tag); return -1; }
    int result = 0;
    if (GetClipboardSequenceNumber() == expected) {
        result = -1;
        if (EmptyClipboard() && SetClipboardData(CF_UNICODETEXT, payload)) {
            payload = NULL;
            if (SetClipboardData(format, tag)) { tag = NULL; result = 1; }
        }
    }
    snippetClipboardClose(owner);
    if (payload) GlobalFree(payload);
    if (tag) GlobalFree(tag);
    return result;
}

static int snippetClipboardHasMarker(UINT format, const uint8_t *marker) {
    HGLOBAL tag = GetClipboardData(format);
    if (!tag || GlobalSize(tag) < 16) return 0;
    const void *data = GlobalLock(tag);
    if (!data) return 0;
    int matches = memcmp(data, marker, 16) == 0;
    GlobalUnlock(tag);
    return matches;
}

// Confirm the staged text before removing the trigger; another copy may have replaced it.
int snippetClipboardStillStaged(const uint8_t *marker, const uint16_t *text, unsigned int length) {
    UINT format = snippetClipboardMarkerFormat();
    HWND owner = format ? snippetClipboardOpen() : NULL;
    if (!owner) return 0;
    int matches = 0;
    if (snippetClipboardHasMarker(format, marker)) {
        HGLOBAL payload = GetClipboardData(CF_UNICODETEXT);
        if (payload && GlobalSize(payload) >= ((SIZE_T)length + 1) * sizeof(uint16_t)) {
            const uint16_t *value = GlobalLock(payload);
            if (value) {
                matches = memcmp(value, text, (SIZE_T)length * sizeof(uint16_t)) == 0 && value[length] == 0;
                GlobalUnlock(payload);
            }
        }
    }
    snippetClipboardClose(owner);
    return matches;
}

// Distinguish a truly empty clipboard from data that Wox cannot decode.
int snippetClipboardIsEmpty(DWORD expected) {
    HWND owner = snippetClipboardOpen();
    if (!owner) return -1;
    SetLastError(ERROR_SUCCESS);
    int count = CountClipboardFormats();
    int empty = count == 0 && GetLastError() != ERROR_SUCCESS ? -1 :
                (GetClipboardSequenceNumber() == expected && count == 0);
    snippetClipboardClose(owner);
    return empty;
}

int snippetClipboardRestoreEmpty(const uint8_t *marker) {
    UINT format = snippetClipboardMarkerFormat();
    if (!format) return -1;
    HWND owner = snippetClipboardOpen();
    if (!owner) return -1;
    int result = 0;
    if (snippetClipboardHasMarker(format, marker)) result = EmptyClipboard() ? 1 : -1;
    snippetClipboardClose(owner);
    return result;
}

// Restoration checks the marker and publishes a prepared payload under one clipboard lock.
static int snippetClipboardRestoreHandle(const uint8_t *marker, UINT payloadFormat, HGLOBAL payload) {
    UINT format = snippetClipboardMarkerFormat();
    if (!format || !payload) { if (payload) GlobalFree(payload); return -1; }
    HWND owner = snippetClipboardOpen();
    if (!owner) { GlobalFree(payload); return -1; }
    int result = 0;
    if (snippetClipboardHasMarker(format, marker)) {
        result = -1;
        if (EmptyClipboard() && SetClipboardData(payloadFormat, payload)) { payload = NULL; result = 1; }
    }
    snippetClipboardClose(owner);
    if (payload) GlobalFree(payload);
    return result;
}

int snippetClipboardRestoreText(const uint8_t *marker, const uint16_t *text, unsigned int length) {
    return snippetClipboardRestoreHandle(marker, CF_UNICODETEXT, snippetClipboardText(text, length));
}

// File paths use the standard double-null-terminated CF_HDROP encoding.
int snippetClipboardRestoreFiles(const uint8_t *marker, const uint16_t *paths, unsigned int length) {
    if (!paths || length < 2) return -1;
    SIZE_T bytes = sizeof(DROPFILES) + (SIZE_T)length * sizeof(uint16_t);
    HGLOBAL payload = GlobalAlloc(GMEM_MOVEABLE | GMEM_ZEROINIT, bytes);
    if (!payload) return -1;
    DROPFILES *drop = GlobalLock(payload);
    if (!drop) { GlobalFree(payload); return -1; }
    drop->pFiles = sizeof(DROPFILES);
    drop->fWide = TRUE;
    memcpy((unsigned char *)drop + sizeof(DROPFILES), paths, (SIZE_T)length * sizeof(uint16_t));
    GlobalUnlock(payload);
    return snippetClipboardRestoreHandle(marker, CF_HDROP, payload);
}

static HGLOBAL snippetClipboardBytes(const uint8_t *bytes, unsigned int length) {
    if (!bytes || !length) return NULL;
    HGLOBAL payload = GlobalAlloc(GMEM_MOVEABLE, length);
    if (!payload) return NULL;
    void *dst = GlobalLock(payload);
    if (!dst) { GlobalFree(payload); return NULL; }
    memcpy(dst, bytes, length);
    GlobalUnlock(payload);
    return payload;
}

// Publish PNG and DIB together after checking that the staged marker still owns the clipboard.
int snippetClipboardRestoreImage(const uint8_t *marker, const uint8_t *png, unsigned int pngLength,
                                 const uint8_t *dib, unsigned int dibLength) {
    UINT markerFormat = snippetClipboardMarkerFormat();
    UINT pngFormat = RegisterClipboardFormatW(L"PNG");
    HGLOBAL pngData = snippetClipboardBytes(png, pngLength);
    HGLOBAL dibData = snippetClipboardBytes(dib, dibLength);
    if (!markerFormat || !pngFormat || !pngData || !dibData) {
        if (pngData) GlobalFree(pngData);
        if (dibData) GlobalFree(dibData);
        return -1;
    }
    HWND owner = snippetClipboardOpen();
    if (!owner) { GlobalFree(pngData); GlobalFree(dibData); return -1; }
    int result = 0;
    if (snippetClipboardHasMarker(markerFormat, marker)) {
        result = -1;
        if (EmptyClipboard() && SetClipboardData(pngFormat, pngData)) {
            pngData = NULL;
            if (SetClipboardData(CF_DIB, dibData)) { dibData = NULL; result = 1; }
        }
    }
    snippetClipboardClose(owner);
    if (pngData) GlobalFree(pngData);
    if (dibData) GlobalFree(dibData);
    return result;
}
