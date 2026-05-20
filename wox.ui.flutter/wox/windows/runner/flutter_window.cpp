#include "flutter_window.h"

#include <algorithm>
#include <cmath>
#include <cctype>
#include <cstdint>
#include <cstring>
#include <optional>
#include <mutex>
#include <sstream>
#include <string>
#include <thread>
#include <utility>
#include <vector>
#include <flutter/plugin_registrar_windows.h>
#include <windows.h>
#include <windowsx.h>
#include <dwmapi.h>
#include <gdiplus.h>
#include <objidl.h>

#include "flutter/generated_plugin_registrant.h"
#include "utils.h"
#include "wox_webview/wox_webview_plugin.h"

#ifndef DWMWA_USE_IMMERSIVE_DARK_MODE
#define DWMWA_USE_IMMERSIVE_DARK_MODE 20
#endif

#ifndef DWMWA_WINDOW_CORNER_PREFERENCE
#define DWMWA_WINDOW_CORNER_PREFERENCE 33
#endif

#ifndef DWMWA_SYSTEMBACKDROP_TYPE
#define DWMWA_SYSTEMBACKDROP_TYPE 38
#endif

// After SW_HIDE, Windows may activate another window asynchronously.
// Retry restoring the previous foreground window shortly after hide.
static constexpr UINT_PTR kRestoreForegroundTimerId1 = 0xA11;
static constexpr UINT_PTR kRestoreForegroundTimerId2 = 0xA12;
static constexpr ULONGLONG kPostShowBlurGraceMs = 300;
static constexpr int kDwmSystemBackdropNone = 0;
static constexpr int kDwmSystemBackdropTabbed = 3;
static constexpr int kDwmCornerDoNotRound = 1;
static constexpr int kDwmCornerRound = 2;
static constexpr UINT kScrollingCaptureWheelMessage = WM_APP + 0x51;
static constexpr UINT kScreenshotSelectionDimRegionUpdateMessage = WM_APP + 0x52;
static constexpr wchar_t kScrollingCaptureOverlayWindowClassName[] = L"WoxScrollingCaptureOverlayWindow";
static constexpr wchar_t kScreenshotSelectionInputWindowClassName[] = L"WoxScreenshotSelectionInputWindow";
static constexpr wchar_t kScreenshotSelectionBorderWindowClassName[] = L"WoxScreenshotSelectionBorderWindow";
static constexpr double kScrollingCaptureToolbarSlotHeightDip = 72.0;
static constexpr double kScrollingCaptureToolbarHeightDip = 56.0;
static constexpr double kScrollingCaptureToolbarWidthDip = 124.0;
static constexpr double kScrollingCaptureToolbarCornerRadiusDip = 18.0;

// Store window instance for window procedure
FlutterWindow *g_window_instance = nullptr;
static std::once_flag g_gdiplus_init_once;
static ULONG_PTR g_gdiplus_token = 0;

static void EnsureGdiplusInitialized()
{
  std::call_once(g_gdiplus_init_once, []() {
    Gdiplus::GdiplusStartupInput startup_input;
    Gdiplus::GdiplusStartup(&g_gdiplus_token, &startup_input, nullptr);
  });
}

