#include "clipboard_windows.h"
#include <shlobj.h>
#include <shellapi.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>


// Registered PNG clipboard format (lazily initialized)
static UINT g_pngFormat = 0;

static UINT getPNGFormat() {
    if (g_pngFormat == 0) {
        g_pngFormat = RegisterClipboardFormatW(L"PNG");
    }
    return g_pngFormat;
}

// openClipboardRetry opens the clipboard with up to 5 retries.
// Returns TRUE on success, FALSE on failure.
static BOOL openClipboardRetry() {
    for (int i = 0; i < 5; i++) {
        if (OpenClipboard(NULL)) {
            return TRUE;
        }
        Sleep(10 + i * 10);
    }
    return FALSE;
}

// clipboardGetContentType checks what type of data is on the clipboard.
// Returns: 0=empty, 1=text, 2=image, 3=file
// Priority: file > image > text
int clipboardGetContentType() {
    if (IsClipboardFormatAvailable(CF_HDROP)) {
        return CLIPBOARD_TYPE_FILE;
    }

    UINT pngFmt = getPNGFormat();
    if (IsClipboardFormatAvailable(CF_DIB) ||
        IsClipboardFormatAvailable(CF_DIBV5) ||
        (pngFmt != 0 && IsClipboardFormatAvailable(pngFmt))) {
        return CLIPBOARD_TYPE_IMAGE;
    }

    if (IsClipboardFormatAvailable(CF_UNICODETEXT)) {
        return CLIPBOARD_TYPE_TEXT;
    }

    return CLIPBOARD_TYPE_EMPTY;
}

// clipboardReadText reads CF_UNICODETEXT from the clipboard.
// On success, *outText is a malloc'd UTF-16 buffer, *outLen is the number of wchar_t characters (not including null).
// Caller must free *outText. Returns 0 on success, negative on error.
int clipboardReadText(wchar_t **outText, int *outLen) {
    *outText = NULL;
    *outLen = 0;

    if (!IsClipboardFormatAvailable(CF_UNICODETEXT)) {
        return -1;  // no text data
    }

    if (!openClipboardRetry()) {
        return -2;  // failed to open
    }

    HANDLE hMem = GetClipboardData(CF_UNICODETEXT);
    if (hMem == NULL) {
        CloseClipboard();
        return -3;
    }

    SIZE_T sizeBytes = GlobalSize(hMem);
    if (sizeBytes == 0) {
        CloseClipboard();
        return -4;
    }

    wchar_t *pMem = (wchar_t *)GlobalLock(hMem);
    if (pMem == NULL) {
        CloseClipboard();
        return -5;
    }

    // Limit to 32 MB
    SIZE_T maxBytes = 32 * 1024 * 1024;
    if (sizeBytes > maxBytes) {
        sizeBytes = maxBytes;
    }

    int charCount = (int)(sizeBytes / sizeof(wchar_t));
    if (charCount <= 0) {
        GlobalUnlock(hMem);
        CloseClipboard();
        return -6;
    }

    // Find actual string length (null-terminated)
    int actualLen = 0;
    while (actualLen < charCount && pMem[actualLen] != 0) {
        actualLen++;
    }

    wchar_t *buf = (wchar_t *)malloc((actualLen + 1) * sizeof(wchar_t));
    if (buf == NULL) {
        GlobalUnlock(hMem);
        CloseClipboard();
        return -7;
    }
    memcpy(buf, pMem, actualLen * sizeof(wchar_t));
    buf[actualLen] = 0;

    GlobalUnlock(hMem);
    CloseClipboard();

    *outText = buf;
    *outLen = actualLen;
    return 0;
}

