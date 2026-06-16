#!/usr/bin/env python3
import argparse
import json
import subprocess
import sys
import urllib.parse
import urllib.request
from urllib.error import HTTPError, URLError

API_BASE = "https://api.iconify.design"


def http_get_json(url: str) -> dict:
    return json.loads(http_get_text(url))


def http_get_text(url: str) -> str:
    try:
        with urllib.request.urlopen(url) as response:
            return response.read().decode("utf-8")
    except (HTTPError, URLError):
        try:
            result = subprocess.run(
                ["curl", "-fsSL", url],
                check=True,
                capture_output=True,
                text=True,
            )
            return result.stdout
        except (FileNotFoundError, subprocess.CalledProcessError) as error:
            raise SystemExit(
                "Unable to reach Iconify API. Check network access or rerun outside the sandbox."
            ) from error


def build_svg_url(icon: str, height: int, color: str) -> str:
    if ":" not in icon:
        raise SystemExit(f"Invalid icon name: {icon}. Expected prefix:name")

    prefix, name = icon.split(":", 1)
    path = f"/{urllib.parse.quote(prefix)}/{urllib.parse.quote(name)}.svg"
    params: dict[str, str] = {"height": str(height)}
    if color:
        params["color"] = color

    return f"{API_BASE}{path}?{urllib.parse.urlencode(params)}"


def build_search_url(args: argparse.Namespace) -> str:
    limit = max(32, min(args.limit, 999))
    params: dict[str, str] = {
        "query": args.query,
        "limit": str(limit),
        "start": str(max(0, args.start)),
    }
    if args.prefixes:
        params["prefixes"] = args.prefixes
    if args.category:
        params["category"] = args.category

    return f"{API_BASE}/search?{urllib.parse.urlencode(params)}"


def palette_label(is_palette: bool) -> str:
    return "color" if is_palette else "monotone"


def should_keep_icon(args: argparse.Namespace, collections: dict, icon: str) -> bool:
    prefix = icon.split(":", 1)[0]
    collection = collections.get(prefix, {})
    is_palette = bool(collection.get("palette"))

    if args.palette == "any":
        return True
    if args.palette == "color":
        return is_palette
    return not is_palette


def format_collection_name(collections: dict, icon: str) -> str:
    prefix = icon.split(":", 1)[0]
    return collections.get(prefix, {}).get("name", prefix)


def search_icons(args: argparse.Namespace) -> int:
    payload = http_get_json(build_search_url(args))
    icons = payload.get("icons", [])
    collections = payload.get("collections", {})
    filtered_icons = [
        icon for icon in icons if should_keep_icon(args, collections, icon)
    ]

    if args.json:
        print(
            json.dumps(
                {
                    "icons": filtered_icons,
                    "total": payload.get("total", len(filtered_icons)),
                    "collections": collections,
                    "request": payload.get("request", {}),
                },
                ensure_ascii=False,
                indent=2,
            )
        )
        return 0

    if not filtered_icons:
        print("No icons matched the filters.", file=sys.stderr)
        return 1

    print(f"Query: {args.query}")
    print(f"Results: {len(filtered_icons)} / {len(icons)} returned")
    print("")

    for index, icon in enumerate(filtered_icons, start=1):
        prefix = icon.split(":", 1)[0]
        collection = collections.get(prefix, {})
        category = collection.get("category", "")
        palette = palette_label(bool(collection.get("palette")))
        svg_url = build_svg_url(icon, args.height, args.color)
        print(f"{index:02d}. {icon}")
        print(f"    family: {format_collection_name(collections, icon)}")
        print(f"    style:  {palette}")
        if category:
            print(f"    group:  {category}")
        print(f"    svg:    {svg_url}")

    return 0


def wrap_svg(svg: str, output_format: str, const_name: str) -> str:
    if output_format == "raw":
        return svg
    if output_format == "ts":
        escaped = svg.replace("`", "\\`")
        return f"export const {const_name} = `{escaped}`;\n"
    if output_format == "py":
        escaped = svg.replace('"""', '\\"""')
        return f'{const_name} = """{escaped}"""\n'
    raise SystemExit(f"Unsupported format: {output_format}")


def fetch_icon(args: argparse.Namespace) -> int:
    svg = http_get_text(build_svg_url(args.icon, args.height, args.color))
    content = wrap_svg(svg, args.format, args.const_name)

    if args.out:
        with open(args.out, "w", encoding="utf-8") as file:
            file.write(content)
    else:
        print(content, end="" if content.endswith("\n") else "\n")

    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Search and fetch Iconify SVG icons")
    subparsers = parser.add_subparsers(dest="command", required=True)

    search_parser = subparsers.add_parser("search", help="Search icons by keyword")
    search_parser.add_argument("query", help="Search keyword")
    search_parser.add_argument(
        "--limit", type=int, default=32, help="Result size, clamped to 32-999"
    )
    search_parser.add_argument("--start", type=int, default=0, help="Pagination offset")
    search_parser.add_argument(
        "--prefixes", default="", help="Comma-separated collection prefixes"
    )
    search_parser.add_argument(
        "--category", default="", help="Category filter passed to Iconify"
    )
    search_parser.add_argument(
        "--palette",
        choices=["any", "color", "monotone"],
        default="any",
        help="Filter collections by color palette support",
    )
    search_parser.add_argument(
        "--height", type=int, default=48, help="Preview SVG height"
    )
    search_parser.add_argument(
        "--color", default="", help="Optional SVG color, for example #ffffff"
    )
    search_parser.add_argument(
        "--json", action="store_true", help="Print raw JSON response"
    )
    search_parser.set_defaults(func=search_icons)

    fetch_parser = subparsers.add_parser("fetch", help="Fetch a specific icon as SVG")
    fetch_parser.add_argument("icon", help="Icon name in prefix:name format")
    fetch_parser.add_argument("--height", type=int, default=48, help="SVG height")
    fetch_parser.add_argument(
        "--color", default="", help="Optional SVG color, for example #ffffff"
    )
    fetch_parser.add_argument(
        "--format",
        choices=["raw", "ts", "py"],
        default="raw",
        help="Output raw SVG or a ready-to-paste constant",
    )
    fetch_parser.add_argument(
        "--const-name", default="ICON_SVG", help="Constant name for ts/py formats"
    )
    fetch_parser.add_argument(
        "--out", default="", help="Write output to a file instead of stdout"
    )
    fetch_parser.set_defaults(func=fetch_icon)

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    return args.func(args)


if __name__ == "__main__":
    raise SystemExit(main())
