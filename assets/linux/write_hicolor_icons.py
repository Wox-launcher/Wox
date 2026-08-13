#!/usr/bin/env python3
"""Write hicolor PNG icons at the sizes software centers extract from packages."""

from __future__ import annotations

import binascii
import pathlib
import struct
import sys
import zlib

HICOLOR_SIZES = (64, 128, 256)


def png_chunk(kind: bytes, data: bytes) -> bytes:
    payload = kind + data
    return struct.pack(">I", len(data)) + payload + struct.pack(">I", binascii.crc32(payload) & 0xFFFFFFFF)


def paeth(left: int, up: int, up_left: int) -> int:
    estimate = left + up - up_left
    distance_left = abs(estimate - left)
    distance_up = abs(estimate - up)
    distance_up_left = abs(estimate - up_left)
    if distance_left <= distance_up and distance_left <= distance_up_left:
        return left
    if distance_up <= distance_up_left:
        return up
    return up_left


def decode_rgba_png(path: pathlib.Path) -> tuple[int, int, bytearray]:
    """Decode an 8-bit RGBA PNG into raw pixels."""
    data = path.read_bytes()
    if not data.startswith(b"\x89PNG\r\n\x1a\n"):
        raise SystemExit(f"{path} is not a PNG file")

    width = height = None
    idat = bytearray()
    offset = 8
    while offset < len(data):
        length = struct.unpack(">I", data[offset : offset + 4])[0]
        offset += 4
        kind = data[offset : offset + 4]
        offset += 4
        payload = data[offset : offset + length]
        offset += length + 4
        if kind == b"IHDR":
            width, height, bit_depth, color_type, compression, filter_method, interlace = struct.unpack(">IIBBBBB", payload)
            if bit_depth != 8 or color_type != 6 or compression or filter_method or interlace:
                raise SystemExit(f"{path} must be an 8-bit non-interlaced RGBA PNG")
        elif kind == b"IDAT":
            idat.extend(payload)
        elif kind == b"IEND":
            break

    if width is None or height is None:
        raise SystemExit(f"{path} is missing a PNG header")

    raw = zlib.decompress(bytes(idat))
    rgba = bytearray(width * height * 4)
    previous = bytearray(width * 4)
    source_offset = 0
    target_offset = 0
    for _ in range(height):
        filter_type = raw[source_offset]
        source_offset += 1
        row = bytearray(raw[source_offset : source_offset + width * 4])
        source_offset += width * 4
        for index in range(width * 4):
            left = row[index - 4] if index >= 4 else 0
            up = previous[index]
            up_left = previous[index - 4] if index >= 4 else 0
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
        rgba[target_offset : target_offset + width * 4] = row
        target_offset += width * 4
        previous = row
    return width, height, rgba


def encode_rgba_png(width: int, height: int, rgba: bytes) -> bytes:
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


def box_downsample(rgba: bytearray, width: int, height: int, factor: int) -> bytearray:
    # Integer box filter keeps opaque icon edges clean without extra image libraries.
    out_width = width // factor
    out_height = height // factor
    output = bytearray(out_width * out_height * 4)
    area = factor * factor
    for out_y in range(out_height):
        for out_x in range(out_width):
            red = green = blue = alpha = 0
            for dy in range(factor):
                row = ((out_y * factor) + dy) * width
                for dx in range(factor):
                    index = (row + (out_x * factor) + dx) * 4
                    red += rgba[index]
                    green += rgba[index + 1]
                    blue += rgba[index + 2]
                    alpha += rgba[index + 3]
            offset = (out_y * out_width + out_x) * 4
            output[offset : offset + 4] = bytes((red // area, green // area, blue // area, alpha // area))
    return output


def write_hicolor_icons(source_png: pathlib.Path, destination_root: pathlib.Path, icon_file: str) -> None:
    """Write 64, 128, and 256 PNG icons so local .rpm/.deb installs show the app icon."""
    width, height, rgba = decode_rgba_png(source_png)
    if width != height:
        raise SystemExit(f"{source_png} must be square")
    for size in HICOLOR_SIZES:
        if width % size != 0:
            raise SystemExit(f"{source_png} ({width}x{height}) cannot be boxed down to {size}x{size}")
        sized = box_downsample(rgba, width, height, width // size) if size != width else bytearray(rgba)
        output_path = destination_root / f"{size}x{size}" / icon_file
        output_path.parent.mkdir(parents=True, exist_ok=True)
        output_path.write_bytes(encode_rgba_png(size, size, sized))


if __name__ == "__main__":
    if len(sys.argv) != 4:
        raise SystemExit(f"Usage: {pathlib.Path(sys.argv[0]).name} SOURCE_PNG DEST_ROOT ICON_FILE")
    write_hicolor_icons(pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2]), sys.argv[3])
