import json
import unittest

from wox_plugin import WoxPreview, WoxPreviewType


class WebViewPreviewTest(unittest.TestCase):
    def test_default_hidden_round_trip_and_legacy_default(self):
        """Hidden previews remain available and older payloads still open by default."""
        preview = WoxPreview(preview_data="content", default_hidden=True)
        encoded = preview.to_json()
        self.assertTrue(json.loads(encoded)["DefaultHidden"])
        decoded = WoxPreview.from_json(encoded)
        self.assertTrue(decoded.default_hidden)
        self.assertEqual(decoded.preview_data, "content")
        self.assertFalse(WoxPreview.from_json('{"PreviewData":"legacy"}').default_hidden)

    def test_html_preview_round_trip(self):
        """HTML and its Unicode content survive SDK serialization and parsing."""
        html = '<!doctype html><html><body><h1 title="preview">你好 Wox</h1></body></html>'
        preview = WoxPreview(preview_type=WoxPreviewType.WEBVIEW, preview_data=json.dumps({"html": html}))
        encoded = preview.to_json()
        self.assertEqual(json.loads(encoded)["PreviewType"], "webview")
        decoded = WoxPreview.from_json(encoded)
        self.assertEqual(decoded.preview_type, WoxPreviewType.WEBVIEW)
        self.assertEqual(json.loads(decoded.preview_data), {"html": html})


if __name__ == "__main__":
    unittest.main()
