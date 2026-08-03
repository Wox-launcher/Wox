if get_option('threads')
    source_file = [
        'jerry-port-context.cpp'
    ]

    subloader_dep += [declare_dependency(
        include_directories : include_directories('.'),
        sources : source_file,
        compile_args : ['-DJERRY_EXTERNAL_CONTEXT=1']
    )]
endif
