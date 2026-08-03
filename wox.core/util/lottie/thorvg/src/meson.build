compiler_flags = []
override_options = []

lib_type = get_option('default_library')

if lib_type == 'shared'
    compiler_flags += ['-DTVG_EXPORT', '-DTVG_BUILD']
else
    compiler_flags += ['-DTVG_STATIC']
endif

cc = meson.get_compiler('cpp')

if host_machine.system() == 'windows'
    compiler_flags += ['-DNOMINMAX']
endif

if cc.get_id() == 'clang-cl'
    if simd_type == 'avx'
        compiler_flags += ['/clang:-mavx']
    endif
    if simd_type == 'neon-arm'
        compiler_flags += ['/clang:-mfpu=neon']
    endif
    compiler_flags += ['/clang:-Wno-unknown-pragmas']

    if logging == false
        compiler_flags += ['/clang:-Wdouble-promotion']
    endif

    if get_option('b_sanitize') == 'none'
        override_options += ['cpp_eh=none','cpp_rtti=false']
        compiler_flags += ['/clang:-fno-math-errno', '/clang:-Woverloaded-virtual',
                           '/clang:-fno-stack-protector', '/clang:-fno-unwind-tables' ,
                           '/clang:-fno-asynchronous-unwind-tables']
    endif
elif (cc.get_id() != 'msvc')
    if simd_type == 'avx'
        compiler_flags += ['-mavx']
    endif
    if simd_type == 'neon-arm'
        compiler_flags += ['-mfpu=neon']
    endif
    compiler_flags += ['-Wno-unknown-pragmas']

    if logging == false
        compiler_flags += ['-Wdouble-promotion']
    endif

    if get_option('b_sanitize') == 'none'
        compiler_flags += ['-fno-exceptions', '-fno-rtti', '-fno-stack-protector', '-fno-math-errno',
                           '-fno-unwind-tables', '-fno-asynchronous-unwind-tables', '-Woverloaded-virtual']
    endif
endif

subdir('common')
subdir('renderer')
subdir('loaders')
subdir('savers')

thorvg_lib_dep = [common_dep, utils_dep, loader_dep, saver_dep]

if host_machine.system() != 'windows' and host_machine.system() != 'android' and host_machine.system() != 'emscripten'
    thread_dep = meson.get_compiler('cpp').find_library('pthread')
    thorvg_lib_dep += [thread_dep]
endif

subdir('bindings')

thorvg_lib = library(
    'thorvg-' + vmaj,
    include_directories    : headers,
    version                : meson.project_version(),
    soversion              : host_machine.system() != 'windows' ? vmaj : '',
    dependencies           : thorvg_lib_dep,
    install                : true,
    cpp_args               : compiler_flags,
    gnu_symbol_visibility  : 'hidden',
    override_options       : override_options
)

thorvg_dep = declare_dependency(
  compile_args: (lib_type == 'static') ? ['-DTVG_STATIC'] : [],
  include_directories: thorvg_inc,
  link_with: thorvg_lib,
)
meson.override_dependency('thorvg-' + vmaj, thorvg_dep)

pkg_mod = import('pkgconfig')

pkg_mod.generate(
    libraries    : thorvg_lib,
    version      : meson.project_version(),
    name         : 'libthorvg',
    filebase     : 'thorvg-' + vmaj,
    subdirs      : 'thorvg-' + vmaj,
    description  : 'A Thor library for rendering vector graphics'
)
