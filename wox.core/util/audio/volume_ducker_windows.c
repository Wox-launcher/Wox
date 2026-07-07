#include <windows.h>
#include <mmdeviceapi.h>
#include <endpointvolume.h>
#include <stdlib.h>

// getSystemVolumeWin returns the current system output volume as 0-100.
int getSystemVolumeWin() {
    HRESULT hr;
    IMMDeviceEnumerator *pEnum = NULL;
    IMMDevice *pDevice = NULL;
    IAudioEndpointVolume *pEndpoint = NULL;
    float volume = 0.5f;

    CoInitialize(NULL);

    hr = CoCreateInstance(&CLSID_MMDeviceEnumerator, NULL, CLSCTX_ALL,
                          &IID_IMMDeviceEnumerator, (void**)&pEnum);
    if (FAILED(hr)) goto cleanup;

    hr = pEnum->lpVtbl->GetDefaultAudioEndpoint(pEnum, eRender, eConsole, &pDevice);
    if (FAILED(hr)) goto cleanup;

    hr = pDevice->lpVtbl->Activate(pDevice, &IID_IAudioEndpointVolume,
                                    CLSCTX_ALL, NULL, (void**)&pEndpoint);
    if (FAILED(hr)) goto cleanup;

    hr = pEndpoint->lpVtbl->GetMasterVolumeLevelScalar(pEndpoint, &volume);
    if (FAILED(hr)) goto cleanup;

cleanup:
    if (pEndpoint) pEndpoint->lpVtbl->Release(pEndpoint);
    if (pDevice) pDevice->lpVtbl->Release(pDevice);
    if (pEnum) pEnum->lpVtbl->Release(pEnum);
    CoUninitialize();

    return (int)(volume * 100.0f + 0.5f);
}

// setSystemVolumeWin sets the system output volume (0-100).
void setSystemVolumeWin(int volume) {
    HRESULT hr;
    IMMDeviceEnumerator *pEnum = NULL;
    IMMDevice *pDevice = NULL;
    IAudioEndpointVolume *pEndpoint = NULL;
    float fVolume = (float)volume / 100.0f;

    CoInitialize(NULL);

    hr = CoCreateInstance(&CLSID_MMDeviceEnumerator, NULL, CLSCTX_ALL,
                          &IID_IMMDeviceEnumerator, (void**)&pEnum);
    if (FAILED(hr)) goto cleanup;

    hr = pEnum->lpVtbl->GetDefaultAudioEndpoint(pEnum, eRender, eConsole, &pDevice);
    if (FAILED(hr)) goto cleanup;

    hr = pDevice->lpVtbl->Activate(pDevice, &IID_IAudioEndpointVolume,
                                    CLSCTX_ALL, NULL, (void**)&pEndpoint);
    if (FAILED(hr)) goto cleanup;

    hr = pEndpoint->lpVtbl->SetMasterVolumeLevelScalar(pEndpoint, fVolume, NULL);

cleanup:
    if (pEndpoint) pEndpoint->lpVtbl->Release(pEndpoint);
    if (pDevice) pDevice->lpVtbl->Release(pDevice);
    if (pEnum) pEnum->lpVtbl->Release(pEnum);
    CoUninitialize();
}