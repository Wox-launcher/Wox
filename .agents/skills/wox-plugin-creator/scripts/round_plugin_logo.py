#!/usr/bin/env python3
"""Apply an antialiased rounded-square mask to a square plugin logo."""

import argparse
from pathlib import Path

from PIL import Image, ImageChops, ImageDraw, ImageOps


def round_logo(image, size, radius):
    """Resize the artwork and intersect its alpha with an antialiased mask."""
    if image.width != image.height:
        raise ValueError("The artwork must be square; crop it before masking")
    image = image.convert("RGBA").resize((size, size), Image.Resampling.LANCZOS)
    # Draw at higher resolution so the curved boundary has smooth partial pixels.
    scale = 4
    mask = Image.new("L", (size * scale, size * scale), 0)
    ImageDraw.Draw(mask).rounded_rectangle(
        (0, 0, size * scale - 1, size * scale - 1),
        radius=round(size * scale * radius),
        fill=255,
    )
    mask = mask.resize((size, size), Image.Resampling.LANCZOS)
    image.putalpha(ImageChops.multiply(image.getchannel("A"), mask))
    return image


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input", type=Path)
    parser.add_argument(
        "output", type=Path, help="New PNG path; existing files are not overwritten"
    )
    parser.add_argument(
        "--size", type=int, default=512, help="Output width and height in pixels"
    )
    parser.add_argument(
        "--radius",
        type=float,
        default=0.23,
        help="Corner radius as a fraction of the side",
    )
    parser.add_argument(
        "--crop", type=int, nargs=4, metavar=("LEFT", "TOP", "RIGHT", "BOTTOM")
    )
    args = parser.parse_args()
    if args.size < 4 or not 0 < args.radius <= 0.5:
        parser.error("size must be at least 4 and radius must be in (0, 0.5]")
    if args.output.suffix.lower() != ".png":
        parser.error("output must be a PNG file")

    with Image.open(args.input) as source:
        image = ImageOps.exif_transpose(source)
        if args.crop:
            left, top, right, bottom = args.crop
            if not (
                0 <= left < right <= image.width and 0 <= top < bottom <= image.height
            ):
                parser.error("crop must be inside the input image")
            image = image.crop((left, top, right, bottom))
        try:
            result = round_logo(image, args.size, args.radius)
        except ValueError as exc:
            parser.error(str(exc))

    with args.output.open("xb") as output:
        result.save(output, format="PNG")
    print(f"Saved {args.output} ({args.size} x {args.size})")


if __name__ == "__main__":
    main()
