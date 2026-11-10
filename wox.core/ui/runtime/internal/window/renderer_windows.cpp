#define WIN32_LEAN_AND_MEAN
#include <windows.h>

#include <d2d1_1.h>
#include <d2d1effects.h>
#include <d2d1helper.h>
#include <d3d11.h>
#include <dcomp.h>
#include <dwrite.h>
#include <dxgi1_2.h>
#include <dxgi1_3.h>
#include <dxgi1_4.h>

#include <algorithm>
#include <cmath>
#include <cstdio>
#include <new>
#include <mutex>
#include <string>
#include <vector>

#include "renderer_windows.h"
#include "floating_material_windows_tone.h"

struct CachedImageBitmap {
  uint64_t image_id = 0;
  uint64_t byte_size = 0;
  uint64_t last_used = 0;
  ID2D1Bitmap1 *bitmap = nullptr;
};

// StableLineBox is the line box of a string that contains Latin, Han, Kana, and Hangul.
// Caching it keeps every later string on that same baseline.
struct StableLineBox {
  std::wstring family;
  float font_size = 0;
  uint8_t weight = 0;
  uint8_t italic = 0;
  uint8_t font_slot = 0;
  bool cjk_override = false;
  float baseline = 0;
  float height = 0;
  bool occupied = false;
};

// Groups containing backdrop materials use readable targets. Ordinary groups retain D2D's pooled layers.
struct CompositeLayer {
  ID2D1Bitmap1 *bitmap = nullptr;
  ID2D1Bitmap1 *parent = nullptr;
  ID2D1LinearGradientBrush *mask = nullptr;
  D2D1_RECT_F bounds = {};
  D2D1_MATRIX_3X2_F transform = {};
  float opacity = 1;
};

struct WoxRenderer {
  ID3D11Device *device = nullptr;
  IDXGISwapChain1 *swap_chain = nullptr;
  IDXGISwapChain1 *overlay_swap_chain = nullptr;
  IDCompositionDevice *composition_device = nullptr;
  IDCompositionTarget *composition_target = nullptr;
  IDCompositionVisual *composition_root = nullptr;
 IDCompositionRectangleClip *window_clip = nullptr;
  IDCompositionVisual *composition_visual = nullptr;
  IDCompositionVisual *overlay_visual = nullptr;
  ID2D1Factory1 *d2d_factory = nullptr;
  ID2D1Device *d2d_device = nullptr;
  ID2D1DeviceContext *d2d_context = nullptr;
  ID2D1Bitmap1 *target_bitmap = nullptr;
  ID2D1Bitmap1 *overlay_target_bitmap = nullptr;
  // Only the few physical pixel columns beneath the active caret are retained.
  ID2D1Bitmap1 *caret_backdrop = nullptr;
  D2D1_RECT_F caret_rect = {};
  D2D1_RECT_U caret_pixels = {};
  float caret_scale = 0.0f;
  bool caret_valid = false;
  WoxRendererResourceStats resource_stats = {};
  std::vector<CachedImageBitmap> cached_image_bitmaps;
  // Uploads too large for the retained cache but referenced again within the same frame, such as
  // a repeated theme surface slice. They live until EndDraw so one image costs one upload per frame.
  std::vector<CachedImageBitmap> frame_image_bitmaps;
  uint64_t cached_image_bitmap_bytes = 0;
  uint64_t cached_image_use_serial = 0;
  ID2D1SolidColorBrush *brush = nullptr;
  // Floating material blur resources (see wox_renderer_floating_material). The source
  // bitmap holds a copy of the back buffer under the surface and only grows, so a panel
  // that repaints every second reuses it; a hidden window releases it with the image cache.
  ID2D1Bitmap1 *material_source_bitmap = nullptr;
  D2D1_SIZE_U material_source_size = {};
  ID2D1Effect *material_crop_effect = nullptr;
  ID2D1Effect *material_blur_effect = nullptr;
  ID2D1Effect *material_tone_effect = nullptr;
  IDWriteFactory *dwrite_factory = nullptr;
	std::wstring font_family = L"Segoe UI";
  bool uses_default_font_family = true;
  float scale = 1.0f;
  bool frame_open = false;
  bool overlay_active = false;
  std::vector<CompositeLayer> composite_layers;
  std::vector<ID2D1Bitmap1 *> composite_cache;
  std::vector<ID2D1Bitmap1 *> retired_composite_bitmaps;
  uint64_t composite_bitmap_bytes = 0;
  bool clip_active = false;
  D2D1_RECT_F clip_rect = {};
  ID2D1PathGeometry *clip_polygon = nullptr;
  bool test_offscreen = false;
  bool damage_clip_active = false;
  RECT present_dirty_rect = {};
  bool present_dirty = false;
  bool embedded_surface_overlay_enabled = false;
  bool simulate_device_removed = false;
  bool uses_warp = false;
  D3D_FEATURE_LEVEL feature_level = D3D_FEATURE_LEVEL_9_1;
  DXGI_ADAPTER_DESC adapter_description = {};
  HRESULT last_composition_commit_result = S_OK;
  HRESULT last_end_draw_result = S_OK;
  HRESULT last_present_result = S_OK;
  uint32_t width = 1;
  uint32_t height = 1;
  // Line boxes for the mixed-script sample, keyed by font, size, and style.
  StableLineBox stable_line_boxes[16] = {};
  int stable_line_box_cursor = 0;
};

static std::mutex shared_d3d_device_mutex;
static ID3D11Device *shared_d3d_device = nullptr;
static uint32_t shared_d3d_device_users = 0;
static bool shared_d3d_uses_warp = false;
static D3D_FEATURE_LEVEL shared_d3d_feature_level = D3D_FEATURE_LEVEL_9_1;

