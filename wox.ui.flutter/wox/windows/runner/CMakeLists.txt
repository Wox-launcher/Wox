cmake_minimum_required(VERSION 3.14)
project(runner LANGUAGES CXX)

# Define the application target. To change its name, change BINARY_NAME in the
# top-level CMakeLists.txt, not the value here, or `flutter run` will no longer
# work.
#
# Any new source files that you add to the application should be added here.
add_executable(${BINARY_NAME} WIN32
  "flutter_window.cpp"
  "main.cpp"
  "result_drag_bridge.cpp"
  "utils.cpp"
  "win32_window.cpp"
  "wox_preview_handler_plugin.cpp"
  "wox_webview/graphics_context.cc"
  "wox_webview/texture_bridge.cc"
  "wox_webview/webview.cc"
  "wox_webview/webview_bridge.cc"
  "wox_webview/webview_host.cc"
  "wox_webview/webview_platform.cc"
  "wox_webview/webview_windows_plugin.cc"
  "wox_webview/util/direct3d11.interop.cc"
  "wox_webview/util/rohelper.cc"
  "wox_webview/util/string_converter.cc"
  "${FLUTTER_MANAGED_DIR}/generated_plugin_registrant.cc"
  "Runner.rc"
  "runner.exe.manifest"
)

# Apply the standard set of build settings. This can be removed for applications
# that need different build settings.
apply_standard_settings(${BINARY_NAME})
target_compile_features(${BINARY_NAME} PUBLIC cxx_std_20)
if(MSVC)
  target_compile_options(${BINARY_NAME} PRIVATE /await)
endif()

# Add preprocessor definitions for the build version.
target_compile_definitions(${BINARY_NAME} PRIVATE "FLUTTER_VERSION=\"${FLUTTER_VERSION}\"")
target_compile_definitions(${BINARY_NAME} PRIVATE "FLUTTER_VERSION_MAJOR=${FLUTTER_VERSION_MAJOR}")
target_compile_definitions(${BINARY_NAME} PRIVATE "FLUTTER_VERSION_MINOR=${FLUTTER_VERSION_MINOR}")
target_compile_definitions(${BINARY_NAME} PRIVATE "FLUTTER_VERSION_PATCH=${FLUTTER_VERSION_PATCH}")
target_compile_definitions(${BINARY_NAME} PRIVATE "FLUTTER_VERSION_BUILD=${FLUTTER_VERSION_BUILD}")
target_compile_definitions(${BINARY_NAME} PRIVATE FLUTTER_PLUGIN_IMPL)

# Disable Windows macros that collide with C++ standard library functions.
target_compile_definitions(${BINARY_NAME} PRIVATE "NOMINMAX")

# Add dependency libraries and include directories. Add any application-specific
# dependencies here.
target_link_libraries(${BINARY_NAME} PRIVATE flutter flutter_wrapper_app flutter_wrapper_plugin)
target_link_libraries(${BINARY_NAME} PRIVATE "Shell32.lib")
target_link_libraries(${BINARY_NAME} PRIVATE "Ole32.lib")
target_link_libraries(${BINARY_NAME} PRIVATE "Shlwapi.lib")
target_link_libraries(${BINARY_NAME} PRIVATE "gdiplus.lib")
target_link_libraries(${BINARY_NAME} PRIVATE "dwmapi.lib")
target_link_libraries(${BINARY_NAME} PRIVATE "UIAutomationCore.lib")
target_link_libraries(${BINARY_NAME} PRIVATE "dxgi.lib")
target_link_libraries(${BINARY_NAME} PRIVATE "d3d11.lib")
target_link_libraries(${BINARY_NAME} PRIVATE ${CMAKE_BINARY_DIR}/packages/Microsoft.Web.WebView2/build/native/Microsoft.Web.WebView2.targets)
target_link_libraries(${BINARY_NAME} PRIVATE ${CMAKE_BINARY_DIR}/packages/Microsoft.Windows.ImplementationLibrary/build/native/Microsoft.Windows.ImplementationLibrary.targets)
target_include_directories(${BINARY_NAME} PRIVATE "${CMAKE_SOURCE_DIR}")
target_include_directories(${BINARY_NAME} PRIVATE "${CMAKE_CURRENT_SOURCE_DIR}/wox_webview")
target_include_directories(${BINARY_NAME} PRIVATE "${CMAKE_CURRENT_SOURCE_DIR}/wox_webview/include")

# Run the Flutter tool portions of the build. This must not be removed.
add_dependencies(${BINARY_NAME} flutter_assemble)

set(WOX_WEBVIEW_WIL_VERSION "1.0.220914.1")
set(WOX_WEBVIEW_WEBVIEW2_VERSION "1.0.1210.39")
set(WOX_WEBVIEW_NUGET_URL https://dist.nuget.org/win-x86-commandline/v5.10.0/nuget.exe)
set(WOX_WEBVIEW_NUGET_SHA256 852b71cc8c8c2d40d09ea49d321ff56fd2397b9d6ea9f96e532530307bbbafd3)

find_program(WOX_WEBVIEW_NUGET nuget)
if(NOT WOX_WEBVIEW_NUGET)
  set(WOX_WEBVIEW_NUGET ${CMAKE_BINARY_DIR}/nuget.exe)
  if(NOT EXISTS ${WOX_WEBVIEW_NUGET})
    file(DOWNLOAD ${WOX_WEBVIEW_NUGET_URL} ${WOX_WEBVIEW_NUGET})
  endif()

  file(SHA256 ${WOX_WEBVIEW_NUGET} WOX_WEBVIEW_NUGET_HASH)
  if(NOT WOX_WEBVIEW_NUGET_HASH STREQUAL WOX_WEBVIEW_NUGET_SHA256)
    message(FATAL_ERROR "Integrity check for ${WOX_WEBVIEW_NUGET} failed.")
  endif()
endif()

add_custom_target(wox_webview_dependencies_download ALL)
add_custom_command(
  TARGET wox_webview_dependencies_download PRE_BUILD
  COMMAND ${WOX_WEBVIEW_NUGET} install Microsoft.Windows.ImplementationLibrary -Version ${WOX_WEBVIEW_WIL_VERSION} -ExcludeVersion -OutputDirectory ${CMAKE_BINARY_DIR}/packages
  COMMAND ${WOX_WEBVIEW_NUGET} install Microsoft.Web.WebView2 -Version ${WOX_WEBVIEW_WEBVIEW2_VERSION} -ExcludeVersion -OutputDirectory ${CMAKE_BINARY_DIR}/packages
  DEPENDS ${WOX_WEBVIEW_NUGET}
)
add_dependencies(${BINARY_NAME} wox_webview_dependencies_download)

if(NOT FLUTTER_WEBVIEW_WINDOWS_USE_TEXTURE_FALLBACK)
  target_compile_definitions(${BINARY_NAME} PRIVATE HAVE_FLUTTER_D3D_TEXTURE)
  target_sources(${BINARY_NAME} PRIVATE "wox_webview/texture_bridge_gpu.cc")
else()
  target_sources(${BINARY_NAME} PRIVATE "wox_webview/texture_bridge_fallback.cc" "wox_webview/util/cpuid/cpuinfo.cc")
  if(MSVC)
    target_compile_options(${BINARY_NAME} PRIVATE /arch:AVX2)
  endif()
endif()
