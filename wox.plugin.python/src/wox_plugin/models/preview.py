"""
Wox Preview Models

This module provides preview models for displaying rich content in Wox results.

Previews allow plugins to show detailed information, images, files, and web
content in a dedicated preview panel when a result is selected.
"""

from dataclasses import dataclass, field
from enum import Enum
import json
from typing import Any, Dict, List, Optional

from .image import WoxImage


class WoxPreviewType(str, Enum):
    """
    Enumeration of supported preview types in Wox.

    Each type represents a different way to render preview content:
    - MARKDOWN: Render Markdown formatted text
    - TEXT: Plain text display
    - IMAGE: Display an image (using WoxImage)
    - URL: Load and display a web page
    - FILE: Display a file (various formats supported)
    - LIST: Display structured rows using WoxPreviewListData JSON
    - REMOTE: Load preview data from a remote URL
    """

    MARKDOWN = "markdown"
    """
    Markdown formatted text.

    The preview_data should contain Markdown markup which will be
    rendered to HTML for display. Supports standard Markdown syntax
    including headers, lists, code blocks, links, etc.

    Example:
        preview = WoxPreview(
            preview_type=WoxPreviewType.MARKDOWN,
            preview_data="# Header\\n\\n- Item 1\\n- Item 2"
        )
    """

    TEXT = "text"
    """
    Plain text display.

    The preview_data is displayed as-is without any formatting.
    Newlines and whitespace are preserved. Use this for simple
    text content or when you don't need rich formatting.

    Example:
        preview = WoxPreview(
            preview_type=WoxPreviewType.TEXT,
            preview_data="This is plain text.\\n\\nLine 2\\nLine 3"
        )
    """

    IMAGE = "image"
    """
    Display an image.

    The preview_data should be a WoxImage serialized to string format
    (i.e., the result of calling str(WoxImage(...))).

    Example:
        icon = WoxImage.new_absolute("/path/to/image.png")
        preview = WoxPreview(
            preview_type=WoxPreviewType.IMAGE,
            preview_data=str(icon)  # "absolute:/path/to/image.png"
        )
    """

    URL = "url"
    """
    Load and display a web page.

    The preview_data should be a URL to a web page. Wox will load
    and render the page in an embedded browser.

    Note: This may have security and privacy implications as the
    page can execute JavaScript and access cookies.

    Example:
        preview = WoxPreview(
            preview_type=WoxPreviewType.URL,
            preview_data="https://example.com"
        )
    """

    FILE = "file"
    """
    Display a file from the file system.

    The preview_data should be a file path. Wox will attempt to
    render the file based on its extension. Supported formats include:
    - Markdown files (.md)
    - Image files (.jpg, .png, .gif, .svg, etc.)
    - PDF files (.pdf)
    - Text files (.txt, .json, .xml, etc.)

    Example:
        preview = WoxPreview(
            preview_type=WoxPreviewType.FILE,
            preview_data="/path/to/document.pdf"
        )
    """

    LIST = "list"
    """
    Display a structured list of preview rows.

    The preview_data should be WoxPreviewListData.to_json(). This generic row
    shape replaces the old file-only preview so plugins can show progress,
    status, or selected files with the same visual contract.

    Example:
        data = WoxPreviewListData(items=[
            WoxPreviewListItem(
                icon=WoxImage.new_emoji("✓"),
                title="photo.jpg",
                subtitle="Saved 42 KB",
                tails=[ResultTail(text="Done", text_category=ResultTailTextCategory.SUCCESS)]
            )
        ])
        preview = WoxPreview(
            preview_type=WoxPreviewType.LIST,
            preview_data=data.to_json()
        )
    """

    REMOTE = "remote"
    """
    Load preview data from a remote URL.

    The preview_data should be a URL that returns WoxPreview JSON data
    when fetched. This allows plugins to dynamically generate previews
    from an external service.

    Example:
        preview = WoxPreview(
            preview_type=WoxPreviewType.REMOTE,
            preview_data="https://api.example.com/preview/123"
        )
    """


