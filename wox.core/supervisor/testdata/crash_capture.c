// Exercise the production WER callback independently of Windows WER policy.
#include "../native/wox_crash_handler.c"

int main(void) {
    WER_RUNTIME_EXCEPTION_INFORMATION information = {0};
    BOOL claimed = FALSE;
    DWORD signatures = 0;
    information.dwSize = sizeof(information);
    information.hProcess = GetCurrentProcess();
    information.hThread = GetCurrentThread();
    information.bIsFatal = TRUE;
    information.exceptionRecord.ExceptionCode = EXCEPTION_ACCESS_VIOLATION;
    RtlCaptureContext(&information.context);
    return (int)OutOfProcessExceptionEventCallback(NULL, &information, &claimed, NULL, NULL, &signatures);
}
