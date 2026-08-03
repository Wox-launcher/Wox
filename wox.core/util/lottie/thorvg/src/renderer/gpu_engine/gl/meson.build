source_file = [
   'tvgGl.h',
   'tvgGlCommon.h',
   'tvgGlEffect.h',
   'tvgGlGpuBuffer.h',
   'tvgGlTextureMgr.h',
   'tvgGlProgram.h',
   'tvgGlRenderer.h',
   'tvgGlRenderPass.h',
   'tvgGlRenderTarget.h',
   'tvgGlRenderTask.h',
   'tvgGlShader.h',
   'tvgGlShaderSrc.h',
   'tvgGlSolidBatch.h',
   'tvgGlStencilCoverBatch.h',
   'tvgGl.cpp',
   'tvgGlEffect.cpp',
   'tvgGlGeometry.cpp',
   'tvgGlGpuBuffer.cpp',
   'tvgGlTextureMgr.cpp',
   'tvgGlProgram.cpp',
   'tvgGlRenderer.cpp',
   'tvgGlSolidBatch.cpp',
   'tvgGlRenderPass.cpp',
   'tvgGlRenderTarget.cpp',
   'tvgGlRenderTask.cpp',
   'tvgGlShader.cpp',
   'tvgGlShaderSrc.cpp',
   'tvgGlStencilCoverBatch.cpp',
   'tvgGlTessellator.cpp',
   'tvgGlTessellator.h',
]

#force to use gles
if cc.get_id() == 'emscripten'
    gl_variant = 'OpenGL ES'
endif

if gl_variant == 'OpenGL ES'
    compiler_flags += ['-DTHORVG_GL_TARGET_GLES=1']
else
    compiler_flags += ['-DTHORVG_GL_TARGET_GL=1']
endif

engine_dep += [declare_dependency(
    include_directories : include_directories('.'),
    sources             : source_file,
)]
