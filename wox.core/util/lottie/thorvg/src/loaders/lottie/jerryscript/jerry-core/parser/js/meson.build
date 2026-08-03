source_file = [
    'byte-code.h',
    'common.h',
    'js-lexer.h',
    'js-parser-limits.h',
    'js-parser.h',
    'js-scanner.h',
    'js-parser-internal.h',
    'js-parser-tagged-template-literal.h',
    'js-scanner-internal.h',
    'parser-errors.h',
    'parser-error-messages.inc.h',
    'byte-code.cpp',
    'common.cpp',
    'js-lexer.cpp',
    'js-parser.cpp',
    'js-parser-expr.cpp',
    'js-parser-mem.cpp',
    'js-parser-module.cpp',
    'js-parser-statm.cpp',
    'js-parser-tagged-template-literal.cpp',
    'js-parser-util.cpp',
    'js-scanner.cpp',
    'js-scanner-ops.cpp',
    'js-scanner-util.cpp',
    'parser-errors.cpp'
]

subloader_dep += [declare_dependency(
    include_directories : include_directories('.'),
    sources : source_file
)]
