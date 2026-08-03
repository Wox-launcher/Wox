source_file = [
   'tvgCapi.cpp',
]

thorvg_lib_dep += [declare_dependency(
    include_directories : include_directories('.'),
    sources : source_file
)]

install_headers('thorvg_capi.h', subdir: 'thorvg-' + vmaj)
headers += include_directories('.')
