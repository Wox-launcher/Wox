subloader_dep = []

if svg_loader
    subdir('svg')
endif

if ttf_loader or otf_loader
    subdir('sfnt')
endif

if lottie_loader
    subdir('lottie')
endif

if png_loader
    if get_option('static')
        subdir('png')
    else
        subdir('external_png')
        if not png_dep.found()
            subdir('png')
        endif
    endif
endif

if jpg_loader
    if get_option('static')
        subdir('jpg')
    else
        subdir('external_jpg')
        if not jpg_dep.found()
            subdir('jpg')
        endif
    endif
endif

if webp_loader
    if get_option('static')
        subdir('webp')
    else
        subdir('external_webp')
        if not webp_dep.found()
            subdir('webp')
        endif
    endif
endif

subdir('raw')

loader_dep = declare_dependency(
    dependencies: subloader_dep,
    include_directories : include_directories('.'),
)
