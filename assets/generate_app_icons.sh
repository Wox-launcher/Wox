#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_root="$(cd "$script_dir/.." && pwd -P)"
source_png="$script_dir/app.png"

python_bin="${PYTHON:-}"
if [[ -z "$python_bin" ]]; then
  if command -v python3 >/dev/null 2>&1; then
    python_bin="python3"
  elif command -v python >/dev/null 2>&1; then
    python_bin="python"
  else
    echo "python3 or python is required" >&2
    exit 1
  fi
fi

if [[ ! -f "$source_png" ]]; then
  echo "Missing source icon: $source_png" >&2
  exit 1
fi

echo "[icons] Source: $source_png"
echo "[icons] Resizing image resources with $python_bin..."
"$python_bin" - "$source_png" "$repo_root" <<'PY'
import binascii
import math
import os
import struct
import sys
import zlib


source_png = sys.argv[1]
repo_root = sys.argv[2]
assets_dir = os.path.dirname(source_png)


def log(message):
    print(f"[icons] {message}", flush=True)


def rel(path):
    return os.path.relpath(path, repo_root).replace(os.sep, "/")


def png_chunk(kind, data):
    payload = kind + data
    return struct.pack(">I", len(data)) + payload + struct.pack(">I", binascii.crc32(payload) & 0xFFFFFFFF)


def paeth(left, up, up_left):
    estimate = left + up - up_left
    distance_left = abs(estimate - left)
    distance_up = abs(estimate - up)
    distance_up_left = abs(estimate - up_left)
    if distance_left <= distance_up and distance_left <= distance_up_left:
        return left
    if distance_up <= distance_up_left:
        return up
    return up_left


def decode_png(path):
    # Decode common 8-bit non-interlaced PNG files into straight RGBA bytes.
    with open(path, "rb") as handle:
        data = handle.read()

    if not data.startswith(b"\x89PNG\r\n\x1a\n"):
        raise SystemExit(f"{path} is not a PNG file")

    width = None
    height = None
    bit_depth = None
    color_type = None
    palette = None
    transparency = b""
    idat = bytearray()
    offset = 8

    while offset < len(data):
        length = struct.unpack(">I", data[offset : offset + 4])[0]
        offset += 4
        kind = data[offset : offset + 4]
        offset += 4
        payload = data[offset : offset + length]
        offset += length
        offset += 4

        if kind == b"IHDR":
            width, height, bit_depth, color_type, compression, filter_method, interlace = struct.unpack(">IIBBBBB", payload)
            if bit_depth != 8:
                raise SystemExit("Only 8-bit PNG files are supported")
            if compression != 0 or filter_method != 0 or interlace != 0:
                raise SystemExit("Only non-interlaced standard PNG files are supported")
        elif kind == b"PLTE":
            palette = [tuple(payload[index : index + 3]) for index in range(0, len(payload), 3)]
        elif kind == b"tRNS":
            transparency = payload
        elif kind == b"IDAT":
            idat.extend(payload)
        elif kind == b"IEND":
            break

    if width is None or height is None or color_type is None:
        raise SystemExit("PNG header is missing")

    channels_by_type = {0: 1, 2: 3, 3: 1, 4: 2, 6: 4}
    if color_type not in channels_by_type:
        raise SystemExit(f"Unsupported PNG color type: {color_type}")
    if color_type == 3 and palette is None:
        raise SystemExit("Palette PNG is missing PLTE chunk")

    channels = channels_by_type[color_type]
    row_bytes = width * channels
    raw = zlib.decompress(bytes(idat))
    rgba = bytearray(width * height * 4)
    previous = bytearray(row_bytes)
    source_offset = 0
    target_offset = 0

    for _ in range(height):
        filter_type = raw[source_offset]
        source_offset += 1
        row = bytearray(raw[source_offset : source_offset + row_bytes])
        source_offset += row_bytes

        for index in range(row_bytes):
            left = row[index - channels] if index >= channels else 0
            up = previous[index]
            up_left = previous[index - channels] if index >= channels else 0
            if filter_type == 1:
                row[index] = (row[index] + left) & 0xFF
            elif filter_type == 2:
                row[index] = (row[index] + up) & 0xFF
            elif filter_type == 3:
                row[index] = (row[index] + ((left + up) >> 1)) & 0xFF
            elif filter_type == 4:
                row[index] = (row[index] + paeth(left, up, up_left)) & 0xFF
            elif filter_type != 0:
                raise SystemExit(f"Unsupported PNG row filter: {filter_type}")

        if color_type == 6:
            rgba[target_offset : target_offset + width * 4] = row
            target_offset += width * 4
        elif color_type == 2:
            for index in range(0, row_bytes, 3):
                rgba[target_offset : target_offset + 4] = bytes((row[index], row[index + 1], row[index + 2], 255))
                target_offset += 4
        elif color_type == 0:
            for value in row:
                rgba[target_offset : target_offset + 4] = bytes((value, value, value, 255))
                target_offset += 4
        elif color_type == 4:
            for index in range(0, row_bytes, 2):
                rgba[target_offset : target_offset + 4] = bytes((row[index], row[index], row[index], row[index + 1]))
                target_offset += 4
        elif color_type == 3:
            for value in row:
                red, green, blue = palette[value]
                alpha = transparency[value] if value < len(transparency) else 255
                rgba[target_offset : target_offset + 4] = bytes((red, green, blue, alpha))
                target_offset += 4

        previous = row

    return width, height, rgba


