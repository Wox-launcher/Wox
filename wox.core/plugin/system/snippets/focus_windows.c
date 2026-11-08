#include <windows.h>
#include <ole2.h>
#include <UIAutomation.h>
#include <wchar.h>

#include "focus_windows.h"

// Standard EDIT controls delete one Unicode code point or one CRLF per Backspace.
int snippetWindowsUsesCodepointBackspace(uint64_t target) {
    HWND window = (HWND)(uintptr_t)target;
    WCHAR name[32] = {0};
    return window && GetClassNameW(window, name, 32) && lstrcmpiW(name, L"Edit") == 0;
}

// The caller owns the focused element and must have initialized COM.
static IUIAutomationElement *snippetWindowsFocusedElement(void) {
    IUIAutomation *automation = NULL;
    IUIAutomationElement *element = NULL;
    HRESULT result = CoCreateInstance(&CLSID_CUIAutomation, NULL, CLSCTX_INPROC_SERVER,
                                      &IID_IUIAutomation, (void **)&automation);
    if (FAILED(result) || !automation) return NULL;
    result = automation->lpVtbl->GetFocusedElement(automation, &element);
    automation->lpVtbl->Release(automation);
    if (FAILED(result) && element) {
        element->lpVtbl->Release(element);
        return NULL;
    }
    return element;
}

// A window class alone cannot identify password fields inside browsers or custom controls.
// Fail closed if UI Automation cannot identify the focused element or its password state.
int snippetWindowsFocusedControlSafe(void) {
    HRESULT init = CoInitializeEx(NULL, COINIT_MULTITHREADED);
    if (FAILED(init) && init != RPC_E_CHANGED_MODE) return 0;
    IUIAutomationElement *element = snippetWindowsFocusedElement();
    BOOL password = TRUE, focused = FALSE;
    int safe = element && SUCCEEDED(element->lpVtbl->get_CurrentIsPassword(element, &password)) && !password &&
               SUCCEEDED(element->lpVtbl->get_CurrentHasKeyboardFocus(element, &focused)) && focused;
    if (element) element->lpVtbl->Release(element);
    if (SUCCEEDED(init)) CoUninitialize();
    return safe;
}

// Verify the exact trigger before extending the selection so stale keyboard
// input cannot cause unrelated text to be replaced.
static SnippetTriggerSelection snippetWindowsSelectTrigger(IUIAutomationTextPattern *pattern, const uint16_t *trigger, int length) {
    IUIAutomationTextRangeArray *ranges = NULL;
    IUIAutomationTextRange *range = NULL;
    int count = 0, moved = 0;
    BSTR text = NULL;
    SnippetTriggerSelection result = SNIPPET_TRIGGER_UNAVAILABLE;
    if (SUCCEEDED(pattern->lpVtbl->GetSelection(pattern, &ranges)) && ranges &&
        SUCCEEDED(ranges->lpVtbl->get_Length(ranges, &count)) && count == 1 &&
        SUCCEEDED(ranges->lpVtbl->GetElement(ranges, 0, &range)) && range) {
        result = SNIPPET_TRIGGER_CHANGED;
        if (SUCCEEDED(range->lpVtbl->MoveEndpointByUnit(range, TextPatternRangeEndpoint_Start, TextUnit_Character, -length, &moved)) &&
            moved == -length && SUCCEEDED(range->lpVtbl->GetText(range, length, &text)) &&
            SysStringLen(text) == (unsigned int)length && wmemcmp(text, (const WCHAR *)trigger, length) == 0 &&
            SUCCEEDED(range->lpVtbl->Select(range))) {
            result = SNIPPET_TRIGGER_SELECTED;
        }
    }
    SysFreeString(text);
    if (range) range->lpVtbl->Release(range);
    if (ranges) ranges->lpVtbl->Release(ranges);
    return result;
}

// Chrome's address bar selects its inline autocomplete suffix. The first
// Backspace clears that suffix, leaving too few Backspaces to erase the trigger.
// Match the accessible control class to limit selection-based replacement to
// the address bar and keep ordinary webpage inputs on the normal path.
SnippetTriggerSelection snippetWindowsSelectOmniboxTrigger(const uint16_t *trigger, int length) {
    HRESULT init = CoInitializeEx(NULL, COINIT_MULTITHREADED);
    if (FAILED(init) && init != RPC_E_CHANGED_MODE) return SNIPPET_TRIGGER_UNAVAILABLE;
    IUIAutomationElement *element = snippetWindowsFocusedElement();
    IUIAutomationTextPattern *pattern = NULL;
    BSTR name = NULL;
    SnippetTriggerSelection result = SNIPPET_TRIGGER_UNAVAILABLE;
    if (element && SUCCEEDED(element->lpVtbl->get_CurrentClassName(element, &name)) &&
        name && lstrcmpW(name, L"OmniboxViewViews") == 0 &&
        SUCCEEDED(element->lpVtbl->GetCurrentPatternAs(element, UIA_TextPatternId,
                                                      &IID_IUIAutomationTextPattern, (void **)&pattern)) && pattern) {
        result = snippetWindowsSelectTrigger(pattern, trigger, length);
    }
    if (pattern) pattern->lpVtbl->Release(pattern);
    SysFreeString(name);
    if (element) element->lpVtbl->Release(element);
    if (SUCCEEDED(init)) CoUninitialize();
    return result;
}
