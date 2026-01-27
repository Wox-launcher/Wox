#include <windows.h>

extern void finderActivatedCallbackCGO(int pid);

void startFinderMonitor() {
    // Stub: In real impl, use SetWinEventHook and listen for EVENT_SYSTEM_FOREGROUND.
    // Check if new foreground window process name is explorer.exe
    // If so, callback.
}

void stopFinderMonitor() {
    // UnhookWinEvent
}
