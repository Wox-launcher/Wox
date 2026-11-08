#ifndef WOX_SNIPPET_FOCUS_WINDOWS_H
#define WOX_SNIPPET_FOCUS_WINDOWS_H

#include <stdint.h>

typedef enum {
    SNIPPET_TRIGGER_UNAVAILABLE,
    SNIPPET_TRIGGER_SELECTED,
    SNIPPET_TRIGGER_CHANGED,
} SnippetTriggerSelection;

int snippetWindowsFocusedControlSafe(void);
int snippetWindowsUsesCodepointBackspace(uint64_t target);
SnippetTriggerSelection snippetWindowsSelectOmniboxTrigger(const uint16_t *trigger, int length);

#endif
