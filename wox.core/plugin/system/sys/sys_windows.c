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
