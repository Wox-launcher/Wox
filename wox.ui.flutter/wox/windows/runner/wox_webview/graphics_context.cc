#include "graphics_context.h"

#include "util/d3dutil.h"
#include "util/direct3d11.interop.h"

GraphicsContext::GraphicsContext(rx::RoHelper* rohelper) : rohelper_(rohelper) {
  device_ = CreateD3DDevice();
  if (!device_) {
    return;
  }

  device_->GetImmediateContext(device_context_.put());
  if (FAILED(util::CreateDirect3D11DeviceFromDXGIDevice(
          device_.try_as<IDXGIDevice>().get(),
          (IInspectable**)device_winrt_.put()))) {
    return;
  }

  valid_ = true;
}

winrt::com_ptr<ABI::Windows::UI::Composition::ICompositor>
GraphicsContext::CreateCompositor() {
  HSTRING className;
  HSTRING_HEADER classNameHeader;

  if (FAILED(rohelper_->GetStringReference(
          RuntimeClass_Windows_UI_Composition_Compositor, &className,
          &classNameHeader))) {
    return nullptr;
  }

  winrt::com_ptr<IActivationFactory> af;
  if (FAILED(rohelper_->GetActivationFactory(
          className, __uuidof(IActivationFactory), af.put_void()))) {
    return nullptr;
  }

  winrt::com_ptr<ABI::Windows::UI::Composition::ICompositor> compositor;
  if (FAILED(af->ActivateInstance(
          reinterpret_cast<IInspectable**>(compositor.put())))) {
    return nullptr;
  }

  return compositor;
}

winrt::com_ptr<ABI::Windows::Graphics::Capture::IGraphicsCaptureItem>
GraphicsContext::CreateGraphicsCaptureItemFromVisual(
    ABI::Windows::UI::Composition::IVisual* visual) const {
  HSTRING className;
  HSTRING_HEADER classNameHeader;

  if (FAILED(rohelper_->GetStringReference(
          RuntimeClass_Windows_Graphics_Capture_GraphicsCaptureItem, &className,
          &classNameHeader))) {
    return nullptr;
  }

  ABI::Windows::Graphics::Capture::IGraphicsCaptureItemStatics*
      capture_item_statics;
  if (FAILED(rohelper_->GetActivationFactory(
          className,
          __uuidof(
              ABI::Windows::Graphics::Capture::IGraphicsCaptureItemStatics),
          (void**)&capture_item_statics))) {
    return nullptr;
  }

  winrt::com_ptr<ABI::Windows::Graphics::Capture::IGraphicsCaptureItem>
      capture_item;
  if (FAILED(
          capture_item_statics->CreateFromVisual(visual, capture_item.put()))) {
    return nullptr;
  }

  return capture_item;
}

winrt::com_ptr<ABI::Windows::Graphics::Capture::IDirect3D11CaptureFramePool>
GraphicsContext::CreateCaptureFramePool(
    ABI::Windows::Graphics::DirectX::Direct3D11::IDirect3DDevice* device,
    ABI::Windows::Graphics::DirectX::DirectXPixelFormat pixelFormat,
    INT32 numberOfBuffers, ABI::Windows::Graphics::SizeInt32 size) const {
  HSTRING className;
  HSTRING_HEADER classNameHeader;

  if (FAILED(rohelper_->GetStringReference(
          RuntimeClass_Windows_Graphics_Capture_Direct3D11CaptureFramePool,
          &className, &classNameHeader))) {
    return nullptr;
  }

  ABI::Windows::Graphics::Capture::IDirect3D11CaptureFramePoolStatics*
      capture_frame_pool_statics;
  if (FAILED(rohelper_->GetActivationFactory(
          className,
          __uuidof(ABI::Windows::Graphics::Capture::
                       IDirect3D11CaptureFramePoolStatics),
          (void**)&capture_frame_pool_statics))) {
    return nullptr;
  }

  winrt::com_ptr<ABI::Windows::Graphics::Capture::IDirect3D11CaptureFramePool>
      capture_frame_pool;

  if (FAILED(capture_frame_pool_statics->Create(device, pixelFormat,
                                                numberOfBuffers, size,
                                                capture_frame_pool.put()))) {
    return nullptr;
  }

  return capture_frame_pool;
}

winrt::com_ptr<ABI::Windows::Graphics::Capture::IDirect3D11CaptureFramePool>
GraphicsContext::CreateFreeThreadedCaptureFramePool(
    ABI::Windows::Graphics::DirectX::Direct3D11::IDirect3DDevice* device,
    ABI::Windows::Graphics::DirectX::DirectXPixelFormat pixelFormat,
    INT32 numberOfBuffers, ABI::Windows::Graphics::SizeInt32 size) const {
  HSTRING className;
  HSTRING_HEADER classNameHeader;

  if (FAILED(rohelper_->GetStringReference(
          RuntimeClass_Windows_Graphics_Capture_Direct3D11CaptureFramePool,
          &className, &classNameHeader))) {
    return nullptr;
  }

  ABI::Windows::Graphics::Capture::IDirect3D11CaptureFramePoolStatics2*
      capture_frame_pool_statics;
  if (FAILED(rohelper_->GetActivationFactory(
          className,
          __uuidof(ABI::Windows::Graphics::Capture::
                       IDirect3D11CaptureFramePoolStatics2),
          (void**)&capture_frame_pool_statics))) {
    return nullptr;
  }

  winrt::com_ptr<ABI::Windows::Graphics::Capture::IDirect3D11CaptureFramePool>
      capture_frame_pool;

  if (FAILED(capture_frame_pool_statics->CreateFreeThreaded(
          device, pixelFormat, numberOfBuffers, size,
          capture_frame_pool.put()))) {
    return nullptr;
  }

  return capture_frame_pool;
}
