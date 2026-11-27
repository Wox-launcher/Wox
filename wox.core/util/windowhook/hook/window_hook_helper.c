#define _WIN32_WINNT 0x0600

#include <windows.h>
#include <shellapi.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>

// Mirror WoxWindowHookDiagnostic from window_hook.c.
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

typedef void *(WINAPI *AttachStickyFn)(HWND target, DWORD expectedPid, HWND overlay, WoxWindowHookDiagnostic *diagnostic);
typedef BOOL(WINAPI *DetachStickyFn)(void *handle);
typedef BOOL(WINAPI *DialogCommandFn)(HWND dialog, DWORD expectedPid, const WCHAR *targetPath, WoxWindowHookDiagnostic *diagnostic);

static void refuse(int code, const char *reason)
{
    printf("fail reason=%s\n", reason);
    fflush(stdout);
    ExitProcess((UINT)code);
}

static bool parseUint64(const WCHAR *s, uint64_t *out)
{
    if (s == NULL || *s == L'\0')
    {
        return false;
    }
    WCHAR *end = NULL;
    unsigned long long value = wcstoull(s, &end, 10);
    if (end == s || *end != L'\0')
    {
        return false;
    }
    *out = (uint64_t)value;
    return true;
}

static const WCHAR *matchFlag(const WCHAR *arg, const WCHAR *name, const WCHAR **inlineValue)
{
    size_t offset = 1;
    if (arg[0] != L'-')
    {
        return NULL;
    }
    if (arg[1] == L'-')
    {
        offset = 2;
    }
    size_t nameLen = wcslen(name);
    if (wcsncmp(arg + offset, name, nameLen) != 0)
    {
        return NULL;
    }
    const WCHAR *rest = arg + offset + nameLen;
    if (*rest == L'=')
    {
        *inlineValue = rest + 1;
        return name;
    }
    if (*rest == L'\0')
    {
        *inlineValue = NULL;
        return name;
    }
    return NULL;
}

static const WCHAR *nextFlagValue(int argc, LPWSTR *argv, int *index, const WCHAR *inlineValue)
{
    if (inlineValue != NULL)
    {
        return inlineValue;
    }
    if (*index + 1 >= argc)
    {
        return NULL;
    }
    *index += 1;
    return argv[*index];
}

static void printDiagnosticFailure(const char *kind, const WoxWindowHookDiagnostic *diag, DWORD callErr)
{
    printf(
        "fail reason=%s stage=%lu win32Error=%lu hresult=0x%08lX targetPid=%lu targetThread=%lu shellView=%lu hookInstalled=%lu callbackEntered=%lu waitResult=%lu callErr=%lu\n",
        kind,
        (unsigned long)diag->stage,
        (unsigned long)diag->win32Error,
        (unsigned long)(uint32_t)diag->hresult,
        (unsigned long)diag->targetPid,
        (unsigned long)diag->targetThread,
        (unsigned long)diag->shellViewFound,
        (unsigned long)diag->hookInstalled,
        (unsigned long)diag->callbackEntered,
        (unsigned long)diag->waitResult,
        (unsigned long)callErr);
    fflush(stdout);
}

