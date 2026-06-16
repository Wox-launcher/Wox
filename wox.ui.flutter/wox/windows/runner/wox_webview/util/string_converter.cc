#include "string_converter.h"

#include <windows.h>

namespace util {
std::string Utf8FromUtf16(std::wstring_view utf16_string) {
  if (utf16_string.empty()) {
    return std::string();
  }

  auto src_length = static_cast<int>(utf16_string.size());
  int target_length =
      ::WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, utf16_string.data(),
                            src_length, nullptr, 0, nullptr, nullptr);

  std::string utf8_string;
  if (target_length <= 0 || target_length > utf8_string.max_size()) {
    return utf8_string;
  }
  utf8_string.resize(target_length);
  int converted_length = ::WideCharToMultiByte(
      CP_UTF8, WC_ERR_INVALID_CHARS, utf16_string.data(), src_length,
      utf8_string.data(), target_length, nullptr, nullptr);
  if (converted_length == 0) {
    return std::string();
  }
  return utf8_string;
}

std::wstring Utf16FromUtf8(std::string_view utf8_string) {
  if (utf8_string.empty()) {
    return std::wstring();
  }

  auto src_length = static_cast<int>(utf8_string.size());
  int target_length =
      ::MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, utf8_string.data(),
                            src_length, nullptr, 0);

  std::wstring utf16_string;
  if (target_length <= 0 || target_length > utf16_string.max_size()) {
    return utf16_string;
  }
  utf16_string.resize(target_length);
  int converted_length =
      ::MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, utf8_string.data(),
                            src_length, utf16_string.data(), target_length);
  if (converted_length == 0) {
    return std::wstring();
  }
  return utf16_string;
}

}  // namespace util
