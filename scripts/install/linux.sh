#!/usr/bin/env bash
# Parse the whole installer before running any part of a streamed download.
(
  set -euo pipefail

  readonly repo=https://github.com/Wox-launcher/Wox
  readonly assets_url=https://raw.githubusercontent.com/Wox-launcher/Wox/master/assets
  readonly desktop_name=io.github.WoxLauncher.Wox.desktop
  readonly icon_name=io.github.WoxLauncher.Wox.png

  die() {
    printf '%s\n' "$*" >&2
    exit 1
  }

  exists() {
    [[ -e "$1" || -L "$1" ]]
  }

  download_verified() {
    local asset=$1 expected actual
    expected=$(awk -v asset="$asset" '$2 == asset { print $1 }' "$work/SHA256SUMS")
    [[ "$expected" =~ ^[a-f0-9]{64}$ ]] || die "Missing or invalid SHA-256 checksum for $asset."
    "${curl_https[@]}" --output "$work/$asset" "$release/$asset" || die "Could not download $asset."
    actual=$(sha256sum < "$work/$asset")
    [[ "${actual%% *}" == "$expected" ]] || die "SHA-256 mismatch for $asset."
  }

  [[ "$(uname -s)" == Linux && "$(uname -m)" == x86_64 ]] || die "Wox requires Linux x64."
  [[ "$EUID" != 0 ]] || die "Run this installer as your desktop user."
  for tool in curl sha256sum ldd; do
    command -v "$tool" >/dev/null || die "This installer requires $tool."
  done

  data_home=${XDG_DATA_HOME:-}
  [[ "$data_home" == /* ]] || data_home="$HOME/.local/share"
  install_dir="$data_home/wox"
  desktop_file="$data_home/applications/$desktop_name"
  icon_file="$data_home/icons/hicolor/256x256/apps/$icon_name"
  existing="Wox is already installed. Use its existing installation method or update it from Wox."
  if exists "$install_dir" || exists "$desktop_file" || exists "$icon_file" || command -v wox >/dev/null; then
    die "$existing"
  fi
  IFS=: read -r -a desktop_roots <<< "${XDG_DATA_DIRS:-/usr/local/share:/usr/share}"
  for root in "$HOME/.local/share" "${desktop_roots[@]}"; do
    if [[ "$root" == /* ]] && exists "$root/applications/$desktop_name"; then
      die "$existing"
    fi
  done

  work=$(mktemp -d)
  desktop_staging=
  install_created=0
  icon_staging=
  cleanup() {
    # The desktop entry is linked last. An interrupted install can remove only files it created.
    if [[ -z "$desktop_staging" || ! "$desktop_staging" -ef "$desktop_file" ]]; then
      if [[ -n "$icon_staging" && "$icon_staging" -ef "$icon_file" ]]; then
        rm -f -- "$icon_file"
      fi
      if (( install_created )); then
        rm -f -- "$install_dir/wox"
        rmdir -- "$install_dir" || true
      fi
    fi
    [[ -z "$desktop_staging" ]] || rm -f -- "$desktop_staging"
    [[ -z "$icon_staging" ]] || rm -f -- "$icon_staging"
    rm -rf -- "$work"
  }
  trap cleanup EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM

  curl_https=(curl --disable --fail --silent --show-error --location --proto '=https' --proto-redir '=https' --connect-timeout 15 --max-time 300)
  latest=$("${curl_https[@]}" --head --output /dev/null --write-out '%{url_effective}' "$repo/releases/latest") ||
    die "Could not look up the latest Wox release."
  tag=${latest#"$repo/releases/tag/"}
  [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "Could not identify the latest stable Wox release."
  release="$repo/releases/download/$tag"
  "${curl_https[@]}" --output "$work/SHA256SUMS" "$release/SHA256SUMS" ||
    die "Could not download checksums for Wox $tag. See $repo/releases."

  binary=wox-linux-amd64
  download_verified "$binary"
  chmod 700 -- "$work/$binary"
  # ldd may exit successfully even if a shared library is missing.
  if ! dependencies=$(LC_ALL=C ldd "$work/$binary" 2>&1) || [[ "$dependencies" == *"not found"* ]]; then
    printf 'Wox cannot load its required system libraries:\n%s\n' "$dependencies" >&2
    die "Try the AppImage from $release/wox-linux-amd64.AppImage."
  fi
  "${curl_https[@]}" --output "$work/$desktop_name" "$assets_url/linux/wox.desktop" || die "Could not download $desktop_name."
  "${curl_https[@]}" --output "$work/$icon_name" "$assets_url/app.png" || die "Could not download $icon_name."

  exec_path="$install_dir/wox"
  # The desktop file and its quoted Exec argument each apply a layer of escaping.
  exec_path=${exec_path//\\/\\\\\\\\}
  exec_path=${exec_path//\"/\\\\\"}
  exec_path=${exec_path//\$/\\\\\$}
  exec_path=${exec_path//\`/\\\\\`}
  exec_path=${exec_path//%/%%}
  [[ "$exec_path" != *$'\n'* && "$exec_path" != *$'\r'* && "$exec_path" != *=* ]] ||
    die "The install path contains characters that the application menu cannot use."

  mkdir -p -- "$data_home/applications" "${icon_file%/*}"
  mkdir -- "$install_dir" || die "Could not create $install_dir. $existing"
  install_created=1
  install -m 755 -- "$work/$binary" "$install_dir/wox"

  icon_staging=$(mktemp "${icon_file%/*}/.wox-install.XXXXXXXX")
  install -m 644 -- "$work/$icon_name" "$icon_staging"
  ln -T -- "$icon_staging" "$icon_file" || die "Could not install the Wox icon. $existing"

  desktop_staging=$(mktemp "$data_home/applications/.wox-install.XXXXXXXX")
  while IFS= read -r line; do
    if [[ "$line" == Exec=* ]]; then
      printf 'Exec="%s" %%U\n' "$exec_path"
    else
      printf '%s\n' "$line"
    fi
  done < "$work/$desktop_name" > "$desktop_staging"
  chmod 644 -- "$desktop_staging"
  ln -T -- "$desktop_staging" "$desktop_file" || die "Could not create the Wox application entry. $existing"
  printf 'Installed Wox %s. Open Wox from your application menu.\n' "$tag"
)