static int holdSticky(HMODULE dll, uintptr_t target, DWORD pid, uintptr_t overlay)
{
    if (overlay == 0)
    {
        refuse(2, "invalid-arguments");
    }

    AttachStickyFn attach = (AttachStickyFn)(void *)GetProcAddress(dll, "WoxWindowHookAttachSticky");
    if (attach == NULL)
    {
        DWORD err = GetLastError();
        printf("fail reason=missing-attach err=%lu\n", (unsigned long)err);
        fflush(stdout);
        return 4;
    }

    DetachStickyFn detach = (DetachStickyFn)(void *)GetProcAddress(dll, "WoxWindowHookDetachSticky");
    if (detach == NULL)
    {
        DWORD err = GetLastError();
        printf("fail reason=missing-detach err=%lu\n", (unsigned long)err);
        fflush(stdout);
        return 4;
    }

    WoxWindowHookDiagnostic diag;
    memset(&diag, 0, sizeof(diag));
    SetLastError(0);
    void *handle = attach((HWND)target, pid, (HWND)overlay, &diag);
    DWORD callErr = GetLastError();
    if (handle == NULL)
    {
        printDiagnosticFailure("attach", &diag, callErr);
        return 5;
    }

    printf("ok\n");
    fflush(stdout);

    // Block on stdin until Wox closes the pipe or crashes so the hook never leaks.
    HANDLE stdinHandle = GetStdHandle(STD_INPUT_HANDLE);
    char drain[1024];
    DWORD bytesRead = 0;
    while (ReadFile(stdinHandle, drain, sizeof(drain), &bytesRead, NULL) && bytesRead > 0)
    {
    }

    detach(handle);
    return 0;
}

static int runDialogCommand(HMODULE dll, const char *exportName, uintptr_t target, DWORD pid)
{
    const size_t maxBytes = 256 * 1024;
    HANDLE stdinHandle = GetStdHandle(STD_INPUT_HANDLE);
    size_t cap = 4096;
    size_t len = 0;
    char *raw = (char *)malloc(cap);
    if (raw == NULL)
    {
        refuse(2, "read-path err=oom");
    }

    for (;;)
    {
        if (len == cap)
        {
            if (cap >= maxBytes)
            {
                free(raw);
                refuse(2, "read-path err=too-large");
            }
            cap *= 2;
            char *next = (char *)realloc(raw, cap);
            if (next == NULL)
            {
                free(raw);
                refuse(2, "read-path err=oom");
            }
            raw = next;
        }
        DWORD bytesRead = 0;
        BOOL ok = ReadFile(stdinHandle, raw + len, (DWORD)(cap - len), &bytesRead, NULL);
        if (!ok)
        {
            DWORD err = GetLastError();
            if (err == ERROR_BROKEN_PIPE)
            {
                break;
            }
            free(raw);
            printf("fail reason=read-path err=%lu\n", (unsigned long)err);
            fflush(stdout);
            return 2;
        }
        if (bytesRead == 0)
        {
            break;
        }
        len += bytesRead;
    }

    while (len > 0 && (raw[len - 1] == '\r' || raw[len - 1] == '\n'))
    {
        len--;
    }
    if (len == 0)
    {
        free(raw);
        refuse(2, "empty-path");
    }
    if (memchr(raw, 0, len) != NULL)
    {
        free(raw);
        refuse(2, "bad-path");
    }

    int wideCount = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, raw, (int)len, NULL, 0);
    if (wideCount <= 0)
    {
        DWORD err = GetLastError();
        free(raw);
        printf("fail reason=bad-path err=%lu\n", (unsigned long)err);
        fflush(stdout);
        return 2;
    }

    WCHAR *widePath = (WCHAR *)malloc((size_t)(wideCount + 1) * sizeof(WCHAR));
    if (widePath == NULL)
    {
        free(raw);
        refuse(2, "bad-path err=oom");
    }
    if (MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, raw, (int)len, widePath, wideCount) <= 0)
    {
        DWORD err = GetLastError();
        free(widePath);
        free(raw);
        printf("fail reason=bad-path err=%lu\n", (unsigned long)err);
        fflush(stdout);
        return 2;
    }
    widePath[wideCount] = L'\0';
    free(raw);

    DialogCommandFn proc = (DialogCommandFn)(void *)GetProcAddress(dll, exportName);
    if (proc == NULL)
    {
        DWORD err = GetLastError();
        free(widePath);
        printf("fail reason=missing-export export=%s err=%lu\n", exportName, (unsigned long)err);
        fflush(stdout);
        return 4;
    }

    WoxWindowHookDiagnostic diag;
    memset(&diag, 0, sizeof(diag));
    SetLastError(0);
    BOOL result = proc((HWND)target, pid, widePath, &diag);
    DWORD callErr = GetLastError();
    free(widePath);

    if (!result)
    {
        printDiagnosticFailure("command", &diag, callErr);
        return 5;
    }

    printf("ok\n");
    fflush(stdout);
    return 0;
}

