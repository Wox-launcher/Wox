import unittest

from wox_plugin import (
    PluginSettingDefinitionItem,
    PluginSettingDefinitionType,
    PluginSettingValueTable,
    PluginSettingValueTableColumn,
    PluginSettingValueTableGroup,
)


class SettingTableTest(unittest.TestCase):
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