static std::string Base64Encode(const std::vector<uint8_t> &data)
{
  static constexpr char kAlphabet[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
  std::string encoded;
  encoded.reserve(((data.size() + 2) / 3) * 4);

  size_t index = 0;
  while (index + 2 < data.size())
  {
    const uint32_t value = (static_cast<uint32_t>(data[index]) << 16) |
                           (static_cast<uint32_t>(data[index + 1]) << 8) |
                           static_cast<uint32_t>(data[index + 2]);
    encoded.push_back(kAlphabet[(value >> 18) & 0x3F]);
    encoded.push_back(kAlphabet[(value >> 12) & 0x3F]);
    encoded.push_back(kAlphabet[(value >> 6) & 0x3F]);
    encoded.push_back(kAlphabet[value & 0x3F]);
    index += 3;
  }

  if (index < data.size())
  {
    uint32_t value = static_cast<uint32_t>(data[index]) << 16;
    encoded.push_back(kAlphabet[(value >> 18) & 0x3F]);
    if (index + 1 < data.size())
    {
      value |= static_cast<uint32_t>(data[index + 1]) << 8;
      encoded.push_back(kAlphabet[(value >> 12) & 0x3F]);
      encoded.push_back(kAlphabet[(value >> 6) & 0x3F]);
      encoded.push_back('=');
    }
    else
    {
      encoded.push_back(kAlphabet[(value >> 12) & 0x3F]);
      encoded.push_back('=');
      encoded.push_back('=');
    }
  }

  return encoded;
}

static bool GetPngEncoderClsid(CLSID *out_clsid)
{
  EnsureGdiplusInitialized();

  UINT encoder_count = 0;
  UINT encoder_size = 0;
  if (Gdiplus::GetImageEncodersSize(&encoder_count, &encoder_size) != Gdiplus::Ok || encoder_size == 0)
  {
    return false;
  }

  std::vector<uint8_t> buffer(encoder_size);
  auto *encoders = reinterpret_cast<Gdiplus::ImageCodecInfo *>(buffer.data());
  if (Gdiplus::GetImageEncoders(encoder_count, encoder_size, encoders) != Gdiplus::Ok)
  {
    return false;
  }

  for (UINT i = 0; i < encoder_count; ++i)
  {
    if (wcscmp(encoders[i].MimeType, L"image/png") == 0)
    {
      *out_clsid = encoders[i].Clsid;
      return true;
    }
  }

  return false;
}

static std::wstring Utf16FromUtf8(const std::string &utf8_string)
{
  if (utf8_string.empty())
  {
    return std::wstring();
  }

  const int target_length = ::MultiByteToWideChar(
      CP_UTF8, MB_ERR_INVALID_CHARS, utf8_string.data(),
      static_cast<int>(utf8_string.size()), nullptr, 0);
  if (target_length <= 0)
  {
    return std::wstring();
  }

  std::wstring utf16_string(target_length, L'\0');
  const int converted_length = ::MultiByteToWideChar(
      CP_UTF8, MB_ERR_INVALID_CHARS, utf8_string.data(),
      static_cast<int>(utf8_string.size()), utf16_string.data(), target_length);
  if (converted_length != target_length)
  {
    return std::wstring();
  }

  return utf16_string;
}

static bool ReadFileBytes(const std::wstring &file_path, std::vector<uint8_t> *bytes_out, std::string *error)
{
  HANDLE file = ::CreateFileW(file_path.c_str(), GENERIC_READ, FILE_SHARE_READ, nullptr, OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, nullptr);
  if (file == INVALID_HANDLE_VALUE)
  {
    if (error != nullptr)
    {
      *error = "Failed to open screenshot file";
    }
    return false;
  }

  LARGE_INTEGER file_size{};
  if (!::GetFileSizeEx(file, &file_size))
  {
    ::CloseHandle(file);
    if (error != nullptr)
    {
      *error = "Failed to read screenshot file size";
    }
    return false;
  }

  if (file_size.QuadPart < 0 || file_size.QuadPart > static_cast<LONGLONG>(128 * 1024 * 1024))
  {
    ::CloseHandle(file);
    if (error != nullptr)
    {
      *error = "Screenshot file is too large for clipboard export";
    }
    return false;
  }

  bytes_out->assign(static_cast<size_t>(file_size.QuadPart), 0);
  if (!bytes_out->empty())
  {
    DWORD bytes_read = 0;
    const DWORD bytes_to_read = static_cast<DWORD>(bytes_out->size());
    if (!::ReadFile(file, bytes_out->data(), bytes_to_read, &bytes_read, nullptr) || bytes_read != bytes_to_read)
    {
      ::CloseHandle(file);
      if (error != nullptr)
      {
        *error = "Failed to read screenshot file bytes";
      }
      return false;
    }
  }

  ::CloseHandle(file);
  return true;
}

static bool BuildClipboardDibData(Gdiplus::Bitmap *bitmap, std::vector<uint8_t> *dib_out, std::string *error)
{
  const UINT width = bitmap->GetWidth();
  const UINT height = bitmap->GetHeight();
  if (width == 0 || height == 0)
  {
    if (error != nullptr)
    {
      *error = "Screenshot bitmap has invalid dimensions";
    }
    return false;
  }

  HBITMAP hbitmap = nullptr;
  if (bitmap->GetHBITMAP(Gdiplus::Color(0, 0, 0, 0), &hbitmap) != Gdiplus::Ok || hbitmap == nullptr)
  {
    if (error != nullptr)
    {
      *error = "Failed to convert screenshot bitmap to HBITMAP";
    }
    return false;
  }

  HDC screen_dc = ::GetDC(nullptr);
  HDC memory_dc = ::CreateCompatibleDC(screen_dc);
  if (screen_dc == nullptr || memory_dc == nullptr)
  {
    if (memory_dc != nullptr)
    {
      ::DeleteDC(memory_dc);
    }
    if (screen_dc != nullptr)
    {
      ::ReleaseDC(nullptr, screen_dc);
    }
    ::DeleteObject(hbitmap);
    if (error != nullptr)
    {
      *error = "Failed to create device context for screenshot clipboard export";
    }
    return false;
  }

  BITMAPINFOHEADER header{};
  header.biSize = sizeof(BITMAPINFOHEADER);
  header.biWidth = static_cast<LONG>(width);
  header.biHeight = static_cast<LONG>(height);
  header.biPlanes = 1;
  header.biBitCount = 32;
  header.biCompression = BI_RGB;
  header.biSizeImage = static_cast<DWORD>(width * height * 4);

  dib_out->assign(sizeof(BITMAPINFOHEADER) + header.biSizeImage, 0);
  std::memcpy(dib_out->data(), &header, sizeof(BITMAPINFOHEADER));
  HGDIOBJ previous_bitmap = ::SelectObject(memory_dc, hbitmap);
  if (!::GetDIBits(memory_dc, hbitmap, 0, height, dib_out->data() + sizeof(BITMAPINFOHEADER), reinterpret_cast<BITMAPINFO *>(dib_out->data()), DIB_RGB_COLORS))
  {
    if (previous_bitmap != nullptr)
    {
      ::SelectObject(memory_dc, previous_bitmap);
    }
    ::DeleteDC(memory_dc);
    ::ReleaseDC(nullptr, screen_dc);
    ::DeleteObject(hbitmap);
    if (error != nullptr)
    {
      *error = "Failed to build CF_DIB data for screenshot clipboard export";
    }
    return false;
  }

  if (previous_bitmap != nullptr)
  {
    ::SelectObject(memory_dc, previous_bitmap);
  }
  ::DeleteDC(memory_dc);
  ::ReleaseDC(nullptr, screen_dc);
  ::DeleteObject(hbitmap);
  return true;
}

static bool OpenClipboardRetry()
{
  for (int attempt = 0; attempt < 10; ++attempt)
  {
    if (::OpenClipboard(nullptr))
    {
      return true;
    }
    ::Sleep(10);
  }
  return false;
}

static bool WriteClipboardImageBytes(const std::vector<uint8_t> &png_bytes, const std::vector<uint8_t> &dib_bytes, std::string *error)
{
  if (dib_bytes.empty())
  {
    if (error != nullptr)
    {
      *error = "Screenshot clipboard export requires CF_DIB bytes";
    }
    return false;
  }

  if (!OpenClipboardRetry())
  {
    if (error != nullptr)
    {
      *error = "Failed to open Windows clipboard";
    }
    return false;
  }

  if (!::EmptyClipboard())
  {
    ::CloseClipboard();
    if (error != nullptr)
    {
      *error = "Failed to clear Windows clipboard";
    }
    return false;
  }

  // Windows screenshot paste remains most compatible when we publish both the registered PNG
  // format and CF_DIB. The PNG keeps transparency-aware consumers fast, while CF_DIB preserves
  // compatibility with native apps that ignore the registered PNG clipboard type.
  const UINT png_format = ::RegisterClipboardFormatW(L"PNG");
  if (png_format != 0 && !png_bytes.empty())
  {
    HGLOBAL png_handle = ::GlobalAlloc(GMEM_MOVEABLE, png_bytes.size());
    if (png_handle != nullptr)
    {
      void *png_memory = ::GlobalLock(png_handle);
      if (png_memory != nullptr)
      {
        std::memcpy(png_memory, png_bytes.data(), png_bytes.size());
        ::GlobalUnlock(png_handle);
        if (::SetClipboardData(png_format, png_handle) == nullptr)
        {
          ::GlobalFree(png_handle);
        }
      }
      else
      {
        ::GlobalFree(png_handle);
      }
    }
  }

  HGLOBAL dib_handle = ::GlobalAlloc(GMEM_MOVEABLE, dib_bytes.size());
  if (dib_handle == nullptr)
  {
    ::CloseClipboard();
    if (error != nullptr)
    {
      *error = "Failed to allocate CF_DIB clipboard buffer";
    }
    return false;
  }

  void *dib_memory = ::GlobalLock(dib_handle);
  if (dib_memory == nullptr)
  {
    ::GlobalFree(dib_handle);
    ::CloseClipboard();
    if (error != nullptr)
    {
      *error = "Failed to lock CF_DIB clipboard buffer";
    }
    return false;
  }

  std::memcpy(dib_memory, dib_bytes.data(), dib_bytes.size());
  ::GlobalUnlock(dib_handle);

  if (::SetClipboardData(CF_DIB, dib_handle) == nullptr)
  {
    ::GlobalFree(dib_handle);
    ::CloseClipboard();
    if (error != nullptr)
    {
      *error = "Failed to publish CF_DIB screenshot data to clipboard";
    }
    return false;
  }

  ::CloseClipboard();
  return true;
}

static bool WriteClipboardImageFile(const std::string &file_path, std::string *error)
{
  const std::wstring wide_file_path = Utf16FromUtf8(file_path);
  if (wide_file_path.empty())
  {
    if (error != nullptr)
    {
      *error = "Invalid screenshot clipboard file path";
    }
    return false;
  }

  std::vector<uint8_t> png_bytes;
  if (!ReadFileBytes(wide_file_path, &png_bytes, error))
  {
    return false;
  }

  // Flutter already persisted the final annotated PNG, so the Windows runner should derive its
  // clipboard-native formats from that one file instead of forcing Go to reopen and decode it.
  // This keeps the websocket payload tiny while still publishing the CF_DIB data Windows paste
  // targets need for compatibility.
  EnsureGdiplusInitialized();
  Gdiplus::Bitmap bitmap(wide_file_path.c_str());
  if (bitmap.GetLastStatus() != Gdiplus::Ok)
  {
    if (error != nullptr)
    {
      *error = "Failed to load screenshot file for clipboard export";
    }
    return false;
  }

  std::vector<uint8_t> dib_bytes;
  if (!BuildClipboardDibData(&bitmap, &dib_bytes, error))
  {
    return false;
  }

  return WriteClipboardImageBytes(png_bytes, dib_bytes, error);
}

static bool EncodeBitmapToPngBase64(HBITMAP bitmap, std::string &png_base64, std::string &error)
{
  CLSID png_clsid{};
  if (!GetPngEncoderClsid(&png_clsid))
  {
    error = "Failed to find PNG encoder";
    return false;
  }

  Gdiplus::Bitmap image(bitmap, nullptr);
  IStream *stream = nullptr;
  if (CreateStreamOnHGlobal(nullptr, TRUE, &stream) != S_OK)
  {
    error = "Failed to create memory stream";
    return false;
  }

  const auto status = image.Save(stream, &png_clsid, nullptr);
  if (status != Gdiplus::Ok)
  {
    error = "Failed to encode monitor image as PNG";
    stream->Release();
    return false;
  }

  HGLOBAL global = nullptr;
  if (GetHGlobalFromStream(stream, &global) != S_OK || global == nullptr)
  {
    error = "Failed to access encoded PNG stream";
    stream->Release();
    return false;
  }

  const SIZE_T size = GlobalSize(global);
  auto *bytes = static_cast<uint8_t *>(GlobalLock(global));
  if (bytes == nullptr || size == 0)
  {
    error = "Failed to lock encoded PNG bytes";
    if (bytes != nullptr)
    {
      GlobalUnlock(global);
    }
    stream->Release();
    return false;
  }

  std::vector<uint8_t> copy(bytes, bytes + size);
  GlobalUnlock(global);
  stream->Release();
  png_base64 = Base64Encode(copy);
  return true;
}

static flutter::EncodableMap BuildRectValue(double x, double y, double width, double height)
{
  flutter::EncodableMap rect;
  rect[flutter::EncodableValue("x")] = flutter::EncodableValue(x);
  rect[flutter::EncodableValue("y")] = flutter::EncodableValue(y);
  rect[flutter::EncodableValue("width")] = flutter::EncodableValue(width);
  rect[flutter::EncodableValue("height")] = flutter::EncodableValue(height);
  return rect;
}

static flutter::EncodableMap BuildRectValue(const RECT &rect)
{
  return BuildRectValue(
      static_cast<double>(rect.left),
      static_cast<double>(rect.top),
      static_cast<double>(rect.right - rect.left),
      static_cast<double>(rect.bottom - rect.top));
}

static flutter::EncodableMap BuildScaledRectValue(const RECT &rect, double scale)
{
  const double safe_scale = scale <= 0 ? 1.0 : scale;
  return BuildRectValue(
      static_cast<double>(rect.left) / safe_scale,
      static_cast<double>(rect.top) / safe_scale,
      static_cast<double>(rect.right - rect.left) / safe_scale,
      static_cast<double>(rect.bottom - rect.top) / safe_scale);
}

static bool TryReadEncodableDouble(const flutter::EncodableMap &map, const char *key, double *value_out, std::string *error_out)
{
  const auto value_it = map.find(flutter::EncodableValue(key));
  if (value_it == map.end())
  {
    if (error_out != nullptr)
    {
      *error_out = std::string("Missing ") + key + " for logicalSelection";
    }
    return false;
  }

  if (const auto *double_value = std::get_if<double>(&value_it->second))
  {
    *value_out = *double_value;
    return true;
  }
  if (const auto *int32_value = std::get_if<int32_t>(&value_it->second))
  {
    *value_out = static_cast<double>(*int32_value);
    return true;
  }
  if (const auto *int64_value = std::get_if<int64_t>(&value_it->second))
  {
    *value_out = static_cast<double>(*int64_value);
    return true;
  }

  if (error_out != nullptr)
  {
    *error_out = std::string("logicalSelection.") + key + " must be a number";
  }
  return false;
}

static bool TryParseLogicalSelectionArgument(const flutter::EncodableValue *arguments, double workspace_scale, std::optional<RECT> *selection_out, std::string *error_out)
{
  selection_out->reset();
  if (arguments == nullptr)
  {
    return true;
  }

  const auto *argument_map = std::get_if<flutter::EncodableMap>(arguments);
  if (argument_map == nullptr)
  {
    if (error_out != nullptr)
    {
      *error_out = "captureAllDisplays arguments must be a map";
    }
    return false;
  }

  const auto selection_it = argument_map->find(flutter::EncodableValue("logicalSelection"));
  if (selection_it == argument_map->end())
  {
    return true;
  }

  const auto *selection_map = std::get_if<flutter::EncodableMap>(&selection_it->second);
  if (selection_map == nullptr)
  {
    if (error_out != nullptr)
    {
      *error_out = "logicalSelection must be a map";
    }
    return false;
  }

  double x = 0;
  double y = 0;
  double width = 0;
  double height = 0;
  if (!TryReadEncodableDouble(*selection_map, "x", &x, error_out) ||
      !TryReadEncodableDouble(*selection_map, "y", &y, error_out) ||
      !TryReadEncodableDouble(*selection_map, "width", &width, error_out) ||
      !TryReadEncodableDouble(*selection_map, "height", &height, error_out))
  {
    return false;
  }

  if (width <= 0 || height <= 0)
  {
    if (error_out != nullptr)
    {
      *error_out = "logicalSelection must have positive width and height";
    }
    return false;
  }

  // Bug fix: Dart sends the scrolling selection in screenshot workspace logical coordinates. The
  // previous Windows region capture treated those values as native pixels, so high-DPI workspaces
  // captured a scaled-down area and Dart normalized it again until it no longer intersected the
  // user's selection. Convert with the same workspace scale used by the native overlay, while still
  // expanding fractional edges outward so preview and export are sourced from the full selection.
  const double safe_scale = workspace_scale <= 0 ? 1.0 : workspace_scale;
  RECT selection{};
  selection.left = static_cast<LONG>(std::floor(x * safe_scale));
  selection.top = static_cast<LONG>(std::floor(y * safe_scale));
  selection.right = static_cast<LONG>(std::ceil((x + width) * safe_scale));
  selection.bottom = static_cast<LONG>(std::ceil((y + height) * safe_scale));
  if (selection.right <= selection.left || selection.bottom <= selection.top)
  {
    if (error_out != nullptr)
    {
      *error_out = "logicalSelection resolves to an empty native rectangle";
    }
    return false;
  }

  *selection_out = selection;
  return true;
}

static bool TryReadNestedRectArgument(const flutter::EncodableMap &arguments, const char *key, double scale, RECT *rect_out, std::string *error_out)
{
  const auto rect_it = arguments.find(flutter::EncodableValue(key));
  if (rect_it == arguments.end())
  {
    if (error_out != nullptr)
    {
      *error_out = std::string("Missing ") + key + " for beginScrollingCaptureOverlay";
    }
    return false;
  }

  const auto *rect_map = std::get_if<flutter::EncodableMap>(&rect_it->second);
  if (rect_map == nullptr)
  {
    if (error_out != nullptr)
    {
      *error_out = std::string(key) + " must be a map";
    }
    return false;
  }

  double x = 0;
  double y = 0;
  double width = 0;
  double height = 0;
  if (!TryReadEncodableDouble(*rect_map, "x", &x, error_out) ||
      !TryReadEncodableDouble(*rect_map, "y", &y, error_out) ||
      !TryReadEncodableDouble(*rect_map, "width", &width, error_out) ||
      !TryReadEncodableDouble(*rect_map, "height", &height, error_out))
  {
    return false;
  }

  if (width <= 0 || height <= 0)
  {
    if (error_out != nullptr)
    {
      *error_out = std::string(key) + " must have positive width and height";
    }
    return false;
  }

  const double safe_scale = scale <= 0 ? 1.0 : scale;
  *rect_out = RECT{
      static_cast<LONG>(std::lround(x * safe_scale)),
      static_cast<LONG>(std::lround(y * safe_scale)),
      static_cast<LONG>(std::lround((x + width) * safe_scale)),
      static_cast<LONG>(std::lround((y + height) * safe_scale))};
  return rect_out->right > rect_out->left && rect_out->bottom > rect_out->top;
}

static bool TryIntersectRects(const RECT &first, const RECT &second, RECT *intersection_out)
{
  RECT intersection{};
  intersection.left = first.left > second.left ? first.left : second.left;
  intersection.top = first.top > second.top ? first.top : second.top;
  intersection.right = first.right < second.right ? first.right : second.right;
  intersection.bottom = first.bottom < second.bottom ? first.bottom : second.bottom;
  if (intersection.right <= intersection.left || intersection.bottom <= intersection.top)
  {
    return false;
  }

  *intersection_out = intersection;
  return true;
}

static bool IsRectEmptyOrInvalid(const RECT &rect)
{
  return rect.right <= rect.left || rect.bottom <= rect.top;
}

static bool IsPointInRect(const RECT &rect, const POINT &point)
{
  return point.x >= rect.left && point.x < rect.right && point.y >= rect.top && point.y < rect.bottom;
}

static RECT RectFromPoints(const POINT &first, const POINT &second)
{
  return RECT{
      first.x < second.x ? first.x : second.x,
      first.y < second.y ? first.y : second.y,
      first.x > second.x ? first.x : second.x,
      first.y > second.y ? first.y : second.y};
}

static RECT ClampRectToBounds(const RECT &rect, const RECT &bounds)
{
  return RECT{
      rect.left < bounds.left ? bounds.left : rect.left,
      rect.top < bounds.top ? bounds.top : rect.top,
      rect.right > bounds.right ? bounds.right : rect.right,
      rect.bottom > bounds.bottom ? bounds.bottom : rect.bottom};
}

static double RectIntersectionArea(const RECT &first, const RECT &second)
{
  RECT intersection{};
  if (!TryIntersectRects(first, second, &intersection))
  {
    return 0;
  }
  return static_cast<double>(intersection.right - intersection.left) * static_cast<double>(intersection.bottom - intersection.top);
}

static HBRUSH ScreenshotSelectionBorderBrush()
{
  static HBRUSH border_brush = CreateSolidBrush(RGB(41, 255, 114));
  return border_brush;
}

static RECT LocalRectForWorkspace(const RECT &rect, const RECT &workspace)
{
  return RECT{
      rect.left - workspace.left,
      rect.top - workspace.top,
      rect.right - workspace.left,
      rect.bottom - workspace.top};
}

static void SetOwnedWindowRegion(HWND hwnd, HRGN region)
{
  if (hwnd == nullptr || !IsWindow(hwnd))
  {
    if (region != nullptr)
    {
      DeleteObject(region);
    }
    return;
  }

  if (SetWindowRgn(hwnd, region, TRUE) == 0 && region != nullptr)
  {
    DeleteObject(region);
  }
}

static HRGN CreateSelectionDimRegion(const RECT &workspace, const RECT &selection)
{
  const RECT local_selection = LocalRectForWorkspace(selection, workspace);
  HRGN dim_region = CreateRectRgn(0, 0, workspace.right - workspace.left, workspace.bottom - workspace.top);
  HRGN clear_region = CreateRectRgn(local_selection.left, local_selection.top, local_selection.right, local_selection.bottom);
  if (dim_region != nullptr && clear_region != nullptr)
  {
    CombineRgn(dim_region, dim_region, clear_region, RGN_DIFF);
  }
  if (clear_region != nullptr)
  {
    DeleteObject(clear_region);
  }
  return dim_region;
}

static bool IsKeyDownMessage(UINT message)
{
  return message == WM_KEYDOWN || message == WM_SYSKEYDOWN;
}

static bool IsKeyUpMessage(UINT message)
{
  return message == WM_KEYUP || message == WM_SYSKEYUP;
}

uint64_t FlutterWindow::MakeKeyboardMessageSignature(UINT message, WPARAM wparam, LPARAM lparam)
{
  const uint64_t virtual_key = static_cast<uint64_t>(wparam & 0xFFFF);
  const uint64_t scancode = static_cast<uint64_t>((lparam >> 16) & 0xFF);
  const uint64_t is_extended = static_cast<uint64_t>((lparam >> 24) & 0x1);
  const uint64_t is_system_key = static_cast<uint64_t>(message == WM_SYSKEYDOWN || message == WM_SYSKEYUP);

  return virtual_key | (scancode << 16) | (is_extended << 24) | (is_system_key << 25);
}

static UINT KeyboardKeyUpMessageFromSignature(uint64_t signature)
{
  const bool is_system_key = ((signature >> 25) & 0x1) != 0;
  return is_system_key ? WM_SYSKEYUP : WM_KEYUP;
}

static WPARAM KeyboardVirtualKeyFromSignature(uint64_t signature)
{
  return static_cast<WPARAM>(signature & 0xFFFF);
}

static LPARAM MakeKeyboardKeyUpLParamFromSignature(uint64_t signature)
{
  // Rebuild the keyup LPARAM from the tracked child keydown signature so the
  // synthetic release matches the original key as closely as possible.
  // Flutter's Windows keyboard path uses both WPARAM and LPARAM fields
  // (scancode / extended bit / system-key bit / transition bits) when mapping
  // the event, so sending only VK_ESCAPE/VK_RETURN without the original shape
  // risks clearing the wrong key or being ignored by the engine.
  LPARAM lparam = 1;
  lparam |= static_cast<LPARAM>((signature >> 16) & 0xFF) << 16;
  lparam |= static_cast<LPARAM>((signature >> 24) & 0x1) << 24;

  if (((signature >> 25) & 0x1) != 0)
  {
    lparam |= static_cast<LPARAM>(1) << 29;
  }

  lparam |= static_cast<LPARAM>(1) << 30;
  lparam |= static_cast<LPARAM>(1) << 31;
  return lparam;
}

static std::optional<WORD> ParseWindowsVirtualKey(const std::string &key)
{
  if (key.size() == 1)
  {
    const unsigned char ch = static_cast<unsigned char>(key[0]);
    if (std::isalpha(ch))
    {
      return static_cast<WORD>(std::toupper(ch));
    }

    if (std::isdigit(ch))
    {
      return static_cast<WORD>(ch);
    }
  }

  if (key == "alt")
    return static_cast<WORD>(VK_LMENU);
  if (key == "control")
    return static_cast<WORD>(VK_LCONTROL);
  if (key == "shift")
    return static_cast<WORD>(VK_LSHIFT);
  if (key == "meta")
    return static_cast<WORD>(VK_LWIN);
  if (key == "escape")
    return static_cast<WORD>(VK_ESCAPE);
  if (key == "enter")
    return static_cast<WORD>(VK_RETURN);
  if (key == "tab")
    return static_cast<WORD>(VK_TAB);
  if (key == "space")
    return static_cast<WORD>(VK_SPACE);
  if (key == "arrowUp")
    return static_cast<WORD>(VK_UP);
  if (key == "arrowDown")
    return static_cast<WORD>(VK_DOWN);
  if (key == "arrowLeft")
    return static_cast<WORD>(VK_LEFT);
  if (key == "arrowRight")
    return static_cast<WORD>(VK_RIGHT);

  return std::nullopt;
}

static bool PostWindowsKeyMessage(HWND hwnd, WORD virtual_key, bool key_up, bool system_key)
{
  if (hwnd == nullptr)
  {
    return false;
  }

  UINT message = key_up ? (system_key ? WM_SYSKEYUP : WM_KEYUP) : (system_key ? WM_SYSKEYDOWN : WM_KEYDOWN);
  LPARAM lparam = 1;
  lparam |= static_cast<LPARAM>(MapVirtualKey(virtual_key, MAPVK_VK_TO_VSC)) << 16;
  if (system_key)
  {
    lparam |= static_cast<LPARAM>(1) << 29;
  }
  if (key_up)
  {
    lparam |= static_cast<LPARAM>(1) << 30;
    lparam |= static_cast<LPARAM>(1) << 31;
  }

  return PostMessage(hwnd, message, virtual_key, lparam) != 0;
}

static std::optional<DWORD> ParseWindowsMouseFlag(const std::string &button, bool button_up)
{
  if (button == "left")
    return button_up ? MOUSEEVENTF_LEFTUP : MOUSEEVENTF_LEFTDOWN;
  if (button == "right")
    return button_up ? MOUSEEVENTF_RIGHTUP : MOUSEEVENTF_RIGHTDOWN;
  if (button == "middle")
    return button_up ? MOUSEEVENTF_MIDDLEUP : MOUSEEVENTF_MIDDLEDOWN;
  return std::nullopt;
}

static bool SendWindowsMouseButtonInput(DWORD mouse_flag)
{
  INPUT input = {};
  input.type = INPUT_MOUSE;
  input.mi.dwFlags = mouse_flag;
  return SendInput(1, &input, sizeof(INPUT)) == 1;
}

static bool SendWindowsMouseWheelInput(int wheel_delta)
{
  // Scrolling capture needs to drive the window under the cursor after the Wox overlay hides. A
  // wheel SendInput event matches normal user scrolling and avoids coupling the feature to UIA or
  // browser-specific scroll APIs.
  INPUT input = {};
  input.type = INPUT_MOUSE;
  input.mi.dwFlags = MOUSEEVENTF_WHEEL;
  input.mi.mouseData = wheel_delta;
  return SendInput(1, &input, sizeof(INPUT)) == 1;
}

static void SetWindowsMousePassthrough(HWND hwnd, bool enabled)
{
  if (hwnd == nullptr || !IsWindow(hwnd))
  {
    return;
  }

  LONG_PTR ex_style = GetWindowLongPtr(hwnd, GWL_EXSTYLE);
  if (enabled)
  {
    ex_style |= WS_EX_TRANSPARENT;
  }
  else
  {
    ex_style &= ~WS_EX_TRANSPARENT;
  }
  SetWindowLongPtr(hwnd, GWL_EXSTYLE, ex_style);
}

static void SetWindowsScreenshotMousePassthrough(HWND root_hwnd, HWND child_hwnd, bool enabled)
{
  // Scrolling capture renders Flutter inside a child HWND on Windows. Making only the root
  // screenshot shell transparent was not enough: SendInput still hit the child Flutter view, so the
  // forwarded wheel event never reached the app under the selected region. Apply the same temporary
  // pass-through style to both windows so the synthetic wheel follows the real desktop target.
  SetWindowsMousePassthrough(root_hwnd, enabled);
  SetWindowsMousePassthrough(child_hwnd, enabled);
}

FlutterWindow::FlutterWindow(const flutter::DartProject &project)
    : project_(project),
      original_window_proc_(nullptr),
      original_child_window_proc_(nullptr),
      child_window_(nullptr),
      previous_active_window_(nullptr)
{
  g_window_instance = this;
}

FlutterWindow::~FlutterWindow()
{
  // Clear global instance
  if (g_window_instance == this)
  {
    g_window_instance = nullptr;
  }
}

void FlutterWindow::Log(const std::string &message)
{
  if (window_manager_channel_)
  {
    window_manager_channel_->InvokeMethod("log", std::make_unique<flutter::EncodableValue>(message));
  }
}

std::string FlutterWindow::RectToString(const RECT &rect) const
{
  std::ostringstream oss;
  oss << "(" << rect.left << "," << rect.top << ")-(" << rect.right << "," << rect.bottom << ")";
  return oss.str();
}

RECT FlutterWindow::GetWindowRectSafe(HWND hwnd) const
{
  RECT rect{};
  if (hwnd != nullptr && IsWindow(hwnd))
  {
    GetWindowRect(hwnd, &rect);
  }
  return rect;
}

void FlutterWindow::SyncFlutterChildWindowToClientArea(HWND hwnd, const char *source, bool engine_handled)
{
  if (child_window_ == nullptr || !IsWindow(child_window_))
  {
    return;
  }

  RECT client_rect{};
  GetClientRect(hwnd, &client_rect);
  const int width = client_rect.right - client_rect.left;
  const int height = client_rect.bottom - client_rect.top;

  MoveWindow(child_window_, client_rect.left, client_rect.top, width, height, TRUE);

  const RECT child_rect = GetWindowRectSafe(child_window_);
  std::ostringstream oss;
  oss << source << ": engineHandled=" << (engine_handled ? "true" : "false")
      << ", client=" << RectToString(client_rect)
      << ", child=" << RectToString(child_rect);
  Log(oss.str());
}

void FlutterWindow::FocusFlutterViewOrRoot(HWND hwnd)
{
  // Keyboard shortcuts are delivered to Flutter through the hosted child HWND, not the top-level
  // runner HWND. Screenshot reveal used to focus the root window after WM_ACTIVATE had already
  // focused the child, so Escape/Enter never reached Dart during capture. Prefer the child and only
  // fall back to the root before Flutter has created or retained a valid view handle.
  if (child_window_ != nullptr && IsWindow(child_window_))
  {
    SetFocus(child_window_);
    return;
  }

  SetFocus(hwnd);
}

void FlutterWindow::TrackChildKeyDown(UINT message, WPARAM wparam, LPARAM lparam)
{
  if (!IsKeyDownMessage(message))
  {
    return;
  }

  // Repeat keydown messages should not create extra pending releases.
  if ((lparam & (static_cast<LPARAM>(1) << 30)) != 0)
  {
    return;
  }

  const uint64_t signature = MakeKeyboardMessageSignature(message, wparam, lparam);
  pending_child_keydowns_.insert(signature);
}

void FlutterWindow::ClearTrackedChildKeyDown(UINT message, WPARAM wparam, LPARAM lparam)
{
  if (!IsKeyUpMessage(message))
  {
    return;
  }

  const uint64_t signature = MakeKeyboardMessageSignature(message, wparam, lparam);
  pending_child_keydowns_.erase(signature);
}

bool FlutterWindow::HasTrackedChildKeyDown(UINT message, WPARAM wparam, LPARAM lparam) const
{
  if (!IsKeyUpMessage(message))
  {
    return false;
  }

  const uint64_t signature = MakeKeyboardMessageSignature(message, wparam, lparam);
  return pending_child_keydowns_.find(signature) != pending_child_keydowns_.end();
}

// Windows occasionally delivers the release for Enter/Escape-style actions
// to the top-level runner window after the keydown has already triggered a
// focus/view transition inside Flutter. In that case the engine sees the
// keydown on the child hwnd but ignores the matching keyup on the root hwnd,
// leaving HardwareKeyboard in a stale "pressed" state. The visible symptom is
// an every-other-press failure: one Enter works, the next one is ignored,
// then the following release clears the stale state again.

// Alternatives considered:
// 1. Reintroduce the old message-loop-to-Dart keyboard bridge.
//    Rejected because it duplicates the engine's keyboard pipeline and turns
//    a root/child routing bug into a broad Windows-only input hack.
// 2. Move all action execution to keyup in higher layers.
//    Rejected because it changes behavior outside Windows and spreads this
//    engine-specific issue into Dart UI code.
// 3. Fix the Flutter engine.
//    This is the ideal long-term solution, but it is outside the Wox runner.

// The chosen compromise is narrow and native: only when a non-repeat keydown
// definitely reached the child hwnd, and the matching keyup later lands on
// the root hwnd, send that release back to the child synchronously so the
// engine can clear its pressed state.
bool FlutterWindow::RerouteIgnoredRootKeyUp(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam)
{
  if (!IsKeyUpMessage(message) || child_window_ == nullptr || hwnd == nullptr)
  {
    return false;
  }

  if (!IsWindow(child_window_) || GetAncestor(child_window_, GA_ROOT) != hwnd)
  {
    return false;
  }

  if (!HasTrackedChildKeyDown(message, wparam, lparam))
  {
    return false;
  }

  SendMessage(child_window_, message, wparam, lparam);
  return true;
}

void FlutterWindow::FlushPendingChildKeyUps(bool skipPhysicallyHeld)
{
  if (pending_child_keydowns_.empty())
  {
    return;
  }

  if (child_window_ == nullptr || !IsWindow(child_window_))
  {
    return;
  }

  // Flush every still-pending child keydown as a synthetic keyup.
  //
  // This handles two scenarios:
  //   1. Hide-on-keydown (e.g. Escape, or a hotkey modifier such as Ctrl):
  //      child receives keydown -> Dart hides the window immediately -> the
  //      real keyup is delivered to whichever window gains focus next, NOT
  //      to Flutter -> HardwareKeyboard keeps the key marked as pressed.
  //   2. Defense-in-depth on show: if a previous hide-flush was ineffective
  //      (engine dropped the synthetic keyup), the show-flush retries.
  //
  // skipPhysicallyHeld controls how physically-depressed keys are handled:
  //   true  (show / capture paths): skip keys the OS reports as still held.
  //         WM_SETFOCUS will re-sync modifier state via GetKeyState, so
  //         sending an orphan keyup for a held key would confuse Flutter.
  //   false (hide path): flush unconditionally.  After SW_HIDE the real
  //         keyup goes to another window, so Flutter will never clear the
  //         pressed state on its own.  This is the fix for the bug where
  //         pressing a modifier-key hotkey to dismiss Wox leaves that
  //         modifier permanently "stuck" in HardwareKeyboard, causing the
  //         next keypress (e.g. 'A') to be misread as Ctrl+A / Alt+A.
  //
  // Take a snapshot for safe iteration: SendMessage below re-enters
  // ChildWindowProc which calls ClearTrackedChildKeyDown, removing
  // entries from pending_child_keydowns_ during iteration.
  const std::unordered_set<uint64_t> snapshot = pending_child_keydowns_;

  for (const uint64_t signature : snapshot)
  {
    const WPARAM vk = KeyboardVirtualKeyFromSignature(signature);

    // When skipPhysicallyHeld is true, skip keys that are still physically
    // pressed so that WM_SETFOCUS can re-sync them correctly on the next show.
    if (skipPhysicallyHeld && (GetAsyncKeyState(static_cast<int>(vk)) & 0x8000) != 0)
    {
      continue;
    }

    SendMessage(
        child_window_,
        KeyboardKeyUpMessageFromSignature(signature),
        vk,
        MakeKeyboardKeyUpLParamFromSignature(signature));
  }
}

HWND FlutterWindow::NormalizeToRootWindow(HWND hwnd) const
{
  if (hwnd == nullptr)
  {
    return nullptr;
  }

  HWND root = GetAncestor(hwnd, GA_ROOTOWNER);
  if (root == nullptr)
  {
    root = GetAncestor(hwnd, GA_ROOT);
  }
  if (root == nullptr)
  {
    root = hwnd;
  }

  return root;
}

bool FlutterWindow::ShouldSuppressBlurForActivatedWindow(HWND selfHwnd, HWND activatedHwnd)
{
  if (selfHwnd == nullptr || activatedHwnd == nullptr)
  {
    return false;
  }

  HWND selfRoot = NormalizeToRootWindow(selfHwnd);
  if (selfRoot == nullptr)
  {
    selfRoot = selfHwnd;
  }

  HWND activatedRoot = NormalizeToRootWindow(activatedHwnd);
  if (activatedRoot == nullptr)
  {
    activatedRoot = activatedHwnd;
  }

  if (activatedRoot == selfRoot || IsChild(selfRoot, activatedHwnd) || IsChild(selfRoot, activatedRoot))
  {
    Log("WM_ACTIVATE: WA_INACTIVE suppressed (same Wox window tree)");
    return true;
  }

  DWORD selfPid = 0;
  DWORD activatedPid = 0;
  GetWindowThreadProcessId(selfRoot, &selfPid);
  GetWindowThreadProcessId(activatedRoot, &activatedPid);
  if (selfPid != 0 && selfPid == activatedPid)
  {
    Log("WM_ACTIVATE: WA_INACTIVE suppressed (same process native host)");
    return true;
  }

  return false;
}

void FlutterWindow::SavePreviousActiveWindow(HWND selfHwnd)
{
  if (selfHwnd == nullptr)
  {
    return;
  }

  HWND fg = GetForegroundWindow();
  if (fg == nullptr)
  {
    return;
  }

  // Normalize to root window (avoid saving child controls)
  HWND root = NormalizeToRootWindow(fg);
  if (root == nullptr)
  {
    root = fg;
  }

  if (root == selfHwnd)
  {
    return;
  }

  if (!IsWindow(root) || !IsWindowVisible(root))
  {
    return;
  }

  previous_active_window_ = root;
  restore_previous_window_on_hide_ = true;

  char fgStr[32];
  sprintf_s(fgStr, "%p", previous_active_window_);
  Log(std::string("Window: saved previous foreground hwnd=") + fgStr);
}

void FlutterWindow::RestorePreviousActiveWindow(HWND selfHwnd)
{
  if (selfHwnd == nullptr)
  {
    return;
  }

  HWND prev = previous_active_window_;
  if (prev == nullptr)
  {
    Log("Window: no previous foreground window saved");
    return;
  }

  // Normalize again (in case we saved a non-root window in the past)
  HWND root = NormalizeToRootWindow(prev);
  if (root != nullptr)
  {
    prev = root;
  }

  if (prev == selfHwnd)
  {
    Log("Window: previous foreground is self, skip restore");
    return;
  }

  if (!IsWindow(prev))
  {
    Log("Window: previous foreground hwnd is invalid (destroyed?)");
    previous_active_window_ = nullptr;
    return;
  }

  char prevStr[32];
  sprintf_s(prevStr, "%p", prev);
  Log(std::string("Window: restoring previous foreground hwnd=") + prevStr);

  // If the previous window is minimized, do not restore it.
  // The user might have minimized it explicitly, and Wox being an overlay shouldn't change the window layout.
  if (IsIconic(prev))
  {
    Log("Window: previous foreground is minimized, skipping restore");
    previous_active_window_ = nullptr;
    return;
  }

  // Fast path: try directly.
  if (SetForegroundWindow(prev))
  {
    BringWindowToTop(prev);
    return;
  }

  // Fallback: Attach input queues temporarily.
  DWORD curTid = GetCurrentThreadId();
  DWORD prevTid = GetWindowThreadProcessId(prev, nullptr);
  bool attached = false;
  if (prevTid != 0 && prevTid != curTid)
  {
    attached = AttachThreadInput(prevTid, curTid, TRUE);
  }

  SetForegroundWindow(prev);
  BringWindowToTop(prev);

  if (attached)
  {
    AttachThreadInput(prevTid, curTid, FALSE);
  }

  if (GetForegroundWindow() == prev)
  {
    Log("Window: restore foreground succeeded (AttachThreadInput)");
    return;
  }

  // Last try: relax foreground restrictions.
  AllowSetForegroundWindow(ASFW_ANY);
  SetForegroundWindow(prev);
  BringWindowToTop(prev);
  Log("Window: restore foreground final attempt completed");
}

void FlutterWindow::DismissStartMenuIfOpen()
{
  HWND fg = GetForegroundWindow();
  if (!fg)
    return;

  WCHAR className[256] = {0};
  GetClassNameW(fg, className, 256);

  DWORD pid = 0;
  GetWindowThreadProcessId(fg, &pid);

  // Get process name for detection
  WCHAR exePath[MAX_PATH] = {0};
  WCHAR *fileName = nullptr;
  bool gotProcessName = false;

  if (pid != 0)
  {
    HANDLE hProcess = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid);
    if (hProcess)
    {
      DWORD pathLen = MAX_PATH;
      if (QueryFullProcessImageNameW(hProcess, 0, exePath, &pathLen))
      {
        gotProcessName = true;
        fileName = wcsrchr(exePath, L'\\');
        if (fileName)
          fileName++;
        else
          fileName = exePath;
      }
      CloseHandle(hProcess);
    }
  }

  // Detect Start Menu / Search overlay by window class or process name
  bool isStartMenu = false;

  // UWP apps (Start Menu, Search) use this window class
  if (wcscmp(className, L"Windows.UI.Core.CoreWindow") == 0)
  {
    isStartMenu = true;
  }

  if (!isStartMenu && gotProcessName && fileName)
  {
    if (_wcsicmp(fileName, L"StartMenuExperienceHost.exe") == 0 ||
        _wcsicmp(fileName, L"SearchHost.exe") == 0 ||
        _wcsicmp(fileName, L"SearchApp.exe") == 0 ||
        _wcsicmp(fileName, L"ShellExperienceHost.exe") == 0)
    {
      isStartMenu = true;
    }
  }

  if (!isStartMenu)
    return;

  Log("Focus: Start Menu detected, dismissing with WM_CLOSE");

  // Clear saved previous window if it was the Start Menu -- we don't want to
  // restore it when Wox hides.
  if (previous_active_window_ == fg || previous_active_window_ == GetAncestor(fg, GA_ROOT))
  {
    previous_active_window_ = nullptr;
  }

  // Post WM_CLOSE to dismiss the Start Menu window.
  // PostMessage bypasses UIPI restrictions that block SendInput.
  PostMessage(fg, WM_CLOSE, 0, 0);
  Sleep(200);
}

