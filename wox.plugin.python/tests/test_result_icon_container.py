import json
import unittest

from wox_plugin.models.image import WoxImage
from wox_plugin.models.result import Result


class ResultIconContainerTest(unittest.TestCase):
    def test_icon_container_round_trip_and_default(self):
        for shown in (False, True):
            result = Result(title="Image", icon=WoxImage(), icon_show_container=shown)
            payload = result.to_json()
            self.assertEqual(json.loads(payload)["IconShowContainer"], shown)
            self.assertEqual(Result.from_json(payload).icon_show_container, shown)
        payload = json.loads(Result(title="Image", icon=WoxImage()).to_json())
        del payload["IconShowContainer"]
        self.assertFalse(Result.from_json(json.dumps(payload)).icon_show_container)


if __name__ == "__main__":
    unittest.main()
