import json
import unittest

from wox_plugin.models.query_response import QueryGridLayout


class QueryGridLayoutCompatibilityTest(unittest.TestCase):
    def test_existing_positional_commands_argument(self):
        layout = QueryGridLayout(4, 80, 60, 2, 3, 1.5, ["images"])
        payload = json.loads(layout.to_json())
        self.assertEqual(payload["Commands"], ["images"])
        self.assertIs(payload["ShowTitle"], False)
        self.assertTrue(json.loads(QueryGridLayout(show_title=True).to_json())["ShowTitle"])


if __name__ == "__main__":
    unittest.main()
