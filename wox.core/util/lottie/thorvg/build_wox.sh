#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "$0")" && pwd)"
goos="$(go env GOOS)"
goarch="$(go env GOARCH)"
build_dir="$root_dir/build/$goos-$goarch"
export CC="${CC:-$(go env CC)}"
export CXX="${CXX:-$(go env CXX)}"

if ! command -v meson >/dev/null 2>&1; then
    echo "meson is required to build ThorVG" >&2
    exit 1
fi
if ! command -v ninja >/dev/null 2>&1; then
    echo "ninja is required to build ThorVG" >&2
    exit 1
fi

setup_args=(
    -Ddefault_library=static
    -Dengines=cpu
    -Dloaders=lottie,png
    -Dbindings=capi
    -Dstatic=true
    -Dthreads=false
    -Dpartial=false
    -Dfile=false
    -Dextra=
    -Dtools=
    -Dtests=false
    -Dlog=false
    --buildtype=release
)

if [[ -f "$build_dir/build.ninja" ]]; then
    meson setup "$build_dir" "$root_dir" --reconfigure "${setup_args[@]}"
else
    meson setup "$build_dir" "$root_dir" "${setup_args[@]}"
fi
meson compile -C "$build_dir"
