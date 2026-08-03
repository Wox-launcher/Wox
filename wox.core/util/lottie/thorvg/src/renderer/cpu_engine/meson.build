source_file = [
   'tvgSwCommon.h',
   'tvgSwRasterC.h',
   'tvgSwRasterAvx.h',
   'tvgSwRasterNeon.h',
   'tvgSwRasterTexmap.h',
   'tvgSwBlendOp.cpp',
   'tvgSwFill.cpp',
   'tvgSwImage.cpp',
   'tvgSwMemPool.cpp',
   'tvgSwPostEffect.cpp',
   'tvgSwRenderer.h',
   'tvgSwRaster.cpp',
   'tvgSwRenderer.cpp',
   'tvgSwRle.cpp',
   'tvgSwShape.cpp',
   'tvgSwStroke.cpp',
   'tvgSwUtil.cpp'
]

omp_dep = []

if openmp
    omp_dep = dependency('openmp', required: false)
endif

engine_dep += [declare_dependency(
    include_directories : include_directories('.'),
    dependencies        : omp_dep,
    sources             : source_file
)]
