#pragma once

#include <flutter/texture_registrar.h>

#include <mutex>

#include "texture_bridge.h"

class TextureBridgeFallback : public TextureBridge {
 public:
  TextureBridgeFallback(GraphicsContext* graphics_context,
                        ABI::Windows::UI::Composition::IVisual* visual);
  ~TextureBridgeFallback() override;

  const FlutterDesktopPixelBuffer* CopyPixelBuffer(size_t width, size_t height);

 private:
  Size staging_texture_size_ = {0, 0};
  winrt::com_ptr<ID3D11Texture2D> staging_texture_{nullptr};
  std::mutex buffer_mutex_;
  std::unique_ptr<uint8_t> backing_pixel_buffer_;
  std::unique_ptr<FlutterDesktopPixelBuffer> pixel_buffer_;

  void ProcessFrame(winrt::com_ptr<ID3D11Texture2D> src_texture);
  void EnsureStagingTexture(uint32_t width, uint32_t height,
                            bool& is_exact_size);
};