// Get the DPI scaling factor for the window
float FlutterWindow::GetDpiScale(HWND hwnd)
{
  // Default DPI is 96
  float dpiScale = 1.0f;

  // Try to use GetDpiForWindow which is available on Windows 10 1607 and later
  HMODULE user32 = GetModuleHandle(TEXT("user32.dll"));
  if (user32)
  {
    typedef UINT(WINAPI * GetDpiForWindowFunc)(HWND);
    GetDpiForWindowFunc getDpiForWindow =
        reinterpret_cast<GetDpiForWindowFunc>(GetProcAddress(user32, "GetDpiForWindow"));

    if (getDpiForWindow)
    {
      UINT dpi = getDpiForWindow(hwnd);
      dpiScale = dpi / 96.0f;
    }
    else
    {
      // Fallback for older Windows versions
      HDC hdc = GetDC(hwnd);
      if (hdc)
      {
        int dpiX = GetDeviceCaps(hdc, LOGPIXELSX);
        dpiScale = dpiX / 96.0f;
        ReleaseDC(hwnd, hdc);
      }
    }
  }

  return dpiScale;
}

void FlutterWindow::ReleaseDisplayCaptures(std::vector<CachedDisplayCapture> *captures)
{
  if (captures == nullptr)
  {
    return;
  }

  for (auto &capture : *captures)
  {
    if (capture.bitmap != nullptr)
    {
      DeleteObject(capture.bitmap);
      capture.bitmap = nullptr;
    }
  }
  captures->clear();
}

void FlutterWindow::ClearCachedDisplayCaptures()
{
  ReleaseDisplayCaptures(&cached_display_captures_);
}

bool FlutterWindow::CaptureDisplaySnapshots(std::vector<CachedDisplayCapture> *captures_out, std::string *error_out, const std::optional<RECT> &logical_selection)
{
  if (captures_out == nullptr || error_out == nullptr)
  {
    return false;
  }

  captures_out->clear();
  error_out->clear();
  const ULONGLONG capture_start = GetTickCount64();

  struct MonitorCaptureContext
  {
    std::vector<CachedDisplayCapture> *captures;
    std::string *error;
    const std::optional<RECT> *logical_selection;
  } context{captures_out, error_out, &logical_selection};

  const BOOL enumerated = EnumDisplayMonitors(
      nullptr,
      nullptr,
      [](HMONITOR monitor, HDC, LPRECT, LPARAM data) -> BOOL
      {
        auto *context = reinterpret_cast<MonitorCaptureContext *>(data);
        MONITORINFOEXW monitor_info{};
        monitor_info.cbSize = sizeof(MONITORINFOEXW);
        if (!GetMonitorInfoW(monitor, reinterpret_cast<MONITORINFO *>(&monitor_info)))
        {
          *context->error = "Failed to query monitor info";
          return FALSE;
        }

        RECT capture_bounds = monitor_info.rcMonitor;
        if (context->logical_selection->has_value())
        {
          RECT intersection{};
          if (!TryIntersectRects(monitor_info.rcMonitor, context->logical_selection->value(), &intersection))
          {
            return TRUE;
          }

          // Long screenshots only need the selected column/region. Capturing the intersection
          // before BitBlt avoids encoding full monitors while keeping preview and export sourced
          // from the exact same native pixels.
          capture_bounds = intersection;
        }

        const int width = capture_bounds.right - capture_bounds.left;
        const int height = capture_bounds.bottom - capture_bounds.top;
        if (width <= 0 || height <= 0)
        {
          *context->error = "Monitor has invalid bounds";
          return FALSE;
        }

        HDC screen_dc = GetDC(nullptr);
        if (screen_dc == nullptr)
        {
          *context->error = "Failed to access desktop device context";
          return FALSE;
        }

        HDC memory_dc = CreateCompatibleDC(screen_dc);
        HBITMAP bitmap = CreateCompatibleBitmap(screen_dc, width, height);
        if (memory_dc == nullptr || bitmap == nullptr)
        {
          if (bitmap != nullptr)
          {
            DeleteObject(bitmap);
          }
          if (memory_dc != nullptr)
          {
            DeleteDC(memory_dc);
          }
          ReleaseDC(nullptr, screen_dc);
          *context->error = "Failed to allocate monitor bitmap";
          return FALSE;
        }

        HGDIOBJ old_bitmap = SelectObject(memory_dc, bitmap);
        const BOOL copied = BitBlt(
            memory_dc,
            0,
            0,
            width,
            height,
            screen_dc,
            capture_bounds.left,
            capture_bounds.top,
            SRCCOPY | CAPTUREBLT);

        SelectObject(memory_dc, old_bitmap);
        DeleteDC(memory_dc);
        ReleaseDC(nullptr, screen_dc);

        if (!copied)
        {
          DeleteObject(bitmap);
          *context->error = "Failed to capture monitor bitmap";
          return FALSE;
        }

        DEVMODEW dev_mode{};
        dev_mode.dmSize = sizeof(DEVMODEW);
        int rotation = 0;
        if (EnumDisplaySettingsExW(monitor_info.szDevice, ENUM_CURRENT_SETTINGS, &dev_mode, 0))
        {
          switch (dev_mode.dmDisplayOrientation)
          {
          case DMDO_90:
            rotation = 90;
            break;
          case DMDO_180:
            rotation = 180;
            break;
          case DMDO_270:
            rotation = 270;
            break;
          default:
            rotation = 0;
            break;
          }
        }

        const UINT dpi = FlutterDesktopGetDpiForMonitor(monitor);
        const double scale = static_cast<double>(dpi) / 96.0;
        context->captures->push_back(CachedDisplayCapture{
            monitor_info.szDevice,
            capture_bounds,
            scale,
            rotation,
            bitmap,
        });
        return TRUE;
      },
      reinterpret_cast<LPARAM>(&context));

  if (!enumerated && error_out->empty())
  {
    *error_out = "Failed to enumerate display monitors";
  }

  if (logical_selection.has_value() && captures_out->empty() && error_out->empty())
  {
    *error_out = "Selection does not intersect any display monitor";
  }

  if (!error_out->empty())
  {
    ReleaseDisplayCaptures(captures_out);
    return false;
  }

  // Timing probe: metadata capture still needs BitBlt snapshots so later selected-display hydration
  // can reuse the cached HBITMAP. Logging only counts and elapsed time makes it clear whether the
  // remaining startup cost is capture itself or later PNG/Flutter work.
  std::ostringstream oss;
  oss << "screenshot_timing event=windows_native_capture displayCount=" << captures_out->size()
      << " selection=" << (logical_selection.has_value() ? RectToString(logical_selection.value()) : "null")
      << " elapsedMs=" << (GetTickCount64() - capture_start);
  Log(oss.str());
  return true;
}

