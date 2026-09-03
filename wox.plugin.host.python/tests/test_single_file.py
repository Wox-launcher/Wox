import importlib.util
import os
import sys
import tempfile
import unittest
from pathlib import Path


def load_single_file_module_helpers():
    path = Path(__file__).resolve().parents[1] / "src" / "wox_plugin_host" / "single_file.py"
    spec = importlib.util.spec_from_file_location("wox_single_file_helpers", path)
    if spec is None or spec.loader is None:
        raise ImportError(f"failed to load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class SingleFileLoaderTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.helpers = load_single_file_module_helpers()

    def test_sanitize_and_generation(self) -> None:
        self.assertEqual("com_example_weather", self.helpers.sanitize_plugin_id("com.example.weather"))
        first = self.helpers.next_single_file_module_name("com.example.weather")
        second = self.helpers.next_single_file_module_name("com.example.weather")
        self.assertNotEqual(first, second)
        self.assertTrue(first.startswith("wox_single_file_com_example_weather_"))
        self.assertTrue(second.startswith("wox_single_file_com_example_weather_"))

    def test_path_escape_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaises(ValueError):
                self.helpers.assert_entry_within_directory(os.path.join(directory, "..", "secret.py"), directory)

    def test_load_requires_plugin_export(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "missing.py"
            path.write_text("value = 1\n", encoding="utf-8")
            with self.assertRaises(AttributeError):
                self.helpers.load_single_file_module("missing", directory, "missing.py")

    def test_load_plugin_object(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "ok.py"
            path.write_text("class P:\n    pass\n\nplugin = P()\n", encoding="utf-8")
            module, module_name = self.helpers.load_single_file_module("ok", directory, "ok.py")
            self.assertTrue(hasattr(module, "plugin"))
            self.assertIn(module_name, sys.modules)
            sys.modules.pop(module_name, None)

    def test_reload_same_size_same_mtime_reads_new_source(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "plugin.py"
            path.write_text('title = "v1"\n\nclass P:\n    pass\n\nplugin = P()\nplugin.title = title\n', encoding="utf-8")
            first, first_name = self.helpers.load_single_file_module("reload", directory, "plugin.py")
            self.assertEqual(first.plugin.title, "v1")

            mtime = path.stat().st_mtime
            path.write_text('title = "v2"\n\nclass P:\n    pass\n\nplugin = P()\nplugin.title = title\n', encoding="utf-8")
            os.utime(path, (mtime, mtime))
            second, second_name = self.helpers.load_single_file_module("reload", directory, "plugin.py")
            self.assertEqual(second.plugin.title, "v2")
            sys.modules.pop(first_name, None)
            sys.modules.pop(second_name, None)


if __name__ == "__main__":
    unittest.main()
