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
static constexpr UINT kScreenshotSelectionHoverMoveMessage = WM_APP + 0x53;
static constexpr UINT kScreenshotDisplaySnapshotPayloadReadyMessage = WM_APP + 0x54;
static constexpr UINT_PTR kScreenshotSelectionHoverProbeTimerId = 0xA13;
static constexpr UINT kScreenshotSelectionHoverProbeDelayMs = 60;
static constexpr UINT kScreenshotSelectionStartupHoverProbeDelayMs = 1200;
static constexpr wchar_t kScrollingCaptureOverlayWindowClassName[] = L"WoxScrollingCaptureOverlayWindow";
static constexpr wchar_t kScreenshotSelectionInputWindowClassName[] = L"WoxScreenshotSelectionInputWindow";
static constexpr wchar_t kScreenshotSelectionBorderWindowClassName[] = L"WoxScreenshotSelectionBorderWindow";
static constexpr double kScrollingCaptureToolbarSlotHeightDip = 72.0;
static constexpr double kScrollingCaptureToolbarHeightDip = 56.0;
static constexpr double kScrollingCaptureToolbarWidthDip = 124.0;
static constexpr double kScrollingCaptureToolbarCornerRadiusDip = 18.0;
static constexpr int kScreenshotHoverMinimumDimension = 200;
static constexpr int kScreenshotHoverDisplaySizedWidthPercent = 90;
static constexpr int kScreenshotHoverDisplaySizedHeightPercent = 75;
static constexpr int kScreenshotHoverChromeBandHeight = 96;
static constexpr int kScreenshotHoverChromeMaxHeight = 96;

// Store window instance for window procedure
FlutterWindow *g_window_instance = nullptr;
static std::once_flag g_gdiplus_init_once;
static ULONG_PTR g_gdiplus_token = 0;

// GetWindowsBuildNumberForCapabilities mirrors the backdrop support check without
// exposing Win32Window internals to the Flutter method channel.
static DWORD GetWindowsBuildNumberForCapabilities()
{
  OSVERSIONINFOEX osvi = {0};
  osvi.dwOSVersionInfoSize = sizeof(OSVERSIONINFOEX);
  typedef NTSTATUS(WINAPI *RtlGetVersionPtr)(PRTL_OSVERSIONINFOW);
  HMODULE h_ntdll = GetModuleHandleW(L"ntdll.dll");
  if (h_ntdll)
  {
    RtlGetVersionPtr rtl_get_version = reinterpret_cast<RtlGetVersionPtr>(GetProcAddress(h_ntdll, "RtlGetVersion"));
    if (rtl_get_version)
    {
      rtl_get_version(reinterpret_cast<PRTL_OSVERSIONINFOW>(&osvi));
    }
  }
  return osvi.dwBuildNumber;
}

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

