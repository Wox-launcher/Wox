#include "result_drag_bridge.h"

#include <flutter/method_channel.h>
#include <flutter/standard_method_codec.h>
#include <objidl.h>
#include <shlobj.h>

#include <algorithm>
#include <atomic>
#include <cstring>
#include <memory>
#include <string>
#include <variant>
#include <vector>

namespace
{
std::unique_ptr<flutter::MethodChannel<flutter::EncodableValue>> g_result_drag_channel;
HWND g_owner_window = nullptr;

std::wstring Utf16FromUtf8(const std::string &value)
{
  if (value.empty())
  {
    return std::wstring();
  }
  int length = ::MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, value.data(), static_cast<int>(value.size()), nullptr, 0);
  if (length <= 0)
  {
    return std::wstring();
  }
  std::wstring result(length, L'\0');
  if (::MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, value.data(), static_cast<int>(value.size()), result.data(), length) == 0)
  {
    return std::wstring();
  }
  return result;
}

flutter::EncodableValue StatusResult(const std::string &status)
{
  return flutter::EncodableValue(flutter::EncodableMap{
      {flutter::EncodableValue("status"), flutter::EncodableValue(status)},
  });
}

HGLOBAL CreateHDropGlobal(const std::vector<std::wstring> &files)
{
  size_t path_char_count = 1;
  for (const auto &file : files)
  {
    path_char_count += file.size() + 1;
  }

  const SIZE_T data_size = sizeof(DROPFILES) + path_char_count * sizeof(wchar_t);
  HGLOBAL memory = ::GlobalAlloc(GMEM_MOVEABLE | GMEM_ZEROINIT, data_size);
  if (memory == nullptr)
  {
    return nullptr;
  }

  auto *drop_files = static_cast<DROPFILES *>(::GlobalLock(memory));
  if (drop_files == nullptr)
  {
    ::GlobalFree(memory);
    return nullptr;
  }

  drop_files->pFiles = sizeof(DROPFILES);
  drop_files->fWide = TRUE;

  auto *cursor = reinterpret_cast<wchar_t *>(reinterpret_cast<BYTE *>(drop_files) + sizeof(DROPFILES));
  for (const auto &file : files)
  {
    std::copy(file.begin(), file.end(), cursor);
    cursor += file.size();
    *cursor++ = L'\0';
  }
  *cursor = L'\0';

  ::GlobalUnlock(memory);
  return memory;
}

HGLOBAL DuplicateGlobalMemory(HGLOBAL source)
{
  const SIZE_T size = ::GlobalSize(source);
  if (size == 0)
  {
    return nullptr;
  }

  HGLOBAL target = ::GlobalAlloc(GMEM_MOVEABLE, size);
  if (target == nullptr)
  {
    return nullptr;
  }

  void *source_ptr = ::GlobalLock(source);
  void *target_ptr = ::GlobalLock(target);
  if (source_ptr == nullptr || target_ptr == nullptr)
  {
    if (source_ptr != nullptr)
    {
      ::GlobalUnlock(source);
    }
    if (target_ptr != nullptr)
    {
      ::GlobalUnlock(target);
    }
    ::GlobalFree(target);
    return nullptr;
  }

  std::memcpy(target_ptr, source_ptr, size);
  ::GlobalUnlock(source);
  ::GlobalUnlock(target);
  return target;
}

bool IsFileFormat(const FORMATETC *format)
{
  return format != nullptr &&
         format->cfFormat == CF_HDROP &&
         (format->tymed & TYMED_HGLOBAL) != 0 &&
         format->dwAspect == DVASPECT_CONTENT;
}

