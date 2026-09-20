import unittest

from wox_plugin.models.plugin_tool import (
    InvokePluginToolOption,
    InvokePluginToolResult,
    ListPluginToolsResult,
    PluginToolAnnotations,
    PluginToolDescriptor,
    RegisterPluginToolResult,
)


class PluginToolModelTest(unittest.TestCase):
    def test_descriptor_round_trip_uses_core_field_names(self):
        descriptor = PluginToolDescriptor(
            name="create_note",
            description="Create a note",
            input_schema={"type": "object", "properties": {"text": {"type": "string"}}},
            output_schema={"type": "object", "properties": {"noteId": {"type": "string"}}},
            annotations=PluginToolAnnotations(read_only=False, requires_ui=False, idempotent=False, destructive=False),
        )
        encoded = descriptor.to_dict()
        self.assertEqual(encoded["Name"], "create_note")
        self.assertEqual(encoded["InputSchema"]["type"], "object")
        self.assertFalse(encoded["Annotations"]["RequiresUI"])
        restored = PluginToolDescriptor.from_dict(encoded)
        self.assertEqual(restored.name, "create_note")
        self.assertEqual(restored.output_schema["properties"]["noteId"]["type"], "string")

    def test_invoke_option_and_error_result(self):
        option = InvokePluginToolOption(plugin_id="notes", name="create_note", arguments={"text": "hello"})
        self.assertEqual(option.to_dict()["PluginId"], "notes")
        result = InvokePluginToolResult.from_dict(
            {"Output": None, "Error": {"Code": "INVALID_ARGUMENTS", "Message": "query is required"}}
        )
        self.assertEqual(result.error.code, "INVALID_ARGUMENTS")
        self.assertEqual(result.output, {})

    def test_list_and_register_results(self):
        listed = ListPluginToolsResult.from_dict(
            {
                "Tools": [
                    {
                        "PluginId": "notes",
                        "PluginName": "Notes",
                        "Tool": {"Name": "open_note", "Description": "Open", "InputSchema": {"type": "object"}, "OutputSchema": {"type": "object"}},
                    }
                ]
            }
        )
        self.assertEqual(listed.tools[0].plugin_id, "notes")
        self.assertEqual(listed.tools[0].tool.name, "open_note")
        registered = RegisterPluginToolResult.from_dict({"Error": None})
        self.assertIsNone(registered.error)
        failed = RegisterPluginToolResult.from_dict({"Error": {"Code": "TOOL_ALREADY_REGISTERED", "Message": "exists"}})
        self.assertEqual(failed.error.code, "TOOL_ALREADY_REGISTERED")


if __name__ == "__main__":
    unittest.main()