def encode_png(width, height, rgba):
    scanlines = bytearray()
    row_width = width * 4
    for row in range(height):
        scanlines.append(0)
        start = row * row_width
        scanlines.extend(rgba[start : start + row_width])
    return b"".join(
        [
            b"\x89PNG\r\n\x1a\n",
            png_chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 6, 0, 0, 0)),
            png_chunk(b"IDAT", zlib.compress(bytes(scanlines), 9)),
            png_chunk(b"IEND", b""),
        ]
    )


def sinc(value):
    if value == 0:
        return 1.0
    value *= math.pi
    return math.sin(value) / value


def lanczos(value):
    value = abs(value)
    if value >= 3:
        return 0.0
    return sinc(value) * sinc(value / 3)


def build_weights(source_size, target_size):
    # Precompute Lanczos weights so every output size is sampled from the source PNG.
    scale = source_size / target_size
    filter_scale = max(1.0, scale)
    support = 3.0 * filter_scale
    weights = []

    for target in range(target_size):
        center = (target + 0.5) * scale - 0.5
        left = max(0, int(math.floor(center - support)))
        right = min(source_size - 1, int(math.ceil(center + support)))
        entries = []
        total = 0.0
        for source in range(left, right + 1):
            weight = lanczos((source - center) / filter_scale)
            if weight == 0:
                continue
            entries.append((source, weight))
            total += weight

        if not entries or total == 0:
            nearest = min(source_size - 1, max(0, int(round(center))))
            weights.append([(nearest, 1.0)])
            continue

        weights.append([(source, weight / total) for source, weight in entries])

    return weights


def clamp_byte(value):
    return max(0, min(255, int(round(value))))


def resize_rgba(source_rgba, source_width, source_height, target_width, target_height):
    # Resize in premultiplied alpha space to avoid dark fringes around transparent edges.
    if source_width == target_width and source_height == target_height:
        return bytearray(source_rgba)

    horizontal_weights = build_weights(source_width, target_width)
    vertical_weights = build_weights(source_height, target_height)
    intermediate = [(0.0, 0.0, 0.0, 0.0)] * (source_height * target_width)

    for y in range(source_height):
        row_offset = y * source_width * 4
        intermediate_offset = y * target_width
        for target_x, entries in enumerate(horizontal_weights):
            red = 0.0
            green = 0.0
            blue = 0.0
            alpha = 0.0
            for source_x, weight in entries:
                pixel_offset = row_offset + source_x * 4
                pixel_alpha = source_rgba[pixel_offset + 3] / 255.0
                red += source_rgba[pixel_offset] * pixel_alpha * weight
                green += source_rgba[pixel_offset + 1] * pixel_alpha * weight
                blue += source_rgba[pixel_offset + 2] * pixel_alpha * weight
                alpha += pixel_alpha * weight
            intermediate[intermediate_offset + target_x] = (red, green, blue, alpha)

    output = bytearray(target_width * target_height * 4)
    for target_y, entries in enumerate(vertical_weights):
        output_offset = target_y * target_width * 4
        for target_x in range(target_width):
            red = 0.0
            green = 0.0
            blue = 0.0
            alpha = 0.0
            for source_y, weight in entries:
                pixel = intermediate[source_y * target_width + target_x]
                red += pixel[0] * weight
                green += pixel[1] * weight
                blue += pixel[2] * weight
                alpha += pixel[3] * weight

            if alpha <= 0:
                output[output_offset : output_offset + 4] = b"\x00\x00\x00\x00"
            else:
                output[output_offset : output_offset + 4] = bytes(
                    (
                        clamp_byte(red / alpha),
                        clamp_byte(green / alpha),
                        clamp_byte(blue / alpha),
                        clamp_byte(alpha * 255),
                    )
                )
            output_offset += 4

    return output


