import importlib.util
import os
import sys
import tempfile
import unittest
from pathlib import Path
from types import ModuleType
from unittest.mock import patch


def load_package_module_helpers():
    package_path = Path(__file__).resolve().parents[1] / "src" / "wox_plugin_host"
    spec = importlib.util.spec_from_loader("wox_plugin_host", loader=None, is_package=True)
    if spec is None:
        raise ImportError(f"failed to load {package_path}")
    package = importlib.util.module_from_spec(spec)
    package.__path__ = [str(package_path)]
    sys.modules.setdefault("wox_plugin_host", package)
    from wox_plugin_host.package_module import load_package_module, release_package_module

    return load_package_module, release_package_module


def write_plugin(root: Path, files: dict[str, str]) -> None:
    for name, content in files.items():
        path = root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8")


class PackageModuleLoaderTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        load_package_module, release_package_module = load_package_module_helpers()
        cls.load_package_module = staticmethod(load_package_module)
        cls.release_package_module = staticmethod(release_package_module)

    def test_same_entry_name_stays_isolated(self) -> None:
        with tempfile.TemporaryDirectory() as left_dir, tempfile.TemporaryDirectory() as right_dir:
            write_plugin(Path(left_dir), {"main.py": "class P:\n    pass\n\nplugin = P()\nplugin.marker = 'left'\n"})
            write_plugin(Path(right_dir), {"main.py": "class P:\n    pass\n\nplugin = P()\nplugin.marker = 'right'\n"})
            left, left_name = self.load_package_module("left", left_dir, "main.py")
            right, right_name = self.load_package_module("right", right_dir, "main.py")
            try:
                self.assertEqual(left.plugin.marker, "left")
                self.assertEqual(right.plugin.marker, "right")
                self.assertNotEqual(left_name, right_name)
                self.assertNotIn("main", sys.modules)
                self.assertNotIn(left_dir, sys.path)
                self.assertNotIn(right_dir, sys.path)
            finally:
                self.release_package_module(left_name)
                self.release_package_module(right_name)

    def test_relative_imports_stay_inside_each_plugin(self) -> None:
        with tempfile.TemporaryDirectory() as left_dir, tempfile.TemporaryDirectory() as right_dir:
            sources = {
                "wox_stopwatch/__init__.py": "",
                "wox_stopwatch/state.py": "token = {token!r}\n",
                "wox_stopwatch/main.py": "from .state import token\n\nclass P:\n    pass\n\nplugin = P()\nplugin.token = token\n",
            }
            write_plugin(
                Path(left_dir),
                {name: content.format(token="left") if "state.py" in name else content for name, content in sources.items()},
            )
            write_plugin(
                Path(right_dir),
                {name: content.format(token="right") if "state.py" in name else content for name, content in sources.items()},
            )
            left, left_name = self.load_package_module("stopwatch-left", left_dir, "wox_stopwatch/main.py")
            right, right_name = self.load_package_module("stopwatch-right", right_dir, "wox_stopwatch/main.py")
            try:
                self.assertEqual(left.plugin.token, "left")
                self.assertEqual(right.plugin.token, "right")
                self.assertNotIn("wox_stopwatch", sys.modules)
                self.assertNotIn("wox_stopwatch.main", sys.modules)
            finally:
                self.release_package_module(left_name)
                self.release_package_module(right_name)

    def test_root_init_is_not_executed(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            write_plugin(
                Path(directory),
                {
                    "__init__.py": "raise RuntimeError('root init')\n",
                    "main.py": "class P:\n    pass\n\nplugin = P()\n",
                },
            )
            module, module_name = self.load_package_module("flat", directory, "main.py")
            try:
                self.assertIsNotNone(module.plugin)
            finally:
                self.release_package_module(module_name)

    def test_nested_init_can_import_entry(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            write_plugin(
                Path(directory),
                {
                    "src/__init__.py": "from .main import plugin\n",
                    "src/main.py": "class P:\n    pass\n\nplugin = P()\nplugin.origin = 'main'\n",
                },
            )
            module, module_name = self.load_package_module("iconify", directory, "src/main.py")
            try:
                self.assertEqual(module.plugin.origin, "main")
            finally:
                self.release_package_module(module_name)

    def test_file_locations_resolve_plugin_resources(self) -> None:
        """Both flat entries and package initializers can locate nearby assets."""
        for entry in ("main.py", "pkg/main.py"):
            with self.subTest(entry=entry), tempfile.TemporaryDirectory() as directory:
                write_plugin(
                    Path(directory),
                    {
                        "asset.txt": "plugin resource",
                        "pkg/__init__.py": "from pathlib import Path\nresource = Path(__file__).parent.parent / 'asset.txt'\n",
                        "main.py": "from pathlib import Path\nplugin = Path(__file__).with_name('asset.txt').read_text()\n",
                        "pkg/main.py": "from . import resource\nplugin = resource.read_text()\n",
                    },
                )
                module, module_name = self.load_package_module("resources", directory, entry)
                self.addCleanup(self.release_package_module, module_name)
                self.assertEqual(module.plugin, "plugin resource")
                self.assertEqual(Path(module.__file__), Path(directory) / entry)
                if entry.startswith("pkg/"):
                    package = sys.modules[module.__package__]
                    self.assertEqual(Path(package.__file__), Path(directory) / "pkg/__init__.py")

    def test_absolute_imports_stay_inside_each_plugin(self) -> None:
        """Keep import binding semantics while isolating transitive local imports."""
        with tempfile.TemporaryDirectory() as left_dir, tempfile.TemporaryDirectory() as right_dir:
            source = (
                "import helper\n"
                "from helper import marker as direct\n"
                "import src.state\n"
                "import src.state as alias\n"
                "from src import state\n"
                "from src.state import marker as nested\n"
                "from .helper import marker as relative\n"
                "plugin = (helper.marker, direct, src.state.marker, alias.marker, state.marker, nested, relative)\n"
            )
            # A global cached name must not satisfy either plugin's local import.
            global_helper = ModuleType("helper")
            global_helper.marker = "global"
            with patch.dict(sys.modules, {"helper": global_helper}):
                modules = []
                for marker, directory in (("left", left_dir), ("right", right_dir)):
                    write_plugin(
                        Path(directory),
                        {
                            "main.py": source,
                            "helper.py": "from src.state import marker\n",
                            "src/__init__.py": "from src.state import marker\n",
                            "src/state.py": f"marker = {marker!r}\n",
                        },
                    )
                    module, module_name = self.load_package_module(f"absolute-{marker}", directory, "main.py")
                    self.addCleanup(self.release_package_module, module_name)
                    modules.append(module)
                    self.assertEqual(module.plugin, (marker,) * 7)
                    self.assertNotIn(directory, sys.path)
                self.assertIsNot(modules[0], modules[1])
                self.assertIs(sys.modules["helper"], global_helper)
                self.assertNotIn("src", sys.modules)
                self.assertNotIn("src.state", sys.modules)

    def test_delayed_absolute_import_survives_other_plugin_unload(self) -> None:
        """Query-time imports use the owning namespace after another plugin exits."""
        with tempfile.TemporaryDirectory() as left_dir, tempfile.TemporaryDirectory() as right_dir:
            loaded = []
            for marker, directory in (("left", left_dir), ("right", right_dir)):
                write_plugin(
                    Path(directory),
                    {
                        "main.py": (
                            "def query():\n"
                            "    import late\n"
                            "    from src.late import marker\n"
                            "    return late.marker, marker\n"
                            "plugin = query\n"
                        ),
                        "late.py": f"marker = {marker!r}\n",
                        "src/late.py": f"marker = {marker!r}\n",
                    },
                )
                module, module_name = self.load_package_module(f"delayed-{marker}", directory, "main.py")
                self.addCleanup(self.release_package_module, module_name)
                loaded.append((module, module_name))
            left, left_name = loaded[0]
            right, right_name = loaded[1]
            self.assertEqual(left.plugin(), ("left", "left"))
            self.release_package_module(left_name)
            self.assertFalse(any(name.startswith(left_name.rsplit(".", 1)[0] + ".") for name in sys.modules))
            self.assertEqual(right.plugin(), ("right", "right"))
            self.assertIn(right_name, sys.modules)
            self.assertNotIn("late", sys.modules)
            self.assertNotIn("src", sys.modules)

    def test_nonlocal_absolute_imports_use_standard_imports(self) -> None:
        """External dependencies retain their ordinary shared module identity."""
        dependency = ModuleType("wox_test_dependency")
        with tempfile.TemporaryDirectory() as directory, patch.dict(sys.modules, {dependency.__name__: dependency}):
            write_plugin(
                Path(directory),
                {"main.py": "import os\nimport wox_test_dependency\nplugin = (os, wox_test_dependency)\n"},
            )
            module, module_name = self.load_package_module("external", directory, "main.py")
            self.addCleanup(self.release_package_module, module_name)
            self.assertEqual(module.plugin, (os, dependency))

    def test_ids_with_same_sanitized_name_stay_isolated(self) -> None:
        """Distinct IDs must never share a namespace, including after a reload."""
        with tempfile.TemporaryDirectory() as left_dir, tempfile.TemporaryDirectory() as right_dir:
            write_plugin(Path(left_dir), {"main.py": "plugin = 'left'\n"})
            write_plugin(Path(right_dir), {"main.py": "plugin = 'right'\n"})
            left, left_name = self.load_package_module("collision-a", left_dir, "main.py")
            self.addCleanup(self.release_package_module, left_name)
            right, right_name = self.load_package_module("collision_a", right_dir, "main.py")
            self.addCleanup(self.release_package_module, right_name)
            self.assertNotEqual(left_name, right_name)
            self.assertEqual((left.plugin, right.plugin), ("left", "right"))
            self.release_package_module(left_name)
            self.assertIn(right_name, sys.modules)
            reloaded, reloaded_name = self.load_package_module("collision-a", left_dir, "main.py")
            self.addCleanup(self.release_package_module, reloaded_name)
            self.assertNotIn(reloaded_name, (left_name, right_name))
            self.assertEqual((reloaded.plugin, right.plugin), ("left", "right"))

    def test_reload_same_size_same_mtime_reads_new_source(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            write_plugin(
                root,
                {
                    "pkg/__init__.py": "",
                    "pkg/state.py": "label = 'v1'\n",
                    "pkg/main.py": "from .state import label\n\nclass P:\n    pass\n\nplugin = P()\nplugin.label = label\n",
                },
            )
            first, first_name = self.load_package_module("reload", directory, "pkg/main.py")
            self.assertEqual(first.plugin.label, "v1")
            self.release_package_module(first_name)

            state = root / "pkg" / "state.py"
            main = root / "pkg" / "main.py"
            mtime = state.stat().st_mtime
            state.write_text("label = 'v2'\n", encoding="utf-8")
            main.write_text(
                "from .state import label\n\nclass P:\n    pass\n\nplugin = P()\nplugin.label = label\n",
                encoding="utf-8",
            )
            os.utime(state, (mtime, mtime))
            os.utime(main, (mtime, mtime))
            second, second_name = self.load_package_module("reload", directory, "pkg/main.py")
            try:
                self.assertEqual(second.plugin.label, "v2")
            finally:
                self.release_package_module(second_name)

    def test_failed_load_leaves_no_namespace(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            write_plugin(Path(directory), {"helper.py": "value = 1\n", "main.py": "import helper\nraise RuntimeError('boom')\n"})
            meta_path_length = len(sys.meta_path)
            existing_namespaces = {name for name in sys.modules if name.startswith("wox_package_")}
            with self.assertRaises(RuntimeError):
                self.load_package_module("broken", directory, "main.py")
            self.assertEqual(len(sys.meta_path), meta_path_length)
            self.assertEqual({name for name in sys.modules if name.startswith("wox_package_")}, existing_namespaces)
            self.assertNotIn("helper", sys.modules)

    def test_release_keeps_the_other_plugin(self) -> None:
        with tempfile.TemporaryDirectory() as left_dir, tempfile.TemporaryDirectory() as right_dir:
            write_plugin(Path(left_dir), {"main.py": "class P:\n    pass\n\nplugin = P()\nplugin.marker = 'left'\n"})
            write_plugin(Path(right_dir), {"main.py": "class P:\n    pass\n\nplugin = P()\nplugin.marker = 'right'\n"})
            left, left_name = self.load_package_module("keep-left", left_dir, "main.py")
            right, right_name = self.load_package_module("keep-right", right_dir, "main.py")
            self.release_package_module(left_name)
            try:
                self.assertNotIn(left_name, sys.modules)
                self.assertEqual(right.plugin.marker, "right")
                self.assertIn(right_name, sys.modules)
            finally:
                self.release_package_module(right_name)

    def test_path_escape_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaises(ValueError):
                self.load_package_module("escape", directory, os.path.join("..", "secret.py"))

    def test_missing_plugin_export_is_cleaned_up(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            write_plugin(Path(directory), {"main.py": "value = 1\n"})
            meta_path_length = len(sys.meta_path)
            with self.assertRaises(AttributeError):
                self.load_package_module("missing", directory, "main.py")
            self.assertEqual(len(sys.meta_path), meta_path_length)


if __name__ == "__main__":
    unittest.main()