@dataclass
class WoxPreviewListItem:
    """
    One row in WoxPreviewType.LIST.

    Tails accept ResultTail-compatible objects. The preview model keeps this
    loose at runtime to avoid a circular import with the result model while the
    JSON contract still matches normal result tails.
    """

    icon: Optional[WoxImage] = field(default=None)
    title: str = field(default="")
    subtitle: str = field(default="")
    tails: List[Any] = field(default_factory=list)

    def to_dict(self) -> Dict[str, Any]:
        data: Dict[str, Any] = {"title": self.title}
        if self.icon is not None:
            data["icon"] = self.icon.to_dict()
        if self.subtitle:
            data["subtitle"] = self.subtitle
        if self.tails:
            data["tails"] = [_tail_to_dict(tail) for tail in self.tails]
        return data

    @classmethod
    def from_json(cls, json_data: Dict[str, Any]) -> "WoxPreviewListItem":
        raw_icon = json_data.get("icon")
        raw_tails = json_data.get("tails", [])
        return cls(
            icon=WoxImage.from_dict(raw_icon) if isinstance(raw_icon, dict) else None,
            title=str(json_data.get("title", "")),
            subtitle=str(json_data.get("subtitle", "")),
            tails=list(raw_tails) if isinstance(raw_tails, list) else [],
        )


@dataclass
class WoxPreviewListData:
    """
    Structured data for WoxPreviewType.LIST.

    The payload is row-based instead of file-path-based so long-running actions
    can update progress/status previews without custom markdown formatting.
    """

    items: List[WoxPreviewListItem] = field(default_factory=list)

    def to_json(self) -> str:
        """
        Convert to the JSON payload expected by WoxPreview.preview_data.
        """
        return json.dumps({"items": [item.to_dict() for item in self.items]})

    @classmethod
    def from_json(cls, json_data: Dict[str, Any]) -> "WoxPreviewListData":
        """
        Create list preview data from a decoded JSON object.
        """
        raw_items = json_data.get("items", [])
        return cls(
            items=[WoxPreviewListItem.from_json(item) for item in raw_items if isinstance(item, dict)]
            if isinstance(raw_items, list)
            else []
        )

    @classmethod
    def from_preview_data(cls, preview_data: str) -> "WoxPreviewListData":
        """
        Decode the string stored in WoxPreview.preview_data.
        """
        decoded = json.loads(preview_data)
        return cls.from_json(decoded if isinstance(decoded, dict) else {})


def _tail_to_dict(tail: Any) -> Dict[str, Any]:
    if hasattr(tail, "to_json"):
        return json.loads(tail.to_json())
    if isinstance(tail, dict):
        return tail
    raise TypeError(f"Unsupported list preview tail payload: {type(tail)!r}")


class WoxPreviewScrollPosition(str, Enum):
    """
    Enumeration of preview scroll positions.

    Controls where the preview content is scrolled when first displayed.
    """

    BOTTOM = "bottom"
    """
    Scroll to the bottom after preview first shows.

    Use this for content that grows from the top (like logs, chat messages,
    or terminal output) so the user sees the most recent content first.
    """


@dataclass
class WoxPreviewTag:
    """
    Metadata tag shown below preview content.

    The UI now renders preview metadata as compact tags instead of a key/value
    table, so the visible label is separate from the optional tooltip text.
    """

    label: str = field(default="")
    tooltip: str = field(default="")

    def to_dict(self) -> Dict[str, str]:
        return {
            "Label": self.label,
            "Tooltip": self.tooltip,
        }

    @classmethod
    def from_json(cls, json_data: Dict[str, Any]) -> "WoxPreviewTag":
        return cls(
            label=str(json_data.get("Label", "")),
            tooltip=str(json_data.get("Tooltip", "")),
        )


