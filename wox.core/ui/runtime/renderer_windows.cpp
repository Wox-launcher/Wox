#define WIN32_LEAN_AND_MEAN
#include <windows.h>

#include <d2d1_1.h>
#include <d2d1helper.h>
#include <d3d11.h>
#include <dcomp.h>
#include <dwrite.h>
#include <dxgi1_2.h>

#include <algorithm>
#include <cmath>
#include <string>

#include "renderer_windows.h"

struct WoxRenderer {
  ID3D11Device *device = nullptr;
  ID3D11DeviceContext *context = nullptr;
  IDXGISwapChain1 *swap_chain = nullptr;
  IDCompositionDevice *composition_device = nullptr;
  IDCompositionTarget *composition_target = nullptr;
  IDCompositionVisual *composition_visual = nullptr;
  ID2D1Factory1 *d2d_factory = nullptr;
  ID2D1Device *d2d_device = nullptr;
  ID2D1DeviceContext *d2d_context = nullptr;
  ID2D1Bitmap1 *target_bitmap = nullptr;
  ID2D1SolidColorBrush *brush = nullptr;
  IDWriteFactory *dwrite_factory = nullptr;
	std::wstring font_family = L"Segoe UI";
  float scale = 1.0f;
  bool frame_open = false;
  bool clip_active = false;
};

template <typename T>
static void release_com(T **value) {
  if (*value != nullptr) {
    (*value)->Release();
    *value = nullptr;
  }
}

static D2D1_COLOR_F make_color(uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  return D2D1_COLOR_F{
      static_cast<float>(red) / 255.0f,
      static_cast<float>(green) / 255.0f,
      static_cast<float>(blue) / 255.0f,
      static_cast<float>(alpha) / 255.0f,
  };
}

static HRESULT create_target_bitmap(WoxRenderer *renderer) {
  IDXGISurface *surface = nullptr;
  HRESULT result = renderer->swap_chain->GetBuffer(0, IID_IDXGISurface, reinterpret_cast<void **>(&surface));
  if (FAILED(result)) {
    return result;
  }

  D2D1_BITMAP_PROPERTIES1 properties = {};
  properties.pixelFormat.format = DXGI_FORMAT_B8G8R8A8_UNORM;
  properties.pixelFormat.alphaMode = D2D1_ALPHA_MODE_PREMULTIPLIED;
  properties.dpiX = 96.0f;
  properties.dpiY = 96.0f;
  properties.bitmapOptions = D2D1_BITMAP_OPTIONS_TARGET | D2D1_BITMAP_OPTIONS_CANNOT_DRAW;

  result = renderer->d2d_context->CreateBitmapFromDxgiSurface(surface, &properties, &renderer->target_bitmap);
  surface->Release();
  if (FAILED(result)) {
    return result;
  }

  renderer->d2d_context->SetTarget(renderer->target_bitmap);
  return S_OK;
}

static std::wstring utf8_to_wide(const char *text) {
  if (text == nullptr || *text == '\0') {
    return {};
  }

  const int length = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text, -1, nullptr, 0);
  if (length <= 1) {
    return {};
  }

  std::wstring result(static_cast<size_t>(length), L'\0');
  MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text, -1, result.data(), length);
  result.resize(static_cast<size_t>(length - 1));
  return result;
}

// create_text_format keeps drawing and measurement on identical DirectWrite settings.
static HRESULT create_text_format(WoxRenderer *renderer, float font_size, uint8_t font_weight, IDWriteTextFormat **format) {
  if (renderer == nullptr || renderer->dwrite_factory == nullptr || format == nullptr || font_size <= 0.0f) {
    return E_INVALIDARG;
  }
  DWRITE_FONT_WEIGHT native_font_weight;
  switch (font_weight) {
  case 0:
    native_font_weight = DWRITE_FONT_WEIGHT_NORMAL;
    break;
  case 1:
    native_font_weight = DWRITE_FONT_WEIGHT_SEMI_BOLD;
    break;
  default:
    return E_INVALIDARG;
  }
  HRESULT result = renderer->dwrite_factory->CreateTextFormat(
		renderer->font_family.c_str(),
      nullptr,
      native_font_weight,
      DWRITE_FONT_STYLE_NORMAL,
      DWRITE_FONT_STRETCH_NORMAL,
      font_size,
      L"en-us",
      format);
  if (SUCCEEDED(result)) {
    (*format)->SetWordWrapping(DWRITE_WORD_WRAPPING_NO_WRAP);
  }
  return result;
}