bool FlutterWindow::BuildDisplaySnapshotPayloads(const std::vector<CachedDisplayCapture> &captures, bool include_image_bytes, flutter::EncodableList *snapshots_out, std::string *error_out)
{
  if (snapshots_out == nullptr || error_out == nullptr)
  {
    return false;
  }

  snapshots_out->clear();
  error_out->clear();
  const ULONGLONG payload_start = GetTickCount64();
  int encoded_count = 0;

  for (const auto &capture : captures)
  {
    const int width = capture.monitor_bounds.right - capture.monitor_bounds.left;
    const int height = capture.monitor_bounds.bottom - capture.monitor_bounds.top;

    flutter::EncodableMap snapshot;
    snapshot[flutter::EncodableValue("displayId")] = flutter::EncodableValue(Utf8FromUtf16(capture.display_id.c_str()));
    // The old Windows path eagerly PNG-encoded every monitor before the workspace even knew its
    // final bounds. Reusing cached monitor bitmaps lets Flutter ask for geometry first, then load
    // image payloads only after the native workspace shell has already been prepared.
    snapshot[flutter::EncodableValue("logicalBounds")] = flutter::EncodableValue(
        BuildRectValue(
            static_cast<double>(capture.monitor_bounds.left),
            static_cast<double>(capture.monitor_bounds.top),
            static_cast<double>(width),
            static_cast<double>(height)));
    snapshot[flutter::EncodableValue("pixelBounds")] = flutter::EncodableValue(
        BuildRectValue(
            static_cast<double>(capture.monitor_bounds.left),
            static_cast<double>(capture.monitor_bounds.top),
            static_cast<double>(width),
            static_cast<double>(height)));
    snapshot[flutter::EncodableValue("scale")] = flutter::EncodableValue(capture.scale);
    snapshot[flutter::EncodableValue("rotation")] = flutter::EncodableValue(capture.rotation);

    if (include_image_bytes)
    {
      std::string png_base64;
      if (!EncodeBitmapToPngBase64(capture.bitmap, png_base64, *error_out))
      {
        return false;
      }
      encoded_count += 1;
      snapshot[flutter::EncodableValue("imageBytesBase64")] = flutter::EncodableValue(png_base64);
    }

    snapshots_out->push_back(flutter::EncodableValue(snapshot));
  }

  // Timing probe: the original Windows startup encoded every monitor before reveal. This log proves
  // whether a call is metadata-only or limited to the selected display payload.
  std::ostringstream oss;
  oss << "screenshot_timing event=windows_payload_build displayCount=" << captures.size()
      << " encodedCount=" << encoded_count
      << " includeImageBytes=" << (include_image_bytes ? "true" : "false")
      << " elapsedMs=" << (GetTickCount64() - payload_start);
  Log(oss.str());
  return true;
}

const FlutterWindow::CachedDisplayCapture *FlutterWindow::FindCachedDisplayCapture(const std::string &display_id) const
{
  for (const auto &capture : cached_display_captures_)
  {
    if (Utf8FromUtf16(capture.display_id.c_str()) == display_id)
    {
      return &capture;
    }
  }

  return nullptr;
}

bool FlutterWindow::CachedDisplayCapturesMatch(const std::vector<std::string> &display_ids) const
{
  for (const auto &display_id : display_ids)
  {
    if (FindCachedDisplayCapture(display_id) == nullptr)
    {
      return false;
    }
  }

  return !display_ids.empty() || !cached_display_captures_.empty();
}

void FlutterWindow::PrepareCaptureWorkspace(HWND hwnd, const RECT &native_workspace_bounds)
{
  const ULONGLONG prepare_start = GetTickCount64();
  SavePreviousActiveWindow(hwnd);
  FlushPendingChildKeyUps();

  SetWindowPos(
      hwnd,
      HWND_TOPMOST,
      native_workspace_bounds.left,
      native_workspace_bounds.top,
      native_workspace_bounds.right - native_workspace_bounds.left,
      native_workspace_bounds.bottom - native_workspace_bounds.top,
      SWP_FRAMECHANGED | SWP_NOACTIVATE);

  if (flutter_controller_)
  {
    flutter_controller_->ForceRedraw();
  }
  SyncFlutterChildWindowToClientArea(hwnd, "prepareCaptureWorkspace", false);

  screenshot_presentation_state_.prepared = true;
  screenshot_presentation_state_.active = false;
  screenshot_presentation_state_.workspace_scale = static_cast<double>(GetDpiScale(hwnd));
  screenshot_presentation_state_.native_workspace_bounds = native_workspace_bounds;

  std::ostringstream oss;
  oss << "screenshot_timing event=windows_prepare_workspace bounds=" << RectToString(native_workspace_bounds)
      << " elapsedMs=" << (GetTickCount64() - prepare_start);
  Log(oss.str());
}

void FlutterWindow::RevealPreparedCaptureWorkspace(HWND hwnd)
{
  if (!screenshot_presentation_state_.prepared)
  {
    return;
  }

  blur_guard_active_ = true;
  blur_guard_until_tick_ = GetTickCount64() + kPostShowBlurGraceMs;

  const RECT &native_workspace_bounds = screenshot_presentation_state_.native_workspace_bounds;
  const ULONGLONG reveal_start = GetTickCount64();
  SetWindowPos(
      hwnd,
      HWND_TOPMOST,
      native_workspace_bounds.left,
      native_workspace_bounds.top,
      native_workspace_bounds.right - native_workspace_bounds.left,
      native_workspace_bounds.bottom - native_workspace_bounds.top,
      SWP_FRAMECHANGED | SWP_SHOWWINDOW);

  SyncFlutterChildWindowToClientArea(hwnd, "revealPreparedCaptureWorkspace", false);

  // Screenshot capture replaces the standard show() -> focus() sequence on Windows because the
  // generic window-manager path assumes one monitor/DPI. Reapplying the focus restore steps here
  // keeps the prepared capture overlay interactive without reusing the single-monitor geometry path.
  DismissStartMenuIfOpen();
  SavePreviousActiveWindow(hwnd);
  if (!SetForegroundWindow(hwnd))
  {
    AllowSetForegroundWindow(ASFW_ANY);
    SetForegroundWindow(hwnd);
  }
  FocusFlutterViewOrRoot(hwnd);
  BringWindowToTop(hwnd);
  blur_guard_active_ = false;

  screenshot_presentation_state_.prepared = false;
  screenshot_presentation_state_.active = true;

  std::ostringstream oss;
  oss << "screenshot_timing event=windows_reveal_workspace bounds=" << RectToString(native_workspace_bounds)
      << " elapsedMs=" << (GetTickCount64() - reveal_start);
  Log(oss.str());
}

flutter::EncodableMap FlutterWindow::BuildCaptureWorkspaceResponse(const RECT &native_workspace_bounds) const
{
  flutter::EncodableMap response;
  response[flutter::EncodableValue("workspaceBounds")] = flutter::EncodableValue(BuildScaledRectValue(native_workspace_bounds, screenshot_presentation_state_.workspace_scale));
  response[flutter::EncodableValue("workspaceScale")] = flutter::EncodableValue(screenshot_presentation_state_.workspace_scale);
  response[flutter::EncodableValue("presentedByPlatform")] = flutter::EncodableValue(true);
  return response;
}

bool FlutterWindow::BeginScreenshotSelectionOverlay(HWND hwnd, const RECT &workspace_bounds, std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result, std::string *error_out)
{
  if (screenshot_selection_overlay_state_.active)
  {
    if (error_out != nullptr)
    {
      *error_out = "A screenshot selection session is already active";
    }
    if (result != nullptr)
    {
      result->Error("SELECTION_ERROR", "A screenshot selection session is already active");
    }
    return false;
  }

  static bool input_class_registered = false;
  if (!input_class_registered)
  {
    WNDCLASS window_class{};
    window_class.hCursor = LoadCursor(nullptr, IDC_CROSS);
    window_class.lpszClassName = kScreenshotSelectionInputWindowClassName;
    window_class.hInstance = GetModuleHandle(nullptr);
    window_class.hbrBackground = reinterpret_cast<HBRUSH>(GetStockObject(BLACK_BRUSH));
    window_class.lpfnWndProc = FlutterWindow::ScreenshotSelectionInputWindowProc;
    RegisterClass(&window_class);
    input_class_registered = true;
  }

  static bool border_class_registered = false;
  if (!border_class_registered)
  {
    WNDCLASS window_class{};
    window_class.hCursor = LoadCursor(nullptr, IDC_CROSS);
    window_class.lpszClassName = kScreenshotSelectionBorderWindowClassName;
    window_class.hInstance = GetModuleHandle(nullptr);
    window_class.hbrBackground = ScreenshotSelectionBorderBrush();
    window_class.lpfnWndProc = FlutterWindow::ScreenshotSelectionPassiveWindowProc;
    RegisterClass(&window_class);
    border_class_registered = true;
  }

  SavePreviousActiveWindow(hwnd);
  FlushPendingChildKeyUps();

  screenshot_selection_overlay_state_.active = true;
  screenshot_selection_overlay_state_.dragging = false;
  screenshot_selection_overlay_state_.completed = false;
  screenshot_selection_overlay_state_.workspace_bounds = workspace_bounds;
  screenshot_selection_overlay_state_.selection_bounds = {0, 0, 0, 0};
  screenshot_selection_overlay_state_.started_tick = GetTickCount64();

  const int workspace_width = workspace_bounds.right - workspace_bounds.left;
  const int workspace_height = workspace_bounds.bottom - workspace_bounds.top;
  HWND input_window = CreateWindowEx(
      WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_LAYERED,
      kScreenshotSelectionInputWindowClassName,
      L"Wox screenshot selection input",
      WS_POPUP,
      workspace_bounds.left,
      workspace_bounds.top,
      workspace_width,
      workspace_height,
      nullptr,
      nullptr,
      GetModuleHandle(nullptr),
      nullptr);
  if (input_window == nullptr)
  {
    screenshot_selection_overlay_state_ = ScreenshotSelectionOverlayState{};
    if (error_out != nullptr)
    {
      *error_out = "Failed to create screenshot selection input window";
    }
    if (result != nullptr)
    {
      result->Error("SELECTION_ERROR", "Failed to create screenshot selection input window");
    }
    return false;
  }
  screenshot_selection_overlay_state_.input_window = input_window;
  SetLayeredWindowAttributes(input_window, 0, 118, LWA_ALPHA);

  for (int i = 0; i < 4; ++i)
  {
    HWND border_window = CreateWindowEx(
        WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE,
        kScreenshotSelectionBorderWindowClassName,
        L"Wox screenshot selection border",
        WS_POPUP,
        workspace_bounds.left,
        workspace_bounds.top,
        1,
        1,
        nullptr,
        nullptr,
        GetModuleHandle(nullptr),
        nullptr);
    if (border_window == nullptr)
    {
      DestroyScreenshotSelectionOverlayWindows();
      screenshot_selection_overlay_state_ = ScreenshotSelectionOverlayState{};
      if (error_out != nullptr)
      {
        *error_out = "Failed to create screenshot selection border windows";
      }
      if (result != nullptr)
      {
        result->Error("SELECTION_ERROR", "Failed to create screenshot selection border windows");
      }
      return false;
    }

    screenshot_selection_overlay_state_.border_windows.push_back(border_window);
  }

  screenshot_selection_overlay_state_.pending_result = std::move(result);

  // Optimization: do not paint cached HBITMAP snapshots into the selection UI. The old attempt made
  // the native handoff wait on full-screen GDI/GDI+ repaint and produced a visible top-to-bottom
  // shade sweep on large desktops. The first fast overlay used several topmost helper windows, but
  // updating the full-screen dim region during every mouse move still delayed the first feedback.
  // Keep the dim window stable while dragging and move only ordinary green border windows; using
  // WS_EX_TRANSPARENT/layered border windows delayed their paint behind the dimming surface.
  MoveSelectionOverlayWindow(input_window, workspace_bounds, true);
  LayoutScreenshotSelectionOverlay();
  SetForegroundWindow(input_window);
  SetActiveWindow(input_window);
  SetFocus(input_window);

  // Bug fix: the layered full-screen selection HWND can be visible before Windows has routed the
  // first captured drag messages to it, which produced several frames where only the dim mask and
  // cursor moved. A low-level hook observes the active left-button gesture to seed immediate native
  // feedback, but it does not swallow mouse events because blocking the system input path can make
  // dragging feel stuck. The HWND handlers below remain the authoritative capture fallback.
  screenshot_selection_overlay_state_.mouse_hook = SetWindowsHookEx(WH_MOUSE_LL, FlutterWindow::ScreenshotSelectionMouseHookProc, GetModuleHandle(nullptr), 0);

  POINT cursor_position{};
  if (GetCursorPos(&cursor_position))
  {
    const CachedDisplayCapture *cursor_capture = DisplayCaptureForPoint(cursor_position);
    if (cursor_capture != nullptr)
    {
      // Optimization: prewarm once when the native overlay appears, not during drag moves. That
      // overlaps selected-display hydration with the user's selection time while keeping mouse-move
      // feedback entirely native and free from PNG/base64 work.
      EmitScreenshotSelectionDisplayHint(*cursor_capture);
    }
  }

  std::ostringstream oss;
  oss << "screenshot_timing event=windows_native_selection_begin displayCount=" << cached_display_captures_.size()
      << " workspace=" << RectToString(workspace_bounds)
      << " mouseHook=" << (screenshot_selection_overlay_state_.mouse_hook != nullptr ? "true" : "false");
  Log(oss.str());

  return true;
}

void FlutterWindow::MoveSelectionOverlayWindow(HWND hwnd, const RECT &bounds, bool activate)
{
  if (hwnd == nullptr || !IsWindow(hwnd))
  {
    return;
  }

  if (IsRectEmptyOrInvalid(bounds))
  {
    ShowWindow(hwnd, SW_HIDE);
    return;
  }

  SetWindowPos(
      hwnd,
      HWND_TOPMOST,
      bounds.left,
      bounds.top,
      bounds.right - bounds.left,
      bounds.bottom - bounds.top,
      SWP_SHOWWINDOW | (activate ? 0 : SWP_NOACTIVATE));
  RedrawWindow(hwnd, nullptr, nullptr, RDW_INVALIDATE | RDW_ERASE | RDW_UPDATENOW);
}

void FlutterWindow::LayoutScreenshotSelectionOverlay()
{
  if (!screenshot_selection_overlay_state_.active)
  {
    return;
  }

  const RECT workspace = screenshot_selection_overlay_state_.workspace_bounds;
  const RECT selection = ClampRectToBounds(screenshot_selection_overlay_state_.selection_bounds, workspace);
  const bool has_selection = !IsRectEmptyOrInvalid(selection);
  HWND input_window = screenshot_selection_overlay_state_.input_window;
  auto &border_windows = screenshot_selection_overlay_state_.border_windows;

  if (!has_selection)
  {
    SetOwnedWindowRegion(input_window, nullptr);
    InvalidateRect(input_window, nullptr, TRUE);
    for (const auto border_window : border_windows)
    {
      if (border_window != nullptr && IsWindow(border_window))
      {
        ShowWindow(border_window, SW_HIDE);
      }
    }
    return;
  }

  if (border_windows.size() >= 4)
  {
    const int border = 2;
    // Bug fix: the full-screen dim region is expensive to recompute on large virtual desktops.
    // Drag feedback now repaints four tiny green border windows synchronously, so the selection
    // follows the cursor instead of waiting for a later compositor repaint at the drag origin.
    MoveSelectionOverlayWindow(border_windows[0], RECT{selection.left, selection.top, selection.right, selection.top + border});
    MoveSelectionOverlayWindow(border_windows[1], RECT{selection.left, selection.bottom - border, selection.right, selection.bottom});
    MoveSelectionOverlayWindow(border_windows[2], RECT{selection.left, selection.top, selection.left + border, selection.bottom});
    MoveSelectionOverlayWindow(border_windows[3], RECT{selection.right - border, selection.top, selection.right, selection.bottom});
  }
}

void FlutterWindow::ScheduleScreenshotSelectionDimRegionUpdate()
{
  if (!screenshot_selection_overlay_state_.active || screenshot_selection_overlay_state_.input_window == nullptr || !IsWindow(screenshot_selection_overlay_state_.input_window))
  {
    return;
  }

  screenshot_selection_overlay_state_.dim_region_dirty = true;
  if (screenshot_selection_overlay_state_.dim_region_update_posted)
  {
    return;
  }

  // Bug fix: Windows used to leave the selected center dimmed until mouse-up because rebuilding the
  // full-screen region synchronously on every mouse move made large virtual desktops stutter. A
  // posted message still coalesces repeated moves, but unlike WM_TIMER it is not held behind the
  // system timer cadence, so the cut-out follows fast drags more closely.
  if (PostMessage(screenshot_selection_overlay_state_.input_window, kScreenshotSelectionDimRegionUpdateMessage, 0, 0))
  {
    screenshot_selection_overlay_state_.dim_region_update_posted = true;
    return;
  }

  // If Windows cannot queue the message, prefer visual correctness over the old permanently dimmed
  // selection center; this fallback is rare and still avoids touching Flutter state.
  ApplyScreenshotSelectionDimRegion();
}

void FlutterWindow::FlushScreenshotSelectionDimRegionUpdate()
{
  if (!screenshot_selection_overlay_state_.active)
  {
    return;
  }

  if (!screenshot_selection_overlay_state_.dim_region_dirty)
  {
    CancelScreenshotSelectionDimRegionUpdate();
    return;
  }

  ApplyScreenshotSelectionDimRegion();
}

void FlutterWindow::CancelScreenshotSelectionDimRegionUpdate()
{
  screenshot_selection_overlay_state_.dim_region_update_posted = false;
  screenshot_selection_overlay_state_.dim_region_dirty = false;
}