// clipboardReadFilePaths reads CF_HDROP from the clipboard.
// Returns file paths as a null-separated, double-null-terminated wide string buffer.
// *outLen is the total number of wchar_t (including separating nulls and the final double null).
// Caller must free *outPaths. Returns 0 on success, negative on error.
int clipboardReadFilePaths(wchar_t **outPaths, int *outLen) {
    *outPaths = NULL;
    *outLen = 0;

    if (!IsClipboardFormatAvailable(CF_HDROP)) {
        return -1;
    }

    if (!openClipboardRetry()) {
        return -2;
    }

    HANDLE hDrop = GetClipboardData(CF_HDROP);
    if (hDrop == NULL) {
        CloseClipboard();
        return -3;
    }

    UINT fileCount = DragQueryFileW((HDROP)hDrop, 0xFFFFFFFF, NULL, 0);
    if (fileCount == 0) {
        CloseClipboard();
        return -4;
    }

    // Calculate total buffer size needed
    int totalChars = 0;
    for (UINT i = 0; i < fileCount; i++) {
        UINT pathLen = DragQueryFileW((HDROP)hDrop, i, NULL, 0);
        totalChars += pathLen + 1;  // +1 for null separator
    }
    totalChars++;  // final null for double-null termination

    wchar_t *buf = (wchar_t *)malloc(totalChars * sizeof(wchar_t));
    if (buf == NULL) {
        CloseClipboard();
        return -7;
    }

    int pos = 0;
    for (UINT i = 0; i < fileCount; i++) {
        UINT pathLen = DragQueryFileW((HDROP)hDrop, i, NULL, 0);
        DragQueryFileW((HDROP)hDrop, i, buf + pos, pathLen + 1);
        pos += pathLen;
        buf[pos] = 0;  // null separator
        pos++;
    }
    buf[pos] = 0;  // final null

    CloseClipboard();

    *outPaths = buf;
    *outLen = totalChars;
    return 0;
}

// clipboardReadImage reads image data from the clipboard.
// Tries PNG format first, then CF_DIB.
// On success, *outData is a malloc'd buffer, *outLen is byte count, *outIsPNG indicates format.
// Caller must free *outData. Returns 0 on success, negative on error.
int clipboardReadImage(unsigned char **outData, int *outLen, int *outIsPNG, BitmapInfo *outInfo) {
    *outData = NULL;
    *outLen = 0;
    *outIsPNG = 0;
    memset(outInfo, 0, sizeof(BitmapInfo));

    UINT pngFmt = getPNGFormat();
    int hasPNG = (pngFmt != 0 && IsClipboardFormatAvailable(pngFmt));
    int hasDIB = IsClipboardFormatAvailable(CF_DIB);

    if (!hasPNG && !hasDIB) {
        return -1;  // no image data
    }

    if (!openClipboardRetry()) {
        return -2;
    }

    // Try PNG first (higher fidelity, supports transparency natively)
    if (hasPNG) {
        HANDLE hPng = GetClipboardData(pngFmt);
        if (hPng != NULL) {
            SIZE_T pngSize = GlobalSize(hPng);
            if (pngSize > 0 && pngSize <= 128 * 1024 * 1024) {
                void *pPng = GlobalLock(hPng);
                if (pPng != NULL) {
                    unsigned char *buf = (unsigned char *)malloc(pngSize);
                    if (buf != NULL) {
                        memcpy(buf, pPng, pngSize);
                        GlobalUnlock(hPng);
                        CloseClipboard();

                        *outData = buf;
                        *outLen = (int)pngSize;
                        *outIsPNG = 1;
                        return 0;
                    }
                    GlobalUnlock(hPng);
                }
            }
        }
    }

    // Fallback to CF_DIB
    if (hasDIB) {
        HANDLE hDib = GetClipboardData(CF_DIB);
        if (hDib == NULL) {
            CloseClipboard();
            return -3;
        }

        SIZE_T dibSize = GlobalSize(hDib);
        if (dibSize == 0 || dibSize > 128 * 1024 * 1024) {
            CloseClipboard();
            return -4;
        }

        void *pDib = GlobalLock(hDib);
        if (pDib == NULL) {
            CloseClipboard();
            return -5;
        }

        // Copy DIB header info for Go side
        if (dibSize >= sizeof(BITMAPINFOHEADER)) {
            BITMAPINFOHEADER *hdr = (BITMAPINFOHEADER *)pDib;
            outInfo->headerSize = (int)hdr->biSize;
            outInfo->width = (int)hdr->biWidth;
            outInfo->height = (int)hdr->biHeight;
            outInfo->bitCount = (int)hdr->biBitCount;
            outInfo->compression = (int)hdr->biCompression;
            outInfo->sizeImage = (int)hdr->biSizeImage;
            outInfo->clrUsed = (int)hdr->biClrUsed;
        }

        unsigned char *buf = (unsigned char *)malloc(dibSize);
        if (buf == NULL) {
            GlobalUnlock(hDib);
            CloseClipboard();
            return -7;
        }
        memcpy(buf, pDib, dibSize);

        GlobalUnlock(hDib);
        CloseClipboard();

        *outData = buf;
        *outLen = (int)dibSize;
        *outIsPNG = 0;
        return 0;
    }

    CloseClipboard();
    return -6;
}

