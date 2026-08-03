project('thorvg',
        'cpp',
        default_options : ['buildtype=debugoptimized', 'b_sanitize=none', 'werror=false', 'optimization=3', 'cpp_std=c++14', 'strip=true'],
        version : '1.1.0',
        license : 'MIT')

v_array = meson.project_version().split('.')
vmaj = v_array[0]

config_h = configuration_data()

src_dir = '/'.join(meson.current_source_dir().split('\\'))
add_project_arguments('-DTEST_DIR="@0@/test/resources"'.format(src_dir),
                      language : 'cpp')

config_h.set_quoted('THORVG_VERSION_STRING', meson.project_version())

# Multi-Tasking
if get_option('threads')
  config_h.set10('THORVG_THREAD_SUPPORT', true)
endif

# Engines
all_engines = get_option('engines').contains('all')

cpu_engine = false
if all_engines or get_option('engines').contains('cpu')
    cpu_engine = true
    config_h.set10('THORVG_CPU_ENGINE_SUPPORT', true)
endif

gl_engine = false
if all_engines or get_option('engines').contains('gl')
    gl_engine = true
    config_h.set10('THORVG_GL_ENGINE_SUPPORT', true)
endif

wg_engine = false
if all_engines or get_option('engines').contains('wg')
    wg_engine = true
    config_h.set10('THORVG_WG_ENGINE_SUPPORT', true)
endif

# Partial Rendering
if get_option('partial')
  config_h.set10('THORVG_PARTIAL_RENDER_SUPPORT', true)
endif

# Tools
all_tools = get_option('tools').contains('all')
lottie2gif = all_tools or get_option('tools').contains('lottie2gif')
svg2png = all_tools or get_option('tools').contains('svg2png')

# Loaders
all_loaders = get_option('loaders').contains('all')
svg_loader = all_loaders or get_option('loaders').contains('svg') or svg2png
png_loader = all_loaders or get_option('loaders').contains('png')
jpg_loader = all_loaders or get_option('loaders').contains('jpg')
lottie_loader = all_loaders or get_option('loaders').contains('lottie') or lottie2gif
ttf_loader = all_loaders or get_option('loaders').contains('ttf')
otf_loader = all_loaders or get_option('loaders').contains('otf')
webp_loader = all_loaders or get_option('loaders').contains('webp')

# Savers
all_savers = get_option('savers').contains('all')
gif_saver = all_savers or get_option('savers').contains('gif') or lottie2gif

# Logging
logging = get_option('log')

# Loaders/savers/tools config
if svg_loader
    config_h.set10('THORVG_SVG_LOADER_SUPPORT', true)
endif

if png_loader
   config_h.set10('THORVG_PNG_LOADER_SUPPORT', true)
endif

if jpg_loader
    config_h.set10('THORVG_JPG_LOADER_SUPPORT', true)
endif

if lottie_loader
    config_h.set10('THORVG_LOTTIE_LOADER_SUPPORT', true)
endif

if ttf_loader
    config_h.set10('THORVG_SFNT_LOADER_SUPPORT', true)
    config_h.set10('THORVG_TTF_LOADER_SUPPORT', true)
endif

if otf_loader
    config_h.set10('THORVG_SFNT_LOADER_SUPPORT', true)
    config_h.set10('THORVG_OTF_LOADER_SUPPORT', true)
endif

if webp_loader
    config_h.set10('THORVG_WEBP_LOADER_SUPPORT', true)
endif

if gif_saver
    config_h.set10('THORVG_GIF_SAVER_SUPPORT', true)
endif

# Vectorization
simd_type = 'none'

if get_option('simd')
  if host_machine.cpu_family().startswith('x86')
    config_h.set10('THORVG_AVX_VECTOR_SUPPORT', true)
    simd_type = 'avx'
  elif host_machine.cpu_family().startswith('arm')
    config_h.set10('THORVG_NEON_VECTOR_SUPPORT', true)
    simd_type = 'neon-arm'
  elif host_machine.cpu().startswith('aarch')
    config_h.set10('THORVG_NEON_VECTOR_SUPPORT', true)
    simd_type = 'neon-aarch'
  endif
endif

# Bindings
if get_option('bindings').contains('capi')
    config_h.set10('THORVG_CAPI_BINDING_SUPPORT', true)
endif

# Log
if logging
    config_h.set10('THORVG_LOG_ENABLED', true)
endif

# File IO
if get_option('file') == true
    config_h.set10('THORVG_FILE_IO_SUPPORT', true)
endif


# Extra
lottie_exp = lottie_loader and get_option('extra').contains('lottie_exp')

if lottie_exp
    config_h.set10('THORVG_LOTTIE_EXPRESSIONS_SUPPORT', true)
endif

openmp = get_option('extra').contains('openmp')

if openmp
    config_h.set10('THORVG_OPENMP_SUPPORT', true)
endif

gl_variant = ''

if gl_engine
    if get_option('extra').contains('opengl_es')
        gl_variant = 'OpenGL ES'
    else
        gl_variant = 'OpenGL'
    endif
else
    gl_variant = 'None'
endif

# Miscellaneous
config_h.set10('WIN32_LEAN_AND_MEAN', true)

configure_file(
    output: 'config.h',
    configuration: config_h
)

headers = [include_directories('inc'), include_directories('.')]

subdir('inc')
subdir('src')
subdir('tools')

if get_option('tests')
   subdir('test')
endif

summary(
  {
    'Build Type': get_option('buildtype'),
    'Prefix': get_option('prefix'),
    'File I/O': get_option('file'),
    'Multi-Tasking': get_option('threads'),
    'Partial Rendering': get_option('partial'),
    'SIMD Instruction': simd_type,
    'Log Message': get_option('log'),
    'Tests': get_option('tests')
  },
  bool_yn: true,
)

summary(
  {
    'CPU': cpu_engine,
    'GL': gl_engine,
    'WebGPU': wg_engine,
  },
  section: 'Raster Engine',
  bool_yn: true,
)

summary(
  {
    'SVG': svg_loader,
    'TTF': ttf_loader,
    'OTF': otf_loader,
    'LOTTIE': lottie_loader,
    'PNG': png_loader,
    'JPG': jpg_loader,
    'WEBP': webp_loader,
  },
  section: 'Loader',
  bool_yn: true,
)

summary(
  {
    'GIF': gif_saver,
  },
  section: 'Saver',
  bool_yn: true,
)

summary(
  {
    'CAPI': get_option('bindings').contains('capi'),
  },
  section: 'Binding',
  bool_yn: true,
)

summary(
  {
    'Svg2Png': svg2png,
    'Lottie2Gif': lottie2gif,
  },
  section: 'Tool',
  bool_yn: true,
)

summary(
  {
    'Lottie Expressions': lottie_exp,
    'OpenMP': openmp,
    'OpenGL Variant': gl_variant
  },
  section: 'Extra',
  bool_yn: true,
)
