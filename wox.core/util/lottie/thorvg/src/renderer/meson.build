engine_dep = []

if cpu_engine
    subdir('cpu_engine')
endif

if gl_engine or wg_engine
    subdir('gpu_engine')
endif

source_file = [
   'tvgAnimation.h',
   'tvgCanvas.h',
   'tvgFill.h',
   'tvgLoader.h',
   'tvgLoaderMgr.h',
   'tvgPicture.h',
   'tvgRender.h',
   'tvgAccessor.h',
   'tvgSaveModule.h',
   'tvgScene.h',
   'tvgShape.h',
   'tvgTaskScheduler.h',
   'tvgText.h',
   'tvgAccessor.cpp',
   'tvgAnimation.cpp',
   'tvgCanvas.cpp',
   'tvgFill.cpp',
   'tvgInitializer.cpp',
   'tvgLoaderMgr.cpp',
   'tvgPaint.cpp',
   'tvgPicture.cpp',
   'tvgRender.cpp',
   'tvgSaver.cpp',
   'tvgScene.cpp',
   'tvgShape.cpp',
   'tvgTaskScheduler.cpp',
   'tvgText.cpp'
]

common_dep = declare_dependency(
   dependencies        : engine_dep,
   include_directories : include_directories('.'),
   sources             : source_file
)
