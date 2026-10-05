#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <d3d11.h>
#include <dxgi.h>
#include <roapi.h>
#include <winstring.h>
// MinGW's generated Foundation header duplicates BYTE and boolean template specializations.
// Capture does not use IReference<boolean>; skip that duplicate specialization only.
#ifdef __MINGW32__
#define ____FIReference_1_boolean_INTERFACE_DEFINED__
#endif
#include <windows.graphics.capture.interop.h>
#include <windows.foundation.h>
#include "window_capture_windows.h"

namespace capture = ABI::Windows::Graphics::Capture;
namespace direct3d = ABI::Windows::Graphics::DirectX::Direct3D11;

// Scoped COM references release native resources on every failure path.
template <typename T> struct CaptureReference {
  T *value = nullptr;
  ~CaptureReference() { if (value) value->Release(); }
  T **address() { return &value; }
  T *operator->() { return value; }
};

// The Direct3D interop interface is absent from some MinGW header bundles.
struct CaptureSurfaceAccess : IUnknown {
  virtual HRESULT STDMETHODCALLTYPE GetInterface(REFIID iid, void **object) = 0;
};
static const GUID capture_surface_access = {0xa9b3d012, 0x3df2, 0x4ee3, {0xb8, 0xd1, 0x86, 0x95, 0xf4, 0x57, 0xd3, 0xc1}};

// close_capture explicitly stops the capture before releasing the pool and graphics device.
static void close_capture(IUnknown *object) {
  if (!object) return;
  CaptureReference<ABI::Windows::Foundation::IClosable> closable;
  if (SUCCEEDED(object->QueryInterface(__uuidof(ABI::Windows::Foundation::IClosable), (void **)closable.address()))) closable->Close();
}

