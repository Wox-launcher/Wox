import unittest

from wox_plugin.models.ai import AIModel, ChatLoopPolicy, ChatOptions, Conversation


class AIChatPayloadTest(unittest.TestCase):
    def test_conversation_uses_core_field_names_and_png_images(self):
        conversation = Conversation.new_user_message("Hello", images=[b"png"])
        conversation.timestamp = 1
        payload = conversation.to_core_dict()
        self.assertEqual(payload["Role"], "user")
        self.assertEqual(payload["Text"], "Hello")
        self.assertEqual(payload["Timestamp"], 1)
        self.assertEqual(payload["Images"][0]["ImageType"], "base64")
        self.assertTrue(payload["Images"][0]["ImageData"].startswith("data:image/png;base64,"))

    def test_text_conversation_omits_empty_images(self):
        payload = Conversation.new_user_message("Hello").to_core_dict()
        self.assertNotIn("Images", payload)

    def test_options_use_core_duration_units(self):
        payload = ChatOptions(
            thinking_mode="thinking",
            loop_policy=ChatLoopPolicy(max_iterations=-1, retry_on_failure=True, timeout_ms=1500),
        ).to_core_dict()
        self.assertEqual(payload["ThinkingMode"], "thinking")
        self.assertEqual(payload["LoopPolicy"]["MaxIterations"], -1)
        self.assertTrue(payload["LoopPolicy"]["RetryOnFailure"])
        self.assertEqual(payload["LoopPolicy"]["Timeout"], 1_500_000_000)
        self.assertEqual(ChatOptions().to_core_dict(), {})
        self.assertEqual(AIModel(name="deepseek-chat", provider="deepseek").provider, "deepseek")
