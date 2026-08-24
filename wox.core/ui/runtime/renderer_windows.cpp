#define WIN32_LEAN_AND_MEAN
#include <windows.h>

#include <d2d1_1.h>
#include <d2d1helper.h>
#include <d3d11.h>
#include <dcomp.h>
#include <dwrite.h>
#include <dxgi1_2.h>
#include <dxgi1_3.h>

#include <algorithm>
#include <cmath>
#include <new>
#include <mutex>
#include <string>
#include <vector>

#include "renderer_windows.h"

struct CachedImageBitmap {
  uint64_t image_id = 0;
  uint64_t byte_size = 0;
  uint64_t last_used = 0;
  ID2D1Bitmap1 *bitmap = nullptr;
};

struct WoxRenderer {
  ID3D11Device *device = nullptr;
  IDXGISwapChain1 *swap_chain = nullptr;
  IDXGISwapChain1 *overlay_swap_chain = nullptr;
  IDCompositionDevice *composition_device = nullptr;
  IDCompositionTarget *composition_target = nullptr;
  IDCompositionVisual *composition_root = nullptr;
  IDCompositionVisual *composition_visual = nullptr;
  IDCompositionVisual *overlay_visual = nullptr;
  ID2D1Factory1 *d2d_factory = nullptr;
  ID2D1Device *d2d_device = nullptr;
  ID2D1DeviceContext *d2d_context = nullptr;
  ID2D1Bitmap1 *target_bitmap = nullptr;
  ID2D1Bitmap1 *overlay_target_bitmap = nullptr;
  ID2D1Bitmap1 *cached_large_image_bitmap = nullptr;
  uint64_t cached_large_image_id = 0;
  std::vector<CachedImageBitmap> cached_image_bitmaps;
  uint64_t cached_image_bitmap_bytes = 0;
  uint64_t cached_image_use_serial = 0;
  ID2D1SolidColorBrush *brush = nullptr;
  IDWriteFactory *dwrite_factory = nullptr;
	std::wstring font_family = L"Segoe UI";
  bool uses_default_font_family = true;
  float scale = 1.0f;
  bool frame_open = false;
  bool overlay_active = false;
  bool clip_active = false;
  D2D1_RECT_F clip_rect = {};
  bool damage_clip_active = false;
  RECT present_dirty_rect = {};
  bool present_dirty = false;
  bool cache_large_images = false;
  bool embedded_surface_overlay_enabled = false;
  bool simulate_device_removed = false;
  uint32_t width = 1;
  uint32_t height = 1;
};

static std::mutex shared_d3d_device_mutex;
static ID3D11Device *shared_d3d_device = nullptr;
static uint32_t shared_d3d_device_users = 0;

// All renderers run on the UI thread and can share the process GPU device. Screenshot windows
// still own and release their swap chains and D2D contexts, avoiding another slow D3D cold start.
static HRESULT acquire_shared_d3d_device(ID3D11Device **device_out) {
  if (device_out == nullptr) {
    return E_INVALIDARG;
  }
  std::lock_guard<std::mutex> lock(shared_d3d_device_mutex);
  // Existing renderers keep their own COM references, but new renderers must move to a fresh generation.
  if (shared_d3d_device != nullptr && FAILED(shared_d3d_device->GetDeviceRemovedReason())) {
    shared_d3d_device->Release();
    shared_d3d_device = nullptr;
    shared_d3d_device_users = 0;
  }
  if (shared_d3d_device == nullptr) {
    const UINT device_flags = D3D11_CREATE_DEVICE_BGRA_SUPPORT;
    HRESULT result = D3D11CreateDevice(
        nullptr, D3D_DRIVER_TYPE_HARDWARE, nullptr, device_flags, nullptr, 0,
        D3D11_SDK_VERSION, &shared_d3d_device, nullptr, nullptr);
    if (FAILED(result)) {
      result = D3D11CreateDevice(
          nullptr, D3D_DRIVER_TYPE_WARP, nullptr, device_flags, nullptr, 0,
          D3D11_SDK_VERSION, &shared_d3d_device, nullptr, nullptr);
    }
    if (FAILED(result)) {
      shared_d3d_device = nullptr;
      return result;
    }
  }
  shared_d3d_device->AddRef();
  *device_out = shared_d3d_device;
  shared_d3d_device_users++;
  return S_OK;
}

static void release_shared_d3d_device(ID3D11Device **device) {
  if (device == nullptr || *device == nullptr) {
    return;
  }
  std::lock_guard<std::mutex> lock(shared_d3d_device_mutex);
  ID3D11Device *released_device = *device;
  released_device->Release();
  *device = nullptr;
  // A removed shared device can be replaced while another window still owns its old generation.
  if (released_device != shared_d3d_device) {
    return;
  }
  if (shared_d3d_device_users > 0) {
    shared_d3d_device_users--;
  }
  if (shared_d3d_device_users == 0 && shared_d3d_device != nullptr) {
    shared_d3d_device->Release();
    shared_d3d_device = nullptr;
  }
}

