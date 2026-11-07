#include <windows.h>
#include <ole2.h>
#include <UIAutomation.h>
#include <stdint.h>

// Standard EDIT controls delete one Unicode code point or one CRLF per Backspace.
int snippetWindowsUsesCodepointBackspace(uint64_t target) {
    HWND window = (HWND)(uintptr_t)target;
    WCHAR name[32] = {0};
    return window && GetClassNameW(window, name, 32) && lstrcmpiW(name, L"Edit") == 0;
}

// A window class alone cannot identify password fields inside browsers or custom controls.
// Fail closed if UI Automation cannot identify the focused element or its password state.
int snippetWindowsFocusedControlSafe(void) {
    HRESULT init = CoInitializeEx(NULL, COINIT_MULTITHREADED);
    if (FAILED(init) && init != RPC_E_CHANGED_MODE) return 0;
    IUIAutomation *automation = NULL;
    IUIAutomationElement *element = NULL;
    BOOL password = TRUE, focused = FALSE;
    HRESULT result = CoCreateInstance(&CLSID_CUIAutomation, NULL, CLSCTX_INPROC_SERVER,
                                      &IID_IUIAutomation, (void **)&automation);
    if (SUCCEEDED(result) && automation) result = automation->lpVtbl->GetFocusedElement(automation, &element);
    if (SUCCEEDED(result) && element) result = element->lpVtbl->get_CurrentIsPassword(element, &password);
    if (SUCCEEDED(result) && element) result = element->lpVtbl->get_CurrentHasKeyboardFocus(element, &focused);
    if (element) element->lpVtbl->Release(element);
    if (automation) automation->lpVtbl->Release(automation);
    if (SUCCEEDED(init)) CoUninitialize();
    return SUCCEEDED(result) && element && !password && focused;
}
