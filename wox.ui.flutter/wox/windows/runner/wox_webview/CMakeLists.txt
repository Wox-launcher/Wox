cmake_minimum_required(VERSION 3.15)
set(PROJECT_NAME "webview_windows")

set(WIL_VERSION "1.0.220914.1")
set(WEBVIEW_VERSION "1.0.1210.39")

message(VERBOSE "CMake system version is ${CMAKE_SYSTEM_VERSION} (using SDK ${CMAKE_VS_WINDOWS_TARGET_PLATFORM_VERSION})")

project(${PROJECT_NAME} LANGUAGES CXX)

# This value is used when generating builds using this plugin, so it must
# not be changed
set(PLUGIN_NAME "webview_windows_plugin")

set(NUGET_URL https://dist.nuget.org/win-x86-commandline/v5.10.0/nuget.exe)
set(NUGET_SHA256 852b71cc8c8c2d40d09ea49d321ff56fd2397b9d6ea9f96e532530307bbbafd3)

find_program(NUGET nuget)
if(NOT NUGET)
  message(NOTICE "Nuget is not installed.")
  set(NUGET ${CMAKE_BINARY_DIR}/nuget.exe)
  if (NOT EXISTS ${NUGET})
    message(NOTICE "Attempting to download nuget.")
    file(DOWNLOAD ${NUGET_URL} ${NUGET})
  endif()

  file(SHA256 ${NUGET} NUGET_DL_HASH)
  if (NOT NUGET_DL_HASH STREQUAL NUGET_SHA256)
    message(FATAL_ERROR "Integrity check for ${NUGET} failed.")
  endif()
endif()

add_custom_target(${PROJECT_NAME}_DEPENDENCIES_DOWNLOAD ALL)
add_custom_command(
  TARGET ${PROJECT_NAME}_DEPENDENCIES_DOWNLOAD PRE_BUILD
  COMMAND ${NUGET} install Microsoft.Windows.ImplementationLibrary -Version ${WIL_VERSION} -ExcludeVersion -OutputDirectory ${CMAKE_BINARY_DIR}/packages
  COMMAND ${NUGET} install Microsoft.Web.WebView2 -Version ${WEBVIEW_VERSION} -ExcludeVersion -OutputDirectory ${CMAKE_BINARY_DIR}/packages
  DEPENDS ${NUGET}
)

add_library(${PLUGIN_NAME} SHARED
  "webview_windows_plugin.cc"
  "webview_platform.cc"
  "webview.cc"
  "webview_host.cc"
  "webview_bridge.cc"
  "texture_bridge.cc"
  "graphics_context.cc"
  "util/direct3d11.interop.cc"
  "util/rohelper.cc"
  "util/string_converter.cc"
)

if(MSVC)
    target_compile_options(${PLUGIN_NAME} PRIVATE "/await")
endif()

if(NOT FLUTTER_WEBVIEW_WINDOWS_USE_TEXTURE_FALLBACK)
  message(STATUS "Building with D3D texture support.")
  target_compile_definitions("${PLUGIN_NAME}" PRIVATE
    HAVE_FLUTTER_D3D_TEXTURE
  )
  target_sources("${PLUGIN_NAME}" PRIVATE
    "texture_bridge_gpu.cc"
  )
else()
  message(STATUS "Building with fallback PixelBuffer texture.")
  target_sources("${PLUGIN_NAME}" PRIVATE
    "texture_bridge_fallback.cc"
    "util/cpuid/cpuinfo.cc"
  )
  # Enable AVX2 for pixel buffer conversions
  if(MSVC)
    target_compile_options(${PLUGIN_NAME} PRIVATE "/arch:AVX2" "/await")
  endif()
endif()

apply_standard_settings(${PLUGIN_NAME})
target_compile_features(${PLUGIN_NAME} PUBLIC cxx_std_20) # For std::format support

set_target_properties(${PLUGIN_NAME} PROPERTIES CXX_VISIBILITY_PRESET hidden)

target_link_libraries(${PLUGIN_NAME} PRIVATE ${CMAKE_BINARY_DIR}/packages/Microsoft.Web.WebView2/build/native/Microsoft.Web.WebView2.targets)
target_link_libraries(${PLUGIN_NAME} PRIVATE ${CMAKE_BINARY_DIR}/packages/Microsoft.Windows.ImplementationLibrary/build/native/Microsoft.Windows.ImplementationLibrary.targets)

target_compile_definitions(${PLUGIN_NAME} PRIVATE FLUTTER_PLUGIN_IMPL)
target_include_directories(${PLUGIN_NAME} INTERFACE
  "${CMAKE_CURRENT_SOURCE_DIR}/include"
)

target_link_libraries(${PLUGIN_NAME} PRIVATE flutter flutter_wrapper_plugin)

set(webview_windows_bundled_libraries
  PARENT_SCOPE
)