extern "C" int32_t wox_renderer_set_font_family(WoxRenderer *renderer, const char *font_family) {
	if (renderer == nullptr) {
		return E_INVALIDARG;
	}
	const std::wstring family = utf8_to_wide(font_family);
	renderer->font_family = family.empty() ? L"Segoe UI" : family;
	return S_OK;
}

static void destroy_renderer(WoxRenderer *renderer) {
  if (renderer == nullptr) {
    return;
  }

  if (renderer->frame_open && renderer->d2d_context != nullptr) {
    if (renderer->clip_active) {
      renderer->d2d_context->PopAxisAlignedClip();
      renderer->clip_active = false;
    }
    renderer->d2d_context->EndDraw();
  }
  if (renderer->d2d_context != nullptr) {
    renderer->d2d_context->SetTarget(nullptr);
  }
  if (renderer->context != nullptr) {
    renderer->context->ClearState();
  }
  release_com(&renderer->brush);
  release_com(&renderer->target_bitmap);
  release_com(&renderer->d2d_context);
  release_com(&renderer->d2d_device);
  release_com(&renderer->d2d_factory);
  release_com(&renderer->dwrite_factory);
  release_com(&renderer->composition_visual);
  release_com(&renderer->composition_target);
  release_com(&renderer->composition_device);
  release_com(&renderer->swap_chain);
  release_com(&renderer->context);
  release_com(&renderer->device);
  delete renderer;
}

extern "C" int32_t wox_renderer_create(uintptr_t window_handle, uint32_t width, uint32_t height, WoxRenderer **renderer_out) {
  if (window_handle == 0 || renderer_out == nullptr) {
    return E_INVALIDARG;
  }

  auto *renderer = new WoxRenderer();
  *renderer_out = nullptr;

  // DirectComposition requires BGRA surfaces; WARP keeps the window usable when hardware creation fails.
  const UINT device_flags = D3D11_CREATE_DEVICE_BGRA_SUPPORT;
  HRESULT result = D3D11CreateDevice(
      nullptr,
      D3D_DRIVER_TYPE_HARDWARE,
      nullptr,
      device_flags,
      nullptr,
      0,
      D3D11_SDK_VERSION,
      &renderer->device,
      nullptr,
      &renderer->context);
  if (FAILED(result)) {
    result = D3D11CreateDevice(
        nullptr,
        D3D_DRIVER_TYPE_WARP,
        nullptr,
        device_flags,
        nullptr,
        0,
        D3D11_SDK_VERSION,
        &renderer->device,
        nullptr,
        &renderer->context);
  }
  if (FAILED(result)) {
    destroy_renderer(renderer);
    return result;
  }

  IDXGIDevice *dxgi_device = nullptr;
  IDXGIAdapter *adapter = nullptr;
  IDXGIFactory2 *dxgi_factory = nullptr;

  result = renderer->device->QueryInterface(IID_IDXGIDevice, reinterpret_cast<void **>(&dxgi_device));
  if (SUCCEEDED(result)) {
    result = dxgi_device->GetAdapter(&adapter);
  }
  if (SUCCEEDED(result)) {
    result = adapter->GetParent(IID_IDXGIFactory2, reinterpret_cast<void **>(&dxgi_factory));
  }
  if (SUCCEEDED(result)) {
    result = D2D1CreateFactory(D2D1_FACTORY_TYPE_SINGLE_THREADED, __uuidof(ID2D1Factory1), reinterpret_cast<void **>(&renderer->d2d_factory));
  }
  if (SUCCEEDED(result)) {
    result = renderer->d2d_factory->CreateDevice(dxgi_device, &renderer->d2d_device);
  }
  if (SUCCEEDED(result)) {
    result = renderer->d2d_device->CreateDeviceContext(D2D1_DEVICE_CONTEXT_OPTIONS_NONE, &renderer->d2d_context);
  }
  if (SUCCEEDED(result)) {
    result = DWriteCreateFactory(DWRITE_FACTORY_TYPE_SHARED, __uuidof(IDWriteFactory), reinterpret_cast<IUnknown **>(&renderer->dwrite_factory));
  }

  DXGI_SWAP_CHAIN_DESC1 swap_chain_description = {};
  swap_chain_description.Width = width == 0 ? 1 : width;
  swap_chain_description.Height = height == 0 ? 1 : height;
  swap_chain_description.Format = DXGI_FORMAT_B8G8R8A8_UNORM;
  swap_chain_description.SampleDesc.Count = 1;
  swap_chain_description.BufferUsage = DXGI_USAGE_RENDER_TARGET_OUTPUT;
  swap_chain_description.BufferCount = 2;
  swap_chain_description.Scaling = DXGI_SCALING_STRETCH;
  swap_chain_description.SwapEffect = DXGI_SWAP_EFFECT_FLIP_SEQUENTIAL;
  swap_chain_description.AlphaMode = DXGI_ALPHA_MODE_PREMULTIPLIED;

  if (SUCCEEDED(result)) {
    result = dxgi_factory->CreateSwapChainForComposition(renderer->device, &swap_chain_description, nullptr, &renderer->swap_chain);
  }
  if (SUCCEEDED(result)) {
    result = DCompositionCreateDevice(dxgi_device, __uuidof(IDCompositionDevice), reinterpret_cast<void **>(&renderer->composition_device));
  }
  if (SUCCEEDED(result)) {
    result = renderer->composition_device->CreateTargetForHwnd(reinterpret_cast<HWND>(window_handle), TRUE, &renderer->composition_target);
  }
  if (SUCCEEDED(result)) {
    result = renderer->composition_device->CreateVisual(&renderer->composition_visual);
  }
  if (SUCCEEDED(result)) {
    result = renderer->composition_visual->SetContent(renderer->swap_chain);
  }
  if (SUCCEEDED(result)) {
    result = renderer->composition_target->SetRoot(renderer->composition_visual);
  }
  if (SUCCEEDED(result)) {
    result = renderer->composition_device->Commit();
  }
  if (SUCCEEDED(result)) {
    result = create_target_bitmap(renderer);
  }
  if (SUCCEEDED(result)) {
    const D2D1_COLOR_F initial_color = make_color(255, 255, 255, 255);
    result = renderer->d2d_context->CreateSolidColorBrush(initial_color, &renderer->brush);
  }
  if (SUCCEEDED(result)) {
    renderer->d2d_context->SetUnitMode(D2D1_UNIT_MODE_PIXELS);
    renderer->d2d_context->SetTextAntialiasMode(D2D1_TEXT_ANTIALIAS_MODE_GRAYSCALE);
  }

  release_com(&dxgi_factory);
  release_com(&adapter);
  release_com(&dxgi_device);

  if (FAILED(result)) {
    destroy_renderer(renderer);
    return result;
  }

  *renderer_out = renderer;
  return S_OK;
}

