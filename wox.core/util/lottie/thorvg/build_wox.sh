#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "$0")" && pwd)"
goos="$(go env GOOS)"
goarch="$(go env GOARCH)"
build_dir="$root_dir/build/$goos-$goarch"
export CC="${CC:-$(go env CC)}"
export CXX="${CXX:-$(go env CXX)}"

if ! command -v meson >/dev/null 2>&1 || ! command -v ninja >/dev/null 2>&1; then
    core_dir="$(cd "$root_dir/../../.." && pwd)"
    tools_dir="$core_dir/.tmp/thorvg-build-tools"

    if [[ "$goos" == "windows" ]]; then
        tools_bin_dir="$tools_dir/Scripts"
        tools_python="$tools_bin_dir/python.exe"
    else
        tools_bin_dir="$tools_dir/bin"
        tools_python="$tools_bin_dir/python"
    fi

    # Keep build tools project-local so contributors do not need to manage Meson or Ninja on PATH.
    export PATH="$tools_bin_dir:$PATH"
    if ! command -v meson >/dev/null 2>&1 || ! command -v ninja >/dev/null 2>&1; then
        if command -v py >/dev/null 2>&1; then
            python_cmd=(py -3)
        elif command -v python3 >/dev/null 2>&1; then
            python_cmd=(python3)
        elif command -v python >/dev/null 2>&1; then
            python_cmd=(python)
        else
            echo "Python 3 is required to install the ThorVG build tools" >&2
            exit 1
        fi

        echo "Installing ThorVG build tools in $tools_dir"
        "${python_cmd[@]}" -m venv "$tools_dir"
        "$tools_python" -m pip install --disable-pip-version-check meson==1.8.3 ninja==1.13.0
    fi
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
