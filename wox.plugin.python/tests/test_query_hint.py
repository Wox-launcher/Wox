"""Run with python -m unittest discover -s tests -p test_query_hint.py."""

import json
import unittest

from wox_plugin import ChangeQueryParam, Query, QueryElement, QueryHint, QueryType
from wox_plugin.models.query import MetadataCommand
from wox_plugin import (
    RegisterTriggerKeywordOption,
    RegisterTriggerKeywordResult,
    UnregisterTriggerKeywordOption,
    UnregisterTriggerKeywordResult,
)


class QueryHintTest(unittest.TestCase):
    def test_suggestions_round_trip_and_isolation(self):
        hint = QueryHint([QueryElement("filter", "argument", value="cr", suggestions=["created", "assigned"])])
        payload = hint.to_dict()
        restored = QueryHint.from_value(payload)
        self.assertEqual(restored.elements[0].suggestions, ["created", "assigned"])
        self.assertEqual(restored.elements[0].value, "cr")
        payload["Elements"][0]["Suggestions"][0] = "changed"
        self.assertEqual(hint.elements[0].suggestions[0], "created")
        self.assertEqual(restored.elements[0].suggestions[0], "created")
        legacy = QueryHint.from_value({"Elements": [{"Id": "filter", "Kind": "argument"}]})
        self.assertEqual(legacy.elements[0].suggestions, [])
        self.assertNotIn("Suggestions", legacy.to_dict()["Elements"][0])

    def test_trigger_options(self):
        hint = QueryHint([QueryElement("input", "argument", placeholder="Input")])
        payload = RegisterTriggerKeywordOption("g", hint).to_dict()
        self.assertEqual(payload["Keyword"], "g")
        self.assertEqual(payload["QueryHint"]["Elements"][0]["Id"], "input")
        self.assertIsNone(RegisterTriggerKeywordOption("g").to_dict()["QueryHint"])
        self.assertIsNone(RegisterTriggerKeywordOption("g").to_dict()["DebounceMs"])
        self.assertEqual(RegisterTriggerKeywordOption("g", debounce_ms=500).to_dict()["DebounceMs"], 500)
        self.assertEqual(RegisterTriggerKeywordOption("g", debounce_ms=0).to_dict()["DebounceMs"], 0)
        self.assertEqual(UnregisterTriggerKeywordOption("g").to_dict(), {"Keyword": "g"})
        self.assertFalse(RegisterTriggerKeywordResult().success)
        self.assertFalse(UnregisterTriggerKeywordResult().success)

    def test_round_trip_and_legacy(self):
        structure = QueryHint([QueryElement("volume", "argument", value="50", placeholder="Volume")])
        request = ChangeQueryParam(query_type=QueryType.INPUT, query_text="50", query_hint=structure)
        restored = ChangeQueryParam.from_json(request.to_json())
        self.assertEqual(restored.query_hint.elements[0].value, "50")
        command = MetadataCommand("set-volume", "Set volume", aliases=["set volume"], query_hint=structure)
        self.assertEqual(json.loads(command.to_json())["Aliases"], ["set volume"])
        query = Query.from_json(json.dumps({"Type": "input", "QueryHint": json.dumps(structure.to_dict())}))
        self.assertEqual(query.query_hint.elements[0].id, "volume")
        self.assertIsNone(Query.from_json('{"Type":"input"}').query_hint)

    def test_command_debounce_round_trip(self):
        for delay in (None, 0, 500):
            command = MetadataCommand("search", "Search", debounce_ms=delay)
            self.assertEqual(json.loads(command.to_json())["DebounceMs"], delay)
            self.assertEqual(MetadataCommand.from_json(command.to_json()).debounce_ms, delay)
        self.assertIsNone(MetadataCommand.from_json('{"Command":"search"}').debounce_ms)

    def test_change_query_requires_real_content(self):
        with self.assertRaises(ValueError):
            ChangeQueryParam(query_type=QueryType.INPUT, query_hint=QueryHint())
        with self.assertRaises(ValueError):
            ChangeQueryParam(query_type=QueryType.SELECTION)
        self.assertEqual(ChangeQueryParam(query_type=QueryType.INPUT, query_text="").query_text, "")


if __name__ == "__main__":
    unittest.main()
