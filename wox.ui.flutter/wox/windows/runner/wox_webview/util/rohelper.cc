// Based on ANGLE's RoHelper (CompositorNativeWindow11.{cpp,h})
// - https://github.com/google/angle/blob/main/src/libANGLE/renderer/d3d/d3d11/converged/CompositorNativeWindow11.h
// - https://github.com/google/angle/blob/main/src/libANGLE/renderer/d3d/d3d11/converged/CompositorNativeWindow11.cpp
// - https://gist.github.com/clarkezone/43e984fb9bdcd2cfcd9a4f41c208a02f 
//
// Copyright 2018 The ANGLE Project Authors.
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
//     Redistributions of source code must retain the above copyright
//     notice, this list of conditions and the following disclaimer.
//
//     Redistributions in binary form must reproduce the above
//     copyright notice, this list of conditions and the following
//     disclaimer in the documentation and/or other materials provided
//     with the distribution.
//
//     Neither the name of TransGaming Inc., Google Inc., 3DLabs Inc.
//     Ltd., nor the names of their contributors may be used to endorse
//     or promote products derived from this software without specific
//     prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS
// FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE
// COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT,
// INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING,
// BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
// LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
// CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
// LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN
// ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

#include "rohelper.h"

#include <windows.foundation.metadata.h>
#include <wrl.h>

namespace rx {
template <typename T>
bool AssignProcAddress(HMODULE comBaseModule, const char* name, T*& outProc) {
  outProc = reinterpret_cast<T*>(GetProcAddress(comBaseModule, name));
  return *outProc != nullptr;
}

RoHelper::RoHelper(RO_INIT_TYPE init_type)
    : mFpWindowsCreateStringReference(nullptr),
      mFpGetActivationFactory(nullptr),
      mFpWindowsCompareStringOrdinal(nullptr),
      mFpCreateDispatcherQueueController(nullptr),
      mFpWindowsDeleteString(nullptr),
      mFpRoInitialize(nullptr),
      mFpRoUninitialize(nullptr),
      mWinRtAvailable(false),
      mComBaseModule(nullptr),
      mCoreMessagingModule(nullptr) {
#ifdef WINUWP
  mFpWindowsCreateStringReference = &::WindowsCreateStringReference;
  mFpRoInitialize = &::RoInitialize;
  mFpRoUninitialize = &::RoUninitialize;
  mFpWindowsDeleteString = &::WindowsDeleteString;
  mFpGetActivationFactory = &::RoGetActivationFactory;
  mFpWindowsCompareStringOrdinal = &::WindowsCompareStringOrdinal;
  mFpCreateDispatcherQueueController = &::CreateDispatcherQueueController;
  mWinRtAvailable = true;
#else

  mComBaseModule = LoadLibraryA("ComBase.dll");

  if (mComBaseModule == nullptr) {
    return;
  }

  if (!AssignProcAddress(mComBaseModule, "WindowsCreateStringReference",
                         mFpWindowsCreateStringReference)) {
    return;
  }

  if (!AssignProcAddress(mComBaseModule, "RoGetActivationFactory",
                         mFpGetActivationFactory)) {
    return;
  }

  if (!AssignProcAddress(mComBaseModule, "WindowsCompareStringOrdinal",
                         mFpWindowsCompareStringOrdinal)) {
    return;
  }

  if (!AssignProcAddress(mComBaseModule, "WindowsDeleteString",
                         mFpWindowsDeleteString)) {
    return;
  }

  if (!AssignProcAddress(mComBaseModule, "RoInitialize", mFpRoInitialize)) {
    return;
  }

  if (!AssignProcAddress(mComBaseModule, "RoUninitialize", mFpRoUninitialize)) {
    return;
  }

  mCoreMessagingModule = LoadLibraryA("coremessaging.dll");

  if (mCoreMessagingModule == nullptr) {
    return;
  }

  if (!AssignProcAddress(mCoreMessagingModule,
                         "CreateDispatcherQueueController",
                         mFpCreateDispatcherQueueController)) {
    return;
  }

  auto result = RoInitialize(init_type);

  if (SUCCEEDED(result) || result == S_FALSE || result == RPC_E_CHANGED_MODE) {
    mWinRtAvailable = true;
  }
#endif
}

RoHelper::~RoHelper() {
#ifndef WINUWP
  if (mWinRtAvailable) {
    RoUninitialize();
  }

  if (mCoreMessagingModule != nullptr) {
    FreeLibrary(mCoreMessagingModule);
    mCoreMessagingModule = nullptr;
  }

  if (mComBaseModule != nullptr) {
    FreeLibrary(mComBaseModule);
    mComBaseModule = nullptr;
  }
#endif
}

bool RoHelper::WinRtAvailable() const { return mWinRtAvailable; }

bool RoHelper::SupportedWindowsRelease() {
  if (!mWinRtAvailable) {
    return false;
  }

  HSTRING className, contractName;
  HSTRING_HEADER classNameHeader, contractNameHeader;
  boolean isSupported = false;

  HRESULT hr = GetStringReference(
      RuntimeClass_Windows_Foundation_Metadata_ApiInformation, &className,
      &classNameHeader);

  if (FAILED(hr)) {
    return !!isSupported;
  }

  Microsoft::WRL::ComPtr<
      ABI::Windows::Foundation::Metadata::IApiInformationStatics>
      api;

  hr = GetActivationFactory(
      className,
      __uuidof(ABI::Windows::Foundation::Metadata::IApiInformationStatics),
      &api);

  if (FAILED(hr)) {
    return !!isSupported;
  }

  hr = GetStringReference(L"Windows.Foundation.UniversalApiContract",
                          &contractName, &contractNameHeader);
  if (FAILED(hr)) {
    return !!isSupported;
  }

  api->IsApiContractPresentByMajor(contractName, 6, &isSupported);

  return !!isSupported;
}

HRESULT RoHelper::GetStringReference(PCWSTR source, HSTRING* act,
                                     HSTRING_HEADER* header) {
  if (!mWinRtAvailable) {
    return E_FAIL;
  }

  const wchar_t* str = static_cast<const wchar_t*>(source);

  unsigned int length;
  HRESULT hr = SizeTToUInt32(::wcslen(str), &length);
  if (FAILED(hr)) {
    return hr;
  }

  return mFpWindowsCreateStringReference(source, length, header, act);
}

HRESULT RoHelper::GetActivationFactory(const HSTRING act,
                                       const IID& interfaceId, void** fac) {
  if (!mWinRtAvailable) {
    return E_FAIL;
  }
  auto hr = mFpGetActivationFactory(act, interfaceId, fac);
  return hr;
}

HRESULT RoHelper::WindowsCompareStringOrdinal(HSTRING one, HSTRING two,
                                              int* result) {
  if (!mWinRtAvailable) {
    return E_FAIL;
  }
  return mFpWindowsCompareStringOrdinal(one, two, result);
}

HRESULT RoHelper::CreateDispatcherQueueController(
    DispatcherQueueOptions options,
    ABI::Windows::System::IDispatcherQueueController**
        dispatcherQueueController) {
  if (!mWinRtAvailable) {
    return E_FAIL;
  }
  return mFpCreateDispatcherQueueController(options, dispatcherQueueController);
}

HRESULT RoHelper::WindowsDeleteString(HSTRING one) {
  if (!mWinRtAvailable) {
    return E_FAIL;
  }
  return mFpWindowsDeleteString(one);
}

HRESULT RoHelper::RoInitialize(RO_INIT_TYPE type) {
  return mFpRoInitialize(type);
}

void RoHelper::RoUninitialize() { mFpRoUninitialize(); }
}  // namespace rx
