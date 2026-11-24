#!/usr/bin/env bash
# Parse the whole installer before running any part of a streamed download.
(
  set -euo pipefail

  readonly repo=https://github.com/Wox-launcher/Wox

  die() {
    printf '%s\n' "$*" >&2
    exit 1
  }

  check_existing() {
    if [[ -e "$target" || -L "$target" || -e /Applications/Wox.app || -L /Applications/Wox.app ]]; then
      die "Wox is already installed. Use its existing installation method or update it from Wox."
    fi
  }

  [[ "$(uname -s)" == Darwin ]] || die "This installer requires macOS."
  [[ "$EUID" != 0 ]] || die "Run this installer as your desktop user."
  version=$(sw_vers -productVersion)
  [[ "${version%%.*}" -ge 12 ]] || die "Wox requires macOS 12 or later."
  case "$(uname -m)" in
    arm64) arch=arm64 ;;
    x86_64)
      arch=amd64
      if [[ "$(sysctl -n sysctl.proc_translated 2>/dev/null || true)" == 1 ]]; then
        arch=arm64
      fi
      ;;
    *) die "Wox requires an Intel or Apple Silicon Mac." ;;
  esac

  applications="$HOME/Applications"
  target="$applications/Wox.app"
  check_existing

  work=$(mktemp -d)
  mount_point="$work/mount"
  mounted=0
  staging=
  cleanup() {
    if (( mounted )) && ! hdiutil detach "$mount_point" >/dev/null; then
      printf 'Could not detach the Wox disk image at %s.\n' "$mount_point" >&2
    else
      rm -rf -- "$work"
    fi
    [[ -z "$staging" ]] || rm -rf -- "$staging"
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

  asset="wox-mac-$arch.dmg"
  dmg="$work/$asset"
  expected=$(awk -v asset="$asset" '$2 == asset { print $1 }' "$work/SHA256SUMS")
  [[ "$expected" =~ ^[a-f0-9]{64}$ ]] || die "Missing or invalid SHA-256 checksum for $asset."
  "${curl_https[@]}" --output "$dmg" "$release/$asset" || die "Could not download $asset."
  actual=$(shasum -a 256 < "$dmg")
  [[ "${actual%% *}" == "$expected" ]] || die "SHA-256 mismatch for $asset."
  spctl --assess --type install "$dmg" || die "macOS could not verify the Wox disk image."

  mkdir -- "$mount_point"
  hdiutil attach "$dmg" -readonly -nobrowse -mountpoint "$mount_point" >/dev/null
  mounted=1
  codesign --verify --deep --strict -R='anchor apple generic and identifier "com.github.wox"' "$mount_point/Wox.app" ||
    die "macOS could not verify Wox.app."

  mkdir -p -- "$applications"
  staging=$(mktemp -d "$applications/.wox-install.XXXXXXXX")
  ditto "$mount_point/Wox.app" "$staging/Wox.app"
  hdiutil detach "$mount_point" >/dev/null
  mounted=0

  check_existing
  mv -n -- "$staging/Wox.app" "$applications/"
  [[ ! -e "$staging/Wox.app" ]] || die "A Wox installation already exists at $target."
  printf 'Installed Wox %s. Open %s to get started.\n' "$tag" "$target"
)
