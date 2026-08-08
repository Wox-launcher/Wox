#define _WIN32_WINNT 0x0601

#include <windows.h>
#include <dbghelp.h>
#include <werapi.h>
#include <wchar.h>

#define WOX_CRASH_CONFIG_KEY L"Software\\Wox\\CrashHandler"
#define WOX_CRASH_DUMP_FOLDER_VALUE L"DumpFolder"
#define WOX_CRASH_PATH_CAPACITY 32768

static BOOL read_dump_folder(WCHAR *dump_folder, DWORD capacity) {
    HKEY key = NULL;
    DWORD value_type = 0;
    DWORD byte_count = capacity * sizeof(WCHAR);
    LONG status = RegOpenKeyExW(HKEY_CURRENT_USER, WOX_CRASH_CONFIG_KEY, 0, KEY_QUERY_VALUE, &key);
    if (status != ERROR_SUCCESS) {
        return FALSE;
    }

    status = RegQueryValueExW(
        key,
        WOX_CRASH_DUMP_FOLDER_VALUE,
        NULL,
        &value_type,
        (LPBYTE)dump_folder,
        &byte_count
    );
    RegCloseKey(key);
    if (status != ERROR_SUCCESS || (value_type != REG_SZ && value_type != REG_EXPAND_SZ)) {
        return FALSE;
    }

    dump_folder[capacity - 1] = L'\0';
    return dump_folder[0] != L'\0';
}

static BOOL build_dump_paths(
    const WER_RUNTIME_EXCEPTION_INFORMATION *exception_information,
    WCHAR *temporary_path,
    WCHAR *final_path,
    DWORD capacity
) {
    WCHAR dump_folder[WOX_CRASH_PATH_CAPACITY];
    SYSTEMTIME timestamp;
    DWORD process_id;
    int written;

    if (!read_dump_folder(dump_folder, WOX_CRASH_PATH_CAPACITY)) {
        return FALSE;
    }

    process_id = GetProcessId(exception_information->hProcess);
    GetSystemTime(&timestamp);
    written = swprintf(
        final_path,
        capacity,
        L"%ls\\wox-%lu-%04u%02u%02u-%02u%02u%02u-%03u.dmp",
        dump_folder,
        (unsigned long)process_id,
        timestamp.wYear,
        timestamp.wMonth,
        timestamp.wDay,
        timestamp.wHour,
        timestamp.wMinute,
        timestamp.wSecond,
        timestamp.wMilliseconds
    );
    if (written < 0 || (DWORD)written >= capacity) {
        return FALSE;
    }

    written = swprintf(temporary_path, capacity, L"%ls.tmp", final_path);
    return written >= 0 && (DWORD)written < capacity;
}

