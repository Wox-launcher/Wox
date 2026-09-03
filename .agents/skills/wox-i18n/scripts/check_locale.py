#!/usr/bin/env python3
"""Check a Wox locale against the authoritative en_US catalog."""

import argparse
import json
import re
import sys
from collections import Counter
from pathlib import Path


PLACEHOLDER = re.compile(
    r"\{[^{}]+\}|%(?:\[[0-9]+\])?[#0\- +]*(?:[0-9]+|\*)?(?:\.(?:[0-9]+|\*))?[bcdoOqxXUeEfFgGsptvT%]"
)
MAX_DETAILS = 50


def load_catalog(path: Path) -> dict[str, str]:
    """Load a flat JSON catalog and reject duplicate or non-string entries."""

    def reject_duplicates(pairs: list[tuple[str, object]]) -> dict[str, object]:
        result: dict[str, object] = {}
        for key, value in pairs:
            if key in result:
                raise ValueError(f"duplicate key: {key}")
            result[key] = value
        return result

    with path.open(encoding="utf-8") as catalog_file:
        catalog = json.load(catalog_file, object_pairs_hook=reject_duplicates)
    if not isinstance(catalog, dict) or any(
        not isinstance(key, str) or not isinstance(value, str)
        for key, value in catalog.items()
    ):
        raise ValueError("catalog must be a flat object containing string values")
    return catalog


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("locale", help="Locale code such as ko_KR, or a locale JSON path")
    args = parser.parse_args()

    repo_root = Path(__file__).resolve().parents[4]
    lang_dir = repo_root / "wox.core" / "resource" / "lang"
    locale_path = Path(args.locale)
    if locale_path.suffix != ".json":
        locale_path = lang_dir / f"{args.locale}.json"
    elif not locale_path.is_absolute():
        locale_path = repo_root / locale_path

    base_path = lang_dir / "en_US.json"
    try:
        base = load_catalog(base_path)
        locale = load_catalog(locale_path)
    except (OSError, json.JSONDecodeError, ValueError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 2

    missing = [key for key in base if key not in locale]
    extra = [key for key in locale if key not in base]
    empty = [
        key
        for key, value in locale.items()
        if not value.strip() and base.get(key, "").strip()
    ]
    placeholder_mismatches = [
        key
        for key in base
        if key in locale
        if Counter(PLACEHOLDER.findall(base[key]))
        != Counter(PLACEHOLDER.findall(locale[key]))
    ]
    identical = (
        [
            key
            for key in base
            if key in locale
            if base[key] == locale[key] and base[key].strip()
        ]
        if locale_path.resolve() != base_path.resolve()
        else []
    )

    for label, keys in (
        ("missing", missing),
        ("extra", extra),
        ("empty", empty),
        ("placeholder mismatch", placeholder_mismatches),
        ("identical to English (review)", identical),
    ):
        print(f"{label}: {len(keys)}")
        for key in keys[:MAX_DETAILS]:
            print(f"  {key}")
        if len(keys) > MAX_DETAILS:
            print(f"  ... {len(keys) - MAX_DETAILS} more")

    failed = bool(missing or extra or empty or placeholder_mismatches)
    print(f"result: {'FAIL' if failed else 'PASS'}")
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
