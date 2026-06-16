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

#pragma once

#include <dispatcherqueue.h>
#include <roapi.h>
#include <windows.ui.composition.interop.h>

namespace rx {
class RoHelper {
 public:
  RoHelper(RO_INIT_TYPE init_type);
  ~RoHelper();
  bool WinRtAvailable() const;
  bool SupportedWindowsRelease();
  HRESULT GetStringReference(PCWSTR source, HSTRING* act,
                             HSTRING_HEADER* header);
  HRESULT GetActivationFactory(const HSTRING act, const IID& interfaceId,
                               void** fac);
  HRESULT WindowsCompareStringOrdinal(HSTRING one, HSTRING two, int* result);
  HRESULT CreateDispatcherQueueController(
      DispatcherQueueOptions options,
      ABI::Windows::System::IDispatcherQueueController**
          dispatcherQueueController);
  HRESULT WindowsDeleteString(HSTRING one);
  HRESULT RoInitialize(RO_INIT_TYPE type);
  void RoUninitialize();

 private:
  using WindowsCreateStringReference_ = HRESULT __stdcall(PCWSTR, UINT32,
                                                          HSTRING_HEADER*,
                                                          HSTRING*);

  using GetActivationFactory_ = HRESULT __stdcall(HSTRING, REFIID, void**);

  using WindowsCompareStringOrginal_ = HRESULT __stdcall(HSTRING, HSTRING,
                                                         int*);

  using WindowsDeleteString_ = HRESULT __stdcall(HSTRING);

  using CreateDispatcherQueueController_ =
      HRESULT __stdcall(DispatcherQueueOptions,
                        ABI::Windows::System::IDispatcherQueueController**);

  using RoInitialize_ = HRESULT __stdcall(RO_INIT_TYPE);
  using RoUninitialize_ = void __stdcall();

  WindowsCreateStringReference_* mFpWindowsCreateStringReference;
  GetActivationFactory_* mFpGetActivationFactory;
  WindowsCompareStringOrginal_* mFpWindowsCompareStringOrdinal;
  CreateDispatcherQueueController_* mFpCreateDispatcherQueueController;
  WindowsDeleteString_* mFpWindowsDeleteString;
  RoInitialize_* mFpRoInitialize;
  RoUninitialize_* mFpRoUninitialize;

  bool mWinRtAvailable;

  HMODULE mComBaseModule;
  HMODULE mCoreMessagingModule;
};
}  // namespace rx
