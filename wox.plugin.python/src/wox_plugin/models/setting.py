"""
Wox Setting Models

This module provides models for defining plugin settings and user preferences.

Plugins can define settings that appear in the Wox settings UI, allowing users
to customize plugin behavior. Settings support various input types including
text boxes, checkboxes, dropdowns, labels, and more.
"""

from dataclasses import dataclass, field
from typing import Dict, Any, List
from enum import Enum
import json


class PluginSettingDefinitionType(str, Enum):
    """
    Enumeration of plugin setting UI element types.

    Each type represents a different kind of input or display element
    in the plugin settings panel.
    """

    HEAD = "head"
    """
    Section heading/separator.

    Used to group related settings under a heading.
    Displays as bold text with optional tooltip.
    """

    TEXTBOX = "textbox"
    """
    Single-line or multi-line text input field.

    Allows users to enter text values. Can be configured for
    single-line (default) or multi-line input.
    """

    CHECKBOX = "checkbox"
    """
    Boolean toggle/checkbox.

    Used for on/off or true/false settings. Displays as a
    checkbox with a label.
    """

    SELECT = "select"
    """
    Dropdown selection menu.

    Allows users to choose from a predefined list of options.
    Each option has a label (shown to user) and value (stored).
    """

    LABEL = "label"
    """
    Static text label for information display.

    Used to show informational text, descriptions, or help
    content. Not an input field - text is read-only.
    """

    NEWLINE = "newline"
    """
    Vertical spacing/line break.

    Adds blank vertical space between settings for better
    visual organization.
    """

    TABLE = "table"
    """
    Tabular data display with editing capabilities.

    Displays a table of items that users can view, add, edit,
    and delete. Each column can have different data types.
    """

    DYNAMIC = "dynamic"
    """
    Dynamically loaded setting.

    The setting definition is provided by a callback function
    registered via `api.on_get_dynamic_setting()`. This allows
    settings to be generated dynamically based on current state.
    """