// Capture one compositor frame of the HWND itself, including native alpha rather than desktop background pixels.
int32_t wox_capture_windows_window(uintptr_t hwnd, int32_t width, int32_t height, uint8_t *rgba) {
  if (!hwnd || !rgba || width <= 0 || height <= 0 || width > 16384 || height > 16384) return E_INVALIDARG;
  HRESULT initialized = RoInitialize(RO_INIT_MULTITHREADED);
  if (FAILED(initialized) && initialized != RPC_E_CHANGED_MODE) return initialized;
  HRESULT status;
  {
    CaptureReference<ID3D11Device> device;
    CaptureReference<ID3D11DeviceContext> context;
    CaptureReference<IDXGIDevice> dxgi_device;
    CaptureReference<IInspectable> inspectable_device;
    CaptureReference<direct3d::IDirect3DDevice> winrt_device;
    CaptureReference<IGraphicsCaptureItemInterop> interop;
    CaptureReference<capture::IGraphicsCaptureItem> item;
    CaptureReference<capture::IDirect3D11CaptureFramePoolStatics2> factory;
    CaptureReference<capture::IDirect3D11CaptureFramePool> pool;
    CaptureReference<capture::IGraphicsCaptureSession> session;
    CaptureReference<capture::IDirect3D11CaptureFrame> frame;
    CaptureReference<direct3d::IDirect3DSurface> surface;
    CaptureReference<CaptureSurfaceAccess> access;
    CaptureReference<ID3D11Texture2D> texture;
    CaptureReference<ID3D11Texture2D> staging;
    HSTRING item_name = nullptr, pool_name = nullptr;
    status = [&]() -> HRESULT {
      HRESULT result = D3D11CreateDevice(nullptr, D3D_DRIVER_TYPE_HARDWARE, nullptr, D3D11_CREATE_DEVICE_BGRA_SUPPORT,
                                       nullptr, 0, D3D11_SDK_VERSION, device.address(), nullptr, context.address());
      if (FAILED(result)) return result;
      result = device->QueryInterface(__uuidof(IDXGIDevice), (void **)dxgi_device.address());
      if (FAILED(result)) return result;
      // Resolve the interop entry point dynamically so older Windows can keep rectangle capture.
      HMODULE d3d_module = GetModuleHandleW(L"d3d11.dll");
      using CreateDevice = HRESULT (WINAPI *)(IDXGIDevice *, IInspectable **);
      auto create_device = (CreateDevice)GetProcAddress(d3d_module, "CreateDirect3D11DeviceFromDXGIDevice");
      if (!create_device) return E_NOTIMPL;
      result = create_device(dxgi_device.value, inspectable_device.address());
      if (FAILED(result)) return result;
      result = inspectable_device->QueryInterface(__uuidof(direct3d::IDirect3DDevice), (void **)winrt_device.address());
      if (FAILED(result)) return result;
      const wchar_t item_class[] = L"Windows.Graphics.Capture.GraphicsCaptureItem";
      result = WindowsCreateString(item_class, ARRAYSIZE(item_class)-1, &item_name);
      if (FAILED(result)) return result;
      result = RoGetActivationFactory(item_name, __uuidof(IGraphicsCaptureItemInterop), (void **)interop.address());
      if (FAILED(result)) return result;
      result = interop->CreateForWindow((HWND)hwnd, __uuidof(capture::IGraphicsCaptureItem), (void **)item.address());
      if (FAILED(result)) return result;
      ABI::Windows::Graphics::SizeInt32 size;
      result = item->get_Size(&size);
      if (FAILED(result)) return result;
      // Do not stretch a resized window or confuse capture pixels with invisible Win32 frame margins.
      if (size.Width != width || size.Height != height) return HRESULT_FROM_WIN32(ERROR_INVALID_DATA);
      const wchar_t pool_class[] = L"Windows.Graphics.Capture.Direct3D11CaptureFramePool";
      result = WindowsCreateString(pool_class, ARRAYSIZE(pool_class)-1, &pool_name);
      if (FAILED(result)) return result;
      result = RoGetActivationFactory(pool_name, __uuidof(capture::IDirect3D11CaptureFramePoolStatics2), (void **)factory.address());
      if (FAILED(result)) return result;
      result = factory->CreateFreeThreaded(winrt_device.value,
          ABI::Windows::Graphics::DirectX::DirectXPixelFormat_B8G8R8A8UIntNormalized, 1, size, pool.address());
      if (FAILED(result)) return result;
      result = pool->CreateCaptureSession(item.value, session.address());
      if (FAILED(result)) return result;
      CaptureReference<capture::IGraphicsCaptureSession2> cursor_options;
      if (SUCCEEDED(session->QueryInterface(__uuidof(capture::IGraphicsCaptureSession2), (void **)cursor_options.address()))) cursor_options->put_IsCursorCaptureEnabled(false);
      result = session->StartCapture();
      if (FAILED(result)) return result;
      ULONGLONG deadline = GetTickCount64() + 1000;
      do {
        result = pool->TryGetNextFrame(frame.address());
        if (FAILED(result)) return result;
        if (frame.value) break;
        Sleep(5);
      } while (GetTickCount64() < deadline);
      if (!frame.value) return HRESULT_FROM_WIN32(WAIT_TIMEOUT);
      result = frame->get_ContentSize(&size);
      if (FAILED(result)) return result;
      if (size.Width != width || size.Height != height) return HRESULT_FROM_WIN32(ERROR_INVALID_DATA);
      result = frame->get_Surface(surface.address());
      if (FAILED(result)) return result;
      result = surface->QueryInterface(capture_surface_access, (void **)access.address());
      if (FAILED(result)) return result;
      result = access->GetInterface(__uuidof(ID3D11Texture2D), (void **)texture.address());
      if (FAILED(result)) return result;
      D3D11_TEXTURE2D_DESC description;
      texture->GetDesc(&description);
      description.Usage = D3D11_USAGE_STAGING;
      description.BindFlags = description.MiscFlags = 0;
      description.CPUAccessFlags = D3D11_CPU_ACCESS_READ;
      result = device->CreateTexture2D(&description, nullptr, staging.address());
      if (FAILED(result)) return result;
      context->CopyResource(staging.value, texture.value);
      D3D11_MAPPED_SUBRESOURCE mapped;
      result = context->Map(staging.value, 0, D3D11_MAP_READ, 0, &mapped);
      if (FAILED(result)) return result;
      for (int32_t y = 0; y < height; y++) {
        auto input = (const uint8_t *)mapped.pData + y * mapped.RowPitch;
        auto output = rgba + (size_t)y * width * 4;
        for (int32_t x = 0; x < width; x++) {
          output[x*4] = input[x*4+2];
          output[x*4+1] = input[x*4+1];
          output[x*4+2] = input[x*4];
          output[x*4+3] = input[x*4+3];
        }
      }
      context->Unmap(staging.value, 0);
      return S_OK;
    }();
    close_capture(frame.value);
    close_capture(session.value);
    close_capture(pool.value);
    WindowsDeleteString(pool_name);
    WindowsDeleteString(item_name);
  }
  if (SUCCEEDED(initialized)) RoUninitialize();
  return status;
}
