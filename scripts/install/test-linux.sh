#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
mkdir "$work/bin" "$work/release" "$work/home"
export HOME="$work/home" XDG_DATA_DIRS="$work/system" WOX_TEST_RELEASE="$work/release"
export WOX_TEST_ASSETS="$root/assets"
export PATH="$work/bin:/usr/bin:/bin"

printf '#!/bin/sh\nprintf "launched" > "$XDG_DATA_HOME/launched"\n' > "$work/release/wox-linux-amd64"
(cd "$work/release" && sha256sum --text wox-linux-amd64 > SHA256SUMS)

cat > "$work/bin/uname" <<'STUB'
#!/usr/bin/env bash
if [[ "$1" == -s ]]; then
  echo Linux
else
  echo "${WOX_TEST_ARCH:-x86_64}"
fi
STUB
cat > "$work/bin/curl" <<'STUB'
#!/usr/bin/env bash
set -eu
output=
while (( $# > 1 )); do
  if [[ "$1" == --output ]]; then
    output=$2
    shift
  fi
  shift
done
case "$1" in
  https://github.com/Wox-launcher/Wox/releases/latest)
    printf '%s' 'https://github.com/Wox-launcher/Wox/releases/tag/v9.8.7'
    ;;
  https://github.com/Wox-launcher/Wox/releases/download/v9.8.7/*)
    asset=${1##*/}
    cp "$WOX_TEST_RELEASE/$asset" "$output"
    if [[ "$asset" == wox-linux-amd64 ]]; then
      case "${WOX_TEST_DOWNLOAD:-}" in
        interrupted) exit 18 ;;
        corrupt) printf 'unexpected content' >> "$output" ;;
      esac
    fi
    ;;
  https://raw.githubusercontent.com/Wox-launcher/Wox/master/assets/linux/wox.desktop)
    cp "$WOX_TEST_ASSETS/linux/wox.desktop" "$output"
    ;;
  https://raw.githubusercontent.com/Wox-launcher/Wox/master/assets/app.png)
    [[ "${WOX_TEST_DOWNLOAD:-}" != icon-unavailable ]] || exit 22
    cp "$WOX_TEST_ASSETS/app.png" "$output"
    ;;
  *) exit 22 ;;
esac
STUB
cat > "$work/bin/ldd" <<'STUB'
#!/usr/bin/env bash
if [[ "${WOX_TEST_LIBS:-}" == missing ]]; then
  echo 'libgtk-3.so.0 => not found'
fi
STUB
chmod +x "$work/bin/"*

installer="$root/scripts/install/linux.sh"
export XDG_DATA_HOME="$work/Desktop Data"
desktop="$XDG_DATA_HOME/applications/io.github.WoxLauncher.Wox.desktop"
icon="$XDG_DATA_HOME/icons/hicolor/256x256/apps/io.github.WoxLauncher.Wox.png"
if sed '/^)$/,$d' "$installer" | bash > "$work/output" 2>&1; then
  printf 'Expected a truncated installer to fail\n' >&2
  exit 1
fi
test ! -e "$XDG_DATA_HOME"
bash < "$installer"
test -x "$XDG_DATA_HOME/wox/wox"
cmp "$icon" "$WOX_TEST_ASSETS/app.png"
grep -Fx "Exec=\"$XDG_DATA_HOME/wox/wox\" %U" "$desktop"
if command -v gio >/dev/null; then
  gio launch "$desktop"
  for attempt in {1..20}; do
    [[ ! -f "$XDG_DATA_HOME/launched" ]] || break
    sleep 0.1
  done
  test -f "$XDG_DATA_HOME/launched"
fi

expect_failure() {
  local message=$1
  shift
  if "$@" > "$work/output" 2>&1; then
    printf 'Expected installer to fail with %s\n' "$message" >&2
    exit 1
  fi
  grep -F "$message" "$work/output"
}

expect_failure 'already installed' bash "$installer"
cmp "$XDG_DATA_HOME/wox/wox" "$work/release/wox-linux-amd64"
export XDG_DATA_HOME="$work/failed"
expect_failure 'requires Linux x64' env WOX_TEST_ARCH=aarch64 bash "$installer"
expect_failure 'Could not download wox-linux-amd64' env WOX_TEST_DOWNLOAD=interrupted bash "$installer"
expect_failure 'SHA-256 mismatch' env WOX_TEST_DOWNLOAD=corrupt bash "$installer"
expect_failure 'Could not download io.github.WoxLauncher.Wox.png' env WOX_TEST_DOWNLOAD=icon-unavailable bash "$installer"
expect_failure 'libgtk-3.so.0 => not found' env WOX_TEST_LIBS=missing bash "$installer"
grep -F 'wox-linux-amd64.AppImage' "$work/output"
test ! -e "$XDG_DATA_HOME"
