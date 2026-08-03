source_file = [
   'tvgJpgLoader.h',
   'tvgJpgLoader.cpp',
]

jpg_dep = dependency('libturbojpeg', required: false)

if not jpg_dep.found()
    jpg_dep = cc.find_library('turbojpeg', required: false)
endif

if jpg_dep.found()
    subloader_dep += [declare_dependency(
        include_directories : include_directories('.'),
        dependencies : jpg_dep,
        sources : source_file
    )]
endif
