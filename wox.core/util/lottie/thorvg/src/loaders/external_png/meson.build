source_file = [
   'tvgPngLoader.h',
   'tvgPngLoader.cpp',
]

png_dep = dependency('libpng', required: false)

if png_dep.found()
    subloader_dep += [declare_dependency(
        include_directories : include_directories('.'),
        dependencies : png_dep,
        sources : source_file
    )]
endif