void FlutterWindow::ApplyScreenshotSelectionDimRegion()
{
  if (!screenshot_selection_overlay_state_.active)
  {
    return;
  }

  screenshot_selection_overlay_state_.dim_region_update_posted = false;
  screenshot_selection_overlay_state_.dim_region_dirty = false;

  const RECT workspace = screenshot_selection_overlay_state_.workspace_bounds;
  const RECT selection = ClampRectToBounds(screenshot_selection_overlay_state_.selection_bounds, workspace);
  if (IsRectEmptyOrInvalid(selection))
  {
    SetOwnedWindowRegion(screenshot_selection_overlay_state_.input_window, nullptr);
    return;
  }

  // The dimming cut-out is intentionally applied outside the synchronous border-layout path. That
  // keeps the cursor-following border responsive while the selected center still becomes undimmed
  // during drag instead of waiting until the selection completes.
  SetOwnedWindowRegion(screenshot_selection_overlay_state_.input_window, CreateSelectionDimRegion(workspace, selection));
}

void FlutterWindow::UpdateScreenshotSelectionOverlay(const RECT &selection_bounds)
{
  if (!screenshot_selection_overlay_state_.active)
  {
    return;
  }

  const RECT clamped_selection = ClampRectToBounds(selection_bounds, screenshot_selection_overlay_state_.workspace_bounds);
  screenshot_selection_overlay_state_.selection_bounds = clamped_selection;
  // Optimization: mouse-drag feedback stays entirely native. Earlier drag-time display hints made
  // Flutter hydrate/encode a monitor snapshot mid-drag and caused a one-time pause; the final
  // selection result prepares the selected display before Flutter is revealed.
  LayoutScreenshotSelectionOverlay();
  if (IsRectEmptyOrInvalid(clamped_selection))
  {
    CancelScreenshotSelectionDimRegionUpdate();
    return;
  }

  ScheduleScreenshotSelectionDimRegionUpdate();
}

void FlutterWindow::CompleteScreenshotSelectionOverlay(bool cancelled)
{
  if (!screenshot_selection_overlay_state_.active || screenshot_selection_overlay_state_.completed)
  {
    return;
  }

  auto pending_result = std::move(screenshot_selection_overlay_state_.pending_result);
  const RECT selection_bounds = screenshot_selection_overlay_state_.selection_bounds;
  const bool effective_cancelled = cancelled || IsRectEmptyOrInvalid(selection_bounds);
  const ULONGLONG started_tick = screenshot_selection_overlay_state_.started_tick;
  const CachedDisplayCapture *preferred_capture = PreferredDisplayCaptureForSelection(selection_bounds);
  const HWND input_window = screenshot_selection_overlay_state_.input_window;
  if (screenshot_selection_overlay_state_.mouse_hook != nullptr)
  {
    // Bug fix: the hook exists only while the user is drawing the native rectangle. Stop it as soon
    // as selection completes so later clicks on the Flutter editor or the desktop are not swallowed.
    UnhookWindowsHookEx(screenshot_selection_overlay_state_.mouse_hook);
    screenshot_selection_overlay_state_.mouse_hook = nullptr;
  }
  if (screenshot_selection_overlay_state_.dragging || GetCapture() == input_window)
  {
    // Bug fix: completion can arrive from either the HWND capture path or the low-level hook path.
    // Release capture in one shared place so a completed or cancelled drag cannot leave Windows
    // routing the next mouse gesture to the overlay window.
    screenshot_selection_overlay_state_.dragging = false;
    ReleaseCapture();
  }

  flutter::EncodableMap response;
  response[flutter::EncodableValue("wasHandled")] = flutter::EncodableValue(true);
  if (effective_cancelled)
  {
    response[flutter::EncodableValue("selection")] = flutter::EncodableValue();
    response[flutter::EncodableValue("editorVisibleBounds")] = flutter::EncodableValue();
    DestroyScreenshotSelectionOverlayWindows();
    screenshot_selection_overlay_state_ = ScreenshotSelectionOverlayState{};
  }
  else
  {
    // The latest scheduled cut-out must be visible before the native overlay hands selection back
    // to Flutter; otherwise the last drag frame can briefly show the old dimmed center.
    FlushScreenshotSelectionDimRegionUpdate();
    response[flutter::EncodableValue("selection")] = flutter::EncodableValue(BuildRectValue(selection_bounds));
    response[flutter::EncodableValue("editorVisibleBounds")] = flutter::EncodableValue(BuildRectValue(preferred_capture != nullptr ? preferred_capture->monitor_bounds : selection_bounds));
    screenshot_selection_overlay_state_.dragging = false;
    screenshot_selection_overlay_state_.completed = true;
  }

  std::ostringstream oss;
  oss << "screenshot_timing event=windows_native_selection_complete cancelled=" << (effective_cancelled ? "true" : "false")
      << " selection=" << RectToString(selection_bounds)
      << " elapsedMs=" << (started_tick == 0 ? 0 : GetTickCount64() - started_tick);
  Log(oss.str());

  if (pending_result != nullptr)
  {
    pending_result->Success(flutter::EncodableValue(response));
  }
}

void FlutterWindow::DismissNativeSelectionOverlays()
{
  if (!screenshot_selection_overlay_state_.active)
  {
    return;
  }

  auto pending_result = std::move(screenshot_selection_overlay_state_.pending_result);
  DestroyScreenshotSelectionOverlayWindows();
  screenshot_selection_overlay_state_ = ScreenshotSelectionOverlayState{};
  Log("screenshot_timing event=windows_native_selection_dismiss");

  if (pending_result != nullptr)
  {
    flutter::EncodableMap response;
    response[flutter::EncodableValue("wasHandled")] = flutter::EncodableValue(true);
    response[flutter::EncodableValue("selection")] = flutter::EncodableValue();
    response[flutter::EncodableValue("editorVisibleBounds")] = flutter::EncodableValue();
    pending_result->Success(flutter::EncodableValue(response));
  }
}

void FlutterWindow::DestroyScreenshotSelectionOverlayWindows()
{
  CancelScreenshotSelectionDimRegionUpdate();

  if (screenshot_selection_overlay_state_.mouse_hook != nullptr)
  {
    // Bug fix: cleanup may run through cancellation, window teardown, or Dart dismiss. Unhooking
    // here prevents a stale low-level hook from pointing at destroyed overlay state.
    UnhookWindowsHookEx(screenshot_selection_overlay_state_.mouse_hook);
    screenshot_selection_overlay_state_.mouse_hook = nullptr;
  }

  if (screenshot_selection_overlay_state_.input_window != nullptr && IsWindow(screenshot_selection_overlay_state_.input_window))
  {
    DestroyWindow(screenshot_selection_overlay_state_.input_window);
  }
  screenshot_selection_overlay_state_.input_window = nullptr;

  for (const auto border_window : screenshot_selection_overlay_state_.border_windows)
  {
    if (border_window != nullptr && IsWindow(border_window))
    {
      DestroyWindow(border_window);
    }
  }
  screenshot_selection_overlay_state_.border_windows.clear();
}

const FlutterWindow::CachedDisplayCapture *FlutterWindow::PreferredDisplayCaptureForSelection(const RECT &selection_bounds) const
{
  if (IsRectEmptyOrInvalid(selection_bounds))
  {
    return nullptr;
  }

  const POINT center{
      selection_bounds.left + (selection_bounds.right - selection_bounds.left) / 2,
      selection_bounds.top + (selection_bounds.bottom - selection_bounds.top) / 2};
  for (const auto &capture : cached_display_captures_)
  {
    if (IsPointInRect(capture.monitor_bounds, center))
    {
      return &capture;
    }
  }

  const CachedDisplayCapture *best_capture = nullptr;
  double best_area = 0;
  for (const auto &capture : cached_display_captures_)
  {
    const double area = RectIntersectionArea(capture.monitor_bounds, selection_bounds);
    if (area > best_area)
    {
      best_area = area;
      best_capture = &capture;
    }
  }
  return best_capture;
}

const FlutterWindow::CachedDisplayCapture *FlutterWindow::DisplayCaptureForPoint(POINT point) const
{
  for (const auto &capture : cached_display_captures_)
  {
    if (IsPointInRect(capture.monitor_bounds, point))
    {
      return &capture;
    }
  }
  return nullptr;
}

void FlutterWindow::EmitScreenshotSelectionDisplayHint(const CachedDisplayCapture &capture)
{
  if (!window_manager_channel_)
  {
    return;
  }

  flutter::EncodableMap payload;
  payload[flutter::EncodableValue("displayId")] = flutter::EncodableValue(Utf8FromUtf16(capture.display_id.c_str()));
  payload[flutter::EncodableValue("displayBounds")] = flutter::EncodableValue(BuildRectValue(capture.monitor_bounds));
  window_manager_channel_->InvokeMethod("onSelectionDisplayHint", std::make_unique<flutter::EncodableValue>(payload));

  std::ostringstream oss;
  oss << "screenshot_timing event=windows_native_selection_hint displayId=" << Utf8FromUtf16(capture.display_id.c_str())
      << " bounds=" << RectToString(capture.monitor_bounds);
  Log(oss.str());
}

void FlutterWindow::BeginScrollingCaptureOverlay(HWND hwnd, const RECT &workspace_bounds, const RECT &selection_bounds, const RECT &controls_bounds)
{
  DismissScrollingCaptureOverlay();

  static bool overlay_class_registered = false;
  if (!overlay_class_registered)
  {
    WNDCLASS window_class{};
    window_class.hCursor = LoadCursor(nullptr, IDC_ARROW);
    window_class.lpszClassName = kScrollingCaptureOverlayWindowClassName;
    window_class.hInstance = GetModuleHandle(nullptr);
    window_class.hbrBackground = nullptr;
    window_class.lpfnWndProc = FlutterWindow::ScrollingCaptureOverlayWindowProc;
    RegisterClass(&window_class);
    overlay_class_registered = true;
  }

  scrolling_capture_overlay_state_.active = true;
  scrolling_capture_overlay_state_.selection_bounds = selection_bounds;

  const int workspace_width = workspace_bounds.right - workspace_bounds.left;
  const int workspace_height = workspace_bounds.bottom - workspace_bounds.top;
  HWND overlay_window = CreateWindowEx(
      WS_EX_LAYERED | WS_EX_TRANSPARENT | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE,
      kScrollingCaptureOverlayWindowClassName,
      L"Wox scrolling capture overlay",
      WS_POPUP,
      workspace_bounds.left,
      workspace_bounds.top,
      workspace_width,
      workspace_height,
      nullptr,
      nullptr,
      GetModuleHandle(nullptr),
      nullptr);
  scrolling_capture_overlay_state_.overlay_window = overlay_window;
  if (overlay_window != nullptr)
  {
    const RECT local_selection{
        selection_bounds.left - workspace_bounds.left,
        selection_bounds.top - workspace_bounds.top,
        selection_bounds.right - workspace_bounds.left,
        selection_bounds.bottom - workspace_bounds.top};
    HRGN full_region = CreateRectRgn(0, 0, workspace_width, workspace_height);
    HRGN selection_region = CreateRectRgn(local_selection.left, local_selection.top, local_selection.right, local_selection.bottom);
    CombineRgn(full_region, full_region, selection_region, RGN_DIFF);
    SetWindowRgn(overlay_window, full_region, TRUE);
    DeleteObject(selection_region);

    // Match the macOS scrolling mask behavior with a passive topmost overlay. The selected region is
    // physically cut out of the HWND region, so mouse wheel input and BitBlt selection captures both
    // see the native app underneath instead of the Wox screenshot shell.
    SetLayeredWindowAttributes(overlay_window, 0, 118, LWA_ALPHA);
    SetWindowPos(overlay_window, HWND_TOPMOST, workspace_bounds.left, workspace_bounds.top, workspace_width, workspace_height, SWP_NOACTIVATE | SWP_SHOWWINDOW);
    InvalidateRect(overlay_window, nullptr, TRUE);
  }

  SetScrollingCaptureControlsBackdrop(hwnd, true);
  MoveScrollingCaptureControlsWindow(hwnd, controls_bounds);
  scrolling_capture_overlay_state_.mouse_hook = SetWindowsHookEx(WH_MOUSE_LL, FlutterWindow::ScrollingCaptureMouseHookProc, GetModuleHandle(nullptr), 0);
}

void FlutterWindow::DismissScrollingCaptureOverlay()
{
  const bool was_active = scrolling_capture_overlay_state_.active;

  ClearScrollingCaptureControlsRegion();
  if (was_active)
  {
    SetScrollingCaptureControlsBackdrop(GetHandle(), false);
  }

  if (scrolling_capture_overlay_state_.mouse_hook != nullptr)
  {
    UnhookWindowsHookEx(scrolling_capture_overlay_state_.mouse_hook);
    scrolling_capture_overlay_state_.mouse_hook = nullptr;
  }

  if (scrolling_capture_overlay_state_.overlay_window != nullptr)
  {
    DestroyWindow(scrolling_capture_overlay_state_.overlay_window);
    scrolling_capture_overlay_state_.overlay_window = nullptr;
  }

  scrolling_capture_overlay_state_.active = false;
  scrolling_capture_overlay_state_.selection_bounds = {0, 0, 0, 0};
}

void FlutterWindow::MoveScrollingCaptureControlsWindow(HWND hwnd, const RECT &controls_bounds)
{
  SetWindowPos(
      hwnd,
      HWND_TOPMOST,
      controls_bounds.left,
      controls_bounds.top,
      controls_bounds.right - controls_bounds.left,
      controls_bounds.bottom - controls_bounds.top,
      SWP_FRAMECHANGED | SWP_SHOWWINDOW);
  SyncFlutterChildWindowToClientArea(hwnd, "beginScrollingCaptureOverlay", false);
  ApplyScrollingCaptureControlsRegion(hwnd);
  if (flutter_controller_)
  {
    flutter_controller_->ForceRedraw();
  }
  FocusFlutterViewOrRoot(hwnd);
}

void FlutterWindow::SetScrollingCaptureControlsBackdrop(HWND hwnd, bool compact)
{
  if (hwnd == nullptr || !IsWindow(hwnd))
  {
    return;
  }

  if (compact)
  {
    // Bug fix: the normal Wox window uses an acrylic/Mica DWM backdrop. During scrolling capture the
    // compact Flutter view intentionally leaves pixels transparent around the confirm/cancel capsule,
    // but DWM filled those pixels with the window material instead of revealing the native dimming
    // overlay. Disable the backdrop while the compact controls are active so SetWindowRgn is the only
    // shape that contributes visible pixels.
    MARGINS margins = {0, 0, 0, 0};
    DwmExtendFrameIntoClientArea(hwnd, &margins);
    int backdrop_type = kDwmSystemBackdropNone;
    DwmSetWindowAttribute(hwnd, DWMWA_SYSTEMBACKDROP_TYPE, &backdrop_type, sizeof(backdrop_type));
    int corner_preference = kDwmCornerDoNotRound;
    DwmSetWindowAttribute(hwnd, DWMWA_WINDOW_CORNER_PREFERENCE, &corner_preference, sizeof(corner_preference));
  }
  else
  {
    // Restore the same window material used by the normal launcher window after scrolling capture
    // dismisses. Keeping the restore local to the overlay lifecycle avoids changing regular Wox
    // chrome while still removing the compact toolbar backing panel.
    MARGINS margins = {-1};
    DwmExtendFrameIntoClientArea(hwnd, &margins);
    int backdrop_type = kDwmSystemBackdropTabbed;
    DwmSetWindowAttribute(hwnd, DWMWA_SYSTEMBACKDROP_TYPE, &backdrop_type, sizeof(backdrop_type));
    int corner_preference = kDwmCornerRound;
    DwmSetWindowAttribute(hwnd, DWMWA_WINDOW_CORNER_PREFERENCE, &corner_preference, sizeof(corner_preference));
  }
}

HRGN FlutterWindow::CreateScrollingCaptureControlsRegion(int width, int height) const
{
  if (width <= 0 || height <= 0)
  {
    return nullptr;
  }

  const double scale = screenshot_presentation_state_.workspace_scale <= 0 ? 1.0 : screenshot_presentation_state_.workspace_scale;
  const int toolbar_slot_height = std::max(1, static_cast<int>(std::lround(kScrollingCaptureToolbarSlotHeightDip * scale)));
  const int toolbar_height = std::max(1, static_cast<int>(std::lround(kScrollingCaptureToolbarHeightDip * scale)));
  const int toolbar_width = std::max(1, static_cast<int>(std::lround(kScrollingCaptureToolbarWidthDip * scale)));
  const int toolbar_radius = std::max(1, static_cast<int>(std::lround(kScrollingCaptureToolbarCornerRadiusDip * scale)));

  HRGN combined_region = CreateRectRgn(0, 0, 0, 0);
  if (combined_region == nullptr)
  {
    return nullptr;
  }

  const int preview_height = std::max(0, height - toolbar_slot_height);
  if (preview_height > 0)
  {
    HRGN preview_region = CreateRectRgn(0, 0, width, preview_height);
    if (preview_region != nullptr)
    {
      CombineRgn(combined_region, combined_region, preview_region, RGN_OR);
      DeleteObject(preview_region);
    }
  }

  const int toolbar_left = std::max(0, (width - toolbar_width) / 2);
  const int toolbar_top = std::max(0, height - toolbar_height);
  const int toolbar_right = std::min(width, toolbar_left + toolbar_width);
  const int toolbar_bottom = height;
  HRGN toolbar_region = CreateRoundRectRgn(toolbar_left, toolbar_top, toolbar_right + 1, toolbar_bottom + 1, toolbar_radius * 2, toolbar_radius * 2);
  if (toolbar_region != nullptr)
  {
    CombineRgn(combined_region, combined_region, toolbar_region, RGN_OR);
    DeleteObject(toolbar_region);
  }

  return combined_region;
}

void FlutterWindow::ApplyScrollingCaptureControlsRegion(HWND hwnd)
{
  if (hwnd == nullptr || !IsWindow(hwnd))
  {
    return;
  }

  RECT client_rect{};
  GetClientRect(hwnd, &client_rect);
  const int width = client_rect.right - client_rect.left;
  const int height = client_rect.bottom - client_rect.top;

  // Bug fix: Windows keeps the reused Flutter screenshot window backed by its normal acrylic/Mica
  // surface, unlike macOS where AppKit can make unpainted preview pixels fully transparent. Clip the
  // native window to the painted preview and toolbar regions so the compact scrolling controls do
  // not show a gray rectangular backing panel.
  HRGN root_region = CreateScrollingCaptureControlsRegion(width, height);
  if (root_region != nullptr)
  {
    SetWindowRgn(hwnd, root_region, TRUE);
  }

  if (child_window_ != nullptr && IsWindow(child_window_))
  {
    HRGN child_region = CreateScrollingCaptureControlsRegion(width, height);
    if (child_region != nullptr)
    {
      SetWindowRgn(child_window_, child_region, TRUE);
    }
  }
}

