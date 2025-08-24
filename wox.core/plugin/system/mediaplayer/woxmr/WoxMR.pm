package WoxMR;
use strict;
use warnings;
use File::Basename qw(dirname);
use File::Spec;
use Cwd qw(abs_path);
use DynaLoader ();
our @ISA = qw(DynaLoader);
our $VERSION = '0.01';

# Try standard XSLoader path first (auto/WoxMR/WoxMR.bundle)
eval {
    require XSLoader;
    XSLoader::load('WoxMR', $VERSION);
    1;
} or do {
    # Fallback: load flat bundle from same directory, ensure absolute path for Hardened Runtime
    my $dir = abs_path(dirname(__FILE__));
    my $bundle = "woxmr.bundle";
    my $path = File::Spec->catfile($dir, $bundle);

    my $libref = DynaLoader::dl_load_file($path, 0x01)
        or die "Failed to load $path: " . DynaLoader::dl_error();
    my $sym = DynaLoader::dl_find_symbol($libref, 'boot_WoxMR')
        or die "Failed to find boot_WoxMR in $path: " . DynaLoader::dl_error();
    my $xs = DynaLoader::dl_install_xsub('WoxMR::bootstrap', $sym);
    WoxMR::bootstrap();
};

1;