extern "C" int32_t wox_renderer_resize(WoxRenderer *renderer, uint32_t width, uint32_t height) {
  if (renderer == nullptr || width == 0 || height == 0) {
    return S_OK;
  }

  renderer->d2d_context->SetTarget(nullptr);
  release_com(&renderer->target_bitmap);

  HRESULT result = renderer->swap_chain->ResizeBuffers(0, width, height, DXGI_FORMAT_UNKNOWN, 0);
  if (FAILED(result)) {
    return result;
  }
  return create_target_bitmap(renderer);
}

extern "C" int32_t wox_renderer_begin_frame(WoxRenderer *renderer, float scale, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (renderer == nullptr || renderer->d2d_context == nullptr || renderer->frame_open) {
    return E_UNEXPECTED;
  }

  if (scale <= 0.0f) {
    return E_INVALIDARG;
  }
  renderer->d2d_context->BeginDraw();
  renderer->frame_open = true;
  renderer->scale = scale;
  renderer->d2d_context->SetTransform(D2D1::Matrix3x2F::Scale(scale, scale));
  const D2D1_COLOR_F color = make_color(red, green, blue, alpha);
  renderer->d2d_context->Clear(&color);
  return S_OK;
}

extern "C" int32_t wox_renderer_fill_rounded_rect(WoxRenderer *renderer, float x, float y, float width, float height, float radius, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (renderer == nullptr || !renderer->frame_open || renderer->brush == nullptr) {
    return E_UNEXPECTED;
  }

  const D2D1_COLOR_F color = make_color(red, green, blue, alpha);
  renderer->brush->SetColor(color);
  const D2D1_RECT_F rect = {x, y, x + width, y + height};
  if (radius <= 0.0f) {
    renderer->d2d_context->FillRectangle(&rect, renderer->brush);
  } else {
    const D2D1_ROUNDED_RECT rounded_rect = {rect, radius, radius};
    renderer->d2d_context->FillRoundedRectangle(&rounded_rect, renderer->brush);
  }
  return S_OK;
}

