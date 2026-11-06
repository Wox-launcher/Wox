"""Verify the Python SDK wire types and the shared Core fixture."""

import importlib.util
import json
import shutil
import subprocess
import sys
import unittest
from pathlib import Path
from typing import get_type_hints, is_typeddict
from unittest.mock import patch

from typing_woxview import counter

from wox_plugin import WoxViewEventData, WoxViewHandler, WoxViewTree
from wox_plugin.models import woxview


class WoxViewTypesTest(unittest.TestCase):
    def test_shared_fixture_and_event_shape(self):
        fixture = Path(__file__).resolve().parents[2] / "wox.core/ui/woxview/testdata/sdk-counter.json"
        tree = json.loads(fixture.read_text(encoding="utf-8"))
        self.assertEqual(counter, tree)
        self.assertEqual(tree["root"]["children"][1]["props"]["onPress"], {"$handler": "increment"})
        self.assertEqual(WoxViewTree.__required_keys__, {"version", "root"})
        self.assertEqual(WoxViewHandler.__required_keys__, {"$handler"})
        self.assertEqual(set(get_type_hints(WoxViewEventData)), {"Handler", "Args", "Key"})

    def test_all_generated_annotations_resolve(self):
        for name, value in vars(woxview).items():
            if name.startswith("WoxView") and is_typeddict(value):
                with self.subTest(name=name):
                    get_type_hints(value)

    def test_generated_types_use_live_go_schema(self):
        root = Path(__file__).resolve().parents[2]
        spec = importlib.util.spec_from_file_location("woxview_sdk_generator", root / "wox.core/ui/woxview/generate_sdk_types.py")
        assert spec is not None and spec.loader is not None
        generator_module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(generator_module)
        schema = {"$defs": {}, "type": "object", "required": ["version"], "properties": {"version": {"const": 42}}}
        result = subprocess.CompletedProcess([], 0, stdout=json.dumps(schema), stderr="")
        with patch.object(generator_module.subprocess, "run", return_value=result):
            generator = generator_module.Generator(generator_module.load_schema())
        self.assertIn('"version": 42', generator.typescript())
        self.assertIn("version: Literal[42]", generator.python())

    @unittest.skipUnless(shutil.which("go"), "Go is required to verify generated SDK types")
    def test_sdk_types_match_schema(self):
        root = Path(__file__).resolve().parents[2]
        subprocess.run([sys.executable, str(root / "wox.core/ui/woxview/generate_sdk_types.py"), "--check"], check=True)
