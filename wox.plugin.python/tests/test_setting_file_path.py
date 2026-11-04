import unittest

from wox_plugin import PluginSettingDefinitionItem, PluginSettingDefinitionType, PluginSettingValueFilePath, PluginSettingValueTableColumn


class FilePathSettingTest(unittest.TestCase):
    def test_round_trip(self):
        item = PluginSettingDefinitionItem(type=PluginSettingDefinitionType.FILE_PATH, value=PluginSettingValueFilePath(key="file", label="File", extensions=["json", "yaml"]))
        decoded = PluginSettingDefinitionItem.from_dict(item.to_dict())
        self.assertEqual(decoded.value.extensions, ["json", "yaml"])
        column = PluginSettingValueTableColumn(key="file", type="filePath", extensions=["txt"])
        self.assertEqual(PluginSettingValueTableColumn.from_dict(column.to_dict()).extensions, ["txt"])