// Write an HBITMAP to a temp PNG file so Dart receives only a small file-path payload.
static bool SaveBitmapToTempPngFile(HBITMAP bitmap, std::string &png_file_path, std::string &error)
{
  CLSID png_clsid{};
  if (!GetPngEncoderClsid(&png_clsid))
  {
    error = "Failed to find PNG encoder";
    return false;
  }

  wchar_t temp_directory[MAX_PATH + 1]{};
  const DWORD temp_directory_length = GetTempPathW(static_cast<DWORD>(_countof(temp_directory)), temp_directory);
  if (temp_directory_length == 0 || temp_directory_length >= _countof(temp_directory))
  {
    error = "Failed to resolve temp directory for monitor PNG";
    return false;
  }

  wchar_t temp_file_path[MAX_PATH + 1]{};
  if (GetTempFileNameW(temp_directory, L"wox", 0, temp_file_path) == 0)
  {
    error = "Failed to create temp monitor PNG path";
    return false;
  }

  // GDI+ writes the actual PNG bytes; remove the placeholder file created by GetTempFileNameW.
  DeleteFileW(temp_file_path);
  Gdiplus::Bitmap image(bitmap, nullptr);
  const auto status = image.Save(temp_file_path, &png_clsid, nullptr);
  if (status != Gdiplus::Ok)
  {
    DeleteFileW(temp_file_path);
    error = "Failed to write monitor image PNG file";
    return false;
  }

  png_file_path = Utf8FromUtf16(temp_file_path);
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

static long long RectArea(const RECT &rect)
{
  if (IsRectEmptyOrInvalid(rect))
  {
    return 0;
  }

  return static_cast<long long>(rect.right - rect.left) * static_cast<long long>(rect.bottom - rect.top);
}

static bool IsTooSmallForScreenshotHover(const RECT &rect)
{
  // UIA often exposes text runs, icons, and separators as valid rectangles. Hover auto-selection is
  // intended for windows and substantial controls, so require both dimensions to be screenshot-sized.
  const int width = rect.right - rect.left;
  const int height = rect.bottom - rect.top;
  return width <= kScreenshotHoverMinimumDimension || height <= kScreenshotHoverMinimumDimension;
}

static bool IsDisplaySizedScreenshotHoverCandidate(const RECT &rect, const RECT &display_bounds)
{
  const int width = rect.right - rect.left;
  const int height = rect.bottom - rect.top;
  const int display_width = display_bounds.right - display_bounds.left;
  const int display_height = display_bounds.bottom - display_bounds.top;
  return display_width > 0 &&
         display_height > 0 &&
         width >= display_width * kScreenshotHoverDisplaySizedWidthPercent / 100 &&
         height >= display_height * kScreenshotHoverDisplaySizedHeightPercent / 100;
}

static bool IsAlwaysSkippedScreenshotHoverUiaControlType(CONTROLTYPEID control_type)
{
  return control_type == UIA_TitleBarControlTypeId ||
         control_type == UIA_MenuBarControlTypeId ||
         control_type == UIA_MenuItemControlTypeId ||
         control_type == UIA_StatusBarControlTypeId ||
         control_type == UIA_SeparatorControlTypeId;
}

static bool IsTopChromeScreenshotHoverUiaControlType(CONTROLTYPEID control_type)
{
  return control_type == UIA_ButtonControlTypeId ||
         control_type == UIA_PaneControlTypeId ||
         control_type == UIA_ToolBarControlTypeId ||
         control_type == UIA_GroupControlTypeId ||
         control_type == UIA_CustomControlTypeId;
}

static RECT RectFromPoints(const POINT &first, const POINT &second)
{
  return RECT{
      first.x < second.x ? first.x : second.x,
      first.y < second.y ? first.y : second.y,
      first.x > second.x ? first.x : second.x,
      first.y > second.y ? first.y : second.y};
}

static bool IsManualScreenshotSelection(const RECT &rect)
{
  return rect.right - rect.left >= 4 && rect.bottom - rect.top >= 4;
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
  if (screenshot_uia_automation_ != nullptr)
  {
    screenshot_uia_automation_->Release();
    screenshot_uia_automation_ = nullptr;
  }

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

// Clone cached HBITMAPs before async encoding so cache cleanup cannot race the worker thread.
bool FlutterWindow::CloneDisplayCaptures(const std::vector<CachedDisplayCapture> &captures, std::vector<CachedDisplayCapture> *captures_out, std::string *error_out)
{
  if (captures_out == nullptr || error_out == nullptr)
  {
    return false;
  }

  captures_out->clear();
  error_out->clear();
  captures_out->reserve(captures.size());
  for (const auto &capture : captures)
  {
    auto bitmap_copy = static_cast<HBITMAP>(CopyImage(capture.bitmap, IMAGE_BITMAP, 0, 0, LR_CREATEDIBSECTION));
    if (bitmap_copy == nullptr)
    {
      ReleaseDisplayCaptures(captures_out);
      *error_out = "Failed to clone cached monitor bitmap for async payload";
      return false;
    }

    captures_out->push_back(CachedDisplayCapture{
        capture.display_id,
        capture.monitor_bounds,
        capture.scale,
        capture.rotation,
        bitmap_copy,
    });
  }

  return true;
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

bool FlutterWindow::BuildDisplaySnapshotPayloads(const std::vector<CachedDisplayCapture> &captures, FlutterWindow::ScreenshotImagePayloadMode payload_mode, flutter::EncodableList *snapshots_out, std::string *error_out)
{
  int payload_count = 0;
  ULONGLONG elapsed_ms = 0;
  if (!BuildDisplaySnapshotPayloadsCore(captures, payload_mode, snapshots_out, error_out, &payload_count, &elapsed_ms))
  {
    return false;
  }

  // Timing probe: the original Windows startup encoded every monitor before reveal. This log proves
  // whether a call is metadata-only or limited to the selected display payload.
  std::ostringstream oss;
  oss << "screenshot_timing event=windows_payload_build displayCount=" << captures.size()
      << " payloadCount=" << payload_count
      << " payloadMode=" << ScreenshotImagePayloadModeName(payload_mode)
      << " elapsedMs=" << elapsed_ms;
  Log(oss.str());
  return true;
}

// Build snapshot payloads without touching MethodChannel so it can run on a worker thread.
bool FlutterWindow::BuildDisplaySnapshotPayloadsCore(const std::vector<CachedDisplayCapture> &captures, FlutterWindow::ScreenshotImagePayloadMode payload_mode, flutter::EncodableList *snapshots_out, std::string *error_out, int *payload_count_out, ULONGLONG *elapsed_ms_out)
{
  if (snapshots_out == nullptr || error_out == nullptr)
  {
    return false;
  }

  snapshots_out->clear();
  error_out->clear();
  const ULONGLONG payload_start = GetTickCount64();
  int payload_count = 0;

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

    if (payload_mode == ScreenshotImagePayloadMode::kBase64)
    {
      std::string png_base64;
      if (!EncodeBitmapToPngBase64(capture.bitmap, png_base64, *error_out))
      {
        return false;
      }
      payload_count += 1;
      snapshot[flutter::EncodableValue("imageBytesBase64")] = flutter::EncodableValue(png_base64);
    }
    else if (payload_mode == ScreenshotImagePayloadMode::kFilePath)
    {
      std::string png_file_path;
      if (!SaveBitmapToTempPngFile(capture.bitmap, png_file_path, *error_out))
      {
        return false;
      }
      payload_count += 1;
      snapshot[flutter::EncodableValue("imageFilePath")] = flutter::EncodableValue(png_file_path);
    }

    snapshots_out->push_back(flutter::EncodableValue(snapshot));
  }

  if (payload_count_out != nullptr)
  {
    *payload_count_out = payload_count;
  }
  if (elapsed_ms_out != nullptr)
  {
    *elapsed_ms_out = GetTickCount64() - payload_start;
  }
  return true;
}

const char *FlutterWindow::ScreenshotImagePayloadModeName(FlutterWindow::ScreenshotImagePayloadMode payload_mode)
{
  switch (payload_mode)
  {
  case ScreenshotImagePayloadMode::kBase64:
    return "base64";
  case ScreenshotImagePayloadMode::kFilePath:
    return "file";
  case ScreenshotImagePayloadMode::kNone:
  default:
    return "none";
  }
}

// Encode selected display snapshots in the background, then marshal MethodResult completion home.
void FlutterWindow::BuildDisplaySnapshotPayloadsAsync(std::vector<CachedDisplayCapture> captures, std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result)
{
  const HWND target_window = GetHandle();
  std::thread([target_window, captures = std::move(captures), result = std::move(result)]() mutable {
    auto payload_result = std::make_unique<FlutterWindow::DisplaySnapshotPayloadAsyncResult>();
    payload_result->result = std::move(result);
    payload_result->display_count = captures.size();
    payload_result->payload_mode = FlutterWindow::ScreenshotImagePayloadMode::kFilePath;
    payload_result->success = FlutterWindow::BuildDisplaySnapshotPayloadsCore(
        captures,
        FlutterWindow::ScreenshotImagePayloadMode::kFilePath,
        &payload_result->snapshots,
        &payload_result->error,
        &payload_result->payload_count,
        &payload_result->elapsed_ms);
    FlutterWindow::ReleaseDisplayCaptures(&captures);

    auto *raw_result = payload_result.get();
    if (target_window != nullptr && PostMessage(target_window, kScreenshotDisplaySnapshotPayloadReadyMessage, 0, reinterpret_cast<LPARAM>(raw_result)))
    {
      payload_result.release();
      return;
    }

    if (target_window == nullptr)
    {
      // The app window is gone; dropping the pending MethodResult is safer than calling it from a
      // worker thread after the messenger has started tearing down.
      return;
    }
  }).detach();
}

// Complete the pending loadDisplaySnapshots MethodResult on the window thread.
void FlutterWindow::CompleteDisplaySnapshotPayloadAsyncResult(DisplaySnapshotPayloadAsyncResult *payload_result)
{
  if (payload_result == nullptr || payload_result->result == nullptr)
  {
    return;
  }

  std::ostringstream oss;
  oss << "screenshot_timing event=windows_payload_build displayCount=" << payload_result->display_count
      << " payloadCount=" << payload_result->payload_count
      << " payloadMode=" << ScreenshotImagePayloadModeName(payload_result->payload_mode)
      << " thread=background"
      << " elapsedMs=" << payload_result->elapsed_ms;
  Log(oss.str());

  if (!payload_result->success)
  {
    payload_result->result->Error("CAPTURE_ERROR", payload_result->error);
    return;
  }

  payload_result->result->Success(flutter::EncodableValue(payload_result->snapshots));
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
  screenshot_selection_overlay_state_.has_hover_selection = false;
  screenshot_selection_overlay_state_.workspace_bounds = workspace_bounds;
  screenshot_selection_overlay_state_.selection_bounds = {0, 0, 0, 0};
  screenshot_selection_overlay_state_.has_pending_hover_selection = false;
  screenshot_selection_overlay_state_.pending_hover_selection_bounds = {0, 0, 0, 0};
  screenshot_selection_overlay_state_.hover_selection_bounds = {0, 0, 0, 0};
  screenshot_selection_overlay_state_.hover_selection_source.clear();
  screenshot_selection_overlay_state_.hover_selection_root_window = nullptr;
  screenshot_selection_overlay_state_.hover_candidate_bounds.clear();
  screenshot_selection_overlay_state_.hover_candidate_root_window = nullptr;
  screenshot_selection_overlay_state_.has_last_hover_probe_point = false;
  screenshot_selection_overlay_state_.last_hover_probe_point = {0, 0};
  screenshot_selection_overlay_state_.last_hover_probe_tick = 0;
  screenshot_selection_overlay_state_.has_pending_hover_move = false;
  screenshot_selection_overlay_state_.pending_hover_move_point = {0, 0};
  screenshot_selection_overlay_state_.hover_move_message_posted = false;
  screenshot_selection_overlay_state_.has_pending_hover_probe = false;
  screenshot_selection_overlay_state_.pending_hover_probe_point = {0, 0};
  screenshot_selection_overlay_state_.pending_hover_probe_root_window = nullptr;
  screenshot_selection_overlay_state_.hover_probe_revision = 0;
  screenshot_selection_overlay_state_.pending_hover_probe_revision = 0;
  screenshot_selection_overlay_state_.hover_probe_timer_active = false;
  screenshot_selection_overlay_state_.hover_display_sized_uia_rejected = false;
  screenshot_selection_overlay_state_.last_hover_probe_slow_tick = 0;
  screenshot_selection_overlay_state_.last_hover_debug_signature.clear();
  screenshot_selection_overlay_state_.last_hover_debug_tick = 0;
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
  // Bug fix: the selector must show the captured desktop frame, not a translucent live desktop.
  // Keep the input HWND opaque and draw the dimming overlay in WM_PAINT so animated windows cannot
  // continue showing through after the snapshot has been taken.
  SetLayeredWindowAttributes(input_window, 0, 255, LWA_ALPHA);

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

  // Feature change: paint cached HBITMAP snapshots into the selector so the user sees the exact
  // captured frame. The previous translucent live-desktop mask was fast, but animated windows kept
  // moving underneath it and made the selection stage disagree with the final screenshot pixels.
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
    UpdateScreenshotSelectionHover(cursor_position);
    EmitScreenshotSelectionDisplayHint(cursor_position);
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
  const RECT committed_selection = ClampRectToBounds(screenshot_selection_overlay_state_.selection_bounds, workspace);
  const bool has_committed_selection = !IsRectEmptyOrInvalid(committed_selection);
  const RECT hover_selection = ClampRectToBounds(screenshot_selection_overlay_state_.hover_selection_bounds, workspace);
  const bool has_hover_selection = !has_committed_selection && screenshot_selection_overlay_state_.has_hover_selection && !IsRectEmptyOrInvalid(hover_selection);
  const RECT selection = has_committed_selection ? committed_selection : hover_selection;
  const bool has_selection = has_committed_selection || has_hover_selection;
  auto &border_windows = screenshot_selection_overlay_state_.border_windows;

  if (!has_selection)
  {
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
    // Feature change: hover previews reuse the same tiny border windows as committed drag
    // selections. The previous flow could only show a rectangle after mouse-down; keeping hover
    // feedback in the lightweight border path avoids recomputing the full dim region on every
    // mouse move while still making UIA/window targets visible before the click.
    MoveSelectionOverlayWindow(border_windows[0], RECT{selection.left, selection.top, selection.right, selection.top + border});
    MoveSelectionOverlayWindow(border_windows[1], RECT{selection.left, selection.bottom - border, selection.right, selection.bottom});
    MoveSelectionOverlayWindow(border_windows[2], RECT{selection.left, selection.top, selection.left + border, selection.bottom});
    MoveSelectionOverlayWindow(border_windows[3], RECT{selection.right - border, selection.top, selection.right, selection.bottom});
  }
}

// Apply a hover result from either the fast window path or the deferred UIA timer while preserving
// the UIA candidate cache unless the caller explicitly provides a fresh cache.
void FlutterWindow::ApplyScreenshotHoverSelection(const POINT &point, bool has_hover_selection, const RECT &hover_selection, const std::string &hover_source, HWND root_window, const std::vector<RECT> *candidate_bounds, bool update_candidate_bounds, ULONGLONG elapsed_ms)
{
  auto &state = screenshot_selection_overlay_state_;
  const RECT previous_hover_selection = state.hover_selection_bounds;
  const RECT next_hover_selection = has_hover_selection ? hover_selection : RECT{0, 0, 0, 0};
  const bool root_changed = state.hover_selection_root_window != root_window;
  const bool changed =
      has_hover_selection != state.has_hover_selection ||
      hover_source != state.hover_selection_source ||
      root_changed ||
      (has_hover_selection && !EqualRect(&next_hover_selection, &state.hover_selection_bounds)) ||
      (!has_hover_selection && !IsRectEmptyOrInvalid(state.hover_selection_bounds));

  if (update_candidate_bounds)
  {
    if (candidate_bounds != nullptr)
    {
      state.hover_candidate_bounds = *candidate_bounds;
      state.hover_candidate_root_window = !state.hover_candidate_bounds.empty() ? root_window : nullptr;
    }
    else
    {
      state.hover_candidate_bounds.clear();
      state.hover_candidate_root_window = nullptr;
    }
  }

  if (!has_hover_selection || hover_source != "window-display-sized" || root_changed)
  {
    state.hover_display_sized_uia_rejected = false;
  }

  if (!changed)
  {
    return;
  }

  state.has_hover_selection = has_hover_selection;
  state.hover_selection_bounds = next_hover_selection;
  state.hover_selection_source = hover_source;
  state.hover_selection_root_window = has_hover_selection ? root_window : nullptr;
  {
    std::ostringstream oss;
    oss << "screenshot_debug event=windows_hover_change point=" << point.x << "," << point.y
        << " has=" << (has_hover_selection ? "true" : "false")
        << " source=" << hover_source
        << " selection=" << RectToString(state.hover_selection_bounds)
        << " elapsedMs=" << elapsed_ms;
    LogScreenshotHoverDebug(std::string("change|") + hover_source + "|" + RectToString(state.hover_selection_bounds), oss.str());
  }
  // Hover preview uses the same paint-time cut-out as a committed selection. Keep the invalidation
  // local to the old and new rectangles so hover changes do not repaint the whole frozen desktop.
  CancelScreenshotSelectionDimRegionUpdate();
  LayoutScreenshotSelectionOverlay();
  InvalidateScreenshotSelectionDimChange(previous_hover_selection, state.hover_selection_bounds);
}

void FlutterWindow::UpdateScreenshotSelectionHover(const POINT &point)
{
  auto &state = screenshot_selection_overlay_state_;
  if (!state.active ||
      state.dragging ||
      state.completed)
  {
    return;
  }

  state.has_last_hover_probe_point = true;
  state.last_hover_probe_point = point;
  state.last_hover_probe_tick = GetTickCount64();
  HWND root_window = ResolveScreenshotHoverRootWindowAtPoint(point);
  if (!state.hover_candidate_bounds.empty() &&
      state.hover_candidate_root_window != nullptr &&
      root_window != state.hover_candidate_root_window)
  {
    state.hover_candidate_bounds.clear();
    state.hover_candidate_root_window = nullptr;
  }

  if (!state.hover_candidate_bounds.empty())
  {
    RECT cached_selection{};
    std::string cached_source;
    if (TryPickCachedHoverSelection(point, &cached_selection, &cached_source))
    {
      // UIA containers can contain smaller controls. Reusing the cached nested candidates keeps
      // mouse movement in local geometry after the deferred probe has populated the cache.
      ApplyScreenshotHoverSelection(point, true, cached_selection, cached_source, root_window, nullptr, false, 0);
      CancelScreenshotHoverProbeTimer();
      return;
    }
  }

  RECT hover_selection{};
  std::string hover_source;
  const bool has_hover_selection = TryResolveWindowHoverSelection(point, &hover_selection, &hover_source, nullptr);
  if (has_hover_selection)
  {
    ApplyScreenshotHoverSelection(point, true, hover_selection, hover_source, root_window, nullptr, false, 0);
    ScheduleScreenshotHoverProbe(point, root_window);
    return;
  }

  ApplyScreenshotHoverSelection(point, false, RECT{0, 0, 0, 0}, hover_source, nullptr, nullptr, true, 0);
  CancelScreenshotHoverProbeTimer();
}

// Keep the low-level mouse hook out of UIA/window probing. It may use the current UIA cache for
// immediate visual feedback, then coalesces the authoritative hover update through the input HWND.
void FlutterWindow::UpdateScreenshotSelectionHoverFromHook(const POINT &point)
{
  auto &state = screenshot_selection_overlay_state_;
  if (!state.active || state.dragging || state.completed)
  {
    return;
  }

  if (!state.hover_candidate_bounds.empty())
  {
    RECT cached_selection{};
    std::string cached_source;
    if (TryPickCachedHoverSelection(point, &cached_selection, &cached_source))
    {
      ApplyScreenshotHoverSelection(point, true, cached_selection, cached_source, state.hover_candidate_root_window, nullptr, false, 0);
    }
  }

  state.pending_hover_move_point = point;
  state.has_pending_hover_move = true;
  if (state.hover_move_message_posted || state.input_window == nullptr || !IsWindow(state.input_window))
  {
    return;
  }

  if (PostMessage(state.input_window, kScreenshotSelectionHoverMoveMessage, 0, 0))
  {
    state.hover_move_message_posted = true;
  }
}

// Arm a one-shot UIA refinement probe for the latest hover point. Re-arming the same timer
// coalesces fast mouse packets so UIA never runs in the mouse hook or for every pixel move.
void FlutterWindow::ScheduleScreenshotHoverProbe(const POINT &point, HWND root_window)
{
  auto &state = screenshot_selection_overlay_state_;
  if (!state.active || state.dragging || state.completed || state.input_window == nullptr || !IsWindow(state.input_window))
  {
    return;
  }

  if (root_window == nullptr ||
      (state.hover_selection_source == "window-display-sized" && state.hover_display_sized_uia_rejected))
  {
    CancelScreenshotHoverProbeTimer();
    return;
  }

  state.has_pending_hover_probe = true;
  state.pending_hover_probe_point = point;
  state.pending_hover_probe_root_window = root_window;
  state.pending_hover_probe_revision = ++state.hover_probe_revision;
  const ULONGLONG now = GetTickCount64();
  const ULONGLONG age = state.started_tick == 0 || now < state.started_tick ? 0 : now - state.started_tick;
  const UINT delay = age < kScreenshotSelectionStartupHoverProbeDelayMs ? kScreenshotSelectionStartupHoverProbeDelayMs : kScreenshotSelectionHoverProbeDelayMs;
  if (SetTimer(state.input_window, kScreenshotSelectionHoverProbeTimerId, delay, nullptr) != 0)
  {
    state.hover_probe_timer_active = true;
    return;
  }

  state.has_pending_hover_probe = false;
  state.pending_hover_probe_root_window = nullptr;
  state.hover_probe_timer_active = false;
}

// Cancel any deferred UIA work and bump the revision so queued timer messages cannot apply later.
void FlutterWindow::CancelScreenshotHoverProbeTimer()
{
  auto &state = screenshot_selection_overlay_state_;
  if (state.hover_probe_timer_active && state.input_window != nullptr && IsWindow(state.input_window))
  {
    KillTimer(state.input_window, kScreenshotSelectionHoverProbeTimerId);
  }

  state.hover_probe_timer_active = false;
  state.has_pending_hover_probe = false;
  state.pending_hover_probe_root_window = nullptr;
  ++state.hover_probe_revision;
}

// Run the deferred UIA probe only if the cursor is still in the same native window target that
// scheduled it. This prevents late UIA results from replacing newer hover/window feedback.
void FlutterWindow::HandleScreenshotHoverProbeTimer()
{
  auto &state = screenshot_selection_overlay_state_;
  if (state.input_window != nullptr && IsWindow(state.input_window))
  {
    KillTimer(state.input_window, kScreenshotSelectionHoverProbeTimerId);
  }
  state.hover_probe_timer_active = false;

  if (!state.active || state.dragging || state.completed || !state.has_pending_hover_probe)
  {
    return;
  }

  const POINT probe_point = state.pending_hover_probe_point;
  HWND probe_root_window = state.pending_hover_probe_root_window;
  const ULONGLONG probe_revision = state.pending_hover_probe_revision;
  state.has_pending_hover_probe = false;
  state.pending_hover_probe_root_window = nullptr;

  if (probe_root_window == nullptr ||
      (state.hover_selection_source == "window-display-sized" && state.hover_display_sized_uia_rejected) ||
      probe_revision != state.hover_probe_revision)
  {
    return;
  }

  POINT cursor_point = probe_point;
  GetCursorPos(&cursor_point);
  if (ResolveScreenshotHoverRootWindowAtPoint(cursor_point) != probe_root_window)
  {
    return;
  }

  RECT uia_selection{};
  std::string uia_source;
  std::vector<RECT> uia_candidates;
  const ULONGLONG resolve_start = GetTickCount64();
  const bool has_uia_selection = TryResolveUiaHoverSelection(probe_point, &uia_selection, &uia_source, &uia_candidates);
  const ULONGLONG resolve_elapsed = GetTickCount64() - resolve_start;
  if (resolve_elapsed > 16)
  {
    const ULONGLONG now = GetTickCount64();
    if (now - state.last_hover_probe_slow_tick >= 150)
    {
      state.last_hover_probe_slow_tick = now;
      std::ostringstream oss;
      oss << "screenshot_debug event=windows_hover_uia_timer_slow point=" << probe_point.x << "," << probe_point.y
          << " source=" << uia_source
          << " elapsedMs=" << resolve_elapsed;
      Log(oss.str());
    }
  }

  if (!state.active || state.dragging || state.completed || probe_revision != state.hover_probe_revision)
  {
    return;
  }

  cursor_point = probe_point;
  GetCursorPos(&cursor_point);
  if (ResolveScreenshotHoverRootWindowAtPoint(cursor_point) != probe_root_window ||
      (has_uia_selection && !IsPointInRect(uia_selection, cursor_point)))
  {
    return;
  }
  if (!has_uia_selection && (cursor_point.x != probe_point.x || cursor_point.y != probe_point.y))
  {
    return;
  }

  if (has_uia_selection)
  {
    ApplyScreenshotHoverSelection(probe_point, true, uia_selection, uia_source, probe_root_window, &uia_candidates, true, resolve_elapsed);
    return;
  }

  if (uia_source == "uia-display-sized-rejected" &&
      state.hover_selection_source == "window-display-sized" &&
      state.hover_selection_root_window == probe_root_window)
  {
    state.hover_display_sized_uia_rejected = true;
  }
  if (state.hover_candidate_root_window == probe_root_window)
  {
    state.hover_candidate_bounds.clear();
    state.hover_candidate_root_window = nullptr;
  }
}

void FlutterWindow::EmitScreenshotSelectionDisplayHint(const POINT &point)
{
  if (window_manager_channel_ == nullptr)
  {
    return;
  }

  const auto *capture = DisplayCaptureForPoint(point);
  if (capture == nullptr)
  {
    return;
  }

  flutter::EncodableMap payload;
  payload[flutter::EncodableValue("displayId")] = flutter::EncodableValue(Utf8FromUtf16(capture->display_id.c_str()));
  payload[flutter::EncodableValue("displayBounds")] = flutter::EncodableValue(BuildRectValue(capture->monitor_bounds));
  window_manager_channel_->InvokeMethod("onSelectionDisplayHint", std::make_unique<flutter::EncodableValue>(payload));
}

void FlutterWindow::LogScreenshotHoverDebug(const std::string &signature, const std::string &message)
{
  const ULONGLONG now = GetTickCount64();
  auto &state = screenshot_selection_overlay_state_;
  if (now - state.last_hover_debug_tick < 150)
  {
    // Diagnostic logging goes through the Flutter method channel. It is useful while tuning hover
    // hit-tests, but without a global cap it can become part of the mouse-move slowdown being
    // diagnosed, so keep it below the pointer event rate.
    return;
  }

  if (signature == state.last_hover_debug_signature && now - state.last_hover_debug_tick < 1000)
  {
    return;
  }

  state.last_hover_debug_signature = signature;
  state.last_hover_debug_tick = now;
  Log(message);
}

// Pick the innermost practical hover target by choosing the smallest normalized candidate that
// still contains the cursor.
bool FlutterWindow::TryPickSmallestHoverCandidate(const POINT &point, const std::vector<RECT> &candidate_bounds, RECT *selection_out) const
{
  if (selection_out == nullptr || candidate_bounds.empty())
  {
    return false;
  }

  bool found = false;
  RECT best_selection{};
  long long best_area = 0;
  for (const auto &candidate : candidate_bounds)
  {
    RECT normalized{};
    if (!NormalizeHoverSelectionRect(point, candidate, &normalized) || !IsPointInRect(normalized, point))
    {
      continue;
    }

    const long long area = RectArea(normalized);
    if (!found || area < best_area)
    {
      found = true;
      best_selection = normalized;
      best_area = area;
    }
  }

  if (!found)
  {
    return false;
  }

  *selection_out = best_selection;
  return true;
}

bool FlutterWindow::TryPickCachedHoverSelection(const POINT &point, RECT *selection_out, std::string *source_out) const
{
  if (!TryPickSmallestHoverCandidate(point, screenshot_selection_overlay_state_.hover_candidate_bounds, selection_out))
  {
    return false;
  }

  if (source_out != nullptr)
  {
    *source_out = "uia-cache";
  }
  return true;
}

bool FlutterWindow::TryResolveUiaHoverSelection(const POINT &point, RECT *selection_out, std::string *source_out, std::vector<RECT> *candidate_bounds_out)
{
  IUIAutomation *automation = EnsureScreenshotUiaAutomation();
  if (automation == nullptr || selection_out == nullptr)
  {
    if (source_out != nullptr)
    {
      *source_out = "uia-unavailable";
    }
    return false;
  }

  std::vector<RECT> local_candidate_bounds;
  std::vector<RECT> *candidate_bounds = candidate_bounds_out != nullptr ? candidate_bounds_out : &local_candidate_bounds;
  candidate_bounds->clear();
  bool display_sized_rejected = false;

  IUIAutomationElement *point_element = nullptr;
  if (SUCCEEDED(automation->ElementFromPoint(point, &point_element)) && point_element != nullptr)
  {
    RECT point_bounds{};
    const bool has_point_bounds = TryGetUiaElementBounds(point_element, &point_bounds);
    if (has_point_bounds)
    {
      const RECT display_bounds = DisplayBoundsForPoint(point);
      RECT display_intersection{};
      if (TryIntersectRects(point_bounds, display_bounds, &display_intersection) &&
          IsDisplaySizedScreenshotHoverCandidate(display_intersection, display_bounds))
      {
        std::ostringstream oss;
        oss << "screenshot_debug event=windows_hover_uia_display_sized_rejected point=" << point.x << "," << point.y
            << " bounds=" << RectToString(point_bounds);
        LogScreenshotHoverDebug(std::string("uia_display_sized_rejected|") + RectToString(point_bounds), oss.str());
        display_sized_rejected = true;
      }
    }

    if (has_point_bounds)
    {
      AddHoverCandidateRect(point_bounds, candidate_bounds);
      IUIAutomationTreeWalker *point_walker = nullptr;
      if (SUCCEEDED(automation->get_ControlViewWalker(&point_walker)) && point_walker != nullptr)
      {
        RECT ignored_bounds{};
        TryFindDeepestUiaElementBounds(point_walker, point_element, point, 0, &ignored_bounds, candidate_bounds);
        point_walker->Release();
      }

      if (TryPickSmallestHoverCandidate(point, *candidate_bounds, selection_out))
      {
        if (source_out != nullptr)
        {
          *source_out = "uia-point";
        }
        point_element->Release();
        return true;
      }
    }

    if (has_point_bounds)
    {
      std::ostringstream oss;
      oss << "screenshot_debug event=windows_hover_uia_point_rejected point=" << point.x << "," << point.y
          << " bounds=" << RectToString(point_bounds);
      LogScreenshotHoverDebug(std::string("uia_point_rejected|") + RectToString(point_bounds), oss.str());
    }
    point_element->Release();
  }

  HWND underlying_window = FindUnderlyingWindowAtPoint(point);
  if (underlying_window == nullptr)
  {
    if (source_out != nullptr)
    {
      *source_out = display_sized_rejected ? "uia-display-sized-rejected" : "uia-no-underlying-window";
    }
    std::ostringstream oss;
    oss << "screenshot_debug event=windows_hover_no_underlying_window point=" << point.x << "," << point.y;
    LogScreenshotHoverDebug("uia_no_underlying_window", oss.str());
    return false;
  }

  IUIAutomationElement *window_element = nullptr;
  if (FAILED(automation->ElementFromHandle(underlying_window, &window_element)) || window_element == nullptr)
  {
    if (source_out != nullptr)
    {
      *source_out = display_sized_rejected ? "uia-display-sized-rejected" : "uia-handle-failed";
    }
    return false;
  }

  IUIAutomationTreeWalker *walker = nullptr;
  const HRESULT walker_result = automation->get_ControlViewWalker(&walker);
  if (FAILED(walker_result) || walker == nullptr)
  {
    window_element->Release();
    if (source_out != nullptr)
    {
      *source_out = display_sized_rejected ? "uia-display-sized-rejected" : "uia-walker-failed";
    }
    return false;
  }

  RECT deepest_bounds{};
  const bool found_deepest = TryFindDeepestUiaElementBounds(walker, window_element, point, 0, &deepest_bounds, candidate_bounds);
  const bool found = found_deepest && TryPickSmallestHoverCandidate(point, *candidate_bounds, selection_out);
  walker->Release();
  window_element->Release();
  if (found && source_out != nullptr)
  {
    *source_out = "uia-tree";
  }
  else if (found_deepest)
  {
    std::ostringstream oss;
    oss << "screenshot_debug event=windows_hover_uia_tree_rejected point=" << point.x << "," << point.y
        << " bounds=" << RectToString(deepest_bounds)
        << " hwnd=" << reinterpret_cast<uintptr_t>(underlying_window);
    LogScreenshotHoverDebug(std::string("uia_tree_rejected|") + RectToString(deepest_bounds), oss.str());
  }
  if (!found && display_sized_rejected && source_out != nullptr)
  {
    *source_out = "uia-display-sized-rejected";
  }
  return found;
}

bool FlutterWindow::TryResolveWindowHoverSelection(const POINT &point, RECT *selection_out, std::string *source_out, std::vector<RECT> *candidate_bounds_out)
{
  HWND target_window = FindUnderlyingWindowAtPoint(point);
  if (target_window == nullptr || selection_out == nullptr)
  {
    if (source_out != nullptr)
    {
      *source_out = "window-no-target";
    }
    return false;
  }

  HWND root_window = GetAncestor(target_window, GA_ROOTOWNER);
  if (root_window == nullptr || !IsSelectableScreenshotHoverWindow(root_window))
  {
    root_window = target_window;
  }
  if (!IsSelectableScreenshotHoverWindow(root_window))
  {
    if (source_out != nullptr)
    {
      *source_out = "window-not-selectable";
    }
    return false;
  }

  RECT window_bounds{};
  if (FAILED(DwmGetWindowAttribute(root_window, DWMWA_EXTENDED_FRAME_BOUNDS, &window_bounds, sizeof(window_bounds))) ||
      IsRectEmptyOrInvalid(window_bounds))
  {
    if (!GetWindowRect(root_window, &window_bounds))
    {
      return false;
    }
  }

  const RECT display_bounds = DisplayBoundsForPoint(point);
  RECT display_intersection{};
  const bool is_display_sized_window =
      IsZoomed(root_window) &&
      TryIntersectRects(window_bounds, display_bounds, &display_intersection) &&
      IsDisplaySizedScreenshotHoverCandidate(display_intersection, display_bounds);

  if (NormalizeHoverSelectionRect(point, window_bounds, selection_out, true))
  {
    if (candidate_bounds_out != nullptr)
    {
      candidate_bounds_out->clear();
      candidate_bounds_out->push_back(*selection_out);
    }
    if (source_out != nullptr)
    {
      *source_out = is_display_sized_window ? "window-display-sized" : "window";
    }
    return true;
  }

  std::ostringstream oss;
  oss << "screenshot_debug event=windows_hover_window_rejected point=" << point.x << "," << point.y
      << " bounds=" << RectToString(window_bounds)
      << " hwnd=" << reinterpret_cast<uintptr_t>(root_window);
  LogScreenshotHoverDebug(std::string("window_rejected|") + RectToString(window_bounds), oss.str());
  if (source_out != nullptr)
  {
    *source_out = "window-rejected";
  }
  return false;
}

bool FlutterWindow::TryGetUiaElementBounds(IUIAutomationElement *element, RECT *bounds_out)
{
  if (element == nullptr || bounds_out == nullptr)
  {
    return false;
  }

  BOOL is_offscreen = FALSE;
  if (FAILED(element->get_CurrentIsOffscreen(&is_offscreen)) || is_offscreen)
  {
    return false;
  }

  RECT bounds{};
  if (FAILED(element->get_CurrentBoundingRectangle(&bounds)) || IsRectEmptyOrInvalid(bounds))
  {
    return false;
  }

  UIA_HWND native_window_value = 0;
  HWND native_window = nullptr;
  if (SUCCEEDED(element->get_CurrentNativeWindowHandle(&native_window_value)) && native_window_value != 0)
  {
    // Bug fix: UIA can hit desktop/taskbar child elements that still report valid bounds. Reject
    // elements whose native HWND belongs to a non-selectable shell/helper window so empty desktop
    // areas do not become hover targets before the window fallback gets a chance to run.
    native_window = reinterpret_cast<HWND>(native_window_value);
    if (!IsSelectableScreenshotHoverWindow(native_window))
    {
      return false;
    }
  }

  if (IsChromeLikeScreenshotHoverUiaElement(element, bounds, native_window))
  {
    return false;
  }

  *bounds_out = bounds;
  return true;
}

bool FlutterWindow::IsChromeLikeScreenshotHoverUiaElement(IUIAutomationElement *element, const RECT &bounds, HWND native_window)
{
  if (element == nullptr)
  {
    return false;
  }

  CONTROLTYPEID control_type = 0;
  if (FAILED(element->get_CurrentControlType(&control_type)))
  {
    return false;
  }

  if (IsAlwaysSkippedScreenshotHoverUiaControlType(control_type))
  {
    return true;
  }

  if (!IsTopChromeScreenshotHoverUiaControlType(control_type))
  {
    return false;
  }

  const int height = bounds.bottom - bounds.top;
  if (height <= 0 || height > kScreenshotHoverChromeMaxHeight)
  {
    return false;
  }

  HWND reference_window = native_window;
  if (reference_window == nullptr)
  {
    const POINT center{(bounds.left + bounds.right) / 2, (bounds.top + bounds.bottom) / 2};
    reference_window = FindUnderlyingWindowAtPoint(center);
  }
  if (reference_window == nullptr)
  {
    return false;
  }

  HWND root_window = GetAncestor(reference_window, GA_ROOTOWNER);
  if (root_window == nullptr || !IsSelectableScreenshotHoverWindow(root_window))
  {
    root_window = reference_window;
  }

  RECT root_bounds{};
  if (FAILED(DwmGetWindowAttribute(root_window, DWMWA_EXTENDED_FRAME_BOUNDS, &root_bounds, sizeof(root_bounds))) ||
      IsRectEmptyOrInvalid(root_bounds))
  {
    if (!GetWindowRect(root_window, &root_bounds))
    {
      return false;
    }
  }

  // Bug fix: caption buttons and titlebar toolbars are not tiny by absolute pixel size, so the
  // generic minimum-size filter lets them through. Treat short button/pane/toolbar candidates near
  // the top edge of their owning window as window chrome and let the user drag manually if they
  // really need that strip.
  return bounds.top >= root_bounds.top - 2 && bounds.bottom <= root_bounds.top + kScreenshotHoverChromeBandHeight;
}

void FlutterWindow::AddHoverCandidateRect(const RECT &candidate, std::vector<RECT> *candidate_bounds_out) const
{
  if (candidate_bounds_out == nullptr || IsRectEmptyOrInvalid(candidate))
  {
    return;
  }

  if (IsTooSmallForScreenshotHover(candidate))
  {
    return;
  }

  for (const auto &existing : *candidate_bounds_out)
  {
    if (EqualRect(&existing, &candidate))
    {
      return;
    }
  }

  if (candidate_bounds_out->size() >= 160)
  {
    return;
  }

  candidate_bounds_out->push_back(candidate);
}

bool FlutterWindow::TryFindDeepestUiaElementBounds(IUIAutomationTreeWalker *walker, IUIAutomationElement *element, const POINT &point, int depth, RECT *bounds_out, std::vector<RECT> *candidate_bounds_out)
{
  if (walker == nullptr || element == nullptr || bounds_out == nullptr || depth > 12)
  {
    return false;
  }

  RECT current_bounds{};
  if (!TryGetUiaElementBounds(element, &current_bounds) || !IsPointInRect(current_bounds, point))
  {
    return false;
  }

  AddHoverCandidateRect(current_bounds, candidate_bounds_out);
  RECT best_bounds = current_bounds;
  IUIAutomationElement *child = nullptr;
  HRESULT child_result = walker->GetFirstChildElement(element, &child);
  int scanned_children = 0;
  while (SUCCEEDED(child_result) && child != nullptr && scanned_children < 80)
  {
    RECT child_bounds{};
    if (TryGetUiaElementBounds(child, &child_bounds))
    {
      RECT nested_intersection{};
      if (TryIntersectRects(current_bounds, child_bounds, &nested_intersection) && !IsRectEmptyOrInvalid(nested_intersection))
      {
        // Optimization fix: cache sibling/child rectangles while walking the current UIA path. On
        // later mouse moves inside the same large container, the overlay can switch to a smaller
        // nested rectangle with local geometry instead of doing another cross-process UIA probe.
        AddHoverCandidateRect(child_bounds, candidate_bounds_out);
      }
    }

    if (IsPointInRect(child_bounds, point) && TryFindDeepestUiaElementBounds(walker, child, point, depth + 1, &child_bounds, candidate_bounds_out))
    {
      best_bounds = child_bounds;
    }

    IUIAutomationElement *next_child = nullptr;
    child_result = walker->GetNextSiblingElement(child, &next_child);
    child->Release();
    child = next_child;
    ++scanned_children;
  }

  *bounds_out = best_bounds;
  return true;
}

bool FlutterWindow::NormalizeHoverSelectionRect(const POINT &point, const RECT &candidate, RECT *selection_out, bool allow_display_sized_candidate) const
{
  if (selection_out == nullptr || IsRectEmptyOrInvalid(candidate))
  {
    return false;
  }

  const RECT workspace = screenshot_selection_overlay_state_.workspace_bounds;
  RECT workspace_intersection{};
  if (!TryIntersectRects(candidate, workspace, &workspace_intersection) || !IsPointInRect(workspace_intersection, point))
  {
    return false;
  }

  const RECT display_bounds = DisplayBoundsForPoint(point);
  RECT display_intersection{};
  if (!TryIntersectRects(workspace_intersection, display_bounds, &display_intersection) || !IsPointInRect(display_intersection, point))
  {
    return false;
  }

  if (IsTooSmallForScreenshotHover(display_intersection))
  {
    return false;
  }

  if (!allow_display_sized_candidate && IsDisplaySizedScreenshotHoverCandidate(display_intersection, display_bounds))
  {
    // Bug fix: UIA can return a desktop, monitor, wallpaper, or full-screen pane rectangle before
    // it exposes the underlying app control. Accepting that as a hover hit made empty desktop areas
    // selectable, so only validated window fallback candidates may cover most of a display.
    return false;
  }

  *selection_out = display_intersection;
  return true;
}

bool FlutterWindow::IsScreenshotOverlayWindow(HWND hwnd)
{
  if (hwnd == nullptr)
  {
    return true;
  }

  if (hwnd == screenshot_selection_overlay_state_.input_window || hwnd == GetHandle() || hwnd == child_window_)
  {
    return true;
  }

  for (const auto border_window : screenshot_selection_overlay_state_.border_windows)
  {
    if (hwnd == border_window)
    {
      return true;
    }
  }

  wchar_t class_name[128]{};
  GetClassNameW(hwnd, class_name, static_cast<int>(_countof(class_name)));
  return wcscmp(class_name, kScreenshotSelectionInputWindowClassName) == 0 ||
         wcscmp(class_name, kScreenshotSelectionBorderWindowClassName) == 0 ||
         wcscmp(class_name, kScrollingCaptureOverlayWindowClassName) == 0 ||
         wcscmp(class_name, L"Shell_TrayWnd") == 0 ||
         wcscmp(class_name, L"Shell_SecondaryTrayWnd") == 0 ||
         wcscmp(class_name, L"Progman") == 0 ||
         wcscmp(class_name, L"WorkerW") == 0;
}

bool FlutterWindow::IsSelectableScreenshotHoverWindow(HWND hwnd)
{
  // Bug fix: hover selection previously treated any visible HWND-sized rectangle as selectable,
  // including shell desktop children and fully transparent helper windows. Centralizing the native
  // visibility checks keeps both UIA native-window hits and window fallback from previewing areas
  // that the user cannot meaningfully select as an app/window/control.
  if (hwnd == nullptr || IsScreenshotOverlayWindow(hwnd))
  {
    return false;
  }

  HWND root_window = GetAncestor(hwnd, GA_ROOT);
  if (root_window != nullptr && root_window != hwnd && IsScreenshotOverlayWindow(root_window))
  {
    return false;
  }

  HWND visibility_window = root_window != nullptr ? root_window : hwnd;
  if (!IsWindowVisible(visibility_window) || IsIconic(visibility_window))
  {
    return false;
  }

  BOOL is_cloaked = FALSE;
  if (SUCCEEDED(DwmGetWindowAttribute(visibility_window, DWMWA_CLOAKED, &is_cloaked, sizeof(is_cloaked))) && is_cloaked)
  {
    return false;
  }

  LONG_PTR ex_style = GetWindowLongPtr(visibility_window, GWL_EXSTYLE);
  if ((ex_style & WS_EX_LAYERED) != 0)
  {
    COLORREF color_key = 0;
    BYTE alpha = 255;
    DWORD flags = 0;
    if (GetLayeredWindowAttributes(visibility_window, &color_key, &alpha, &flags) && (flags & LWA_ALPHA) != 0 && alpha == 0)
    {
      return false;
    }
  }

  return true;
}

HWND FlutterWindow::FindUnderlyingWindowAtPoint(const POINT &point)
{
  struct EnumContext
  {
    FlutterWindow *window = nullptr;
    POINT point{0, 0};
    HWND found = nullptr;
  } context{this, point, nullptr};

  EnumWindows(
      [](HWND hwnd, LPARAM lparam) -> BOOL {
        auto *context = reinterpret_cast<EnumContext *>(lparam);
        if (context == nullptr || context->window == nullptr)
        {
          return FALSE;
        }

        if (!context->window->IsSelectableScreenshotHoverWindow(hwnd))
        {
          return TRUE;
        }

        RECT bounds{};
        if (FAILED(DwmGetWindowAttribute(hwnd, DWMWA_EXTENDED_FRAME_BOUNDS, &bounds, sizeof(bounds))) ||
            IsRectEmptyOrInvalid(bounds))
        {
          if (!GetWindowRect(hwnd, &bounds))
          {
            return TRUE;
          }
        }

        if (!IsPointInRect(bounds, context->point))
        {
          return TRUE;
        }

        // Window-level fallback must look past Wox's screenshot helper windows. EnumWindows walks
        // top-level windows in z-order, so the first visible non-Wox window under the cursor is the
        // same target a user expects when UIA cannot provide a useful control rectangle.
        context->found = hwnd;
        return FALSE;
      },
      reinterpret_cast<LPARAM>(&context));

  return context.found;
}

// Resolve the selectable top-level owner used to validate deferred UIA hover results.
HWND FlutterWindow::ResolveScreenshotHoverRootWindowAtPoint(const POINT &point)
{
  HWND target_window = FindUnderlyingWindowAtPoint(point);
  if (target_window == nullptr)
  {
    return nullptr;
  }

  HWND root_window = GetAncestor(target_window, GA_ROOTOWNER);
  if (root_window == nullptr || !IsSelectableScreenshotHoverWindow(root_window))
  {
    root_window = target_window;
  }
  if (!IsSelectableScreenshotHoverWindow(root_window))
  {
    return nullptr;
  }

  return root_window;
}

IUIAutomation *FlutterWindow::EnsureScreenshotUiaAutomation()
{
  if (screenshot_uia_automation_ != nullptr)
  {
    return screenshot_uia_automation_;
  }

  IUIAutomation *automation = nullptr;
  if (FAILED(CoCreateInstance(CLSID_CUIAutomation, nullptr, CLSCTX_INPROC_SERVER, IID_PPV_ARGS(&automation))) || automation == nullptr)
  {
    return nullptr;
  }

  // Feature change: screenshot hover preview needs UIA at mouse-move speed, while the existing Go
  // selection path creates UIA only for focused text reads. Caching the automation object for the
  // runner process keeps the new control-level preview local to the native overlay without adding a
  // cross-process Flutter/Core call on every pointer update.
  screenshot_uia_automation_ = automation;
  return screenshot_uia_automation_;
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

  // Bug fix: the selection input window now paints a frozen screenshot background. Physically
  // cutting a hole in the HWND region would reveal the live desktop inside the selection, so the
  // undimmed center is now a paint-time cut-out on top of the cached bitmap.
  SetOwnedWindowRegion(screenshot_selection_overlay_state_.input_window, nullptr);
  InvalidateRect(screenshot_selection_overlay_state_.input_window, nullptr, FALSE);
}

void FlutterWindow::InvalidateScreenshotSelectionDimChange(const RECT &old_selection_bounds, const RECT &new_selection_bounds)
{
  HWND input_window = screenshot_selection_overlay_state_.input_window;
  if (input_window == nullptr || !IsWindow(input_window))
  {
    return;
  }

  const RECT workspace = screenshot_selection_overlay_state_.workspace_bounds;
  RECT dirty{};
  bool has_dirty = false;
  auto addDirtySelection = [&](const RECT &selection) {
    RECT clamped = ClampRectToBounds(selection, workspace);
    if (IsRectEmptyOrInvalid(clamped))
    {
      return;
    }

    RECT local = LocalRectForWorkspace(clamped, workspace);
    InflateRect(&local, 3, 3);
    RECT client_rect{};
    GetClientRect(input_window, &client_rect);
    RECT clipped{};
    if (!TryIntersectRects(local, client_rect, &clipped))
    {
      return;
    }

    if (!has_dirty)
    {
      dirty = clipped;
      has_dirty = true;
      return;
    }

    dirty.left = std::min(dirty.left, clipped.left);
    dirty.top = std::min(dirty.top, clipped.top);
    dirty.right = std::max(dirty.right, clipped.right);
    dirty.bottom = std::max(dirty.bottom, clipped.bottom);
  };

  // Performance fix: the frozen screenshot backdrop made each drag move repaint the whole virtual
  // desktop. Only pixels inside the old or new selection can change dimming state, so invalidate
  // that union while keeping the rest of the static screenshot untouched.
  addDirtySelection(old_selection_bounds);
  addDirtySelection(new_selection_bounds);
  if (has_dirty)
  {
    InvalidateRect(input_window, &dirty, FALSE);
  }
}

void FlutterWindow::UpdateScreenshotSelectionOverlay(const RECT &selection_bounds)
{
  if (!screenshot_selection_overlay_state_.active)
  {
    return;
  }

  auto &state = screenshot_selection_overlay_state_;
  const RECT previous_selection = state.selection_bounds;
  const RECT previous_hover_selection = state.hover_selection_bounds;
  const RECT clamped_selection = ClampRectToBounds(selection_bounds, state.workspace_bounds);
  const bool has_hover_state =
      state.has_hover_selection ||
      !IsRectEmptyOrInvalid(state.hover_selection_bounds) ||
      !state.hover_selection_source.empty() ||
      state.hover_selection_root_window != nullptr ||
      !state.hover_candidate_bounds.empty() ||
      state.hover_candidate_root_window != nullptr ||
      state.has_last_hover_probe_point ||
      state.last_hover_probe_tick != 0 ||
      state.has_pending_hover_move ||
      state.hover_move_message_posted ||
      state.has_pending_hover_probe ||
      state.hover_probe_timer_active;
  if (EqualRect(&previous_selection, &clamped_selection) && !has_hover_state)
  {
    // Performance fix: the low-level mouse hook and the input HWND can both report the same drag
    // point. Once the frozen screenshot is painted by dirty tiles, duplicate no-op rectangles still
    // cost border layout and invalidation work, so ignore exact repeats after hover state is clear.
    return;
  }

  state.selection_bounds = clamped_selection;
  state.has_hover_selection = false;
  state.hover_selection_bounds = {0, 0, 0, 0};
  state.hover_selection_source.clear();
  state.hover_selection_root_window = nullptr;
  state.hover_candidate_bounds.clear();
  state.hover_candidate_root_window = nullptr;
  state.has_last_hover_probe_point = false;
  state.last_hover_probe_tick = 0;
  state.has_pending_hover_move = false;
  state.hover_move_message_posted = false;
  state.hover_display_sized_uia_rejected = false;
  CancelScreenshotHoverProbeTimer();
  // Drag feedback stays native. The one-shot display hint may still warm Flutter in parallel, but
  // it no longer carries base64 PNGs through the channel on the mouse-move path.
  LayoutScreenshotSelectionOverlay();
  CancelScreenshotSelectionDimRegionUpdate();
  InvalidateScreenshotSelectionDimChange(previous_selection, clamped_selection);
  InvalidateScreenshotSelectionDimChange(previous_hover_selection, clamped_selection);
}

void FlutterWindow::BeginScreenshotSelectionPointerDown(const POINT &point)
{
  if (!screenshot_selection_overlay_state_.active || screenshot_selection_overlay_state_.completed || screenshot_selection_overlay_state_.dragging)
  {
    return;
  }

  screenshot_selection_overlay_state_.dragging = true;
  screenshot_selection_overlay_state_.drag_start = point;
  screenshot_selection_overlay_state_.has_pending_hover_selection = screenshot_selection_overlay_state_.has_hover_selection && !IsRectEmptyOrInvalid(screenshot_selection_overlay_state_.hover_selection_bounds);
  screenshot_selection_overlay_state_.pending_hover_selection_bounds =
      screenshot_selection_overlay_state_.has_pending_hover_selection ? screenshot_selection_overlay_state_.hover_selection_bounds : RECT{0, 0, 0, 0};

  // Bug fix: a hover preview should be committed only after mouse-up confirms the user did not draw
  // a manual rectangle. Mouse-down now records the hover candidate but still starts the normal drag
  // path, so pressing and dragging keeps manual box selection available.
  UpdateScreenshotSelectionOverlay(RectFromPoints(point, point));
}

void FlutterWindow::CompleteScreenshotSelectionPointerUp(const POINT &point)
{
  if (!screenshot_selection_overlay_state_.active || !screenshot_selection_overlay_state_.dragging)
  {
    return;
  }

  const RECT manual_selection = RectFromPoints(screenshot_selection_overlay_state_.drag_start, point);
  const bool should_use_pending_hover =
      screenshot_selection_overlay_state_.has_pending_hover_selection &&
      !IsManualScreenshotSelection(manual_selection);

  screenshot_selection_overlay_state_.dragging = false;
  if (should_use_pending_hover)
  {
    // Bug fix: plain click on a hover preview still enters annotation, but only at mouse-up after
    // confirming no meaningful manual rectangle was drawn. This preserves the new control-level
    // click behavior without stealing the user's chance to drag from the same starting point.
    screenshot_selection_overlay_state_.selection_bounds = ClampRectToBounds(
        screenshot_selection_overlay_state_.pending_hover_selection_bounds,
        screenshot_selection_overlay_state_.workspace_bounds);
    screenshot_selection_overlay_state_.has_pending_hover_selection = false;
    screenshot_selection_overlay_state_.pending_hover_selection_bounds = {0, 0, 0, 0};
    screenshot_selection_overlay_state_.has_hover_selection = false;
    screenshot_selection_overlay_state_.hover_selection_bounds = {0, 0, 0, 0};
    screenshot_selection_overlay_state_.hover_selection_source.clear();
    screenshot_selection_overlay_state_.hover_selection_root_window = nullptr;
    screenshot_selection_overlay_state_.hover_candidate_bounds.clear();
    screenshot_selection_overlay_state_.hover_candidate_root_window = nullptr;
    CompleteScreenshotSelectionOverlay(false);
    return;
  }

  screenshot_selection_overlay_state_.has_pending_hover_selection = false;
  screenshot_selection_overlay_state_.pending_hover_selection_bounds = {0, 0, 0, 0};
  UpdateScreenshotSelectionOverlay(manual_selection);
  CompleteScreenshotSelectionOverlay(false);
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
  CancelScreenshotHoverProbeTimer();
  screenshot_selection_overlay_state_.has_pending_hover_move = false;
  screenshot_selection_overlay_state_.hover_move_message_posted = false;
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
    screenshot_selection_overlay_state_.has_pending_hover_selection = false;
    screenshot_selection_overlay_state_.pending_hover_selection_bounds = {0, 0, 0, 0};
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
  CancelScreenshotHoverProbeTimer();

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

RECT FlutterWindow::DisplayBoundsForPoint(POINT point) const
{
  const CachedDisplayCapture *display_capture = DisplayCaptureForPoint(point);
  if (display_capture != nullptr)
  {
    return display_capture->monitor_bounds;
  }

  MONITORINFO monitor_info{};
  monitor_info.cbSize = sizeof(monitor_info);
  HMONITOR monitor = MonitorFromPoint(point, MONITOR_DEFAULTTONEAREST);
  if (monitor != nullptr && GetMonitorInfoW(monitor, &monitor_info))
  {
    return monitor_info.rcMonitor;
  }

  return screenshot_selection_overlay_state_.workspace_bounds;
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

void FlutterWindow::PaintScreenshotSelectionOverlay(HWND hwnd)
{
  PAINTSTRUCT paint{};
  HDC hdc = BeginPaint(hwnd, &paint);
  if (hdc == nullptr)
  {
    return;
  }

  const RECT workspace = screenshot_selection_overlay_state_.workspace_bounds;
  RECT client_rect{};
  GetClientRect(hwnd, &client_rect);
  RECT dirty_client_rect = paint.rcPaint;
  if (IsRectEmptyOrInvalid(dirty_client_rect))
  {
    dirty_client_rect = client_rect;
  }

  const int dirty_width = dirty_client_rect.right - dirty_client_rect.left;
  const int dirty_height = dirty_client_rect.bottom - dirty_client_rect.top;
  if (dirty_width <= 0 || dirty_height <= 0)
  {
    EndPaint(hwnd, &paint);
    return;
  }

  const RECT dirty_workspace_rect{
      dirty_client_rect.left + workspace.left,
      dirty_client_rect.top + workspace.top,
      dirty_client_rect.right + workspace.left,
      dirty_client_rect.bottom + workspace.top};

  HDC paint_dc = hdc;
  HDC buffer_dc = CreateCompatibleDC(hdc);
  HBITMAP buffer_bitmap = nullptr;
  HGDIOBJ old_buffer_bitmap = nullptr;
  if (buffer_dc != nullptr)
  {
    buffer_bitmap = CreateCompatibleBitmap(hdc, dirty_width, dirty_height);
    if (buffer_bitmap != nullptr)
    {
      old_buffer_bitmap = SelectObject(buffer_dc, buffer_bitmap);
      if (old_buffer_bitmap != nullptr)
      {
        paint_dc = buffer_dc;
      }
      else
      {
        DeleteObject(buffer_bitmap);
        DeleteDC(buffer_dc);
        buffer_bitmap = nullptr;
        buffer_dc = nullptr;
      }
    }
    else
    {
      DeleteDC(buffer_dc);
      buffer_dc = nullptr;
    }
  }

  // Bug fix: painting the full-screen snapshot and dim mask directly into the visible HWND made
  // startup show a top-to-bottom shade sweep on large desktops. Compose the whole selector frame in
  // memory first, then present it with one BitBlt so the initial cover appears as a single frame.
  const bool uses_buffer = paint_dc == buffer_dc && buffer_dc != nullptr && buffer_bitmap != nullptr;
  const LONG surface_workspace_left = uses_buffer ? dirty_workspace_rect.left : workspace.left;
  const LONG surface_workspace_top = uses_buffer ? dirty_workspace_rect.top : workspace.top;
  RECT surface_rect = uses_buffer ? RECT{0, 0, dirty_width, dirty_height} : dirty_client_rect;
  FillRect(paint_dc, &surface_rect, reinterpret_cast<HBRUSH>(GetStockObject(BLACK_BRUSH)));

  auto drawSnapshotArea = [&](const RECT &area_workspace_rect) {
    for (const auto &capture : cached_display_captures_)
    {
      if (capture.bitmap == nullptr)
      {
        continue;
      }

      RECT draw_rect{};
      if (!TryIntersectRects(capture.monitor_bounds, area_workspace_rect, &draw_rect))
      {
        continue;
      }

      HDC memory_dc = CreateCompatibleDC(hdc);
      if (memory_dc == nullptr)
      {
        continue;
      }

      HGDIOBJ old_bitmap = SelectObject(memory_dc, capture.bitmap);
      BitBlt(
          paint_dc,
          draw_rect.left - surface_workspace_left,
          draw_rect.top - surface_workspace_top,
          draw_rect.right - draw_rect.left,
          draw_rect.bottom - draw_rect.top,
          memory_dc,
          draw_rect.left - capture.monitor_bounds.left,
          draw_rect.top - capture.monitor_bounds.top,
          SRCCOPY);
      SelectObject(memory_dc, old_bitmap);
      DeleteDC(memory_dc);
    }
  };

  drawSnapshotArea(dirty_workspace_rect);

  EnsureGdiplusInitialized();
  RECT undim_rect{};
  bool has_undim_rect = false;
  {
    Gdiplus::Graphics graphics(paint_dc);
    Gdiplus::SolidBrush dim_brush(Gdiplus::Color(118, 0, 0, 0));
    const Gdiplus::Rect dim_rect(surface_rect.left, surface_rect.top, surface_rect.right - surface_rect.left, surface_rect.bottom - surface_rect.top);
    graphics.FillRectangle(&dim_brush, dim_rect);
    const RECT committed_selection = ClampRectToBounds(screenshot_selection_overlay_state_.selection_bounds, workspace);
    const RECT hover_selection = ClampRectToBounds(screenshot_selection_overlay_state_.hover_selection_bounds, workspace);
    const RECT selection =
        !IsRectEmptyOrInvalid(committed_selection)
            ? committed_selection
            : (screenshot_selection_overlay_state_.has_hover_selection ? hover_selection : RECT{0, 0, 0, 0});
    if (!IsRectEmptyOrInvalid(selection))
    {
      if (TryIntersectRects(selection, dirty_workspace_rect, &undim_rect))
      {
        // Performance fix: after dimming the dirty tile, redraw only the selected intersection from
        // the cached bitmap. This preserves the frozen-background cut-out without repainting the
        // full desktop on every drag move.
        has_undim_rect = true;
      }
    }
  }
  if (has_undim_rect)
  {
    drawSnapshotArea(undim_rect);
  }

  if (buffer_dc != nullptr && buffer_bitmap != nullptr)
  {
    BitBlt(hdc, dirty_client_rect.left, dirty_client_rect.top, dirty_width, dirty_height, buffer_dc, 0, 0, SRCCOPY);
    SelectObject(buffer_dc, old_buffer_bitmap);
    DeleteObject(buffer_bitmap);
    DeleteDC(buffer_dc);
  }

  EndPaint(hwnd, &paint);
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

void FlutterWindow::EmitScrollingCaptureWheelEvent(int wheel_delta)
{
  if (window_manager_channel_)
  {
    const double delta_y = -static_cast<double>(wheel_delta) / static_cast<double>(WHEEL_DELTA);
    flutter::EncodableMap payload;
    // Bug fix: Dart needs the native wheel direction to choose append vs prepend stitching. Windows
    // reports positive wheel deltas for upward scrolls, so normalize to Flutter's convention where
    // positive deltaY means scrolling down through the page.
    payload[flutter::EncodableValue("deltaY")] = flutter::EncodableValue(delta_y);
    payload[flutter::EncodableValue("rawDeltaY")] = flutter::EncodableValue(static_cast<double>(wheel_delta));
    window_manager_channel_->InvokeMethod("onScrollingCaptureWheel", std::make_unique<flutter::EncodableValue>(payload));
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
    EmitScrollingCaptureWheelEvent(static_cast<int>(lparam));
    return 0;
  }
  if (message == kScreenshotDisplaySnapshotPayloadReadyMessage)
  {
    auto payload_result = std::unique_ptr<DisplaySnapshotPayloadAsyncResult>(reinterpret_cast<DisplaySnapshotPayloadAsyncResult *>(lparam));
    CompleteDisplaySnapshotPayloadAsyncResult(payload_result.get());
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
    g_window_instance->PaintScreenshotSelectionOverlay(hwnd);
    return 0;
  case WM_SETCURSOR:
    SetCursor(LoadCursor(nullptr, IDC_CROSS));
    return TRUE;
  case kScreenshotSelectionDimRegionUpdateMessage:
    g_window_instance->FlushScreenshotSelectionDimRegionUpdate();
    return 0;
  case kScreenshotSelectionHoverMoveMessage:
  {
    auto &state = g_window_instance->screenshot_selection_overlay_state_;
    state.hover_move_message_posted = false;
    if (state.has_pending_hover_move && !state.dragging && !state.completed)
    {
      const POINT point = state.pending_hover_move_point;
      state.has_pending_hover_move = false;
      g_window_instance->UpdateScreenshotSelectionHover(point);
    }
    else
    {
      state.has_pending_hover_move = false;
    }
    return 0;
  }
  case WM_TIMER:
    if (wparam == kScreenshotSelectionHoverProbeTimerId)
    {
      g_window_instance->HandleScreenshotHoverProbeTimer();
      return 0;
    }
    break;
  case WM_KEYDOWN:
    if (wparam == VK_ESCAPE)
    {
      g_window_instance->CompleteScreenshotSelectionOverlay(true);
      return 0;
    }
    break;
  case WM_LBUTTONDOWN:
  {
    if (g_window_instance->screenshot_selection_overlay_state_.completed)
    {
      return 0;
    }

    POINT point{GET_X_LPARAM(lparam), GET_Y_LPARAM(lparam)};
    ClientToScreen(hwnd, &point);
    if (!g_window_instance->screenshot_selection_overlay_state_.dragging)
    {
      g_window_instance->BeginScreenshotSelectionPointerDown(point);
    }

    SetCapture(hwnd);
    SetFocus(hwnd);
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
    else if (!g_window_instance->screenshot_selection_overlay_state_.completed)
    {
      POINT point{GET_X_LPARAM(lparam), GET_Y_LPARAM(lparam)};
      ClientToScreen(hwnd, &point);
      g_window_instance->UpdateScreenshotSelectionHover(point);
      return 0;
    }
    break;
  case WM_LBUTTONUP:
    if (g_window_instance->screenshot_selection_overlay_state_.dragging)
    {
      POINT point{GET_X_LPARAM(lparam), GET_Y_LPARAM(lparam)};
      ClientToScreen(hwnd, &point);
      ReleaseCapture();
      g_window_instance->CompleteScreenshotSelectionPointerUp(point);
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
        g_window_instance->BeginScreenshotSelectionPointerDown(mouse->pt);
        break;
      case WM_MOUSEMOVE:
        if (state.dragging)
        {
          if (GetCapture() != state.input_window)
          {
            // Performance fix: after the input HWND owns capture, WM_MOUSEMOVE is the authoritative
            // drag path. The low-level hook remains as a fallback for missed capture, but skipping
            // duplicate hook moves avoids repainting the frozen screenshot dirty tile twice.
            g_window_instance->UpdateScreenshotSelectionOverlay(RectFromPoints(state.drag_start, mouse->pt));
          }
        }
        else
        {
          g_window_instance->UpdateScreenshotSelectionHoverFromHook(mouse->pt);
        }
        break;
      case WM_LBUTTONUP:
        if (state.dragging)
        {
          if (GetCapture() != state.input_window)
          {
            // Bug fix: keep mouse-up completion in the same owner as drag updates. If the input
            // HWND has capture it will finish selection through WM_LBUTTONUP; the hook only covers
            // fallback cases where native capture delivery was missed.
            g_window_instance->CompleteScreenshotSelectionPointerUp(mouse->pt);
          }
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
      const int wheel_delta = GET_WHEEL_DELTA_WPARAM(mouse->mouseData);
      // The native mask is mouse-transparent, so the wheel already scrolls the app underneath. This
      // hook mirrors macOS' global scroll monitor and forwards the wheel direction so Dart can stitch
      // newly captured content above or below the existing long screenshot.
      PostMessage(g_window_instance->GetHandle(), kScrollingCaptureWheelMessage, 0, static_cast<LPARAM>(wheel_delta));
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
      if (!BuildDisplaySnapshotPayloads(captures, ScreenshotImagePayloadMode::kBase64, &snapshots, &capture_error))
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
      if (!BuildDisplaySnapshotPayloads(cached_display_captures_, ScreenshotImagePayloadMode::kNone, &snapshots, &capture_error))
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
      std::vector<CachedDisplayCapture> async_captures;
      if (!CloneDisplayCaptures(filtered_captures, &async_captures, &capture_error))
      {
        result->Error("CAPTURE_ERROR", capture_error);
        return;
      }

      // PNG encoding still costs CPU even when the channel payload is only a file path. Clone the
      // HBITMAPs quickly, then let a worker thread encode/write files and post the MethodResult
      // back to the UI thread when the small path payload is ready.
      BuildDisplaySnapshotPayloadsAsync(std::move(async_captures), std::move(result));
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
      ClearCachedDisplayCaptures();
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
    else if (method_name == "supportsMicaBackdrop")
    {
      // Keep Dart's transparency controls aligned with the native backdrop gate used by Win32Window.
      result->Success(flutter::EncodableValue(GetWindowsBuildNumberForCapabilities() >= 22000));
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

      SetWindowPos(hwnd, nullptr, x, y, scaledWidth, scaledHeight, SWP_NOZORDER);
      result->Success();
    }
    else if (method_name == "constrainSizeToCursorDisplayWorkArea")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (!arguments)
      {
        result->Error("INVALID_ARGUMENTS", "Arguments must be provided for constrainSizeToCursorDisplayWorkArea");
        return;
      }

      auto width_it = arguments->find(flutter::EncodableValue("width"));
      auto height_it = arguments->find(flutter::EncodableValue("height"));
      auto fraction_it = arguments->find(flutter::EncodableValue("maxWorkAreaFraction"));

      if (width_it == arguments->end() || height_it == arguments->end() || fraction_it == arguments->end())
      {
        result->Error("INVALID_ARGUMENTS", "Width, height and maxWorkAreaFraction must be provided for constrainSizeToCursorDisplayWorkArea");
        return;
      }

      double width = std::get<double>(width_it->second);
      double height = std::get<double>(height_it->second);
      double max_work_area_fraction = std::get<double>(fraction_it->second);

      POINT cursor_pos;
      GetCursorPos(&cursor_pos);

      HMONITOR monitor = MonitorFromPoint(cursor_pos, MONITOR_DEFAULTTONEAREST);
      MONITORINFO monitor_info;
      monitor_info.cbSize = sizeof(MONITORINFO);

      if (!GetMonitorInfo(monitor, &monitor_info))
      {
        result->Error("MONITOR_ERROR", "Failed to get monitor info");
        return;
      }

      const UINT dpi = FlutterDesktopGetDpiForMonitor(monitor);
      const double dpi_scale = dpi <= 0 ? 1.0 : static_cast<double>(dpi) / 96.0;
      const double safe_fraction = max_work_area_fraction <= 0 ? 1.0 : std::clamp(max_work_area_fraction, 0.0, 1.0);
      const double work_area_width = static_cast<double>(monitor_info.rcWork.right - monitor_info.rcWork.left) / dpi_scale;
      const double work_area_height = static_cast<double>(monitor_info.rcWork.bottom - monitor_info.rcWork.top) / dpi_scale;
      const double max_width = work_area_width * safe_fraction;
      const double max_height = work_area_height * safe_fraction;
      const double constrained_width = width > max_width ? std::floor(max_width) : width;
      const double constrained_height = height > max_height ? std::floor(max_height) : height;

      flutter::EncodableMap response;
      response[flutter::EncodableValue("width")] = flutter::EncodableValue(constrained_width <= 0 ? width : constrained_width);
      response[flutter::EncodableValue("height")] = flutter::EncodableValue(constrained_height <= 0 ? height : constrained_height);
      result->Success(flutter::EncodableValue(response));
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
