#pragma once

#include <D3d11.h>
#include <windows.graphics.capture.h>
#include <windows.ui.composition.h>
#include <winrt/Windows.Foundation.h>

#include "util/rohelper.h"

class GraphicsContext {
 public:
  GraphicsContext(rx::RoHelper* rohelper);

  inline bool IsValid() const { return valid_; }

  ABI::Windows::Graphics::DirectX::Direct3D11::IDirect3DDevice* device() const {
    return device_winrt_.get();
  }
  ID3D11Device* d3d_device() const { return device_.get(); }
  ID3D11DeviceContext* d3d_device_context() const {
    return device_context_.get();
  }

  winrt::com_ptr<ABI::Windows::UI::Composition::ICompositor> CreateCompositor();

  winrt::com_ptr<ABI::Windows::Graphics::Capture::IGraphicsCaptureItem>
  CreateGraphicsCaptureItemFromVisual(
      ABI::Windows::UI::Composition::IVisual* visual) const;

  winrt::com_ptr<ABI::Windows::Graphics::Capture::IDirect3D11CaptureFramePool>
  CreateCaptureFramePool(
      ABI::Windows::Graphics::DirectX::Direct3D11::IDirect3DDevice* device,
      ABI::Windows::Graphics::DirectX::DirectXPixelFormat pixelFormat,
      INT32 numberOfBuffers, ABI::Windows::Graphics::SizeInt32 size) const;

  winrt::com_ptr<ABI::Windows::Graphics::Capture::IDirect3D11CaptureFramePool>
  CreateFreeThreadedCaptureFramePool(
      ABI::Windows::Graphics::DirectX::Direct3D11::IDirect3DDevice* device,
      ABI::Windows::Graphics::DirectX::DirectXPixelFormat pixelFormat,
      INT32 numberOfBuffers, ABI::Windows::Graphics::SizeInt32 size) const;

 private:
  bool valid_ = false;
  rx::RoHelper* rohelper_;
  winrt::com_ptr<ABI::Windows::Graphics::DirectX::Direct3D11::IDirect3DDevice>
      device_winrt_;
  winrt::com_ptr<ID3D11Device> device_{nullptr};
  winrt::com_ptr<ID3D11DeviceContext> device_context_{nullptr};
};
