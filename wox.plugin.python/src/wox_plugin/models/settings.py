from typing import List, Callable
from pydantic import BaseModel

from ..types import Platform, PluginSettingDefinitionType
from .context import Context


class PluginSettingValueStyle(BaseModel):
    """Style configuration for plugin settings"""

    PaddingLeft: int
    PaddingTop: int
    PaddingRight: int
    PaddingBottom: int
    Width: int
    LabelWidth: int


class PluginSettingDefinitionValue(BaseModel):
    """Base class for plugin setting definition values"""

    def get_key(self) -> str:
        """Get the key of the setting"""
        raise NotImplementedError

    def get_default_value(self) -> str:
        """Get the default value of the setting"""
        raise NotImplementedError

    def translate(self, translator: Callable[[Context, str], str]) -> None:
        """Translate the setting using the provided translator"""
        raise NotImplementedError


class PluginSettingDefinitionItem(BaseModel):
    """Plugin setting definition item"""

    Type: PluginSettingDefinitionType
    Value: PluginSettingDefinitionValue
    DisabledInPlatforms: List[Platform]
    IsPlatformSpecific: bool


class MetadataCommand(BaseModel):
    """Metadata for plugin commands"""

    Command: str
    Description: str
