#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <roapi.h>
#include <shcore.h>
#include <winstring.h>
// MinGW's Foundation header duplicates the BYTE and boolean specializations.
#ifdef __MINGW32__
#define ____FIReference_1_boolean_INTERFACE_DEFINED__
#endif
#include <windows.globalization.h>
#include <windows.graphics.imaging.h>
#include <windows.storage.h>
#include <windows.system.userprofile.h>
#include <wrl/client.h>
#include <wrl/wrappers/corewrappers.h>

#include <algorithm>
#include <cmath>
#include <cstdint>
#include <new>
#include <string>
#include <vector>

namespace wox::ocr {

namespace foundation = ABI::Windows::Foundation;
namespace globalization = ABI::Windows::Globalization;
namespace imaging = ABI::Windows::Graphics::Imaging;
namespace storage = ABI::Windows::Storage;
namespace userProfile = ABI::Windows::System::UserProfile;
using Microsoft::WRL::ComPtr;
using Microsoft::WRL::Wrappers::HString;
using foundation::Collections::IVectorView;
using foundation::IAsyncOperation;

// These WinRT interfaces are missing from MinGW's headers. The vtable prefixes
// and IIDs follow the Windows SDK. Keep external linkage because the concrete
// implementations live in the system WinRT DLLs.
struct BitmapDecoderStatics : IInspectable {
  virtual HRESULT STDMETHODCALLTYPE get_BmpDecoderId(GUID *) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_JpegDecoderId(GUID *) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_PngDecoderId(GUID *) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_TiffDecoderId(GUID *) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_GifDecoderId(GUID *) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_JpegXRDecoderId(GUID *) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_IcoDecoderId(GUID *) = 0;
  virtual HRESULT STDMETHODCALLTYPE GetDecoderInformationEnumerator(IInspectable **) = 0;
  virtual HRESULT STDMETHODCALLTYPE CreateAsync(storage::Streams::IRandomAccessStream *, IAsyncOperation<IInspectable *> **) = 0;
};
const IID bitmapDecoderStaticsID = {0x438ccb26, 0xbcef, 0x4e95, {0xba, 0xd6, 0x23, 0xa8, 0x22, 0xe5, 0x8d, 0x01}};

struct BitmapFrame : IInspectable {
  virtual HRESULT STDMETHODCALLTYPE GetThumbnailAsync(IInspectable **) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_BitmapProperties(IInspectable **) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_BitmapPixelFormat(imaging::BitmapPixelFormat *) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_BitmapAlphaMode(imaging::BitmapAlphaMode *) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_DpiX(double *) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_DpiY(double *) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_PixelWidth(UINT32 *) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_PixelHeight(UINT32 *) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_OrientedPixelWidth(UINT32 *) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_OrientedPixelHeight(UINT32 *) = 0;
};
const IID bitmapFrameID = {0x72a49a1c, 0x8081, 0x438d, {0x91, 0xbc, 0x94, 0xec, 0xfc, 0x81, 0x85, 0xc6}};

struct BitmapTransform : IInspectable {
  virtual HRESULT STDMETHODCALLTYPE get_ScaledWidth(UINT32 *) = 0;
  virtual HRESULT STDMETHODCALLTYPE put_ScaledWidth(UINT32) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_ScaledHeight(UINT32 *) = 0;
  virtual HRESULT STDMETHODCALLTYPE put_ScaledHeight(UINT32) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_InterpolationMode(UINT32 *) = 0;
  virtual HRESULT STDMETHODCALLTYPE put_InterpolationMode(UINT32) = 0;
};
const IID bitmapTransformID = {0xae755344, 0xe268, 0x4d35, {0xad, 0xcf, 0xe9, 0x95, 0xd3, 0x1a, 0x8d, 0x34}};

constexpr UINT32 bitmapInterpolationFant = 3;
constexpr UINT32 exifOrientationRespect = 1;
constexpr UINT32 colorManagementDoNotManage = 0;

struct BitmapFrameWithSoftwareBitmap : IInspectable {
  virtual HRESULT STDMETHODCALLTYPE GetSoftwareBitmapAsync(IAsyncOperation<imaging::ISoftwareBitmap *> **) = 0;
  virtual HRESULT STDMETHODCALLTYPE GetSoftwareBitmapConvertedAsync(
      imaging::BitmapPixelFormat, imaging::BitmapAlphaMode, IAsyncOperation<imaging::ISoftwareBitmap *> **) = 0;
  virtual HRESULT STDMETHODCALLTYPE GetSoftwareBitmapTransformedAsync(
      imaging::BitmapPixelFormat, imaging::BitmapAlphaMode, IInspectable *, UINT32, UINT32,
      IAsyncOperation<imaging::ISoftwareBitmap *> **) = 0;
};
const IID bitmapFrameWithSoftwareBitmapID = {0xfe287c9a, 0x420c, 0x4963, {0x87, 0xad, 0x69, 0x14, 0x36, 0xe0, 0x83, 0x83}};

struct OcrResult : IInspectable {
  virtual HRESULT STDMETHODCALLTYPE get_Lines(IInspectable **) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_TextAngle(IInspectable **) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_Text(HSTRING *) = 0;
};

struct OcrEngine : IInspectable {
  virtual HRESULT STDMETHODCALLTYPE RecognizeAsync(imaging::ISoftwareBitmap *, IAsyncOperation<OcrResult *> **) = 0;
};

using LanguageList = IVectorView<globalization::Language *>;
struct OcrEngineStatics : IInspectable {
  virtual HRESULT STDMETHODCALLTYPE get_MaxImageDimension(UINT32 *) = 0;
  virtual HRESULT STDMETHODCALLTYPE get_AvailableRecognizerLanguages(LanguageList **) = 0;
  virtual HRESULT STDMETHODCALLTYPE IsLanguageSupported(globalization::ILanguage *, boolean *) = 0;
  virtual HRESULT STDMETHODCALLTYPE TryCreateFromLanguage(globalization::ILanguage *, OcrEngine **) = 0;
  virtual HRESULT STDMETHODCALLTYPE TryCreateFromUserProfileLanguages(OcrEngine **) = 0;
};
const IID ocrEngineStaticsID = {0x5bffa85a, 0x3384, 0x3540, {0x99, 0x40, 0x69, 0x91, 0x20, 0xd4, 0x28, 0xa8}};

namespace {

void check(HRESULT status) {
  if (FAILED(status)) {
    throw status;
  }
}

struct WinRTApartment {
  WinRTApartment() { check(RoInitialize(RO_INIT_MULTITHREADED)); }
  ~WinRTApartment() { RoUninitialize(); }
};

template <typename Interface>
ComPtr<Interface> activationFactory(PCWSTR className, REFIID id = __uuidof(Interface)) {
  HString name;
  check(name.Set(className));
  ComPtr<Interface> factory;
  check(RoGetActivationFactory(name.Get(), id, reinterpret_cast<void **>(factory.GetAddressOf())));
  return factory;
}

// Keep the async object on this apartment and let Go interrupt any outstanding wait.
template <typename Object>
ComPtr<Object> await(IAsyncOperation<Object *> *operation, HANDLE cancelEvent) {
  ComPtr<IAsyncInfo> info;
  check(operation->QueryInterface(IID_PPV_ARGS(info.GetAddressOf())));
  AsyncStatus status;
  check(info->get_Status(&status));
  while (status == Started) {
    DWORD wait = WaitForSingleObject(cancelEvent, 10);
    if (wait == WAIT_OBJECT_0) {
      info->Cancel();
      throw E_ABORT;
    }
    if (wait == WAIT_FAILED) {
      throw HRESULT_FROM_WIN32(GetLastError());
    }
    check(info->get_Status(&status));
  }
  ComPtr<Object> result;
  check(operation->GetResults(result.GetAddressOf()));
  return result;
}

void closeWinRT(IUnknown *object) {
  if (!object) {
    return;
  }
  ComPtr<foundation::IClosable> closable;
  if (SUCCEEDED(object->QueryInterface(IID_PPV_ARGS(closable.GetAddressOf())))) {
    closable->Close();
  }
}

template <typename T>
struct ClosableGuard {
  ComPtr<T> &ref;
  ~ClosableGuard() { closeWinRT(ref.Get()); }
};

// Cap OCR raster size (~2.6 MP) so high-DPI captures do not decode huge WIC bitmaps in-process.
constexpr uint64_t kMaxOCRBitmapPixels = 2600000;

ComPtr<BitmapTransform> createScaledTransform(UINT32 width, UINT32 height) {
  HString className;
  check(className.Set(L"Windows.Graphics.Imaging.BitmapTransform"));
  ComPtr<IInspectable> instance;
  check(RoActivateInstance(className.Get(), instance.GetAddressOf()));
  ComPtr<BitmapTransform> transform;
  check(instance->QueryInterface(bitmapTransformID, reinterpret_cast<void **>(transform.GetAddressOf())));
  check(transform->put_ScaledWidth(width));
  check(transform->put_ScaledHeight(height));
  check(transform->put_InterpolationMode(bitmapInterpolationFant));
  return transform;
}

// Use the same WinRT decoder for clipboard images and screenshot files, converting to Gray8
// and downscaling oversized rasters during WIC decode to keep OCR native memory bounded.
ComPtr<imaging::ISoftwareBitmap> decodeBitmap(PCWSTR path, UINT32 maxDimension, HANDLE cancelEvent) {
  ComPtr<storage::Streams::IRandomAccessStream> stream;
  ClosableGuard<storage::Streams::IRandomAccessStream> streamGuard{stream};
  check(CreateRandomAccessStreamOnFile(path, storage::FileAccessMode_Read, IID_PPV_ARGS(stream.GetAddressOf())));
  auto factory = activationFactory<BitmapDecoderStatics>(L"Windows.Graphics.Imaging.BitmapDecoder", bitmapDecoderStaticsID);
  ComPtr<IAsyncOperation<IInspectable *>> decode;
  check(factory->CreateAsync(stream.Get(), decode.GetAddressOf()));
  auto decoder = await(decode.Get(), cancelEvent);

  UINT32 width = 0;
  UINT32 height = 0;
  ComPtr<BitmapFrame> frameInfo;
  if (SUCCEEDED(decoder->QueryInterface(bitmapFrameID, reinterpret_cast<void **>(frameInfo.GetAddressOf())))) {
    if (FAILED(frameInfo->get_PixelWidth(&width)) || FAILED(frameInfo->get_PixelHeight(&height))) {
      width = 0;
      height = 0;
    }
  }

  UINT32 scaledWidth = width;
  UINT32 scaledHeight = height;
  const uint64_t pixels = static_cast<uint64_t>(width) * static_cast<uint64_t>(height);
  if (width > 0 && height > 0 &&
      (pixels > kMaxOCRBitmapPixels || (maxDimension > 0 && (width > maxDimension || height > maxDimension)))) {
    double scale = 1.0;
    if (pixels > kMaxOCRBitmapPixels) {
      scale = std::min(scale, std::sqrt(static_cast<double>(kMaxOCRBitmapPixels) / static_cast<double>(pixels)));
    }
    if (maxDimension > 0 && width > maxDimension) {
      scale = std::min(scale, static_cast<double>(maxDimension) / static_cast<double>(width));
    }
    if (maxDimension > 0 && height > maxDimension) {
      scale = std::min(scale, static_cast<double>(maxDimension) / static_cast<double>(height));
    }
    scaledWidth = std::max<UINT32>(1, static_cast<UINT32>(std::lround(static_cast<double>(width) * scale)));
    scaledHeight = std::max<UINT32>(1, static_cast<UINT32>(std::lround(static_cast<double>(height) * scale)));
    if (maxDimension > 0) {
      scaledWidth = std::min(scaledWidth, maxDimension);
      scaledHeight = std::min(scaledHeight, maxDimension);
    }
  }

  ComPtr<BitmapFrameWithSoftwareBitmap> frame;
  check(decoder->QueryInterface(bitmapFrameWithSoftwareBitmapID, reinterpret_cast<void **>(frame.GetAddressOf())));

  if (scaledWidth < width || scaledHeight < height) {
    auto transform = createScaledTransform(scaledWidth, scaledHeight);
    try {
      ComPtr<IAsyncOperation<imaging::ISoftwareBitmap *>> bitmap;
      check(frame->GetSoftwareBitmapTransformedAsync(
          imaging::BitmapPixelFormat_Gray8, imaging::BitmapAlphaMode_Ignore,
          transform.Get(), exifOrientationRespect, colorManagementDoNotManage, bitmap.GetAddressOf()));
      return await(bitmap.Get(), cancelEvent);
    } catch (HRESULT) {
      if (WaitForSingleObject(cancelEvent, 0) == WAIT_OBJECT_0) {
        throw E_ABORT;
      }
      ComPtr<IAsyncOperation<imaging::ISoftwareBitmap *>> bitmap;
      check(frame->GetSoftwareBitmapTransformedAsync(
          imaging::BitmapPixelFormat_Bgra8, imaging::BitmapAlphaMode_Premultiplied,
          transform.Get(), exifOrientationRespect, colorManagementDoNotManage, bitmap.GetAddressOf()));
      return await(bitmap.Get(), cancelEvent);
    }
  }

  try {
    ComPtr<IAsyncOperation<imaging::ISoftwareBitmap *>> bitmap;
    check(frame->GetSoftwareBitmapConvertedAsync(
        imaging::BitmapPixelFormat_Gray8, imaging::BitmapAlphaMode_Ignore, bitmap.GetAddressOf()));
    return await(bitmap.Get(), cancelEvent);
  } catch (HRESULT) {
    if (WaitForSingleObject(cancelEvent, 0) == WAIT_OBJECT_0) {
      throw E_ABORT;
    }
    ComPtr<IAsyncOperation<imaging::ISoftwareBitmap *>> bitmap;
    check(frame->GetSoftwareBitmapAsync(bitmap.GetAddressOf()));
    return await(bitmap.Get(), cancelEvent);
  }
}

std::wstring readString(HSTRING value) {
  UINT32 length = 0;
  PCWSTR buffer = WindowsGetStringRawBuffer(value, &length);
  return std::wstring(buffer, length);
}

std::vector<std::wstring> installedLanguages(OcrEngineStatics *factory) {
  ComPtr<LanguageList> languages;
  check(factory->get_AvailableRecognizerLanguages(languages.GetAddressOf()));
  UINT32 count = 0;
  check(languages->get_Size(&count));
  std::vector<std::wstring> tags;
  for (UINT32 index = 0; index < count; ++index) {
    ComPtr<globalization::ILanguage> language;
    check(languages->GetAt(index, language.GetAddressOf()));
    HString tag;
    check(language->get_LanguageTag(tag.GetAddressOf()));
    tags.push_back(readString(tag.Get()));
  }
  return tags;
}

std::vector<std::wstring> profileLanguages() {
  auto profile = activationFactory<userProfile::IGlobalizationPreferencesStatics>(L"Windows.System.UserProfile.GlobalizationPreferences");
  ComPtr<IVectorView<HSTRING>> languages;
  check(profile->get_Languages(languages.GetAddressOf()));
  UINT32 count = 0;
  check(languages->get_Size(&count));
  std::vector<std::wstring> tags;
  for (UINT32 index = 0; index < count; ++index) {
    HString tag;
    check(languages->GetAt(index, tag.GetAddressOf()));
    tags.push_back(readString(tag.Get()));
  }
  return tags;
}

std::wstring recognize(OcrEngine *engine, imaging::ISoftwareBitmap *bitmap, HANDLE cancelEvent) {
  ComPtr<IAsyncOperation<OcrResult *>> operation;
  check(engine->RecognizeAsync(bitmap, operation.GetAddressOf()));
  auto result = await(operation.Get(), cancelEvent);
  HString text;
  check(result->get_Text(text.GetAddressOf()));
  return readString(text.Get());
}

// Skip unavailable or invalid language candidates and collect text from the rest.
std::wstring recognizeLanguages(OcrEngineStatics *factory, imaging::ISoftwareBitmap *bitmap, HANDLE cancelEvent,
                                const std::vector<std::wstring> &tags) {
  auto languageFactory = activationFactory<globalization::ILanguageFactory>(L"Windows.Globalization.Language");
  std::wstring text;
  for (const auto &tag : tags) {
    if (WaitForSingleObject(cancelEvent, 0) == WAIT_OBJECT_0) {
      throw E_ABORT;
    }
    try {
      HString name;
      check(name.Set(tag.c_str()));
      ComPtr<globalization::ILanguage> language;
      check(languageFactory->CreateLanguage(name.Get(), language.GetAddressOf()));
      ComPtr<OcrEngine> engine;
      check(factory->TryCreateFromLanguage(language.Get(), engine.GetAddressOf()));
      if (!engine) {
        continue;
      }
      auto recognized = recognize(engine.Get(), bitmap, cancelEvent);
      if (!recognized.empty()) {
        if (!text.empty()) {
          text += L'\n';
        }
        text += recognized;
      }
    } catch (HRESULT status) {
      if (WaitForSingleObject(cancelEvent, 0) == WAIT_OBJECT_0) {
        throw status;
      }
    }
  }
  return text;
}

} // namespace
} // namespace wox::ocr

