import json
import unittest

from wox_plugin.models.toolbar_msg import ToolbarMsg


class ToolbarMsgTests(unittest.TestCase):
    def test_hide_icon_is_opt_in_and_preserves_positional_arguments(self):
        default = ToolbarMsg("status", "Working")
        self.assertFalse(json.loads(default.to_json())["HideIcon"])
        text_only = ToolbarMsg("count", "20,646 files", hide_icon=True)
        self.assertTrue(json.loads(text_only.to_json())["HideIcon"])
        positional = ToolbarMsg("status", "Working", default.icon, 50, True, [])
        self.assertFalse(positional.hide_icon)
        self.assertEqual(json.loads(positional.to_json())["Progress"], 50)