// clipboardWriteText writes CF_UNICODETEXT to the clipboard.
// text must be a null-terminated wide string, textLen is the number of characters (not including null).
// Returns 0 on success, negative on error.
int clipboardWriteText(const wchar_t *text, int textLen) {
    if (!openClipboardRetry()) {
        return -1;
    }

    if (!EmptyClipboard()) {
        CloseClipboard();
        return -2;
    }

    if (textLen <= 0) {
        // Empty text: just clear clipboard
        CloseClipboard();
        return 0;
    }

    SIZE_T bufSize = (textLen + 1) * sizeof(wchar_t);
    HGLOBAL hMem = GlobalAlloc(GMEM_MOVEABLE, bufSize);
    if (hMem == NULL) {
        CloseClipboard();
        return -3;
    }

    wchar_t *pMem = (wchar_t *)GlobalLock(hMem);
    if (pMem == NULL) {
        GlobalFree(hMem);
        CloseClipboard();
        return -4;
    }
    memcpy(pMem, text, textLen * sizeof(wchar_t));
    pMem[textLen] = 0;
    GlobalUnlock(hMem);

    if (SetClipboardData(CF_UNICODETEXT, hMem) == NULL) {
        GlobalFree(hMem);
        CloseClipboard();
        return -5;
    }

    CloseClipboard();
    return 0;
}

// clipboardWriteFilePaths writes a CF_HDROP payload with a null-separated,
// double-null-terminated UTF-16 path buffer.
// Returns 0 on success, negative on error.
int clipboardWriteFilePaths(const wchar_t *paths, int totalLen) {
    if (paths == NULL || totalLen <= 1) {
        return -1;
    }

    if (!openClipboardRetry()) {
        return -2;
    }

    if (!EmptyClipboard()) {
        CloseClipboard();
        return -3;
    }

    SIZE_T dataSize = sizeof(DROPFILES) + ((SIZE_T)totalLen * sizeof(wchar_t));
    HGLOBAL hDrop = GlobalAlloc(GMEM_MOVEABLE | GMEM_ZEROINIT, dataSize);
    if (hDrop == NULL) {
        CloseClipboard();
        return -4;
    }

    DROPFILES *dropFiles = (DROPFILES *)GlobalLock(hDrop);
    if (dropFiles == NULL) {
        GlobalFree(hDrop);
        CloseClipboard();
        return -5;
    }

    dropFiles->pFiles = sizeof(DROPFILES);
    dropFiles->pt.x = 0;
    dropFiles->pt.y = 0;
    dropFiles->fNC = FALSE;
    dropFiles->fWide = TRUE;
    memcpy(((BYTE *)dropFiles) + sizeof(DROPFILES), paths, ((SIZE_T)totalLen * sizeof(wchar_t)));
    GlobalUnlock(hDrop);

    if (SetClipboardData(CF_HDROP, hDrop) == NULL) {
        GlobalFree(hDrop);
        CloseClipboard();
        return -6;
    }

    CloseClipboard();
    return 0;
}

