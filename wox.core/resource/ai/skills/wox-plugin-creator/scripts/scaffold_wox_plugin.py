#!/usr/bin/env python3
import argparse
import getpass
import json
import shutil
import subprocess
import uuid
from pathlib import Path

RUNTIMES = ["nodejs", "python", "script-nodejs", "script-python"]
TEMPLATE_REPOS = {
    "nodejs": "https://github.com/Wox-launcher/Wox.Plugin.Template.Nodejs",
    "python": "https://github.com/Wox-launcher/Wox.Plugin.Template.Python",
}


def detect_repo_root() -> Path:
    start = Path(__file__).resolve()
    for parent in [start] + list(start.parents):
        if (parent / "wox.core").is_dir():
            return parent
    raise SystemExit("Unable to locate repo root containing 'wox.core'.")


def get_skill_templates_dir() -> Path:
    home_dir = Path.home()
    skills_dir = (
        home_dir
        / ".wox"
        / "ai"
        / "skills"
        / "wox-plugin-creator"
        / "assets"
        / "script_plugin_templates"
    )
    if skills_dir.is_dir():
        return skills_dir

    repo_root = detect_repo_root()
    return (
        repo_root
        / "wox.core/resource/ai/skills/wox-plugin-creator/assets/script_plugin_templates"
    )


def ensure_empty_dir(path: Path, force: bool) -> None:
    if path.exists() and any(path.iterdir()) and not force:
        raise SystemExit(
            f"Output directory not empty: {path}. Use --force to overwrite."
        )
    path.mkdir(parents=True, exist_ok=True)


def prepare_clone_target(path: Path, force: bool) -> None:
    if path.exists():
        if force:
            shutil.rmtree(path)
        elif any(path.iterdir()):
            raise SystemExit(
                f"Output directory not empty: {path}. Use --force to overwrite."
            )
        else:
            path.rmdir()


def run(cmd: list[str]) -> None:
    subprocess.run(cmd, check=True)


def apply_placeholders(path: Path, values: dict[str, str]) -> None:
    try:
        content = path.read_text(encoding="utf-8")
    except UnicodeDecodeError:
        return
    updated = render_template(content, values)
    if updated != content:
        path.write_text(updated, encoding="utf-8")


def apply_placeholders_in_tree(root: Path, values: dict[str, str]) -> None:
    for path in root.rglob("*"):
        if ".git" in path.parts:
            continue
        if path.is_file():
            apply_placeholders(path, values)


def render_template(content: str, values: dict[str, str]) -> str:
    rendered = content
    for key, value in values.items():
        rendered = rendered.replace(f"{{{{{key}}}}}", value)
        rendered = rendered.replace(f"{{{{.{key}}}}}", value)
    return rendered


def scaffold_script_plugin(
    template_path: Path, output_path: Path, values: dict[str, str]
) -> None:
    content = template_path.read_text(encoding="utf-8")
    output_path.write_text(render_template(content, values), encoding="utf-8")


def sanitize_script_name(name: str) -> str:
    cleaned = "".join(ch for ch in name if ch.isalnum())
    return cleaned or "Plugin"


def default_script_entry(name: str, ext: str) -> str:
    safe_name = sanitize_script_name(name)
    return f"Wox.Plugin.Script.{safe_name}.{ext}"


def resolve_script_output(
    output_dir: Path, entry: str, ext: str, force: bool
) -> tuple[Path, str]:
    if output_dir.suffix == f".{ext}":
        if output_dir.exists() and output_dir.is_dir():
            raise SystemExit(f"Output path is a directory: {output_dir}")
        if output_dir.exists() and not force:
            raise SystemExit(
                f"Output file already exists: {output_dir}. Use --force to overwrite."
            )
        output_dir.parent.mkdir(parents=True, exist_ok=True)
        return output_dir, output_dir.name

    ensure_empty_dir(output_dir, force)
    output_path = output_dir / entry
    if output_path.exists() and not force:
        raise SystemExit(
            f"Output file already exists: {output_path}. Use --force to overwrite."
        )
    return output_path, entry


def main() -> None:
    parser = argparse.ArgumentParser(description="Scaffold Wox plugins")
    parser.add_argument("--type", required=True, choices=RUNTIMES)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--plugin-id", default="")
    parser.add_argument("--name", default="")
    parser.add_argument("--description", default="")
    parser.add_argument("--trigger-keywords", default="")
    parser.add_argument("--author", default="")
    parser.add_argument("--version", default="0.1.0")
    parser.add_argument("--min-wox-version", default="2.0.0")
    parser.add_argument("--entry", default="")
    parser.add_argument("--force", action="store_true")

    args = parser.parse_args()
    output_dir = Path(args.output_dir).resolve()

    if not args.name:
        raise SystemExit("--name is required")

    trigger_keywords = [
        kw.strip() for kw in args.trigger_keywords.split(",") if kw.strip()
    ]
    trigger_keywords_json = json.dumps(trigger_keywords)

    if not trigger_keywords:
        raise SystemExit("--trigger-keywords is required")

    plugin_id = args.plugin_id or str(uuid.uuid4())
    description = args.description or args.name
    author = args.author or getpass.getuser()

    values = {
        "PluginID": plugin_id,
        "Name": args.name,
        "Description": description,
        "Author": author,
        "TriggerKeywordsJSON": trigger_keywords_json,
    }

    if args.type in TEMPLATE_REPOS:
        prepare_clone_target(output_dir, args.force)
        run(
            ["git", "clone", "--depth", "1", TEMPLATE_REPOS[args.type], str(output_dir)]
        )
        apply_placeholders_in_tree(output_dir, values)
        print(f"Cloned {args.type} template into {output_dir}")
        return

    templates_dir = get_skill_templates_dir()
    if args.type == "script-nodejs":
        template_path = templates_dir / "template.js"
        entry = args.entry or default_script_entry(args.name, "js")
        output_path, entry_name = resolve_script_output(
            output_dir, entry, "js", args.force
        )
    elif args.type == "script-python":
        template_path = templates_dir / "template.py"
        entry = args.entry or default_script_entry(args.name, "py")
        output_path, entry_name = resolve_script_output(
            output_dir, entry, "py", args.force
        )
    else:
        raise SystemExit(f"Unsupported script runtime: {args.type}")
    values["ENTRY"] = entry_name
    scaffold_script_plugin(template_path, output_path, values)

    print(f"Scaffolded {args.type} plugin at {output_dir}")


if __name__ == "__main__":
    main()
