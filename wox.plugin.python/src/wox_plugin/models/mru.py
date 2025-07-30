from dataclasses import dataclass
from typing import Optional, Callable, TYPE_CHECKING
from .image import WoxImage

if TYPE_CHECKING:
    from .result import Result


@dataclass
class MRUData:
    """MRU (Most Recently Used) data structure"""
    plugin_id: str
    title: str
    sub_title: str
    icon: WoxImage
    context_data: str

    @classmethod
    def from_dict(cls, data: dict) -> 'MRUData':
        """Create MRUData from dictionary"""
        return cls(
            plugin_id=data.get('PluginID', ''),
            title=data.get('Title', ''),
            sub_title=data.get('SubTitle', ''),
            icon=WoxImage.from_dict(data.get('Icon', {})),
            context_data=data.get('ContextData', '')
        )

    def to_dict(self) -> dict:
        """Convert MRUData to dictionary"""
        return {
            'PluginID': self.plugin_id,
            'Title': self.title,
            'SubTitle': self.sub_title,
            'Icon': self.icon.to_dict(),
            'ContextData': self.context_data
        }


# Type alias for MRU restore callback
# Note: We use forward reference to avoid circular import
MRURestoreCallback = Callable[['MRUData'], Optional['Result']]