@dataclass
class PluginSettingValueStyle:
    """
    Style configuration for plugin setting UI elements.

    Controls the layout and spacing of setting items in the
    settings panel.

    Attributes:
        padding_left: Left padding in pixels
        padding_top: Top padding in pixels
        padding_right: Right padding in pixels
        padding_bottom: Bottom padding in pixels
        width: Width of the setting element (0 = default)
        label_width: Width of the label column (0 = default)

    Example usage:
        style = PluginSettingValueStyle(
            padding_left=20,
            padding_top=10,
            width=300
        )
    """

    padding_left: int = field(default=0)
    """
    Left padding in pixels.

    Adds space between the left edge of the settings panel
    and the setting element.
    """

    padding_top: int = field(default=0)
    """
    Top padding in pixels.

    Adds space above the setting element.
    """

    padding_right: int = field(default=0)
    """
    Right padding in pixels.

    Adds space between the setting element and the right edge.
    """

    padding_bottom: int = field(default=0)
    """
    Bottom padding in pixels.

    Adds space below the setting element.
    """

    width: int = field(default=0)
    """
    Width of the setting element in pixels.

    A value of 0 uses the default/automatic width.
    """

    label_width: int = field(default=0)
    """
    Width of the label column in pixels.

    Controls how much horizontal space the label takes.
    A value of 0 uses the default width.
    """

    def to_dict(self) -> Dict[str, Any]:
        """
        Convert to dictionary with camelCase naming.

        Returns:
            Dictionary representation
        """
        return {
            "PaddingLeft": self.padding_left,
            "PaddingTop": self.padding_top,
            "PaddingRight": self.padding_right,
            "PaddingBottom": self.padding_bottom,
            "Width": self.width,
            "LabelWidth": self.label_width,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "PluginSettingValueStyle":
        """
        Create from dictionary with camelCase naming.

        Args:
            data: Dictionary with style properties

        Returns:
            A new PluginSettingValueStyle instance
        """
        return cls(
            padding_left=data.get("PaddingLeft", 0),
            padding_top=data.get("PaddingTop", 0),
            padding_right=data.get("PaddingRight", 0),
            padding_bottom=data.get("PaddingBottom", 0),
            width=data.get("Width", 0),
            label_width=data.get("LabelWidth", 0),
        )


@dataclass
class PluginSettingDefinitionValue:
    """
    Base class for plugin setting values.

    Provides the common structure for all setting types.
    Typically, you would use the specific subclasses like
    PluginSettingValueTextBox or PluginSettingValueCheckBox.

    Attributes:
        key: Unique identifier for the setting
        default_value: Default value for the setting

    Example usage:
        value = PluginSettingDefinitionValue(
            key="my_setting",
            default_value="default"
        )
    """

    key: str
    """
    Unique identifier for the setting.

    This key is used to store and retrieve the setting value
    via `api.get_setting()` and `api.save_setting()`.

    Should be unique across all settings in your plugin.
    """

    default_value: str = field(default="")
    """
    Default value for the setting.

    If the user hasn't set a value, this default is returned
    by `api.get_setting()`.

    For checkboxes, use "true" or "false" (case-insensitive).
    """

    def get_key(self) -> str:
        """
        Get the setting key.

        Returns:
            The key identifier for this setting
        """
        return self.key

    def get_default_value(self) -> str:
        """
        Get the default value.

        Returns:
            The default value string
        """
        return self.default_value

    def to_dict(self) -> Dict[str, Any]:
        """
        Convert to dictionary.

        Returns:
            Dictionary with Key and DefaultValue
        """
        return {
            "Key": self.key,
            "DefaultValue": self.default_value,
        }


@dataclass
class PluginSettingValueTextBox(PluginSettingDefinitionValue):
    """
    Text box setting for single or multi-line text input.

    Provides a text input field where users can enter arbitrary
    text values. Supports single-line (default) or multi-line modes.

    Attributes:
        key: Setting identifier
        label: Display label for the text box
        suffix: Optional suffix text (e.g., unit, placeholder)
        tooltip: Help text shown on hover
        max_lines: Maximum number of lines (1 = single-line, >1 = multi-line)
        style: Visual styling options

    Example usage:
        # Single-line text box
        setting = PluginSettingValueTextBox(
            key="api_key",
            label="API Key",
            tooltip="Enter your API key here"
        )

        # Multi-line text box
        setting = PluginSettingValueTextBox(
            key="template",
            label="Message Template",
            max_lines=5
        )
    """

    label: str = field(default="")
    """
    Display label for the text box.

    Shown next to the input field in the settings UI.
    Can be a translation key for i18n.
    """

    suffix: str = field(default="")
    """
    Optional suffix text.

    Text displayed after the input field. Common uses:
    - Units: "ms", "MB", "px"
    - Placeholder indicators: "(optional)"
    - Additional context: "in seconds"
    """

    tooltip: str = field(default="")
    """
    Help text shown on hover.

    Provides additional information about what the setting
    does or what value should be entered.
    """

    max_lines: int = field(default=1)
    """
    Maximum number of lines for the text box.

    - 1: Single-line input (default)
    - >1: Multi-line text area with specified maximum lines

    Use values like 3, 5, or 10 for multi-line input.
    """

    style: PluginSettingValueStyle = field(default_factory=PluginSettingValueStyle)
    """
    Visual styling options.

    Controls padding, width, and other layout properties.
    """

    def to_dict(self) -> Dict[str, Any]:
        """
        Convert to dictionary with camelCase naming.

        Returns:
            Dictionary representation
        """
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
    """
    Checkbox setting for boolean on/off values.

    Provides a toggle checkbox that stores "true" or "false"
    string values.

    Attributes:
        key: Setting identifier
        label: Display label for the checkbox
        tooltip: Help text shown on hover
        style: Visual styling options

    Example usage:
        setting = PluginSettingValueCheckBox(
            key="enabled",
            label="Enable Feature",
            default_value="true",
            tooltip="When enabled, the feature will be active"
        )

        # Read the value:
        enabled = await api.get_setting(ctx, "enabled")
        is_enabled = enabled.lower() == "true"
    """

    label: str = field(default="")
    """
    Display label for the checkbox.

    Shown next to the checkbox in the settings UI.
    Can be a translation key for i18n.
    """

    tooltip: str = field(default="")
    """
    Help text shown on hover.

    Explains what the checkbox does or what happens
    when it's checked/unchecked.
    """

    style: PluginSettingValueStyle = field(default_factory=PluginSettingValueStyle)
    """
    Visual styling options.

    Controls padding, width, and other layout properties.
    """

    def to_dict(self) -> Dict[str, Any]:
        """
        Convert to dictionary with camelCase naming.

        Returns:
            Dictionary representation
        """
        return {
            "Key": self.key,
            "Label": self.label,
            "DefaultValue": self.default_value,
            "Tooltip": self.tooltip,
            "Style": self.style.to_dict(),
        }


@dataclass
class PluginSettingValueLabel(PluginSettingDefinitionValue):
    """
    Static label for displaying informational text.

    Displays read-only text in the settings panel, useful for
    section descriptions, help content, or informational messages.

    Attributes:
        key: Setting identifier (unused for labels, can be empty)
        content: The text content to display
        tooltip: Help text shown on hover
        style: Visual styling options

    Example usage:
        setting = PluginSettingValueLabel(
            content="This plugin requires an API key to function.",
            tooltip="See documentation for how to obtain an API key"
        )
    """

    content: str = field(default="")
    """
    The text content to display.

    Shown as read-only text in the settings UI.
    Can contain multiple lines.
    """

    tooltip: str = field(default="")
    """
    Help text shown on hover.

    Provides additional information when the user hovers
    over the label.
    """

    style: PluginSettingValueStyle = field(default_factory=PluginSettingValueStyle)
    """
    Visual styling options.

    Controls padding, width, and other layout properties.
    """

    def to_dict(self) -> Dict[str, Any]:
        """
        Convert to dictionary with camelCase naming.

        Returns:
            Dictionary representation
        """
        return {
            "Content": self.content,
            "Tooltip": self.tooltip,
            "Style": self.style.to_dict(),
        }


@dataclass
class PluginSettingDefinitionItem:
    """
    A complete plugin setting definition item.

    Combines a setting type with its value configuration and
    platform-specific settings.

    Attributes:
        type: The type of setting (TEXTBOX, CHECKBOX, etc.)
        value: The value configuration (specific to the type)
        disabled_in_platforms: Platforms where this setting is hidden
        is_platform_specific: Whether this setting varies by platform

    Example usage:
        # Simple textbox setting
        item = PluginSettingDefinitionItem(
            type=PluginSettingDefinitionType.TEXTBOX,
            value=PluginSettingValueTextBox(
                key="api_key",
                label="API Key"
            )
        )

        # Platform-specific setting
        item = PluginSettingDefinitionItem(
            type=PluginSettingDefinitionType.TEXTBOX,
            value=PluginSettingValueTextBox(
                key="path",
                label="Executable Path"
            ),
            disabled_in_platforms=["linux", "darwin"]
        )
    """

    type: PluginSettingDefinitionType
    """
    The type of setting UI element.

    Determines how the setting is displayed and what kind
    of input it accepts.
    """

    value: PluginSettingDefinitionValue
    """
    The value configuration for this setting.

    Should be an instance of the appropriate subclass:
    - PluginSettingValueTextBox for TEXTBOX
    - PluginSettingValueCheckBox for CHECKBOX
    - PluginSettingValueLabel for LABEL
    - etc.
    """

    disabled_in_platforms: List[str] = field(default_factory=list)
    """
    Platforms where this setting is hidden.

    List of platform identifiers where this setting should not
    be displayed. Valid values: "windows", "linux", "darwin".

    Example: ["linux", "darwin"] would hide the setting on
    Linux and macOS, showing it only on Windows.
    """

    is_platform_specific: bool = field(default=False)
    """
    Whether this setting has different values per platform.

    When True, the setting value is stored separately for each
    platform (Windows, Linux, macOS). This allows the same
    setting to have different values depending on the OS.

    Example: A file path setting might need different values
    on different platforms due to different directory structures.
    """

    def to_dict(self) -> Dict[str, Any]:
        """
        Convert to dictionary with camelCase naming.

        Returns:
            Dictionary representation
        """
        return {
            "Type": self.type,
            "Value": self.value.to_dict(),
            "DisabledInPlatforms": self.disabled_in_platforms,
            "IsPlatformSpecific": self.is_platform_specific,
        }

    def to_json(self) -> str:
        """
        Convert to JSON string.

        Returns:
            JSON string representation
        """
        return json.dumps(self.to_dict())

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "PluginSettingDefinitionItem":
        """
        Create from dictionary.

        Args:
            data: Dictionary with setting definition data

        Returns:
            A new PluginSettingDefinitionItem instance
        """
        setting_type = PluginSettingDefinitionType(data.get("Type", "textbox"))

        # Create appropriate value object based on type
        value_data = data.get("Value", {})
        value: PluginSettingDefinitionValue
        if setting_type == PluginSettingDefinitionType.TEXTBOX:
            value = PluginSettingValueTextBox(
                key=value_data.get("Key", ""),
                label=value_data.get("Label", ""),
                suffix=value_data.get("Suffix", ""),
                default_value=value_data.get("DefaultValue", ""),
                tooltip=value_data.get("Tooltip", ""),
                max_lines=value_data.get("MaxLines", 1),
                style=PluginSettingValueStyle.from_dict(value_data.get("Style", {})),
            )
        elif setting_type == PluginSettingDefinitionType.CHECKBOX:
            value = PluginSettingValueCheckBox(
                key=value_data.get("Key", ""),
                label=value_data.get("Label", ""),
                default_value=value_data.get("DefaultValue", ""),
                tooltip=value_data.get("Tooltip", ""),
                style=PluginSettingValueStyle.from_dict(value_data.get("Style", {})),
            )
        elif setting_type == PluginSettingDefinitionType.LABEL:
            value = PluginSettingValueLabel(
                key=value_data.get("Key", ""),
                content=value_data.get("Content", ""),
                tooltip=value_data.get("Tooltip", ""),
                style=PluginSettingValueStyle.from_dict(value_data.get("Style", {})),
            )
        else:
            # Default to basic value
            value = PluginSettingDefinitionValue(key=value_data.get("Key", ""), default_value=value_data.get("DefaultValue", ""))

        return cls(
            type=setting_type,
            value=value,
            disabled_in_platforms=data.get("DisabledInPlatforms", []),
            is_platform_specific=data.get("IsPlatformSpecific", False),
        )

    @classmethod
    def from_json(cls, json_str: str) -> "PluginSettingDefinitionItem":
        """
        Create from JSON string.

        Args:
            json_str: JSON string containing setting definition

        Returns:
            A new PluginSettingDefinitionItem instance
        """
        data = json.loads(json_str)
        return cls.from_dict(data)


# Helper functions for creating common setting types
#
# These convenience functions simplify creating the most common
# setting types without needing to manually construct the full
# PluginSettingDefinitionItem structure.


def create_textbox_setting(key: str, label: str, default_value: str = "", tooltip: str = "") -> PluginSettingDefinitionItem:
    """
    Create a textbox setting with commonly used defaults.

    Convenience function for creating a single-line text input
    setting with the specified properties.

    Args:
        key: Unique identifier for the setting
        label: Display label for the text box
        default_value: Default value (empty string by default)
        tooltip: Optional help text

    Returns:
        A configured PluginSettingDefinitionItem

    Example:
        setting = create_textbox_setting(
            key="username",
            label="Username",
            default_value="",
            tooltip="Enter your username"
        )
    """
    return PluginSettingDefinitionItem(
        type=PluginSettingDefinitionType.TEXTBOX,
        value=PluginSettingValueTextBox(key=key, label=label, default_value=default_value, tooltip=tooltip),
    )


def create_checkbox_setting(key: str, label: str, default_value: str = "false", tooltip: str = "") -> PluginSettingDefinitionItem:
    """
    Create a checkbox setting with commonly used defaults.

    Convenience function for creating a boolean toggle setting
    with the specified properties.

    Args:
        key: Unique identifier for the setting
        label: Display label for the checkbox
        default_value: Default value ("false" by default, use "true" for checked)
        tooltip: Optional help text

    Returns:
        A configured PluginSettingDefinitionItem

    Example:
        setting = create_checkbox_setting(
            key="auto_save",
            label="Auto Save",
            default_value="true",
            tooltip="Automatically save changes"
        )
    """
    return PluginSettingDefinitionItem(
        type=PluginSettingDefinitionType.CHECKBOX,
        value=PluginSettingValueCheckBox(key=key, label=label, default_value=default_value, tooltip=tooltip),
    )


def create_label_setting(content: str, tooltip: str = "") -> PluginSettingDefinitionItem:
    """
    Create a label setting for displaying informational text.

    Convenience function for creating a static text label
    with the specified content.

    Args:
        content: The text content to display
        tooltip: Optional help text shown on hover

    Returns:
        A configured PluginSettingDefinitionItem

    Example:
        setting = create_label_setting(
            content="Note: Changes require restarting the plugin",
            tooltip="This setting is applied on plugin startup"
        )
    """
    return PluginSettingDefinitionItem(
        type=PluginSettingDefinitionType.LABEL,
        value=PluginSettingValueLabel(
            key="",  # Labels don't need keys
            content=content,
            tooltip=tooltip,
        ),
    )
