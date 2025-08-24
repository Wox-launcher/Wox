#!/usr/bin/perl
use strict;
use warnings;
use JSON;
use FindBin;
use File::Basename qw(dirname);
use lib dirname(__FILE__);

use WoxMR;

my $cmd = $ARGV[0] || 'get';
if ($cmd eq 'get') {
    my $json = WoxMR::get_now_playing_json();
    if (!defined $json || $json eq '') {
        die "MediaRemote not available or no now playing info";
    }
    print $json;
} elsif ($cmd eq 'toggle') {
    my $ok = WoxMR::toggle();
    if (!$ok) {
        # Try an extra get to ensure session is alive
        my $json = WoxMR::get_now_playing_json();
        if (defined $json && $json ne '') {
            $ok = WoxMR::toggle();
        }
    }
    if (!$ok) { die "toggle failed"; }
    print '{"ok":true}';
} else {
    die "Unknown command: $cmd";
}

