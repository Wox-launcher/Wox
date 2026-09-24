import unittest

from wox_plugin import (
    PluginSettingDefinitionItem,
    PluginSettingDefinitionType,
    PluginSettingValueTable,
    PluginSettingValueTableColumn,
    PluginSettingValueTableGroup,
    PluginSettingValueTextBox,
)


class SettingTableTest(unittest.TestCase):
    def test_password_round_trip(self):
        item = PluginSettingDefinitionItem(
            type=PluginSettingDefinitionType.PASSWORD,
            value=PluginSettingValueTextBox(key="api_key", label="API Key", tooltip="Enter your API key", max_lines=1),
        )

        decoded = PluginSettingDefinitionItem.from_dict(item.to_dict())
        self.assertEqual(decoded.to_dict(), item.to_dict())

    def test_table_groups_round_trip(self):
        item = PluginSettingDefinitionItem(
            type=PluginSettingDefinitionType.TABLE,
            value=PluginSettingValueTable(
                key="sites",
                title="Sites",
                groups=[PluginSettingValueTableGroup(key="advanced", title="Advanced", collapsed_by_default=True)],
                columns=[
                    PluginSettingValueTableColumn(key="name", label="Name", type="text"),
                    PluginSettingValueTableColumn(key="injectCss", label="Inject CSS", type="text", hide_in_table=True, group="advanced"),
                ],
            ),
        )

        decoded = PluginSettingDefinitionItem.from_dict(item.to_dict())
        table = decoded.value
        self.assertIsInstance(table, PluginSettingValueTable)
        self.assertEqual(table.key, "sites")
        self.assertEqual(table.groups[0].key, "advanced")
        self.assertTrue(table.groups[0].collapsed_by_default)
        self.assertEqual(table.columns[1].group, "advanced")
        self.assertTrue(table.columns[1].hide_in_table)


if __name__ == "__main__":
    unittest.main()
