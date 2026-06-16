#pragma once

#include <string>

namespace util {
std::string Utf8FromUtf16(std::wstring_view utf16_string);
std::wstring Utf16FromUtf8(std::string_view utf8_string);
}  // namespace util
