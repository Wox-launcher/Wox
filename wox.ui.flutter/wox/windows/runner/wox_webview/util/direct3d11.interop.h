
#pragma once

#include <inspectable.h>
#include <windows.foundation.h>
#include <winrt/windows.graphics.directx.direct3d11.h>

#include "dxgi.h"

namespace Windows {
namespace Graphics {
namespace DirectX {
namespace Direct3D11 {
struct __declspec(uuid("A9B3D012-3DF2-4EE3-B8D1-8695F457D3C1"))
    IDirect3DDxgiInterfaceAccess : ::IUnknown {
  virtual HRESULT __stdcall GetInterface(GUID const& id, void** object) = 0;
};

}  // namespace Direct3D11
}  // namespace DirectX
}  // namespace Graphics
}  // namespace Windows

namespace util {

HRESULT CreateDirect3D11DeviceFromDXGIDevice(IDXGIDevice* dxgiDevice,
                                             IInspectable** graphicsDevice);

template <typename T>
auto GetDXGIInterfaceFromObject(
    winrt::Windows::Foundation::IInspectable const& object) {
  auto access = object.as<
      Windows::Graphics::DirectX::Direct3D11::IDirect3DDxgiInterfaceAccess>();
  winrt::com_ptr<T> result;
  winrt::check_hresult(
      access->GetInterface(winrt::guid_of<T>(), result.put_void()));
  return result;
}

template <typename T>
auto TryGetDXGIInterfaceFromObject(const winrt::com_ptr<IInspectable>& object) {
  auto access = object.try_as<
      Windows::Graphics::DirectX::Direct3D11::IDirect3DDxgiInterfaceAccess>();
  winrt::com_ptr<T> result;
  access->GetInterface(winrt::guid_of<T>(), result.put_void());
  return result;
}

}  // namespace util