extern "C" int32_t wox_renderer_fill_convex_polygon(WoxRenderer *renderer, const float *points, int32_t point_count, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (renderer == nullptr || !renderer->frame_open || renderer->brush == nullptr || renderer->d2d_factory == nullptr || points == nullptr || point_count < 3 || point_count > 16) {
    return E_INVALIDARG;
  }

  ID2D1PathGeometry *geometry = nullptr;
  HRESULT result = renderer->d2d_factory->CreatePathGeometry(&geometry);
  ID2D1GeometrySink *sink = nullptr;
  if (SUCCEEDED(result)) {
    result = geometry->Open(&sink);
  }
  if (SUCCEEDED(result)) {
    sink->BeginFigure(D2D1::Point2F(points[0], points[1]), D2D1_FIGURE_BEGIN_FILLED);
    for (int32_t index = 1; index < point_count; index++) {
      sink->AddLine(D2D1::Point2F(points[index * 2], points[index * 2 + 1]));
    }
    sink->EndFigure(D2D1_FIGURE_END_CLOSED);
    result = sink->Close();
  }
  if (SUCCEEDED(result)) {
    renderer->brush->SetColor(make_color(red, green, blue, alpha));
    renderer->d2d_context->FillGeometry(geometry, renderer->brush);
  }
  release_com(&sink);
  release_com(&geometry);
  return result;
}

extern "C" int32_t wox_renderer_stroke_rounded_rect(WoxRenderer *renderer, float x, float y, float width, float height, float radius, float stroke_width, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (renderer == nullptr || !renderer->frame_open || renderer->brush == nullptr || width <= 0.0f || height <= 0.0f || stroke_width <= 0.0f) {
    return E_INVALIDARG;
  }

  renderer->brush->SetColor(make_color(red, green, blue, alpha));
  const float inset = stroke_width * 0.5f;
  const D2D1_RECT_F rect = {x + inset, y + inset, x + width - inset, y + height - inset};
  if (radius <= 0.0f) {
    renderer->d2d_context->DrawRectangle(&rect, renderer->brush, stroke_width);
  } else {
    const float inset_radius = std::max(0.0f, radius - inset);
    const D2D1_ROUNDED_RECT rounded_rect = {rect, inset_radius, inset_radius};
    renderer->d2d_context->DrawRoundedRectangle(&rounded_rect, renderer->brush, stroke_width);
  }
  return S_OK;
}

extern "C" int32_t wox_renderer_draw_text(WoxRenderer *renderer, const char *text, float x, float y, float width, float height, float font_size, uint8_t font_weight, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (renderer == nullptr || !renderer->frame_open || renderer->brush == nullptr || renderer->dwrite_factory == nullptr) {
    return E_UNEXPECTED;
  }

  const std::wstring wide_text = utf8_to_wide(text);
  if (wide_text.empty()) {
    return S_OK;
  }

  // ponytail: create formats per invalidated frame; cache by style when animated text makes this measurable.
  IDWriteTextFormat *format = nullptr;
  HRESULT result = create_text_format(renderer, font_size, font_weight, &format);
  if (FAILED(result)) {
    return result;
  }

  const D2D1_COLOR_F color = make_color(red, green, blue, alpha);
  renderer->brush->SetColor(color);
  const D2D1_RECT_F rect = {x, y, x + width, y + height};
  renderer->d2d_context->DrawTextW(
      wide_text.c_str(),
      static_cast<UINT32>(wide_text.size()),
      format,
      &rect,
      renderer->brush,
      D2D1_DRAW_TEXT_OPTIONS_CLIP,
      DWRITE_MEASURING_MODE_NATURAL);
  format->Release();
  return S_OK;
}

