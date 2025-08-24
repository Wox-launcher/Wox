use ExtUtils::MakeMaker;

WriteMakefile(
    NAME         => 'WoxMR',
    VERSION_FROM => 'WoxMR.pm',
    OBJECT       => '$(O_FILES) wox_mr.o',
    INC          => '-I.',
    dynamic_lib  => { OTHERLDFLAGS => '$(ARCHFLAGS) -framework Foundation -framework Cocoa' },
    XS           => { 'WoxMR.xs' => 'WoxMR.c' },
    clean        => { FILES => 'WoxMR.c wox_mr.o' },
);

sub MY::postamble {
    return <<'END';
wox_mr.o: wox_mr.m
	$(CC) -c -fobjc-arc -fPIC $(ARCHFLAGS) $(CCCDLFLAGS) $(INC) wox_mr.m -o wox_mr.o
END
}

