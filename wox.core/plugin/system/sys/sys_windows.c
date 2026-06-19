#define INITGUID
#define COBJMACROS
#include "sys_windows.h"

#include <endpointvolume.h>
#include <mmdeviceapi.h>

static HRESULT wox_sys_initialize_com(BOOL *needs_uninitialize) {
    *needs_uninitialize = FALSE;
    HRESULT hr = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
    if (hr == RPC_E_CHANGED_MODE) {
        return S_OK;
    }
    if (FAILED(hr)) {
        return hr;
    }
    *needs_uninitialize = TRUE;
    return S_OK;
}

static HRESULT wox_sys_open_audio_endpoint(IAudioEndpointVolume **endpoint, BOOL *needs_uninitialize) {
    HRESULT hr = wox_sys_initialize_com(needs_uninitialize);
    if (FAILED(hr)) {
        return hr;
    }

    IMMDeviceEnumerator *enumerator = NULL;
    IMMDevice *device = NULL;

    hr = CoCreateInstance(&CLSID_MMDeviceEnumerator, NULL, CLSCTX_ALL, &IID_IMMDeviceEnumerator, (void **)&enumerator);
    if (SUCCEEDED(hr)) {
        hr = IMMDeviceEnumerator_GetDefaultAudioEndpoint(enumerator, eRender, eMultimedia, &device);
    }
    if (SUCCEEDED(hr)) {
        hr = IMMDevice_Activate(device, &IID_IAudioEndpointVolume, CLSCTX_ALL, NULL, (void **)endpoint);
    }

    if (device != NULL) {
        IMMDevice_Release(device);
    }
    if (enumerator != NULL) {
        IMMDeviceEnumerator_Release(enumerator);
    }
    if (FAILED(hr) && *needs_uninitialize) {
        CoUninitialize();
        *needs_uninitialize = FALSE;
    }
    return hr;
}

HRESULT wox_sys_set_master_volume(float level) {
    BOOL needs_uninitialize = FALSE;
    IAudioEndpointVolume *endpoint = NULL;
    HRESULT hr = wox_sys_open_audio_endpoint(&endpoint, &needs_uninitialize);
    if (SUCCEEDED(hr)) {
        hr = IAudioEndpointVolume_SetMasterVolumeLevelScalar(endpoint, level, NULL);
    }
    if (endpoint != NULL) {
        IAudioEndpointVolume_Release(endpoint);
    }
    if (needs_uninitialize) {
        CoUninitialize();
    }
    return hr;
}

HRESULT wox_sys_volume_step_up(void) {
    BOOL needs_uninitialize = FALSE;
    IAudioEndpointVolume *endpoint = NULL;
    HRESULT hr = wox_sys_open_audio_endpoint(&endpoint, &needs_uninitialize);
    if (SUCCEEDED(hr)) {
        hr = IAudioEndpointVolume_VolumeStepUp(endpoint, NULL);
    }
    if (endpoint != NULL) {
        IAudioEndpointVolume_Release(endpoint);
    }
    if (needs_uninitialize) {
        CoUninitialize();
    }
    return hr;
}

HRESULT wox_sys_volume_step_down(void) {
    BOOL needs_uninitialize = FALSE;
    IAudioEndpointVolume *endpoint = NULL;
    HRESULT hr = wox_sys_open_audio_endpoint(&endpoint, &needs_uninitialize);
    if (SUCCEEDED(hr)) {
        hr = IAudioEndpointVolume_VolumeStepDown(endpoint, NULL);
    }
    if (endpoint != NULL) {
        IAudioEndpointVolume_Release(endpoint);
    }
    if (needs_uninitialize) {
        CoUninitialize();
    }
    return hr;
}

HRESULT wox_sys_toggle_mute(void) {
    BOOL needs_uninitialize = FALSE;
    IAudioEndpointVolume *endpoint = NULL;
    HRESULT hr = wox_sys_open_audio_endpoint(&endpoint, &needs_uninitialize);
    if (SUCCEEDED(hr)) {
        BOOL muted = FALSE;
        hr = IAudioEndpointVolume_GetMute(endpoint, &muted);
        if (SUCCEEDED(hr)) {
            hr = IAudioEndpointVolume_SetMute(endpoint, !muted, NULL);
        }
    }
    if (endpoint != NULL) {
        IAudioEndpointVolume_Release(endpoint);
    }
    if (needs_uninitialize) {
        CoUninitialize();
    }
    return hr;
}

static BOOL wox_sys_is_shortcut_modifier_pressed(void) {
    return (GetAsyncKeyState(VK_CONTROL) & 0x8000) != 0 ||
           (GetAsyncKeyState(VK_MENU) & 0x8000) != 0 ||
           (GetAsyncKeyState(VK_SHIFT) & 0x8000) != 0 ||
           (GetAsyncKeyState(VK_LWIN) & 0x8000) != 0 ||
           (GetAsyncKeyState(VK_RWIN) & 0x8000) != 0;
}

static void wox_sys_wait_shortcut_modifiers_release(void) {
    for (int i = 0; i < 20; i++) {
        if (!wox_sys_is_shortcut_modifier_pressed()) {
            return;
        }
        Sleep(50);
    }
}

// Task View has no stable shell URI, so use the same shortcut users press manually.
HRESULT wox_sys_show_task_view(void) {
    INPUT ip[4];
    ZeroMemory(ip, sizeof(ip));

    wox_sys_wait_shortcut_modifiers_release();

    ip[0].type = INPUT_KEYBOARD;
    ip[0].ki.wVk = VK_LWIN;

    ip[1].type = INPUT_KEYBOARD;
    ip[1].ki.wVk = VK_TAB;

    ip[2].type = INPUT_KEYBOARD;
    ip[2].ki.wVk = VK_TAB;
    ip[2].ki.dwFlags = KEYEVENTF_KEYUP;

    ip[3].type = INPUT_KEYBOARD;
    ip[3].ki.wVk = VK_LWIN;
    ip[3].ki.dwFlags = KEYEVENTF_KEYUP;

    UINT sent = SendInput(4, ip, sizeof(INPUT));
    if (sent != 4) {
        DWORD err = GetLastError();
        if (err == 0) {
            err = ERROR_GEN_FAILURE;
        }
        return HRESULT_FROM_WIN32(err);
    }

    return S_OK;
}
