#ifndef CLIPBOARD_WINDOWS_H
#define CLIPBOARD_WINDOWS_H

#include <windows.h>

// Content type constants
#define CLIPBOARD_TYPE_EMPTY 0
#define CLIPBOARD_TYPE_TEXT  1
#define CLIPBOARD_TYPE_IMAGE 2
#define CLIPBOARD_TYPE_FILE  3

// BitmapInfo is returned alongside DIB data so Go can decode the image.
typedef struct {
    int headerSize;
    int width;
    int height;
    int bitCount;
    int compression;
    int sizeImage;
    int clrUsed;
} BitmapInfo;

int clipboardGetContentType();
int clipboardReadText(wchar_t **outText, int *outLen);
int clipboardReadFilePaths(wchar_t **outPaths, int *outLen);
int clipboardReadImage(unsigned char **outData, int *outLen, int *outIsPNG, BitmapInfo *outInfo);
int clipboardWriteText(const wchar_t *text, int textLen);
int clipboardWriteFilePaths(const wchar_t *paths, int totalLen);
int clipboardWriteImage(const unsigned char *pngData, int pngLen,
                        const unsigned char *dibData, int dibLen);
DWORD clipboardGetSequenceNumber();
int clipboardGetDiagnosticInfo(char *buf, int bufLen);

#endif
