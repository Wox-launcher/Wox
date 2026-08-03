source_file = [
   'tvgGpuCommon.h',
   'tvgGpuCommon.cpp'
]

engine_dep += [declare_dependency(
    include_directories : include_directories('.'),
    sources             : source_file
)]

if gl_engine
    subdir('gl')
endif

if wg_engine
    subdir('wg')
endif