void FlutterWindow::ClearScrollingCaptureControlsRegion()
{
  HWND hwnd = GetHandle();
  if (hwnd != nullptr && IsWindow(hwnd))
  {
    SetWindowRgn(hwnd, nullptr, TRUE);
  }
  if (child_window_ != nullptr && IsWindow(child_window_))
  {
    SetWindowRgn(child_window_, nullptr, TRUE);
  }
}

void FlutterWindow::PaintScrollingCaptureOverlay(HWND hwnd)
{
  PAINTSTRUCT paint{};
  HDC hdc = BeginPaint(hwnd, &paint);
  if (hdc == nullptr)
  {
    return;
  }

  RECT client_rect{};
  GetClientRect(hwnd, &client_rect);
  HBRUSH overlay_brush = CreateSolidBrush(RGB(0, 0, 0));
  FillRect(hdc, &client_rect, overlay_brush);
  DeleteObject(overlay_brush);
  // Bug fix: do not draw the green selection border on Windows. BitBlt-based scrolling frames can
  // capture native overlay pixels at the selection edge, unlike the macOS capture path, so the
  // border polluted the stitched preview image.

  EndPaint(hwnd, &paint);
}

void FlutterWindow::EmitScrollingCaptureWheelEvent()
{
  if (window_manager_channel_)
  {
    window_manager_channel_->InvokeMethod("onScrollingCaptureWheel", std::make_unique<flutter::EncodableValue>(flutter::EncodableMap()));
  }
}

bool FlutterWindow::IsPointInScrollingCaptureSelection(POINT point) const
{
  const RECT &selection = scrolling_capture_overlay_state_.selection_bounds;
  return scrolling_capture_overlay_state_.active &&
         point.x >= selection.left &&
         point.x < selection.right &&
         point.y >= selection.top &&
         point.y < selection.bottom;
}

bool FlutterWindow::OnCreate()
{
  if (!Win32Window::OnCreate())
  {
    return false;
  }

  RECT frame = GetClientArea();

  // The size here must match the window dimensions to avoid unnecessary surface
  // creation / destruction in the startup path.
  flutter_controller_ = std::make_unique<flutter::FlutterViewController>(frame.right - frame.left, frame.bottom - frame.top, project_);
  // Ensure that basic setup of the controller was successful.
  if (!flutter_controller_->engine() || !flutter_controller_->view())
  {
    return false;
  }
  RegisterPlugins(flutter_controller_->engine());
  RegisterWoxWebviewPlugin(flutter_controller_->engine()->GetRegistrarForPlugin("WoxWebviewPlugin"));

  // Set up window manager method channel
  window_manager_channel_ = std::make_unique<flutter::MethodChannel<flutter::EncodableValue>>(
      flutter_controller_->engine()->messenger(), "com.wox.windows_window_manager",
      &flutter::StandardMethodCodec::GetInstance());

  window_manager_channel_->SetMethodCallHandler(
      [this](const auto &call, auto result)
      {
        HandleWindowManagerMethodCall(call, std::move(result));
      });

  // Replace the window procedure to capture window events
  HWND hwnd = GetHandle();
  if (hwnd != nullptr)
  {
    original_window_proc_ = reinterpret_cast<WNDPROC>(GetWindowLongPtr(hwnd, GWLP_WNDPROC));
    SetWindowLongPtr(hwnd, GWLP_WNDPROC, reinterpret_cast<LONG_PTR>(WindowProc));
  }

  child_window_ = flutter_controller_->view()->GetNativeWindow();
  SetChildContent(child_window_);

  if (child_window_ != nullptr)
  {
    original_child_window_proc_ = reinterpret_cast<WNDPROC>(GetWindowLongPtr(child_window_, GWLP_WNDPROC));
    SetWindowLongPtr(child_window_, GWLP_WNDPROC, reinterpret_cast<LONG_PTR>(ChildWindowProc));
  }

  flutter_controller_->engine()->SetNextFrameCallback([&]()
                                                      {
                                                        // hidden-at-launch
                                                        // this->Show();
                                                      });

  // Flutter can complete the first frame before the "show window" callback is
  // registered. The following call ensures a frame is pending to ensure the
  // window is shown. It is a no-op if the first frame hasn't completed yet.
  flutter_controller_->ForceRedraw();

  return true;
}

void FlutterWindow::OnDestroy()
{
  DismissNativeSelectionOverlays();
  DismissScrollingCaptureOverlay();
  pending_child_keydowns_.clear();
  ClearCachedDisplayCaptures();

  if (child_window_ != nullptr && original_child_window_proc_ != nullptr)
  {
    SetWindowLongPtr(child_window_, GWLP_WNDPROC, reinterpret_cast<LONG_PTR>(original_child_window_proc_));
    original_child_window_proc_ = nullptr;
    child_window_ = nullptr;
  }

  // Restore original window procedure
  HWND hwnd = GetHandle();
  if (hwnd != nullptr && original_window_proc_ != nullptr)
  {
    SetWindowLongPtr(hwnd, GWLP_WNDPROC, reinterpret_cast<LONG_PTR>(original_window_proc_));
  }

  if (flutter_controller_)
  {
    flutter_controller_ = nullptr;
  }

  Win32Window::OnDestroy();
}

LRESULT
FlutterWindow::MessageHandler(HWND hwnd, UINT const message, WPARAM const wparam, LPARAM const lparam) noexcept
{
  if (message == kScrollingCaptureWheelMessage)
  {
    EmitScrollingCaptureWheelEvent();
    return 0;
  }

  if (message == WM_SIZE)
  {
    std::optional<LRESULT> top_level_result;

    if (flutter_controller_)
    {
      // Keep the existing dispatch order for instrumentation so we can verify
      // whether the engine handles WM_SIZE before the base runner resizes the
      // hosted child window.
      top_level_result = flutter_controller_->HandleTopLevelWindowProc(hwnd, message, wparam, lparam);
    }

    std::ostringstream oss;
    oss << "WM_SIZE: engineHandled=" << (top_level_result.has_value() ? "true" : "false");
    Log(oss.str());

    // Keep the hosted Flutter child window in sync even when the engine
    // handles WM_SIZE before the base runner processes it.
    SyncFlutterChildWindowToClientArea(hwnd, "WM_SIZE", top_level_result.has_value());
    if (scrolling_capture_overlay_state_.active)
    {
      // Bug fix: the compact scrolling panel can resize after each accepted frame as the stitched
      // preview aspect ratio changes. Refresh the clipping region with the new client size so the
      // Windows backing panel does not reappear after native preview resize.
      ApplyScrollingCaptureControlsRegion(hwnd);
    }

    if (top_level_result)
    {
      return *top_level_result;
    }

    return Win32Window::MessageHandler(hwnd, message, wparam, lparam);
  }

  // Give Flutter, including plugins, an opportunity to handle window messages.
  if (flutter_controller_)
  {
    // Reroute BEFORE HandleTopLevelWindowProc. Otherwise the engine's
    // top-level handler consumes the WM_KEYUP at the root window without
    // generating a Dart KeyUpEvent, leaving HardwareKeyboard._pressedKeys
    // with a stale entry. The visible symptom is that the affected key
    // stops working until the next app restart.
    if (RerouteIgnoredRootKeyUp(hwnd, message, wparam, lparam))
    {
      return 0;
    }

    std::optional<LRESULT> result = flutter_controller_->HandleTopLevelWindowProc(hwnd, message, wparam, lparam);

    if (result)
    {
      return *result;
    }
  }

  switch (message)
  {
  case WM_TIMER:
    if (wparam == kRestoreForegroundTimerId1 || wparam == kRestoreForegroundTimerId2)
    {
      KillTimer(hwnd, static_cast<UINT_PTR>(wparam));
      // Only restore when this window is still hidden.
      if (IsWindowVisible(hwnd) == 0)
      {
        RestorePreviousActiveWindow(hwnd);
      }
      return 0;
    }
    break;
  case WM_FONTCHANGE:
    flutter_controller_->engine()->ReloadSystemFonts();
    break;
  }

  return Win32Window::MessageHandler(hwnd, message, wparam, lparam);
}

void FlutterWindow::SendWindowEvent(const std::string &eventName)
{
  if (window_manager_channel_)
  {
    window_manager_channel_->InvokeMethod(eventName, std::make_unique<flutter::EncodableValue>(flutter::EncodableMap()));
  }
}

LRESULT CALLBACK FlutterWindow::WindowProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam)
{
  // If window instance is not available, use default window procedure
  if (g_window_instance == nullptr || g_window_instance->original_window_proc_ == nullptr)
  {
    return DefWindowProc(hwnd, message, wparam, lparam);
  }

  // Handle window messages and send events to Flutter
  switch (message)
  {
  case WM_ACTIVATE:
    if (LOWORD(wparam) == WA_ACTIVE || LOWORD(wparam) == WA_CLICKACTIVE)
    {
      // g_window_instance->SendWindowEvent("onWindowFocus");
    }
    else
    {
      HWND activatedHwnd = reinterpret_cast<HWND>(lparam);
      if (!g_window_instance->ShouldSuppressBlurForActivatedWindow(hwnd, activatedHwnd))
      {
        const bool in_post_show_grace = GetTickCount64() < g_window_instance->blur_guard_until_tick_;
        if (g_window_instance->blur_guard_active_ || in_post_show_grace)
        {
          if (g_window_instance->blur_guard_active_)
          {
            g_window_instance->Log("WM_ACTIVATE: WA_INACTIVE suppressed (show-to-focus transition)");
          }
          else
          {
            g_window_instance->Log("WM_ACTIVATE: WA_INACTIVE suppressed (post-show grace)");
          }
        }
        else
        {
          g_window_instance->restore_previous_window_on_hide_ = false;
          g_window_instance->previous_active_window_ = nullptr;
          g_window_instance->SendWindowEvent("onWindowBlur");
        }
      }
    }
    break;
  }

  // Call the original window procedure
  return CallWindowProc(g_window_instance->original_window_proc_, hwnd, message, wparam, lparam);
}

LRESULT CALLBACK FlutterWindow::ChildWindowProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam)
{
  if (g_window_instance == nullptr || g_window_instance->original_child_window_proc_ == nullptr)
  {
    return DefWindowProc(hwnd, message, wparam, lparam);
  }

  if (IsKeyDownMessage(message))
  {
    g_window_instance->TrackChildKeyDown(message, wparam, lparam);
  }
  else if (IsKeyUpMessage(message))
  {
    g_window_instance->ClearTrackedChildKeyDown(message, wparam, lparam);
  }

  const LRESULT result = CallWindowProc(g_window_instance->original_child_window_proc_, hwnd, message, wparam, lparam);

  return result;
}

LRESULT CALLBACK FlutterWindow::ScrollingCaptureOverlayWindowProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam)
{
  if (g_window_instance == nullptr)
  {
    return DefWindowProc(hwnd, message, wparam, lparam);
  }

  switch (message)
  {
  case WM_NCHITTEST:
    return HTTRANSPARENT;
  case WM_ERASEBKGND:
    return 1;
  case WM_PAINT:
    g_window_instance->PaintScrollingCaptureOverlay(hwnd);
    return 0;
  default:
    return DefWindowProc(hwnd, message, wparam, lparam);
  }
}

LRESULT CALLBACK FlutterWindow::ScreenshotSelectionInputWindowProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam)
{
  if (g_window_instance == nullptr)
  {
    return DefWindowProc(hwnd, message, wparam, lparam);
  }

  switch (message)
  {
  case WM_ERASEBKGND:
    return 1;
  case WM_PAINT:
  {
    PAINTSTRUCT paint{};
    HDC hdc = BeginPaint(hwnd, &paint);
    if (hdc != nullptr)
    {
      FillRect(hdc, &paint.rcPaint, reinterpret_cast<HBRUSH>(GetStockObject(BLACK_BRUSH)));
    }
    EndPaint(hwnd, &paint);
    return 0;
  }
  case WM_SETCURSOR:
    SetCursor(LoadCursor(nullptr, IDC_CROSS));
    return TRUE;
  case kScreenshotSelectionDimRegionUpdateMessage:
    g_window_instance->FlushScreenshotSelectionDimRegionUpdate();
    return 0;
  case WM_KEYDOWN:
    if (wparam == VK_ESCAPE)
    {
      g_window_instance->CompleteScreenshotSelectionOverlay(true);
      return 0;
    }
    break;
  case WM_LBUTTONDOWN:
  {
    POINT point{GET_X_LPARAM(lparam), GET_Y_LPARAM(lparam)};
    ClientToScreen(hwnd, &point);
    g_window_instance->screenshot_selection_overlay_state_.dragging = true;
    g_window_instance->screenshot_selection_overlay_state_.drag_start = point;
    SetCapture(hwnd);
    SetFocus(hwnd);
    g_window_instance->UpdateScreenshotSelectionOverlay(RectFromPoints(point, point));
    return 0;
  }
  case WM_MOUSEMOVE:
    if (g_window_instance->screenshot_selection_overlay_state_.dragging)
    {
      POINT point{GET_X_LPARAM(lparam), GET_Y_LPARAM(lparam)};
      ClientToScreen(hwnd, &point);
      g_window_instance->UpdateScreenshotSelectionOverlay(RectFromPoints(g_window_instance->screenshot_selection_overlay_state_.drag_start, point));
      return 0;
    }
    break;
  case WM_LBUTTONUP:
    if (g_window_instance->screenshot_selection_overlay_state_.dragging)
    {
      POINT point{GET_X_LPARAM(lparam), GET_Y_LPARAM(lparam)};
      ClientToScreen(hwnd, &point);
      g_window_instance->screenshot_selection_overlay_state_.dragging = false;
      ReleaseCapture();
      g_window_instance->UpdateScreenshotSelectionOverlay(RectFromPoints(g_window_instance->screenshot_selection_overlay_state_.drag_start, point));
      g_window_instance->CompleteScreenshotSelectionOverlay(false);
      return 0;
    }
    break;
  case WM_CANCELMODE:
    if (g_window_instance->screenshot_selection_overlay_state_.dragging)
    {
      g_window_instance->screenshot_selection_overlay_state_.dragging = false;
      ReleaseCapture();
      g_window_instance->CompleteScreenshotSelectionOverlay(true);
      return 0;
    }
    break;
  default:
    break;
  }

  return DefWindowProc(hwnd, message, wparam, lparam);
}

LRESULT CALLBACK FlutterWindow::ScreenshotSelectionPassiveWindowProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam)
{
  switch (message)
  {
  case WM_NCHITTEST:
    return HTTRANSPARENT;
  case WM_SETCURSOR:
    SetCursor(LoadCursor(nullptr, IDC_CROSS));
    return TRUE;
  default:
    return DefWindowProc(hwnd, message, wparam, lparam);
  }
}

LRESULT CALLBACK FlutterWindow::ScreenshotSelectionMouseHookProc(int code, WPARAM wparam, LPARAM lparam)
{
  if (code == HC_ACTION && g_window_instance != nullptr)
  {
    const auto *mouse = reinterpret_cast<MSLLHOOKSTRUCT *>(lparam);
    auto &state = g_window_instance->screenshot_selection_overlay_state_;
    if (mouse != nullptr && state.active && !state.completed)
    {
      switch (wparam)
      {
      case WM_LBUTTONDOWN:
        // Bug fix: seed the first drag point through the low-level hook instead of waiting for the
        // layered mask HWND to receive capture. The hook is observer-only; returning through
        // CallNextHookEx keeps Windows' normal cursor and capture delivery alive.
        state.dragging = true;
        state.drag_start = mouse->pt;
        g_window_instance->UpdateScreenshotSelectionOverlay(RectFromPoints(mouse->pt, mouse->pt));
        break;
      case WM_MOUSEMOVE:
        if (state.dragging)
        {
          g_window_instance->UpdateScreenshotSelectionOverlay(RectFromPoints(state.drag_start, mouse->pt));
        }
        break;
      case WM_LBUTTONUP:
        if (state.dragging)
        {
          g_window_instance->UpdateScreenshotSelectionOverlay(RectFromPoints(state.drag_start, mouse->pt));
          g_window_instance->CompleteScreenshotSelectionOverlay(false);
        }
        break;
      default:
        break;
      }
    }
  }

  return CallNextHookEx(nullptr, code, wparam, lparam);
}

LRESULT CALLBACK FlutterWindow::ScrollingCaptureMouseHookProc(int code, WPARAM wparam, LPARAM lparam)
{
  if (code == HC_ACTION && g_window_instance != nullptr && wparam == WM_MOUSEWHEEL)
  {
    const auto *mouse = reinterpret_cast<MSLLHOOKSTRUCT *>(lparam);
    if (mouse != nullptr && g_window_instance->IsPointInScrollingCaptureSelection(mouse->pt))
    {
      // The native mask is mouse-transparent, so the wheel already scrolls the app underneath. This
      // hook mirrors macOS' global scroll monitor by telling Dart only that a new frame should be
      // captured after the target app has moved.
      PostMessage(g_window_instance->GetHandle(), kScrollingCaptureWheelMessage, 0, 0);
    }
  }

  return CallNextHookEx(nullptr, code, wparam, lparam);
}

