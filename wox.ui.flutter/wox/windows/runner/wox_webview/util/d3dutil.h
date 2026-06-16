#pragma once

#include <D3d11.h>
#include <winrt/Windows.Foundation.h>
#include <winrt/Windows.System.h>

inline auto CreateD3DDevice(D3D_DRIVER_TYPE const type,
                            winrt::com_ptr<ID3D11Device>& device) {
  WINRT_ASSERT(!device);

  UINT flags =
      D3D11_CREATE_DEVICE_BGRA_SUPPORT | D3D11_CREATE_DEVICE_VIDEO_SUPPORT;

  //#ifdef _DEBUG
  //	flags |= D3D11_CREATE_DEVICE_DEBUG;
  //#endif

  return D3D11CreateDevice(nullptr, type, nullptr, flags, nullptr, 0,
                           D3D11_SDK_VERSION, device.put(), nullptr, nullptr);
}

inline auto CreateD3DDevice() {
  winrt::com_ptr<ID3D11Device> device;
  HRESULT hr = CreateD3DDevice(D3D_DRIVER_TYPE_HARDWARE, device);

  if (DXGI_ERROR_UNSUPPORTED == hr) {
    CreateD3DDevice(D3D_DRIVER_TYPE_WARP, device);
  }

  return device;
}
