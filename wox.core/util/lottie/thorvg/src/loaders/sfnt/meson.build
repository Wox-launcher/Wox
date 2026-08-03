source_file = [
   'tvgSfntLoader.h',
   'tvgSfntReader.h',
   'tvgSfntLoader.cpp',
   'tvgSfntReader.cpp'
]

if ttf_loader
    source_file += [
        'tvgTtfReader.h',
        'tvgTtfReader.cpp'
    ]
endif

if otf_loader
    source_file += [
        'tvgOtfReader.h',
        'tvgOtfReader.cpp'
    ]
endif

subloader_dep += [declare_dependency(
    include_directories : include_directories('.'),
    sources : source_file
)]
