#pragma once

#include <windows.ui.composition.interop.h>

namespace util {

winrt::com_ptr<ABI::Windows::UI::Composition::Desktop::IDesktopWindowTarget>
TryCreateDesktopWindowTarget(
    const winrt::com_ptr<ABI::Windows::UI::Composition::ICompositor>&
        compositor,
    HWND window) {
  namespace abi = ABI::Windows::UI::Composition::Desktop;
  auto interop = compositor.try_as<abi::ICompositorDesktopInterop>();

  winrt::com_ptr<abi::IDesktopWindowTarget> target;
  interop->CreateDesktopWindowTarget(window, true, target.put());
  return target;
}

}  // namespace util
