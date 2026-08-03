source_file = [
    'tvgWgBindGroups.h',
    'tvgWgCommon.h',
    'tvgWgCompositor.h',
    'tvgWgGeometry.h',
    'tvgWgPipelines.h',
    'tvgWgRenderData.h',
    'tvgWgRenderer.h',
    'tvgWgRenderTarget.h',
    'tvgWgRenderTask.h',
    'tvgWgShaderSrc.h',
    'tvgWgShaderTypes.h',
    'tvgWgTextureMgr.h',
    'tvgWgTessellator.h',
    'tvgWgBindGroups.cpp',
    'tvgWgCommon.cpp',
    'tvgWgCompositor.cpp',
    'tvgWgGeometry.cpp',
    'tvgWgPipelines.cpp',
    'tvgWgRenderData.cpp',
    'tvgWgRenderer.cpp',
    'tvgWgRenderTarget.cpp',
    'tvgWgRenderTask.cpp',
    'tvgWgShaderSrc.cpp',
    'tvgWgShaderTypes.cpp',
    'tvgWgTextureMgr.cpp',
    'tvgWgTessellator.cpp'
]

wgpu_dep = []
if host_machine.system() != 'emscripten'
    wgpu_dep = dependency('wgpu_native')
endif

engine_dep += [declare_dependency(
    dependencies        : wgpu_dep,
    include_directories : include_directories('.'),
    sources : source_file
)]
