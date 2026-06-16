#include "texture_bridge_fallback.h"

#include <iostream>

#include "util/direct3d11.interop.h"
#include "util/swizzle.h"

TextureBridgeFallback::TextureBridgeFallback(
    GraphicsContext* graphics_context,
    ABI::Windows::UI::Composition::IVisual* visual)
    : TextureBridge(graphics_context, visual) {}

TextureBridgeFallback::~TextureBridgeFallback() {
  const std::lock_guard<std::mutex> lock(buffer_mutex_);
}

void TextureBridgeFallback::ProcessFrame(
    winrt::com_ptr<ID3D11Texture2D> src_texture) {
  D3D11_TEXTURE2D_DESC desc;
  src_texture->GetDesc(&desc);

  const auto width = desc.Width;
  const auto height = desc.Height;

  bool is_exact_size;
  EnsureStagingTexture(width, height, is_exact_size);

  auto device_context = graphics_context_->d3d_device_context();
  auto staging_texture = staging_texture_.get();

  if (is_exact_size) {
    device_context->CopyResource(staging_texture, src_texture.get());
  } else {
    D3D11_BOX client_box;
    client_box.top = 0;
    client_box.left = 0;
    client_box.right = width;
    client_box.bottom = height;
    client_box.front = 0;
    client_box.back = 1;
    device_context->CopySubresourceRegion(staging_texture, 0, 0, 0, 0,
                                          src_texture.get(), 0, &client_box);
  }

  D3D11_MAPPED_SUBRESOURCE mappedResource;
  if (!SUCCEEDED(device_context->Map(staging_texture, 0, D3D11_MAP_READ, 0,
                                     &mappedResource))) {
    return;
  }

  {
    const std::lock_guard<std::mutex> lock(buffer_mutex_);
    if (!pixel_buffer_ || pixel_buffer_->width != width ||
        pixel_buffer_->height != height) {
      if (!pixel_buffer_) {
        pixel_buffer_ = std::make_unique<FlutterDesktopPixelBuffer>();
        pixel_buffer_->release_context = &buffer_mutex_;
        // Gets invoked after the FlutterDesktopPixelBuffer's
        // backing buffer has been uploaded.
        pixel_buffer_->release_callback = [](void* opaque) {
          auto mutex = reinterpret_cast<std::mutex*>(opaque);
          // Gets locked just before |CopyPixelBuffer| returns.
          mutex->unlock();
        };
      }
      pixel_buffer_->width = width;
      pixel_buffer_->height = height;
      const auto size = width * height * 4;
      backing_pixel_buffer_.reset(new uint8_t[size]);
      pixel_buffer_->buffer = backing_pixel_buffer_.get();
    }

    const auto src_pitch_in_pixels = mappedResource.RowPitch / 4;
    RGBA_to_BGRA(reinterpret_cast<uint32_t*>(backing_pixel_buffer_.get()),
                 static_cast<const uint32_t*>(mappedResource.pData), height,
                 src_pitch_in_pixels, width);
  }

  device_context->Unmap(staging_texture, 0);
}

void TextureBridgeFallback::EnsureStagingTexture(uint32_t width,
                                                 uint32_t height,
                                                 bool& is_exact_size) {
  // Only recreate an existing texture if it's too small.
  if (!staging_texture_ || staging_texture_size_.width < width ||
      staging_texture_size_.height < height) {
    D3D11_TEXTURE2D_DESC dstDesc = {};
    dstDesc.ArraySize = 1;
    dstDesc.MipLevels = 1;
    dstDesc.BindFlags = 0;
    dstDesc.CPUAccessFlags = D3D11_CPU_ACCESS_READ;
    dstDesc.Format = static_cast<DXGI_FORMAT>(kPixelFormat);
    dstDesc.Width = width;
    dstDesc.Height = height;
    dstDesc.MiscFlags = 0;
    dstDesc.SampleDesc.Count = 1;
    dstDesc.SampleDesc.Quality = 0;
    dstDesc.Usage = D3D11_USAGE_STAGING;

    staging_texture_ = nullptr;
    if (!SUCCEEDED(graphics_context_->d3d_device()->CreateTexture2D(
            &dstDesc, nullptr, staging_texture_.put()))) {
      std::cerr << "Creating dst texture failed" << std::endl;
      return;
    }

    staging_texture_size_ = {width, height};
  }

  is_exact_size = staging_texture_size_.width == width &&
                  staging_texture_size_.height == height;
}

const FlutterDesktopPixelBuffer* TextureBridgeFallback::CopyPixelBuffer(
    size_t width, size_t height) {
  const std::lock_guard<std::mutex> lock(mutex_);

  if (!is_running_) {
    return nullptr;
  }

  if (last_frame_) {
    ProcessFrame(last_frame_);
  }

  auto buffer = pixel_buffer_.get();
  // Only lock the mutex if the buffer is not null
  // (to ensure the release callback gets called)
  if (buffer) {
    // Gets unlocked in the FlutterDesktopPixelBuffer's release callback.
    buffer_mutex_.lock();
  }
  return buffer;
}
