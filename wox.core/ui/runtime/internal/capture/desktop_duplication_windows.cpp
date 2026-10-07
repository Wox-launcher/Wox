#define WIN32_LEAN_AND_MEAN
#include <windows.h>

#include <d3d11.h>
#include <dxgi.h>
#include <dxgi1_2.h>

#include <cstdint>
#include <cstring>
#include <new>
#include <vector>

#include "desktop_duplication_windows.h"

namespace {

template <typename T>
void release_com(T **object) {
  if (object != nullptr && *object != nullptr) {
    (*object)->Release();
    *object = nullptr;
  }
}

}  // namespace

struct WoxDXGIRectCapturer {
  ID3D11Device *device = nullptr;
  ID3D11DeviceContext *context = nullptr;
  IDXGIOutputDuplication *duplication = nullptr;
  ID3D11Texture2D *staging = nullptr;
  IDXGIAdapter *adapter = nullptr;
  IDXGIOutput1 *output = nullptr;
  int32_t width = 0;
  int32_t height = 0;
  D3D11_BOX box = {};
  std::vector<uint8_t> last_frame;
  bool has_frame = false;
};

static bool output_contains_rect(const RECT &output, int32_t x, int32_t y, int32_t width, int32_t height) {
  return x >= output.left && y >= output.top && x + width <= output.right && y + height <= output.bottom;
}

static HRESULT duplicate_output(WoxDXGIRectCapturer *capturer) {
  release_com(&capturer->duplication);
  if (capturer->output == nullptr || capturer->device == nullptr) {
    return E_FAIL;
  }
  return capturer->output->DuplicateOutput(capturer->device, &capturer->duplication);
}

static HRESULT copy_mapped_frame(WoxDXGIRectCapturer *capturer, const D3D11_MAPPED_SUBRESOURCE &mapped, uint8_t *bgra, int32_t stride) {
  const int32_t row_bytes = capturer->width * 4;
  capturer->last_frame.resize(static_cast<size_t>(capturer->height) * static_cast<size_t>(row_bytes));
  for (int32_t row = 0; row < capturer->height; row++) {
    const uint8_t *source = static_cast<const uint8_t *>(mapped.pData) + static_cast<size_t>(row) * mapped.RowPitch;
    uint8_t *cached = capturer->last_frame.data() + static_cast<size_t>(row) * static_cast<size_t>(row_bytes);
    std::memcpy(cached, source, static_cast<size_t>(row_bytes));
    std::memcpy(bgra + static_cast<size_t>(row) * static_cast<size_t>(stride), source, static_cast<size_t>(row_bytes));
  }
  capturer->has_frame = true;
  return S_OK;
}

static void copy_last_frame(WoxDXGIRectCapturer *capturer, uint8_t *bgra, int32_t stride) {
  const int32_t row_bytes = capturer->width * 4;
  for (int32_t row = 0; row < capturer->height; row++) {
    std::memcpy(bgra + static_cast<size_t>(row) * static_cast<size_t>(stride),
                capturer->last_frame.data() + static_cast<size_t>(row) * static_cast<size_t>(row_bytes),
                static_cast<size_t>(row_bytes));
  }
}