@dataclass
class WoxPreview:
    """
    Preview model for displaying rich content in Wox results.

    Previews are shown in a side panel when a result is selected, allowing
    plugins to display detailed information without cluttering the main
    results list.

    Attributes:
        preview_type: The type of preview content to display
        preview_data: The actual content data (format depends on preview_type)
        preview_tags: Optional metadata tags shown below preview content
        preview_properties: Deprecated key/value metadata kept for compatibility
        scroll_position: Initial scroll position when preview is shown

    Example usage:
        # Markdown preview
        preview = WoxPreview(
            preview_type=WoxPreviewType.MARKDOWN,
            preview_data="# Documentation\\n\\nThis is **bold** text."
        )

        # Image preview
        icon = WoxImage.new_absolute("/path/to/screenshot.png")
        preview = WoxPreview(
            preview_type=WoxPreviewType.IMAGE,
            preview_data=str(icon)
        )

        # File preview
        preview = WoxPreview(
            preview_type=WoxPreviewType.FILE,
            preview_data="/path/to/readme.md"
        )
    """

    preview_type: WoxPreviewType = field(default=WoxPreviewType.TEXT)
    """
    The type of preview content to display.

    Determines how the preview_data is interpreted and rendered.
    """

    preview_data: str = field(default="")
    """
    The actual preview content.

    The format of this field depends on preview_type:
    - MARKDOWN: Markdown markup string
    - TEXT: Plain text string
    - IMAGE: WoxImage serialized as "type:value" string
    - URL: HTTP/HTTPS URL
    - FILE: File system path
    - LIST: WoxPreviewListData JSON string
    - REMOTE: URL that returns WoxPreview JSON
    """

    preview_properties: Dict[str, str] = field(default_factory=dict)
    """
    Deprecated: use preview_tags instead.

    The launcher still maps each legacy key/value pair to a metadata tag with
    the value as the visible label and the key as the tooltip, so older plugins
    keep their current display while new plugins can use preview_tags directly.
    """

    scroll_position: WoxPreviewScrollPosition = field(default=WoxPreviewScrollPosition.BOTTOM)
    """
    Initial scroll position when preview is first displayed.

    Controls where the content is scrolled when the preview appears.
    Default is BOTTOM which scrolls to the end of the content.
    """

    preview_tags: List[WoxPreviewTag] = field(default_factory=list)
    """
    Metadata tags shown below the preview content.

    This field is intentionally appended after existing constructor fields so
    older positional WoxPreview(...) calls keep their argument meanings while
    new plugins can pass preview_tags by keyword.
    """

    def to_json(self) -> str:
        """
        Convert to JSON string with camelCase naming.

        The output uses camelCase property names for compatibility
        with the Wox C# backend.

        Returns:
            JSON string representation of this preview
        """
        return json.dumps(
            {
                "PreviewType": self.preview_type,
                "PreviewData": self.preview_data,
                "PreviewTags": [tag.to_dict() for tag in self.preview_tags],
                "PreviewProperties": self.preview_properties,
                "ScrollPosition": self.scroll_position,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "WoxPreview":
        """
        Create from JSON string with camelCase naming.

        Args:
            json_str: JSON string containing preview data

        Returns:
            A new WoxPreview instance
        """
        data = json.loads(json_str)

        if not data.get("PreviewType"):
            data["PreviewType"] = WoxPreviewType.TEXT

        if not data.get("ScrollPosition"):
            data["ScrollPosition"] = WoxPreviewScrollPosition.BOTTOM

        return cls(
            preview_type=WoxPreviewType(data.get("PreviewType")),
            preview_data=data.get("PreviewData", ""),
            preview_tags=[WoxPreviewTag.from_json(item) for item in data.get("PreviewTags", []) if isinstance(item, dict)]
            if isinstance(data.get("PreviewTags", []), list)
            else [],
            preview_properties=data.get("PreviewProperties", {}),
            scroll_position=WoxPreviewScrollPosition(data.get("ScrollPosition")),
        )
