"""Generate SDK wire types from the current Go WoxView schema."""

from __future__ import annotations

import argparse
import json
import keyword
import re
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
TS_OUTPUT = ROOT / "wox.plugin.nodejs/types/woxview.d.ts"
PY_OUTPUT = ROOT / "wox.plugin.python/src/wox_plugin/models/woxview.py"


def load_schema() -> dict:
    """Read the current schema from Go instead of the checked-in golden fixture."""
    try:
        result = subprocess.run(
            ["go", "run", "./ui/woxview/cmd/schema"],
            cwd=ROOT / "wox.core",
            check=True,
            capture_output=True,
            text=True,
            encoding="utf-8",
        )
    except FileNotFoundError as error:
        raise RuntimeError("Go is required to generate WoxView SDK types") from error
    except subprocess.CalledProcessError as error:
        raise RuntimeError(
            f"Failed to generate the WoxView schema:\n{error.stderr.strip()}"
        ) from error
    return json.loads(result.stdout)


def type_name(name: str) -> str:
    """Map protocol definition names to public SDK names."""
    return "WoxView" + "".join(
        part[:1].upper() + part[1:] for part in re.split(r"[^A-Za-z0-9]+", name) if part
    )


class Generator:
    """Translate the generated schema's structural subset into SDK declarations."""

    def __init__(self, schema: dict):
        self.schema = schema
        self.python_objects: dict[str, tuple[dict, list[str]]] = {}

    def ts_type(self, schema: dict) -> str:
        if "$ref" in schema:
            return type_name(schema["$ref"].split("/")[-1])
        if "const" in schema:
            return json.dumps(schema["const"], ensure_ascii=False)
        if "enum" in schema:
            return " | ".join(json.dumps(v, ensure_ascii=False) for v in schema["enum"])
        if (
            schema.get("type") == "object"
            and "properties" in schema
            and "oneOf" in schema
        ):
            base = {key: value for key, value in schema.items() if key != "oneOf"}
            return f"({self.ts_type(base)} & ({' | '.join(self.ts_type(part) for part in schema['oneOf'])}))"
        for operator, joiner in (("anyOf", " | "), ("oneOf", " | "), ("allOf", " & ")):
            if operator in schema:
                return (
                    "("
                    + joiner.join(self.ts_type(part) for part in schema[operator])
                    + ")"
                )
        kind = schema.get("type")
        if kind == "object" or "properties" in schema:
            required = schema.get("required", [])
            return (
                "{ "
                + "; ".join(
                    f"{json.dumps(k)}{'' if k in required else '?'}: {self.ts_type(v)}"
                    for k, v in schema.get("properties", {}).items()
                )
                + " }"
            )
        if kind == "array":
            item = (
                "never"
                if schema.get("maxItems") == 0
                else self.ts_type(schema["items"])
            )
            return f"Array<{item}>"
        return {
            "string": "string",
            "integer": "number",
            "number": "number",
            "boolean": "boolean",
            "null": "null",
            None: "unknown",
        }[kind]

    def py_type(self, schema: dict, name: str) -> str:
        if "$ref" in schema:
            return type_name(schema["$ref"].split("/")[-1])
        if "const" in schema:
            return f"Literal[{schema['const']!r}]"
        if "enum" in schema:
            return "Literal[" + ", ".join(repr(v) for v in schema["enum"]) + "]"
        if (
            schema.get("type") == "object"
            and "properties" in schema
            and "oneOf" in schema
        ):
            variants = []
            for index, branch in enumerate(schema["oneOf"]):
                props = dict(schema["properties"])
                for key, value in branch.get("properties", {}).items():
                    if (
                        "const" in value
                        or value.get("type") == "null"
                        or key not in props
                    ):
                        props[key] = value
                merged = {
                    "type": "object",
                    "properties": props,
                    "required": list(
                        set(schema.get("required", []))
                        | set(branch.get("required", []))
                    ),
                }
                variants.append(self.py_type(merged, name + "Variant" + str(index + 1)))
            return " | ".join(variants)
        for operator in ("anyOf", "oneOf"):
            if operator in schema:
                parts = []
                for index, part in enumerate(schema[operator]):
                    node_type = part.get("properties", {}).get("type", {}).get("const")
                    child = (
                        type_name(node_type + "Node")
                        if node_type
                        else name + "Variant" + str(index + 1)
                    )
                    parts.append(self.py_type(part, child))
                return " | ".join(parts)
        if "allOf" in schema:
            # Schema intersections add validation constraints; Python has no intersection type.
            return next(
                (
                    self.py_type(p, name)
                    for p in schema["allOf"]
                    if "$ref" in p or p.get("type")
                ),
                "Any",
            )
        kind = schema.get("type")
        if kind == "object":
            self.python_objects[name] = (
                schema.get("properties", {}),
                schema.get("required", []),
            )
            return name
        if kind == "array":
            item = (
                "NoReturn"
                if schema.get("maxItems") == 0
                else self.py_type(schema["items"], name + "Item")
            )
            return f"list[{item}]"
        return {
            "string": "str",
            "integer": "int",
            "number": "float",
            "boolean": "bool",
            "null": "None",
            None: "Any",
        }[kind]

    def typescript(self) -> str:
        lines = ["// Generated from WoxView JSON Schema; do not edit."]
        for name, schema in self.schema["$defs"].items():
            lines.append(f"export type {type_name(name)} = {self.ts_type(schema)}")
        lines.append(f"export type WoxViewTree = {self.ts_type(self.schema)}")
        return "\n".join(lines) + "\n"

    def python(self) -> str:
        aliases = []
        for name, schema in self.schema["$defs"].items():
            public = type_name(name)
            expression = self.py_type(schema, public)
            if expression != public:
                aliases.append((public, expression))
        self.py_type(self.schema, "WoxViewTree")
        objects = []
        emitted = set()
        while True:
            pending = [
                (name, spec)
                for name, spec in self.python_objects.items()
                if name not in emitted
            ]
            if not pending:
                break
            for name, (props, required) in pending:
                emitted.add(name)
                fields = [
                    (
                        key,
                        self.py_type(value, name + type_name(key)[7:]),
                        key in required,
                    )
                    for key, value in props.items()
                ]
                if any(
                    not key.isidentifier() or keyword.iskeyword(key)
                    for key, _, _ in fields
                ):
                    if any(not req for _, _, req in fields):
                        raise ValueError(f"Unsupported mixed required keys in {name}")
                    body = ", ".join(f"{key!r}: {value}" for key, value, _ in fields)
                    # The wire handler is the only current non-identifier key.
                    if any(
                        value not in ("str", "int", "float", "bool")
                        for _, value, _ in fields
                    ):
                        raise ValueError(f"Unsupported forward reference in {name}")
                    objects.append(f'{name} = TypedDict("{name}", {{{body}}})')
                    continue
                mandatory = [(key, value) for key, value, req in fields if req]
                optional = [(key, value) for key, value, req in fields if not req]
                if mandatory and optional:
                    objects.append(
                        f"class {name}Required(TypedDict):\n"
                        + "\n".join(f"    {k}: {v}" for k, v in mandatory)
                    )
                    objects.append(
                        f"class {name}({name}Required, total=False):\n"
                        + "\n".join(f"    {k}: {v}" for k, v in optional)
                    )
                else:
                    base = "TypedDict" if mandatory else "TypedDict, total=False"
                    objects.append(
                        f"class {name}({base}):\n"
                        + (
                            "\n".join(f"    {k}: {v}" for k, v, _ in fields)
                            or "    pass"
                        )
                    )
        # Classes use postponed annotations; aliases are defined afterward in dependency order.
        pending = dict(aliases)
        ordered = []
        while pending:
            ready = [
                (name, expression)
                for name, expression in pending.items()
                if not (set(re.findall(r"\bWoxView\w+\b", expression)) & set(pending))
            ]
            if not ready:
                raise ValueError(f"Recursive Python aliases: {', '.join(pending)}")
            for name, expression in ready:
                ordered.append(f"{name}: TypeAlias = {expression}")
                del pending[name]
        header = '"""Generated WoxView wire types. Keys match the JSON protocol."""\n\nfrom __future__ import annotations\n\nfrom typing import Any, Literal, NoReturn, TypeAlias, TypedDict\n\n\n'
        return header + "\n\n".join(objects + ["\n".join(ordered)]) + "\n"


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    generator = Generator(load_schema())
    for path, content in (
        (TS_OUTPUT, generator.typescript()),
        (PY_OUTPUT, generator.python()),
    ):
        if args.check:
            if not path.is_file() or path.read_text(encoding="utf-8") != content:
                raise SystemExit(f"Generated types are stale: {path}")
        else:
            path.write_text(content, encoding="utf-8", newline="\n")


if __name__ == "__main__":
    main()