// All renderers run on the UI thread and can share the process GPU device. Screenshot windows
// still own and release their swap chains and D2D contexts, avoiding another slow D3D cold start.
static HRESULT acquire_shared_d3d_device(bool force_warp, ID3D11Device **device_out, bool *uses_warp_out, D3D_FEATURE_LEVEL *feature_level_out) {
  if (device_out == nullptr || uses_warp_out == nullptr || feature_level_out == nullptr) {
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
    shared_d3d_uses_warp = force_warp;
    HRESULT result = E_FAIL;
    if (!force_warp) {
      result = D3D11CreateDevice(
          nullptr, D3D_DRIVER_TYPE_HARDWARE, nullptr, device_flags, nullptr, 0,
          D3D11_SDK_VERSION, &shared_d3d_device, &shared_d3d_feature_level, nullptr);
    }
    if (force_warp || FAILED(result)) {
      shared_d3d_uses_warp = true;
      result = D3D11CreateDevice(
          nullptr, D3D_DRIVER_TYPE_WARP, nullptr, device_flags, nullptr, 0,
          D3D11_SDK_VERSION, &shared_d3d_device, &shared_d3d_feature_level, nullptr);
    }
    if (FAILED(result)) {
      shared_d3d_device = nullptr;
      return result;
    }
  }
  shared_d3d_device->AddRef();
  *device_out = shared_d3d_device;
  *uses_warp_out = shared_d3d_uses_warp;
  *feature_level_out = shared_d3d_feature_level;
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
  // Keep the final WARP reference across hidden-window trims: destroying the software device
  // can block the UI thread in FreeLibrary/LdrUnloadDll (issue #4637). Reuse it on the next show.
  if (shared_d3d_device_users == 0 && shared_d3d_device != nullptr && !shared_d3d_uses_warp) {
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

// wox_renderer_process_gpu_memory asks the driver what it currently attributes to this process
// instead of reconstructing a total from the caches we happen to track. DXGI's per-process usage
// already covers swap chain back buffers, uploaded bitmaps, shader and command buffers, and
// driver-internal state, which is what the shared Direct3D device retains while any window lives.
// It reports S_FALSE with zeroed counters when no renderer holds the shared device, so a trimmed
// hidden process is distinguishable from a query failure.
extern "C" int32_t wox_renderer_process_gpu_memory(WoxRendererGpuMemory *memory) {
  if (memory == nullptr) {
    return E_INVALIDARG;
  }
  *memory = {};

  ID3D11Device *device = nullptr;
  {
    std::lock_guard<std::mutex> lock(shared_d3d_device_mutex);
    if (shared_d3d_device == nullptr) {
      return S_FALSE;
    }
    device = shared_d3d_device;
    device->AddRef();
  }

  IDXGIDevice *dxgi_device = nullptr;
  IDXGIAdapter *adapter = nullptr;
  IDXGIAdapter3 *memory_adapter = nullptr;
  HRESULT result = device->QueryInterface(IID_IDXGIDevice, reinterpret_cast<void **>(&dxgi_device));
  if (SUCCEEDED(result)) {
    result = dxgi_device->GetAdapter(&adapter);
  }
  if (SUCCEEDED(result)) {
    result = adapter->QueryInterface(__uuidof(IDXGIAdapter3), reinterpret_cast<void **>(&memory_adapter));
  }
  if (SUCCEEDED(result)) {
    // DXGI reports an adapter-local and a non-local segment, but which one is system memory
    // depends on the adapter. A discrete GPU keeps the local segment in dedicated video memory
    // and uses the non-local segment for system memory. An integrated GPU carves its local
    // segment out of system RAM and leaves the non-local segment unused, so reporting its local
    // usage as video memory would hide it from the process footprint it actually belongs to.
    D3D11_FEATURE_DATA_D3D11_OPTIONS2 options = {};
    const bool unified_memory =
        SUCCEEDED(device->CheckFeatureSupport(D3D11_FEATURE_D3D11_OPTIONS2, &options, sizeof(options))) &&
        options.UnifiedMemoryArchitecture;
    memory->unified_memory = unified_memory ? 1 : 0;

    DXGI_QUERY_VIDEO_MEMORY_INFO local = {};
    const bool has_local = SUCCEEDED(memory_adapter->QueryVideoMemoryInfo(0, DXGI_MEMORY_SEGMENT_GROUP_LOCAL, &local));
    DXGI_QUERY_VIDEO_MEMORY_INFO non_local = {};
    const bool has_non_local = SUCCEEDED(memory_adapter->QueryVideoMemoryInfo(0, DXGI_MEMORY_SEGMENT_GROUP_NON_LOCAL, &non_local));

    if (unified_memory) {
      if (has_local) {
        memory->system_bytes = static_cast<int64_t>(local.CurrentUsage);
        memory->system_budget_bytes = static_cast<int64_t>(local.Budget);
      }
    } else {
      if (has_non_local) {
        memory->system_bytes = static_cast<int64_t>(non_local.CurrentUsage);
        memory->system_budget_bytes = static_cast<int64_t>(non_local.Budget);
      }
      if (has_local) {
        memory->dedicated_bytes = static_cast<int64_t>(local.CurrentUsage);
        memory->dedicated_budget_bytes = static_cast<int64_t>(local.Budget);
      }
    }
  }

  release_com(&memory_adapter);
  release_com(&adapter);
  release_com(&dxgi_device);
  device->Release();
  return result;
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

// Readable groups reuse a bounded pool across frames; in-flight targets cannot be reused before EndDraw.
static HRESULT acquire_composite_bitmap(WoxRenderer *renderer, ID2D1Bitmap1 *parent, ID2D1Bitmap1 **out) {
  const auto size = parent->GetPixelSize();
  const auto format = parent->GetPixelFormat();
  for (auto entry = renderer->composite_cache.begin(); entry != renderer->composite_cache.end(); ++entry) {
    const auto cached_size = (*entry)->GetPixelSize();
    const auto cached_format = (*entry)->GetPixelFormat();
    if (cached_size.width == size.width && cached_size.height == size.height && cached_format.format == format.format && cached_format.alphaMode == format.alphaMode) {
      *out = *entry;
      renderer->composite_cache.erase(entry);
      return S_OK;
    }
  }
  auto properties = D2D1::BitmapProperties1(D2D1_BITMAP_OPTIONS_TARGET, format, 96, 96);
  const HRESULT result = renderer->d2d_context->CreateBitmap(size, nullptr, 0, &properties, out);
  if (SUCCEEDED(result)) renderer->composite_bitmap_bytes += static_cast<uint64_t>(size.width) * size.height * 4;
  return result;
}

static void retire_composite_bitmap(WoxRenderer *renderer, ID2D1Bitmap1 **bitmap) {
  if (*bitmap != nullptr) renderer->retired_composite_bitmaps.push_back(*bitmap);
  *bitmap = nullptr;
}

// A hidden/resized renderer drops the pool; completed frames retain at most 16 MiB of group targets.
static void finish_composite_bitmaps(WoxRenderer *renderer, bool discard) {
  for (auto bitmap : renderer->retired_composite_bitmaps) renderer->composite_cache.push_back(bitmap);
  renderer->retired_composite_bitmaps.clear();
  uint64_t retained = 0;
  for (auto entry = renderer->composite_cache.begin(); entry != renderer->composite_cache.end();) {
    const auto size = (*entry)->GetPixelSize();
    const uint64_t bytes = static_cast<uint64_t>(size.width) * size.height * 4;
    if (discard || retained + bytes > 16 * 1024 * 1024) {
      (*entry)->Release();
      renderer->composite_bitmap_bytes -= bytes;
      entry = renderer->composite_cache.erase(entry);
    } else {
      retained += bytes;
      ++entry;
    }
  }
}

static void release_material_resources(WoxRenderer *renderer) {
  release_com(&renderer->caret_backdrop);
  renderer->caret_valid = false;
  release_com(&renderer->material_tone_effect);
  release_com(&renderer->material_blur_effect);
  release_com(&renderer->material_crop_effect);
  release_com(&renderer->material_source_bitmap);
  renderer->material_source_size = {};
}

// release_retired_image_bitmaps keeps evicted and frame-scoped resources alive until EndDraw has
// consumed every bitmap referenced by the current frame.
static void release_retired_image_bitmaps(WoxRenderer *renderer) {
  for (CachedImageBitmap &entry : renderer->frame_image_bitmaps) {
    release_com(&entry.bitmap);
  }
  renderer->frame_image_bitmaps.clear();
}

static ID2D1Bitmap1 *find_cached_image_bitmap(WoxRenderer *renderer, uint64_t image_id) {
  for (CachedImageBitmap &entry : renderer->cached_image_bitmaps) {
    if (entry.image_id == image_id) {
      entry.last_used = ++renderer->cached_image_use_serial;
      renderer->resource_stats.cache_hits++;
      return entry.bitmap;
    }
  }
  for (const CachedImageBitmap &entry : renderer->frame_image_bitmaps) {
    if (entry.image_id == image_id) {
      renderer->resource_stats.cache_hits++;
      return entry.bitmap;
    }
  }
  return nullptr;
}

// Scenes need several atlases/backgrounds resident together. One byte-bounded LRU also
// covers medium images, which previously fell between the icon and single-preview caches.
static bool cache_image_bitmap(WoxRenderer *renderer, uint64_t image_id, uint64_t byte_size, ID2D1Bitmap1 *bitmap) {
  constexpr size_t max_cached_image_count = 128;
  constexpr uint64_t max_cached_image_bytes = 32ULL * 1024ULL * 1024ULL;
  if (image_id == 0 || bitmap == nullptr || byte_size > max_cached_image_bytes) {
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
    // Retain identity until EndDraw: repeated tiles still reuse an evicted image.
    renderer->frame_image_bitmaps.push_back(*oldest);
    renderer->resource_stats.cache_evictions++;
    oldest->bitmap = nullptr;
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

static bool native_ui_font_weight(uint8_t font_weight, DWRITE_FONT_WEIGHT *native_font_weight) {
  if (native_font_weight == nullptr) {
    return false;
  }
  switch (font_weight) {
  case 0:
    *native_font_weight = DWRITE_FONT_WEIGHT_NORMAL;
    return true;
  case 1:
    *native_font_weight = DWRITE_FONT_WEIGHT_SEMI_BOLD;
    return true;
  default:
    return false;
  }
}

// create_text_format keeps drawing and measurement on identical DirectWrite settings.
static HRESULT create_text_format(WoxRenderer *renderer, float font_size, uint8_t font_weight, uint8_t font_family, uint8_t italic, IDWriteTextFormat **format) {
  if (renderer == nullptr || renderer->dwrite_factory == nullptr || format == nullptr || font_size <= 0.0f) {
    return E_INVALIDARG;
  }
  DWRITE_FONT_WEIGHT native_font_weight;
  if (!native_ui_font_weight(font_weight, &native_font_weight)) {
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

// apply_default_cjk_font keeps the Windows default aligned with Flutter without
// overriding a configured application font. YaHei UI is required because the
// print YaHei face has no Semibold; CJK then silently renders as Regular.
static HRESULT apply_default_cjk_font(IDWriteTextLayout *layout, const std::wstring &text, DWRITE_FONT_WEIGHT font_weight) {
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
    HRESULT result = layout->SetFontFamilyName(L"Microsoft YaHei UI", range);
    if (FAILED(result)) {
      return result;
    }
    result = layout->SetFontWeight(font_weight, range);
    if (FAILED(result)) {
      return result;
    }
  }
  return S_OK;
}

// Ag中あ한. Han is assigned Microsoft YaHei UI on the default UI font; Kana and
// Hangul stay on DirectWrite's fallback (Yu Gothic UI and Malgun Gothic here).
// One line box has to cover every run or the baseline moves as scripts come and go.
static const wchar_t kStableScriptSample[] = L"Ag\u4E2D\u3042\uD55C";

// layout_line_box reads the tallest baseline and the deepest descent in a layout.
static bool layout_line_box(IDWriteTextLayout *layout, float *baseline, float *height) {
  UINT32 line_count = 0;
  layout->GetLineMetrics(nullptr, 0, &line_count);
  if (line_count == 0) {
    return false;
  }
  std::vector<DWRITE_LINE_METRICS> lines(line_count);
  if (FAILED(layout->GetLineMetrics(lines.data(), line_count, &line_count))) {
    return false;
  }
  float line_baseline = 0;
  float descent = 0;
  for (UINT32 index = 0; index < line_count; index++) {
    line_baseline = std::max(line_baseline, lines[index].baseline);
    descent = std::max(descent, lines[index].height - lines[index].baseline);
  }
  *baseline = line_baseline;
  *height = line_baseline + descent;
  return true;
}

// create_text_layout_unpinned builds one layout on the same font path as drawing, without a stable line box.
static HRESULT create_text_layout_unpinned(WoxRenderer *renderer, const std::wstring &text, float font_size, uint8_t font_weight, uint8_t font_family, uint8_t italic, float width, float height, IDWriteTextLayout **layout) {
  IDWriteTextFormat *format = nullptr;
  HRESULT result = create_text_format(renderer, font_size, font_weight, font_family, italic, &format);
  if (FAILED(result)) {
    return result;
  }
  result = renderer->dwrite_factory->CreateTextLayout(text.c_str(), static_cast<UINT32>(text.size()), format, width, height, layout);
  format->Release();
  DWRITE_FONT_WEIGHT native_font_weight;
  if (SUCCEEDED(result) && renderer->uses_default_font_family && font_family == 0 && native_ui_font_weight(font_weight, &native_font_weight)) {
    result = apply_default_cjk_font(*layout, text, native_font_weight);
    if (FAILED(result)) {
      release_com(layout);
    }
  }
  return result;
}

// stable_script_line_box returns the line box of the mixed-script sample for this face.
static bool stable_script_line_box(WoxRenderer *renderer, float font_size, uint8_t font_weight, uint8_t font_family, uint8_t italic, float *baseline, float *height) {
  const bool cjk_override = renderer->uses_default_font_family && font_family == 0;
  const int slot_count = static_cast<int>(sizeof(renderer->stable_line_boxes) / sizeof(renderer->stable_line_boxes[0]));
  for (const StableLineBox &slot : renderer->stable_line_boxes) {
    if (slot.occupied && slot.family == renderer->font_family && slot.font_size == font_size && slot.weight == font_weight && slot.italic == italic && slot.font_slot == font_family && slot.cjk_override == cjk_override) {
      *baseline = slot.baseline;
      *height = slot.height;
      return true;
    }
  }
  IDWriteTextLayout *layout = nullptr;
  HRESULT result = create_text_layout_unpinned(renderer, kStableScriptSample, font_size, font_weight, font_family, italic, 1000000.0f, 1000000.0f, &layout);
  if (FAILED(result)) {
    return false;
  }
  const bool measured = layout_line_box(layout, baseline, height);
  layout->Release();
  if (!measured) {
    return false;
  }
  StableLineBox &slot = renderer->stable_line_boxes[renderer->stable_line_box_cursor % slot_count];
  renderer->stable_line_box_cursor++;
  slot.family = renderer->font_family;
  slot.font_size = font_size;
  slot.weight = font_weight;
  slot.italic = italic;
  slot.font_slot = font_family;
  slot.cjk_override = cjk_override;
  slot.baseline = *baseline;
  slot.height = *height;
  slot.occupied = true;
  return true;
}

// pin_stable_script_line_box keeps Han, Kana, Hangul, and Latin on one baseline.
// A line otherwise adopts whichever fallback face it happens to use: YaHei UI is
// shorter than Segoe UI, and Malgun Gothic is taller, so the glyphs jump as the
// script mix changes.
static void pin_stable_script_line_box(WoxRenderer *renderer, IDWriteTextLayout *layout, float font_size, uint8_t font_weight, uint8_t font_family, uint8_t italic) {
  float baseline = 0;
  float height = 0;
  if (!stable_script_line_box(renderer, font_size, font_weight, font_family, italic, &baseline, &height)) {
    return;
  }
  float content_baseline = 0;
  float content_height = 0;
  if (!layout_line_box(layout, &content_baseline, &content_height)) {
    return;
  }
  const float descent = std::max(height - baseline, content_height - content_baseline);
  baseline = std::max(baseline, content_baseline);
  height = baseline + descent;
  if (content_baseline + 0.01f < baseline || content_height + 0.01f < height) {
    layout->SetLineSpacing(DWRITE_LINE_SPACING_METHOD_UNIFORM, height, baseline);
  }
}

// create_text_layout keeps drawing and measurement on the same font fallback path.
static HRESULT create_text_layout(WoxRenderer *renderer, const std::wstring &text, float font_size, uint8_t font_weight, uint8_t font_family, uint8_t italic, float width, float height, IDWriteTextLayout **layout) {
  HRESULT result = create_text_layout_unpinned(renderer, text, font_size, font_weight, font_family, italic, width, height, layout);
  if (SUCCEEDED(result)) {
    pin_stable_script_line_box(renderer, *layout, font_size, font_weight, font_family, italic);
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
    wox_renderer_clear_clip(renderer);
    while (!renderer->composite_layers.empty()) wox_renderer_end_edge_fade(renderer);
    if (renderer->damage_clip_active) {
      renderer->d2d_context->PopAxisAlignedClip();
      renderer->damage_clip_active = false;
    }
    renderer->d2d_context->EndDraw();
  }
  release_retired_image_bitmaps(renderer);
  finish_composite_bitmaps(renderer, true);
  if (renderer->d2d_context != nullptr) {
    renderer->d2d_context->SetTarget(nullptr);
  }
  release_com(&renderer->brush);
  clear_cached_image_bitmaps(renderer);
  release_material_resources(renderer);
  release_com(&renderer->overlay_target_bitmap);
  release_com(&renderer->target_bitmap);
  release_com(&renderer->d2d_context);
  release_com(&renderer->d2d_device);
  release_com(&renderer->d2d_factory);
  release_com(&renderer->dwrite_factory);
  release_com(&renderer->overlay_visual);
  release_com(&renderer->composition_visual);
  release_com(&renderer->window_clip);
 release_com(&renderer->composition_root);
  release_com(&renderer->composition_target);
  release_com(&renderer->composition_device);
  release_com(&renderer->overlay_swap_chain);
  release_com(&renderer->swap_chain);
  release_shared_d3d_device(&renderer->device);
  delete renderer;
}

extern "C" int32_t wox_renderer_create(uintptr_t window_handle, uint32_t width, uint32_t height, int32_t enable_embedded_surface_overlay, int32_t force_warp, WoxRenderer **renderer_out) {
  if (window_handle == 0 || renderer_out == nullptr) {
    return E_INVALIDARG;
  }

  auto *renderer = new WoxRenderer();
  *renderer_out = nullptr;
  renderer->embedded_surface_overlay_enabled = enable_embedded_surface_overlay != 0;
  renderer->width = width == 0 ? 1 : width;
  renderer->height = height == 0 ? 1 : height;

  HRESULT result = acquire_shared_d3d_device(force_warp != 0, &renderer->device, &renderer->uses_warp, &renderer->feature_level);
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
    adapter->GetDesc(&renderer->adapter_description);
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
    renderer->last_composition_commit_result = result;
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

// wox_renderer_get_diagnostics returns a compact snapshot suitable for user-provided logs.
extern "C" int32_t wox_renderer_get_diagnostics(WoxRenderer *renderer, char *buffer, uint32_t buffer_size) {
  if (renderer == nullptr || buffer == nullptr || buffer_size == 0) {
    return E_INVALIDARG;
  }
  char adapter_name[256] = {};
  WideCharToMultiByte(CP_UTF8, 0, renderer->adapter_description.Description, -1, adapter_name, sizeof(adapter_name), nullptr, nullptr);
  const HRESULT removed_reason = renderer->device == nullptr ? E_UNEXPECTED : renderer->device->GetDeviceRemovedReason();
  std::snprintf(
      buffer,
      buffer_size,
      "adapter=%s vendorId=0x%04X deviceId=0x%04X subsystemId=0x%08X revision=%u luid=%08X:%08X warp=%d featureLevel=0x%04X size=%ux%u swapEffect=flip_sequential bufferCount=2 alpha=premultiplied compositionCommit=0x%08X endDraw=0x%08X present=0x%08X deviceRemovedReason=0x%08X",
      adapter_name,
      renderer->adapter_description.VendorId,
      renderer->adapter_description.DeviceId,
      renderer->adapter_description.SubSysId,
      renderer->adapter_description.Revision,
      static_cast<uint32_t>(renderer->adapter_description.AdapterLuid.HighPart),
      renderer->adapter_description.AdapterLuid.LowPart,
      renderer->uses_warp,
      static_cast<uint32_t>(renderer->feature_level),
      renderer->width,
      renderer->height,
      static_cast<uint32_t>(renderer->last_composition_commit_result),
      static_cast<uint32_t>(renderer->last_end_draw_result),
      static_cast<uint32_t>(renderer->last_present_result),
      static_cast<uint32_t>(removed_reason));
  buffer[buffer_size - 1] = '\0';
  return S_OK;
}

extern "C" int32_t wox_renderer_resize(WoxRenderer *renderer, uint32_t width, uint32_t height) {
  if (renderer == nullptr || width == 0 || height == 0) {
    return S_OK;
  }
  if (width == renderer->width && height == renderer->height) {
    return S_OK;
  }

  renderer->d2d_context->SetTarget(nullptr);
  finish_composite_bitmaps(renderer, true);
  renderer->caret_valid = false;
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
  clear_cached_image_bitmaps(renderer);

  IDXGIDevice3 *dxgi_device = nullptr;
  finish_composite_bitmaps(renderer, true);
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
  release_material_resources(renderer);
  finish_composite_bitmaps(renderer, true);
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
  renderer->resource_stats = {};
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

extern "C" void wox_renderer_invalidate_caret(WoxRenderer *renderer) {
  renderer->caret_valid = false;
}

// Capture after text/background paint and before the caret, including when it is hidden.
// Failed allocation/copy simply disables the fast path until a later normal frame.
extern "C" void wox_renderer_capture_caret(WoxRenderer *renderer, float x, float y, float width, float height) {
  renderer->caret_valid = false;
  if (!renderer->frame_open || renderer->overlay_active) return;
  const float scale = renderer->scale;
  const D2D1_SIZE_U target = renderer->target_bitmap->GetPixelSize();
  const auto clamp_pixel = [](float value, uint32_t limit) {
    return static_cast<uint32_t>(std::min<float>(std::max(0.0f, value), static_cast<float>(limit)));
  };
  const D2D1_RECT_U source = {
    clamp_pixel(std::floor(x * scale), target.width), clamp_pixel(std::floor(y * scale), target.height),
    clamp_pixel(std::ceil((x + width) * scale), target.width), clamp_pixel(std::ceil((y + height) * scale), target.height)
  };
  if (source.right <= source.left || source.bottom <= source.top) return;
  // Partial updates outside this caret have not restored its background this frame.
  if (renderer->present_dirty && (source.left < static_cast<uint32_t>(renderer->present_dirty_rect.left) ||
      source.top < static_cast<uint32_t>(renderer->present_dirty_rect.top) ||
      source.right > static_cast<uint32_t>(renderer->present_dirty_rect.right) ||
      source.bottom > static_cast<uint32_t>(renderer->present_dirty_rect.bottom))) return;
  const D2D1_SIZE_U size = D2D1::SizeU(source.right - source.left, source.bottom - source.top);
  if (renderer->caret_backdrop != nullptr) {
    const auto old = renderer->caret_backdrop->GetPixelSize();
    if (old.width != size.width || old.height != size.height) release_com(&renderer->caret_backdrop);
  }
  if (renderer->caret_backdrop == nullptr) {
    const auto properties = D2D1::BitmapProperties1(D2D1_BITMAP_OPTIONS_NONE, renderer->target_bitmap->GetPixelFormat(), 96.0f, 96.0f);
    if (FAILED(renderer->d2d_context->CreateBitmap(size, nullptr, 0, &properties, &renderer->caret_backdrop))) return;
  }
  if (FAILED(renderer->d2d_context->Flush())) return;
  const D2D1_POINT_2U origin = {};
  if (FAILED(renderer->caret_backdrop->CopyFromBitmap(&origin, renderer->target_bitmap, &source))) return;
  renderer->caret_rect = D2D1::RectF(x, y, x + width, y + height);
  renderer->caret_pixels = source;
  renderer->caret_scale = scale;
  renderer->caret_valid = true;
}

// S_FALSE requests normal rendering before any pixels are touched. Restore with SOURCE_COPY
// so translucent backgrounds do not accumulate alpha across successive blink phases.
extern "C" int32_t wox_renderer_repaint_caret(WoxRenderer *renderer, float scale, float x, float y, float width, float height, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  const auto rect = renderer->caret_rect;
  if (!renderer->caret_valid || renderer->caret_backdrop == nullptr || renderer->caret_scale != scale ||
      rect.left != x || rect.top != y || rect.right != x + width || rect.bottom != y + height) return S_FALSE;
  const auto pixels = renderer->caret_pixels;
  // Snap damage outward to physical pixels so restoring antialiased edge pixels is exact.
  HRESULT result = wox_renderer_begin_frame(renderer, scale, pixels.left / scale, pixels.top / scale,
      (pixels.right - pixels.left) / scale, (pixels.bottom - pixels.top) / scale, 0, 0, 0, 0);
  if (FAILED(result)) return result;
  renderer->d2d_context->SetTransform(D2D1::Matrix3x2F::Identity());
  const D2D1_POINT_2F offset = D2D1::Point2F(static_cast<float>(pixels.left), static_cast<float>(pixels.top));
  renderer->d2d_context->DrawImage(renderer->caret_backdrop, &offset, nullptr, D2D1_INTERPOLATION_MODE_NEAREST_NEIGHBOR, D2D1_COMPOSITE_MODE_SOURCE_COPY);
  // begin_frame cleared the counters. Reusing the captured backdrop is a real cache hit;
  // the caret fill below is new paint and must not increment this.
  renderer->resource_stats.cache_hits++;
  renderer->d2d_context->SetTransform(D2D1::Matrix3x2F::Scale(scale, scale));
  return wox_renderer_fill_rounded_rect(renderer, x, y, width, height, 0, red, green, blue, alpha);
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

extern "C" int32_t wox_renderer_draw_image(WoxRenderer *renderer, uint64_t image_id, const uint8_t *pixels, uint32_t image_width, uint32_t image_height, uint32_t row_stride, uint8_t pixel_format, float x, float y, float width, float height, float rotation_radians, float corner_radius, float source_x, float source_y, float source_width, float source_height) {
  if (renderer == nullptr || !renderer->frame_open || pixels == nullptr || image_width == 0 || image_height == 0 || row_stride < image_width * 4 || pixel_format > 1 || width <= 0.0f || height <= 0.0f) {
    return E_INVALIDARG;
  }

  const bool bgra_opaque = pixel_format == 1;
  D2D1_BITMAP_PROPERTIES1 properties = D2D1::BitmapProperties1(
      D2D1_BITMAP_OPTIONS_NONE,
      D2D1::PixelFormat(bgra_opaque ? DXGI_FORMAT_B8G8R8A8_UNORM : DXGI_FORMAT_R8G8B8A8_UNORM,
                        bgra_opaque ? D2D1_ALPHA_MODE_IGNORE : D2D1_ALPHA_MODE_PREMULTIPLIED),
      96.0f,
      96.0f);
  ID2D1Bitmap1 *bitmap = nullptr;
  bool release_bitmap = true;
  const uint64_t image_bytes = static_cast<uint64_t>(row_stride) * image_height;
  if (image_id != 0 && (bitmap = find_cached_image_bitmap(renderer, image_id)) != nullptr) {
    release_bitmap = false;
  } else {
    HRESULT result = renderer->d2d_context->CreateBitmap(D2D1::SizeU(image_width, image_height), pixels, row_stride, &properties, &bitmap);
    if (FAILED(result)) {
      return result;
    }
    renderer->resource_stats.image_creates++;
    renderer->resource_stats.image_uploads++;
    if (cache_image_bitmap(renderer, image_id, image_bytes, bitmap)) {
      release_bitmap = false;
    } else if (image_id != 0) {
      renderer->frame_image_bitmaps.push_back(CachedImageBitmap{image_id, image_bytes, 0, bitmap});
      release_bitmap = false;
    }
  }
  HRESULT result = S_OK;
  D2D1_MATRIX_3X2_F transform;
  renderer->d2d_context->GetTransform(&transform);
  D2D1_RECT_F destination = {x, y, x + width, y + height};
  // Snap only axis-aligned edges, in physical target coordinates. Rounding local
  // geometry before an affine map makes equivalent snapshot/rebased draws differ.
  if (rotation_radians == 0.0f && transform._12 == 0.0f && transform._21 == 0.0f) {
    const auto snap = [](float value, float scale, float offset) {
      if (scale == 0.0f) return value;
      const float result = (std::round(value * scale + offset) - offset) / scale;
      return std::isfinite(result) ? result : value;
    };
    destination = {snap(x, transform._11, transform._31), snap(y, transform._22, transform._32),
                   snap(x + width, transform._11, transform._31), snap(y + height, transform._22, transform._32)};
  }
  const D2D1_RECT_F source = {source_x, source_y, source_x + source_width, source_y + source_height};
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

// ensure_material_resources lazily creates the crop -> Gaussian blur -> tone graph and grows the
// backdrop copy to at least the requested pixel size. It only grows, so a panel and a
// tooltip of different sizes in the same frame share one bitmap instead of reallocating.
static HRESULT ensure_material_resources(WoxRenderer *renderer, uint32_t width, uint32_t height) {
  if (renderer->material_source_bitmap == nullptr || renderer->material_source_size.width < width || renderer->material_source_size.height < height) {
    release_com(&renderer->material_source_bitmap);
    const D2D1_SIZE_U size = D2D1::SizeU(std::max(width, renderer->material_source_size.width), std::max(height, renderer->material_source_size.height));
    renderer->material_source_size = {};
    // Same pixel format as the swap chain: CopyFromBitmap does not convert.
    D2D1_BITMAP_PROPERTIES1 properties = {};
    properties.pixelFormat.format = DXGI_FORMAT_B8G8R8A8_UNORM;
    properties.pixelFormat.alphaMode = D2D1_ALPHA_MODE_PREMULTIPLIED;
    properties.dpiX = 96.0f;
    properties.dpiY = 96.0f;
    properties.bitmapOptions = D2D1_BITMAP_OPTIONS_NONE;
    HRESULT result = renderer->d2d_context->CreateBitmap(size, nullptr, 0, &properties, &renderer->material_source_bitmap);
    if (FAILED(result)) {
      return result;
    }
    renderer->material_source_size = size;
  }
  if (renderer->material_crop_effect == nullptr) {
    HRESULT result = renderer->d2d_context->CreateEffect(CLSID_D2D1Crop, &renderer->material_crop_effect);
    if (FAILED(result)) {
      return result;
    }
  }
  if (renderer->material_blur_effect == nullptr) {
    HRESULT result = renderer->d2d_context->CreateEffect(CLSID_D2D1GaussianBlur, &renderer->material_blur_effect);
    if (FAILED(result)) {
      return result;
    }
    renderer->material_blur_effect->SetInputEffect(0, renderer->material_crop_effect);
    // SPEED lets Direct2D downsample internally for large kernels, which is what a frosted
    // backdrop wants anyway. HARD keeps the output inside the copied region; the margin the
    // caller adds around the surface is what keeps its edge from sampling transparent padding.
    // MinGW's d2d1effects.h lacks the D2D1_GAUSSIANBLUR_OPTIMIZATION enum; SPEED is 0 in the Windows SDK.
    const UINT32 optimization_speed = 0;
    const UINT32 border_mode = D2D1_BORDER_MODE_HARD;
    renderer->material_blur_effect->SetValue(D2D1_GAUSSIANBLUR_PROP_OPTIMIZATION, optimization_speed);
    renderer->material_blur_effect->SetValue(D2D1_GAUSSIANBLUR_PROP_BORDER_MODE, border_mode);
  }
  if (renderer->material_tone_effect == nullptr) {
    HRESULT result = renderer->d2d_context->CreateEffect(CLSID_D2D1ColorMatrix, &renderer->material_tone_effect);
    if (FAILED(result)) {
      return result;
    }
    renderer->material_tone_effect->SetInputEffect(0, renderer->material_blur_effect);
  }
  return S_OK;
}

// Clip suspension keeps the authored geometry alive while a backdrop snapshot flushes prior drawing.
static void suspend_renderer_clip(WoxRenderer *renderer) {
  if (!renderer->clip_active) return;
  if (renderer->clip_polygon != nullptr) renderer->d2d_context->PopLayer();
  else renderer->d2d_context->PopAxisAlignedClip();
  renderer->clip_active = false;
}

// Clips are recorded in target logical coordinates and never inherit the current paint matrix.
static void resume_renderer_clip(WoxRenderer *renderer) {
  D2D1_MATRIX_3X2_F transform;
  renderer->d2d_context->GetTransform(&transform);
  renderer->d2d_context->SetTransform(D2D1::Matrix3x2F::Scale(renderer->scale, renderer->scale));
  if (renderer->clip_polygon != nullptr) {
    auto layer = D2D1::LayerParameters1(D2D1::InfiniteRect(), renderer->clip_polygon);
    layer.layerOptions = D2D1_LAYER_OPTIONS1_INITIALIZE_FROM_BACKGROUND;
    const auto blend = renderer->d2d_context->GetPrimitiveBlend();
    renderer->d2d_context->SetPrimitiveBlend(D2D1_PRIMITIVE_BLEND_COPY);
    renderer->d2d_context->PushLayer(&layer, nullptr);
    renderer->d2d_context->SetPrimitiveBlend(blend);
  } else {
    renderer->d2d_context->PushAxisAlignedClip(renderer->clip_rect, D2D1_ANTIALIAS_MODE_PER_PRIMITIVE);
  }
  renderer->d2d_context->SetTransform(transform);
  renderer->clip_active = true;
}

// blur_floating_material_backdrop replaces the pixels inside the rounded surface with a
// blurred copy of what the frame has drawn there so far. Direct2D runs on the selected device:
// copy the region (plus margin) out of the back buffer, run it through the blur graph, and
// draw it back inside the transformed rounded shape. Every
// step that can fail runs before the target is touched, so a failure leaves it as it was.
static HRESULT blur_floating_material_backdrop(WoxRenderer *renderer, float x, float y, float width, float height, float radius, float blur_sigma, float blur_margin, float brightness, float saturation, uint8_t tint_red, uint8_t tint_green, uint8_t tint_blue) {
  const float scale = renderer->scale;
  D2D1::Matrix3x2F transform;
  renderer->d2d_context->GetTransform(&transform);
  const D2D1_SIZE_U target_size = renderer->target_bitmap->GetPixelSize();
  const auto clamp_pixel = [](float value, uint32_t limit) {
    return static_cast<uint32_t>(std::min<float>(std::max(0.0f, value), static_cast<float>(limit)));
  };
  // Sampling is in physical target axes, not the surface's rotated/scaled local axes.
  const D2D1_POINT_2F points[] = {
      transform.TransformPoint(D2D1::Point2F(x, y)), transform.TransformPoint(D2D1::Point2F(x + width, y)),
      transform.TransformPoint(D2D1::Point2F(x + width, y + height)), transform.TransformPoint(D2D1::Point2F(x, y + height))};
  float min_x = points[0].x, max_x = points[0].x, min_y = points[0].y, max_y = points[0].y;
  for (const auto &point : points) {
    min_x = std::min(min_x, point.x); max_x = std::max(max_x, point.x);
    min_y = std::min(min_y, point.y); max_y = std::max(max_y, point.y);
  }
  const uint32_t left = clamp_pixel(std::floor(min_x - blur_margin * scale), target_size.width);
  const uint32_t top = clamp_pixel(std::floor(min_y - blur_margin * scale), target_size.height);
  const uint32_t right = clamp_pixel(std::ceil(max_x + blur_margin * scale), target_size.width);
  const uint32_t bottom = clamp_pixel(std::ceil(max_y + blur_margin * scale), target_size.height);
  if (right <= left || bottom <= top) {
    return S_OK;
  }
  const uint32_t source_width = right - left;
  const uint32_t source_height = bottom - top;

  HRESULT result = ensure_material_resources(renderer, source_width, source_height);
  if (FAILED(result)) {
    return result;
  }
  // Suppress bright, saturated icon blobs without changing the surface's authored alpha.
  const auto tone = floating_material_tone_matrix(tint_red / 255.0f, tint_green / 255.0f, tint_blue / 255.0f, brightness, saturation);
  result = renderer->material_tone_effect->SetValue(D2D1_COLORMATRIX_PROP_COLOR_MATRIX, tone);
  if (SUCCEEDED(result)) {
    result = renderer->material_tone_effect->SetValue(D2D1_COLORMATRIX_PROP_CLAMP_OUTPUT, TRUE);
  }
  if (FAILED(result)) {
    return result;
  }
  // The copy reads the back buffer, so every primitive recorded before the material
  // must have landed there first.
  const bool restore_clip = renderer->clip_active;
  suspend_renderer_clip(renderer);
  result = renderer->d2d_context->Flush();
  const D2D1_POINT_2U destination = D2D1::Point2U(0, 0);
  const D2D1_RECT_U source = D2D1::RectU(left, top, right, bottom);
  ID2D1Bitmap1 *backdrop = renderer->composite_layers.empty() ? renderer->target_bitmap : renderer->composite_layers.back().bitmap;
  if (SUCCEEDED(result)) result = backdrop == nullptr ? E_UNEXPECTED : renderer->material_source_bitmap->CopyFromBitmap(&destination, backdrop, &source);
  if (restore_clip) resume_renderer_clip(renderer);
  if (FAILED(result)) {
    return result;
  }
  renderer->material_crop_effect->SetInput(0, renderer->material_source_bitmap);
  // The cached bitmap may be larger than this region; crop so stale pixels beyond it never
  // bleed into the kernel.
  const D2D1_VECTOR_4F crop = {0.0f, 0.0f, static_cast<float>(source_width), static_cast<float>(source_height)};
  renderer->material_crop_effect->SetValue(D2D1_CROP_PROP_RECT, crop);
  // The bitmap is in physical pixels and drawn under an identity transform below, so the
  // standard deviation is physical as well.
  const float sigma = blur_sigma * scale;
  renderer->material_blur_effect->SetValue(D2D1_GAUSSIANBLUR_PROP_STANDARD_DEVIATION, sigma);

  // Keep the exact rounded silhouette; clearing an axis-aligned envelope damages rotated corners.
  const D2D1_RECT_F bounds = {x, y, x + width, y + height};
  const float clamped_radius = std::min(std::max(0.0f, radius), std::min(width, height) * 0.5f);
  ID2D1RoundedRectangleGeometry *shape = nullptr;
  result = renderer->d2d_factory->CreateRoundedRectangleGeometry(D2D1::RoundedRect(bounds, clamped_radius, clamped_radius), &shape);
  if (SUCCEEDED(result)) {
    // COPY on the layer replaces all four channels through its antialiased mask, rather than
    // source-over compositing the blurred translucent backdrop onto the original a second time.
    const D2D1_POINT_2F offset = D2D1::Point2F(static_cast<float>(left), static_cast<float>(top));
    D2D1_LAYER_PARAMETERS1 layer = {};
    layer.contentBounds = D2D1::InfiniteRect();
    layer.maskAntialiasMode = D2D1_ANTIALIAS_MODE_PER_PRIMITIVE;
    layer.maskTransform = D2D1::Matrix3x2F::Identity();
    layer.opacity = 1.0f;
    layer.geometricMask = shape;
    layer.layerOptions = D2D1_LAYER_OPTIONS1_INITIALIZE_FROM_BACKGROUND;
    const auto blend = renderer->d2d_context->GetPrimitiveBlend();
    renderer->d2d_context->SetPrimitiveBlend(D2D1_PRIMITIVE_BLEND_COPY);
    renderer->d2d_context->PushLayer(&layer, nullptr);
    renderer->d2d_context->SetTransform(D2D1::Matrix3x2F::Identity());
    renderer->d2d_context->DrawImage(renderer->material_tone_effect, &offset, nullptr, D2D1_INTERPOLATION_MODE_LINEAR, D2D1_COMPOSITE_MODE_SOURCE_COPY);
    renderer->d2d_context->SetTransform(transform);
    renderer->d2d_context->PopLayer();
    renderer->d2d_context->SetPrimitiveBlend(blend);
  }
  release_com(&shape);
  // Drop the bitmap reference held by the graph so a later resize can release it promptly.
  renderer->material_crop_effect->SetInput(0, nullptr);
  return result;
}

// wox_renderer_floating_material realises DisplayList.FloatingMaterial on Windows: blur the
// main surface under the rounded rectangle in place, then paint the theme tint and hairline
// edge over it. It must run after the content beneath the surface and before the surface
// content, which is the order the display list records. WARP must use the same blur and
// tone graph: tint alone leaves sharp background text showing through floating controls.
// The overlay surface has nothing beneath the panel to sample (WebView pixels live in a
// sibling visual), so its tint remains opaque instead.
extern "C" int32_t wox_renderer_floating_material(WoxRenderer *renderer, float x, float y, float width, float height, float radius, float blur_sigma, float blur_margin, float brightness, float saturation, uint8_t tint_red, uint8_t tint_green, uint8_t tint_blue, uint8_t tint_alpha, uint8_t edge_red, uint8_t edge_green, uint8_t edge_blue, uint8_t edge_alpha) {
  if (renderer == nullptr || !renderer->frame_open || renderer->brush == nullptr || renderer->target_bitmap == nullptr) {
    return E_UNEXPECTED;
  }
  if (width <= 0.0f || height <= 0.0f) {
    return E_INVALIDARG;
  }
  if (!renderer->overlay_active && (blur_sigma > 0.0f || brightness != 1.0f || saturation != 1.0f)) {
    const HRESULT result = blur_floating_material_backdrop(renderer, x, y, width, height, radius, blur_sigma, blur_margin, brightness, saturation, tint_red, tint_green, tint_blue);
    // A lost device must surface through the normal recovery path; any other failure only
    // costs the blur, and the tint below still reads as a panel.
    if (FAILED(result) && (renderer->test_offscreen || FAILED(renderer->device->GetDeviceRemovedReason()))) {
      return result;
    }
  }
  if (renderer->overlay_active && tint_alpha != 0) {
    tint_alpha = 255;
  }
  if (tint_alpha != 0) {
    const int32_t result = wox_renderer_fill_rounded_rect(renderer, x, y, width, height, radius, tint_red, tint_green, tint_blue, tint_alpha);
    if (FAILED(result)) {
      return result;
    }
  }
  if (edge_alpha != 0) {
    return wox_renderer_stroke_rounded_rect(renderer, x, y, width, height, radius, 1.0f, edge_red, edge_green, edge_blue, edge_alpha);
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
  ID2D1PathGeometry *clip_polygon = renderer->clip_polygon;
  if (clip_polygon != nullptr) clip_polygon->AddRef();
  wox_renderer_clear_clip(renderer);
  if (renderer->damage_clip_active) {
    renderer->d2d_context->PopAxisAlignedClip();
    renderer->damage_clip_active = false;
  }
  result = renderer->d2d_context->EndDraw();
  if (FAILED(result)) {
    release_com(&clip_polygon);
    return result;
  }
  renderer->d2d_context->SetTarget(renderer->overlay_target_bitmap);
  renderer->d2d_context->BeginDraw();
  const D2D1_COLOR_F transparent = D2D1::ColorF(0, 0.0f);
  renderer->d2d_context->Clear(&transparent);
  renderer->overlay_active = true;
  if (restore_clip) {
    renderer->clip_rect = clip_rect;
    renderer->clip_polygon = clip_polygon;
    resume_renderer_clip(renderer);
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

// Geometry is local logical space; display scaling is applied exactly once at this boundary.
extern "C" int32_t wox_renderer_set_transform(WoxRenderer *renderer, float a, float b, float c, float d, float tx, float ty) {
  if (renderer == nullptr || !renderer->frame_open) return E_UNEXPECTED;
  const auto matrix = D2D1::Matrix3x2F(a, b, c, d, tx, ty) * D2D1::Matrix3x2F::Scale(renderer->scale, renderer->scale);
  if (!std::isfinite(matrix._11) || !std::isfinite(matrix._12) || !std::isfinite(matrix._21) || !std::isfinite(matrix._22) || !std::isfinite(matrix._31) || !std::isfinite(matrix._32)) return E_INVALIDARG;
  renderer->d2d_context->SetTransform(matrix);
  return S_OK;
}

// Recorded clips are already in target logical coordinates, independent of the current paint matrix.
extern "C" int32_t wox_renderer_set_clip_rect(WoxRenderer *renderer, float x, float y, float width, float height) {
  if (renderer == nullptr || !renderer->frame_open) {
    return E_UNEXPECTED;
  }
  wox_renderer_clear_clip(renderer);
  const float clipped_width = width > 0.0f ? width : 0.0f;
  const float clipped_height = height > 0.0f ? height : 0.0f;
  const D2D1_RECT_F rect = {x, y, x + clipped_width, y + clipped_height};
  renderer->clip_rect = rect;
  resume_renderer_clip(renderer);
  return S_OK;
}

// Axis-aligned D2D clips only keep a rotated rectangle's envelope. A path layer preserves its silhouette.
extern "C" int32_t wox_renderer_set_clip_polygon(WoxRenderer *renderer, const float *points, int32_t point_count) {
  if (renderer == nullptr || !renderer->frame_open) return E_UNEXPECTED;
  if (points == nullptr || point_count < 3) return E_INVALIDARG;
  for (int32_t i = 0; i < point_count; i++) {
    if (!std::isfinite(points[i * 2]) || !std::isfinite(points[i * 2 + 1])) return E_INVALIDARG;
  }
  ID2D1PathGeometry *geometry = nullptr;
  ID2D1GeometrySink *sink = nullptr;
  HRESULT result = renderer->d2d_factory->CreatePathGeometry(&geometry);
  if (SUCCEEDED(result)) result = geometry->Open(&sink);
  if (SUCCEEDED(result)) {
    sink->BeginFigure(D2D1::Point2F(points[0], points[1]), D2D1_FIGURE_BEGIN_FILLED);
    for (int32_t i = 1; i < point_count; i++) sink->AddLine(D2D1::Point2F(points[i * 2], points[i * 2 + 1]));
    sink->EndFigure(D2D1_FIGURE_END_CLOSED);
    result = sink->Close();
  }
  release_com(&sink);
  if (FAILED(result)) {
    release_com(&geometry);
    return result;
  }
  wox_renderer_clear_clip(renderer);
  renderer->clip_polygon = geometry;
  resume_renderer_clip(renderer);
  return S_OK;
}

extern "C" int32_t wox_renderer_clear_clip(WoxRenderer *renderer) {
  if (renderer == nullptr || !renderer->frame_open) {
    return E_UNEXPECTED;
  }
  if (renderer->clip_active) {
    suspend_renderer_clip(renderer);
    release_com(&renderer->clip_polygon);
  }
  return S_OK;
}

// Only backdrop-reading groups need an explicit target: D2D's pending layer content is not readable.
static HRESULT begin_composite(WoxRenderer *renderer, float x, float y, float width, float height, float opacity, ID2D1LinearGradientBrush *mask, bool backdrop) {
  wox_renderer_clear_clip(renderer);
  CompositeLayer group;
  group.bounds = D2D1::RectF(x, y, x + width, y + height);
  group.opacity = opacity;
  group.mask = mask;
  renderer->d2d_context->GetTransform(&group.transform);
  if (mask != nullptr) mask->AddRef();
  if (backdrop) {
    group.parent = renderer->composite_layers.empty() ? (renderer->overlay_active ? renderer->overlay_target_bitmap : renderer->target_bitmap) : renderer->composite_layers.back().bitmap;
    // The encoder marks all ancestors of a material, so no opaque native layer can hide its pixels.
    if (group.parent == nullptr) {
      release_com(&group.mask);
      return E_UNEXPECTED;
    }
    HRESULT result = acquire_composite_bitmap(renderer, group.parent, &group.bitmap);
    if (SUCCEEDED(result)) result = renderer->d2d_context->Flush();
    if (SUCCEEDED(result)) result = group.bitmap->CopyFromBitmap(nullptr, group.parent, nullptr);
    if (FAILED(result)) {
      retire_composite_bitmap(renderer, &group.bitmap);
      release_com(&group.mask);
      return result;
    }
    renderer->d2d_context->SetTarget(group.bitmap);
  } else {
    auto layer = D2D1::LayerParameters1(group.bounds, nullptr, D2D1_ANTIALIAS_MODE_PER_PRIMITIVE, D2D1::Matrix3x2F::Identity(), opacity, mask);
    renderer->d2d_context->PushLayer(&layer, nullptr);
  }
  renderer->composite_layers.push_back(group);
  return S_OK;
}

// A layer opacity brush fades only its content, preserving the translucent backdrop.
extern "C" int32_t wox_renderer_begin_edge_fade(WoxRenderer *renderer, float x, float y, float width, float height, float top, float bottom, int32_t backdrop) {
  if (renderer == nullptr || !renderer->frame_open || height <= 0) return E_INVALIDARG;
  D2D1_GRADIENT_STOP stops[] = {
      {0, D2D1::ColorF(1, 1, 1, top > 0 ? 0.0f : 1.0f)},
      {top / height, D2D1::ColorF(1, 1, 1, 1)},
      {1 - bottom / height, D2D1::ColorF(1, 1, 1, 1)},
      {1, D2D1::ColorF(1, 1, 1, bottom > 0 ? 0.0f : 1.0f)}};
  ID2D1GradientStopCollection *collection = nullptr;
  ID2D1LinearGradientBrush *mask = nullptr;
  HRESULT result = renderer->d2d_context->CreateGradientStopCollection(stops, 4, D2D1_GAMMA_1_0, D2D1_EXTEND_MODE_CLAMP, &collection);
  if (SUCCEEDED(result)) {
    const auto gradient = D2D1::LinearGradientBrushProperties(D2D1::Point2F(x, y), D2D1::Point2F(x, y + height));
    result = renderer->d2d_context->CreateLinearGradientBrush(gradient, collection, &mask);
  }
  if (SUCCEEDED(result)) {
    result = begin_composite(renderer, x, y, width, height, 1, mask, backdrop != 0);
  }
  if (mask != nullptr) mask->Release();
  if (collection != nullptr) collection->Release();
  return result;
}

// Uniform alpha applies to the completed layer, preserving opaque overlap inside it.
extern "C" int32_t wox_renderer_begin_opacity(WoxRenderer *renderer, float x, float y, float width, float height, float opacity, int32_t backdrop) {
  if (renderer == nullptr || !renderer->frame_open) return E_UNEXPECTED;
  return begin_composite(renderer, x, y, width, height, opacity, nullptr, backdrop != 0);
}

// Opacity and edge fades share a layer stack. Clear the inner clip before popping;
// Go restores the enclosing clip after the layer boundary.
extern "C" int32_t wox_renderer_end_edge_fade(WoxRenderer *renderer) {
  if (renderer == nullptr || !renderer->frame_open || renderer->composite_layers.empty()) return E_UNEXPECTED;
  wox_renderer_clear_clip(renderer);
  CompositeLayer group = renderer->composite_layers.back();
  renderer->composite_layers.pop_back();
  HRESULT result = S_OK;
  if (group.bitmap != nullptr) {
    result = renderer->d2d_context->Flush();
    D2D1_MATRIX_3X2_F transform;
    renderer->d2d_context->GetTransform(&transform);
    ID2D1Bitmap1 *before = nullptr, *mask = nullptr;
    ID2D1Effect *inside = nullptr, *outside = nullptr, *combined = nullptr;
    if (SUCCEEDED(result)) result = acquire_composite_bitmap(renderer, group.parent, &before);
    if (SUCCEEDED(result)) result = before->CopyFromBitmap(nullptr, group.parent, nullptr);
    if (SUCCEEDED(result)) result = acquire_composite_bitmap(renderer, group.parent, &mask);
    if (SUCCEEDED(result)) result = renderer->d2d_context->CreateEffect(CLSID_D2D1Composite, &inside);
    if (SUCCEEDED(result)) result = renderer->d2d_context->CreateEffect(CLSID_D2D1Composite, &outside);
    if (SUCCEEDED(result)) result = renderer->d2d_context->CreateEffect(CLSID_D2D1Composite, &combined);
    if (SUCCEEDED(result)) {
      renderer->d2d_context->SetTarget(mask);
      renderer->d2d_context->Clear(nullptr);
      renderer->d2d_context->SetTransform(group.transform);
      if (group.mask != nullptr) {
        group.mask->SetOpacity(group.opacity);
        renderer->d2d_context->FillRectangle(group.bounds, group.mask);
      } else {
        renderer->brush->SetColor(D2D1::ColorF(1, 1, 1, group.opacity));
        renderer->d2d_context->FillRectangle(group.bounds, renderer->brush);
      }
      result = renderer->d2d_context->Flush();
    }
    renderer->d2d_context->SetTarget(group.parent);
    if (SUCCEEDED(result)) {
      // Interpolate all premultiplied channels: after*mask + before*(1-mask).
      // D2D layer.opacity scales source alpha under COPY; it does not provide this interpolation.
      inside->SetInput(0, group.bitmap);
      inside->SetInput(1, mask);
      inside->SetValue(D2D1_COMPOSITE_PROP_MODE, D2D1_COMPOSITE_MODE_DESTINATION_IN);
      outside->SetInput(0, before);
      outside->SetInput(1, mask);
      outside->SetValue(D2D1_COMPOSITE_PROP_MODE, D2D1_COMPOSITE_MODE_DESTINATION_OUT);
      combined->SetInputEffect(0, outside);
      combined->SetInputEffect(1, inside);
      combined->SetValue(D2D1_COMPOSITE_PROP_MODE, D2D1_COMPOSITE_MODE_PLUS);
      renderer->d2d_context->SetTransform(D2D1::Matrix3x2F::Identity());
      renderer->d2d_context->DrawImage(combined, nullptr, nullptr, D2D1_INTERPOLATION_MODE_NEAREST_NEIGHBOR, D2D1_COMPOSITE_MODE_SOURCE_COPY);
    }
    renderer->d2d_context->SetTransform(transform);
    release_com(&combined);
    release_com(&outside);
    release_com(&inside);
    retire_composite_bitmap(renderer, &mask);
    retire_composite_bitmap(renderer, &before);
    retire_composite_bitmap(renderer, &group.bitmap);
  } else {
    renderer->d2d_context->PopLayer();
  }
  release_com(&group.mask);
  return result;
}

// Inspect the glyph runs DirectWrite shaped for the whole line. Cluster membership
// comes from the layout, including ligatures, combining marks, and fallback fonts.
class TextInkRenderer final : public IDWriteTextRenderer {
 public:
  explicit TextInkRenderer(WoxTextInkRange &ink) : ink_(ink) {}

  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID iid, void **object) override {
    if (object == nullptr) return E_POINTER;
    *object = nullptr;
    if (iid != __uuidof(IUnknown) && iid != __uuidof(IDWritePixelSnapping) &&
        iid != __uuidof(IDWriteTextRenderer)) return E_NOINTERFACE;
    *object = static_cast<IDWriteTextRenderer *>(this);
    AddRef();
    return S_OK;
  }

  // The renderer is stack-owned for the synchronous layout Draw call.
  ULONG STDMETHODCALLTYPE AddRef() override { return ++refs_; }
  ULONG STDMETHODCALLTYPE Release() override { return --refs_; }
  HRESULT STDMETHODCALLTYPE IsPixelSnappingDisabled(void *, BOOL *disabled) override {
    *disabled = TRUE;
    return S_OK;
  }
  HRESULT STDMETHODCALLTYPE GetCurrentTransform(void *, DWRITE_MATRIX *transform) override {
    *transform = {1, 0, 0, 1, 0, 0};
    return S_OK;
  }
  HRESULT STDMETHODCALLTYPE GetPixelsPerDip(void *, FLOAT *scale) override {
    *scale = 1;
    return S_OK;
  }
  HRESULT STDMETHODCALLTYPE DrawUnderline(void *, FLOAT, FLOAT, const DWRITE_UNDERLINE *, IUnknown *) override { return S_OK; }
  HRESULT STDMETHODCALLTYPE DrawStrikethrough(void *, FLOAT, FLOAT, const DWRITE_STRIKETHROUGH *, IUnknown *) override { return S_OK; }
  HRESULT STDMETHODCALLTYPE DrawInlineObject(void *, FLOAT, FLOAT, IDWriteInlineObject *, BOOL, BOOL, IUnknown *) override { return S_OK; }

  HRESULT STDMETHODCALLTYPE DrawGlyphRun(void *, FLOAT, FLOAT baseline, DWRITE_MEASURING_MODE,
      const DWRITE_GLYPH_RUN *run, const DWRITE_GLYPH_RUN_DESCRIPTION *description, IUnknown *) override {
    if (description == nullptr || description->clusterMap == nullptr || run->glyphCount == 0) return S_OK;
    const UINT32 start = static_cast<UINT32>(ink_.start);
    const UINT32 end = static_cast<UINT32>(ink_.end);
    const UINT32 run_start = description->textPosition;
    if (run_start >= end || run_start + description->stringLength <= start) return S_OK;

    enum Cluster : uint8_t { Continuation, Unselected, Selected };
    std::vector<Cluster> clusters(run->glyphCount, Continuation);
    for (UINT32 index = 0; index < description->stringLength; ++index) {
      const UINT16 glyph = description->clusterMap[index];
      if (glyph < run->glyphCount) {
        const UINT32 position = run_start + index;
        clusters[glyph] = std::max(clusters[glyph], position >= start && position < end ? Selected : Unselected);
      }
    }

    std::vector<DWRITE_GLYPH_METRICS> metrics(run->glyphCount);
    const HRESULT result = run->fontFace->GetDesignGlyphMetrics(run->glyphIndices, run->glyphCount, metrics.data(), run->isSideways);
    if (FAILED(result)) return result;
    DWRITE_FONT_METRICS font = {};
    run->fontFace->GetMetrics(&font);
    const float scale = run->fontEmSize / font.designUnitsPerEm;
    bool selected = false;
    for (UINT32 glyph = 0; glyph < run->glyphCount; ++glyph) {
      if (clusters[glyph] != Continuation) selected = clusters[glyph] == Selected;
      if (!selected) continue;
      const auto &metric = metrics[glyph];
      const float width = static_cast<float>(metric.advanceWidth) - metric.leftSideBearing - metric.rightSideBearing;
      const float height = static_cast<float>(metric.advanceHeight) - metric.topSideBearing - metric.bottomSideBearing;
      if (width <= 0 || height <= 0) continue;
      const float ascender_offset = run->glyphOffsets == nullptr ? 0 : run->glyphOffsets[glyph].ascenderOffset;
      const float top = baseline - ascender_offset + (metric.topSideBearing - metric.verticalOriginY) * scale;
      const float bottom = top + height * scale;
      const bool has_ink = ink_.bottom > ink_.top;
      ink_.top = has_ink ? std::min(ink_.top, top) : top;
      ink_.bottom = has_ink ? std::max(ink_.bottom, bottom) : bottom;
    }
    return S_OK;
  }

 private:
  WoxTextInkRange &ink_;
  ULONG refs_ = 1;
};

extern "C" int32_t wox_renderer_measure_text(WoxRenderer *renderer, const char *text, float font_size, uint8_t font_weight, uint8_t font_family, uint8_t italic, float *width, float *height, float *baseline, WoxTextInkRange *ink) {
  if (renderer == nullptr || text == nullptr || width == nullptr || height == nullptr || baseline == nullptr) {
    return E_INVALIDARG;
  }
  *width = 0.0f;
  *height = 0.0f;
  *baseline = 0.0f;
  const std::wstring wide_text = utf8_to_wide(text);
  if (ink != nullptr && (ink->start < 0 || ink->end <= ink->start || static_cast<size_t>(ink->end) > wide_text.size())) {
    return E_INVALIDARG;
  }
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
  if (SUCCEEDED(result) && ink != nullptr) {
    ink->top = ink->bottom = 0;
    TextInkRenderer ink_renderer(*ink);
    result = layout->Draw(nullptr, &ink_renderer, 0, 0);
  }
  layout->Release();
  return result;
}

// wox_renderer_measure_default_text_for_test exercises the default UI font path without a window.
extern "C" int32_t wox_renderer_measure_default_text_for_test(const char *text, float font_size, uint8_t font_weight, float *width, float *height, float *baseline) {
  if (text == nullptr || width == nullptr || height == nullptr || baseline == nullptr) {
    return E_INVALIDARG;
  }
  WoxRenderer renderer;
  HRESULT result = DWriteCreateFactory(DWRITE_FACTORY_TYPE_SHARED, __uuidof(IDWriteFactory), reinterpret_cast<IUnknown **>(&renderer.dwrite_factory));
  if (FAILED(result)) {
    return result;
  }
  renderer.font_family = L"Segoe UI";
  renderer.uses_default_font_family = true;
  result = wox_renderer_measure_text(&renderer, text, font_size, font_weight, 0, 0, width, height, baseline, nullptr);
  release_com(&renderer.dwrite_factory);
  return result;
}

extern "C" int32_t wox_renderer_end_frame(WoxRenderer *renderer) {
  if (renderer == nullptr || !renderer->frame_open) {
    return E_UNEXPECTED;
  }

  wox_renderer_clear_clip(renderer);
  // Encoding can fail inside a layer; unwind it before ending the frame.
  while (!renderer->composite_layers.empty()) wox_renderer_end_edge_fade(renderer);
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
  release_retired_image_bitmaps(renderer);
  finish_composite_bitmaps(renderer, false);
  renderer->last_end_draw_result = result;
  renderer->frame_open = false;
  if (renderer->simulate_device_removed) {
    renderer->simulate_device_removed = false;
    result = DXGI_ERROR_DEVICE_REMOVED;
  }
  if (FAILED(result)) {
    return result;
  }
  if (renderer->test_offscreen) return S_OK;
  DXGI_PRESENT_PARAMETERS parameters = {};
  if (renderer->present_dirty) {
    parameters.DirtyRectsCount = 1;
    parameters.pDirtyRects = &renderer->present_dirty_rect;
  }
  result = renderer->swap_chain->Present1(1, 0, &parameters);
  renderer->last_present_result = result;
  if (FAILED(result)) {
    return result;
  }
  if (renderer->overlay_swap_chain != nullptr) {
    result = renderer->overlay_swap_chain->Present1(1, 0, &parameters);
    renderer->last_present_result = result;
    return result;
  }
  return S_OK;
}

extern "C" int64_t wox_renderer_resident_bytes(WoxRenderer *renderer) {
  if (renderer == nullptr) {
    return 0;
  }
  const auto caret_size = renderer->caret_backdrop == nullptr ? D2D1::SizeU(0, 0) : renderer->caret_backdrop->GetPixelSize();
  return static_cast<int64_t>(renderer->cached_image_bitmap_bytes + renderer->composite_bitmap_bytes + static_cast<uint64_t>(caret_size.width) * caret_size.height * 4);
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

// The HWND region does not replace clipping the DirectComposition visual tree (including embedded overlays).
extern "C" int32_t wox_renderer_set_corner_radius(WoxRenderer *renderer, float physical_radius) {
 if (renderer == nullptr || renderer->composition_root == nullptr) return E_INVALIDARG;
 HRESULT result = S_OK;
 if (physical_radius < 0) {
  result = renderer->composition_root->SetClip(static_cast<IDCompositionClip *>(nullptr));
 } else {
  if (renderer->window_clip == nullptr) result = renderer->composition_device->CreateRectangleClip(&renderer->window_clip);
  if (FAILED(result)) return result;
  float radius = std::min(physical_radius, std::min(renderer->width, renderer->height) * 0.5f);
  auto clip = renderer->window_clip;
  result = clip->SetLeft(0.0f);
  if (SUCCEEDED(result)) result = clip->SetTop(0.0f);
  if (SUCCEEDED(result)) result = clip->SetRight(static_cast<float>(renderer->width));
  if (SUCCEEDED(result)) result = clip->SetBottom(static_cast<float>(renderer->height));
  if (SUCCEEDED(result)) result = clip->SetTopLeftRadiusX(radius);
  if (SUCCEEDED(result)) result = clip->SetTopLeftRadiusY(radius);
  if (SUCCEEDED(result)) result = clip->SetTopRightRadiusX(radius);
  if (SUCCEEDED(result)) result = clip->SetTopRightRadiusY(radius);
  if (SUCCEEDED(result)) result = clip->SetBottomLeftRadiusX(radius);
  if (SUCCEEDED(result)) result = clip->SetBottomLeftRadiusY(radius);
  if (SUCCEEDED(result)) result = clip->SetBottomRightRadiusX(radius);
  if (SUCCEEDED(result)) result = clip->SetBottomRightRadiusY(radius);
  if (SUCCEEDED(result)) result = renderer->composition_root->SetClip(clip);
 }
 if (SUCCEEDED(result)) result = renderer->composition_device->Commit();
 return result;
}

// Offscreen targets replay the Go encoder and production frame lifecycle without a HWND or presentation.
extern "C" int32_t wox_renderer_test_create_offscreen(uint32_t width, uint32_t height, WoxRenderer **out) {
  if (out == nullptr || width == 0 || height == 0) return E_INVALIDARG;
  *out = nullptr;
  auto *renderer = new WoxRenderer();
  renderer->test_offscreen = true;
  renderer->uses_warp = true;
  renderer->width = width;
  renderer->height = height;
  IDXGIDevice *dxgi = nullptr;
  HRESULT result = D3D11CreateDevice(nullptr, D3D_DRIVER_TYPE_WARP, nullptr, D3D11_CREATE_DEVICE_BGRA_SUPPORT, nullptr, 0, D3D11_SDK_VERSION, &renderer->device, nullptr, nullptr);
  if (SUCCEEDED(result)) result = renderer->device->QueryInterface(IID_IDXGIDevice, reinterpret_cast<void **>(&dxgi));
  if (SUCCEEDED(result)) result = D2D1CreateFactory(D2D1_FACTORY_TYPE_SINGLE_THREADED, __uuidof(ID2D1Factory1), reinterpret_cast<void **>(&renderer->d2d_factory));
  if (SUCCEEDED(result)) result = renderer->d2d_factory->CreateDevice(dxgi, &renderer->d2d_device);
  if (SUCCEEDED(result)) result = renderer->d2d_device->CreateDeviceContext(D2D1_DEVICE_CONTEXT_OPTIONS_NONE, &renderer->d2d_context);
  if (SUCCEEDED(result)) result = DWriteCreateFactory(DWRITE_FACTORY_TYPE_SHARED, __uuidof(IDWriteFactory), reinterpret_cast<IUnknown **>(&renderer->dwrite_factory));
  auto properties = D2D1::BitmapProperties1(D2D1_BITMAP_OPTIONS_TARGET, D2D1::PixelFormat(DXGI_FORMAT_B8G8R8A8_UNORM, D2D1_ALPHA_MODE_PREMULTIPLIED), 96, 96);
  if (SUCCEEDED(result)) result = renderer->d2d_context->CreateBitmap(D2D1::SizeU(width, height), nullptr, 0, &properties, &renderer->target_bitmap);
  if (SUCCEEDED(result)) {
    renderer->d2d_context->SetDpi(96, 96);
    result = renderer->d2d_context->CreateSolidColorBrush(D2D1::ColorF(1, 1, 1, 1), &renderer->brush);
  }
  release_com(&dxgi);
  if (FAILED(result)) {
    destroy_renderer(renderer);
    return result;
  }
  *out = renderer;
  return S_OK;
}

// Read back premultiplied RGBA so native and software assertions use identical channel ordering.
extern "C" int32_t wox_renderer_test_read_pixels(WoxRenderer *renderer, uint8_t *pixels, uint32_t length) {
  if (renderer == nullptr || !renderer->test_offscreen || renderer->frame_open || pixels == nullptr || length != static_cast<uint64_t>(renderer->width) * renderer->height * 4) return E_INVALIDARG;
  ID2D1Bitmap1 *readback = nullptr;
  auto properties = D2D1::BitmapProperties1(D2D1_BITMAP_OPTIONS_CPU_READ | D2D1_BITMAP_OPTIONS_CANNOT_DRAW, D2D1::PixelFormat(DXGI_FORMAT_B8G8R8A8_UNORM, D2D1_ALPHA_MODE_PREMULTIPLIED), 96, 96);
  HRESULT result = renderer->d2d_context->CreateBitmap(D2D1::SizeU(renderer->width, renderer->height), nullptr, 0, &properties, &readback);
  if (SUCCEEDED(result)) result = readback->CopyFromBitmap(nullptr, renderer->target_bitmap, nullptr);
  D2D1_MAPPED_RECT mapped = {};
  if (SUCCEEDED(result)) result = readback->Map(D2D1_MAP_OPTIONS_READ, &mapped);
  if (SUCCEEDED(result)) {
    for (uint32_t y = 0; y < renderer->height; y++) {
      for (uint32_t x = 0; x < renderer->width; x++) {
        const uint8_t *src = mapped.bits + y * mapped.pitch + x * 4;
        uint8_t *dst = pixels + (y * renderer->width + x) * 4;
        dst[0] = src[2]; dst[1] = src[1]; dst[2] = src[0]; dst[3] = src[3];
      }
    }
    readback->Unmap();
  }
  release_com(&readback);
  return result;
}

// Exercise production effects on a WARP bitmap without opening a window.
static int32_t test_warp_bitmap(uint8_t *pixels, int32_t size, float scale, float top, float bottom, bool material, float opacity) {
  WoxRenderer renderer;
  IDXGIDevice *dxgi = nullptr;
  ID2D1Bitmap1 *target = nullptr, *readback = nullptr;
  HRESULT result = D3D11CreateDevice(nullptr, D3D_DRIVER_TYPE_WARP, nullptr, D3D11_CREATE_DEVICE_BGRA_SUPPORT, nullptr, 0, D3D11_SDK_VERSION, &renderer.device, nullptr, nullptr);
  if (SUCCEEDED(result)) result = renderer.device->QueryInterface(IID_IDXGIDevice, reinterpret_cast<void **>(&dxgi));
  if (SUCCEEDED(result)) result = D2D1CreateFactory(D2D1_FACTORY_TYPE_SINGLE_THREADED, __uuidof(ID2D1Factory1), reinterpret_cast<void **>(&renderer.d2d_factory));
  if (SUCCEEDED(result)) result = renderer.d2d_factory->CreateDevice(dxgi, &renderer.d2d_device);
  if (SUCCEEDED(result)) result = renderer.d2d_device->CreateDeviceContext(D2D1_DEVICE_CONTEXT_OPTIONS_NONE, &renderer.d2d_context);
  D2D1_BITMAP_PROPERTIES1 properties = {};
  properties.pixelFormat = D2D1::PixelFormat(material ? DXGI_FORMAT_B8G8R8A8_UNORM : DXGI_FORMAT_R8G8B8A8_UNORM, D2D1_ALPHA_MODE_PREMULTIPLIED);
  properties.dpiX = properties.dpiY = 96;
  properties.bitmapOptions = D2D1_BITMAP_OPTIONS_TARGET;
  if (SUCCEEDED(result)) result = renderer.d2d_context->CreateBitmap(D2D1::SizeU(size, size), nullptr, 0, &properties, &target);
  if (SUCCEEDED(result)) {
    renderer.d2d_context->SetTarget(target);
    renderer.d2d_context->SetDpi(properties.dpiX, properties.dpiY);
    renderer.d2d_context->SetTransform(D2D1::Matrix3x2F::Scale(scale, scale));
    result = renderer.d2d_context->CreateSolidColorBrush(D2D1::ColorF(1, 1, 1, 1), &renderer.brush);
  }
  if (SUCCEEDED(result)) {
    renderer.frame_open = true;
    renderer.d2d_context->BeginDraw();
    renderer.uses_warp = true;
    renderer.scale = scale;
    renderer.target_bitmap = target;
    if (material) {
      const auto black = D2D1::ColorF(0, 0, 0, 1);
      renderer.d2d_context->Clear(&black);
      wox_renderer_fill_rounded_rect(&renderer, 48, 0, 48, 96, 0, 255, 255, 255, 255);
      result = wox_renderer_floating_material(&renderer, 16, 16, 64, 64, 6, 4, 12, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0);
    } else if (opacity >= 0) {
      const auto glass = D2D1::ColorF(0, 0, 1, 0.5f);
      renderer.d2d_context->Clear(&glass);
      result = wox_renderer_begin_opacity(&renderer, 0, 8, 96, 80, opacity, 0);
      if (SUCCEEDED(result)) {
        wox_renderer_fill_rounded_rect(&renderer, 0, 8, 64, 40, 0, 255, 0, 0, 255);
        wox_renderer_fill_rounded_rect(&renderer, 32, 8, 64, 40, 0, 0, 255, 0, 255);
        result = wox_renderer_begin_opacity(&renderer, 0, 60, 96, 20, 0.5f, 0);
        if (SUCCEEDED(result)) {
          wox_renderer_fill_rounded_rect(&renderer, 0, 60, 96, 20, 0, 255, 255, 255, 255);
          result = wox_renderer_end_edge_fade(&renderer);
        }
        wox_renderer_end_edge_fade(&renderer);
      }
      wox_renderer_fill_rounded_rect(&renderer, 0, 90, 96, 6, 0, 255, 0, 0, 255);
    } else {
      const auto glass = D2D1::ColorF(0, 0, 1, 0.5f);
      renderer.d2d_context->Clear(&glass);
      result = wox_renderer_begin_edge_fade(&renderer, 0, 8, 96, 80, top, bottom, 0);
      if (SUCCEEDED(result)) {
        wox_renderer_set_clip_rect(&renderer, 0, 8, 96, 80);
        wox_renderer_fill_rounded_rect(&renderer, 0, 8, 96, 80, 0, 255, 255, 255, 255);
        result = wox_renderer_end_edge_fade(&renderer);
      }
      wox_renderer_fill_rounded_rect(&renderer, 0, 90, 96, 6, 0, 255, 0, 0, 255);
    }
    HRESULT draw_result = renderer.d2d_context->EndDraw();
    if (SUCCEEDED(result)) result = draw_result;
    renderer.frame_open = false;
  }
  properties.bitmapOptions = D2D1_BITMAP_OPTIONS_CPU_READ | D2D1_BITMAP_OPTIONS_CANNOT_DRAW;
  if (SUCCEEDED(result)) result = renderer.d2d_context->CreateBitmap(D2D1::SizeU(size, size), nullptr, 0, &properties, &readback);
  if (SUCCEEDED(result)) result = readback->CopyFromBitmap(nullptr, target, nullptr);
  D2D1_MAPPED_RECT mapped = {};
  if (SUCCEEDED(result)) result = readback->Map(D2D1_MAP_OPTIONS_READ, &mapped);
  if (SUCCEEDED(result)) {
    for (int row = 0; row < size; row++) memcpy(pixels + row * size * 4, mapped.bits + row * mapped.pitch, size * 4);
    readback->Unmap();
  }
  release_material_resources(&renderer);
  if (readback != nullptr) readback->Release();
  if (target != nullptr) target->Release();
  if (renderer.brush != nullptr) renderer.brush->Release();
  if (renderer.d2d_context != nullptr) renderer.d2d_context->Release();
  if (renderer.d2d_device != nullptr) renderer.d2d_device->Release();
  if (renderer.d2d_factory != nullptr) renderer.d2d_factory->Release();
  if (dxgi != nullptr) dxgi->Release();
  if (renderer.device != nullptr) renderer.device->Release();
  return result;
}

extern "C" int32_t wox_renderer_test_edge_fade(uint8_t *pixels, int32_t size, float scale, float top, float bottom) {
  return test_warp_bitmap(pixels, size, scale, top, bottom, false, -1);
}

extern "C" int32_t wox_renderer_test_floating_material(uint8_t *pixels, int32_t size, float scale) {
  return test_warp_bitmap(pixels, size, scale, 0, 0, true, -1);
}

// Offscreen WARP exercises production group composition without opening a window.
extern "C" int32_t wox_renderer_test_opacity(uint8_t *pixels, int32_t size, float scale, float opacity) {
  return test_warp_bitmap(pixels, size, scale, 0, 0, false, opacity);
}

// Report actual native cache activity, rather than estimating one upload per image draw.
extern "C" void wox_renderer_resource_stats(WoxRenderer *renderer, WoxRendererResourceStats *stats) {
  if (stats == nullptr) return;
  *stats = renderer == nullptr ? WoxRendererResourceStats{} : renderer->resource_stats;
  stats->resident_bytes = wox_renderer_resident_bytes(renderer);
}
