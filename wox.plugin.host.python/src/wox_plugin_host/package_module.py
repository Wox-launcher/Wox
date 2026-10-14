import builtins
import importlib
import importlib.abc
import importlib.machinery
import importlib.util
import os
import sys
from types import ModuleType
from typing import Any

from .single_file import assert_entry_within_directory

_package_generations: dict[str, int] = {}
_package_finders: dict[str, "_PluginNamespaceFinder"] = {}


def _next_package_module_name(plugin_id: str) -> str:
    """Encode IDs without losing punctuation that distinguishes plugins."""
    generation = _package_generations.get(plugin_id, 0) + 1
    _package_generations[plugin_id] = generation
    return f"wox_package_{plugin_id.encode('utf-8').hex()}_{generation}"


class _SourceLoader(importlib.abc.Loader):
    """Execute a plugin file from its current source.

    Timestamp-based bytecode validation can reuse stale code when a same-size
    overwrite falls within the same timestamp second.
    """

    def __init__(self, filepath: str, finder: "_PluginNamespaceFinder") -> None:
        self.filepath = filepath
        self.finder = finder

    def create_module(self, spec: importlib.machinery.ModuleSpec) -> None:
        return None

    def exec_module(self, module: ModuleType) -> None:
        """Keep import statements scoped to this plugin, including delayed calls."""
        module.__dict__["__builtins__"] = dict(vars(builtins), __import__=self.finder.import_module)
        with open(self.filepath, encoding="utf-8") as source_file:
            source = source_file.read()
        exec(compile(source, self.filepath, "exec"), module.__dict__)


class _PluginNamespaceFinder(importlib.abc.MetaPathFinder):
    """Resolve one plugin's modules under a private prefix.

    The prefix keeps two plugins that both ship main.py, src/main.py, or the
    same package directory from sharing sys.modules. The plugin root __init__.py
    is not used as this namespace: packaged templates re-export plugin from
    there, and the host has always loaded the entry module directly.
    """

    def __init__(self, root_name: str, plugin_directory: str) -> None:
        self.root_name = root_name
        self.plugin_directory = plugin_directory

    def import_module(
        self,
        name: str,
        globals: dict[str, Any] | None = None,
        locals: dict[str, Any] | None = None,
        fromlist: tuple[str, ...] = (),
        level: int = 0,
    ) -> ModuleType:
        """Redirect local absolute imports without publishing global aliases."""
        top_name = name.split(".", 1)[0]
        if level == 0 and self.find_spec(f"{self.root_name}.{top_name}") is not None:
            qualified_name = f"{self.root_name}.{name}"
            module = builtins.__import__(qualified_name, globals, locals, fromlist, level)
            # `import src.helper` binds src, whereas `from src.helper import x`
            # returns the requested module. The private prefix must stay hidden.
            if not fromlist:
                return sys.modules[f"{self.root_name}.{top_name}"]
            return module
        return builtins.__import__(name, globals, locals, fromlist, level)

    def find_spec(
        self,
        fullname: str,
        path: object = None,
        target: ModuleType | None = None,
    ) -> importlib.machinery.ModuleSpec | None:
        """Resolve source files and packages inside this plugin's namespace."""
        del path, target
        if fullname == self.root_name:
            spec = importlib.machinery.ModuleSpec(fullname, None, is_package=True)
            spec.submodule_search_locations = [self.plugin_directory]
            return spec
        prefix = self.root_name + "."
        if not fullname.startswith(prefix):
            return None

        relative_parts = fullname[len(prefix) :].split(".")
        if any(not part.isidentifier() for part in relative_parts):
            return None
        parent = self.plugin_directory
        for part in relative_parts[:-1]:
            parent = os.path.join(parent, part)
        name = relative_parts[-1]
        package_dir = os.path.join(parent, name)
        module_file = os.path.join(parent, name + ".py")
        init_file = os.path.join(package_dir, "__init__.py")
        if os.path.isdir(package_dir) and (os.path.isfile(init_file) or not os.path.isfile(module_file)):
            if not self._inside(package_dir):
                return None
            if os.path.isfile(init_file):
                # File-location specs initialize __file__ for both entry modules
                # and packages; origin alone does not enable that behavior.
                return importlib.util.spec_from_file_location(
                    fullname, init_file, loader=_SourceLoader(init_file, self), submodule_search_locations=[package_dir]
                )
            spec = importlib.machinery.ModuleSpec(fullname, None, is_package=True)
            spec.submodule_search_locations = [package_dir]
            return spec
        if os.path.isfile(module_file) and self._inside(module_file):
            return importlib.util.spec_from_file_location(fullname, module_file, loader=_SourceLoader(module_file, self))
        return None

    def _inside(self, candidate: str) -> bool:
        try:
            return os.path.commonpath([os.path.abspath(candidate), self.plugin_directory]) == self.plugin_directory
        except ValueError:
            return False


def _entry_module_parts(plugin_directory: str, entry: str) -> tuple[str, list[str]]:
    """Validate the entry path before deriving its private module name."""
    directory = os.path.abspath(plugin_directory)
    entry_path = assert_entry_within_directory(os.path.join(directory, entry), directory)
    relative = os.path.relpath(entry_path, directory).replace("\\", "/")
    if relative.lower().endswith(".py"):
        relative = relative[:-3]
    parts = [part for part in relative.split("/") if part and part != "."]
    if not parts or any(part == ".." or not part.isidentifier() for part in parts):
        raise ValueError(f"plugin entry cannot be imported: {entry}")
    return directory, parts


def load_package_module(plugin_id: str, plugin_directory: str, entry: str) -> tuple[ModuleType, str]:
    """Load a package plugin under a module name that no other plugin can reuse."""
    directory, parts = _entry_module_parts(plugin_directory, entry)
    root_name = _next_package_module_name(plugin_id)
    finder = _PluginNamespaceFinder(root_name, directory)
    sys.meta_path.insert(0, finder)
    _package_finders[root_name] = finder
    try:
        root_spec = importlib.machinery.ModuleSpec(root_name, None, is_package=True)
        root_spec.submodule_search_locations = [directory]
        sys.modules[root_name] = importlib.util.module_from_spec(root_spec)
        module_name = ".".join((root_name, *parts))
        module = importlib.import_module(module_name)
        if not hasattr(module, "plugin") or getattr(module, "plugin") is None:
            raise AttributeError("Plugin module does not have a 'plugin' attribute")
        return module, module_name
    except Exception:
        release_package_module(root_name)
        raise


def release_package_module(module_name: str) -> bool:
    """Remove one package plugin's finder and every module loaded under it.

    Returns false when the name is not a package namespace this loader created.
    """
    root_name = module_name.split(".", 1)[0]
    finder = _package_finders.pop(root_name, None)
    if finder is None:
        return False
    try:
        sys.meta_path.remove(finder)
    except ValueError:
        pass
    for name in list(sys.modules):
        if name == root_name or name.startswith(root_name + "."):
            sys.modules.pop(name, None)
    return True
