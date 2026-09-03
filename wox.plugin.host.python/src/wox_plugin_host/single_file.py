import os
import re
import sys
from types import ModuleType
from typing import Dict

_single_file_generations: Dict[str, int] = {}


def sanitize_plugin_id(plugin_id: str) -> str:
    sanitized = re.sub(r"[^0-9A-Za-z_]", "_", plugin_id)
    if not sanitized:
        sanitized = "plugin"
    if sanitized[0].isdigit():
        sanitized = f"p_{sanitized}"
    return sanitized


def next_single_file_module_name(plugin_id: str) -> str:
    generation = _single_file_generations.get(plugin_id, 0) + 1
    _single_file_generations[plugin_id] = generation
    return f"wox_single_file_{sanitize_plugin_id(plugin_id)}_{generation}"


def assert_entry_within_directory(entry_path: str, directory: str) -> str:
    file_abs = os.path.abspath(entry_path)
    dir_abs = os.path.abspath(directory)
    try:
        if os.path.commonpath([file_abs, dir_abs]) != dir_abs:
            raise ValueError(f"plugin entry escapes plugin directory: {entry_path}")
    except ValueError as exc:
        if "plugin entry escapes" in str(exc):
            raise
        raise ValueError(f"plugin entry escapes plugin directory: {entry_path}") from exc
    return file_abs


def load_single_file_module(plugin_id: str, plugin_directory: str, entry: str) -> tuple[ModuleType, str]:
    module_path = assert_entry_within_directory(os.path.join(plugin_directory, entry), plugin_directory)
    module_name = next_single_file_module_name(plugin_id)
    # Exec the current source instead of importlib's file loader. A same-size
    # overwrite can keep the previous mtime on Windows (1s resolution), and
    # SourceFileLoader would then reuse stale __pycache__ bytecode.
    with open(module_path, encoding="utf-8") as source_file:
        source = source_file.read()
    module = ModuleType(module_name)
    module.__file__ = module_path
    sys.modules[module_name] = module
    try:
        exec(compile(source, module_path, "exec"), module.__dict__)
        if not hasattr(module, "plugin") or getattr(module, "plugin") is None:
            raise AttributeError("Plugin module does not have a 'plugin' attribute")
    except Exception:
        sys.modules.pop(module_name, None)
        raise
    return module, module_name