void FlutterWindow::HandleWindowManagerMethodCall(
    const flutter::MethodCall<flutter::EncodableValue> &method_call,
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result)
{
  const std::string &method_name = method_call.method_name();
  HWND hwnd = GetHandle();

  if (hwnd == nullptr)
  {
    result->Error("WINDOW_ERROR", "Failed to get window handle");
    return;
  }

  try
  {
    if (method_name == "inputKeyDown" || method_name == "inputKeyUp")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (arguments == nullptr)
      {
        result->Error("INVALID_ARGUMENTS", "Missing arguments for keyboard input");
        return;
      }

      auto key_it = arguments->find(flutter::EncodableValue("key"));
      if (key_it == arguments->end())
      {
        result->Error("INVALID_ARGUMENTS", "Missing key for keyboard input");
        return;
      }

      const auto *key = std::get_if<std::string>(&key_it->second);
      if (key == nullptr)
      {
        result->Error("INVALID_ARGUMENTS", "Key must be a string");
        return;
      }

      auto virtual_key = ParseWindowsVirtualKey(*key);
      if (!virtual_key.has_value())
      {
        result->Error("UNSUPPORTED_KEY", "Unsupported key for Windows system input");
        return;
      }

      const bool key_up = method_name == "inputKeyUp";
      const bool is_alt = *virtual_key == VK_MENU || *virtual_key == VK_LMENU || *virtual_key == VK_RMENU;
      const bool handled = PostWindowsKeyMessage(hwnd, *virtual_key, key_up, is_alt);
      if (!handled)
      {
        result->Error("INPUT_ERROR", "Failed to send keyboard input");
        return;
      }

      result->Success();
    }
    else if (method_name == "inputMouseMove")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (arguments == nullptr)
      {
        result->Error("INVALID_ARGUMENTS", "Missing arguments for mouse move");
        return;
      }

      auto x_it = arguments->find(flutter::EncodableValue("x"));
      auto y_it = arguments->find(flutter::EncodableValue("y"));
      if (x_it == arguments->end() || y_it == arguments->end())
      {
        result->Error("INVALID_ARGUMENTS", "Missing coordinates for mouse move");
        return;
      }

      double x = std::get<double>(x_it->second);
      double y = std::get<double>(y_it->second);
      if (!SetCursorPos(static_cast<int>(std::lround(x)), static_cast<int>(std::lround(y))))
      {
        result->Error("INPUT_ERROR", "Failed to move mouse cursor");
        return;
      }

      result->Success();
    }
    else if (method_name == "inputMouseDown" || method_name == "inputMouseUp")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (arguments == nullptr)
      {
        result->Error("INVALID_ARGUMENTS", "Missing arguments for mouse button input");
        return;
      }

      auto button_it = arguments->find(flutter::EncodableValue("button"));
      if (button_it == arguments->end())
      {
        result->Error("INVALID_ARGUMENTS", "Missing mouse button");
        return;
      }

      const auto *button = std::get_if<std::string>(&button_it->second);
      if (button == nullptr)
      {
        result->Error("INVALID_ARGUMENTS", "Mouse button must be a string");
        return;
      }

      auto mouse_flag = ParseWindowsMouseFlag(*button, method_name == "inputMouseUp");
      if (!mouse_flag.has_value())
      {
        result->Error("UNSUPPORTED_BUTTON", "Unsupported mouse button for Windows system input");
        return;
      }

      if (!SendWindowsMouseButtonInput(*mouse_flag))
      {
        result->Error("INPUT_ERROR", "Failed to send mouse button input");
        return;
      }

      result->Success();
    }
    else if (method_name == "inputMouseScroll")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (arguments == nullptr)
      {
        result->Error("INVALID_ARGUMENTS", "Missing arguments for mouse scroll");
        return;
      }

      auto delta_y_it = arguments->find(flutter::EncodableValue("deltaY"));
      if (delta_y_it == arguments->end())
      {
        result->Error("INVALID_ARGUMENTS", "Missing scroll delta");
        return;
      }

      const double delta_y = std::get<double>(delta_y_it->second);
      const int wheel_delta = -static_cast<int>(std::lround(delta_y * WHEEL_DELTA));
      if (wheel_delta == 0)
      {
        result->Success();
        return;
      }

      // The scrolling screenshot overlay receives the Flutter wheel event first for preview
      // bookkeeping. Briefly marking Wox as transparent lets the forwarded wheel input hit the
      // window underneath the selected rectangle instead of the topmost capture shell.
      SetWindowsScreenshotMousePassthrough(hwnd, child_window_, true);
      const bool sent = SendWindowsMouseWheelInput(wheel_delta);
      Sleep(50);
      SetWindowsScreenshotMousePassthrough(hwnd, child_window_, false);

      if (!sent)
      {
        result->Error("INPUT_ERROR", "Failed to send mouse wheel input");
        return;
      }

      result->Success();
    }
    else if (method_name == "captureAllDisplays")
    {
      std::optional<RECT> logical_selection;
      std::vector<CachedDisplayCapture> captures;
      std::string capture_error;
      const double selection_workspace_scale = screenshot_presentation_state_.workspace_scale <= 0 ? static_cast<double>(GetDpiScale(hwnd)) : screenshot_presentation_state_.workspace_scale;
      if (!TryParseLogicalSelectionArgument(method_call.arguments(), selection_workspace_scale, &logical_selection, &capture_error))
      {
        result->Error("INVALID_ARGUMENTS", capture_error);
        return;
      }

      if (!CaptureDisplaySnapshots(&captures, &capture_error, logical_selection))
      {
        result->Error("CAPTURE_ERROR", capture_error);
        return;
      }

      flutter::EncodableList snapshots;
      if (!BuildDisplaySnapshotPayloads(captures, true, &snapshots, &capture_error))
      {
        ReleaseDisplayCaptures(&captures);
        result->Error("CAPTURE_ERROR", capture_error);
        return;
      }

      if (logical_selection.has_value())
      {
        // Selection captures are transient scrolling frames. Caching them would make a later
        // loadDisplaySnapshots call reuse cropped bitmaps where the normal screenshot flow expects
        // full displays, so keep the region optimization local to this request.
        ReleaseDisplayCaptures(&captures);
      }
      else
      {
        ClearCachedDisplayCaptures();
        cached_display_captures_ = std::move(captures);
      }
      result->Success(flutter::EncodableValue(snapshots));
    }
    else if (method_name == "captureDisplayMetadata")
    {
      std::vector<CachedDisplayCapture> captures;
      std::string capture_error;
      if (!CaptureDisplaySnapshots(&captures, &capture_error))
      {
        result->Error("CAPTURE_ERROR", capture_error);
        return;
      }

      ClearCachedDisplayCaptures();
      cached_display_captures_ = std::move(captures);

      flutter::EncodableList snapshots;
      if (!BuildDisplaySnapshotPayloads(cached_display_captures_, false, &snapshots, &capture_error))
      {
        result->Error("CAPTURE_ERROR", capture_error);
        return;
      }

      result->Success(flutter::EncodableValue(snapshots));
    }
    else if (method_name == "loadDisplaySnapshots")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (arguments == nullptr)
      {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for loadDisplaySnapshots");
        return;
      }

      auto display_ids_it = arguments->find(flutter::EncodableValue("displayIds"));
      if (display_ids_it == arguments->end())
      {
        result->Error("INVALID_ARGUMENTS", "Missing displayIds for loadDisplaySnapshots");
        return;
      }

      const auto *display_ids_value = std::get_if<flutter::EncodableList>(&display_ids_it->second);
      if (display_ids_value == nullptr)
      {
        result->Error("INVALID_ARGUMENTS", "displayIds must be a list");
        return;
      }

      std::vector<std::string> display_ids;
      display_ids.reserve(display_ids_value->size());
      for (const auto &display_id_value : *display_ids_value)
      {
        const auto *display_id = std::get_if<std::string>(&display_id_value);
        if (display_id == nullptr)
        {
          result->Error("INVALID_ARGUMENTS", "displayIds must contain strings");
          return;
        }
        display_ids.push_back(*display_id);
      }

      if (!display_ids.empty() && !CachedDisplayCapturesMatch(display_ids))
      {
        std::vector<CachedDisplayCapture> captures;
        std::string capture_error;
        if (!CaptureDisplaySnapshots(&captures, &capture_error))
        {
          result->Error("CAPTURE_ERROR", capture_error);
          return;
        }

        ClearCachedDisplayCaptures();
        cached_display_captures_ = std::move(captures);
      }

      std::vector<CachedDisplayCapture> filtered_captures;
      if (display_ids.empty())
      {
        filtered_captures = cached_display_captures_;
      }
      else
      {
        for (const auto &display_id : display_ids)
        {
          const auto *capture = FindCachedDisplayCapture(display_id);
          if (capture == nullptr)
          {
            result->Error("CAPTURE_ERROR", "Failed to locate cached display capture");
            return;
          }
          filtered_captures.push_back(*capture);
        }
      }

      std::string capture_error;
      flutter::EncodableList snapshots;
      if (!BuildDisplaySnapshotPayloads(filtered_captures, true, &snapshots, &capture_error))
      {
        result->Error("CAPTURE_ERROR", capture_error);
        return;
      }

      result->Success(flutter::EncodableValue(snapshots));
    }
    else if (method_name == "writeClipboardImageFile")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (arguments == nullptr)
      {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for writeClipboardImageFile");
        return;
      }

      auto file_path_it = arguments->find(flutter::EncodableValue("filePath"));
      if (file_path_it == arguments->end())
      {
        result->Error("INVALID_ARGUMENTS", "Missing filePath for writeClipboardImageFile");
        return;
      }

      const auto *file_path = std::get_if<std::string>(&file_path_it->second);
      if (file_path == nullptr || file_path->empty())
      {
        result->Error("INVALID_ARGUMENTS", "filePath must be a non-empty string");
        return;
      }

      std::string clipboard_error;
      if (!WriteClipboardImageFile(*file_path, &clipboard_error))
      {
        result->Error("CLIPBOARD_ERROR", clipboard_error);
        return;
      }

      result->Success();
    }
    else if (method_name == "selectCaptureRegion")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (arguments == nullptr)
      {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for selectCaptureRegion");
        return;
      }

      auto x_it = arguments->find(flutter::EncodableValue("x"));
      auto y_it = arguments->find(flutter::EncodableValue("y"));
      auto width_it = arguments->find(flutter::EncodableValue("width"));
      auto height_it = arguments->find(flutter::EncodableValue("height"));
      if (x_it == arguments->end() || y_it == arguments->end() || width_it == arguments->end() || height_it == arguments->end())
      {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for selectCaptureRegion");
        return;
      }

      const double x = std::get<double>(x_it->second);
      const double y = std::get<double>(y_it->second);
      const double width = std::get<double>(width_it->second);
      const double height = std::get<double>(height_it->second);
      const RECT native_workspace_bounds{
          static_cast<LONG>(std::lround(x)),
          static_cast<LONG>(std::lround(y)),
          static_cast<LONG>(std::lround(x + width)),
          static_cast<LONG>(std::lround(y + height))};

      if (cached_display_captures_.empty())
      {
        // Native selection is a capability path that depends on metadata capture caching monitor
        // bounds. If the cache is missing, return unhandled so Dart can use the older Flutter path.
        flutter::EncodableMap response;
        response[flutter::EncodableValue("wasHandled")] = flutter::EncodableValue(false);
        result->Success(flutter::EncodableValue(response));
        return;
      }

      std::string selection_error;
      if (!BeginScreenshotSelectionOverlay(hwnd, native_workspace_bounds, std::move(result), &selection_error))
      {
        return;
      }
      return;
    }
    else if (method_name == "prepareCaptureWorkspace")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (arguments == nullptr)
      {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for prepareCaptureWorkspace");
        return;
      }

      auto x_it = arguments->find(flutter::EncodableValue("x"));
      auto y_it = arguments->find(flutter::EncodableValue("y"));
      auto width_it = arguments->find(flutter::EncodableValue("width"));
      auto height_it = arguments->find(flutter::EncodableValue("height"));
      if (x_it == arguments->end() || y_it == arguments->end() || width_it == arguments->end() || height_it == arguments->end())
      {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for prepareCaptureWorkspace");
        return;
      }

      const double x = std::get<double>(x_it->second);
      const double y = std::get<double>(y_it->second);
      const double width = std::get<double>(width_it->second);
      const double height = std::get<double>(height_it->second);
      const RECT native_workspace_bounds{
          static_cast<LONG>(std::lround(x)),
          static_cast<LONG>(std::lround(y)),
          static_cast<LONG>(std::lround(x + width)),
          static_cast<LONG>(std::lround(y + height))};

      PrepareCaptureWorkspace(hwnd, native_workspace_bounds);
      result->Success(flutter::EncodableValue(BuildCaptureWorkspaceResponse(native_workspace_bounds)));
    }
    else if (method_name == "presentCaptureWorkspace")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (arguments == nullptr)
      {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for presentCaptureWorkspace");
        return;
      }

      auto x_it = arguments->find(flutter::EncodableValue("x"));
      auto y_it = arguments->find(flutter::EncodableValue("y"));
      auto width_it = arguments->find(flutter::EncodableValue("width"));
      auto height_it = arguments->find(flutter::EncodableValue("height"));
      if (x_it == arguments->end() || y_it == arguments->end() || width_it == arguments->end() || height_it == arguments->end())
      {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for presentCaptureWorkspace");
        return;
      }

      const double x = std::get<double>(x_it->second);
      const double y = std::get<double>(y_it->second);
      const double width = std::get<double>(width_it->second);
      const double height = std::get<double>(height_it->second);
      const RECT native_workspace_bounds{
          static_cast<LONG>(std::lround(x)),
          static_cast<LONG>(std::lround(y)),
          static_cast<LONG>(std::lround(x + width)),
          static_cast<LONG>(std::lround(y + height))};

      PrepareCaptureWorkspace(hwnd, native_workspace_bounds);
      RevealPreparedCaptureWorkspace(hwnd);
      result->Success(flutter::EncodableValue(BuildCaptureWorkspaceResponse(native_workspace_bounds)));
    }
    else if (method_name == "revealPreparedCaptureWorkspace")
    {
      RevealPreparedCaptureWorkspace(hwnd);
      result->Success();
    }
    else if (method_name == "beginScrollingCaptureOverlay")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (arguments == nullptr)
      {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for beginScrollingCaptureOverlay");
        return;
      }

      const double scale = screenshot_presentation_state_.workspace_scale <= 0 ? static_cast<double>(GetDpiScale(hwnd)) : screenshot_presentation_state_.workspace_scale;
      RECT workspace_bounds{};
      RECT selection_bounds{};
      RECT controls_bounds{};
      std::string parse_error;
      if (!TryReadNestedRectArgument(*arguments, "workspaceBounds", scale, &workspace_bounds, &parse_error) ||
          !TryReadNestedRectArgument(*arguments, "selection", scale, &selection_bounds, &parse_error) ||
          !TryReadNestedRectArgument(*arguments, "controlsBounds", scale, &controls_bounds, &parse_error))
      {
        result->Error("INVALID_ARGUMENTS", parse_error);
        return;
      }

      // Windows now follows the macOS scrolling-capture handoff: the fullscreen capture shell is
      // replaced by a passive native mask, while the reused Flutter window becomes the compact
      // preview/toolbox. Keeping this state native lets wheel input reach the selected app directly.
      BeginScrollingCaptureOverlay(hwnd, workspace_bounds, selection_bounds, controls_bounds);
      result->Success();
    }
    else if (method_name == "dismissCaptureWorkspacePresentation")
    {
      DismissScrollingCaptureOverlay();
      screenshot_presentation_state_.active = false;
      screenshot_presentation_state_.prepared = false;
      screenshot_presentation_state_.workspace_scale = 1.0;
      screenshot_presentation_state_.native_workspace_bounds = {0, 0, 0, 0};
      SyncFlutterChildWindowToClientArea(hwnd, "dismissCaptureWorkspacePresentation", false);
      result->Success();
    }
    else if (method_name == "dismissNativeSelectionOverlays")
    {
      DismissNativeSelectionOverlays();
      result->Success();
    }
    else if (method_name == "debugCaptureWorkspaceState")
    {
      RECT root_rect{};
      GetWindowRect(hwnd, &root_rect);
      const double current_scale = static_cast<double>(GetDpiScale(hwnd));

      flutter::EncodableMap response;
      response[flutter::EncodableValue("isCapturePresentationActive")] = flutter::EncodableValue(screenshot_presentation_state_.active);
      response[flutter::EncodableValue("workspaceScale")] = flutter::EncodableValue(screenshot_presentation_state_.workspace_scale);
      response[flutter::EncodableValue("workspaceBounds")] = flutter::EncodableValue(
          BuildScaledRectValue(screenshot_presentation_state_.native_workspace_bounds, screenshot_presentation_state_.workspace_scale));
      response[flutter::EncodableValue("windowBounds")] = flutter::EncodableValue(BuildScaledRectValue(root_rect, current_scale));
      response[flutter::EncodableValue("nativeWorkspaceBounds")] = flutter::EncodableValue(BuildRectValue(screenshot_presentation_state_.native_workspace_bounds));
      result->Success(flutter::EncodableValue(response));
    }
    else if (method_name == "setSize")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (arguments)
      {
        auto width_it = arguments->find(flutter::EncodableValue("width"));
        auto height_it = arguments->find(flutter::EncodableValue("height"));
        if (width_it != arguments->end() && height_it != arguments->end())
        {
          double width = std::get<double>(width_it->second);
          double height = std::get<double>(height_it->second);

          // Get DPI scale factor
          float dpiScale = GetDpiScale(hwnd);

          // Apply DPI scaling to get physical pixels
          int scaledWidth = static_cast<int>(width * dpiScale);
          int scaledHeight = static_cast<int>(height * dpiScale);

          RECT rect;
          GetWindowRect(hwnd, &rect);
          SetWindowPos(hwnd, nullptr, rect.left, rect.top, scaledWidth, scaledHeight, SWP_NOZORDER | SWP_FRAMECHANGED);

          RECT root_rect{};
          RECT client_rect{};
          GetWindowRect(hwnd, &root_rect);
          GetClientRect(hwnd, &client_rect);
          const RECT child_rect = GetWindowRectSafe(child_window_);
          std::ostringstream oss;
          oss << "setSize: logical=" << width << "x" << height
              << ", physical=" << scaledWidth << "x" << scaledHeight
              << ", root=" << RectToString(root_rect)
              << ", client=" << RectToString(client_rect)
              << ", child=" << RectToString(child_rect);
          Log(oss.str());

          // Force Flutter to redraw immediately to match the new window size
          if (flutter_controller_)
          {
            flutter_controller_->ForceRedraw();
          }

          result->Success();
        }
        else
        {
          result->Error("INVALID_ARGUMENTS", "Invalid arguments for setSize");
        }
      }
      else
      {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for setSize");
      }
    }
    else if (method_name == "setBounds")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (arguments)
      {
        auto x_it = arguments->find(flutter::EncodableValue("x"));
        auto y_it = arguments->find(flutter::EncodableValue("y"));
        auto width_it = arguments->find(flutter::EncodableValue("width"));
        auto height_it = arguments->find(flutter::EncodableValue("height"));
        if (x_it != arguments->end() && y_it != arguments->end() && width_it != arguments->end() && height_it != arguments->end())
        {
          double x = std::get<double>(x_it->second);
          double y = std::get<double>(y_it->second);
          double width = std::get<double>(width_it->second);
          double height = std::get<double>(height_it->second);

          struct MonitorFindData
          {
            LONG targetX, targetY;
            HMONITOR foundMonitor;
            UINT foundDpi;
          } findData = {static_cast<LONG>(x), static_cast<LONG>(y), nullptr, 96};

          EnumDisplayMonitors(nullptr, nullptr, [](HMONITOR hMon, HDC, LPRECT, LPARAM lParam) -> BOOL
                              {
                                auto *data = reinterpret_cast<MonitorFindData *>(lParam);
                                MONITORINFO mi = {sizeof(mi)};
                                if (GetMonitorInfo(hMon, &mi))
                                {
                                  UINT dpi = FlutterDesktopGetDpiForMonitor(hMon);
                                  float scale = dpi / 96.0f;

                                  LONG logLeft = static_cast<LONG>(mi.rcMonitor.left / scale);
                                  LONG logTop = static_cast<LONG>(mi.rcMonitor.top / scale);
                                  LONG logRight = static_cast<LONG>(mi.rcMonitor.right / scale);
                                  LONG logBottom = static_cast<LONG>(mi.rcMonitor.bottom / scale);

                                  if (data->targetX >= logLeft && data->targetX < logRight &&
                                      data->targetY >= logTop && data->targetY < logBottom)
                                  {
                                    data->foundMonitor = hMon;
                                    data->foundDpi = dpi;
                                    return FALSE;
                                  }
                                }
                                return TRUE; }, reinterpret_cast<LPARAM>(&findData));

          if (findData.foundMonitor == nullptr)
          {
            findData.foundMonitor = MonitorFromPoint({0, 0}, MONITOR_DEFAULTTOPRIMARY);
            findData.foundDpi = FlutterDesktopGetDpiForMonitor(findData.foundMonitor);
          }

          float dpiScale = findData.foundDpi / 96.0f;
          int scaledX = static_cast<int>(x * dpiScale);
          int scaledY = static_cast<int>(y * dpiScale);
          int scaledWidth = static_cast<int>(width * dpiScale);
          int scaledHeight = static_cast<int>(height * dpiScale);

          SetWindowPos(hwnd, nullptr, scaledX, scaledY, scaledWidth, scaledHeight, SWP_NOZORDER | SWP_FRAMECHANGED);

          RECT root_rect{};
          RECT client_rect{};
          GetWindowRect(hwnd, &root_rect);
          GetClientRect(hwnd, &client_rect);
          const RECT child_rect = GetWindowRectSafe(child_window_);
          std::ostringstream oss;
          oss << "setBounds: logicalPos=" << x << "," << y
              << ", logicalSize=" << width << "x" << height
              << ", physicalPos=" << scaledX << "," << scaledY
              << ", physicalSize=" << scaledWidth << "x" << scaledHeight
              << ", root=" << RectToString(root_rect)
              << ", client=" << RectToString(client_rect)
              << ", child=" << RectToString(child_rect);
          Log(oss.str());

          if (flutter_controller_)
          {
            flutter_controller_->ForceRedraw();
          }

          result->Success();
        }
        else
        {
          result->Error("INVALID_ARGUMENTS", "Invalid arguments for setBounds");
        }
      }
      else
      {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for setBounds");
      }
    }
    else if (method_name == "getPosition")
    {
      RECT rect;
      GetWindowRect(hwnd, &rect);

      // Get DPI scale factor
      float dpiScale = GetDpiScale(hwnd);

      // Apply DPI scaling to logical pixels (physical to logical)
      double scaledX = static_cast<double>(rect.left) / dpiScale;
      double scaledY = static_cast<double>(rect.top) / dpiScale;

      flutter::EncodableMap position;
      position[flutter::EncodableValue("x")] = flutter::EncodableValue(scaledX);
      position[flutter::EncodableValue("y")] = flutter::EncodableValue(scaledY);
      result->Success(flutter::EncodableValue(position));
    }
    else if (method_name == "getSize")
    {
      RECT rect;
      GetWindowRect(hwnd, &rect);

      // Get DPI scale factor
      float dpiScale = GetDpiScale(hwnd);

      // Convert physical pixels to logical pixels
      double width = static_cast<double>(rect.right - rect.left) / dpiScale;
      double height = static_cast<double>(rect.bottom - rect.top) / dpiScale;

      flutter::EncodableMap size;
      size[flutter::EncodableValue("width")] = flutter::EncodableValue(width);
      size[flutter::EncodableValue("height")] = flutter::EncodableValue(height);
      result->Success(flutter::EncodableValue(size));
    }
    else if (method_name == "setPosition")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (arguments)
      {
        auto x_it = arguments->find(flutter::EncodableValue("x"));
        auto y_it = arguments->find(flutter::EncodableValue("y"));
        if (x_it != arguments->end() && y_it != arguments->end())
        {
          double x = std::get<double>(x_it->second);
          double y = std::get<double>(y_it->second);

          // COORDINATE SYSTEM EXPLANATION:
          // ... (existing logic) ...

          struct MonitorFindData
          {
            LONG targetX, targetY;
            HMONITOR foundMonitor;
            UINT foundDpi;
          } findData = {static_cast<LONG>(x), static_cast<LONG>(y), nullptr, 96};

          // Enumerate all monitors to find which one contains our logical point
          EnumDisplayMonitors(nullptr, nullptr, [](HMONITOR hMon, HDC, LPRECT, LPARAM lParam) -> BOOL
                              {
                                auto *data = reinterpret_cast<MonitorFindData *>(lParam);
                                MONITORINFO mi = {sizeof(mi)};
                                if (GetMonitorInfo(hMon, &mi))
                                {
                                  UINT dpi = FlutterDesktopGetDpiForMonitor(hMon);
                                  float scale = dpi / 96.0f;

                                  LONG logLeft = static_cast<LONG>(mi.rcMonitor.left / scale);
                                  LONG logTop = static_cast<LONG>(mi.rcMonitor.top / scale);
                                  LONG logRight = static_cast<LONG>(mi.rcMonitor.right / scale);
                                  LONG logBottom = static_cast<LONG>(mi.rcMonitor.bottom / scale);

                                  if (data->targetX >= logLeft && data->targetX < logRight &&
                                      data->targetY >= logTop && data->targetY < logBottom)
                                  {
                                    data->foundMonitor = hMon;
                                    data->foundDpi = dpi;
                                    return FALSE; // Found the correct monitor, stop enumeration
                                  }
                                }
                                return TRUE; // Not this monitor, continue searching
                              },
                              reinterpret_cast<LPARAM>(&findData));

          if (findData.foundMonitor == nullptr)
          {
            findData.foundMonitor = MonitorFromPoint({0, 0}, MONITOR_DEFAULTTOPRIMARY);
            findData.foundDpi = FlutterDesktopGetDpiForMonitor(findData.foundMonitor);
          }

          float dpiScale = findData.foundDpi / 96.0f;
          int scaledX = static_cast<int>(x * dpiScale);
          int scaledY = static_cast<int>(y * dpiScale);

          RECT rect;
          GetWindowRect(hwnd, &rect);
          int width = rect.right - rect.left;
          int height = rect.bottom - rect.top;
          SetWindowPos(hwnd, nullptr, scaledX, scaledY, width, height, SWP_NOZORDER | SWP_NOSIZE);
          result->Success();
        }
        else
        {
          result->Error("INVALID_ARGUMENTS", "Invalid arguments for setPosition");
        }
      }
      else
      {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for setPosition");
      }
    }
    else if (method_name == "center")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (!arguments)
      {
        result->Error("INVALID_ARGUMENTS", "Arguments must be provided for center");
        return;
      }

      auto width_it = arguments->find(flutter::EncodableValue("width"));
      auto height_it = arguments->find(flutter::EncodableValue("height"));

      if (width_it == arguments->end() || height_it == arguments->end())
      {
        result->Error("INVALID_ARGUMENTS", "Both width and height must be provided for center");
        return;
      }

      double width = std::get<double>(width_it->second);
      double height = std::get<double>(height_it->second);

      // Get cursor position to determine which monitor to center on
      POINT cursorPos;
      GetCursorPos(&cursorPos);

      // Get the monitor where the cursor is located
      HMONITOR hMonitor = MonitorFromPoint(cursorPos, MONITOR_DEFAULTTONEAREST);
      MONITORINFO monitorInfo;
      monitorInfo.cbSize = sizeof(MONITORINFO);

      if (!GetMonitorInfo(hMonitor, &monitorInfo))
      {
        result->Error("MONITOR_ERROR", "Failed to get monitor info");
        return;
      }

      // Get DPI scale factor for the target monitor
      UINT dpi = FlutterDesktopGetDpiForMonitor(hMonitor);
      float dpiScale = dpi / 96.0f;

      // Apply DPI scaling to get physical pixels
      int scaledWidth = static_cast<int>(width * dpiScale);
      int scaledHeight = static_cast<int>(height * dpiScale);

      // Get monitor work area (physical coordinates), excluding taskbar
      int monitorLeft = monitorInfo.rcWork.left;
      int monitorTop = monitorInfo.rcWork.top;
      int monitorWidth = monitorInfo.rcWork.right - monitorInfo.rcWork.left;
      int monitorHeight = monitorInfo.rcWork.bottom - monitorInfo.rcWork.top;

      // Calculate center position on the mouse's monitor
      int x = monitorLeft + (monitorWidth - scaledWidth) / 2;
      int y = monitorTop + (monitorHeight - scaledHeight) / 2;

      Log("Center: window to " + std::to_string(x) + "," + std::to_string(y) + " with " + std::to_string(scaledWidth) + "," + std::to_string(scaledHeight) + " on monitor at " + std::to_string(monitorLeft) + "," + std::to_string(monitorTop));
      SetWindowPos(hwnd, nullptr, x, y, scaledWidth, scaledHeight, SWP_NOZORDER);
      result->Success();
    }
    else if (method_name == "show")
    {
      SavePreviousActiveWindow(hwnd);

      // Flush stale keyboard state before showing the window.
      // If the previous hide-flush was ineffective (e.g. the engine dropped
      // the synthetic keyup), retrying here clears any remaining entries so
      // the user doesn't encounter stuck keys after Wox reappears.
      FlushPendingChildKeyUps();

      // Suppress transient blur events that fire between show() and the
      // subsequent focus() call from Dart.  Without this, Windows may
      // deactivate the newly-shown window (e.g. Explorer steals focus),
      // sending WM_ACTIVATE/WA_INACTIVE before focus() has a chance to
      // grab the foreground, which causes onWindowBlur -> hideApp().
      blur_guard_active_ = true;
      blur_guard_until_tick_ = GetTickCount64() + kPostShowBlurGraceMs;
      ShowWindow(hwnd, SW_SHOW);
      result->Success();
    }
    else if (method_name == "hide")
    {
      blur_guard_active_ = false;
      blur_guard_until_tick_ = 0;

      // Flush before SW_HIDE with skipPhysicallyHeld=false so that every
      // pending keydown, including modifier keys that are still physically
      // held (e.g. Ctrl/Alt from a hotkey combination), receives a synthetic
      // keyup.  After SW_HIDE the real keyup goes to whichever window gains
      // focus next, not to Flutter, so without this forced flush those modifier
      // keys would remain permanently "pressed" in HardwareKeyboard and cause
      // the next keystroke (e.g. 'A') to be misread as a shortcut (Ctrl+A).
      FlushPendingChildKeyUps(/*skipPhysicallyHeld=*/false);

      HWND fg = GetForegroundWindow();
      bool isForeground = (fg == hwnd || fg == GetAncestor(hwnd, GA_ROOT));
      bool shouldRestorePreviousWindow = restore_previous_window_on_hide_;

      ShowWindow(hwnd, SW_HIDE);

      if (isForeground && shouldRestorePreviousWindow)
      {
        RestorePreviousActiveWindow(hwnd);

        // Retry restore after the system finishes processing activation changes.
        KillTimer(hwnd, kRestoreForegroundTimerId1);
        KillTimer(hwnd, kRestoreForegroundTimerId2);
        SetTimer(hwnd, kRestoreForegroundTimerId1, 30, nullptr);
        SetTimer(hwnd, kRestoreForegroundTimerId2, 200, nullptr);
      }
      else if (!shouldRestorePreviousWindow)
      {
        Log("Window: Wox already lost focus before hiding, skipping restore");
        previous_active_window_ = nullptr;
        KillTimer(hwnd, kRestoreForegroundTimerId1);
        KillTimer(hwnd, kRestoreForegroundTimerId2);
      }
      else
      {
        Log("Window: Wox is not foreground when hiding, skipping restore");
        previous_active_window_ = nullptr;
        KillTimer(hwnd, kRestoreForegroundTimerId1);
        KillTimer(hwnd, kRestoreForegroundTimerId2);
      }

      restore_previous_window_on_hide_ = false;

      result->Success();
    }
    else if (method_name == "focus")
    {
      // If the Start Menu or Search overlay is open, dismiss it first.
      // SetForegroundWindow requires "no menus are active" to succeed.
      DismissStartMenuIfOpen();

      // Save current foreground window before bringing Wox to front.
      SavePreviousActiveWindow(hwnd);

      // Optimization: Try SetForegroundWindow directly first.
      // If we already have permission or are in foreground, this avoids AttachThreadInput
      // which can block for seconds if the foreground window is hung.
      if (SetForegroundWindow(hwnd))
      {
        FocusFlutterViewOrRoot(hwnd);
        BringWindowToTop(hwnd);
        blur_guard_active_ = false;
        result->Success();
        return;
      }

      HWND fg = GetForegroundWindow();
      DWORD curTid = GetCurrentThreadId();
      DWORD fgTid = 0;
      if (fg)
      {
        fgTid = GetWindowThreadProcessId(fg, nullptr);
      }

      bool attached = false;
      if (fg && fgTid != 0 && fgTid != curTid)
      {
        attached = AttachThreadInput(fgTid, curTid, TRUE);
      }

      SetForegroundWindow(hwnd);
      FocusFlutterViewOrRoot(hwnd);
      BringWindowToTop(hwnd);

      if (attached)
      {
        AttachThreadInput(fgTid, curTid, FALSE);
      }

      if (GetForegroundWindow() == hwnd)
      {
        Log("Focus: use attach thread input");
        blur_guard_active_ = false;
        result->Success();
        return;
      }

      INPUT pInputs[2];
      ZeroMemory(pInputs, sizeof(INPUT));

      pInputs[0].type = INPUT_KEYBOARD;
      pInputs[0].ki.wVk = VK_MENU; // Alt down
      pInputs[0].ki.dwFlags = 0;

      pInputs[1].type = INPUT_KEYBOARD;
      pInputs[1].ki.wVk = VK_MENU; // Alt up
      pInputs[1].ki.dwFlags = KEYEVENTF_KEYUP;

      SendInput(2, pInputs, sizeof(INPUT));
      Sleep(10);

      SetForegroundWindow(hwnd);
      FocusFlutterViewOrRoot(hwnd);
      BringWindowToTop(hwnd);

      if (GetForegroundWindow() == hwnd)
      {
        Log("Focus: use Alt key injection");
        blur_guard_active_ = false;
        result->Success();
        return;
      }

      Log("Focus: both methods failed, trying AllowSetForegroundWindow");
      AllowSetForegroundWindow(ASFW_ANY);
      SetForegroundWindow(hwnd);
      FocusFlutterViewOrRoot(hwnd);

      Log("Focus: final attempt completed");
      blur_guard_active_ = false;
      result->Success();
    }
    else if (method_name == "isVisible")
    {
      bool is_visible = IsWindowVisible(hwnd) != 0;
      result->Success(flutter::EncodableValue(is_visible));
    }
    else if (method_name == "setAlwaysOnTop")
    {
      const auto *arguments = std::get_if<bool>(method_call.arguments());
      if (arguments)
      {
        bool always_on_top = *arguments;
        SetWindowPos(hwnd, always_on_top ? HWND_TOPMOST : HWND_NOTOPMOST, 0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE);
        result->Success();
      }
      else
      {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for setAlwaysOnTop");
      }
    }
    else if (method_name == "setAppearance")
    {
      const auto *arguments = std::get_if<std::string>(method_call.arguments());
      if (arguments)
      {
        std::string appearance = *arguments;
        BOOL useDark = (appearance == "dark");
        DwmSetWindowAttribute(hwnd, DWMWA_USE_IMMERSIVE_DARK_MODE, &useDark, sizeof(useDark));
        result->Success();
      }
      else
      {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for setAppearance");
      }
    }
    else if (method_name == "startDragging")
    {
      ReleaseCapture();
      SendMessage(hwnd, WM_NCLBUTTONDOWN, HTCAPTION, 0);
      result->Success();
    }
    else if (method_name == "waitUntilReadyToShow")
    {
      result->Success();
    }
    else
    {
      result->NotImplemented();
    }
  }
  catch (const std::exception &e)
  {
    result->Error("EXCEPTION", std::string("Exception: ") + e.what());
  }
  catch (...)
  {
    result->Error("EXCEPTION", "Unknown exception occurred");
  }
}
