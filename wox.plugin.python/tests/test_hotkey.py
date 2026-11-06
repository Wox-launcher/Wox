import unittest

from wox_plugin import HotkeyEvent, HotkeyKind, HotkeyPhase, PluginSettingDefinitionItem, PluginSettingValueHotkey


class HotkeyContractTest(unittest.TestCase):
    def test_hold_key_event_kind(self):
        event = HotkeyEvent.from_dict({"SettingKey": "record", "Kind": "holdKey", "Phase": "started", "Hotkey": "hold:delete", "ActivationID": "a", "Timestamp": 1})
        self.assertEqual(event.kind, HotkeyKind.HOLD_KEY)

    def test_global_setting_round_trip(self):
        raw = {"Type": "hotkey", "Value": {"Key": "record", "Scope": "global", "AllowedHotkeyKinds": ["holdModifier"], "DefaultValue": "hold:right_alt"}}
        item = PluginSettingDefinitionItem.from_dict(raw)
        self.assertIsInstance(item.value, PluginSettingValueHotkey)
        decoded = PluginSettingDefinitionItem.from_dict(item.to_dict())
        self.assertEqual(decoded.value.scope, "global")
        self.assertEqual(decoded.value.allowed_hotkey_kinds, ["holdModifier"])

    def test_event_preserves_identity_and_environment(self):
        raw = {"SettingKey": "actions", "ItemID": "stable", "ColumnKey": "hotkey", "Kind": "holdModifier", "Phase": "started", "Hotkey": "hold:right_alt", "ActivationID": "activation", "Timestamp": 123, "Env": {"ActiveWindowTitle": "Editor", "ActiveWindowPid": 42}}
        event = HotkeyEvent.from_dict(raw)
        self.assertEqual(event.kind, HotkeyKind.HOLD_MODIFIER)
        self.assertEqual(event.phase, HotkeyPhase.STARTED)
        self.assertEqual(event.item_id, "stable")
        self.assertEqual(event.env.active_window_title, "Editor")
