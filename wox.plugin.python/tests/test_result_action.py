import json
import unittest

from wox_plugin.models.result import ResultAction, ResultActionType
from wox_plugin.models.image import WoxImage


class ResultActionToolbarTest(unittest.TestCase):
    def test_toolbar_visibility_round_trip_for_execute_and_form(self):
        for action_type in (ResultActionType.EXECUTE, ResultActionType.FORM):
            with self.subTest(action_type=action_type):
                action = ResultAction(name="Browse folder", type=action_type, hotkey="shift+enter", show_in_toolbar=True)
                payload = json.loads(action.to_json())
                self.assertIs(payload["ShowInToolbar"], True)
                restored = ResultAction.from_json(json.dumps(payload))
                self.assertTrue(restored.show_in_toolbar)
                self.assertEqual(restored.hotkey, "shift+enter")
                self.assertEqual(restored.type, action_type)

    def test_omitted_flag_defaults_to_false_and_preserves_positional_arguments(self):
        action = ResultAction.from_json('{"Name":"Open","Hotkey":"enter"}')
        self.assertFalse(action.show_in_toolbar)
        self.assertFalse(json.loads(action.to_json())["ShowInToolbar"])
        positional = ResultAction(
            "Open", None, "open", ResultActionType.EXECUTE, [], None, WoxImage(), True, False, "enter", {"path": "file.txt"}, ["launch"]
        )
        self.assertEqual(positional.search_aliases, ["launch"])
        self.assertEqual(positional.context_data, {"path": "file.txt"})
        self.assertFalse(positional.show_in_toolbar)


if __name__ == "__main__":
    unittest.main()
