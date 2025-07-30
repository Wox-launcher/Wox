from dataclasses import dataclass, field
from typing import Dict, Any, List
from enum import Enum
import json


class PluginSettingDefinitionType(str, Enum):
    """Plugin setting definition type enum"""
    HEAD = "head"
    TEXTBOX = "textbox"
    CHECKBOX = "checkbox"
    SELECT = "select"
    LABEL = "label"
    NEWLINE = "newline"
    TABLE = "table"
    DYNAMIC = "dynamic"


@dataclass
class PluginSettingValueStyle:
    """Style configuration for plugin settings"""
    padding_left: int = field(default=0)
    padding_top: int = field(default=0)
    padding_right: int = field(default=0)
    padding_bottom: int = field(default=0)
    width: int = field(default=0)
    label_width: int = field(default=0)

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary with camelCase naming"""
        return {
            "PaddingLeft": self.padding_left,
            "PaddingTop": self.padding_top,
            "PaddingRight": self.padding_right,
            "PaddingBottom": self.padding_bottom,
            "Width": self.width,
            "LabelWidth": self.label_width,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> 'PluginSettingValueStyle':
        """Create from dictionary with camelCase naming"""
        return cls(
            padding_left=data.get('PaddingLeft', 0),
            padding_top=data.get('PaddingTop', 0),
            padding_right=data.get('PaddingRight', 0),
            padding_bottom=data.get('PaddingBottom', 0),
            width=data.get('Width', 0),
            label_width=data.get('LabelWidth', 0),
        )


@dataclass
class PluginSettingDefinitionValue:
    """Base class for plugin setting values"""
    key: str
    default_value: str = field(default="")

    def get_key(self) -> str:
        """Get the setting key"""
        return self.key

    def get_default_value(self) -> str:
        """Get the default value"""
        return self.default_value

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary"""
        return {
            "Key": self.key,
            "DefaultValue": self.default_value,
        }


@dataclass
class PluginSettingValueTextBox(PluginSettingDefinitionValue):
    """Text box setting value"""
    label: str = field(default="")
    suffix: str = field(default="")
    tooltip: str = field(default="")
    max_lines: int = field(default=1)
    style: PluginSettingValueStyle = field(default_factory=PluginSettingValueStyle)

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary with camelCase naming"""
        return {
            "Key": self.key,
            "Label": self.label,
            "Suffix": self.suffix,
            "DefaultValue": self.default_value,
            "Tooltip": self.tooltip,
            "MaxLines": self.max_lines,
            "Style": self.style.to_dict(),
        }


@dataclass
class PluginSettingValueCheckBox(PluginSettingDefinitionValue):
    """Checkbox setting value"""
    label: str = field(default="")
    tooltip: str = field(default="")
    style: PluginSettingValueStyle = field(default_factory=PluginSettingValueStyle)

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary with camelCase naming"""
        return {
            "Key": self.key,
            "Label": self.label,
            "DefaultValue": self.default_value,
            "Tooltip": self.tooltip,
            "Style": self.style.to_dict(),
        }


@dataclass
class PluginSettingValueLabel(PluginSettingDefinitionValue):
    """Label setting value"""
    content: str = field(default="")
    tooltip: str = field(default="")
    style: PluginSettingValueStyle = field(default_factory=PluginSettingValueStyle)

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary with camelCase naming"""
        return {
            "Content": self.content,
            "Tooltip": self.tooltip,
            "Style": self.style.to_dict(),
        }


@dataclass
class PluginSettingDefinitionItem:
    """Plugin setting definition item"""
    type: PluginSettingDefinitionType
    value: PluginSettingDefinitionValue
    disabled_in_platforms: List[str] = field(default_factory=list)
    is_platform_specific: bool = field(default=False)

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary with camelCase naming"""
        return {
            "Type": self.type,
            "Value": self.value.to_dict(),
            "DisabledInPlatforms": self.disabled_in_platforms,
            "IsPlatformSpecific": self.is_platform_specific,
        }

    def to_json(self) -> str:
        """Convert to JSON string"""
        return json.dumps(self.to_dict())

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> 'PluginSettingDefinitionItem':
        """Create from dictionary"""
        setting_type = PluginSettingDefinitionType(data.get('Type', 'textbox'))

        # Create appropriate value object based on type
        value_data = data.get('Value', {})
        value: PluginSettingDefinitionValue
        if setting_type == PluginSettingDefinitionType.TEXTBOX:
            value = PluginSettingValueTextBox(
                key=value_data.get('Key', ''),
                label=value_data.get('Label', ''),
                suffix=value_data.get('Suffix', ''),
                default_value=value_data.get('DefaultValue', ''),
                tooltip=value_data.get('Tooltip', ''),
                max_lines=value_data.get('MaxLines', 1),
                style=PluginSettingValueStyle.from_dict(value_data.get('Style', {}))
            )
        elif setting_type == PluginSettingDefinitionType.CHECKBOX:
            value = PluginSettingValueCheckBox(
                key=value_data.get('Key', ''),
                label=value_data.get('Label', ''),
                default_value=value_data.get('DefaultValue', ''),
                tooltip=value_data.get('Tooltip', ''),
                style=PluginSettingValueStyle.from_dict(value_data.get('Style', {}))
            )
        elif setting_type == PluginSettingDefinitionType.LABEL:
            value = PluginSettingValueLabel(
                key=value_data.get('Key', ''),
                content=value_data.get('Content', ''),
                tooltip=value_data.get('Tooltip', ''),
                style=PluginSettingValueStyle.from_dict(value_data.get('Style', {}))
            )
        else:
            # Default to basic value
            value = PluginSettingDefinitionValue(
                key=value_data.get('Key', ''),
                default_value=value_data.get('DefaultValue', '')
            )

        return cls(
            type=setting_type,
            value=value,
            disabled_in_platforms=data.get('DisabledInPlatforms', []),
            is_platform_specific=data.get('IsPlatformSpecific', False)
        )

    @classmethod
    def from_json(cls, json_str: str) -> 'PluginSettingDefinitionItem':
        """Create from JSON string"""
        data = json.loads(json_str)
        return cls.from_dict(data)


# Helper functions for creating common setting types
def create_textbox_setting(key: str, label: str, default_value: str = "", tooltip: str = "") -> PluginSettingDefinitionItem:
    """Create a textbox setting"""
    return PluginSettingDefinitionItem(
        type=PluginSettingDefinitionType.TEXTBOX,
        value=PluginSettingValueTextBox(
            key=key,
            label=label,
            default_value=default_value,
            tooltip=tooltip
        )
    )


def create_checkbox_setting(key: str, label: str, default_value: str = "false", tooltip: str = "") -> PluginSettingDefinitionItem:
    """Create a checkbox setting"""
    return PluginSettingDefinitionItem(
        type=PluginSettingDefinitionType.CHECKBOX,
        value=PluginSettingValueCheckBox(
            key=key,
            label=label,
            default_value=default_value,
            tooltip=tooltip
        )
    )


def create_label_setting(content: str, tooltip: str = "") -> PluginSettingDefinitionItem:
    """Create a label setting"""
    return PluginSettingDefinitionItem(
        type=PluginSettingDefinitionType.LABEL,
        value=PluginSettingValueLabel(
            key="",  # Labels don't need keys
            content=content,
            tooltip=tooltip
        )
    )