class FormatEtcEnumerator : public IEnumFORMATETC
{
public:
  explicit FormatEtcEnumerator(const FORMATETC &format) : ref_count_(1), format_(format) {}

  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID riid, void **object) override
  {
    if (object == nullptr)
    {
      return E_POINTER;
    }
    if (riid == IID_IUnknown || riid == IID_IEnumFORMATETC)
    {
      *object = static_cast<IEnumFORMATETC *>(this);
      AddRef();
      return S_OK;
    }
    *object = nullptr;
    return E_NOINTERFACE;
  }

  ULONG STDMETHODCALLTYPE AddRef() override
  {
    return ++ref_count_;
  }

  ULONG STDMETHODCALLTYPE Release() override
  {
    ULONG count = --ref_count_;
    if (count == 0)
    {
      delete this;
    }
    return count;
  }

  HRESULT STDMETHODCALLTYPE Next(ULONG count, FORMATETC *formats, ULONG *fetched) override
  {
    if (formats == nullptr)
    {
      return E_POINTER;
    }
    if (fetched != nullptr)
    {
      *fetched = 0;
    }
    if (index_ > 0 || count == 0)
    {
      return S_FALSE;
    }

    formats[0] = format_;
    if (fetched != nullptr)
    {
      *fetched = 1;
    }
    index_ = 1;
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE Skip(ULONG count) override
  {
    if (count == 0 || index_ > 0)
    {
      return S_FALSE;
    }
    index_ = 1;
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE Reset() override
  {
    index_ = 0;
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE Clone(IEnumFORMATETC **enum_format) override
  {
    if (enum_format == nullptr)
    {
      return E_POINTER;
    }
    auto *clone = new FormatEtcEnumerator(format_);
    clone->index_ = index_;
    *enum_format = clone;
    return S_OK;
  }

private:
  std::atomic<ULONG> ref_count_;
  FORMATETC format_;
  ULONG index_ = 0;
};

class FileDataObject : public IDataObject
{
public:
  explicit FileDataObject(HGLOBAL hdrop) : ref_count_(1), hdrop_(hdrop)
  {
    format_.cfFormat = CF_HDROP;
    format_.ptd = nullptr;
    format_.dwAspect = DVASPECT_CONTENT;
    format_.lindex = -1;
    format_.tymed = TYMED_HGLOBAL;
  }

  ~FileDataObject()
  {
    if (hdrop_ != nullptr)
    {
      ::GlobalFree(hdrop_);
    }
  }

  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID riid, void **object) override
  {
    if (object == nullptr)
    {
      return E_POINTER;
    }
    if (riid == IID_IUnknown || riid == IID_IDataObject)
    {
      *object = static_cast<IDataObject *>(this);
      AddRef();
      return S_OK;
    }
    *object = nullptr;
    return E_NOINTERFACE;
  }

  ULONG STDMETHODCALLTYPE AddRef() override
  {
    return ++ref_count_;
  }

  ULONG STDMETHODCALLTYPE Release() override
  {
    ULONG count = --ref_count_;
    if (count == 0)
    {
      delete this;
    }
    return count;
  }

  HRESULT STDMETHODCALLTYPE GetData(FORMATETC *format, STGMEDIUM *medium) override
  {
    if (!IsFileFormat(format) || medium == nullptr)
    {
      return DV_E_FORMATETC;
    }

    HGLOBAL copy = DuplicateGlobalMemory(hdrop_);
    if (copy == nullptr)
    {
      return STG_E_MEDIUMFULL;
    }

    medium->tymed = TYMED_HGLOBAL;
    medium->hGlobal = copy;
    medium->pUnkForRelease = nullptr;
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE GetDataHere(FORMATETC *, STGMEDIUM *) override
  {
    return DATA_E_FORMATETC;
  }

  HRESULT STDMETHODCALLTYPE QueryGetData(FORMATETC *format) override
  {
    return IsFileFormat(format) ? S_OK : DV_E_FORMATETC;
  }

  HRESULT STDMETHODCALLTYPE GetCanonicalFormatEtc(FORMATETC *, FORMATETC *format_out) override
  {
    if (format_out != nullptr)
    {
      format_out->ptd = nullptr;
    }
    return DATA_S_SAMEFORMATETC;
  }

  HRESULT STDMETHODCALLTYPE SetData(FORMATETC *, STGMEDIUM *, BOOL) override
  {
    return E_NOTIMPL;
  }

  HRESULT STDMETHODCALLTYPE EnumFormatEtc(DWORD direction, IEnumFORMATETC **enum_format) override
  {
    if (enum_format == nullptr)
    {
      return E_POINTER;
    }
    if (direction != DATADIR_GET)
    {
      *enum_format = nullptr;
      return E_NOTIMPL;
    }
    *enum_format = new FormatEtcEnumerator(format_);
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE DAdvise(FORMATETC *, DWORD, IAdviseSink *, DWORD *) override
  {
    return OLE_E_ADVISENOTSUPPORTED;
  }

  HRESULT STDMETHODCALLTYPE DUnadvise(DWORD) override
  {
    return OLE_E_ADVISENOTSUPPORTED;
  }

  HRESULT STDMETHODCALLTYPE EnumDAdvise(IEnumSTATDATA **) override
  {
    return OLE_E_ADVISENOTSUPPORTED;
  }

private:
  std::atomic<ULONG> ref_count_;
  HGLOBAL hdrop_;
  FORMATETC format_{};
};

class FileDropSource : public IDropSource
{
public:
  FileDropSource() : ref_count_(1) {}

  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID riid, void **object) override
  {
    if (object == nullptr)
    {
      return E_POINTER;
    }
    if (riid == IID_IUnknown || riid == IID_IDropSource)
    {
      *object = static_cast<IDropSource *>(this);
      AddRef();
      return S_OK;
    }
    *object = nullptr;
    return E_NOINTERFACE;
  }

  ULONG STDMETHODCALLTYPE AddRef() override
  {
    return ++ref_count_;
  }

  ULONG STDMETHODCALLTYPE Release() override
  {
    ULONG count = --ref_count_;
    if (count == 0)
    {
      delete this;
    }
    return count;
  }

  HRESULT STDMETHODCALLTYPE QueryContinueDrag(BOOL escape_pressed, DWORD key_state) override
  {
    if (escape_pressed)
    {
      return DRAGDROP_S_CANCEL;
    }
    if ((key_state & MK_LBUTTON) == 0)
    {
      return DRAGDROP_S_DROP;
    }
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE GiveFeedback(DWORD) override
  {
    return DRAGDROP_S_USEDEFAULTCURSORS;
  }

private:
  std::atomic<ULONG> ref_count_;
};

bool ExtractFiles(const flutter::EncodableValue *arguments, std::vector<std::wstring> *files)
{
  if (arguments == nullptr || files == nullptr)
  {
    return false;
  }
  const auto *args = std::get_if<flutter::EncodableMap>(arguments);
  if (args == nullptr)
  {
    return false;
  }
  auto file_iter = args->find(flutter::EncodableValue("files"));
  if (file_iter == args->end())
  {
    return false;
  }
  const auto *file_list = std::get_if<flutter::EncodableList>(&file_iter->second);
  if (file_list == nullptr)
  {
    return false;
  }

  for (const auto &item : *file_list)
  {
    const auto *file = std::get_if<std::string>(&item);
    if (file == nullptr || file->empty())
    {
      continue;
    }
    std::wstring wide = Utf16FromUtf8(*file);
    if (!wide.empty() && ::GetFileAttributesW(wide.c_str()) != INVALID_FILE_ATTRIBUTES)
    {
      files->push_back(wide);
    }
  }

  return !files->empty();
}

flutter::EncodableValue StartFileDrag(const flutter::EncodableValue *arguments)
{
  std::vector<std::wstring> files;
  if (!ExtractFiles(arguments, &files))
  {
    return StatusResult("error");
  }

  HGLOBAL hdrop = CreateHDropGlobal(files);
  if (hdrop == nullptr)
  {
    return StatusResult("error");
  }

  auto *data_object = new FileDataObject(hdrop);
  auto *drop_source = new FileDropSource();

  if (g_owner_window != nullptr)
  {
    ::ReleaseCapture();
    // DoDragDrop may stay blocked while the target app shows overwrite or
    // permission dialogs. Hide Wox once the drag source is ready so that wait
    // happens behind the target UI instead of looking like a frozen launcher.
    ::ShowWindow(g_owner_window, SW_HIDE);
  }

  DWORD effect = DROPEFFECT_NONE;
  HRESULT hr = ::DoDragDrop(data_object, drop_source, DROPEFFECT_COPY, &effect);

  data_object->Release();
  drop_source->Release();

  if (hr == DRAGDROP_S_DROP && (effect & DROPEFFECT_COPY) != 0)
  {
    return StatusResult("success");
  }
  if (hr == DRAGDROP_S_CANCEL)
  {
    return StatusResult("cancel");
  }
  return StatusResult("error");
}
} // namespace

void RegisterResultDragBridge(flutter::BinaryMessenger *messenger, HWND owner_window)
{
  g_owner_window = owner_window;
  g_result_drag_channel = std::make_unique<flutter::MethodChannel<flutter::EncodableValue>>(
      messenger, "com.wox.result_drag", &flutter::StandardMethodCodec::GetInstance());

  g_result_drag_channel->SetMethodCallHandler(
      [](const auto &call, auto result)
      {
        if (call.method_name() == "startFileDrag")
        {
          result->Success(StartFileDrag(call.arguments()));
          return;
        }
        result->NotImplemented();
      });
}