def write_file(path, data):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "wb") as handle:
        handle.write(data)
    log(f"wrote {rel(path)}")


source_width, source_height, source_rgba = decode_png(source_png)
if source_width != source_height:
    raise SystemExit("assets/app.png must be square")

png_cache = {}


def app_icon_png(size):
    if size not in png_cache:
        log(f"resize {source_width}x{source_height} -> {size}x{size}")
        png_cache[size] = encode_png(size, size, resize_rgba(source_rgba, source_width, source_height, size, size))
    return png_cache[size]


def save_png(path, size):
    write_file(path, app_icon_png(size))


def save_ico(path, sizes):
    # Store PNG frames inside the ICO so Windows can choose the nearest native size.
    frames = [(size, app_icon_png(size)) for size in sizes]
    header = struct.pack("<HHH", 0, 1, len(frames))
    directory = bytearray()
    offset = 6 + len(frames) * 16
    payload = bytearray()
    for size, data in frames:
        encoded_size = 0 if size == 256 else size
        directory.extend(struct.pack("<BBBBHHII", encoded_size, encoded_size, 0, 0, 1, 32, len(data), offset))
        payload.extend(data)
        offset += len(data)
    write_file(path, header + directory + payload)


def save_icns(path):
    # ICNS chunks can carry PNG payloads for the modern macOS icon sizes.
    chunks = [
        (b"icp4", app_icon_png(16)),
        (b"icp5", app_icon_png(32)),
        (b"icp6", app_icon_png(64)),
        (b"ic07", app_icon_png(128)),
        (b"ic08", app_icon_png(256)),
        (b"ic09", app_icon_png(512)),
        (b"ic10", app_icon_png(1024)),
    ]
    total_length = 8 + sum(8 + len(data) for _, data in chunks)
    output = bytearray(b"icns" + struct.pack(">I", total_length))
    for kind, data in chunks:
        output.extend(kind + struct.pack(">I", 8 + len(data)) + data)
    write_file(path, bytes(output))


ico_sizes = [16, 20, 24, 32, 40, 48, 64, 128, 256]
mac_iconset_dir = os.path.join(repo_root, "wox.ui.flutter", "wox", "macos", "Runner", "Assets.xcassets", "AppIcon.appiconset")

save_ico(os.path.join(assets_dir, "app.ico"), ico_sizes)
save_icns(os.path.join(assets_dir, "mac", "app.icns"))

save_png(os.path.join(repo_root, "wox.core", "resource", "app.png"), 512)
save_ico(os.path.join(repo_root, "wox.core", "resource", "app.ico"), ico_sizes)
save_ico(os.path.join(repo_root, "wox.ui.flutter", "wox", "windows", "runner", "resources", "app_icon.ico"), ico_sizes)

for icon_size in [16, 32, 64, 128, 256, 512, 1024]:
    save_png(os.path.join(mac_iconset_dir, f"app_icon_{icon_size}.png"), icon_size)

log("image resources complete")
PY

version="$(sed -n 's/^const CURRENT_VERSION = "\(.*\)"/\1/p' "$repo_root/wox.core/updater/version.go")"
if [[ -z "$version" ]]; then
  echo "Unable to read CURRENT_VERSION from wox.core/updater/version.go" >&2
  exit 1
fi

echo "[icons] Generating Go Windows resource_windows.syso..."
(
  cd "$repo_root/wox.core"
  go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.5.0 \
    -o resource_windows.syso -64 -icon resource/app.ico \
    -manifest resource/wox.exe.manifest \
    -product-name "Wox" -description "Wox" -internal-name "Wox" -original-name "wox-windows-amd64.exe" \
    -product-version "$version" -file-version "$version" \
    -propagate-ver-strings
)

echo "[icons] Generated app icon resources from assets/app.png."
