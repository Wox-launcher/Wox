//go:build windows

#include "clipboard_image_windows.h"
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <cstring>
#include <memory>
#include <new>

// Prepared formats own their allocations until each successful SetClipboardData transfers ownership.
struct WoxWindowsClipboardImage {
  HGLOBAL dib = nullptr;
  HGLOBAL png = nullptr;
  ~WoxWindowsClipboardImage() {
    if (dib) {
      GlobalFree(dib);
    }
    if (png) {
      GlobalFree(png);
    }
  }
};

static HRESULT clipboard_error() {
  DWORD error = GetLastError();
  return HRESULT_FROM_WIN32(error == ERROR_SUCCESS ? ERROR_GEN_FAILURE : error);
}

// Clipboard allocations can be rounded up; never expose uninitialized trailing bytes to consumers.
static void clear_clipboard_padding(HGLOBAL handle, void *memory, size_t size) {
  size_t allocated = GlobalSize(handle);
  if (allocated > size) {
    std::memset(static_cast<uint8_t *>(memory) + size, 0, allocated - size);
  }
}

// Prepare directly from packed RGBA/NRGBA on the calling worker, before acquiring the desktop clipboard.
int32_t wox_windows_prepare_clipboard_image(const uint8_t *pixels, uint32_t width, uint32_t height,
                                          uint32_t row_stride, int32_t premultiplied,
                                          const uint8_t *png, uint32_t png_size, WoxWindowsClipboardImage **image) {
  if (image == nullptr) {
    return E_INVALIDARG;
  }
  *image = nullptr;
  if (pixels == nullptr || width == 0 || height == 0 || width > 16384 || height > 16384 ||
      row_stride < width * 4 || (png_size != 0 && png == nullptr)) {
    return E_INVALIDARG;
  }
  size_t stride = static_cast<size_t>(width) * 4;
  size_t pixel_size = stride * height;
  size_t size = sizeof(BITMAPINFOHEADER) + pixel_size;
  std::unique_ptr<WoxWindowsClipboardImage> prepared(new (std::nothrow) WoxWindowsClipboardImage);
  if (!prepared) {
    return E_OUTOFMEMORY;
  }
  prepared->dib = GlobalAlloc(GMEM_MOVEABLE, size);
  if (!prepared->dib) {
    return E_OUTOFMEMORY;
  }
  auto *header = static_cast<BITMAPINFOHEADER *>(GlobalLock(prepared->dib));
  if (header == nullptr) {
    return clipboard_error();
  }
  *header = {};
  header->biSize = sizeof(BITMAPINFOHEADER);
  header->biWidth = static_cast<LONG>(width);
  header->biHeight = static_cast<LONG>(height);
  header->biPlanes = 1;
  header->biBitCount = 32;
  header->biCompression = BI_RGB;
  header->biSizeImage = static_cast<DWORD>(pixel_size);
  auto *output = reinterpret_cast<uint8_t *>(header + 1);
  for (uint32_t y = 0; y < height; ++y) {
    const uint8_t *src = pixels + static_cast<size_t>(height - 1 - y) * row_stride;
    uint8_t *dst = output + static_cast<size_t>(y) * stride;
    for (uint32_t x = 0; x < width; ++x) {
      uint32_t alpha = src[x * 4 + 3];
      if (!premultiplied || alpha == 255) {
        dst[x * 4] = src[x * 4 + 2];
        dst[x * 4 + 1] = src[x * 4 + 1];
        dst[x * 4 + 2] = src[x * 4];
      } else if (alpha == 0) {
        dst[x * 4] = dst[x * 4 + 1] = dst[x * 4 + 2] = 0;
      } else {
        // Match Go's NRGBAModel precision, including its 16-bit division before truncation.
        for (uint32_t channel = 0; channel < 3; ++channel) {
          dst[x * 4 + channel] = static_cast<uint8_t>((static_cast<uint32_t>(src[x * 4 + 2 - channel]) * 65535 / alpha) >> 8);
        }
      }
      dst[x * 4 + 3] = static_cast<uint8_t>(alpha);
    }
  }
  clear_clipboard_padding(prepared->dib, header, size);
  GlobalUnlock(prepared->dib);
  if (png_size != 0) {
    prepared->png = GlobalAlloc(GMEM_MOVEABLE, png_size);
    if (!prepared->png) {
      return E_OUTOFMEMORY;
    }
    void *memory = GlobalLock(prepared->png);
    if (memory == nullptr) {
      return clipboard_error();
    }
    std::memcpy(memory, png, png_size);
    clear_clipboard_padding(prepared->png, memory, png_size);
    GlobalUnlock(prepared->png);
  }
  *image = prepared.release();
  return S_OK;
}