static BOOL write_minidump(const WER_RUNTIME_EXCEPTION_INFORMATION *exception_information) {
    WCHAR temporary_path[WOX_CRASH_PATH_CAPACITY];
    WCHAR final_path[WOX_CRASH_PATH_CAPACITY];
    EXCEPTION_POINTERS exception_pointers;
    MINIDUMP_EXCEPTION_INFORMATION minidump_exception;
    MINIDUMP_TYPE dump_type;
    HANDLE dump_file;
    BOOL written;

    if (!build_dump_paths(exception_information, temporary_path, final_path, WOX_CRASH_PATH_CAPACITY)) {
        return FALSE;
    }

    dump_file = CreateFileW(
        temporary_path,
        GENERIC_WRITE,
        0,
        NULL,
        CREATE_ALWAYS,
        FILE_ATTRIBUTE_NORMAL | FILE_FLAG_WRITE_THROUGH,
        NULL
    );
    if (dump_file == INVALID_HANDLE_VALUE) {
        return FALSE;
    }

    exception_pointers.ExceptionRecord = (PEXCEPTION_RECORD)&exception_information->exceptionRecord;
    exception_pointers.ContextRecord = (PCONTEXT)&exception_information->context;
    minidump_exception.ThreadId = GetThreadId(exception_information->hThread);
    minidump_exception.ExceptionPointers = &exception_pointers;
    minidump_exception.ClientPointers = FALSE;
    dump_type = (MINIDUMP_TYPE)(
        MiniDumpWithDataSegs |
        MiniDumpWithUnloadedModules |
        MiniDumpWithProcessThreadData |
        MiniDumpWithThreadInfo |
        MiniDumpWithFullMemoryInfo |
        MiniDumpWithIndirectlyReferencedMemory |
        MiniDumpIgnoreInaccessibleMemory
    );

    written = MiniDumpWriteDump(
        exception_information->hProcess,
        GetProcessId(exception_information->hProcess),
        dump_file,
        dump_type,
        &minidump_exception,
        NULL,
        NULL
    );
    if (!written) {
        SetFilePointer(dump_file, 0, NULL, FILE_BEGIN);
        SetEndOfFile(dump_file);
        written = MiniDumpWriteDump(
            exception_information->hProcess,
            GetProcessId(exception_information->hProcess),
            dump_file,
            MiniDumpNormal,
            &minidump_exception,
            NULL,
            NULL
        );
    }

    if (written) {
        written = FlushFileBuffers(dump_file);
    }
    CloseHandle(dump_file);
    if (!written) {
        DeleteFileW(temporary_path);
        return FALSE;
    }

    if (!MoveFileExW(
            temporary_path,
            final_path,
            MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH
        )) {
        DeleteFileW(temporary_path);
        return FALSE;
    }
    return TRUE;
}

__declspec(dllexport) HRESULT WINAPI OutOfProcessExceptionEventCallback(
    PVOID context,
    const PWER_RUNTIME_EXCEPTION_INFORMATION exception_information,
    WINBOOL *ownership_claimed,
    PWSTR event_name,
    PDWORD event_name_size,
    PDWORD signature_count
) {
    (void)context;
    (void)event_name;
    (void)event_name_size;

    if (ownership_claimed != NULL) {
        *ownership_claimed = FALSE;
    }
    if (signature_count != NULL) {
        *signature_count = 0;
    }
    if (exception_information != NULL && exception_information->bIsFatal) {
        write_minidump(exception_information);
    }

    // Wox only preserves a dump here. Returning ownership to WER keeps the
    // normal Windows crash-reporting flow intact.
    return S_OK;
}

__declspec(dllexport) HRESULT WINAPI OutOfProcessExceptionEventSignatureCallback(
    PVOID context,
    const PWER_RUNTIME_EXCEPTION_INFORMATION exception_information,
    DWORD index,
    PWSTR name,
    PDWORD name_size,
    PWSTR value,
    PDWORD value_size
) {
    (void)context;
    (void)exception_information;
    (void)index;
    (void)name;
    (void)name_size;
    (void)value;
    (void)value_size;
    return E_NOTIMPL;
}

__declspec(dllexport) HRESULT WINAPI OutOfProcessExceptionEventDebuggerLaunchCallback(
    PVOID context,
    const PWER_RUNTIME_EXCEPTION_INFORMATION exception_information,
    PBOOL is_custom_debugger,
    PWSTR debugger_launch,
    PDWORD debugger_launch_size,
    PBOOL is_debugger_autolaunch
) {
    (void)context;
    (void)exception_information;
    (void)debugger_launch;
    (void)debugger_launch_size;

    if (is_custom_debugger != NULL) {
        *is_custom_debugger = FALSE;
    }
    if (is_debugger_autolaunch != NULL) {
        *is_debugger_autolaunch = FALSE;
    }
    return S_OK;
}

BOOL WINAPI DllMain(HINSTANCE instance, DWORD reason, LPVOID reserved) {
    (void)instance;
    (void)reason;
    (void)reserved;
    return TRUE;
}
