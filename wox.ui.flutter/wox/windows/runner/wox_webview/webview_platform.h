#pragma once

#include <winrt/base.h>

#include <memory>
#include <optional>
#include <string>

#include "graphics_context.h"
#include "util/rohelper.h"

class WebviewPlatform {
 public:
  WebviewPlatform();
  bool IsSupported() { return valid_; }
  std::optional<std::wstring> GetDefaultDataDirectory();
  bool IsGraphicsCaptureSessionSupported();
  GraphicsContext* graphics_context() const {
    return graphics_context_.get();
  };

  rx::RoHelper* rohelper() const { return rohelper_.get(); }

 private:
  std::unique_ptr<rx::RoHelper> rohelper_;
  winrt::com_ptr<ABI::Windows::System::IDispatcherQueueController>
      dispatcher_queue_controller_;
  std::unique_ptr<GraphicsContext> graphics_context_;
  bool valid_ = false;
};