extern "C" int32_t wox_renderer_draw_image(WoxRenderer *renderer, const uint8_t *pixels, uint32_t image_width, uint32_t image_height, uint32_t row_stride, float x, float y, float width, float height) {
  if (renderer == nullptr || !renderer->frame_open || pixels == nullptr || image_width == 0 || image_height == 0 || row_stride < image_width * 4 || width <= 0.0f || height <= 0.0f) {
    return E_INVALIDARG;
  }

  D2D1_BITMAP_PROPERTIES1 properties = D2D1::BitmapProperties1(
      D2D1_BITMAP_OPTIONS_NONE,
      D2D1::PixelFormat(DXGI_FORMAT_R8G8B8A8_UNORM, D2D1_ALPHA_MODE_PREMULTIPLIED),
      96.0f,
      96.0f);
  ID2D1Bitmap1 *bitmap = nullptr;
  HRESULT result = renderer->d2d_context->CreateBitmap(D2D1::SizeU(image_width, image_height), pixels, row_stride, &properties, &bitmap);
  if (FAILED(result)) {
    return result;
  }
  const auto snap = [renderer](float value) { return std::round(value * renderer->scale) / renderer->scale; };
  const D2D1_RECT_F destination = {snap(x), snap(y), snap(x + width), snap(y + height)};
  const D2D1_RECT_F source = {0.0f, 0.0f, static_cast<float>(image_width), static_cast<float>(image_height)};
  renderer->d2d_context->DrawBitmap(bitmap, &destination, 1.0f, D2D1_INTERPOLATION_MODE_HIGH_QUALITY_CUBIC, &source);
  bitmap->Release();
  return S_OK;
}

extern "C" int32_t wox_renderer_set_clip_rect(WoxRenderer *renderer, float x, float y, float width, float height) {
  if (renderer == nullptr || !renderer->frame_open) {
    return E_UNEXPECTED;
  }
  if (renderer->clip_active) {
    renderer->d2d_context->PopAxisAlignedClip();
  }
  const float clipped_width = width > 0.0f ? width : 0.0f;
  const float clipped_height = height > 0.0f ? height : 0.0f;
  const D2D1_RECT_F rect = {x, y, x + clipped_width, y + clipped_height};
  renderer->d2d_context->PushAxisAlignedClip(rect, D2D1_ANTIALIAS_MODE_PER_PRIMITIVE);
  renderer->clip_active = true;
  return S_OK;
}

extern "C" int32_t wox_renderer_clear_clip(WoxRenderer *renderer) {
  if (renderer == nullptr || !renderer->frame_open) {
    return E_UNEXPECTED;
  }
  if (renderer->clip_active) {
    renderer->d2d_context->PopAxisAlignedClip();
    renderer->clip_active = false;
  }
  return S_OK;
}

extern "C" int32_t wox_renderer_measure_text(WoxRenderer *renderer, const char *text, float font_size, uint8_t font_weight, float *width, float *height, float *baseline) {
  if (renderer == nullptr || text == nullptr || width == nullptr || height == nullptr || baseline == nullptr) {
    return E_INVALIDARG;
  }
  *width = 0.0f;
  *height = 0.0f;
  *baseline = 0.0f;
  const std::wstring wide_text = utf8_to_wide(text);
  if (wide_text.empty()) {
    return S_OK;
  }

  IDWriteTextFormat *format = nullptr;
  HRESULT result = create_text_format(renderer, font_size, font_weight, &format);
  if (FAILED(result)) {
    return result;
  }
  IDWriteTextLayout *layout = nullptr;
  result = renderer->dwrite_factory->CreateTextLayout(wide_text.c_str(), static_cast<UINT32>(wide_text.size()), format, 1000000.0f, 1000000.0f, &layout);
  format->Release();
  if (FAILED(result)) {
    return result;
  }

  DWRITE_TEXT_METRICS metrics = {};
  result = layout->GetMetrics(&metrics);
  if (SUCCEEDED(result)) {
    DWRITE_LINE_METRICS line = {};
    UINT32 line_count = 0;
    result = layout->GetLineMetrics(&line, 1, &line_count);
    if (SUCCEEDED(result) && line_count > 0) {
      *width = metrics.widthIncludingTrailingWhitespace;
      *height = metrics.height;
      *baseline = line.baseline;
    }
  }
  layout->Release();
  return result;
}

extern "C" int32_t wox_renderer_end_frame(WoxRenderer *renderer) {
  if (renderer == nullptr || !renderer->frame_open) {
    return E_UNEXPECTED;
  }

  if (renderer->clip_active) {
    renderer->d2d_context->PopAxisAlignedClip();
    renderer->clip_active = false;
  }
  HRESULT result = renderer->d2d_context->EndDraw();
  renderer->frame_open = false;
  if (result == static_cast<HRESULT>(D2DERR_RECREATE_TARGET)) {
    renderer->d2d_context->SetTarget(nullptr);
    release_com(&renderer->target_bitmap);
    return create_target_bitmap(renderer);
  }
  if (FAILED(result)) {
    return result;
  }
  return renderer->swap_chain->Present(1, 0);
}

extern "C" void wox_renderer_destroy(WoxRenderer *renderer) {
  destroy_renderer(renderer);
}
