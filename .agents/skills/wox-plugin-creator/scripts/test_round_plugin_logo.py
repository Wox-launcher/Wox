import unittest

from PIL import Image

from round_plugin_logo import round_logo


class RoundLogoTest(unittest.TestCase):
    def test_mask_has_transparent_corners_and_smooth_edges(self):
        result = round_logo(Image.new("RGBA", (128, 128), "blue"), 128, 0.23)
        alpha = result.getchannel("A")
        for point in ((0, 0), (127, 0), (0, 127), (127, 127)):
            self.assertEqual(alpha.getpixel(point), 0)
        self.assertEqual(alpha.getpixel((64, 64)), 255)
        self.assertGreater(sum(alpha.histogram()[1:255]), 0)

    def test_preserves_existing_transparency(self):
        source = Image.new("RGBA", (32, 32), (0, 0, 255, 128))
        self.assertEqual(round_logo(source, 32, 0.23).getpixel((16, 16))[3], 128)

    def test_rejects_non_square_source(self):
        with self.assertRaises(ValueError):
            round_logo(Image.new("RGB", (16, 32)), 32, 0.23)


if __name__ == "__main__":
    unittest.main()
