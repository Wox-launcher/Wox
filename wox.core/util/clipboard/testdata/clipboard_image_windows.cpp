#include "../clipboard_image_windows.cpp"
#include <iostream>
#include <vector>
#include <fcntl.h>
#include <io.h>

// Inspect prepared formats without opening or changing the user's clipboard.
int main(int argc, char **argv) {
  _setmode(_fileno(stdin), _O_BINARY);
  _setmode(_fileno(stdout), _O_BINARY);
  constexpr uint32_t width = 256, height = 3, stride = (width+5)*4;
  std::vector<uint8_t> pixels((height-1)*stride+width*4);
  if (!std::cin.read(reinterpret_cast<char *>(pixels.data()), pixels.size())) return 1;
  const uint8_t png[] = {137, 80, 78, 71, 13, 10, 26, 10};
  WoxWindowsClipboardImage *image = nullptr;
  if (wox_windows_prepare_clipboard_image(nullptr, width, height, stride, 1, png, sizeof(png), &image) != E_INVALIDARG || image) return 2;
  if (wox_windows_prepare_clipboard_image(pixels.data(), width, height, width*4-1, 1, png, sizeof(png), &image) != E_INVALIDARG || image) return 3;
  if (wox_windows_prepare_clipboard_image(pixels.data(), width, height, stride, 1, nullptr, sizeof(png), &image) != E_INVALIDARG || image) return 4;
  if (wox_windows_prepare_clipboard_image(pixels.data(), width, height, stride, argc > 1 ? 0 : 1, png, sizeof(png), &image) != S_OK || !image) return 5;
  auto *header = static_cast<BITMAPINFOHEADER *>(GlobalLock(image->dib));
  if (!header || header->biSize != sizeof(BITMAPINFOHEADER) || header->biWidth != width || header->biHeight != height ||
      header->biPlanes != 1 || header->biBitCount != 32 || header->biCompression != BI_RGB || header->biSizeImage != width*height*4) return 6;
  auto *bytes = reinterpret_cast<uint8_t *>(header);
  for (size_t i = sizeof(BITMAPINFOHEADER)+width*height*4; i < GlobalSize(image->dib); ++i) {
    if (bytes[i] != 0) return 7;
  }
  std::cout.write(reinterpret_cast<char *>(header+1), width*height*4);
  GlobalUnlock(image->dib);
  auto *encoded = static_cast<uint8_t *>(GlobalLock(image->png));
  if (!encoded || std::memcmp(encoded, png, sizeof(png)) != 0) return 8;
  for (size_t i = sizeof(png); i < GlobalSize(image->png); ++i) {
    if (encoded[i] != 0) return 9;
  }
  GlobalUnlock(image->png);
  wox_windows_destroy_clipboard_image(image);
  return 0;
}
