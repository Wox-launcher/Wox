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
    def test_trigger_options(self):
        hint = QueryHint([QueryElement("input", "argument", placeholder="Input")])
        payload = RegisterTriggerKeywordOption("g", hint).to_dict()
        self.assertEqual(payload["Keyword"], "g")
        self.assertEqual(payload["QueryHint"]["Elements"][0]["Id"], "input")
        self.assertIsNone(RegisterTriggerKeywordOption("g").to_dict()["QueryHint"])
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

    def test_change_query_requires_real_content(self):
        with self.assertRaises(ValueError):
            ChangeQueryParam(query_type=QueryType.INPUT, query_hint=QueryHint())
        with self.assertRaises(ValueError):
            ChangeQueryParam(query_type=QueryType.SELECTION)
        self.assertEqual(ChangeQueryParam(query_type=QueryType.INPUT, query_text="").query_text, "")


if __name__ == "__main__":
    unittest.main()
