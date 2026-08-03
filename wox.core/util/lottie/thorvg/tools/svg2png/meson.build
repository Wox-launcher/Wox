svg2png_src  = files('svg2png.cpp', 'lodepng.cpp')

executable('tvg-svg2png',
           svg2png_src,
           include_directories : headers,
           cpp_args : compiler_flags,
           install : true,
           link_with : thorvg_lib)