int main(void)
{
    setvbuf(stdout, NULL, _IONBF, 0);
    SetErrorMode(SEM_FAILCRITICALERRORS | SEM_NOOPENFILEERRORBOX);

    int argc = 0;
    LPWSTR *argv = CommandLineToArgvW(GetCommandLineW(), &argc);
    if (argv == NULL)
    {
        refuse(2, "invalid-arguments");
    }

    const WCHAR *dllPath = NULL;
    const WCHAR *command = L"sticky";
    uint64_t targetVal = 0;
    uint64_t pidVal = 0;
    uint64_t overlayVal = 0;

    for (int i = 1; i < argc; i++)
    {
        const WCHAR *inlineValue = NULL;
        if (matchFlag(argv[i], L"dll", &inlineValue) != NULL)
        {
            dllPath = nextFlagValue(argc, argv, &i, inlineValue);
            if (dllPath == NULL)
            {
                LocalFree(argv);
                refuse(2, "invalid-arguments");
            }
        }
        else if (matchFlag(argv[i], L"command", &inlineValue) != NULL)
        {
            command = nextFlagValue(argc, argv, &i, inlineValue);
            if (command == NULL)
            {
                LocalFree(argv);
                refuse(2, "invalid-arguments");
            }
        }
        else if (matchFlag(argv[i], L"target", &inlineValue) != NULL)
        {
            const WCHAR *v = nextFlagValue(argc, argv, &i, inlineValue);
            if (!parseUint64(v, &targetVal))
            {
                LocalFree(argv);
                refuse(2, "invalid-arguments");
            }
        }
        else if (matchFlag(argv[i], L"pid", &inlineValue) != NULL)
        {
            const WCHAR *v = nextFlagValue(argc, argv, &i, inlineValue);
            if (!parseUint64(v, &pidVal))
            {
                LocalFree(argv);
                refuse(2, "invalid-arguments");
            }
        }
        else if (matchFlag(argv[i], L"overlay", &inlineValue) != NULL)
        {
            const WCHAR *v = nextFlagValue(argc, argv, &i, inlineValue);
            if (!parseUint64(v, &overlayVal))
            {
                LocalFree(argv);
                refuse(2, "invalid-arguments");
            }
        }
    }

    if (dllPath == NULL || *dllPath == L'\0' || targetVal == 0 || pidVal == 0 || pidVal > 0xFFFFFFFFULL)
    {
        LocalFree(argv);
        refuse(2, "invalid-arguments");
    }

    HMODULE dll = LoadLibraryW(dllPath);
    if (dll == NULL)
    {
        DWORD err = GetLastError();
        LocalFree(argv);
        printf("fail reason=load-dll err=%lu\n", (unsigned long)err);
        fflush(stdout);
        return 3;
    }

    int exitCode = 0;
    if (wcscmp(command, L"sticky") == 0)
    {
        exitCode = holdSticky(dll, (uintptr_t)targetVal, (DWORD)pidVal, (uintptr_t)overlayVal);
    }
    else if (wcscmp(command, L"navigate") == 0)
    {
        exitCode = runDialogCommand(dll, "WoxWindowHookNavigateDialog", (uintptr_t)targetVal, (DWORD)pidVal);
    }
    else if (wcscmp(command, L"select") == 0)
    {
        exitCode = runDialogCommand(dll, "WoxWindowHookSelectDialogItem", (uintptr_t)targetVal, (DWORD)pidVal);
    }
    else
    {
        printf("fail reason=unknown-command command=\"%ls\"\n", command);
        fflush(stdout);
        exitCode = 2;
    }

    FreeLibrary(dll);
    LocalFree(argv);
    return exitCode;
}