int32_t wox_dxgi_rect_capturer_create(int32_t x, int32_t y, int32_t width, int32_t height, WoxDXGIRectCapturer **out) {
  if (out == nullptr || width <= 0 || height <= 0 || width > 16384 || height > 16384) {
    return -1;
  }
  IDXGIFactory1 *factory = nullptr;
  if (FAILED(CreateDXGIFactory1(__uuidof(IDXGIFactory1), reinterpret_cast<void **>(&factory)))) {
    return -2;
  }

  WoxDXGIRectCapturer *capturer = new (std::nothrow) WoxDXGIRectCapturer();
  if (capturer == nullptr) {
    release_com(&factory);
    return -3;
  }
  capturer->width = width;
  capturer->height = height;

  HRESULT result = E_FAIL;
  for (UINT adapter_index = 0; SUCCEEDED(factory->EnumAdapters(adapter_index, &capturer->adapter)); adapter_index++) {
    IDXGIOutput *output = nullptr;
    for (UINT output_index = 0; SUCCEEDED(capturer->adapter->EnumOutputs(output_index, &output)); output_index++) {
      DXGI_OUTPUT_DESC description = {};
      if (SUCCEEDED(output->GetDesc(&description)) && description.AttachedToDesktop &&
          output_contains_rect(description.DesktopCoordinates, x, y, width, height)) {
        result = output->QueryInterface(__uuidof(IDXGIOutput1), reinterpret_cast<void **>(&capturer->output));
        release_com(&output);
        break;
      }
      release_com(&output);
    }
    if (capturer->output != nullptr) {
      break;
    }
    release_com(&capturer->adapter);
  }
  release_com(&factory);
  if (capturer->output == nullptr || capturer->adapter == nullptr) {
    wox_dxgi_rect_capturer_destroy(capturer);
    return -4;
  }

  D3D_FEATURE_LEVEL feature_level = D3D_FEATURE_LEVEL_11_0;
  result = D3D11CreateDevice(capturer->adapter, D3D_DRIVER_TYPE_UNKNOWN, nullptr, 0, nullptr, 0, D3D11_SDK_VERSION,
                             &capturer->device, &feature_level, &capturer->context);
  if (FAILED(result) || FAILED(duplicate_output(capturer))) {
    wox_dxgi_rect_capturer_destroy(capturer);
    return -5;
  }

  DXGI_OUTPUT_DESC output_description = {};
  if (FAILED(capturer->output->GetDesc(&output_description))) {
    wox_dxgi_rect_capturer_destroy(capturer);
    return -6;
  }
  capturer->box.left = static_cast<UINT>(x - output_description.DesktopCoordinates.left);
  capturer->box.top = static_cast<UINT>(y - output_description.DesktopCoordinates.top);
  capturer->box.front = 0;
  capturer->box.right = capturer->box.left + static_cast<UINT>(width);
  capturer->box.bottom = capturer->box.top + static_cast<UINT>(height);
  capturer->box.back = 1;

  D3D11_TEXTURE2D_DESC staging = {};
  staging.Width = static_cast<UINT>(width);
  staging.Height = static_cast<UINT>(height);
  staging.MipLevels = 1;
  staging.ArraySize = 1;
  staging.Format = DXGI_FORMAT_B8G8R8A8_UNORM;
  staging.SampleDesc.Count = 1;
  staging.Usage = D3D11_USAGE_STAGING;
  staging.CPUAccessFlags = D3D11_CPU_ACCESS_READ;
  if (FAILED(capturer->device->CreateTexture2D(&staging, nullptr, &capturer->staging))) {
    wox_dxgi_rect_capturer_destroy(capturer);
    return -7;
  }
  *out = capturer;
  return 0;
}

int32_t wox_dxgi_rect_capturer_capture(WoxDXGIRectCapturer *capturer, uint8_t *bgra, int32_t stride) {
  if (capturer == nullptr || capturer->duplication == nullptr || capturer->staging == nullptr || bgra == nullptr ||
      stride < capturer->width * 4) {
    return -1;
  }

  DXGI_OUTDUPL_FRAME_INFO info = {};
  IDXGIResource *resource = nullptr;
  HRESULT result = capturer->duplication->AcquireNextFrame(16, &info, &resource);
  if (result == DXGI_ERROR_WAIT_TIMEOUT) {
    if (capturer->has_frame) {
      copy_last_frame(capturer, bgra, stride);
      return 0;
    }
    result = capturer->duplication->AcquireNextFrame(100, &info, &resource);
  }
  if (result == DXGI_ERROR_ACCESS_LOST) {
    if (FAILED(duplicate_output(capturer))) {
      return -2;
    }
    result = capturer->duplication->AcquireNextFrame(100, &info, &resource);
  }
  if (FAILED(result) || resource == nullptr) {
    if (capturer->has_frame) {
      copy_last_frame(capturer, bgra, stride);
      return 0;
    }
    return -3;
  }

  ID3D11Texture2D *source = nullptr;
  result = resource->QueryInterface(__uuidof(ID3D11Texture2D), reinterpret_cast<void **>(&source));
  release_com(&resource);
  if (FAILED(result) || source == nullptr) {
    capturer->duplication->ReleaseFrame();
    return -4;
  }
  capturer->context->CopySubresourceRegion(capturer->staging, 0, 0, 0, 0, source, 0, &capturer->box);
  release_com(&source);

  D3D11_MAPPED_SUBRESOURCE mapped = {};
  result = capturer->context->Map(capturer->staging, 0, D3D11_MAP_READ, 0, &mapped);
  if (FAILED(result) || mapped.pData == nullptr) {
    capturer->duplication->ReleaseFrame();
    return -5;
  }
  copy_mapped_frame(capturer, mapped, bgra, stride);
  capturer->context->Unmap(capturer->staging, 0);
  capturer->duplication->ReleaseFrame();
  return 0;
}

void wox_dxgi_rect_capturer_destroy(WoxDXGIRectCapturer *capturer) {
  if (capturer == nullptr) {
    return;
  }
  release_com(&capturer->staging);
  release_com(&capturer->duplication);
  release_com(&capturer->output);
  release_com(&capturer->context);
  release_com(&capturer->device);
  release_com(&capturer->adapter);
  delete capturer;
}