// WoxWebViewVisual keeps host placement separate from the visual subtree owned by WebView2.
struct WoxWebViewVisual {
  IDCompositionVisual *host = nullptr;
  IDCompositionVisual *target = nullptr;
  IDCompositionRectangleClip *clip = nullptr;
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

static HRESULT create_target_bitmap(WoxRenderer *renderer, IDXGISwapChain1 *swap_chain, ID2D1Bitmap1 **bitmap) {
  IDXGISurface *surface = nullptr;
  HRESULT result = swap_chain->GetBuffer(0, IID_IDXGISurface, reinterpret_cast<void **>(&surface));
  if (FAILED(result)) {
    return result;
  }

  D2D1_BITMAP_PROPERTIES1 properties = {};
  properties.pixelFormat.format = DXGI_FORMAT_B8G8R8A8_UNORM;
  properties.pixelFormat.alphaMode = D2D1_ALPHA_MODE_PREMULTIPLIED;
  properties.dpiX = 96.0f;
  properties.dpiY = 96.0f;
  properties.bitmapOptions = D2D1_BITMAP_OPTIONS_TARGET | D2D1_BITMAP_OPTIONS_CANNOT_DRAW;

  result = renderer->d2d_context->CreateBitmapFromDxgiSurface(surface, &properties, bitmap);
  surface->Release();
  if (FAILED(result)) {
    return result;
  }

  return S_OK;
}

static void clear_cached_image_bitmaps(WoxRenderer *renderer) {
  for (CachedImageBitmap &entry : renderer->cached_image_bitmaps) {
    release_com(&entry.bitmap);
  }
  renderer->cached_image_bitmaps.clear();
  renderer->cached_image_bitmap_bytes = 0;
}

static ID2D1Bitmap1 *find_cached_image_bitmap(WoxRenderer *renderer, uint64_t image_id) {
  for (CachedImageBitmap &entry : renderer->cached_image_bitmaps) {
    if (entry.image_id == image_id) {
      entry.last_used = ++renderer->cached_image_use_serial;
      return entry.bitmap;
    }
  }
  return nullptr;
}

// The launcher normally shows fewer than 16 icons. These limits absorb repaint bursts without
// turning decoded source images into a second, unbounded GPU cache.
static bool cache_image_bitmap(WoxRenderer *renderer, uint64_t image_id, uint64_t byte_size, ID2D1Bitmap1 *bitmap) {
  constexpr size_t max_cached_image_count = 32;
  constexpr uint64_t max_cached_image_entry_bytes = 1ULL * 1024ULL * 1024ULL;
  constexpr uint64_t max_cached_image_bytes = 8ULL * 1024ULL * 1024ULL;
  if (image_id == 0 || bitmap == nullptr || byte_size > max_cached_image_entry_bytes) {
    return false;
  }

  while (!renderer->cached_image_bitmaps.empty() &&
         (renderer->cached_image_bitmaps.size() >= max_cached_image_count ||
          renderer->cached_image_bitmap_bytes + byte_size > max_cached_image_bytes)) {
    auto oldest = std::min_element(
        renderer->cached_image_bitmaps.begin(), renderer->cached_image_bitmaps.end(),
        [](const CachedImageBitmap &left, const CachedImageBitmap &right) {
          return left.last_used < right.last_used;
        });
    renderer->cached_image_bitmap_bytes -= oldest->byte_size;
    release_com(&oldest->bitmap);
    renderer->cached_image_bitmaps.erase(oldest);
  }

  if (renderer->cached_image_bitmap_bytes + byte_size > max_cached_image_bytes) {
    return false;
  }
  renderer->cached_image_bitmaps.push_back(CachedImageBitmap{
      image_id,
      byte_size,
      ++renderer->cached_image_use_serial,
      bitmap,
  });
  renderer->cached_image_bitmap_bytes += byte_size;
  return true;
}

static DXGI_SWAP_CHAIN_DESC1 composition_swap_chain_description(uint32_t width, uint32_t height) {
  DXGI_SWAP_CHAIN_DESC1 description = {};
  description.Width = width == 0 ? 1 : width;
  description.Height = height == 0 ? 1 : height;
  description.Format = DXGI_FORMAT_B8G8R8A8_UNORM;
  description.SampleDesc.Count = 1;
  description.BufferUsage = DXGI_USAGE_RENDER_TARGET_OUTPUT;
  description.BufferCount = 2;
  description.Scaling = DXGI_SCALING_STRETCH;
  description.SwapEffect = DXGI_SWAP_EFFECT_FLIP_SEQUENTIAL;
  description.AlphaMode = DXGI_ALPHA_MODE_PREMULTIPLIED;
  return description;
}

// The overlay is uncommon, so keep its double-buffered surface lazy while preserving its visual layer.
static HRESULT ensure_embedded_surface_overlay(WoxRenderer *renderer) {
  if (!renderer->embedded_surface_overlay_enabled || renderer->overlay_visual == nullptr) {
    return E_UNEXPECTED;
  }
  if (renderer->overlay_swap_chain != nullptr && renderer->overlay_target_bitmap != nullptr) {
    return S_OK;
  }

  IDXGIDevice *dxgi_device = nullptr;
  IDXGIAdapter *adapter = nullptr;
  IDXGIFactory2 *factory = nullptr;
  HRESULT result = renderer->device->QueryInterface(IID_IDXGIDevice, reinterpret_cast<void **>(&dxgi_device));
  if (SUCCEEDED(result)) {
    result = dxgi_device->GetAdapter(&adapter);
  }
  if (SUCCEEDED(result)) {
    result = adapter->GetParent(IID_IDXGIFactory2, reinterpret_cast<void **>(&factory));
  }
  if (SUCCEEDED(result)) {
    const DXGI_SWAP_CHAIN_DESC1 description = composition_swap_chain_description(renderer->width, renderer->height);
    result = factory->CreateSwapChainForComposition(renderer->device, &description, nullptr, &renderer->overlay_swap_chain);
  }
  if (SUCCEEDED(result)) {
    result = create_target_bitmap(renderer, renderer->overlay_swap_chain, &renderer->overlay_target_bitmap);
  }
  if (SUCCEEDED(result)) {
    result = renderer->overlay_visual->SetContent(renderer->overlay_swap_chain);
  }
  if (SUCCEEDED(result)) {
    result = renderer->composition_device->Commit();
  }

  release_com(&factory);
  release_com(&adapter);
  release_com(&dxgi_device);
  if (FAILED(result)) {
    if (renderer->overlay_visual != nullptr) {
      renderer->overlay_visual->SetContent(nullptr);
    }
    release_com(&renderer->overlay_target_bitmap);
    release_com(&renderer->overlay_swap_chain);
  }
  return result;
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
static HRESULT create_text_format(WoxRenderer *renderer, float font_size, uint8_t font_weight, uint8_t font_family, uint8_t italic, IDWriteTextFormat **format) {
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
  const wchar_t *family = font_family == 1 ? L"Consolas" : renderer->font_family.c_str();
  if (font_family > 1 || italic > 1) {
    return E_INVALIDARG;
  }
  HRESULT result = renderer->dwrite_factory->CreateTextFormat(
		family,
      nullptr,
      native_font_weight,
      italic == 1 ? DWRITE_FONT_STYLE_ITALIC : DWRITE_FONT_STYLE_NORMAL,
      DWRITE_FONT_STRETCH_NORMAL,
      font_size,
      L"en-us",
      format);
  if (SUCCEEDED(result)) {
    (*format)->SetWordWrapping(DWRITE_WORD_WRAPPING_NO_WRAP);
  }
  return result;
}

static constexpr bool is_cjk_codepoint(uint32_t codepoint) {
  return (codepoint >= 0x2E80 && codepoint <= 0x303F) ||
         (codepoint >= 0x31C0 && codepoint <= 0x31EF) ||
         (codepoint >= 0x3400 && codepoint <= 0x4DBF) ||
         (codepoint >= 0x4E00 && codepoint <= 0x9FFF) ||
         (codepoint >= 0xF900 && codepoint <= 0xFAFF) ||
         (codepoint >= 0xFE30 && codepoint <= 0xFE4F) ||
         (codepoint >= 0xFF00 && codepoint <= 0xFFEF) ||
         (codepoint >= 0x20000 && codepoint <= 0x2FA1F);
}

static_assert(is_cjk_codepoint(0x4E2D));
static_assert(is_cjk_codepoint(0x20000));
static_assert(!is_cjk_codepoint(L'A'));

// apply_default_cjk_font keeps the Windows default aligned with Flutter without overriding a configured application font.
static HRESULT apply_default_cjk_font(IDWriteTextLayout *layout, const std::wstring &text) {
  for (UINT32 index = 0; index < text.size();) {
    const UINT32 run_start = index;
    uint32_t codepoint = text[index++];
    if (codepoint >= 0xD800 && codepoint <= 0xDBFF && index < text.size()) {
      const uint32_t low = text[index];
      if (low >= 0xDC00 && low <= 0xDFFF) {
        codepoint = 0x10000 + ((codepoint - 0xD800) << 10) + (low - 0xDC00);
        index++;
      }
    }
    if (!is_cjk_codepoint(codepoint)) {
      continue;
    }
    const DWRITE_TEXT_RANGE range = {run_start, index - run_start};
    const HRESULT result = layout->SetFontFamilyName(L"Microsoft YaHei", range);
    if (FAILED(result)) {
      return result;
    }
  }
  return S_OK;
}

// create_text_layout keeps drawing and measurement on the same font fallback path.
static HRESULT create_text_layout(WoxRenderer *renderer, const std::wstring &text, float font_size, uint8_t font_weight, uint8_t font_family, uint8_t italic, float width, float height, IDWriteTextLayout **layout) {
  IDWriteTextFormat *format = nullptr;
  HRESULT result = create_text_format(renderer, font_size, font_weight, font_family, italic, &format);
  if (FAILED(result)) {
    return result;
  }
  result = renderer->dwrite_factory->CreateTextLayout(text.c_str(), static_cast<UINT32>(text.size()), format, width, height, layout);
  format->Release();
  if (SUCCEEDED(result) && renderer->uses_default_font_family && font_family == 0) {
    result = apply_default_cjk_font(*layout, text);
    if (FAILED(result)) {
      release_com(layout);
    }
  }
  return result;
}

extern "C" int32_t wox_renderer_set_font_family(WoxRenderer *renderer, const char *font_family) {
	if (renderer == nullptr) {
		return E_INVALIDARG;
	}
	const std::wstring family = utf8_to_wide(font_family);
	renderer->uses_default_font_family = family.empty();
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
    if (renderer->damage_clip_active) {
      renderer->d2d_context->PopAxisAlignedClip();
      renderer->damage_clip_active = false;
    }
    renderer->d2d_context->EndDraw();
  }
  if (renderer->d2d_context != nullptr) {
    renderer->d2d_context->SetTarget(nullptr);
  }
  release_com(&renderer->brush);
  clear_cached_image_bitmaps(renderer);
  release_com(&renderer->cached_large_image_bitmap);
  release_com(&renderer->overlay_target_bitmap);
  release_com(&renderer->target_bitmap);
  release_com(&renderer->d2d_context);
  release_com(&renderer->d2d_device);
  release_com(&renderer->d2d_factory);
  release_com(&renderer->dwrite_factory);
  release_com(&renderer->overlay_visual);
  release_com(&renderer->composition_visual);
  release_com(&renderer->composition_root);
  release_com(&renderer->composition_target);
  release_com(&renderer->composition_device);
  release_com(&renderer->overlay_swap_chain);
  release_com(&renderer->swap_chain);
  release_shared_d3d_device(&renderer->device);
  delete renderer;
}

extern "C" int32_t wox_renderer_create(uintptr_t window_handle, uint32_t width, uint32_t height, int32_t enable_embedded_surface_overlay, WoxRenderer **renderer_out) {
  if (window_handle == 0 || renderer_out == nullptr) {
    return E_INVALIDARG;
  }

  auto *renderer = new WoxRenderer();
  *renderer_out = nullptr;
  // Screenshot windows disable embedded surfaces and repeatedly draw one virtual-desktop image.
  // Retaining that large source bitmap avoids queuing another full GPU upload for every setup frame.
  renderer->cache_large_images = enable_embedded_surface_overlay == 0;
  renderer->embedded_surface_overlay_enabled = enable_embedded_surface_overlay != 0;
  renderer->width = width == 0 ? 1 : width;
  renderer->height = height == 0 ? 1 : height;

  HRESULT result = acquire_shared_d3d_device(&renderer->device);
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

  const DXGI_SWAP_CHAIN_DESC1 swap_chain_description = composition_swap_chain_description(width, height);

  if (SUCCEEDED(result)) {
    result = dxgi_factory->CreateSwapChainForComposition(renderer->device, &swap_chain_description, nullptr, &renderer->swap_chain);
  }
  if (SUCCEEDED(result)) {
    result = DCompositionCreateDevice(dxgi_device, __uuidof(IDCompositionDevice), reinterpret_cast<void **>(&renderer->composition_device));
  }
  if (SUCCEEDED(result)) {
    result = renderer->composition_device->CreateTargetForHwnd(reinterpret_cast<HWND>(window_handle), FALSE, &renderer->composition_target);
  }
  if (SUCCEEDED(result)) {
    result = renderer->composition_device->CreateVisual(&renderer->composition_root);
  }
  if (SUCCEEDED(result)) {
    result = renderer->composition_device->CreateVisual(&renderer->composition_visual);
  }
  if (SUCCEEDED(result) && renderer->embedded_surface_overlay_enabled) {
    result = renderer->composition_device->CreateVisual(&renderer->overlay_visual);
  }
  if (SUCCEEDED(result)) {
    result = renderer->composition_visual->SetContent(renderer->swap_chain);
  }
  if (SUCCEEDED(result)) {
    result = renderer->composition_root->AddVisual(renderer->composition_visual, FALSE, nullptr);
  }
  if (SUCCEEDED(result) && renderer->embedded_surface_overlay_enabled) {
    result = renderer->composition_root->AddVisual(renderer->overlay_visual, TRUE, renderer->composition_visual);
  }
  if (SUCCEEDED(result)) {
    result = renderer->composition_target->SetRoot(renderer->composition_root);
  }
  if (SUCCEEDED(result)) {
    result = renderer->composition_device->Commit();
  }
  if (SUCCEEDED(result)) {
    result = create_target_bitmap(renderer, renderer->swap_chain, &renderer->target_bitmap);
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
  release_com(&renderer->overlay_target_bitmap);

  HRESULT result = renderer->swap_chain->ResizeBuffers(0, width, height, DXGI_FORMAT_UNKNOWN, 0);
  if (SUCCEEDED(result) && renderer->overlay_swap_chain != nullptr) {
    result = renderer->overlay_swap_chain->ResizeBuffers(0, width, height, DXGI_FORMAT_UNKNOWN, 0);
  }
  if (FAILED(result)) {
    return result;
  }
  result = create_target_bitmap(renderer, renderer->swap_chain, &renderer->target_bitmap);
  if (SUCCEEDED(result) && renderer->overlay_swap_chain != nullptr) {
    result = create_target_bitmap(renderer, renderer->overlay_swap_chain, &renderer->overlay_target_bitmap);
  }
  if (SUCCEEDED(result)) {
    renderer->width = width;
    renderer->height = height;
  }
  return result;
}

extern "C" int32_t wox_renderer_trim(WoxRenderer *renderer) {
  if (renderer == nullptr || renderer->device == nullptr || renderer->frame_open) {
    return E_UNEXPECTED;
  }
  release_com(&renderer->cached_large_image_bitmap);
  renderer->cached_large_image_id = 0;
  clear_cached_image_bitmaps(renderer);

  IDXGIDevice3 *dxgi_device = nullptr;
  const HRESULT result = renderer->device->QueryInterface(__uuidof(IDXGIDevice3), reinterpret_cast<void **>(&dxgi_device));
  if (result == E_NOINTERFACE) {
    return S_OK;
  }
  if (FAILED(result)) {
    return result;
  }
  dxgi_device->Trim();
  dxgi_device->Release();
  return S_OK;
}

extern "C" int32_t wox_renderer_clear_image_cache(WoxRenderer *renderer) {
  if (renderer == nullptr || renderer->d2d_device == nullptr || renderer->frame_open) {
    return E_UNEXPECTED;
  }
  clear_cached_image_bitmaps(renderer);
  // Releasing our bitmap references is not enough: Direct2D keeps internal CPU-side resource
  // caches after upload. A hidden window has no useful warm resources, so release them eagerly.
  renderer->d2d_device->ClearResources(0);
  return S_OK;
}

extern "C" int32_t wox_renderer_begin_frame(WoxRenderer *renderer, float scale, float damage_x, float damage_y, float damage_width, float damage_height, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (renderer == nullptr || renderer->d2d_context == nullptr || renderer->frame_open) {
    return E_UNEXPECTED;
  }

  if (scale <= 0.0f) {
    return E_INVALIDARG;
  }
  renderer->d2d_context->SetTarget(renderer->target_bitmap);
  renderer->d2d_context->BeginDraw();
  renderer->frame_open = true;
  renderer->overlay_active = false;
  renderer->scale = scale;
  renderer->present_dirty = false;
  renderer->d2d_context->SetTransform(D2D1::Matrix3x2F::Scale(scale, scale));
  const D2D1_COLOR_F color = make_color(red, green, blue, alpha);
  if (damage_width > 0.0f && damage_height > 0.0f) {
    const D2D1_RECT_F damage = {damage_x, damage_y, damage_x + damage_width, damage_y + damage_height};
    const D2D1_SIZE_U target_size = renderer->target_bitmap->GetPixelSize();
    // Present1 dirty rectangles use physical pixels while Direct2D draws in logical coordinates.
    renderer->present_dirty_rect = {
        std::max<LONG>(0, static_cast<LONG>(std::floor(damage_x * scale))),
        std::max<LONG>(0, static_cast<LONG>(std::floor(damage_y * scale))),
        std::min<LONG>(static_cast<LONG>(target_size.width), static_cast<LONG>(std::ceil((damage_x + damage_width) * scale))),
        std::min<LONG>(static_cast<LONG>(target_size.height), static_cast<LONG>(std::ceil((damage_y + damage_height) * scale))),
    };
    renderer->present_dirty = renderer->present_dirty_rect.right > renderer->present_dirty_rect.left && renderer->present_dirty_rect.bottom > renderer->present_dirty_rect.top;
    renderer->d2d_context->PushAxisAlignedClip(damage, D2D1_ANTIALIAS_MODE_ALIASED);
    renderer->damage_clip_active = true;
    renderer->brush->SetColor(color);
    renderer->d2d_context->SetPrimitiveBlend(D2D1_PRIMITIVE_BLEND_COPY);
    renderer->d2d_context->FillRectangle(damage, renderer->brush);
    renderer->d2d_context->SetPrimitiveBlend(D2D1_PRIMITIVE_BLEND_SOURCE_OVER);
  } else {
    renderer->d2d_context->Clear(&color);
  }
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

extern "C" int32_t wox_renderer_draw_text(WoxRenderer *renderer, const char *text, float x, float y, float width, float height, float font_size, uint8_t font_weight, uint8_t font_family, uint8_t italic, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (renderer == nullptr || !renderer->frame_open || renderer->brush == nullptr || renderer->dwrite_factory == nullptr) {
    return E_UNEXPECTED;
  }

  const std::wstring wide_text = utf8_to_wide(text);
  if (wide_text.empty()) {
    return S_OK;
  }

  // ponytail: create layouts per invalidated frame; cache by text and style when animated text makes this measurable.
  IDWriteTextLayout *layout = nullptr;
  HRESULT result = create_text_layout(renderer, wide_text, font_size, font_weight, font_family, italic, width, height, &layout);
  if (FAILED(result)) {
    return result;
  }

  const D2D1_COLOR_F color = make_color(red, green, blue, alpha);
  renderer->brush->SetColor(color);
  renderer->d2d_context->DrawTextLayout(D2D1::Point2F(x, y), layout, renderer->brush, static_cast<D2D1_DRAW_TEXT_OPTIONS>(D2D1_DRAW_TEXT_OPTIONS_CLIP | D2D1_DRAW_TEXT_OPTIONS_ENABLE_COLOR_FONT));
  layout->Release();
  return S_OK;
}

extern "C" int32_t wox_renderer_draw_image(WoxRenderer *renderer, uint64_t image_id, const uint8_t *pixels, uint32_t image_width, uint32_t image_height, uint32_t row_stride, float x, float y, float width, float height, float rotation_radians, float corner_radius) {
  if (renderer == nullptr || !renderer->frame_open || pixels == nullptr || image_width == 0 || image_height == 0 || row_stride < image_width * 4 || width <= 0.0f || height <= 0.0f) {
    return E_INVALIDARG;
  }

  D2D1_BITMAP_PROPERTIES1 properties = D2D1::BitmapProperties1(
      D2D1_BITMAP_OPTIONS_NONE,
      D2D1::PixelFormat(DXGI_FORMAT_R8G8B8A8_UNORM, D2D1_ALPHA_MODE_PREMULTIPLIED),
      96.0f,
      96.0f);
  ID2D1Bitmap1 *bitmap = nullptr;
  bool release_bitmap = true;
  const uint64_t image_bytes = static_cast<uint64_t>(row_stride) * image_height;
  const bool cache_bitmap = renderer->cache_large_images && image_id != 0 && image_bytes >= 8ULL * 1024ULL * 1024ULL;
  if (cache_bitmap && renderer->cached_large_image_id == image_id && renderer->cached_large_image_bitmap != nullptr) {
    bitmap = renderer->cached_large_image_bitmap;
    release_bitmap = false;
  } else if (!cache_bitmap && image_id != 0 && (bitmap = find_cached_image_bitmap(renderer, image_id)) != nullptr) {
    release_bitmap = false;
  } else {
    HRESULT result = renderer->d2d_context->CreateBitmap(D2D1::SizeU(image_width, image_height), pixels, row_stride, &properties, &bitmap);
    if (FAILED(result)) {
      return result;
    }
    if (cache_bitmap) {
      release_com(&renderer->cached_large_image_bitmap);
      renderer->cached_large_image_bitmap = bitmap;
      renderer->cached_large_image_id = image_id;
      release_bitmap = false;
    } else if (cache_image_bitmap(renderer, image_id, image_bytes, bitmap)) {
      release_bitmap = false;
    }
  }
  HRESULT result = S_OK;
  const auto snap = [renderer](float value) { return std::round(value * renderer->scale) / renderer->scale; };
  const D2D1_RECT_F destination = {snap(x), snap(y), snap(x + width), snap(y + height)};
  const D2D1_RECT_F source = {0.0f, 0.0f, static_cast<float>(image_width), static_cast<float>(image_height)};
  D2D1_MATRIX_3X2_F transform;
  renderer->d2d_context->GetTransform(&transform);
  if (rotation_radians != 0.0f) {
    const float degrees = rotation_radians * 180.0f / 3.14159265358979323846f;
    const D2D1_POINT_2F center = D2D1::Point2F(x + width * 0.5f, y + height * 0.5f);
    renderer->d2d_context->SetTransform(D2D1::Matrix3x2F::Rotation(degrees, center) * transform);
  }
  ID2D1RoundedRectangleGeometry *clip_geometry = nullptr;
  if (corner_radius > 0.0f) {
    const float radius = std::min(corner_radius, std::min(width, height) * 0.5f);
    const D2D1_ROUNDED_RECT rounded = {destination, radius, radius};
    result = renderer->d2d_factory->CreateRoundedRectangleGeometry(rounded, &clip_geometry);
    if (FAILED(result)) {
      renderer->d2d_context->SetTransform(transform);
      if (release_bitmap) {
        bitmap->Release();
      }
      return result;
    }
    D2D1_LAYER_PARAMETERS1 layer = {};
    layer.contentBounds = D2D1::InfiniteRect();
    layer.geometricMask = clip_geometry;
    layer.maskAntialiasMode = D2D1_ANTIALIAS_MODE_PER_PRIMITIVE;
    layer.maskTransform = D2D1::Matrix3x2F::Identity();
    layer.opacity = 1.0f;
    layer.layerOptions = D2D1_LAYER_OPTIONS1_NONE;
    renderer->d2d_context->PushLayer(&layer, nullptr);
  }
  renderer->d2d_context->DrawBitmap(bitmap, &destination, 1.0f, D2D1_INTERPOLATION_MODE_HIGH_QUALITY_CUBIC, &source);
  if (clip_geometry != nullptr) {
    renderer->d2d_context->PopLayer();
    clip_geometry->Release();
  }
  if (rotation_radians != 0.0f) {
    renderer->d2d_context->SetTransform(transform);
  }
  if (release_bitmap) {
    bitmap->Release();
  }
  return S_OK;
}

extern "C" int32_t wox_renderer_begin_embedded_surface_overlay(WoxRenderer *renderer) {
  if (renderer == nullptr || !renderer->frame_open || renderer->overlay_active) {
    return E_UNEXPECTED;
  }
  HRESULT result = ensure_embedded_surface_overlay(renderer);
  if (FAILED(result)) {
    return result;
  }
  const bool restore_clip = renderer->clip_active;
  const D2D1_RECT_F clip_rect = renderer->clip_rect;
  if (renderer->clip_active) {
    renderer->d2d_context->PopAxisAlignedClip();
    renderer->clip_active = false;
  }
  if (renderer->damage_clip_active) {
    renderer->d2d_context->PopAxisAlignedClip();
    renderer->damage_clip_active = false;
  }
  result = renderer->d2d_context->EndDraw();
  if (FAILED(result)) {
    return result;
  }
  renderer->d2d_context->SetTarget(renderer->overlay_target_bitmap);
  renderer->d2d_context->BeginDraw();
  const D2D1_COLOR_F transparent = D2D1::ColorF(0, 0.0f);
  renderer->d2d_context->Clear(&transparent);
  renderer->overlay_active = true;
  if (restore_clip) {
    renderer->d2d_context->PushAxisAlignedClip(clip_rect, D2D1_ANTIALIAS_MODE_PER_PRIMITIVE);
    renderer->clip_active = true;
    renderer->clip_rect = clip_rect;
  }
  return S_OK;
}

extern "C" int32_t wox_renderer_create_webview_visual(WoxRenderer *renderer, void **visual, void **root_visual_target) {
  if (renderer == nullptr || visual == nullptr || root_visual_target == nullptr) {
    return E_INVALIDARG;
  }
  *visual = nullptr;
  *root_visual_target = nullptr;
  WoxWebViewVisual *webview_visual = new (std::nothrow) WoxWebViewVisual();
  if (webview_visual == nullptr) {
    return E_OUTOFMEMORY;
  }
  HRESULT result = renderer->composition_device->CreateVisual(&webview_visual->host);
  if (SUCCEEDED(result)) {
    result = renderer->composition_device->CreateVisual(&webview_visual->target);
  }
  if (SUCCEEDED(result)) {
    result = renderer->composition_device->CreateRectangleClip(&webview_visual->clip);
  }
  if (SUCCEEDED(result)) {
    result = webview_visual->host->AddVisual(webview_visual->target, TRUE, nullptr);
  }
  if (SUCCEEDED(result)) {
    // Insert the WebView between the application background and its overlay UI.
    result = renderer->composition_root->AddVisual(webview_visual->host, FALSE, renderer->overlay_visual);
  }
  if (SUCCEEDED(result)) {
    result = renderer->composition_device->Commit();
  }
  if (FAILED(result)) {
    release_com(&webview_visual->clip);
    release_com(&webview_visual->target);
    release_com(&webview_visual->host);
    delete webview_visual;
    return result;
  }
  *visual = webview_visual;
  *root_visual_target = webview_visual->target;
  return S_OK;
}

extern "C" int32_t wox_renderer_set_webview_visual_bounds(WoxRenderer *renderer, void *visual, float x, float y, float width, float height, float corner_radius) {
  if (renderer == nullptr || visual == nullptr) {
    return E_INVALIDARG;
  }
  WoxWebViewVisual *webview_visual = static_cast<WoxWebViewVisual *>(visual);
  HRESULT result = webview_visual->host->SetOffsetX(x);
  if (SUCCEEDED(result)) {
    result = webview_visual->host->SetOffsetY(y);
  }
  const float radius = std::max(0.0f, std::min(corner_radius, std::min(width, height) * 0.5f));
  if (SUCCEEDED(result)) {
    result = webview_visual->clip->SetLeft(0.0f);
  }
  if (SUCCEEDED(result)) {
    result = webview_visual->clip->SetTop(0.0f);
  }
  if (SUCCEEDED(result)) {
    result = webview_visual->clip->SetRight(width);
  }
  if (SUCCEEDED(result)) {
    result = webview_visual->clip->SetBottom(height);
  }
  if (SUCCEEDED(result)) {
    result = webview_visual->clip->SetTopLeftRadiusX(radius);
  }
  if (SUCCEEDED(result)) {
    result = webview_visual->clip->SetTopLeftRadiusY(radius);
  }
  if (SUCCEEDED(result)) {
    result = webview_visual->clip->SetTopRightRadiusX(radius);
  }
  if (SUCCEEDED(result)) {
    result = webview_visual->clip->SetTopRightRadiusY(radius);
  }
  if (SUCCEEDED(result)) {
    result = webview_visual->clip->SetBottomLeftRadiusX(radius);
  }
  if (SUCCEEDED(result)) {
    result = webview_visual->clip->SetBottomLeftRadiusY(radius);
  }
  if (SUCCEEDED(result)) {
    result = webview_visual->clip->SetBottomRightRadiusX(radius);
  }
  if (SUCCEEDED(result)) {
    result = webview_visual->clip->SetBottomRightRadiusY(radius);
  }
  if (SUCCEEDED(result)) {
    result = webview_visual->host->SetClip(webview_visual->clip);
  }
  if (SUCCEEDED(result)) {
    result = renderer->composition_device->Commit();
  }
  return result;
}

extern "C" int32_t wox_renderer_remove_webview_visual(WoxRenderer *renderer, void *visual) {
  if (renderer == nullptr || visual == nullptr) {
    return E_INVALIDARG;
  }
  WoxWebViewVisual *webview_visual = static_cast<WoxWebViewVisual *>(visual);
  HRESULT result = renderer->composition_root->RemoveVisual(webview_visual->host);
  if (webview_visual->host != nullptr) {
    HRESULT remove_children_result = webview_visual->host->RemoveAllVisuals();
    if (SUCCEEDED(result)) {
      result = remove_children_result;
    }
  }
  if (SUCCEEDED(result)) {
    result = renderer->composition_device->Commit();
  }
  release_com(&webview_visual->target);
  release_com(&webview_visual->clip);
  release_com(&webview_visual->host);
  delete webview_visual;
  return result;
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
  renderer->clip_rect = rect;
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

extern "C" int32_t wox_renderer_measure_text(WoxRenderer *renderer, const char *text, float font_size, uint8_t font_weight, uint8_t font_family, uint8_t italic, float *width, float *height, float *baseline) {
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

  IDWriteTextLayout *layout = nullptr;
  HRESULT result = create_text_layout(renderer, wide_text, font_size, font_weight, font_family, italic, 1000000.0f, 1000000.0f, &layout);
  if (FAILED(result)) {
    return result;
  }

  DWRITE_TEXT_METRICS metrics = {};
  result = layout->GetMetrics(&metrics);
  if (SUCCEEDED(result)) {
    *width = metrics.widthIncludingTrailingWhitespace;
    *height = metrics.height;
    UINT32 line_count = 0;
    layout->GetLineMetrics(nullptr, 0, &line_count);
    if (line_count > 0) {
      std::vector<DWRITE_LINE_METRICS> lines(line_count);
      result = layout->GetLineMetrics(lines.data(), line_count, &line_count);
      if (SUCCEEDED(result)) {
        *baseline = lines[0].baseline;
      }
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
  if (renderer->damage_clip_active) {
    renderer->d2d_context->PopAxisAlignedClip();
    renderer->damage_clip_active = false;
  }
  HRESULT result = renderer->d2d_context->EndDraw();
  if (SUCCEEDED(result) && renderer->overlay_target_bitmap != nullptr && !renderer->overlay_active) {
    renderer->d2d_context->SetTarget(renderer->overlay_target_bitmap);
    renderer->d2d_context->BeginDraw();
    const D2D1_COLOR_F transparent = D2D1::ColorF(0, 0.0f);
    renderer->d2d_context->Clear(&transparent);
    result = renderer->d2d_context->EndDraw();
  }
  renderer->frame_open = false;
  if (renderer->simulate_device_removed) {
    renderer->simulate_device_removed = false;
    result = DXGI_ERROR_DEVICE_REMOVED;
  }
  if (FAILED(result)) {
    return result;
  }
  DXGI_PRESENT_PARAMETERS parameters = {};
  if (renderer->present_dirty) {
    parameters.DirtyRectsCount = 1;
    parameters.pDirtyRects = &renderer->present_dirty_rect;
  }
  result = renderer->swap_chain->Present1(1, 0, &parameters);
  if (FAILED(result)) {
    return result;
  }
  if (renderer->overlay_swap_chain != nullptr) {
    return renderer->overlay_swap_chain->Present1(1, 0, &parameters);
  }
  return S_OK;
}

extern "C" int32_t wox_renderer_simulate_device_removed(WoxRenderer *renderer) {
  if (renderer == nullptr) {
    return E_INVALIDARG;
  }
  // Automation injects the HRESULT at EndDraw so the normal Go recovery and retry path remains intact.
  renderer->simulate_device_removed = true;
  return S_OK;
}

extern "C" void wox_renderer_destroy(WoxRenderer *renderer) {
  destroy_renderer(renderer);
}