// Only the short publication phase runs on the owner window's UI thread.
int32_t wox_windows_publish_clipboard_image(uintptr_t owner, WoxWindowsClipboardImage *image) {
  if (owner == 0 || image == nullptr || image->dib == nullptr) {
    return E_INVALIDARG;
  }
  UINT png_format = image->png ? RegisterClipboardFormatW(L"PNG") : 0;
  if (image->png && png_format == 0) {
    return clipboard_error();
  }
  bool opened = false;
  for (int attempt = 0; attempt < 10; ++attempt) {
    if (OpenClipboard(reinterpret_cast<HWND>(owner))) {
      opened = true;
      break;
    }
    Sleep(10);
  }
  if (!opened) {
    return clipboard_error();
  }
  HRESULT result = S_OK;
  if (!EmptyClipboard()) {
    result = clipboard_error();
  } else {
    if (image->png) {
      if (SetClipboardData(png_format, image->png)) {
        image->png = nullptr;
      } else {
        result = clipboard_error();
      }
    }
    if (SUCCEEDED(result)) {
      if (SetClipboardData(CF_DIB, image->dib)) {
        image->dib = nullptr;
      } else {
        result = clipboard_error();
      }
    }
    if (FAILED(result)) {
      EmptyClipboard();
    }
  }
  CloseClipboard();
  return result;
}

void wox_windows_destroy_clipboard_image(WoxWindowsClipboardImage *image) {
  delete image;
}

static bool open_clipboard_with_retry(HWND owner) {
  for (int attempt = 0; attempt < 10; ++attempt) {
    if (OpenClipboard(owner) != FALSE) {
      return true;
    }
    Sleep(10);
  }
  return false;
}

extern "C" int32_t wox_windows_write_clipboard_text(uintptr_t owner, const char *text) {
  if (owner == 0 || text == nullptr) {
    return E_INVALIDARG;
  }
  const int wide_length = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text, -1, nullptr, 0);
  if (wide_length <= 0 || static_cast<size_t>(wide_length) > SIZE_MAX / sizeof(wchar_t)) {
    return E_INVALIDARG;
  }
  const size_t byte_count = static_cast<size_t>(wide_length) * sizeof(wchar_t);
  HGLOBAL handle = GlobalAlloc(GMEM_MOVEABLE, byte_count);
  if (handle == nullptr) {
    return E_OUTOFMEMORY;
  }
  auto *memory = static_cast<wchar_t *>(GlobalLock(handle));
  if (memory == nullptr) {
    GlobalFree(handle);
    return clipboard_error();
  }
  if (MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text, -1, memory, wide_length) == 0) {
    GlobalUnlock(handle);
    GlobalFree(handle);
    return E_INVALIDARG;
  }
  GlobalUnlock(handle);

  if (!open_clipboard_with_retry(reinterpret_cast<HWND>(owner))) {
    GlobalFree(handle);
    return clipboard_error();
  }
  if (EmptyClipboard() == FALSE) {
    const HRESULT result = clipboard_error();
    CloseClipboard();
    GlobalFree(handle);
    return result;
  }
  if (SetClipboardData(CF_UNICODETEXT, handle) == nullptr) {
    const HRESULT result = clipboard_error();
    CloseClipboard();
    GlobalFree(handle);
    return result;
  }
  CloseClipboard();
  return S_OK;
}