// clipboardWriteImage writes image data to the clipboard in both PNG (registered format) and CF_DIB formats.
// Returns 0 on success, negative on error.
int clipboardWriteImage(const unsigned char *pngData, int pngLen,
                        const unsigned char *dibData, int dibLen) {
    if (dibLen <= 0) {
        return -1;  // DIB data is required
    }

    if (!openClipboardRetry()) {
        return -2;
    }

    if (!EmptyClipboard()) {
        CloseClipboard();
        return -3;
    }

    // Write PNG format first (for apps that support transparency)
    UINT pngFmt = getPNGFormat();
    if (pngFmt != 0 && pngLen > 0) {
        HGLOBAL hPng = GlobalAlloc(GMEM_MOVEABLE, pngLen);
        if (hPng != NULL) {
            void *pPng = GlobalLock(hPng);
            if (pPng != NULL) {
                memcpy(pPng, pngData, pngLen);
                GlobalUnlock(hPng);
                if (SetClipboardData(pngFmt, hPng) == NULL) {
                    GlobalFree(hPng);
                    // non-fatal: continue with DIB
                }
            } else {
                GlobalFree(hPng);
            }
        }
    }

    // Write CF_DIB for compatibility
    HGLOBAL hDib = GlobalAlloc(GMEM_MOVEABLE, dibLen);
    if (hDib == NULL) {
        CloseClipboard();
        return -4;
    }

    void *pDib = GlobalLock(hDib);
    if (pDib == NULL) {
        GlobalFree(hDib);
        CloseClipboard();
        return -5;
    }
    memcpy(pDib, dibData, dibLen);
    GlobalUnlock(hDib);

    if (SetClipboardData(CF_DIB, hDib) == NULL) {
        GlobalFree(hDib);
        CloseClipboard();
        return -6;
    }

    CloseClipboard();
    return 0;
}

// clipboardGetSequenceNumber returns the current clipboard sequence number.
DWORD clipboardGetSequenceNumber() {
    return GetClipboardSequenceNumber();
}

// clipboardGetDiagnosticInfo builds a diagnostic string with format availability and owner info.
// Writes to buf (max bufLen). Returns the number of characters written (excluding null).
int clipboardGetDiagnosticInfo(char *buf, int bufLen) {
    if (bufLen <= 0) return 0;

    UINT pngFmt = getPNGFormat();
    int hasPNG = (pngFmt != 0 && IsClipboardFormatAvailable(pngFmt));

    // Get clipboard owner info
    HWND hOwner = GetClipboardOwner();
    DWORD ownerPid = 0;
    char ownerClass[128] = {0};
    char ownerTitle[256] = {0};
    if (hOwner != NULL) {
        GetWindowThreadProcessId(hOwner, &ownerPid);
        GetClassNameA(hOwner, ownerClass, sizeof(ownerClass));
        GetWindowTextA(hOwner, ownerTitle, sizeof(ownerTitle));
    }

    // Get open clipboard window info
    HWND hOpen = GetOpenClipboardWindow();
    DWORD openPid = 0;
    char openClass[128] = {0};
    char openTitle[256] = {0};
    if (hOpen != NULL) {
        GetWindowThreadProcessId(hOpen, &openPid);
        GetClassNameA(hOpen, openClass, sizeof(openClass));
        GetWindowTextA(hOpen, openTitle, sizeof(openTitle));
    }

    int n = _snprintf_s(buf, bufLen, _TRUNCATE,
        "snapshot{seq=%u formats{text=%d dib=%d dibv5=%d hdrop=%d bitmap=%d png=%d png_id=%u} "
        "data_owner{hwnd=0x%p pid=%u class=\"%s\" title=\"%s\"} "
        "open_owner{hwnd=0x%p pid=%u class=\"%s\" title=\"%s\"}}",
        GetClipboardSequenceNumber(),
        IsClipboardFormatAvailable(CF_UNICODETEXT) ? 1 : 0,
        IsClipboardFormatAvailable(CF_DIB) ? 1 : 0,
        IsClipboardFormatAvailable(CF_DIBV5) ? 1 : 0,
        IsClipboardFormatAvailable(CF_HDROP) ? 1 : 0,
        IsClipboardFormatAvailable(CF_BITMAP) ? 1 : 0,
        hasPNG ? 1 : 0,
        pngFmt,
        (void *)hOwner, ownerPid, ownerClass, ownerTitle,
        (void *)hOpen, openPid, openClass, openTitle);

    return (n >= 0) ? n : 0;
}