extern "C" HRESULT woxOCRRecognizeMedia(PCWSTR path, PCWSTR languages, uintptr_t event, HSTRING *result) {
  using namespace wox::ocr;
  try {
    WinRTApartment apartment;
    HANDLE cancelEvent = reinterpret_cast<HANDLE>(event);
    auto factory = activationFactory<OcrEngineStatics>(L"Windows.Media.Ocr.OcrEngine", ocrEngineStaticsID);
    UINT32 maxDimension = 0;
    if (FAILED(factory->get_MaxImageDimension(&maxDimension))) {
      maxDimension = 0;
    }
    auto bitmap = decodeBitmap(path, maxDimension, cancelEvent);
    ClosableGuard<imaging::ISoftwareBitmap> bitmapGuard{bitmap};

    // Try explicit languages or installed non-English recognizers first. Mixing
    // in English OCR can add misread text when a CJK recognizer already succeeded.
    const auto installed = installedLanguages(factory.Get());
    std::vector<std::wstring> primary;
    if (*languages) {
      const wchar_t *separator;
      while ((separator = wcschr(languages, L'\n')) != nullptr) {
        primary.emplace_back(languages, separator);
        languages = separator + 1;
      }
      primary.emplace_back(languages);
    } else {
      for (const auto &tag : installed) {
        if (_wcsicmp(tag.c_str(), L"en") != 0 && _wcsnicmp(tag.c_str(), L"en-", 3) != 0) {
          primary.push_back(tag);
        }
      }
    }
    auto text = recognizeLanguages(factory.Get(), bitmap.Get(), cancelEvent, primary);
    if (text.empty()) {
      auto fallback = profileLanguages();
      for (const auto &tag : installed) {
        auto duplicate = std::find_if(fallback.begin(), fallback.end(), [&](const auto &candidate) {
          return _wcsicmp(candidate.c_str(), tag.c_str()) == 0;
        });
        if (duplicate == fallback.end()) {
          fallback.push_back(tag);
        }
      }
      text = recognizeLanguages(factory.Get(), bitmap.Get(), cancelEvent, fallback);
    }
    if (text.empty()) {
      ComPtr<OcrEngine> engine;
      check(factory->TryCreateFromUserProfileLanguages(engine.GetAddressOf()));
      if (!engine) {
        return HRESULT_FROM_WIN32(ERROR_NOT_SUPPORTED);
      }
      text = recognize(engine.Get(), bitmap.Get(), cancelEvent);
    }
    return WindowsCreateString(text.data(), static_cast<UINT32>(text.size()), result);
  } catch (HRESULT status) {
    return status;
  } catch (const std::bad_alloc &) {
    return E_OUTOFMEMORY;
  }
}
