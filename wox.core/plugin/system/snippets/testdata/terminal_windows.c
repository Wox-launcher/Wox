#include <windows.h>
#include <stdio.h>

// The terminal owns a private console; report readiness and capture both input lines.
int wmain(int argc, wchar_t **argv) {
    if (argc != 3) return 1;
    FILE *ready = _wfopen(argv[1], L"w");
    if (!ready) return 2;
    fprintf(ready, "%lu", (unsigned long)GetCurrentProcessId());
    fclose(ready);
    WCHAR line[256];
    DWORD count = 0, second = 0;
    if (!ReadConsoleW(GetStdHandle(STD_INPUT_HANDLE), line, 256, &count, NULL) ||
        !ReadConsoleW(GetStdHandle(STD_INPUT_HANDLE), line + count, 256 - count, &second, NULL)) return 3;
    FILE *result = _wfopen(argv[2], L"wb");
    if (!result) return 4;
    size_t written = fwrite(line, sizeof(WCHAR), count + second, result);
    fclose(result);
    return written == count + second ? 0 : 5;
}
